package handlers

import (
	"context"
	"math/rand/v2"
	"strings"

	"github.com/go-telegram/bot"
	tgmodels "github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/handlers/binary_dice"
	"shizoid/internal/models"
	"shizoid/internal/handlers/captcha"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/telegram"
)

// DefaultHandler handles every update not matched by a command handler: new
// member joins (captcha/greeting) and free-form text the bot may respond to.
func DefaultHandler(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	if len(msg.NewChatMembers) > 0 {
		handleNewMembers(ctx, b, update)
		return
	}

	if msg.Text == "" || isBotCommand(msg) {
		return
	}
	if !app.Ready() || !app.Enabled(ctx) {
		return
	}
	if binary_dice.Respond(ctx, b, update) {
		return
	}
	respond := shouldRespond(ctx, msg)
	logger.Instance().Debug("should respond",
		zap.Int64("chat_id", msg.Chat.ID),
		zap.Bool("respond", respond),
	)
	if !respond {
		return
	}

	chat := app.ChatFrom(ctx)
	if chat == nil {
		return
	}
	logger.Instance().Debug("generate reply",
		zap.Int64("chat_id", chat.ID),
		zap.String("mode", chat.GenerationMode.String()),
	)
	telegram.Typing(ctx, b, update)
	text, err := app.Gen().Reply(ctx, chat, strings.Fields(msg.Text), msg.From.ID)
	if err != nil {
		logger.Instance().Error("generate reply", zap.Error(err))
		return
	}
	if text != "" {
		telegram.Reply(ctx, b, update, text, "")
	}
}

func shouldRespond(ctx context.Context, msg *tgmodels.Message) bool {
	chat := app.ChatFrom(ctx)
	if chat == nil {
		return false
	}
	if msg.Chat.Type == tgmodels.ChatTypePrivate {
		return true
	}
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil && msg.ReplyToMessage.From.ID == app.BotID() {
		return true
	}
	if hasAnchor(ctx, msg.Text) {
		return true
	}
	return rand.IntN(100) < int(chat.Random)
}

func hasAnchor(ctx context.Context, text string) bool {
	anchors := locale.List(app.Locale(ctx), "text.anchors")
	if len(anchors) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(anchors))
	for _, a := range anchors {
		set[strings.ToLower(a)] = struct{}{}
	}
	for _, w := range strings.Fields(strings.ToLower(text)) {
		if _, ok := set[w]; ok {
			return true
		}
	}
	return false
}

func handleNewMembers(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if !app.Enabled(ctx) {
		return
	}
	chat := app.ChatFrom(ctx)
	if chat == nil {
		return
	}
	if chat.CaptchaEnabled() {
		captcha.OnNewMembers(ctx, b, update)
	}
	if chat.Greeting {
		sendGreeting(ctx, b, update)
	}
}

func sendGreeting(ctx context.Context, b *bot.Bot, update *tgmodels.Update) {
	if !app.Ready() {
		return
	}
	text, ok, err := models.Greetings.Get(ctx, update.Message.Chat.ID)
	if err != nil || !ok || text == "" {
		return
	}
	telegram.Reply(ctx, b, update, text, "")
}
