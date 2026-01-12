package config

import (
	"log"

	"github.com/ilyakaznacheev/cleanenv"
)

// TODO
// "github.com/getsentry/sentry-go"
// SENTRY_DSN, SENTRY_RELEASE and SENTRY_ENVIRONMENT
// SentryDSN  string  `env:"SENTRY_DSN"`

type database_config struct {
	Host     string `env:"DATABASE_HOST" env-default:"database"`
	Port     string `env:"DATABASE_PORT" env-default:"5432"`
	Name     string `env:"DATABASE_NAME" env-default:"shizoid"`
	User     string `env:"DATABASE_USER" env-default:"shizoid"`
	Password string `env:"DATABASE_PASSWORD" env-default:"passw07d"`
}

type telegram_config struct {
	Token      string `env:"TELEGRAM_TOKEN" env-default:""`
	WebhookUrl string `env:"WEBHOOK_URL"`
}

type config struct {
	AllowToAll  bool    `env:"ALLOW_TO_ALL" env-default:"false"`
	ContextSize int64   `env:"CONTEXT_SIZE" env-default:"50"`
	BotOwners   []int64 `env:"BOT_OWNERS" env-separator:","`
	BindTo      int16   `env:"BIND_TO" env-default:"3000"`
}

var (
	Database    database_config
	Environment config
	Telegram    telegram_config
)

func init() {
	err := cleanenv.ReadEnv(&Database)
	if err != nil {
		panic(err)
	}

	err = cleanenv.ReadEnv(&Telegram)
	if err != nil {
		panic(err)
	}

	err = cleanenv.ReadEnv(&Environment)
	if err != nil {
		panic(err)
	}

	log.Println("Postgres:")
	log.Println("  Address:", Database.Host)
	log.Println("     Port:", Database.Port)
	log.Println(" Database:", Database.Name)
	log.Println("     User:", Database.User)

	if Telegram.PollMode() {
		log.Println("  Webhook: Not Set")
	} else {
		log.Println("  Webhook:", Telegram.WebhookUrl)
		log.Println("  Bind to:", Environment.BindTo)
	}

	log.Println("Owners:")
	for _, owner := range Environment.BotOwners {
		log.Println("  ", owner)
	}
	log.Println("Context:", Environment.ContextSize)
}

func (l *telegram_config) PollMode() bool {
	return l.WebhookUrl == ""
}
