package generator

import (
	"context"
	"database/sql"
	"math/rand/v2"
	"strconv"
	"strings"
	"unicode"

	"go.uber.org/zap"

	"shizoid/internal/config"
	"shizoid/internal/logger"
	"shizoid/internal/models"
	"shizoid/internal/neural"
)

const (
	safetyCounter = 50
	maxStoryRunes = 1024
)

var sentenceEnders = map[rune]struct{}{'.': {}, '!': {}, '?': {}, '…': {}}

type Generator struct {
	neural *neural.Client
	botID  int64
}

func New(n *neural.Client) *Generator {
	return &Generator{neural: n}
}

// SetBotID records the bot's Telegram user id for role assignment in neural history.
func (g *Generator) SetBotID(id int64) { g.botID = id }

// Reply generates a response seeded from the incoming words, falling back to the
// chat's stored context if the seed yields nothing. In neural mode it first tries
// the neural client and falls back to the classic Markov path on any failure
// (busy slots, timeout or an unavailable backend).
func (g *Generator) Reply(ctx context.Context, chat *models.Chat, words []string, userID int64) (string, error) {
	if chat.GenerationMode == models.GenerationModeNeural && g.neural.ReplyConfigured() {
		logger.Instance().Debug("generator neural attempt",
			zap.Int64("chat_id", chat.ID),
			zap.Int("words", len(words)),
		)
		reply, err := g.neuralReply(ctx, chat, words, userID)
		if err == nil && reply != "" {
			return reply, nil
		}
		logger.Instance().Warn("neural fallback to classic",
			zap.Int64("chat_id", chat.ID),
			zap.Error(err),
		)
	}

	seedIDs, err := g.idsOf(ctx, words)
	if err != nil {
		return "", err
	}
	if reply, err := g.buildSentence(ctx, chat, seedIDs); err != nil {
		return "", err
	} else if reply != "" {
		return reply, nil
	}

	contextIDs, err := g.contextIDs(ctx, chat)
	if err != nil {
		return "", err
	}
	return g.buildSentence(ctx, chat, contextIDs)
}

func (g *Generator) Story(ctx context.Context, chat *models.Chat) (string, error) {
	contextIDs, err := g.contextIDs(ctx, chat)
	if err != nil {
		return "", err
	}
	if len(contextIDs) == 0 {
		return "", nil
	}
	seen := make(map[string]struct{})
	var sentences []string
	var storyLen int
	for range contextIDs {
		s, err := g.buildSentence(ctx, chat, contextIDs)
		if err != nil {
			return "", err
		}
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		add := len([]rune(s))
		if len(sentences) > 0 {
			add += len([]rune(storySeparatorAfter(sentences[len(sentences)-1])))
		}
		if storyLen+add > maxStoryRunes {
			break
		}
		seen[s] = struct{}{}
		sentences = append(sentences, s)
		storyLen += add
	}
	return truncateRunes(joinStorySentences(sentences), maxStoryRunes), nil
}

