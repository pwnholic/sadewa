package exchange

import (
	"context"
	"iter"

	"github.com/cockroachdb/apd/v3"

	"sadewa/pkg/core"
)

// Exchange defines the unified interface for interacting with cryptocurrency exchanges.
// All exchange implementations must provide market data retrieval, account management,
// order execution, and real-time data streaming capabilities.
type Exchange interface {
	Name() string
	Version() string

	GetTicker(ctx context.Context, symbol string, opts ...Option) (*core.Ticker, error)
	GetOrderBook(ctx context.Context, symbol string, opts ...Option) (*core.OrderBook, error)
	GetTrades(ctx context.Context, symbol string, opts ...Option) iter.Seq2[*core.Trade, error]
	GetKlines(ctx context.Context, symbol string, opts ...Option) ([]core.Kline, error)

	GetBalance(ctx context.Context, opts ...Option) ([]core.Balance, error)

	PlaceOrder(ctx context.Context, req *OrderRequest, opts ...Option) (*core.Order, error)
	CancelOrder(ctx context.Context, req *CancelRequest, opts ...Option) (*core.Order, error)
	GetOrder(ctx context.Context, req *OrderQuery, opts ...Option) (*core.Order, error)
	GetOpenOrders(ctx context.Context, symbol string, opts ...Option) ([]core.Order, error)

	SubscribeTicker(ctx context.Context, symbol string, opts ...Option) (<-chan *core.Ticker, <-chan error)
	SubscribeTrades(ctx context.Context, symbol string, opts ...Option) (<-chan *core.Trade, <-chan error)
	SubscribeOrderBook(ctx context.Context, symbol string, opts ...Option) (<-chan *core.OrderBook, <-chan error)
}

// OrderRequest contains the parameters required to place a new order on an exchange.
type OrderRequest struct {
	Symbol        string
	Side          core.OrderSide
	Type          core.OrderType
	Price         apd.Decimal
	Quantity      apd.Decimal
	TimeInForce   core.TimeInForce
	ClientOrderID string
}

// CancelRequest contains the parameters required to cancel an existing order.
type CancelRequest struct {
	Symbol  string
	OrderID string
}

// OrderQuery contains the parameters required to query order status.
type OrderQuery struct {
	Symbol  string
	OrderID string
}
