package eightball

import (
	"context"
	"crypto/sha1"
	"encoding/binary"

	"shizoid/internal/utils"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	Command     = "eightball"
	Description = "Classic 8ball Yes or No questions"
	HandlerType = bot.HandlerTypeMessageText
	MatchType   = bot.MatchTypeCommand
)

var (
	emptyReplies = []string{
		"А?",
		"И чо?",
		"Не тряси бестолку.",
		"В дудку себе помолчи.",
		"А спросить?",
	}

	replies = []string{
		"Бесспорно.",
		"Предрешено.",
		"Никаких сомнений!",
		"Определённо да.",
		"Можешь быть уверен в этом.",
		"Мне кажется — «да»",
		"Вероятнее всего.",
		"Хорошие перспективы.",
		"Знаки говорят — «да».",
		"Да.",
		"Пока не ясно.",
		"Cпроси завтра.",
		"Лучше не рассказывать.",
		"Сегодня нельзя предсказать.",
		"Сконцентрируйся и спроси опять.",
		"Даже не думай.",
		"Мой ответ — «нет».",
		"По моим данным — «нет».",
		"Перспективы не очень хорошие.",
		"Весьма сомнительно.",
	}
)

func Handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, messageParams(update))
}

func messageParams(update *models.Update) *bot.SendMessageParams {
	payload := utils.ExtractCommandPayloadText(update)
	return &bot.SendMessageParams{
		ChatID:          update.Message.Chat.ID,
		MessageThreadID: update.Message.MessageThreadID,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		Text: response(payload, update.Message.From.ID),
	}
}

func response(payload string, userId int64) string {
	if payload == "" {
		return utils.PickRandomString(emptyReplies)
	}
	digestResult := digest(payload, userId, time.Now())
	return replies[digestResult%uint64(len(replies))]
}

func digest(text string, userID int64, now time.Time) uint64 {
	sum := sha1.Sum([]byte(text))
	numeric := binary.BigEndian.Uint64(sum[:8])
	midnight := now.Truncate(24 * time.Hour)
	return numeric - uint64(userID) - uint64(midnight.Unix()/100)
}
