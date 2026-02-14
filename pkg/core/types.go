package core

import (
	"time"

	"github.com/cockroachdb/apd/v3"
)

// OrderSide represents the direction of an order (buy or sell).
type OrderSide int

// Order side constants define the direction of a trade.
const (
	// SideBuy indicates an order to purchase an asset.
	SideBuy OrderSide = iota
	// SideSell indicates an order to sell an asset.
	SideSell
)

// String returns the string representation of the order side ("BUY" or "SELL").
func (s OrderSide) String() string {
	return [...]string{"BUY", "SELL"}[s]
}

// MarshalJSON implements json.Marshaler for OrderSide.
func (s OrderSide) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler for OrderSide.
// It accepts both uppercase and lowercase formats.
func (s *OrderSide) UnmarshalJSON(data []byte) error {
	str := string(data)
	switch str {
	case `"BUY"`, `"buy"`:
		*s = SideBuy
	case `"SELL"`, `"sell"`:
		*s = SideSell
	}
	return nil
}

// OrderType represents the type of order to place on an exchange.
type OrderType int

// Order type constants define how an order is executed.
const (
	// TypeMarket executes immediately at the best available price.
	TypeMarket OrderType = iota
	// TypeLimit executes at a specified price or better.
	TypeLimit
	// TypeStopLoss triggers a market order when price reaches stop price.
	TypeStopLoss
	// TypeStopLossLimit triggers a limit order when price reaches stop price.
	TypeStopLossLimit
	// TypeTakeProfit triggers a market order when price reaches target.
	TypeTakeProfit
	// TypeTakeProfitLimit triggers a limit order when price reaches target.
	TypeTakeProfitLimit
)

// String returns the string representation of the order type.
func (t OrderType) String() string {
	return [...]string{"MARKET", "LIMIT", "STOP_LOSS", "STOP_LOSS_LIMIT", "TAKE_PROFIT", "TAKE_PROFIT_LIMIT"}[t]
}

// MarshalJSON implements json.Marshaler for OrderType.
func (t OrderType) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler for OrderType.
// It accepts both uppercase and lowercase formats.
func (t *OrderType) UnmarshalJSON(data []byte) error {
	str := string(data)
	switch str {
	case `"MARKET"`, `"market"`:
		*t = TypeMarket
	case `"LIMIT"`, `"limit"`:
		*t = TypeLimit
	case `"STOP_LOSS"`, `"stop_loss"`:
		*t = TypeStopLoss
	case `"STOP_LOSS_LIMIT"`, `"stop_loss_limit"`:
		*t = TypeStopLossLimit
	case `"TAKE_PROFIT"`, `"take_profit"`:
		*t = TypeTakeProfit
	case `"TAKE_PROFIT_LIMIT"`, `"take_profit_limit"`:
		*t = TypeTakeProfitLimit
	}
	return nil
}

// OrderStatus represents the current state of an order.
type OrderStatus int

// Order status constants define the lifecycle state of an order.
const (
	// StatusNew indicates the order has been accepted by the exchange.
	StatusNew OrderStatus = iota
	// StatusPartiallyFilled indicates the order has been partially filled.
	StatusPartiallyFilled
	// StatusFilled indicates the order has been completely filled.
	StatusFilled
	// StatusCanceling indicates a cancel request has been submitted.
	StatusCanceling
	// StatusCanceled indicates the order has been canceled.
	StatusCanceled
	// StatusRejected indicates the order was rejected by the exchange.
	StatusRejected
	// StatusExpired indicates the order has expired.
	StatusExpired
)

// String returns the string representation of the order status.
func (s OrderStatus) String() string {
	return [...]string{"NEW", "PARTIALLY_FILLED", "FILLED", "CANCELING", "CANCELED", "REJECTED", "EXPIRED"}[s]
}

// IsTerminal returns true if the order is in a terminal state (no further changes possible).
func (s OrderStatus) IsTerminal() bool {
	return s == StatusFilled || s == StatusCanceled || s == StatusRejected || s == StatusExpired
}

// MarshalJSON implements json.Marshaler for OrderStatus.
func (s OrderStatus) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler for OrderStatus.
// It accepts both uppercase and lowercase formats.
func (s *OrderStatus) UnmarshalJSON(data []byte) error {
	str := string(data)
	switch str {
	case `"NEW"`, `"new"`:
		*s = StatusNew
	case `"PARTIALLY_FILLED"`, `"partially_filled"`:
		*s = StatusPartiallyFilled
	case `"FILLED"`, `"filled"`:
		*s = StatusFilled
	case `"CANCELING"`, `"canceling"`:
		*s = StatusCanceling
	case `"CANCELED"`, `"canceled"`:
		*s = StatusCanceled
	case `"REJECTED"`, `"rejected"`:
		*s = StatusRejected
	case `"EXPIRED"`, `"expired"`:
		*s = StatusExpired
	}
	return nil
}

// TimeInForce defines how long an order remains active.
type TimeInForce int

// Time in force constants define order lifetime behavior.
const (
	// GTC (Good Till Canceled) keeps the order active until filled or canceled.
	GTC TimeInForce = iota
	// IOC (Immediate Or Cancel) requires immediate execution; unfilled portion is canceled.
	IOC
	// FOK (Fill Or Kill) requires complete immediate execution or cancellation.
	FOK
)

// String returns the string representation of time in force.
func (t TimeInForce) String() string {
	return [...]string{"GTC", "IOC", "FOK"}[t]
}

