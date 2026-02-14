package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRateLimitConfig(t *testing.T) {
	config := RateLimitConfig{
		RequestsPerSecond: 100,
		OrdersPerSecond:   10,
		Burst:             50,
	}

	assert.Equal(t, 100, config.RequestsPerSecond)
	assert.Equal(t, 10, config.OrdersPerSecond)
	assert.Equal(t, 50, config.Burst)
}
