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

func TestClassifyGenerationAction(t *testing.T) {
	action, mode := classifyGenerationAction("")
	assert.Equal(t, generationShow, action)

	action, mode = classifyGenerationAction("magic")
	assert.Equal(t, generationUnknown, action)
	assert.Equal(t, models.GenerationMode(0), mode)

	action, mode = classifyGenerationAction("neural")
	assert.Equal(t, generationSet, action)
	assert.Equal(t, models.GenerationModeNeural, mode)
}

func TestModeList(t *testing.T) {
	assert.Equal(t, "classic, simplified, neural", modeList())
}
