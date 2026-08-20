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
	"shizoid/internal/handlers/ids"
	"shizoid/internal/handlers/lang"
	"shizoid/internal/handlers/me"
	"shizoid/internal/handlers/news"
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
	handler     bot.HandlerFunc

	role         role
	replyOnDeny  bool // reply with the localized denial instead of staying silent
	needsEnabled bool // requires the bot enabled in this chat (app.Enabled)
}

func commands() []command {
	return buildCommands(app.Neural().SummaryConfigured())
}

// buildCommands lists the registered commands. /news is included only when a
// neural summary chain is configured, since that is the chain it generates
// issues over; without it the command would sit in the menu doing nothing.
func buildCommands(newsEnabled bool) []command {
	cmds := []command{
		{name: eightball.Command, description: eightball.Description, handler: eightball.Handler},
		{name: gab.Command, description: gab.Description, handler: gab.Handler,
			role: roleAdmin, replyOnDeny: true, needsEnabled: true},
		{name: generation.Command, description: generation.Description, handler: generation.Handler,
			needsEnabled: true},
		{name: greeting.Command, description: greeting.Description, handler: greeting.Handler,
			role: roleAdmin, replyOnDeny: true, needsEnabled: true},
		{name: ids.Command, description: ids.Description, handler: ids.Handler},
		{name: lang.Command, description: lang.Description, handler: lang.Handler,
			needsEnabled: true},
		{name: me.Command, description: me.Description, handler: me.Handler},
		{name: ping.Command, description: ping.Description, handler: ping.Handler},
		{name: prompt.Command, description: prompt.Description, handler: prompt.Handler,
			role: roleOwner, replyOnDeny: true, needsEnabled: true},
		{name: say.Command, description: say.Description, handler: say.Handler,
			role: roleOwner},
		{name: start.Command, description: start.Description, handler: start.Handler,
			role: roleOwner},
		{name: status.Command, description: status.Description, handler: status.Handler,
			needsEnabled: true},
		{name: stop.Command, description: stop.Description, handler: stop.Handler,
			role: roleOwner},
		{name: captcha.Command, description: captcha.Description, handler: captcha.Handler,
			role: roleAdmin, replyOnDeny: true, needsEnabled: true},
		{name: winner.Command, description: winner.Description, handler: winner.Handler,
			needsEnabled: true},
	}
	if newsEnabled {
		cmds = append(cmds, command{name: news.Command, description: news.Description, handler: news.Handler,
			needsEnabled: true})
	}
	return cmds
}

// gate enforces the command's declared requirements once, so handlers do not
// repeat nil/enabled/permission prologues.
func gate(c command) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		msg := update.Message
		if msg == nil || msg.From == nil {
			return
		}
		if c.needsEnabled && !app.Enabled(ctx) {
			return
		}
		switch c.role {
		case roleAdmin:
			if !utils.IsChatAdmin(ctx, b, msg.Chat.ID, msg.From.ID) {
				if c.replyOnDeny {
					telegram.Reply(ctx, b, update, locale.Random(app.Locale(ctx), "nok"))
				}
				return
			}
		case roleOwner:
			if !app.IsOwner(msg.From.ID) {
				if c.replyOnDeny {
					telegram.Reply(ctx, b, update, locale.Random(app.Locale(ctx), "nok"))
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
