package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-telegram/bot"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/config"
	"shizoid/internal/handlers"
	"shizoid/internal/logger"
	"shizoid/internal/migrations"
	"shizoid/internal/models"
	"shizoid/internal/scheduler"
	"shizoid/internal/sentry"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	if err := config.Load(*configPath); err != nil {
		panic(err)
	}
	logger.Init(config.Development(), config.LogLevel())
	sentry.Init()
	defer sentry.Flush()

	db, err := models.OpenDB(
		config.Database.Host,
		config.Database.Port,
		config.Database.User,
		config.Database.Password,
		config.Database.Name,
	)
	if err != nil {
		logger.Instance().Fatal("database connection", zap.Error(err))
	}
	defer db.Close()

	if err := migrations.Run(db); err != nil {
		logger.Instance().Fatal("migrations", zap.Error(err))
	}
	app.Init(db)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	options := []bot.Option{
		bot.WithDefaultHandler(handlers.DefaultHandler),
		bot.WithMiddlewares(sentry.Recover, handlers.LogUpdate, handlers.Ingest),
		bot.WithSkipGetMe(), // verified below; avoids 5s init timeout on slow Telegram API during hot reload
	}
	if config.Telegram.WebhookSecretToken != "" {
		options = append(options, bot.WithWebhookSecretToken(config.Telegram.WebhookSecretToken))
	}

	botInstance, err := bot.New(config.Telegram.Token, options...)
	if err != nil {
		logger.Instance().Fatal("telegram bot", zap.Error(err))
	}

	if me, err := botInstance.GetMe(ctx); err == nil {
		app.SetBotID(me.ID)
		app.SetBotUsername(me.Username)
	} else {
		logger.Instance().Warn("getMe", zap.Error(err))
		if id := botInstance.ID(); id != 0 {
			app.SetBotID(id)
		}
	}

	handlers.RegisterHandlers(ctx, botInstance)

	sched := scheduler.Start(botInstance)
	defer sched.Stop()

	if config.Telegram.PollMode() {
		botInstance.Start(ctx)
	} else {
		server := &http.Server{
			Addr:              fmt.Sprintf(":%d", config.Environment.BindTo),
			Handler:           botInstance.WebhookHandler(),
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
		}
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Instance().Fatal("webhook server", zap.Error(err))
			}
		}()
		botInstance.StartWebhook(ctx)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Instance().Warn("webhook server shutdown", zap.Error(err))
		}
	}
}
