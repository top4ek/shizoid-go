package utils

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"shizoid/internal/config"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// ParseLeadingCommand extracts the command name and optional @mention from a
// message that starts with /.
func ParseLeadingCommand(text string) (command, mention string, ok bool) {
	if text == "" || text[0] != '/' {
		return "", "", false
	}
	end := strings.IndexByte(text, ' ')
	if end == -1 {
		end = len(text)
	}
	token := text[1:end]
	command, mention, _ = strings.Cut(token, "@")
	return command, mention, true
}

// MatchesLeadingCommand reports whether text is /name or /name@botUsername.
// Command and @mention are compared case-insensitively.
func MatchesLeadingCommand(text, name, botUsername string) bool {
	cmd, mention, ok := ParseLeadingCommand(text)
	if !ok || !strings.EqualFold(cmd, name) {
		return false
	}
	if mention != "" && botUsername != "" && !strings.EqualFold(mention, botUsername) {
		return false
	}
	return true
}

func ExtractCommandPayloadText(update *models.Update) string {
	if update == nil || update.Message == nil {
		return ""
	}
	array := strings.SplitN(update.Message.Text, " ", 2)
	if len(array) == 2 {
		return array[1]
	}
	return ""
}

func IsOwner(userID int64) bool {
	return slices.Contains(config.Environment.BotOwners, userID)
}

func IsBotOwner(update *models.Update) bool {
	if update == nil || update.Message == nil || update.Message.From == nil {
		return false
	}
	return IsOwner(update.Message.From.ID)
}

// IsChatAdmin reports whether the user is an administrator (or creator) of the
// chat, or a bot owner. In private chats the sole user is treated as admin.
func IsChatAdmin(ctx context.Context, b *bot.Bot, chatID, userID int64) bool {
	if IsOwner(userID) {
		return true
	}
	if chatID > 0 && chatID == userID {
		return true
	}
	admins, err := b.GetChatAdministrators(ctx, &bot.GetChatAdministratorsParams{ChatID: chatID})
	if err != nil {
		return false
	}
	for _, a := range admins {
		if id := adminUserID(a); id == userID {
			return true
		}
	}
	return false
}

func adminUserID(member models.ChatMember) int64 {
	switch member.Type {
	case models.ChatMemberTypeOwner:
		if member.Owner != nil && member.Owner.User != nil {
			return member.Owner.User.ID
		}
	case models.ChatMemberTypeAdministrator:
		if member.Administrator != nil {
			return member.Administrator.User.ID
		}
	}
	return 0
}

func UserName(user *models.User) (string, error) {
	if user == nil {
		return "Unknown", errors.New("user is nil")
	}
	name := DisplayName(user.Username, user.FirstName, user.LastName)
	if name == "Unknown" {
		return name, errors.New("unable to find username")
	}
	return name, nil
}

// DisplayName picks the best available label for a Telegram user.
func DisplayName(username, firstName, lastName string) string {
	if username != "" {
		return "@" + username
	}
	if firstName != "" {
		return firstName
	}
	if lastName != "" {
		return lastName
	}
	return "Unknown"
}

func PickRandomString(str []string) string {
	if len(str) == 0 {
		return ""
	}
	return str[rand.IntN(len(str))]
}

// UserMarkdownLink renders a Telegram MarkdownV2 inline link to a user.
func UserMarkdownLink(userID int64, username, label string) string {
	if label == "" {
		label = "Unknown"
	}
	escaped := bot.EscapeMarkdown(label)
	if username != "" {
		return fmt.Sprintf("[%s](https://t.me/%s)", escaped, username)
	}
	if userID != 0 {
		return fmt.Sprintf("[%s](tg://user?id=%d)", escaped, userID)
	}
	return escaped
}
