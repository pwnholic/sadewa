package bybit

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sadewa/internal/keyring"
	"sadewa/pkg/core"
	"sadewa/pkg/exchange"
)

var _ exchange.Exchange = (*BybitExchange)(nil)

func TestBybitExchange_ImplementsInterface(t *testing.T) {
	var ex exchange.Exchange = &BybitExchange{}
	assert.NotNil(t, ex)
}

func TestNew_ValidConfig(t *testing.T) {
	config := core.DefaultConfig("bybit")

	ex, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, ex)

	assert.Equal(t, "bybit", ex.Name())
	assert.Equal(t, "5", ex.Version())
}

func TestNew_InvalidConfig(t *testing.T) {
	config := &core.Config{}

	ex, err := New(config)
	require.Error(t, err)
	require.Nil(t, ex)
}

func TestNew_SpotProduction(t *testing.T) {
	config := core.DefaultConfig("bybit")
	config.MarketType = core.MarketTypeSpot
	config.Sandbox = false

	ex, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, ex)
}

func TestNew_SpotSandbox(t *testing.T) {
	config := core.DefaultConfig("bybit")
	config.MarketType = core.MarketTypeSpot
	config.Sandbox = true

	ex, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, ex)
}

func TestNew_FuturesProduction(t *testing.T) {
	config := core.DefaultConfig("bybit")
	config.MarketType = core.MarketTypeFutures
	config.Sandbox = false

	ex, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, ex)
}

func TestNew_FuturesSandbox(t *testing.T) {
	config := core.DefaultConfig("bybit")
	config.MarketType = core.MarketTypeFutures
	config.Sandbox = true

	ex, err := New(config)
	require.NoError(t, err)
	require.NotNil(t, ex)
}

func TestWithKeyRing(t *testing.T) {
	config := core.DefaultConfig("bybit")
	kr := newTestKeyRing()

	ex, err := New(config, WithKeyRing(kr))
	require.NoError(t, err)
	require.NotNil(t, ex)
}

func TestWithLogger(t *testing.T) {
	config := core.DefaultConfig("bybit")

	ex, err := New(config, WithLogger(newTestLogger()))
	require.NoError(t, err)
	require.NotNil(t, ex)
}

func TestBaseURL_SpotProduction(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeSpot, Sandbox: false}
	assert.Equal(t, "https://api.bybit.com", baseURL(config))
}

func TestBaseURL_SpotSandbox(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeSpot, Sandbox: true}
	assert.Equal(t, "https://api-testnet.bybit.com", baseURL(config))
}

func TestBaseURL_FuturesProduction(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeFutures, Sandbox: false}
	assert.Equal(t, "https://api.bybit.com", baseURL(config))
}

func TestBaseURL_FuturesSandbox(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeFutures, Sandbox: true}
	assert.Equal(t, "https://api-testnet.bybit.com", baseURL(config))
}

func TestWSURL_SpotProduction(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeSpot, Sandbox: false}
	assert.Equal(t, "wss://stream.bybit.com/v5/public/spot", wsURL(config))
}

func TestWSURL_SpotSandbox(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeSpot, Sandbox: true}
	assert.Equal(t, "wss://stream-testnet.bybit.com/v5/public/spot", wsURL(config))
}

func TestWSURL_FuturesProduction(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeFutures, Sandbox: false}
	assert.Equal(t, "wss://stream.bybit.com/v5/public/spot", wsURL(config))
}

func TestWSURL_FuturesSandbox(t *testing.T) {
	config := &core.Config{MarketType: core.MarketTypeFutures, Sandbox: true}
	assert.Equal(t, "wss://stream-testnet.bybit.com/v5/public/spot", wsURL(config))
}

func TestBybitExchange_Close(t *testing.T) {
	config := core.DefaultConfig("bybit")

	ex, err := New(config)
	require.NoError(t, err)

	err = ex.Close()
	assert.NoError(t, err)
}

func TestRegister(t *testing.T) {
	container := exchange.NewContainer()
	config := core.DefaultConfig("bybit")

	err := Register(container, config)
	require.NoError(t, err)

	ex, err := container.Get("bybit")
	require.NoError(t, err)
	assert.NotNil(t, ex)
	assert.Equal(t, "bybit", ex.Name())
}

func newTestKeyRing() *keyring.KeyRing {
	return keyring.NewKeyRing([]*keyring.APIKey{
		{ID: "test", Key: "test-key", Secret: "test-secret"},
	}, keyring.RotationRoundRobin)
}

func newTestLogger() zerolog.Logger {
	return zerolog.Nop()
}
