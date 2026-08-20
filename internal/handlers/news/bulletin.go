// Package news posts a daily satirical news issue about what happened in a chat
// and lets chat admins switch it on with a style.
package news

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/config"
	"shizoid/internal/logger"
	"shizoid/internal/models"
	"shizoid/internal/telegram"
	"shizoid/internal/utils"
)

const (
	// window is how far back one issue looks.
	window = 24 * time.Hour
	// minMessages keeps a quiet chat from getting an issue about nothing.
	minMessages = 10
	// budgetMargin mirrors the scheduler's summary margin: slack for the role
	// labels and JSON envelope the byte budget cannot account for.
	budgetMargin = 256

	logHeader = "Chat log:\n"
)

// PostChat builds and sends one news issue for a chat, reporting whether an
// issue went out.
func PostChat(ctx context.Context, b *bot.Bot, chat *models.Chat, now time.Time) bool {
	if !chat.NewsEnabled() {
		return false
	}
	system := buildSystem(chat)
	budget := messageBudget(system)
	if budget <= 0 {
		return false
	}

	rows, err := app.Store().Messages.RecentByBytesSince(ctx, chat.ID, now.Add(-window), budget)
	if err != nil {
		logger.Instance().Error("news: messages", zap.Int64("chat_id", chat.ID), zap.Error(err))
		return false
	}
	lines := fitLog(formatLog(rows, app.BotID()), budget)
	if len(lines) < minMessages {
		return false
	}

	logger.Instance().Debug("news issue",
		zap.Int64("chat_id", chat.ID),
		zap.Int("messages", len(lines)),
		zap.Int("system_bytes", len(system)),
	)
	text, err := app.Neural().News(ctx, system, buildUserMessage(chat.Locale, lines))
	if err != nil {
		logger.Instance().Warn("news: generate", zap.Int64("chat_id", chat.ID), zap.Error(err))
		return false
	}
	if text = strings.TrimSpace(text); text == "" {
		return false
	}

	if _, err := telegram.SendToChat(ctx, b, chat.ID, text, telegram.ChatMessageOpts{
		DisableNotification: true,
		DisableLinkPreview:  true,
	}); err != nil {
		logger.Instance().Error("news: send", zap.Int64("chat_id", chat.ID), zap.Error(err))
		return false
	}
	return true
}

// buildSystem layers the global issue instructions, the chat's requested style
// and the chat's persona prompt as labeled blocks. config.Environment.AppPrompt
// is deliberately left out: its length and formatting rules cap replies at a few
// sentences with no line starting a list, which a multi-item issue cannot obey.
func buildSystem(chat *models.Chat) string {
	var parts []string
	if p := strings.TrimSpace(config.Environment.NewsPrompt); p != "" {
		parts = append(parts, p)
	}
	if chat.News.Valid {
		if style := strings.TrimSpace(chat.News.String); style != "" {
			parts = append(parts, config.NewsStyleHeader+style)
		}
	}
	if chat.SystemPrompt.Valid {
		if p := strings.TrimSpace(chat.SystemPrompt.String); p != "" {
			parts = append(parts, config.ChatInstructionsHeader+p)
		}
	}
	return strings.Join(parts, "\n\n")
}

func buildUserMessage(lang string, lines []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Chat locale: %s\n\n", lang)
	b.WriteString(logHeader)
	b.WriteString(strings.Join(lines, "\n"))
	return b.String()
}

// formatLog turns newest-first rows into chronological "Name: text" lines,
// dropping the bot's own messages so an issue never reports on itself. One row
// must yield exactly one line: the whole log is flattened into a single user
// message, so a member whose text contained a newline could otherwise forge an
// extra "Name: text" entry.
func formatLog(rows []models.MessageRow, botID int64) []string {
	lines := make([]string, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		text := strings.TrimSpace(row.Text)
		if text == "" {
			continue
		}
		if row.UserID == botID || (row.IsBot.Valid && row.IsBot.Bool) {
			continue
		}
		name := utils.DisplayName(row.Username.String, row.FirstName.String, row.LastName.String)
		lines = append(lines, oneLine(name)+": "+oneLine(text))
	}
	return lines
}

// oneLine collapses the line breaks that would let a message forge a log entry.
func oneLine(s string) string {
	return strings.TrimSpace(lineBreaks.Replace(s))
}

var lineBreaks = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")

// fitLog drops the oldest lines until the rendered log fits the budget. The
// query budget counts message text only, while the log also carries a name per
// line, so the excess is trimmed here.
func fitLog(lines []string, budget int) []string {
	size := 0
	for _, l := range lines {
		size += len(l) + 1
	}
	for size > budget && len(lines) > 0 {
		size -= len(lines[0]) + 1
		lines = lines[1:]
	}
	return lines
}

func messageBudget(system string) int {
	return config.MaxSummaryContextBytes - len(system) - len(logHeader) - budgetMargin
}
