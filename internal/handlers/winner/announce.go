package winner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/config"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/models"
	"shizoid/internal/telegram"
	"shizoid/internal/utils"
)

const (
	// announceTimeout bounds the model call so the job's slice of the queue
	// budget keeps room to send the message if the model stalls.
	announceTimeout = 4 * time.Minute
	// announcementTTL is how long a queued announcement stays worth sending.
	// Past it the ceremony is about a day nobody remembers.
	announcementTTL = 48 * time.Hour
	// topLimit is how many rows the year leaderboard carries.
	topLimit = 10
	// scorerLimit is how many scorers the announcement talks about. It matches
	// the pool the draw picks the winner from.
	scorerLimit = 3
	// blockSeparatorRunes is what joinBlock puts between two blocks.
	blockSeparatorRunes = 2
)

// announcement is the snapshot one draw is queued as: the prompt the model is
// asked to run, and the deterministic text that goes under its answer. It has
// to be a snapshot because the inputs do not survive the wait - the draw resets
// the day's scores and the prune trims the chat log it quotes.
type announcement struct {
	System string `json:"system"`
	User   string `json:"user"`
	Result string `json:"result"`
}

// Announce delivers the result of one daily draw. With a summary chain
// configured the whole announcement is queued: its ceremony needs the model, and
// the draw row is already committed, so it waits for one rather than going out
// bare. Without a chain there is nothing to wait for and it is sent right away.
func Announce(ctx context.Context, b *bot.Bot, chat *models.Chat, chosen models.ScoreEntry, standings []models.ScoreEntry, now time.Time) error {
	a := render(ctx, chat, chosen, standings, now)
	if !app.Neural().SummaryConfigured() {
		return send(ctx, b, chat.ID, a.Result)
	}
	payload, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return app.Store().SummaryJobs.Enqueue(ctx, chat.ID, models.SummaryJobWinner, payload, announcementTTL)
}

// Deliver posts one queued announcement. A failure is returned rather than
// logged so the queue retries it, the send included - unless Telegram refused
// for good, in which case retrying would only re-run the model against a chat
// that can never receive the message.
func Deliver(ctx context.Context, b *bot.Bot, chatID int64, payload []byte) error {
	var a announcement
	if err := json.Unmarshal(payload, &a); err != nil {
		return err
	}
	text, err := neuralText(ctx, a.System, a.User)
	if err != nil {
		return err
	}
	err = send(ctx, b, chatID, joinBlock(fitCeremony(text, a.Result), a.Result))
	if err != nil && telegram.IsPermanentError(err) {
		logger.Instance().Error("winner announcement dropped",
			zap.Int64("chat_id", chatID),
			zap.Error(err),
		)
		return nil
	}
	return err
}

// fitCeremony bounds the model's half of the announcement. The 4-8 sentence
// limit is a prompt instruction the model is free to ignore, and the message is
// cut at its end, so an unbounded ceremony would push the winner line, the
// mention and the year leaderboard out of the message. The two halves are
// sanitized apart as well: the result block is already MarkdownV2, and a stray
// marker in the model's text must not pair with one of its markers.
func fitCeremony(text, result string) string {
	budget := telegram.MaxMessageRunes - utf8.RuneCountInString(result) - blockSeparatorRunes
	return telegram.FitV2(text, budget)
}

// render assembles both halves of an announcement: the prompt for the model and
// the result block that goes under whatever it answers. The leaderboard is read
// here, so a queued announcement reports the standings as of the draw.
func render(ctx context.Context, chat *models.Chat, chosen models.ScoreEntry, standings []models.ScoreEntry, now time.Time) announcement {
	lang := chat.Locale
	label := winnerLabel(chat.Winner.String, lang)
	year, err := app.Store().Winners.TopOfYear(ctx, chat.ID, topLimit)
	if err != nil {
		logger.Instance().Error("winners: top of year", zap.Error(err))
	}
	system, user := buildPrompt(ctx, chat, label, chosen, standings, now)
	return announcement{
		System: system,
		User:   user,
		Result: joinBlock(locale.T(lang, "winner.winner",
			"name", telegram.FormatPlain(label),
			"user", FormatWinnerUser(lang, chosen.UserID, chosen.Username, chosen.Name)),
			yearBlock(lang, year)),
	}
}

func send(ctx context.Context, b *bot.Bot, chatID int64, text string) error {
	_, err := telegram.SendToChat(ctx, b, chatID, text, telegram.ChatMessageOpts{
		DisableNotification: true,
		DisableLinkPreview:  true,
	})
	return err
}

func winnerLabel(label, lang string) string {
	if label == "" {
		return locale.T(lang, "winner.default")
	}
	return label
}

