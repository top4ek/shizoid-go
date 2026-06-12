// Package sentry provides optional error reporting. It is a no-op unless
// sentry.dsn is set in the application config.
package sentry

import (
	"context"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"shizoid/internal/config"
	"shizoid/internal/logger"
	"shizoid/internal/version"
)

// Init initializes Sentry if a DSN is configured.
func Init() {
	if !config.SentryEnabled() {
		return
	}
	release := config.Sentry.Release
	if release == "" {
		release = version.Version()
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:         config.Sentry.DSN,
		Environment: config.Sentry.Environment,
		Release:     release,
	}); err != nil {
		logger.Instance().Error("sentry init", zap.Error(err))
	}
}

// Flush waits for buffered events to be sent before shutdown.
func Flush() {
	if config.SentryEnabled() {
		sentry.Flush(2 * time.Second)
	}
}

// Capture reports an error to Sentry when enabled.
func Capture(err error) {
	if err == nil {
		return
	}
	if config.SentryEnabled() {
		sentry.CaptureException(err)
	}
}

// Recover is a middleware that recovers from handler panics and reports them.
func Recover(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		defer func() {
			if r := recover(); r != nil {
				logger.Instance().Error("handler panic", zap.Any("panic", r))
				if config.SentryEnabled() {
					sentry.CurrentHub().Recover(r)
					sentry.Flush(2 * time.Second)
				}
			}
		}()
		next(ctx, b, update)
	}
}
