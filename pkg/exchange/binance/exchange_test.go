package binance

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sadewa/internal/keyring"
	"sadewa/pkg/core"
	"sadewa/pkg/exchange"
)

var _ exchange.Exchange = (*BinanceExchange)(nil)

func TestBinanceExchange_ImplementsInterface(t *testing.T) {
	var ex exchange.Exchange = &BinanceExchange{}
	assert.NotNil(t, ex)
}

func TestNew_BasicConfig(t *testing.T) {
	config := core.DefaultConfig("binance")

	ex, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, ex)

	assert.Equal(t, "binance", ex.Name())
	assert.Equal(t, "3", ex.Version())
}

func TestNew_SpotProduction(t *testing.T) {
	config := core.DefaultConfig("binance")
	config.MarketType = core.MarketTypeSpot
	config.Sandbox = false

	ex, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, ex)
}

func TestNew_SpotSandbox(t *testing.T) {
	config := core.DefaultConfig("binance")
	config.MarketType = core.MarketTypeSpot
	config.Sandbox = true

	ex, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, ex)
}

func TestNew_FuturesProduction(t *testing.T) {
	config := core.DefaultConfig("binance")
	config.MarketType = core.MarketTypeFutures
	config.Sandbox = false

	ex, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, ex)
}

func TestNew_FuturesSandbox(t *testing.T) {
	config := core.DefaultConfig("binance")
	config.MarketType = core.MarketTypeFutures
	config.Sandbox = true

	ex, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, ex)
}

func TestWithKeyRing(t *testing.T) {
	config := core.DefaultConfig("binance")
	kr := newTestKeyRing()

	ex, err := New(config, WithKeyRing(kr))
	require.NoError(t, err)
	require.NotNil(t, ex)
}

func TestWithLogger(t *testing.T) {
	config := core.DefaultConfig("binance")

	ex, err := New(config, WithLogger(newTestLogger()))
	require.NoError(t, err)
	require.NotNil(t, ex)
}

func TestBaseURL_SpotProduction(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeSpot, Sandbox: false}
	assert.Equal(t, "https://api.binance.com", baseURL(config))
}

func TestBaseURL_SpotSandbox(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeSpot, Sandbox: true}
	assert.Equal(t, "https://testnet.binance.vision", baseURL(config))
}

func TestBaseURL_FuturesProduction(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeFutures, Sandbox: false}
	assert.Equal(t, "https://fapi.binance.com", baseURL(config))
}

func TestBaseURL_FuturesSandbox(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeFutures, Sandbox: true}
	assert.Equal(t, "https://testnet.binancefuture.com", baseURL(config))
}

func TestWSURL_SpotProduction(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeSpot, Sandbox: false}
	assert.Equal(t, "wss://stream.binance.com:9443/ws", wsURL(config))
}

func TestWSURL_SpotSandbox(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeSpot, Sandbox: true}
	assert.Equal(t, "wss://testnet.binance.vision/ws", wsURL(config))
}

func TestWSURL_FuturesProduction(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeFutures, Sandbox: false}
	assert.Equal(t, "wss://fstream.binance.com/ws", wsURL(config))
}

func TestWSURL_FuturesSandbox(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeFutures, Sandbox: true}
	assert.Equal(t, "wss://stream.binancefuture.com/ws", wsURL(config))
}

func TestBinanceExchange_Close(t *testing.T) {
	config := core.DefaultConfig("binance")

	ex, err := New(config)
	require.NoError(t, err)

	err = ex.Close()
	assert.NoError(t, err)
}

func TestRegister(t *testing.T) {
	container := exchange.NewContainer()
	config := core.DefaultConfig("binance")

	err := Register(container, config)
	require.NoError(t, err)

	ex, err := container.Get("binance")
	require.NoError(t, err)
	assert.NotNil(t, ex)
	assert.Equal(t, "binance", ex.Name())
}

func newTestKeyRing() *keyring.KeyRing {
	return keyring.NewKeyRing([]*keyring.APIKey{
		{ID: "test", Key: "test-key", Secret: "test-secret"},
	}, keyring.RotationRoundRobin)
}

func newTestLogger() zerolog.Logger {
	return zerolog.Nop()
}
