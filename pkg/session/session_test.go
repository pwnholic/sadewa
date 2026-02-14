package session

import (
	"context"
	"iter"
	"testing"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/stretchr/testify/assert"

	"sadewa/pkg/core"
	"sadewa/pkg/exchange"
)

var _ = context.Background

type MockExchange struct {
	name                   string
	version                string
	getTickerFunc          func(ctx context.Context, symbol string, opts ...exchange.Option) (*core.Ticker, error)
	getOrderBookFunc       func(ctx context.Context, symbol string, opts ...exchange.Option) (*core.OrderBook, error)
	getTradesFunc          func(ctx context.Context, symbol string, opts ...exchange.Option) iter.Seq2[*core.Trade, error]
	getKlinesFunc          func(ctx context.Context, symbol string, opts ...exchange.Option) ([]core.Kline, error)
	getBalanceFunc         func(ctx context.Context, opts ...exchange.Option) ([]core.Balance, error)
	placeOrderFunc         func(ctx context.Context, req *exchange.OrderRequest, opts ...exchange.Option) (*core.Order, error)
	cancelOrderFunc        func(ctx context.Context, req *exchange.CancelRequest, opts ...exchange.Option) (*core.Order, error)
	getOrderFunc           func(ctx context.Context, req *exchange.OrderQuery, opts ...exchange.Option) (*core.Order, error)
	getOpenOrdersFunc      func(ctx context.Context, symbol string, opts ...exchange.Option) ([]core.Order, error)
	subscribeTickerFunc    func(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.Ticker, <-chan error)
	subscribeTradesFunc    func(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.Trade, <-chan error)
	subscribeOrderBookFunc func(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.OrderBook, <-chan error)
}

func (m *MockExchange) Name() string {
	return m.name
}

func (m *MockExchange) Version() string {
	return m.version
}

func (m *MockExchange) GetTicker(ctx context.Context, symbol string, opts ...exchange.Option) (*core.Ticker, error) {
	if m.getTickerFunc != nil {
		return m.getTickerFunc(ctx, symbol, opts...)
	}
	return &core.Ticker{Symbol: symbol}, nil
}

func (m *MockExchange) GetOrderBook(ctx context.Context, symbol string, opts ...exchange.Option) (*core.OrderBook, error) {
	if m.getOrderBookFunc != nil {
		return m.getOrderBookFunc(ctx, symbol, opts...)
	}
	return &core.OrderBook{Symbol: symbol}, nil
}

func (m *MockExchange) GetTrades(ctx context.Context, symbol string, opts ...exchange.Option) iter.Seq2[*core.Trade, error] {
	if m.getTradesFunc != nil {
		return m.getTradesFunc(ctx, symbol, opts...)
	}
	return func(yield func(*core.Trade, error) bool) {
		yield(&core.Trade{Symbol: symbol}, nil)
	}
}

func (m *MockExchange) GetKlines(ctx context.Context, symbol string, opts ...exchange.Option) ([]core.Kline, error) {
	if m.getKlinesFunc != nil {
		return m.getKlinesFunc(ctx, symbol, opts...)
	}
	return []core.Kline{{Symbol: symbol}}, nil
}

func (m *MockExchange) GetBalance(ctx context.Context, opts ...exchange.Option) ([]core.Balance, error) {
	if m.getBalanceFunc != nil {
		return m.getBalanceFunc(ctx, opts...)
	}
	return []core.Balance{{Asset: "BTC"}}, nil
}

func (m *MockExchange) PlaceOrder(ctx context.Context, req *exchange.OrderRequest, opts ...exchange.Option) (*core.Order, error) {
	if m.placeOrderFunc != nil {
		return m.placeOrderFunc(ctx, req, opts...)
	}
	return &core.Order{Symbol: req.Symbol}, nil
}

func (m *MockExchange) CancelOrder(ctx context.Context, req *exchange.CancelRequest, opts ...exchange.Option) (*core.Order, error) {
	if m.cancelOrderFunc != nil {
		return m.cancelOrderFunc(ctx, req, opts...)
	}
	return &core.Order{Symbol: req.Symbol}, nil
}

