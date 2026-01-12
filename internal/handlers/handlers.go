package handlers

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"shizoid/internal/handlers/eightball"
	"shizoid/internal/handlers/gab"
	"shizoid/internal/handlers/ids"
	"shizoid/internal/handlers/me"
	"shizoid/internal/handlers/ping"
	"shizoid/internal/handlers/say"
	"shizoid/internal/handlers/start"
	"shizoid/internal/handlers/status"
	"shizoid/internal/handlers/stop"
	"shizoid/internal/handlers/winner"
	"shizoid/internal/logger"
)

func RegisterHandlers(ctx context.Context, b *bot.Bot) {
	b.DeleteMyCommands(ctx, &bot.DeleteMyCommandsParams{
		Scope: &models.BotCommandScopeDefault{},
	})

	b.RegisterHandler(eightball.HandlerType, eightball.Command, eightball.MatchType, eightball.Handler)
	b.RegisterHandler(gab.HandlerType, gab.Command, gab.MatchType, gab.Handler)
	b.RegisterHandler(ids.HandlerType, ids.Command, ids.MatchType, ids.Handler)
	b.RegisterHandler(me.HandlerType, me.Command, me.MatchType, me.Handler)
	b.RegisterHandler(ping.HandlerType, ping.Command, ping.MatchType, ping.Handler)
	b.RegisterHandler(say.HandlerType, say.Command, say.MatchType, say.Handler)
	b.RegisterHandler(start.HandlerType, start.Command, start.MatchType, start.Handler)
	b.RegisterHandler(status.HandlerType, status.Command, status.MatchType, status.Handler)
	b.RegisterHandler(stop.HandlerType, stop.Command, stop.MatchType, stop.Handler)
	b.RegisterHandler(winner.HandlerType, winner.Command, winner.MatchType, winner.Handler)

	commands := []models.BotCommand{
		{Command: eightball.Command, Description: eightball.Description},
		{Command: gab.Command, Description: gab.Description},
		{Command: ids.Command, Description: ids.Description},
		{Command: me.Command, Description: me.Description},
		{Command: ping.Command, Description: ping.Description},
		{Command: say.Command, Description: say.Description},
		{Command: start.Command, Description: start.Description},
		{Command: status.Command, Description: status.Description},
		{Command: stop.Command, Description: stop.Description},
		{Command: winner.Command, Description: winner.Description},
	}

	b.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: commands,
		Scope:    &models.BotCommandScopeDefault{},
	})
}

func DefaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	logger.Instance().Info(update.Message.Text)
}
