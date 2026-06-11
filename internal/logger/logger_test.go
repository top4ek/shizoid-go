package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstanceProduction(t *testing.T) {
	Init(false, "info")
	logger := Instance()
	assert.NotNil(t, logger)
}

func TestInstanceDevelopment(t *testing.T) {
	Init(true, "debug")
	logger := Instance()
	assert.NotNil(t, logger)
}
