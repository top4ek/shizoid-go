package eightball

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"shizoid/internal/app"
	"shizoid/internal/locale"
	"shizoid/internal/telegram"
	"shizoid/internal/utils"
)

const (
	Command     = "eightball"
	Description = "Classic 8ball Yes or No questions"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil || !app.Enabled(ctx) {
		return
	}
	payload := utils.ExtractCommandPayloadText(update)
	text := response(app.Locale(ctx), payload, update.Message.From.ID)
	if text != "" {
		telegram.Reply(ctx, b, update, text, "")
	}
}

func response(lang, payload string, userID int64) string {
	if payload == "" {
		empty := locale.List(lang, "eightball.empty")
		if len(empty) == 0 {
			return "?"
		}
		return utils.PickRandomString(empty)
	}
	replies := locale.List(lang, "eightball.replies")
	if len(replies) == 0 {
		return "?"
	}
	d := digest(payload, userID, time.Now())
	return replies[d%uint64(len(replies))]
}

func digest(text string, userID int64, now time.Time) uint64 {
	sum := sha1.Sum([]byte(text))
	numeric := binary.BigEndian.Uint64(sum[:8])
	midnight := now.Truncate(24 * time.Hour)
	return numeric - uint64(userID) - uint64(midnight.Unix()/100)
}
