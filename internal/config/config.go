package config

import (
	"github.com/ilyakaznacheev/cleanenv"

	"shizoid/internal/models"
	"shizoid/internal/neural"
)

type ValidationError struct {
	Field string
	Msg   string
}

type Settings struct {
	AppEnv      string         `yaml:"app_env" env:"APP_ENV" env-default:"production"`
	AppLogLevel string         `yaml:"log_level" env:"LOG_LEVEL"`
	Database    databaseConfig `yaml:"database" env-prefix:"DATABASE_"`
	Telegram    telegramConfig `yaml:"telegram" env-prefix:"TELEGRAM_"`
	Sentry      sentryConfig   `yaml:"sentry" env-prefix:"SENTRY_"`
	App         appConfig      `yaml:"app" env-prefix:"APP_"`
	Neural      neuralConfig   `yaml:"neural"`
}

type databaseConfig struct {
	Host     string `yaml:"host" env:"HOST" env-default:"database"`
	Port     string `yaml:"port" env:"PORT" env-default:"5432"`
	Name     string `yaml:"name" env:"NAME" env-default:"shizoid"`
	User     string `yaml:"user" env:"USER" env-default:"shizoid"`
	Password string `yaml:"password" env:"PASSWORD"`
}

type telegramConfig struct {
	Token              string `yaml:"token"`
	WebhookUrl         string `yaml:"webhook_url"`
	WebhookSecretToken string `yaml:"webhook_secret_token"`
}

type sentryConfig struct {
	DSN         string `yaml:"dsn" env:"DSN"`
	Environment string `yaml:"environment" env:"ENVIRONMENT" env-default:"production"`
	Release     string `yaml:"release" env:"RELEASE"`
}

type runtimeConfig struct {
	AppEnv      string
	AppLogLevel string
}

type appConfig struct {
	AllowToAll     bool    `yaml:"allow_to_all" env:"ALLOW_TO_ALL"`
	BotOwners      []int64 `yaml:"bot_owners" env:"BOT_OWNERS"`
	BindTo         int16   `yaml:"bind_to" env:"BIND_TO" env-default:"3000"`
	Locale         string  `yaml:"locale" env:"LOCALE" env-default:"ru"`
	GenerationMode string  `yaml:"generation_mode" env:"GENERATION_MODE" env-default:"neural"`
	WinnerCron     string  `yaml:"winner_cron" env:"WINNER_CRON" env-default:"20 1 * * *"`
	CaptchaCron    string  `yaml:"captcha_cron" env:"CAPTCHA_CRON" env-default:"@every 1m"`

	MemoryCron string `yaml:"memory_cron" env:"MEMORY_CRON" env-default:"0 */3 * * *"`
	NewsCron   string `yaml:"news_cron" env:"NEWS_CRON" env-default:"40 4 * * *"`

	Prompts promptBlocks `yaml:"prompts" env-prefix:"PROMPT_"`

	AppPrompt     string `yaml:"-"`
	NewsPrompt    string `yaml:"-"`
	SummaryPrompt string `yaml:"-"`
}

type promptBlocks struct {
	ChatRole       string `yaml:"chat_role" env:"CHAT_ROLE"`
	ChatMemory     string `yaml:"chat_memory" env:"CHAT_MEMORY"`
	ChatFormat     string `yaml:"chat_format" env:"CHAT_FORMAT"`
	ChatLength     string `yaml:"chat_length" env:"CHAT_LENGTH"`
	TelegramMarkup string `yaml:"telegram_markup" env:"TELEGRAM_MARKUP"`
	Precedence     string `yaml:"precedence" env:"PRECEDENCE"`
	NewsRole       string `yaml:"news_role" env:"NEWS_ROLE"`
	NewsSource     string `yaml:"news_source" env:"NEWS_SOURCE"`
	NewsFormat     string `yaml:"news_format" env:"NEWS_FORMAT"`
	NewsTone       string `yaml:"news_tone" env:"NEWS_TONE"`
	SummaryRole    string `yaml:"summary_role" env:"SUMMARY_ROLE"`
	SummaryRules   string `yaml:"summary_rules" env:"SUMMARY_RULES"`
	SummaryFormat  string `yaml:"summary_format" env:"SUMMARY_FORMAT"`
}

type neuralConfig struct {
	Reply   []neural.Provider `yaml:"reply"`
	Summary []neural.Provider `yaml:"summary"`
}

var (
	Database               databaseConfig
	Environment            appConfig
	DefaultGenerationMode  models.GenerationMode
	Telegram               telegramConfig
	Sentry                 sentryConfig
	Runtime                runtimeConfig
	Neural                 neuralConfig
	MaxReplyContextBytes   int
	MaxSummaryContextBytes int
)

const defaultReplyContextBytes = 16384

func Load(path string) error {
	var settings Settings
	if err := cleanenv.ReadConfig(path, &settings); err != nil {
		return err
	}
	applyPromptDefaults(&settings.App)

	Database = settings.Database
	Telegram = settings.Telegram
	Environment = settings.App
	Sentry = settings.Sentry
	Runtime = runtimeConfig{
		AppEnv:      settings.AppEnv,
		AppLogLevel: settings.AppLogLevel,
	}
	Neural = settings.Neural
	MaxReplyContextBytes = maxReplyContextBytes(Neural.Reply)
	if MaxReplyContextBytes <= 0 {
		MaxReplyContextBytes = defaultReplyContextBytes
	}
	MaxSummaryContextBytes = maxReplyContextBytes(Neural.Summary)
	if MaxSummaryContextBytes <= 0 {
		MaxSummaryContextBytes = defaultReplyContextBytes
	}

	if mode, ok := models.ParseGenerationMode(Environment.GenerationMode); ok {
		DefaultGenerationMode = mode
	} else {
		DefaultGenerationMode = models.GenerationModeNeural
	}
	return validate()
}

func Development() bool {
	return Runtime.AppEnv == "development" || Runtime.AppEnv == "dev"
}

func LogLevel() string {
	return Runtime.AppLogLevel
}

func validate() error {
	if Telegram.Token == "" {
		return &ValidationError{Field: "telegram.token", Msg: "required"}
	}
	if !Telegram.PollMode() && Telegram.WebhookUrl == "" {
		return &ValidationError{Field: "telegram.webhook_url", Msg: "required when not using poll mode"}
	}
	return nil
}

func (e *ValidationError) Error() string {
	return "config: " + e.Field + ": " + e.Msg
}

func (l *telegramConfig) PollMode() bool {
	return l.WebhookUrl == ""
}

func maxReplyContextBytes(reply []neural.Provider) int {
	max := 0
	for _, p := range reply {
		if p.ContextSize > max {
			max = p.ContextSize
		}
	}
	return max
}

// SentryEnabled reports whether Sentry integration should be initialized.
func SentryEnabled() bool {
	return Sentry.DSN != ""
}
