package handlers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasAnchor(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"bare anchor", "шиза", true},
		{"punctuation attached", "привет, шиза!", true},
		{"inflected form", "шизика видел вчера", true},
		{"uppercase", "ШИЗА, ответь", true},
		{"mixed case", "Шизик", true},
		{"no anchor", "погода сегодня хорошая", false},
		{"empty text", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, hasAnchor(context.Background(), c.text))
		})
	}
}
