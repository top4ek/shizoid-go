package telegram

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/go-telegram/bot"
	"go.uber.org/zap"

	"shizoid/internal/config"
	"shizoid/internal/logger"
)

// EnsureWebhookSecret generates a random webhook secret when in webhook mode
// and none is configured. Idempotent.
func EnsureWebhookSecret() error {
	if config.Telegram.PollMode() || config.Telegram.WebhookSecretToken != "" {
		return nil
	}

	secret, err := generateWebhookSecretToken()
	if err != nil {
		return fmt.Errorf("generate webhook secret token: %w", err)
	}
	config.Telegram.WebhookSecretToken = secret
	logger.Instance().Debug("generated webhook secret token", zap.String("webhook_secret_token", secret))
	return nil
}

func generateWebhookSecretToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ConfigureDelivery registers poll or webhook delivery mode with Telegram.
func ConfigureDelivery(ctx context.Context, b *bot.Bot) error {
	if config.Telegram.PollMode() {
		if err := clearWebhookForPoll(ctx, b); err != nil {
			logger.Instance().Warn("telegram delete webhook", zap.Error(err))
		}
		logger.Instance().Info("telegram delivery mode", zap.String("mode", "poll"))
		return nil
	}

	if err := EnsureWebhookSecret(); err != nil {
		return err
	}

	params := &bot.SetWebhookParams{
		URL:            config.Telegram.WebhookUrl,
		SecretToken:    config.Telegram.WebhookSecretToken,
		AllowedUpdates: AllowedUpdates(),
	}
	if _, err := b.SetWebhook(ctx, params); err != nil {
		return err
	}
	logger.Instance().Info("telegram delivery mode",
		zap.String("mode", "webhook"),
		zap.String("url", config.Telegram.WebhookUrl),
	)
	return nil
}

// clearWebhookForPoll removes an active webhook before long polling.
func clearWebhookForPoll(ctx context.Context, b *bot.Bot) error {
	info, err := b.GetWebhookInfo(ctx)
	if err != nil {
		return err
	}
	if info.URL == "" {
		return nil
	}
	_, err = b.DeleteWebhook(ctx, &bot.DeleteWebhookParams{})
	return err
}
