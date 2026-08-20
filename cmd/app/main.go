package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-telegram/bot"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for goose migrations
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"shizoid/internal/app"
	"shizoid/internal/config"
	"shizoid/internal/handlers"
	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/migrations"
	"shizoid/internal/models"
	"shizoid/internal/scheduler"
	"shizoid/internal/sentry"
	"shizoid/internal/telegram"
	"shizoid/internal/utils"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		panic(err)
	}
}

// runMigrations applies goose migrations over a short-lived database/sql
// connection (goose requires *sql.DB; the app itself runs on pgxpool).
func runMigrations(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	return migrations.Run(db)
}

// run wires the process together: parse flags, bring up config, storage and the
// bot, then serve until the signal context is canceled.
func run(args []string) error {
	fs := flag.NewFlagSet("shizoid", flag.ContinueOnError)
	configPath := fs.String("config", "config.yaml", "path to config file")
	migrateOnly := fs.Bool("migrate-only", false, "run database migrations and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}

	pool, err := bootstrap(*configPath, *migrateOnly)
	if err != nil || pool == nil {
		return err // nil pool with nil err means --migrate-only finished
	}
	defer pool.Close()

	sentry.Init()
	defer sentry.Flush()

	app.Init(pool)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	botInstance := newBot(ctx)

	handlers.RegisterHandlers(ctx, botInstance)

	sched := scheduler.Start(botInstance)
	defer stopScheduler(sched)

	serve(ctx, botInstance)
	return nil
}

// bootstrap loads config, logging and locales, then applies migrations and opens
// the pool. A nil pool with a nil error means --migrate-only did its job.
func bootstrap(configPath string, migrateOnly bool) (*pgxpool.Pool, error) {
	if err := config.Load(configPath); err != nil {
		return nil, err
	}
	logger.Init(config.Development(), config.LogLevel())

	if err := locale.Load(); err != nil {
		logger.Instance().Fatal("locales", zap.Error(err))
	}

	dsn := models.DSN(
		config.Database.Host,
		config.Database.Port,
		config.Database.User,
		config.Database.Password,
		config.Database.Name,
	)
	if err := runMigrations(dsn); err != nil {
		logger.Instance().Fatal("migrations", zap.Error(err))
	}
	if migrateOnly {
		logger.Instance().Info("migrations applied")
		return nil, nil
	}

	pool, err := models.OpenPool(context.Background(), dsn)
	if err != nil {
		logger.Instance().Fatal("database connection", zap.Error(err))
	}
	return pool, nil
}

// newBot constructs the bot and resolves its own Telegram identity, which the
// generator needs to tell its past replies apart from user messages. Every
// failure here is fatal: without a usable bot there is nothing to serve.
func newBot(ctx context.Context) *bot.Bot {
	if err := telegram.EnsureWebhookSecret(); err != nil {
		logger.Instance().Fatal("webhook secret token", zap.Error(err))
	}

	options := []bot.Option{
		bot.WithDefaultHandler(handlers.DefaultHandler),
		bot.WithMiddlewares(sentry.Recover, handlers.LogUpdate, handlers.Ingest),
		bot.WithAllowedUpdates(telegram.AllowedUpdates()),
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

	if err := telegram.ConfigureDelivery(ctx, botInstance); err != nil {
		logger.Instance().Fatal("telegram delivery mode", zap.Error(err))
	}
	return botInstance
}

// serve blocks until ctx is canceled, receiving updates by long polling or over
// the webhook HTTP server depending on the configured delivery mode.
func serve(ctx context.Context, botInstance *bot.Bot) {
	if config.Telegram.PollMode() {
		botInstance.Start(ctx)
		return
	}

	addr := fmt.Sprintf(":%d", config.Environment.BindTo)
	server := &http.Server{
		Addr:              addr,
		Handler:           utils.HTTPWithPing(botInstance.WebhookHandler()),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	logger.Instance().Info("webhook server listening",
		zap.String("addr", addr),
		zap.String("health", "/ping"),
	)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Instance().Fatal("webhook server", zap.Error(err))
		}
	}()

	botInstance.StartWebhook(ctx)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Instance().Warn("webhook server shutdown", zap.Error(err))
	} else {
		logger.Instance().Info("webhook server stopped")
	}
}

// stopScheduler lets in-flight cron jobs and background stat writes finish so a
// restart does not lose learn/append/score writes.
func stopScheduler(sched *cron.Cron) {
	stopCtx := sched.Stop()
	select {
	case <-stopCtx.Done():
	case <-time.After(30 * time.Second):
		logger.Instance().Warn("scheduler stop timed out, jobs still running")
	}
	handlers.WaitCollectStats(30 * time.Second)
}
