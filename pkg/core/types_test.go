package core

import (
	"testing"

	"github.com/cockroachdb/apd/v3"
	"github.com/stretchr/testify/assert"
)

func TestOrderSide_String(t *testing.T) {
	tests := []struct {
		name string
		side OrderSide
		want string
	}{
		{"buy", SideBuy, "BUY"},
		{"sell", SideSell, "SELL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.side.String())
		})
	}
}

func TestOrderType_String(t *testing.T) {
	tests := []struct {
		name      string
		orderType OrderType
		want      string
	}{
		{"market", TypeMarket, "MARKET"},
		{"limit", TypeLimit, "LIMIT"},
		{"stop_loss", TypeStopLoss, "STOP_LOSS"},
		{"stop_loss_limit", TypeStopLossLimit, "STOP_LOSS_LIMIT"},
		{"take_profit", TypeTakeProfit, "TAKE_PROFIT"},
		{"take_profit_limit", TypeTakeProfitLimit, "TAKE_PROFIT_LIMIT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.orderType.String())
		})
	}
}

func TestOrderStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status OrderStatus
		want   string
	}{
		{"new", StatusNew, "NEW"},
		{"partially_filled", StatusPartiallyFilled, "PARTIALLY_FILLED"},
		{"filled", StatusFilled, "FILLED"},
		{"canceling", StatusCanceling, "CANCELING"},
		{"canceled", StatusCanceled, "CANCELED"},
		{"rejected", StatusRejected, "REJECTED"},
		{"expired", StatusExpired, "EXPIRED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.String())
		})
	}
}

func TestOrderStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		status   OrderStatus
		expected bool
	}{
		{"new", StatusNew, false},
		{"partially_filled", StatusPartiallyFilled, false},
		{"canceling", StatusCanceling, false},
		{"filled", StatusFilled, true},
		{"canceled", StatusCanceled, true},
		{"rejected", StatusRejected, true},
		{"expired", StatusExpired, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.IsTerminal())
		})
	}
}

func TestTimeInForce_String(t *testing.T) {
	tests := []struct {
		name string
		tif  TimeInForce
		want string
	}{
		{"gtc", GTC, "GTC"},
		{"ioc", IOC, "IOC"},
		{"fok", FOK, "FOK"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.tif.String())
		})
	}
}

func TestTicker_JSON(t *testing.T) {
	ticker := &Ticker{
		Symbol: "BTC/USDT",
	}

	var bid, ask, last, high, low, vol apd.Decimal
	bid.SetString("50000.00")
	ask.SetString("50001.00")
	last.SetString("50000.50")
	high.SetString("51000.00")
	low.SetString("49000.00")
	vol.SetString("1234.56")

	ticker.Bid = bid
	ticker.Ask = ask
	ticker.Last = last
	ticker.High = high
	ticker.Low = low
	ticker.Volume = vol

	assert.Equal(t, "BTC/USDT", ticker.Symbol)
	assert.False(t, ticker.Bid.IsZero())
	assert.False(t, ticker.Ask.IsZero())
}

func TestOrder_JSON(t *testing.T) {
	order := &Order{
		ID:            "12345",
		ClientOrderID: "client-123",
		Symbol:        "BTC/USDT",
		Side:          SideBuy,
		Type:          TypeLimit,
		Status:        StatusNew,
		TimeInForce:   GTC,
	}

	var price, qty, filled apd.Decimal
	price.SetString("50000.00")
	qty.SetString("0.1")
	filled.SetString("0")

	order.Price = price
	order.Quantity = qty
	order.FilledQuantity = filled

	assert.Equal(t, "12345", order.ID)
	assert.Equal(t, SideBuy, order.Side)
	assert.Equal(t, TypeLimit, order.Type)
	assert.Equal(t, StatusNew, order.Status)
}

func TestBalance_JSON(t *testing.T) {
	var free, locked apd.Decimal
	free.SetString("1.5")
	locked.SetString("0.5")

	balance := &Balance{
		Asset:  "BTC",
		Free:   free,
		Locked: locked,
	}

	assert.Equal(t, "BTC", balance.Asset)
	assert.False(t, balance.Free.IsZero())
	assert.False(t, balance.Locked.IsZero())
}

func TestOrderBook(t *testing.T) {
	var price1, qty1, price2, qty2 apd.Decimal
	price1.SetString("50000.00")
	qty1.SetString("1.0")
	price2.SetString("50001.00")
	qty2.SetString("2.0")

	ob := &OrderBook{
		Symbol: "BTC/USDT",
		Bids: []OrderBookLevel{
			{Price: price1, Quantity: qty1},
		},
		Asks: []OrderBookLevel{
			{Price: price2, Quantity: qty2},
		},
	}

	assert.Equal(t, "BTC/USDT", ob.Symbol)
	assert.Len(t, ob.Bids, 1)
	assert.Len(t, ob.Asks, 1)
}
