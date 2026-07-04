package handlers

import (
	"context"
	"slices"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/config"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/telegram"
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

// role is the unconditional permission a command requires. Commands whose
// permission depends on the payload (view for everyone, set for admins:
// generation, lang, winner) declare roleEveryone and check inside.
type role int

const (
	roleEveryone role = iota
	roleAdmin         // chat admin (or bot owner)
	roleOwner         // bot owner
)

type command struct {
	name        string
	description string
	handlerType bot.HandlerType
	matchType   bot.MatchType
	handler     bot.HandlerFunc

	role         role
	replyOnDeny  bool // reply with the localized denial instead of staying silent
	needsReady   bool // requires the data layer (app.Ready)
	needsEnabled bool // requires the bot enabled in this chat (app.Enabled)
}

func commands() []command {
	return []command{
		{name: eightball.Command, description: eightball.Description, handlerType: eightball.HandlerType, matchType: eightball.MatchType, handler: eightball.Handler},
		{name: gab.Command, description: gab.Description, handlerType: gab.HandlerType, matchType: gab.MatchType, handler: gab.Handler,
			role: roleAdmin, replyOnDeny: true, needsReady: true, needsEnabled: true},
		{name: generation.Command, description: generation.Description, handlerType: generation.HandlerType, matchType: generation.MatchType, handler: generation.Handler,
			needsEnabled: true},
		{name: greeting.Command, description: greeting.Description, handlerType: greeting.HandlerType, matchType: greeting.MatchType, handler: greeting.Handler,
			role: roleAdmin, replyOnDeny: true, needsReady: true, needsEnabled: true},
		{name: idle.Command, description: idle.Description, handlerType: idle.HandlerType, matchType: idle.MatchType, handler: idle.Handler,
			role: roleAdmin, replyOnDeny: true, needsReady: true, needsEnabled: true},
		{name: ids.Command, description: ids.Description, handlerType: ids.HandlerType, matchType: ids.MatchType, handler: ids.Handler},
		{name: lang.Command, description: lang.Description, handlerType: lang.HandlerType, matchType: lang.MatchType, handler: lang.Handler,
			needsReady: true, needsEnabled: true},
		{name: me.Command, description: me.Description, handlerType: me.HandlerType, matchType: me.MatchType, handler: me.Handler},
		{name: ping.Command, description: ping.Description, handlerType: ping.HandlerType, matchType: ping.MatchType, handler: ping.Handler},
		{name: prompt.Command, description: prompt.Description, handlerType: prompt.HandlerType, matchType: prompt.MatchType, handler: prompt.Handler,
			role: roleOwner, replyOnDeny: true, needsReady: true, needsEnabled: true},
		{name: say.Command, description: say.Description, handlerType: say.HandlerType, matchType: say.MatchType, handler: say.Handler,
			role: roleOwner},
		{name: start.Command, description: start.Description, handlerType: start.HandlerType, matchType: start.MatchType, handler: start.Handler,
			role: roleOwner, needsReady: true},
		{name: status.Command, description: status.Description, handlerType: status.HandlerType, matchType: status.MatchType, handler: status.Handler,
			needsReady: true, needsEnabled: true},
		{name: stop.Command, description: stop.Description, handlerType: stop.HandlerType, matchType: stop.MatchType, handler: stop.Handler,
			role: roleOwner, needsReady: true},
		{name: captcha.Command, description: captcha.Description, handlerType: captcha.HandlerType, matchType: captcha.MatchType, handler: captcha.Handler,
			role: roleAdmin, replyOnDeny: true, needsReady: true, needsEnabled: true},
		{name: winner.Command, description: winner.Description, handlerType: winner.HandlerType, matchType: winner.MatchType, handler: winner.Handler,
			needsReady: true, needsEnabled: true},
	}
}

// gate enforces the command's declared requirements once, so handlers do not
// repeat nil/ready/enabled/permission prologues.
func gate(c command) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		msg := update.Message
		if msg == nil || msg.From == nil {
			return
		}
		if c.needsReady && !app.Ready() {
			return
		}
		if c.needsEnabled && !app.Enabled(ctx) {
			return
		}
		switch c.role {
		case roleAdmin:
			if !utils.IsChatAdmin(ctx, b, msg.Chat.ID, msg.From.ID) {
				if c.replyOnDeny {
					telegram.Reply(ctx, b, update, locale.T(app.Locale(ctx), "common.not_admin"))
				}
				return
			}
		case roleOwner:
			if !app.IsOwner(msg.From.ID) {
				if c.replyOnDeny {
					telegram.Reply(ctx, b, update, locale.T(app.Locale(ctx), "common.not_owner"))
				}
				return
			}
		}
		c.handler(ctx, b, update)
	}
}

// RegisterHandlers wires command handlers, the captcha callback, and publishes
// the bot command list to Telegram. The command menu is scoped by role: the
// default scope lists only public commands, chat administrators additionally
// see roleAdmin commands, and each configured bot owner sees the full list in
// their private chat with the bot.
func RegisterHandlers(ctx context.Context, b *bot.Bot) {
	if _, err := b.DeleteMyCommands(ctx, &bot.DeleteMyCommandsParams{Scope: &models.BotCommandScopeDefault{}}); err != nil {
		logger.Instance().Warn("delete my commands", zap.Error(err))
	}

	cmds := commands()
	commandsByRole := map[role][]models.BotCommand{}
	for _, c := range cmds {
		name := c.name
		handler := gate(c)
		b.RegisterHandlerMatchFunc(func(update *models.Update) bool {
			if update.Message == nil {
				return false
			}
			return utils.MatchesLeadingCommand(update.Message.Text, name, app.BotUsername())
		}, handler)
		commandsByRole[c.role] = append(commandsByRole[c.role], models.BotCommand{Command: c.name, Description: c.description})
	}

	b.RegisterHandler(captcha.CallbackType, captcha.CallbackPrefix, captcha.CallbackMatch, captcha.Callback)

	b.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		return update.ChatMember != nil
	}, ChatMemberHandler)

	// A more specific scope replaces the list entirely, so each wider role
	// repeats the narrower ones.
	everyoneMenu := commandsByRole[roleEveryone]
	adminMenu := slices.Concat(everyoneMenu, commandsByRole[roleAdmin])
	ownerMenu := slices.Concat(adminMenu, commandsByRole[roleOwner])

	setCommands(ctx, b, &models.BotCommandScopeDefault{}, everyoneMenu)
	setCommands(ctx, b, &models.BotCommandScopeAllChatAdministrators{}, adminMenu)
	for _, ownerID := range config.Environment.BotOwners {
		setCommands(ctx, b, &models.BotCommandScopeChat{ChatID: ownerID}, ownerMenu)
	}
}

func setCommands(ctx context.Context, b *bot.Bot, scope models.BotCommandScope, commands []models.BotCommand) {
	if _, err := b.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: commands,
		Scope:    scope,
	}); err != nil {
		logger.Instance().Warn("set my commands", zap.Error(err))
	}
}
