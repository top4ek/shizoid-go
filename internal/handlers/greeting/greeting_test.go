package greeting

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"shizoid/internal/telegram"
)

func TestParseGreetingAction(t *testing.T) {
	cases := []struct {
		payload string
		want    greetingAction
	}{
		{"", greetingUsage},
		{"  ", greetingUsage},
		{"disable", greetingClear},
		{"DISABLE", greetingClear},
		{"Welcome!", greetingSet},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, parseGreetingAction(c.payload), c.payload)
	}
}

func TestValidateGreetingText(t *testing.T) {
	assert.NoError(t, telegram.ValidateV2("*Welcome*"))
	assert.Error(t, telegram.ValidateV2("Welcome (everyone)."))
}
