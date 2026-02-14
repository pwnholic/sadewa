package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOperation_String(t *testing.T) {
	tests := []struct {
		name string
		op   Operation
		want string
	}{
		{"get_ticker", OpGetTicker, "GET_TICKER"},
		{"get_order_book", OpGetOrderBook, "GET_ORDER_BOOK"},
		{"get_trades", OpGetTrades, "GET_TRADES"},
		{"get_klines", OpGetKlines, "GET_KLINES"},
		{"get_balance", OpGetBalance, "GET_BALANCE"},
		{"place_order", OpPlaceOrder, "PLACE_ORDER"},
		{"cancel_order", OpCancelOrder, "CANCEL_ORDER"},
		{"get_order", OpGetOrder, "GET_ORDER"},
		{"get_open_orders", OpGetOpenOrders, "GET_OPEN_ORDERS"},
		{"get_order_history", OpGetOrderHistory, "GET_ORDER_HISTORY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.op.String())
		})
	}
}

func TestOperation_Count(t *testing.T) {
	ops := []Operation{
		OpGetTicker,
		OpGetOrderBook,
		OpGetTrades,
		OpGetKlines,
		OpGetBalance,
		OpPlaceOrder,
		OpCancelOrder,
		OpGetOrder,
		OpGetOpenOrders,
		OpGetOrderHistory,
	}

	assert.Len(t, ops, 10)
}