func (m *MockExchange) GetOrder(ctx context.Context, req *exchange.OrderQuery, opts ...exchange.Option) (*core.Order, error) {
	if m.getOrderFunc != nil {
		return m.getOrderFunc(ctx, req, opts...)
	}
	return &core.Order{Symbol: req.Symbol, ID: req.OrderID}, nil
}

func (m *MockExchange) GetOpenOrders(ctx context.Context, symbol string, opts ...exchange.Option) ([]core.Order, error) {
	if m.getOpenOrdersFunc != nil {
		return m.getOpenOrdersFunc(ctx, symbol, opts...)
	}
	return []core.Order{}, nil
}

func (m *MockExchange) SubscribeTicker(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.Ticker, <-chan error) {
	if m.subscribeTickerFunc != nil {
		return m.subscribeTickerFunc(ctx, symbol, opts...)
	}
	tickerCh := make(chan *core.Ticker)
	errCh := make(chan error)
	close(tickerCh)
	close(errCh)
	return tickerCh, errCh
}

func (m *MockExchange) SubscribeTrades(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.Trade, <-chan error) {
	if m.subscribeTradesFunc != nil {
		return m.subscribeTradesFunc(ctx, symbol, opts...)
	}
	tradeCh := make(chan *core.Trade)
	errCh := make(chan error)
	close(tradeCh)
	close(errCh)
	return tradeCh, errCh
}

func (m *MockExchange) SubscribeOrderBook(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.OrderBook, <-chan error) {
	if m.subscribeOrderBookFunc != nil {
		return m.subscribeOrderBookFunc(ctx, symbol, opts...)
	}
	obCh := make(chan *core.OrderBook)
	errCh := make(chan error)
	close(obCh)
	close(errCh)
	return obCh, errCh
}

func TestNewSession(t *testing.T) {
	tests := []struct {
		name    string
		config  *core.Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  core.DefaultConfig("test"),
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "invalid config - empty exchange",
			config: &core.Config{
				Timeout:           10 * time.Second,
				RateLimitRequests: 1200,
				RateLimitPeriod:   time.Minute,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := New(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, session)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, session)
			assert.Equal(t, SessionStateNew, session.State())
		})
	}
}

func TestNewSessionWithContainer(t *testing.T) {
	container := exchange.NewContainer()
	config := core.DefaultConfig("test")

	session, err := NewSession(container, config)
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, SessionStateNew, session.State())
}

func TestNewSessionWithContainer_NilContainer(t *testing.T) {
	config := core.DefaultConfig("test")

	session, err := NewSession(nil, config)
	assert.Error(t, err)
	assert.Nil(t, session)
}

func TestNewSessionWithContainer_NilConfig(t *testing.T) {
	container := exchange.NewContainer()

	session, err := NewSession(container, nil)
	assert.Error(t, err)
	assert.Nil(t, session)
}

func TestSession_SetExchange(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	mockExchange := &MockExchange{name: "mock"}
	session.RegisterExchange("mock", mockExchange)

	err = session.SetExchange("mock")
	assert.NoError(t, err)
	assert.Equal(t, SessionStateActive, session.State())
	assert.Equal(t, "mock", session.CurrentExchange())
}

