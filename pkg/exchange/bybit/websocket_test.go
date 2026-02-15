package bybit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sadewa/pkg/core"
)

func TestNewBybitWSClient(t *testing.T) {
	client := NewBybitWSClient(false)
	assert.NotNil(t, client)
}

func TestNewBybitWSClient_Sandbox(t *testing.T) {
	client := NewBybitWSClient(true)
	assert.NotNil(t, client)
	assert.True(t, client.sandbox)
}

func TestNewBybitWSClientWithConfig(t *testing.T) {
	config := BybitWSConfig{Sandbox: true}
	client := NewBybitWSClientWithConfig(config)
	assert.NotNil(t, client)
	assert.True(t, client.sandbox)
}

func TestBybitWSClient_SubscribeTicker(t *testing.T) {
	client := NewBybitWSClient(false)

	err := client.SubscribeTicker("BTC/USDT", func(_ *core.Ticker) {})
	assert.NoError(t, err)
}

func TestBybitWSClient_UnsubscribeTicker(t *testing.T) {
	client := NewBybitWSClient(false)

	_ = client.SubscribeTicker("BTC/USDT", func(_ *core.Ticker) {})
	client.UnsubscribeTicker("BTC/USDT")
}

func TestBybitWSClient_SubscribeOrderBook(t *testing.T) {
	client := NewBybitWSClient(false)

	err := client.SubscribeOrderBook("BTC/USDT", func(_ *core.OrderBook) {})
	assert.NoError(t, err)
}

func TestBybitWSClient_UnsubscribeOrderBook(t *testing.T) {
	client := NewBybitWSClient(false)

	_ = client.SubscribeOrderBook("BTC/USDT", func(_ *core.OrderBook) {})
	client.UnsubscribeOrderBook("BTC/USDT")
}

func TestBybitWSClient_SubscribeTrades(t *testing.T) {
	client := NewBybitWSClient(false)

	err := client.SubscribeTrades("BTC/USDT", func(_ *core.Trade) {})
	assert.NoError(t, err)
}

func TestBybitWSClient_UnsubscribeTrades(t *testing.T) {
	client := NewBybitWSClient(false)

	_ = client.SubscribeTrades("BTC/USDT", func(_ *core.Trade) {})
	client.UnsubscribeTrades("BTC/USDT")
}

func TestBybitWSClient_SubscribeKlines(t *testing.T) {
	client := NewBybitWSClient(false)

	err := client.SubscribeKlines("BTC/USDT", "1m", func(_ *core.Kline) {})
	assert.NoError(t, err)
}

func TestBybitWSClient_UnsubscribeKlines(t *testing.T) {
	client := NewBybitWSClient(false)

	_ = client.SubscribeKlines("BTC/USDT", "1m", func(_ *core.Kline) {})
	client.UnsubscribeKlines("BTC/USDT", "1m")
}

func TestBybitWSClient_Subscriptions(t *testing.T) {
	client := NewBybitWSClient(false)

	subs := client.Subscriptions()
	assert.NotNil(t, subs)
}

func TestBybitWSClient_IsConnected(t *testing.T) {
	client := NewBybitWSClient(false)

	assert.False(t, client.IsConnected())
}

func TestBybitWSClient_SetLogger(t *testing.T) {
	client := NewBybitWSClient(false)

	// Should not panic
	client.SetLogger(newTestLogger())
}
