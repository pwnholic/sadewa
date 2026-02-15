package exchange

import (
	"context"
	"iter"
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sadewa/pkg/core"
)

type mockExchange struct {
	name string
}

func (m *mockExchange) Name() string    { return m.name }
func (m *mockExchange) Version() string { return "1.0" }
func (m *mockExchange) GetTicker(ctx context.Context, s string, opts ...Option) (*core.Ticker, error) {
	return nil, nil
}
func (m *mockExchange) GetOrderBook(ctx context.Context, s string, opts ...Option) (*core.OrderBook, error) {
	return nil, nil
}
func (m *mockExchange) GetTrades(ctx context.Context, s string, opts ...Option) iter.Seq2[*core.Trade, error] {
	return nil
}
func (m *mockExchange) GetKlines(ctx context.Context, s string, opts ...Option) ([]core.Kline, error) {
	return nil, nil
}
func (m *mockExchange) GetBalance(ctx context.Context, opts ...Option) ([]core.Balance, error) {
	return nil, nil
}
func (m *mockExchange) PlaceOrder(ctx context.Context, req *OrderRequest, opts ...Option) (*core.Order, error) {
	return nil, nil
}
func (m *mockExchange) CancelOrder(ctx context.Context, req *CancelRequest, opts ...Option) (*core.Order, error) {
	return nil, nil
}
func (m *mockExchange) GetOrder(ctx context.Context, req *OrderQuery, opts ...Option) (*core.Order, error) {
	return nil, nil
}
func (m *mockExchange) GetOpenOrders(ctx context.Context, s string, opts ...Option) ([]core.Order, error) {
	return nil, nil
}
func (m *mockExchange) SubscribeTicker(ctx context.Context, s string, opts ...Option) (<-chan *core.Ticker, <-chan error) {
	return nil, nil
}
func (m *mockExchange) SubscribeTrades(ctx context.Context, s string, opts ...Option) (<-chan *core.Trade, <-chan error) {
	return nil, nil
}
func (m *mockExchange) SubscribeOrderBook(ctx context.Context, s string, opts ...Option) (<-chan *core.OrderBook, <-chan error) {
	return nil, nil
}
func (m *mockExchange) SubscribeKlines(ctx context.Context, s string, opts ...Option) (<-chan *core.Kline, <-chan error) {
	return nil, nil
}
func (m *mockExchange) Close() error { return nil }

func TestContainer_NewContainer(t *testing.T) {
	c := NewContainer()
	assert.NotNil(t, c)
	assert.NotNil(t, c.exchanges)
}

func TestContainer_Register(t *testing.T) {
	c := NewContainer()
	ex := &mockExchange{name: "test"}

	c.Register("test", ex)

	assert.True(t, c.Exists("test"))
}

func TestContainer_Get(t *testing.T) {
	c := NewContainer()
	ex := &mockExchange{name: "test"}
	c.Register("test", ex)

	got, err := c.Get("test")
	require.NoError(t, err)
	assert.Equal(t, "test", got.Name())

	_, err = c.Get("notfound")
	assert.Error(t, err)
}

func TestContainer_Names(t *testing.T) {
	c := NewContainer()
	c.Register("binance", &mockExchange{name: "binance"})
	c.Register("coinbase", &mockExchange{name: "coinbase"})

	names := c.Names()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "binance")
	assert.Contains(t, names, "coinbase")
}

func TestContainer_Unregister(t *testing.T) {
	c := NewContainer()
	c.Register("test", &mockExchange{name: "test"})

	c.Unregister("test")

	assert.False(t, c.Exists("test"))
}

func TestContainer_Clear(t *testing.T) {
	c := NewContainer()
	c.Register("a", &mockExchange{name: "a"})
	c.Register("b", &mockExchange{name: "b"})

	c.Clear()

	assert.Empty(t, c.Names())
}

func TestApplyOptions(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		opts := ApplyOptions()
		assert.Equal(t, 0, opts.Limit)
		assert.Equal(t, "", opts.Interval)
	})

	t.Run("with all options", func(t *testing.T) {
		opts := ApplyOptions(
			WithLimit(100),
			WithInterval("1h"),
			WithMarketType(1),
		)
		assert.Equal(t, 100, opts.Limit)
		assert.Equal(t, "1h", opts.Interval)
		assert.Equal(t, 1, int(opts.MarketType))
	})
}

func TestOrderRequest(t *testing.T) {
	req := &OrderRequest{
		Symbol:      "BTC/USDT",
		Side:        core.SideBuy,
		Type:        core.TypeLimit,
		Price:       apd.Decimal{},
		Quantity:    apd.Decimal{},
		TimeInForce: core.GTC,
	}
	assert.Equal(t, "BTC/USDT", req.Symbol)
	assert.Equal(t, core.SideBuy, req.Side)
}

func TestCancelRequest(t *testing.T) {
	req := &CancelRequest{Symbol: "BTC/USDT", OrderID: "123"}
	assert.Equal(t, "BTC/USDT", req.Symbol)
	assert.Equal(t, "123", req.OrderID)
}

func TestOrderQuery(t *testing.T) {
	q := &OrderQuery{Symbol: "BTC/USDT", OrderID: "123"}
	assert.Equal(t, "BTC/USDT", q.Symbol)
	assert.Equal(t, "123", q.OrderID)
}