func (g *Generator) Learn(ctx context.Context, chatID int64, text string) error {
	tokens := strings.Fields(text)
	if len(tokens) == 0 {
		return nil
	}
	if err := models.Words.EnsureWords(ctx, tokens); err != nil {
		return err
	}
	idMap, err := models.Words.ToIDs(ctx, tokens)
	if err != nil {
		return err
	}

	null := sql.NullInt64{}
	seq := []sql.NullInt64{null}
	for _, tok := range tokens {
		if id, ok := idMap[tok]; ok {
			seq = append(seq, sql.NullInt64{Int64: id, Valid: true})
		} else {
			seq = append(seq, null)
		}
		if endsSentence(tok) {
			seq = append(seq, null)
		}
	}
	if last := seq[len(seq)-1]; last.Valid {
		seq = append(seq, null)
	}

	for len(seq) > 0 {
		first := seq[0]
		second := at(seq, 1)
		third := at(seq, 2)
		if !first.Valid && !third.Valid && len(seq) > 2 {
			seq = seq[3:]
			continue
		}
		seq = seq[1:]
		if err := models.Pairs.LearnTrigram(ctx, chatID, first, second, third); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) buildSentence(ctx context.Context, chat *models.Chat, seedIDs []int64) (string, error) {
	if len(seedIDs) == 0 {
		return "", nil
	}
	simplified := chat.GenerationMode == models.GenerationModeSimplified

	first := models.MatchNull()
	second := models.MatchIn(seedIDs)

	var ids []sql.NullInt64
	for len(ids) < safetyCounter {
		pair, err := models.Pairs.FetchPair(ctx, chat.ID, first, second)
		if err != nil {
			return "", err
		}
		if pair == nil {
			break
		}
		if len(ids) == 0 {
			ids = append(ids, pair.SecondID)
		}
		reply := pickReply(pair.Replies)
		if reply == nil || !reply.WordID.Valid {
			break
		}
		ids = append(ids, reply.WordID)
		if simplified {
			first = models.MatchAny()
		} else {
			first = models.MatchEq(pair.SecondID)
		}
		second = models.MatchEq(reply.WordID)
	}

	return g.idsToSentence(ctx, ids)
}

func (g *Generator) idsToSentence(ctx context.Context, ids []sql.NullInt64) (string, error) {
	var valid []int64
	for _, id := range ids {
		if id.Valid {
			valid = append(valid, id.Int64)
		}
	}
	if len(valid) == 0 {
		return "", nil
	}
	wordsByID, err := models.Words.ToWords(ctx, valid)
	if err != nil {
		return "", err
	}
	out := make([]string, 0, len(valid))
	for _, id := range valid {
		if w, ok := wordsByID[id]; ok {
			out = append(out, w)
		}
	}
	return capitalize(strings.Join(out, " ")), nil
}

func (g *Generator) idsOf(ctx context.Context, words []string) ([]int64, error) {
	if len(words) == 0 {
		return nil, nil
	}
	idMap, err := models.Words.ToIDs(ctx, words)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for _, w := range words {
		if id, ok := idMap[w]; ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (g *Generator) contextIDs(ctx context.Context, chat *models.Chat) ([]int64, error) {
	texts, err := models.Messages.RecentTextsByBytes(ctx, chat.ID, config.MaxReplyContextBytes)
	if err != nil {
		return nil, err
	}
	var words []string
	for _, t := range texts {
		words = append(words, strings.Fields(t)...)
	}
	ids, err := g.idsOf(ctx, words)
	if err != nil {
		return nil, err
	}
	if len(ids) > len(words) {
		ids = ids[:len(words)]
	}
	return ids, nil
}

// neuralReply assembles the payload (system prompt, chat history) and asks the
// neural client for an answer.
func (g *Generator) neuralReply(ctx context.Context, chat *models.Chat, words []string, userID int64) (string, error) {
	current := strings.TrimSpace(strings.Join(words, " "))
	if current == "" {
		return "", nil
	}
	system := g.buildNeuralSystem(chat)
	history := g.neuralHistory(ctx, chat, current, userID)
	logger.Instance().Debug("generator neural context",
		zap.Int64("chat_id", chat.ID),
		zap.Int("system_bytes", len(system)),
		zap.Int("history_messages", len(history)),
	)
	return g.neural.ReplyWithHistory(ctx, system, history)
}

func (g *Generator) buildNeuralSystem(chat *models.Chat) string {
	var parts []string
	if p := strings.TrimSpace(config.Environment.AppPrompt); p != "" {
		parts = append(parts, p)
	}
	if chat.SystemPrompt.Valid {
		if p := strings.TrimSpace(chat.SystemPrompt.String); p != "" {
			parts = append(parts, p)
		}
	}
	if chat.Memory.Valid {
		if m := strings.TrimSpace(chat.Memory.String); m != "" {
			parts = append(parts, "Long-term chat memory:\n"+m)
		}
	}
	return strings.Join(parts, "\n\n")
}

func (g *Generator) neuralHistory(ctx context.Context, chat *models.Chat, currentUser string, currentUserID int64) []neural.HistoryMessage {
	if config.MaxReplyContextBytes <= 0 {
		return appendCurrentMessage(nil, currentUser, currentUserID, 0)
	}
	rows, err := models.Messages.RecentByBytes(ctx, chat.ID, config.MaxReplyContextBytes)
	if err != nil {
		return appendCurrentMessage(nil, currentUser, currentUserID, 0)
	}
	return buildNeuralHistory(rows, g.botID, currentUser, currentUserID)
}

func buildNeuralHistory(rows []models.MessageRow, botID int64, currentUser string, currentUserID int64) []neural.HistoryMessage {
	var dedupedUserID int64
	if len(rows) > 0 && rows[0].UserID == currentUserID && strings.TrimSpace(rows[0].Text) == currentUser {
		dedupedUserID = rows[0].UserID
		rows = rows[1:]
	}

	msgs := make([]neural.HistoryMessage, 0, len(rows)+1)
	for i := len(rows) - 1; i >= 0; i-- {
		text := strings.TrimSpace(rows[i].Text)
		if text == "" {
			continue
		}
		if rows[i].UserID == botID || (rows[i].IsBot.Valid && rows[i].IsBot.Bool) {
			msgs = append(msgs, neural.HistoryMessage{Role: "assistant", Text: text})
			continue
		}
		msgs = append(msgs, neural.HistoryMessage{
			Role: "user",
			Name: strconv.FormatInt(rows[i].UserID, 10),
			Text: text,
		})
	}
	msgs = appendCurrentMessage(msgs, currentUser, currentUserID, dedupedUserID)
	if len(msgs) == 0 {
		return nil
	}
	return normalizeRoleAlternation(msgs)
}

// normalizeRoleAlternation inserts empty assistant turns between consecutive user
// messages so strict chat templates (Gemma, some Qwen builds) accept the payload.
func normalizeRoleAlternation(msgs []neural.HistoryMessage) []neural.HistoryMessage {
	if len(msgs) < 2 {
		return msgs
	}
	out := make([]neural.HistoryMessage, 0, len(msgs)*2)
	for _, msg := range msgs {
		if len(out) > 0 && out[len(out)-1].Role == msg.Role {
			switch msg.Role {
			case "user":
				out = append(out, neural.HistoryMessage{Role: "assistant", Text: "."})
			case "assistant":
				prev := out[len(out)-1]
				prev.Text = strings.TrimSpace(prev.Text + "\n\n" + msg.Text)
				out[len(out)-1] = prev
				continue
			}
		}
		out = append(out, msg)
	}
	return out
}

func appendCurrentMessage(msgs []neural.HistoryMessage, currentUser string, currentUserID, dedupedUserID int64) []neural.HistoryMessage {
	currentUser = strings.TrimSpace(currentUser)
	if currentUser == "" {
		return msgs
	}
	if len(msgs) > 0 {
		last := msgs[len(msgs)-1]
		if last.Role == "user" && last.Text == currentUser {
			id := dedupedUserID
			if id == 0 {
				id = currentUserID
			}
			if id == 0 || last.Name == strconv.FormatInt(id, 10) {
				return msgs
			}
		}
	}
	userID := dedupedUserID
	if userID == 0 {
		userID = currentUserID
	}
	msg := neural.HistoryMessage{Role: "user", Text: currentUser}
	if userID != 0 {
		msg.Name = strconv.FormatInt(userID, 10)
	}
	return append(msgs, msg)
}

func pickReply(replies []models.ReplyRow) *models.ReplyRow {
	if len(replies) == 0 {
		return nil
	}
	pool := 3 + len(replies)/2
	if pool > len(replies) {
		pool = len(replies)
	}
	return &replies[rand.IntN(pool)]
}

func endsSentence(token string) bool {
	r := []rune(token)
	if len(r) == 0 {
		return false
	}
	_, ok := sentenceEnders[r[len(r)-1]]
	return ok
}

func storySeparatorAfter(prev string) string {
	if endsSentence(prev) {
		return " "
	}
	return ". "
}

func joinStorySentences(sentences []string) string {
	if len(sentences) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(sentences[0])
	for i := 1; i < len(sentences); i++ {
		b.WriteString(storySeparatorAfter(sentences[i-1]))
		b.WriteString(sentences[i])
	}
	return b.String()
}

func at(s []sql.NullInt64, i int) sql.NullInt64 {
	if i < len(s) {
		return s[i]
	}
	return sql.NullInt64{}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
