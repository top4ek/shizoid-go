package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstance(t *testing.T) {
	logger := Instance()
	assert.NotNil(t, logger)
}
