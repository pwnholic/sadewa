package core

// Operation represents a type of action that can be performed on an exchange.
type Operation int

// Operation constants define all supported exchange operations.
const (
	// OpGetTicker retrieves current market ticker data for a symbol.
	OpGetTicker Operation = iota
	// OpGetOrderBook retrieves the current order book depth.
	OpGetOrderBook
	// OpGetTrades retrieves recent trades for a symbol.
	OpGetTrades
	// OpGetKlines retrieves candlestick/OHLCV data.
	OpGetKlines
	// OpGetBalance retrieves account balance information.
	OpGetBalance
	// OpPlaceOrder submits a new order to the exchange.
	OpPlaceOrder
	// OpCancelOrder cancels an existing order.
	OpCancelOrder
	// OpGetOrder retrieves details of a specific order.
	OpGetOrder
	// OpGetOpenOrders retrieves all open orders.
	OpGetOpenOrders
	// OpGetOrderHistory retrieves historical orders.
	OpGetOrderHistory
)

// String returns the string representation of the operation.
func (o Operation) String() string {
	return [...]string{
		"GET_TICKER",
		"GET_ORDER_BOOK",
		"GET_TRADES",
		"GET_KLINES",
		"GET_BALANCE",
		"PLACE_ORDER",
		"CANCEL_ORDER",
		"GET_ORDER",
		"GET_OPEN_ORDERS",
		"GET_ORDER_HISTORY",
	}[o]
}
