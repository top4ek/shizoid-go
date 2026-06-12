package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerationModeString(t *testing.T) {
	assert.Equal(t, "classic", GenerationModeClassic.String())
	assert.Equal(t, "simplified", GenerationModeSimplified.String())
	assert.Equal(t, "unknown", GenerationMode(99).String())
}

func TestParseGenerationMode(t *testing.T) {
	mode, ok := ParseGenerationMode("classic")
	assert.True(t, ok)
	assert.Equal(t, GenerationModeClassic, mode)

	mode, ok = ParseGenerationMode("  SIMPLIFIED ")
	assert.True(t, ok)
	assert.Equal(t, GenerationModeSimplified, mode)

	_, ok = ParseGenerationMode("neural")
	assert.False(t, ok)

	_, ok = ParseGenerationMode("bogus")
	assert.False(t, ok)
}

func TestValidGenerationMode(t *testing.T) {
	assert.True(t, ValidGenerationMode(GenerationModeClassic))
	assert.True(t, ValidGenerationMode(GenerationModeSimplified))
	assert.False(t, ValidGenerationMode(GenerationMode(2)))
	assert.False(t, ValidGenerationMode(GenerationMode(99)))
}

func TestGenerationModes(t *testing.T) {
	assert.Equal(t, []GenerationMode{
		GenerationModeClassic,
		GenerationModeSimplified,
	}, GenerationModes())
}
