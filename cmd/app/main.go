package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/go-telegram/bot"

	"shizoid/internal/config"
	"shizoid/internal/handlers"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	options := []bot.Option{
		bot.WithWebhookSecretToken(config.Telegram.WebhookUrl),
		bot.WithDefaultHandler(handlers.DefaultHandler),
		// bot.WithDebug(),
		// bot.UseTestEnvironment(),
	}

	bot_instance, err := bot.New(config.Telegram.Token, options...)
	if err != nil {
		panic(err)
	}

	handlers.RegisterHandlers(ctx, bot_instance)

	if config.Telegram.PollMode() {
		bot_instance.Start(ctx)
	} else {
		go func() {
			port := fmt.Sprintf(":%d", config.Environment.BindTo)
			http.ListenAndServe(port, bot_instance.WebhookHandler())
		}()
		bot_instance.StartWebhook(ctx)
	}
}
