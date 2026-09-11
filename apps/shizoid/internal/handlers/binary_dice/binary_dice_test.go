package binary_dice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTriggers(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name string
		text string
		want bool
	}{
		{"anchor and question", "пить чай или кофе сейчас?", true},
		{"no question mark", "пить чай или кофе сейчас", false},
		{"too few words", "чай или кофе?", false},
		{"no anchor", "как дела у тебя сегодня?", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, triggers(ctx, c.text))
		})
	}
}
