package utils

import (
	"errors"
	"math/rand/v2"
	"slices"
	"strings"

	"shizoid/internal/config"

	"github.com/go-telegram/bot/models"
)

func ExtractCommandPayloadText(update *models.Update) string {
	array := strings.SplitN(update.Message.Text, " ", 2)
	if len(array) == 2 {
		return array[1]
	} else {
		return ""
	}
}

func IsBotOwner(update *models.Update) bool {
	return slices.Contains(config.Environment.BotOwners, update.Message.From.ID)
}

func UserName(user *models.User) (string, error) {
	if user.Username != "" {
		return user.Username, nil
	}
	if user.FirstName != "" {
		return user.FirstName, nil
	}
	if user.LastName != "" {
		return user.LastName, nil
	}
	return "Unknown", errors.New("Unable to find username")
}

func PickRandomString(str []string) string {
	return str[rand.IntN(len(str))]
}
