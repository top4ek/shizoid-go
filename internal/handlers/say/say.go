package say

import (
	"context"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"shizoid/internal/telegram"
	"shizoid/internal/utils"
)

const (
	Command     = "say"
	Description = "Says something from bot's name"
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	// gate enforces roleOwner, so only the payload is left to check.
	if !canReply(update) {
		return
	}

	telegram.Impersonate(ctx, b, update, text(update), telegram.ChatMessageOpts{})
}

func canReply(update *models.Update) bool {
	return text(update) != ""
}

func text(update *models.Update) string {
	return strings.TrimSpace(utils.ExtractCommandPayloadText(update))
}
