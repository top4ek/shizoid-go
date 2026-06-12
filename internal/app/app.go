package app

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"sync/atomic"

	"go.uber.org/zap"

	"shizoid/internal/config"
	"shizoid/internal/generator"
	"shizoid/internal/logger"
	"shizoid/internal/models"
	"shizoid/internal/neural"
)

var (
	gen         *generator.Generator
	neuralCli   *neural.Client
	botID       atomic.Int64
	botUsername atomic.Value
)

// Init wires the data layer, neural client and generator from a database handle.
func Init(db *sql.DB) {
	models.Init(db)
	neuralCli = neural.New(config.Neural.Reply, config.Neural.Summary)
	gen = generator.New(neuralCli)
	logger.Instance().Debug("app init",
		zap.Int("neural_reply_providers", len(config.Neural.Reply)),
		zap.Int("neural_summary_providers", len(config.Neural.Summary)),
		zap.Bool("neural_configured", neuralCli.ReplyConfigured()),
	)
}

// Gen returns the shared text generator.
func Gen() *generator.Generator { return gen }

// Neural returns the shared neural client (may be nil if not initialized).
func Neural() *neural.Client { return neuralCli }

// Ready reports whether the data layer is initialized.
func Ready() bool { return models.DB() != nil }

// SetBotID records the bot's own Telegram id.
func SetBotID(id int64) {
	botID.Store(id)
	if gen != nil {
		gen.SetBotID(id)
	}
}

// BotID returns the bot's own Telegram id.
func BotID() int64 { return botID.Load() }

// SetBotUsername records the bot's Telegram @username (without @).
func SetBotUsername(username string) {
	botUsername.Store(strings.ToLower(username))
}

// BotUsername returns the bot's Telegram @username (without @).
func BotUsername() string {
	v, _ := botUsername.Load().(string)
	return v
}

// IsOwner reports whether the user id is a configured bot owner.
func IsOwner(userID int64) bool {
	return slices.Contains(config.Environment.BotOwners, userID)
}

type ctxKey int

const (
	chatKey ctxKey = iota
	participationKey
	skipMessageHistoryKey
)

// WithChat stores the resolved chat in the context.
func WithChat(ctx context.Context, c *models.Chat) context.Context {
	return context.WithValue(ctx, chatKey, c)
}

// ChatFrom retrieves the resolved chat from the context (nil if absent).
func ChatFrom(ctx context.Context) *models.Chat {
	c, _ := ctx.Value(chatKey).(*models.Chat)
	return c
}

// WithParticipation stores the resolved participation in the context.
func WithParticipation(ctx context.Context, p *models.Participation) context.Context {
	return context.WithValue(ctx, participationKey, p)
}

// ParticipationFrom retrieves the participation from the context (nil if absent).
func ParticipationFrom(ctx context.Context) *models.Participation {
	p, _ := ctx.Value(participationKey).(*models.Participation)
	return p
}

// WithSkipMessageHistory marks the update as a bot command; replies must not be stored.
func WithSkipMessageHistory(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipMessageHistoryKey, true)
}

// SkipMessageHistory reports whether message history persistence should be skipped.
func SkipMessageHistory(ctx context.Context) bool {
	skip, _ := ctx.Value(skipMessageHistoryKey).(bool)
	return skip
}

// Locale returns the locale to use for the current chat, falling back to config.
func Locale(ctx context.Context) string {
	if c := ChatFrom(ctx); c != nil && c.Locale != "" {
		return c.Locale
	}
	if config.Environment.Locale != "" {
		return config.Environment.Locale
	}
	return "ru"
}

// Enabled reports whether the bot should act in the current chat.
func Enabled(ctx context.Context) bool {
	if config.Environment.AllowToAll {
		return true
	}
	c := ChatFrom(ctx)
	return c != nil && c.Enabled()
}
