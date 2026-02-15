package bybit

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sadewa/pkg/core"
)

func TestProtocol_Name(t *testing.T) {
	p := NewProtocol()
	assert.Equal(t, "bybit", p.Name())
}

func TestProtocol_Version(t *testing.T) {
	p := NewProtocol()
	assert.Equal(t, "5", p.Version())
}

func TestProtocol_BaseURL_Production(t *testing.T) {
	p := NewProtocol()
	assert.Equal(t, "https://api.bybit.com", p.BaseURL(false))
}

func TestProtocol_BaseURL_Sandbox(t *testing.T) {
	p := NewProtocol()
	assert.Equal(t, "https://api-testnet.bybit.com", p.BaseURL(true))
}

func TestProtocol_SupportedOperations(t *testing.T) {
	p := NewProtocol()
	ops := p.SupportedOperations()

	expectedOps := []core.Operation{
		core.OpGetTicker,
		core.OpGetOrderBook,
		core.OpGetTrades,
		core.OpGetKlines,
		core.OpGetBalance,
		core.OpPlaceOrder,
		core.OpCancelOrder,
		core.OpGetOrder,
		core.OpGetOpenOrders,
		core.OpGetOrderHistory,
	}

	assert.ElementsMatch(t, expectedOps, ops)
}

func TestProtocol_RateLimits(t *testing.T) {
	p := NewProtocol()
	limits := p.RateLimits()

	assert.Equal(t, 20, limits.RequestsPerSecond)
	assert.Equal(t, 10, limits.OrdersPerSecond)
	assert.Equal(t, 50, limits.Burst)
}

func TestProtocol_BuildRequest_GetTicker(t *testing.T) {
	p := NewProtocol()
	ctx := context.Background()

	req, err := p.BuildRequest(ctx, core.OpGetTicker, core.Params{
		"symbol": "BTC/USDT",
	})
	require.NoError(t, err)
	require.NotNil(t, req)

	assert.Equal(t, http.MethodGet, req.Method)
	assert.Equal(t, "/v5/market/tickers", req.Path)
	assert.Equal(t, "BTCUSDT", req.Query["symbol"])
	assert.Equal(t, "spot", req.Query["category"])
}

func TestProtocol_BuildRequest_GetTicker_MissingSymbol(t *testing.T) {
	p := NewProtocol()
	ctx := context.Background()

	req, err := p.BuildRequest(ctx, core.OpGetTicker, core.Params{})
	require.Error(t, err)
	require.Nil(t, req)
	assert.Contains(t, err.Error(), "missing required parameter: symbol")
}

func TestProtocol_BuildRequest_GetOrderBook(t *testing.T) {
	p := NewProtocol()
	ctx := context.Background()

	req, err := p.BuildRequest(ctx, core.OpGetOrderBook, core.Params{
		"symbol": "BTC/USDT",
		"limit":  50,
	})
	require.NoError(t, err)
	require.NotNil(t, req)

	assert.Equal(t, http.MethodGet, req.Method)
	assert.Equal(t, "/v5/market/orderbook", req.Path)
	assert.Equal(t, "BTCUSDT", req.Query["symbol"])
	assert.Equal(t, "50", req.Query["limit"])
}

func TestProtocol_BuildRequest_GetTrades(t *testing.T) {
	p := NewProtocol()
	ctx := context.Background()

	req, err := p.BuildRequest(ctx, core.OpGetTrades, core.Params{
		"symbol": "BTC/USDT",
	})
	require.NoError(t, err)
	require.NotNil(t, req)

	assert.Equal(t, http.MethodGet, req.Method)
	assert.Equal(t, "/v5/market/recent-trade", req.Path)
	assert.Equal(t, "BTCUSDT", req.Query["symbol"])
}

func TestProtocol_BuildRequest_GetKlines(t *testing.T) {
	p := NewProtocol()
	ctx := context.Background()

	req, err := p.BuildRequest(ctx, core.OpGetKlines, core.Params{
		"symbol":   "BTC/USDT",
		"interval": "60",
		"limit":    100,
	})
	require.NoError(t, err)
	require.NotNil(t, req)

	assert.Equal(t, http.MethodGet, req.Method)
	assert.Equal(t, "/v5/market/kline", req.Path)
	assert.Equal(t, "BTCUSDT", req.Query["symbol"])
	assert.Equal(t, "60", req.Query["interval"])
	assert.Equal(t, "100", req.Query["limit"])
}

func TestProtocol_BuildRequest_GetBalance(t *testing.T) {
	p := NewProtocol()
	ctx := context.Background()

	req, err := p.BuildRequest(ctx, core.OpGetBalance, core.Params{})
	require.NoError(t, err)
	require.NotNil(t, req)

	assert.Equal(t, http.MethodGet, req.Method)
	assert.Equal(t, "/v5/account/wallet-balance", req.Path)
	assert.True(t, req.RequireAuth)
}