func TestSession_SetExchange_NotFound(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	err = session.SetExchange("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, SessionStateNew, session.State())
}

func TestSession_State(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	assert.Equal(t, SessionStateNew, session.State())

	mockExchange := &MockExchange{name: "mock"}
	session.RegisterExchange("mock", mockExchange)
	session.SetExchange("mock")
	assert.Equal(t, SessionStateActive, session.State())

	session.Close()
	assert.Equal(t, SessionStateClosed, session.State())
}

func TestSession_Close(t *testing.T) {
	config := core.DefaultConfig("test")
	config.CacheEnabled = true
	session, err := New(config)
	assert.NoError(t, err)

	mockExchange := &MockExchange{name: "mock"}
	session.RegisterExchange("mock", mockExchange)
	session.SetExchange("mock")

	err = session.Close()
	assert.NoError(t, err)
	assert.Equal(t, SessionStateClosed, session.State())
}

func TestSession_GetTicker_NoExchange(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	_, err = session.GetTicker(context.Background(), "BTC/USDT")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exchange not set")
}

func TestSession_GetTicker(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	mockExchange := &MockExchange{
		name: "mock",
		getTickerFunc: func(ctx context.Context, symbol string, opts ...exchange.Option) (*core.Ticker, error) {
			return &core.Ticker{Symbol: symbol}, nil
		},
	}
	session.RegisterExchange("mock", mockExchange)
	session.SetExchange("mock")

	ticker, err := session.GetTicker(context.Background(), "BTC/USDT")
	assert.NoError(t, err)
	assert.Equal(t, "BTC/USDT", ticker.Symbol)
}

func TestSession_GetOrderBook(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	mockExchange := &MockExchange{
		name: "mock",
		getOrderBookFunc: func(ctx context.Context, symbol string, opts ...exchange.Option) (*core.OrderBook, error) {
			return &core.OrderBook{Symbol: symbol}, nil
		},
	}
	session.RegisterExchange("mock", mockExchange)
	session.SetExchange("mock")

	ob, err := session.GetOrderBook(context.Background(), "BTC/USDT")
	assert.NoError(t, err)
	assert.Equal(t, "BTC/USDT", ob.Symbol)
}

func TestSession_GetTrades(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	mockExchange := &MockExchange{
		name: "mock",
		getTradesFunc: func(ctx context.Context, symbol string, opts ...exchange.Option) iter.Seq2[*core.Trade, error] {
			return func(yield func(*core.Trade, error) bool) {
				yield(&core.Trade{Symbol: symbol, ID: "1"}, nil)
				yield(&core.Trade{Symbol: symbol, ID: "2"}, nil)
			}
		},
	}
	session.RegisterExchange("mock", mockExchange)
	session.SetExchange("mock")

	var trades []*core.Trade
	for trade, err := range session.GetTrades(context.Background(), "BTC/USDT") {
		assert.NoError(t, err)
		trades = append(trades, trade)
	}
	assert.Len(t, trades, 2)
}

func TestSession_GetKlines(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	mockExchange := &MockExchange{
		name: "mock",
		getKlinesFunc: func(ctx context.Context, symbol string, opts ...exchange.Option) ([]core.Kline, error) {
			return []core.Kline{{Symbol: symbol}}, nil
		},
	}
	session.RegisterExchange("mock", mockExchange)
	session.SetExchange("mock")

	klines, err := session.GetKlines(context.Background(), "BTC/USDT")
	assert.NoError(t, err)
	assert.Len(t, klines, 1)
}

func TestSession_GetBalance(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	mockExchange := &MockExchange{
		name: "mock",
		getBalanceFunc: func(ctx context.Context, opts ...exchange.Option) ([]core.Balance, error) {
			return []core.Balance{{Asset: "BTC"}}, nil
		},
	}
	session.RegisterExchange("mock", mockExchange)
	session.SetExchange("mock")

	balances, err := session.GetBalance(context.Background())
	assert.NoError(t, err)
	assert.Len(t, balances, 1)
}

func TestSession_PlaceOrder(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	mockExchange := &MockExchange{
		name: "mock",
		placeOrderFunc: func(ctx context.Context, req *exchange.OrderRequest, opts ...exchange.Option) (*core.Order, error) {
			return &core.Order{Symbol: req.Symbol, ID: "123"}, nil
		},
	}
	session.RegisterExchange("mock", mockExchange)
	session.SetExchange("mock")

	var qty apd.Decimal
	qty.SetInt64(1)

	order, err := session.PlaceOrder(context.Background(), &exchange.OrderRequest{
		Symbol:   "BTC/USDT",
		Side:     core.SideBuy,
		Type:     core.TypeLimit,
		Quantity: qty,
	})
	assert.NoError(t, err)
	assert.Equal(t, "BTC/USDT", order.Symbol)
	assert.Equal(t, "123", order.ID)
}

func TestSession_CancelOrder(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	mockExchange := &MockExchange{
		name: "mock",
		cancelOrderFunc: func(ctx context.Context, req *exchange.CancelRequest, opts ...exchange.Option) (*core.Order, error) {
			return &core.Order{Symbol: req.Symbol, ID: req.OrderID, Status: core.StatusCanceled}, nil
		},
	}
	session.RegisterExchange("mock", mockExchange)
	session.SetExchange("mock")

	order, err := session.CancelOrder(context.Background(), &exchange.CancelRequest{
		Symbol:  "BTC/USDT",
		OrderID: "123",
	})
	assert.NoError(t, err)
	assert.Equal(t, core.StatusCanceled, order.Status)
}

func TestSession_GetOrder(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	mockExchange := &MockExchange{
		name: "mock",
		getOrderFunc: func(ctx context.Context, req *exchange.OrderQuery, opts ...exchange.Option) (*core.Order, error) {
			return &core.Order{Symbol: req.Symbol, ID: req.OrderID}, nil
		},
	}
	session.RegisterExchange("mock", mockExchange)
	session.SetExchange("mock")

	order, err := session.GetOrder(context.Background(), &exchange.OrderQuery{
		Symbol:  "BTC/USDT",
		OrderID: "123",
	})
	assert.NoError(t, err)
	assert.Equal(t, "123", order.ID)
}

func TestSession_GetOpenOrders(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	mockExchange := &MockExchange{
		name: "mock",
		getOpenOrdersFunc: func(ctx context.Context, symbol string, opts ...exchange.Option) ([]core.Order, error) {
			return []core.Order{{Symbol: symbol, ID: "1"}}, nil
		},
	}
	session.RegisterExchange("mock", mockExchange)
	session.SetExchange("mock")

	orders, err := session.GetOpenOrders(context.Background(), "BTC/USDT")
	assert.NoError(t, err)
	assert.Len(t, orders, 1)
}

func TestCache_GetSet(t *testing.T) {
	cache := NewCache(1 * time.Second)
	ctx := context.Background()

	err := cache.Set(ctx, "key1", "value1", 0)
	assert.NoError(t, err)

	val, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", val)
}

func TestCache_Get_NotExists(t *testing.T) {
	cache := NewCache(1 * time.Second)
	ctx := context.Background()

	val, err := cache.Get(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestCache_Get_Expired(t *testing.T) {
	cache := NewCache(1 * time.Nanosecond)
	ctx := context.Background()

	err := cache.Set(ctx, "key1", "value1", 1*time.Nanosecond)
	assert.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	val, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestCache_Delete(t *testing.T) {
	cache := NewCache(1 * time.Second)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)

	err := cache.Delete(ctx, "key1")
	assert.NoError(t, err)

	val, err := cache.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestCache_Clear(t *testing.T) {
	cache := NewCache(1 * time.Second)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)

	cache.Clear()

	val1, _ := cache.Get(ctx, "key1")
	val2, _ := cache.Get(ctx, "key2")

	assert.Nil(t, val1)
	assert.Nil(t, val2)
}

func TestSession_ClearCache(t *testing.T) {
	config := core.DefaultConfig("test")
	config.CacheEnabled = true
	session, err := New(config)
	assert.NoError(t, err)

	session.cache.Set(context.Background(), "key1", "value1", 0)

	session.ClearCache()

	val, _ := session.cache.Get(context.Background(), "key1")
	assert.Nil(t, val)
}

func TestSession_Config(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	retrievedConfig := session.Config()
	assert.Equal(t, config, retrievedConfig)
}

func TestSession_CurrentExchange(t *testing.T) {
	config := core.DefaultConfig("test")
	session, err := New(config)
	assert.NoError(t, err)

	assert.Equal(t, "", session.CurrentExchange())

	mockExchange := &MockExchange{name: "mock"}
	session.RegisterExchange("mock", mockExchange)
	session.SetExchange("mock")

	assert.Equal(t, "mock", session.CurrentExchange())
}

func TestSessionState_String(t *testing.T) {
	assert.Equal(t, "NEW", SessionStateNew.String())
	assert.Equal(t, "ACTIVE", SessionStateActive.String())
	assert.Equal(t, "CLOSED", SessionStateClosed.String())
}
