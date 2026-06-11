package status

import (
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/stretchr/testify/assert"

	"shizoid/internal/locale"
	"shizoid/internal/logger"
	"shizoid/internal/models"
)

func TestMain(m *testing.M) {
	logger.Init(true, "")
	os.Exit(m.Run())
}

func TestStatusText(t *testing.T) {
	activeAt := sql.NullTime{Time: time.Now(), Valid: true}
	captchaAt := sql.NullTime{Time: time.Now(), Valid: true}
	chat := &models.Chat{
		Random:           25,
		ActiveAt:         activeAt,
		GenerationMode:   models.GenerationModeClassic,
		Winner:           sql.NullString{String: "daily", Valid: true},
		CaptchaEnabledAt: captchaAt,
		Greeting:         true,
	}
	got := statusText("ru", chat, 42)
	yes := bot.EscapeMarkdown(locale.T("ru", "yes"))

	assert.Contains(t, got, yes)
	assert.Equal(t, 3, strings.Count(got, yes))
	assert.Contains(t, got, "25")
	assert.Contains(t, got, "42")
	assert.Contains(t, got, "daily")
	assert.Contains(t, got, bot.EscapeMarkdown("ru"))
}

func TestStatusTextInactiveWinnerDisabled(t *testing.T) {
	chat := &models.Chat{Random: 0}
	got := statusText("ru", chat, 0)
	no := bot.EscapeMarkdown(locale.T("ru", "no"))

	assert.Contains(t, got, no)
	assert.Equal(t, 3, strings.Count(got, no))
	assert.Contains(t, got, bot.EscapeMarkdown(locale.T("ru", "winner.disabled")))
}
