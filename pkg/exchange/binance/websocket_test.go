package binance

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sadewa/pkg/core"
)

func TestNewBinanceWSClient(t *testing.T) {
	client := NewBinanceWSClient(false)
	assert.NotNil(t, client)
}

func TestNewBinanceWSClient_Sandbox(t *testing.T) {
	client := NewBinanceWSClient(true)
	assert.NotNil(t, client)
}

func TestBuildStreamName(t *testing.T) {
	tests := []struct {
		symbol     string
		streamType string
		expected   string
	}{
		{"BTC/USDT", "ticker", "btcusdt@ticker"},
		{"ETH/USDT", "trade", "ethusdt@trade"},
		{"BTC/USDT", "depth", "btcusdt@depth"},
		{"ETH/USDT", "kline", "ethusdt@kline"},
	}

	for _, tt := range tests {
		result := buildStreamName(tt.symbol, tt.streamType)
		assert.Equal(t, tt.expected, result)
	}
}

func TestBinanceWSClient_SubscribeTicker(t *testing.T) {
	client := NewBinanceWSClient(false)

	err := client.SubscribeTicker("BTC/USDT", func(_ *core.Ticker) {})
	assert.NoError(t, err)
}

func TestBinanceWSClient_UnsubscribeTicker(t *testing.T) {
	client := NewBinanceWSClient(false)

	_ = client.SubscribeTicker("BTC/USDT", func(_ *core.Ticker) {})
	client.UnsubscribeTicker("BTC/USDT")
}

func TestBinanceWSClient_SubscribeOrderBook(t *testing.T) {
	client := NewBinanceWSClient(false)

	err := client.SubscribeOrderBook("BTC/USDT", func(_ *core.OrderBook) {})
	assert.NoError(t, err)
}

func TestBinanceWSClient_SubscribeTrades(t *testing.T) {
	client := NewBinanceWSClient(false)

	err := client.SubscribeTrades("BTC/USDT", func(_ *core.Trade) {})
	assert.NoError(t, err)
}

func TestBinanceWSClient_SubscribeKlines(t *testing.T) {
	client := NewBinanceWSClient(false)

	err := client.SubscribeKlines("BTC/USDT", "1m", func(_ *core.Kline) {})
	assert.NoError(t, err)
}

func TestBinanceWSClient_SubscribeAggTrades(t *testing.T) {
	client := NewBinanceWSClient(false)

	err := client.SubscribeAggTrades("BTC/USDT", func(_ *core.Trade) {})
	assert.NoError(t, err)
}

func TestBinanceWSClient_SubscribeMiniTicker(t *testing.T) {
	client := NewBinanceWSClient(false)

	err := client.SubscribeMiniTicker("BTC/USDT", func(_ *core.Ticker) {})
	assert.NoError(t, err)
}
