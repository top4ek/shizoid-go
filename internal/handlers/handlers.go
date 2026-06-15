package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/logger"
	"shizoid/internal/utils"

	"shizoid/internal/handlers/captcha"
	"shizoid/internal/handlers/eightball"
	"shizoid/internal/handlers/gab"
	"shizoid/internal/handlers/generation"
	"shizoid/internal/handlers/greeting"
	"shizoid/internal/handlers/idle"
	"shizoid/internal/handlers/ids"
	"shizoid/internal/handlers/lang"
	"shizoid/internal/handlers/me"
	"shizoid/internal/handlers/ping"
	"shizoid/internal/handlers/prompt"
	"shizoid/internal/handlers/say"
	"shizoid/internal/handlers/start"
	"shizoid/internal/handlers/status"
	"shizoid/internal/handlers/stop"
	"shizoid/internal/handlers/winner"
)

type command struct {
	name        string
	description string
	handlerType bot.HandlerType
	matchType   bot.MatchType
	handler     bot.HandlerFunc
}

func commands() []command {
	return []command{
		{eightball.Command, eightball.Description, eightball.HandlerType, eightball.MatchType, eightball.Handler},
		{gab.Command, gab.Description, gab.HandlerType, gab.MatchType, gab.Handler},
		{generation.Command, generation.Description, generation.HandlerType, generation.MatchType, generation.Handler},
		{greeting.Command, greeting.Description, greeting.HandlerType, greeting.MatchType, greeting.Handler},
		{idle.Command, idle.Description, idle.HandlerType, idle.MatchType, idle.Handler},
		{ids.Command, ids.Description, ids.HandlerType, ids.MatchType, ids.Handler},
		{lang.Command, lang.Description, lang.HandlerType, lang.MatchType, lang.Handler},
		{me.Command, me.Description, me.HandlerType, me.MatchType, me.Handler},
		{ping.Command, ping.Description, ping.HandlerType, ping.MatchType, ping.Handler},
		{prompt.Command, prompt.Description, prompt.HandlerType, prompt.MatchType, prompt.Handler},
		{say.Command, say.Description, say.HandlerType, say.MatchType, say.Handler},
		{start.Command, start.Description, start.HandlerType, start.MatchType, start.Handler},
		{status.Command, status.Description, status.HandlerType, status.MatchType, status.Handler},
		{stop.Command, stop.Description, stop.HandlerType, stop.MatchType, stop.Handler},
		{captcha.Command, captcha.Description, captcha.HandlerType, captcha.MatchType, captcha.Handler},
		{winner.Command, winner.Description, winner.HandlerType, winner.MatchType, winner.Handler},
	}
}

// RegisterHandlers wires command handlers, the captcha callback, and publishes
// the bot command list to Telegram.
func RegisterHandlers(ctx context.Context, b *bot.Bot) {
	if _, err := b.DeleteMyCommands(ctx, &bot.DeleteMyCommandsParams{Scope: &models.BotCommandScopeDefault{}}); err != nil {
		logger.Instance().Warn("delete my commands", zap.Error(err))
	}

	cmds := commands()
	botCommands := make([]models.BotCommand, 0, len(cmds))
	for _, c := range cmds {
		name := c.name
		handler := c.handler
		b.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			if update.Message == nil {
				return false
			}
			return utils.MatchesLeadingCommand(update.Message.Text, name, app.BotUsername())
		}, handler)
		botCommands = append(botCommands, models.BotCommand{Command: c.name, Description: c.description})
	}

	b.RegisterHandler(captcha.CallbackType, captcha.CallbackPrefix, captcha.CallbackMatch, captcha.Callback)

	b.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		return update.ChatMember != nil
	}, ChatMemberHandler)

	if _, err := b.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: botCommands,
		Scope:    &models.BotCommandScopeDefault{},
	}); err != nil {
		logger.Instance().Warn("set my commands", zap.Error(err))
	}
}
