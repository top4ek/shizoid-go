package models

import "strings"

type GenerationMode int16

const (
	GenerationModeClassic GenerationMode = iota
	GenerationModeSimplified
	GenerationModeNeural
)

var generationModeNames = [...]string{
	GenerationModeClassic:    "classic",
	GenerationModeSimplified: "simplified",
	GenerationModeNeural:     "neural",
}

func (m GenerationMode) String() string {
	if int(m) < 0 || int(m) >= len(generationModeNames) {
		return "unknown"
	}
	return generationModeNames[m]
}

func ParseGenerationMode(s string) (GenerationMode, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	for i, name := range generationModeNames {
		if s == name {
			return GenerationMode(i), true
		}
	}
	return 0, false
}

func ValidGenerationMode(m GenerationMode) bool {
	return m == GenerationModeClassic || m == GenerationModeSimplified || m == GenerationModeNeural
}

func GenerationModes() []GenerationMode {
	return []GenerationMode{
		GenerationModeClassic,
		GenerationModeSimplified,
		GenerationModeNeural,
	}
}
