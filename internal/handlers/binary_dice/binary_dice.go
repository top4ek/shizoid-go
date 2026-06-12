// Package binary_dice answers either/or questions with a random verdict.
package binary_dice

import (
	"context"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"shizoid/internal/app"
	"shizoid/internal/locale"
	"shizoid/internal/telegram"
)

func Respond(ctx context.Context, b *bot.Bot, update *models.Update) bool {
	msg := update.Message
	chat := app.ChatFrom(ctx)
	if msg == nil || chat == nil || msg.Text == "" {
		return false
	}
	if !triggers(ctx, msg.Text) {
		return false
	}
	answer := locale.Random(app.Locale(ctx), "binary_dice.answers")
	if answer == "" {
		return false
	}
	telegram.Reply(ctx, b, update, answer, "")
	return true
}

func triggers(ctx context.Context, text string) bool {
	trimmed := strings.TrimSpace(text)
	if !strings.HasSuffix(trimmed, "?") {
		return false
	}
	words := strings.Fields(trimmed)
	if len(words) <= 3 {
		return false
	}
	anchors := locale.List(app.Locale(ctx), "binary_dice.anchors")
	set := make(map[string]struct{}, len(anchors))
	for _, a := range anchors {
		set[strings.ToLower(a)] = struct{}{}
	}
	for _, w := range words {
		if _, ok := set[strings.ToLower(strings.Trim(w, "?,.!"))]; ok {
			return true
		}
	}
	return false
}
