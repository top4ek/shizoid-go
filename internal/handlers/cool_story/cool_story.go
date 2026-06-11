package cool_story

import (
	"context"
	"math/rand/v2"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/telegram"
)

const (
	Command     = "cool_story"
	Description = "Cool story bro"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || !app.Enabled(ctx) {
		return
	}
	chat := app.ChatFrom(ctx)
	lang := app.Locale(ctx)
	if chat == nil || !app.Ready() {
		return
	}

	if shouldTellStory(chat.Random, rand.IntN(100)) {
		telegram.Typing(ctx, b, update)
		story, err := app.Gen().Story(ctx, chat)
		if err != nil {
			logger.Instance().Error("cool story", zap.Error(err))
		} else if story != "" {
			telegram.Reply(ctx, b, update, story, "")
			return
		}
	}
	telegram.Reply(ctx, b, update, locale.Random(lang, "cool_story.lazy"), "")
}

func shouldTellStory(random int16, roll int) bool {
	return roll < int(random)+50
}