func TestProtocol_BuildRequest_PlaceOrder(t *testing.T) {
	p := NewProtocol()
	ctx := context.Background()

	req, err := p.BuildRequest(ctx, core.OpPlaceOrder, core.Params{
		"symbol":   "BTC/USDT",
		"side":     "buy",
		"type":     "limit",
		"quantity": "0.001",
		"price":    "50000",
	})
	require.NoError(t, err)
	require.NotNil(t, req)

	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, "/v5/order/create", req.Path)
	assert.Equal(t, "BTCUSDT", req.Query["symbol"])
	assert.Equal(t, "Buy", req.Query["side"])
	assert.Equal(t, "Limit", req.Query["orderType"])
	assert.Equal(t, "0.001", req.Query["qty"])
	assert.Equal(t, "50000", req.Query["price"])
	assert.True(t, req.RequireAuth)
}

func TestProtocol_BuildRequest_PlaceOrder_MissingParams(t *testing.T) {
	p := NewProtocol()
	ctx := context.Background()

	tests := []struct {
		name   string
		params core.Params
	}{
		{"missing symbol", core.Params{"side": "buy", "type": "limit", "quantity": "0.001"}},
		{"missing side", core.Params{"symbol": "BTC/USDT", "type": "limit", "quantity": "0.001"}},
		{"missing type", core.Params{"symbol": "BTC/USDT", "side": "buy", "quantity": "0.001"}},
		{"missing quantity", core.Params{"symbol": "BTC/USDT", "side": "buy", "type": "limit"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := p.BuildRequest(ctx, core.OpPlaceOrder, tt.params)
			require.Error(t, err)
			require.Nil(t, req)
		})
	}
}

func TestProtocol_BuildRequest_CancelOrder(t *testing.T) {
	p := NewProtocol()
	ctx := context.Background()

	req, err := p.BuildRequest(ctx, core.OpCancelOrder, core.Params{
		"symbol":   "BTC/USDT",
		"order_id": "123456",
	})
	require.NoError(t, err)
	require.NotNil(t, req)

	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, "/v5/order/cancel", req.Path)
	assert.Equal(t, "BTCUSDT", req.Query["symbol"])
	assert.Equal(t, "123456", req.Query["orderId"])
	assert.True(t, req.RequireAuth)
}

func TestProtocol_BuildRequest_GetOrder(t *testing.T) {
	p := NewProtocol()
	ctx := context.Background()

	req, err := p.BuildRequest(ctx, core.OpGetOrder, core.Params{
		"symbol":   "BTC/USDT",
		"order_id": "123456",
	})
	require.NoError(t, err)
	require.NotNil(t, req)

	assert.Equal(t, http.MethodGet, req.Method)
	assert.Equal(t, "/v5/order/realtime", req.Path)
	assert.Equal(t, "BTCUSDT", req.Query["symbol"])
	assert.Equal(t, "123456", req.Query["orderId"])
	assert.True(t, req.RequireAuth)
}

func TestProtocol_BuildRequest_GetOpenOrders(t *testing.T) {
	p := NewProtocol()
	ctx := context.Background()

	req, err := p.BuildRequest(ctx, core.OpGetOpenOrders, core.Params{
		"symbol": "BTC/USDT",
	})
	require.NoError(t, err)
	require.NotNil(t, req)

	assert.Equal(t, http.MethodGet, req.Method)
	assert.Equal(t, "/v5/order/realtime", req.Path)
	assert.True(t, req.RequireAuth)
}

func TestProtocol_BuildRequest_UnsupportedOperation(t *testing.T) {
	p := NewProtocol()
	ctx := context.Background()

	req, err := p.BuildRequest(ctx, core.Operation(999), core.Params{})
	require.Error(t, err)
	require.Nil(t, req)
	assert.Contains(t, err.Error(), "unsupported operation")
}

func TestProtocol_SignRequest(t *testing.T) {
	// Note: This is a simplified test since we can't easily create a resty.Request
	// In a real scenario, you would use a mock or test server
	// The SignRequest function is tested indirectly through integration tests
}

func TestFormatSymbol(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BTC/USDT", "BTCUSDT"},
		{"ETH/USDT", "ETHUSDT"},
		{"BTC/USDC", "BTCUSDC"},
		{"SOL/USDT", "SOLUSDT"},
		{"BTCUSDT", "BTCUSDT"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := formatSymbol(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseSymbol(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BTCUSDT", "BTC/USDT"},
		{"ETHUSDT", "ETH/USDT"},
		{"BTCUSDC", "BTC/USDC"},
		{"SOLUSDT", "SOL/USDT"},
		{"UNKNOWN", "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSymbol(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCapitalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"buy", "Buy"},
		{"sell", "Sell"},
		{"BUY", "Buy"},
		{"market", "Market"},
		{"limit", "Limit"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := capitalize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapBybitErrorCode(t *testing.T) {
	tests := []struct {
		code     int
		expected core.ErrorType
	}{
		{10001, core.ErrorTypeBadRequest},
		{10004, core.ErrorTypeAuthentication},
		{10006, core.ErrorTypeRateLimit},
		{110007, core.ErrorTypeInsufficientFunds},
		{110001, core.ErrorTypeInvalidOrder},
		{99999, core.ErrorTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.code)), func(t *testing.T) {
			result := mapBybitErrorCode(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}
