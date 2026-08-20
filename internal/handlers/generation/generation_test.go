package generation

import (
	"testing"

	tgmodels "github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"

	"shizoid/internal/models"
)

func TestNormalizedPayload(t *testing.T) {
	update := &tgmodels.Update{
		Message: &tgmodels.Message{Text: "/generation  Classic "},
	}
	assert.Equal(t, "classic", normalizedPayload(update))
}

// normalizedPayload feeds models.ParseGenerationMode directly, so the payload
// forms the handler accepts are pinned here.
func TestNormalizedPayloadFeedsParseGenerationMode(t *testing.T) {
	mode, ok := models.ParseGenerationMode(normalizedPayload(
		&tgmodels.Update{Message: &tgmodels.Message{Text: "/generation  Neural "}}))
	assert.True(t, ok)
	assert.Equal(t, models.GenerationModeNeural, mode)

	_, ok = models.ParseGenerationMode(normalizedPayload(
		&tgmodels.Update{Message: &tgmodels.Message{Text: "/generation magic"}}))
	assert.False(t, ok)
}

func TestModeList(t *testing.T) {
	assert.Equal(t, "classic, simplified, neural", modeList())
}
