package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerationModeString(t *testing.T) {
	assert.Equal(t, "classic", GenerationModeClassic.String())
	assert.Equal(t, "simplified", GenerationModeSimplified.String())
	assert.Equal(t, "neural", GenerationModeNeural.String())
	assert.Equal(t, "unknown", GenerationMode(99).String())
}

func TestParseGenerationMode(t *testing.T) {
	mode, ok := ParseGenerationMode("classic")
	assert.True(t, ok)
	assert.Equal(t, GenerationModeClassic, mode)

	mode, ok = ParseGenerationMode("  SIMPLIFIED ")
	assert.True(t, ok)
	assert.Equal(t, GenerationModeSimplified, mode)

	mode, ok = ParseGenerationMode("neural")
	assert.True(t, ok)
	assert.Equal(t, GenerationModeNeural, mode)

	_, ok = ParseGenerationMode("bogus")
	assert.False(t, ok)
}

func TestValidGenerationMode(t *testing.T) {
	assert.True(t, ValidGenerationMode(GenerationModeClassic))
	assert.True(t, ValidGenerationMode(GenerationModeSimplified))
	assert.True(t, ValidGenerationMode(GenerationModeNeural))
	assert.False(t, ValidGenerationMode(GenerationMode(3)))
	assert.False(t, ValidGenerationMode(GenerationMode(99)))
}

func TestGenerationModes(t *testing.T) {
	assert.Equal(t, []GenerationMode{
		GenerationModeClassic,
		GenerationModeSimplified,
		GenerationModeNeural,
	}, GenerationModes())
}
