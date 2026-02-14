package transport

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewWSClient(t *testing.T) {
	config := WSConfig{
		URL: "wss://example.com/ws",
	}

	client := NewWSClient(config)

	assert.NotNil(t, client)
	assert.False(t, client.IsConnected())
	assert.Equal(t, 1*time.Second, client.config.ReconnectBaseWait)
	assert.Equal(t, 30*time.Second, client.config.ReconnectMaxWait)
	assert.Equal(t, 10*time.Second, client.config.PingInterval)
	assert.Equal(t, 20*time.Second, client.config.PongWait)
}

func TestWSClient_Subscribe(t *testing.T) {
	client := NewWSClient(WSConfig{URL: "wss://example.com/ws"})

	handler := func(data []byte) error { return nil }

	err := client.Subscribe("test-channel", handler)
	assert.NoError(t, err)

	subs := client.Subscriptions()
	assert.Len(t, subs, 1)
	assert.Contains(t, subs, "test-channel")
}

func TestWSClient_Unsubscribe(t *testing.T) {
	client := NewWSClient(WSConfig{URL: "wss://example.com/ws"})

	handler := func(data []byte) error { return nil }
	_ = client.Subscribe("test-channel", handler)

	err := client.Unsubscribe("test-channel")
	assert.NoError(t, err)

	subs := client.Subscriptions()
	assert.Len(t, subs, 0)
}

func TestWSClient_MultipleSubscriptions(t *testing.T) {
	client := NewWSClient(WSConfig{URL: "wss://example.com/ws"})

	handler := func(data []byte) error { return nil }

	_ = client.Subscribe("channel1", handler)
	_ = client.Subscribe("channel2", handler)
	_ = client.Subscribe("channel3", handler)

	subs := client.Subscriptions()
	assert.Len(t, subs, 3)
}

func TestWSClient_CalculateBackoff(t *testing.T) {
	client := NewWSClient(WSConfig{
		URL:               "wss://example.com/ws",
		ReconnectBaseWait: 1 * time.Second,
		ReconnectMaxWait:  30 * time.Second,
	})

	tests := []struct {
		attempts int
		expected time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second},
		{10, 30 * time.Second},
	}

	for _, tt := range tests {
		result := client.calculateBackoff(tt.attempts)
		assert.Equal(t, tt.expected, result, "backoff for attempt %d", tt.attempts)
	}
}

func TestWSClient_IsConnected(t *testing.T) {
	client := NewWSClient(WSConfig{URL: "wss://example.com/ws"})

	assert.False(t, client.IsConnected())

	client.mu.Lock()
	client.connected = true
	client.mu.Unlock()

	assert.True(t, client.IsConnected())
}

func TestWSClient_WriteMessage_NotConnected(t *testing.T) {
	client := NewWSClient(WSConfig{URL: "wss://example.com/ws"})

	err := client.WriteMessage([]byte("test"))
	assert.Error(t, err)
}

func TestWSClient_SendPing_NotConnected(t *testing.T) {
	client := NewWSClient(WSConfig{URL: "wss://example.com/ws"})

	err := client.SendPing()
	assert.Error(t, err)
}
