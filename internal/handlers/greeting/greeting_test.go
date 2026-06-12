package greeting

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