// buildPrompt assembles the announcement prompt out of the draw and the last day
// of chat. Both halves come back empty when no summary chain is configured:
// there is nothing to run them on.
func buildPrompt(ctx context.Context, chat *models.Chat, label string, chosen models.ScoreEntry, standings []models.ScoreEntry, now time.Time) (system, user string) {
	if !app.Neural().SummaryConfigured() {
		return "", ""
	}
	system = buildSystem(chat)
	budget := logBudget(len(system))
	rows := chatRows(ctx, chat.ID, budget, now)
	draw := buildDraw(chat.Locale, label, chosen, standings, rows)
	user = buildUserMessage(draw, fitLog(formatLog(rows, app.BotID()), budget))

	logger.Instance().Debug("winner announcement",
		zap.Int64("chat_id", chat.ID),
		zap.Int("system_bytes", len(system)),
		zap.Int("user_bytes", len(user)),
	)
	return system, user
}

// neuralText runs the announcement prompt. An empty prompt means no summary
// chain is configured, which is not a failure: the announcement is just the
// result block then.
func neuralText(ctx context.Context, system, user string) (string, error) {
	if system == "" && user == "" {
		return "", nil
	}
	callCtx, cancel := context.WithTimeout(ctx, announceTimeout)
	defer cancel()
	text, err := app.Neural().Winner(callCtx, system, user)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

// buildSystem layers the global announcement instructions, the chat's persona
// and its long-term memory as labeled blocks. config.Environment.AppPrompt is
// deliberately left out: its length and formatting rules cap replies at a few
// sentences, which a multi-part ceremony cannot obey.
func buildSystem(chat *models.Chat) string {
	var parts []string
	if p := strings.TrimSpace(config.Environment.WinnerPrompt); p != "" {
		parts = append(parts, p)
	}
	if chat.SystemPrompt.Valid {
		if p := strings.TrimSpace(chat.SystemPrompt.String); p != "" {
			parts = append(parts, config.ChatInstructionsHeader+p)
		}
	}
	if chat.Memory.Valid {
		if m := strings.TrimSpace(chat.Memory.String); m != "" {
			parts = append(parts, config.ChatMemoryHeader+m)
		}
	}
	return strings.Join(parts, "\n\n")
}

// buildUserMessage states the draw, then appends whatever of the last day of
// chat fitted the context budget. An empty log is not an error: the prompt has
// the model improvise that part from the scores and the memory.
func buildUserMessage(draw string, lines []string) string {
	if len(lines) == 0 {
		return draw
	}
	var b strings.Builder
	b.WriteString(draw)
	b.WriteString("\n")
	b.WriteString(logHeader)
	b.WriteString(strings.Join(lines, "\n"))
	return b.String()
}

// buildDraw states the draw as one line per scorer, each carrying that scorer's
// own words of the day: without them the model has nothing to say about why a
// place was earned. Places are spelled out rather than numbered, and the list
// length is stated, so the model cannot invent a place that was not drawn.
func buildDraw(lang, label string, chosen models.ScoreEntry, standings []models.ScoreEntry, rows []models.MessageRow) string {
	top := standings
	if len(top) > scorerLimit {
		top = top[:scorerLimit]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Chat locale: %s\n\n", lang)
	fmt.Fprintf(&b, "Prize: %s\n", oneLine(label))
	fmt.Fprintf(&b, "Scorers of the day, %d in total:\n", len(top))
	for i, e := range top {
		name := scorerName(lang, e)
		fmt.Fprintf(&b, "Place %d: %s - %d points", i+1, name, e.Score)
		if e.UserID == chosen.UserID {
			b.WriteString(" (won the prize)")
		}
		if said := scorerExcerpt(rows, e.UserID); said != "" {
			fmt.Fprintf(&b, ". What %s wrote today: %s", name, said)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// scorerName renders a scorer the way formatLog renders their log lines, so the
// model can tie the two together.
func scorerName(lang string, e models.ScoreEntry) string {
	if e.Username == "" && oneLine(e.Name) == "" {
		return locale.T(lang, "winner.default")
	}
	return oneLine(utils.DisplayName(e.Username, e.Name, ""))
}

func chatRows(ctx context.Context, chatID int64, budget int, now time.Time) []models.MessageRow {
	if budget <= 0 {
		return nil
	}
	rows, err := app.Store().Messages.RecentByBytesSince(ctx, chatID, now.Add(-window), budget)
	if err != nil {
		logger.Instance().Error("winners: messages", zap.Int64("chat_id", chatID), zap.Error(err))
		return nil
	}
	return rows
}