// MarshalJSON implements json.Marshaler for TimeInForce.
func (t TimeInForce) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler for TimeInForce.
// It accepts both uppercase and lowercase formats.
func (t *TimeInForce) UnmarshalJSON(data []byte) error {
	str := string(data)
	switch str {
	case `"GTC"`, `"gtc"`:
		*t = GTC
	case `"IOC"`, `"ioc"`:
		*t = IOC
	case `"FOK"`, `"fok"`:
		*t = FOK
	}
	return nil
}

// Ticker represents real-time market data for a trading pair.
// It contains current bid/ask prices, 24-hour high/low, and volume statistics.
type Ticker struct {
	// Symbol is the trading pair identifier (e.g., "BTC/USDT").
	Symbol string `json:"symbol"`
	// Bid is the highest price a buyer is willing to pay.
	Bid apd.Decimal `json:"bid"`
	// Ask is the lowest price a seller is willing to accept.
	Ask apd.Decimal `json:"ask"`
	// Last is the price of the most recent trade.
	Last apd.Decimal `json:"last"`
	// High is the highest price in the last 24 hours.
	High apd.Decimal `json:"high"`
	// Low is the lowest price in the last 24 hours.
	Low apd.Decimal `json:"low"`
	// Volume is the total trading volume in the last 24 hours.
	Volume apd.Decimal `json:"volume"`
	// Timestamp is when this ticker data was generated.
	Timestamp time.Time `json:"timestamp"`
}

// Order represents an exchange order with all its details.
// It tracks the order from submission through execution to completion.
type Order struct {
	// ID is the exchange-assigned order identifier.
	ID string `json:"id"`
	// ClientOrderID is the client-assigned order identifier.
	ClientOrderID string `json:"client_order_id"`
	// Symbol is the trading pair for this order.
	Symbol string `json:"symbol"`
	// Side indicates whether this is a buy or sell order.
	Side OrderSide `json:"side"`
	// Type defines how the order executes (market, limit, etc.).
	Type OrderType `json:"type"`
	// Price is the limit price for limit orders.
	Price apd.Decimal `json:"price"`
	// Quantity is the total order quantity.
	Quantity apd.Decimal `json:"quantity"`
	// FilledQuantity is the amount that has been executed.
	FilledQuantity apd.Decimal `json:"filled_quantity"`
	// RemainingQty is the unfilled portion of the order.
	RemainingQty apd.Decimal `json:"remaining_quantity"`
	// Status is the current state of the order.
	Status OrderStatus `json:"status"`
	// TimeInForce defines how long the order remains active.
	TimeInForce TimeInForce `json:"time_in_force"`
	// CreatedAt is when the order was submitted.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the order was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// Balance represents account balance for a single asset.
type Balance struct {
	// Asset is the currency or token symbol (e.g., "BTC", "USDT").
	Asset string `json:"asset"`
	// Free is the available balance for trading.
	Free apd.Decimal `json:"free"`
	// Locked is the balance locked in open orders.
	Locked apd.Decimal `json:"locked"`
}

// Trade represents a single executed trade from an order.
// An order may result in multiple trades if partially filled over time.
type Trade struct {
	// ID is the exchange-assigned trade identifier.
	ID string `json:"id"`
	// OrderID links this trade to its parent order.
	OrderID string `json:"order_id"`
	// Symbol is the trading pair for this trade.
	Symbol string `json:"symbol"`
	// Side indicates whether this was a buy or sell.
	Side OrderSide `json:"side"`
	// Price is the execution price of this trade.
	Price apd.Decimal `json:"price"`
	// Quantity is the amount executed in this trade.
	Quantity apd.Decimal `json:"quantity"`
	// Fee is the trading fee charged.
	Fee apd.Decimal `json:"fee"`
	// FeeAsset is the currency in which the fee was charged.
	FeeAsset string `json:"fee_asset"`
	// Timestamp is when the trade was executed.
	Timestamp time.Time `json:"timestamp"`
}

// Kline represents a candlestick/OHLCV data point for a time period.
// It contains open, high, low, close prices and volume for the interval.
type Kline struct {
	// Symbol is the trading pair for this kline.
	Symbol string `json:"symbol"`
	// OpenTime is the start of the candlestick period.
	OpenTime time.Time `json:"open_time"`
	// Open is the price at the start of the period.
	Open apd.Decimal `json:"open"`
	// High is the highest price during the period.
	High apd.Decimal `json:"high"`
	// Low is the lowest price during the period.
	Low apd.Decimal `json:"low"`
	// Close is the price at the end of the period.
	Close apd.Decimal `json:"close"`
	// Volume is the total trading volume during the period.
	Volume apd.Decimal `json:"volume"`
	// CloseTime is the end of the candlestick period.
	CloseTime time.Time `json:"close_time"`
	// QuoteVolume is the total value traded in quote currency.
	QuoteVolume apd.Decimal `json:"quote_volume"`
	// NumTrades is the number of trades executed during the period.
	NumTrades int64 `json:"num_trades"`
}

// OrderBookLevel represents a single price level in the order book.
type OrderBookLevel struct {
	// Price is the limit price for this level.
	Price apd.Decimal `json:"price"`
	// Quantity is the total quantity available at this price.
	Quantity apd.Decimal `json:"quantity"`
}

// OrderBook represents the current state of the order book for a trading pair.
// It contains sorted lists of bids (buy orders) and asks (sell orders).
type OrderBook struct {
	// Symbol is the trading pair for this order book.
	Symbol string `json:"symbol"`
	// Bids are buy orders sorted by price descending.
	Bids []OrderBookLevel `json:"bids"`
	// Asks are sell orders sorted by price ascending.
	Asks []OrderBookLevel `json:"asks"`
	// Timestamp is when this snapshot was taken.
	Timestamp time.Time `json:"timestamp"`
}
