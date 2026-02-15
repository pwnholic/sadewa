package bybit

import (
	"testing"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sadewa/pkg/core"
)

func TestNewNormalizer(t *testing.T) {
	n := NewNormalizer()
	assert.NotNil(t, n)
}

func TestNormalizeTicker(t *testing.T) {
	n := NewNormalizer()

	data := &bybitTicker{
		Symbol:      "BTCUSDT",
		LastPrice:   "50000.50",
		HighPrice:   "51000.00",
		LowPrice:    "49000.00",
		Volume:      "1000000.5",
		Bid1Price:   "50000.00",
		Ask1Price:   "50001.00",
		PriceChange: "0.05",
	}

	ticker := n.NormalizeTicker(data)

	assert.Equal(t, "BTC/USDT", ticker.Symbol)
	assert.Equal(t, "50000.00", ticker.Bid.String())
	assert.Equal(t, "50001.00", ticker.Ask.String())
	assert.Equal(t, "50000.50", ticker.Last.String())
	assert.Equal(t, "51000.00", ticker.High.String())
	assert.Equal(t, "49000.00", ticker.Low.String())
	assert.Equal(t, "1000000.5", ticker.Volume.String())
	assert.NotZero(t, ticker.Timestamp)
}

func TestNormalizeOrder(t *testing.T) {
	n := NewNormalizer()

	data := &bybitOrder{
		OrderID:     "123456789",
		OrderLinkID: "my-order-1",
		Symbol:      "BTCUSDT",
		Side:        "Buy",
		OrderType:   "Limit",
		Price:       "50000.00",
		Qty:         "0.1",
		CumExecQty:  "0.05",
		OrderStatus: "PartiallyFilled",
		TimeInForce: "GTC",
		CreatedTime: "1640000000000",
		UpdatedTime: "1640000100000",
	}

	order, err := n.NormalizeOrder(data)
	require.NoError(t, err)

	assert.Equal(t, "123456789", order.ID)
	assert.Equal(t, "my-order-1", order.ClientOrderID)
	assert.Equal(t, "BTC/USDT", order.Symbol)
	assert.Equal(t, core.SideBuy, order.Side)
	assert.Equal(t, core.TypeLimit, order.Type)
	assert.Equal(t, "50000.00", order.Price.String())
	assert.Equal(t, "0.1", order.Quantity.String())
	assert.Equal(t, "0.05", order.FilledQuantity.String())
	assert.Equal(t, "0.05", order.RemainingQty.String())
	assert.Equal(t, core.StatusPartiallyFilled, order.Status)
	assert.Equal(t, core.GTC, order.TimeInForce)
	assert.NotZero(t, order.CreatedAt)
	assert.NotZero(t, order.UpdatedAt)
}

func TestNormalizeOrder_Filled(t *testing.T) {
	n := NewNormalizer()

	data := &bybitOrder{
		OrderID:     "123456789",
		Symbol:      "BTCUSDT",
		Side:        "Sell",
		OrderType:   "Market",
		Qty:         "0.1",
		CumExecQty:  "0.1",
		OrderStatus: "Filled",
		TimeInForce: "IOC",
	}

	order, err := n.NormalizeOrder(data)
	require.NoError(t, err)

	assert.Equal(t, core.SideSell, order.Side)
	assert.Equal(t, core.TypeMarket, order.Type)
	assert.Equal(t, core.StatusFilled, order.Status)
	assert.Equal(t, core.IOC, order.TimeInForce)
	assert.True(t, order.RemainingQty.IsZero())
}

func TestNormalizeBalance(t *testing.T) {
	n := NewNormalizer()

	data := &bybitBalance{
		Coin:          "BTC",
		WalletBalance: "1.5",
		Free:          "1.0",
		Locked:        "0.5",
	}

	balance := n.NormalizeBalance(data)

	assert.Equal(t, "BTC", balance.Asset)
	assert.Equal(t, "1.0", balance.Free.String())
	assert.Equal(t, "0.5", balance.Locked.String())
}

func TestNormalizeBalances(t *testing.T) {
	n := NewNormalizer()

	account := &bybitAccount{
		List: []struct {
			Coin []bybitBalance `json:"coin"`
		}{
			{
				Coin: []bybitBalance{
					{Coin: "BTC", Free: "1.0", Locked: "0.5"},
					{Coin: "ETH", Free: "10.0", Locked: "2.0"},
					{Coin: "USDT", Free: "50000", Locked: "1000"},
				},
			},
		},
	}

	balances := n.NormalizeBalances(account)

	require.Len(t, balances, 3)
	assert.Equal(t, "BTC", balances[0].Asset)
	assert.Equal(t, "ETH", balances[1].Asset)
	assert.Equal(t, "USDT", balances[2].Asset)
}

func TestNormalizeTrade(t *testing.T) {
	n := NewNormalizer()

	data := &bybitTrade{
		ExecID:  "trade-123",
		Symbol:  "BTCUSDT",
		Side:    "Buy",
		Price:   "50000.00",
		Qty:     "0.1",
		Time:    "1640000000000",
		IsMaker: true,
	}

	trade := n.NormalizeTrade(data)

	assert.Equal(t, "trade-123", trade.ID)
	assert.Equal(t, "BTC/USDT", trade.Symbol)
	assert.Equal(t, core.SideBuy, trade.Side)
	assert.Equal(t, "50000.00", trade.Price.String())
	assert.Equal(t, "0.1", trade.Quantity.String())
	assert.NotZero(t, trade.Timestamp)
}

func TestNormalizeMyTrade(t *testing.T) {
	n := NewNormalizer()

	data := &bybitMyTrade{
		ExecID:        "exec-456",
		OrderID:       "order-789",
		OrderLinkID:   "my-order",
		Symbol:        "BTCUSDT",
		Side:          "Sell",
		Price:         "51000.00",
		Qty:           "0.05",
		Fee:           "0.0001",
		FeeCurrencyID: "BTC",
		ExecTime:      "1640000000000",
	}

	trade := n.NormalizeMyTrade(data)

	assert.Equal(t, "exec-456", trade.ID)
	assert.Equal(t, "order-789", trade.OrderID)
	assert.Equal(t, "BTC/USDT", trade.Symbol)
	assert.Equal(t, core.SideSell, trade.Side)
	assert.Equal(t, "51000.00", trade.Price.String())
	assert.Equal(t, "0.05", trade.Quantity.String())
	assert.Equal(t, "0.0001", trade.Fee.String())
	assert.Equal(t, "BTC", trade.FeeAsset)
	assert.NotZero(t, trade.Timestamp)
}

func TestNormalizeKline(t *testing.T) {
	n := NewNormalizer()

	data := &bybitKline{
		Start:    1640000000000,
		End:      1640003600000,
		Interval: "60",
		Open:     "50000.00",
		High:     "51000.00",
		Low:      "49000.00",
		Close:    "50500.00",
		Volume:   "1000.5",
		Turnover: "50000000",
	}

	kline, err := n.NormalizeKline(data, "BTC/USDT")
	require.NoError(t, err)

	assert.Equal(t, "BTC/USDT", kline.Symbol)
	assert.Equal(t, time.UnixMilli(1640000000000), kline.OpenTime)
	assert.Equal(t, time.UnixMilli(1640003600000), kline.CloseTime)
	assert.Equal(t, "50000.00", kline.Open.String())
	assert.Equal(t, "51000.00", kline.High.String())
	assert.Equal(t, "49000.00", kline.Low.String())
	assert.Equal(t, "50500.00", kline.Close.String())
	assert.Equal(t, "1000.5", kline.Volume.String())
	assert.Equal(t, "50000000", kline.QuoteVolume.String())
}

func TestNormalizeKlines(t *testing.T) {
	n := NewNormalizer()

	data := []bybitKline{
		{
			Start:    1640000000000,
			End:      1640003600000,
			Open:     "50000.00",
			High:     "51000.00",
			Low:      "49000.00",
			Close:    "50500.00",
			Volume:   "1000.5",
			Turnover: "50000000",
		},
		{
			Start:    1640003600000,
			End:      1640007200000,
			Open:     "50500.00",
			High:     "51500.00",
			Low:      "50000.00",
			Close:    "51000.00",
			Volume:   "1200.5",
			Turnover: "60000000",
		},
	}

	klines, err := n.NormalizeKlines(data, "BTC/USDT")
	require.NoError(t, err)
	require.Len(t, klines, 2)

	assert.Equal(t, "BTC/USDT", klines[0].Symbol)
	assert.Equal(t, "BTC/USDT", klines[1].Symbol)
}

func TestNormalizeOrderBook(t *testing.T) {
	n := NewNormalizer()

	data := &bybitOrderBook{
		Symbol: "BTCUSDT",
		Bids:   [][]string{{"50000.00", "1.5"}, {"49999.00", "2.0"}},
		Asks:   [][]string{{"50001.00", "1.0"}, {"50002.00", "0.5"}},
		Time:   1640000000000,
	}

	orderBook, err := n.NormalizeOrderBook(data, "BTC/USDT")
	require.NoError(t, err)

	assert.Equal(t, "BTC/USDT", orderBook.Symbol)
	require.Len(t, orderBook.Bids, 2)
	require.Len(t, orderBook.Asks, 2)

	assert.Equal(t, "50000.00", orderBook.Bids[0].Price.String())
	assert.Equal(t, "1.5", orderBook.Bids[0].Quantity.String())
	assert.Equal(t, "50001.00", orderBook.Asks[0].Price.String())
	assert.Equal(t, "1.0", orderBook.Asks[0].Quantity.String())
}

func TestNormalizeTrades(t *testing.T) {
	n := NewNormalizer()

	data := []bybitTrade{
		{ExecID: "1", Side: "Buy", Price: "50000", Qty: "0.1", Time: "1640000000000"},
		{ExecID: "2", Side: "Sell", Price: "50100", Qty: "0.2", Time: "1640000001000"},
	}

	trades := n.NormalizeTrades(data, "BTC/USDT")

	require.Len(t, trades, 2)
	assert.Equal(t, "BTC/USDT", trades[0].Symbol)
	assert.Equal(t, "BTC/USDT", trades[1].Symbol)
	assert.Equal(t, "1", trades[0].ID)
	assert.Equal(t, "2", trades[1].ID)
}

func TestNormalizeOrders(t *testing.T) {
	n := NewNormalizer()

	data := []bybitOrder{
		{OrderID: "1", Symbol: "BTCUSDT", Side: "Buy", OrderType: "Limit", Qty: "0.1", OrderStatus: "New"},
		{OrderID: "2", Symbol: "ETHUSDT", Side: "Sell", OrderType: "Market", Qty: "1.0", OrderStatus: "Filled"},
	}

	orders, err := n.NormalizeOrders(data)
	require.NoError(t, err)
	require.Len(t, orders, 2)

	assert.Equal(t, "1", orders[0].ID)
	assert.Equal(t, "2", orders[1].ID)
	assert.Equal(t, "BTC/USDT", orders[0].Symbol)
	assert.Equal(t, "ETH/USDT", orders[1].Symbol)
}

func TestDenormalizeOrder(t *testing.T) {
	n := NewNormalizer()

	var price, qty apd.Decimal
	_, _, _ = apd.BaseContext.SetString(&price, "50000.00")
	_, _, _ = apd.BaseContext.SetString(&qty, "0.1")

	order := &core.Order{
		Symbol:        "BTC/USDT",
		Side:          core.SideBuy,
		Type:          core.TypeLimit,
		Price:         price,
		Quantity:      qty,
		TimeInForce:   core.GTC,
		ClientOrderID: "my-order-1",
	}

	params := n.DenormalizeOrder(order)

	assert.Equal(t, "BTCUSDT", params.Symbol)
	assert.Equal(t, "BUY", params.Side)
	assert.Equal(t, "LIMIT", params.Type)
	assert.Equal(t, "0.1", params.Quantity)
	assert.Equal(t, "50000.00", params.Price)
	assert.Equal(t, "my-order-1", params.ClientOrderID)
}

func TestParseDecimal(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"50000.50", "50000.50"},
		{"0.1", "0.1"},
		{"1000000", "1000000"},
		{"", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var dest apd.Decimal
			err := parseDecimal(&dest, tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, dest.String())
		})
	}
}

func TestParseBybitOrderSide(t *testing.T) {
	tests := []struct {
		input    string
		expected core.OrderSide
	}{
		{"Buy", core.SideBuy},
		{"Sell", core.SideSell},
		{"unknown", core.SideBuy},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBybitOrderSide(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseBybitOrderType(t *testing.T) {
	tests := []struct {
		input    string
		expected core.OrderType
	}{
		{"Market", core.TypeMarket},
		{"Limit", core.TypeLimit},
		{"unknown", core.TypeMarket},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBybitOrderType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseBybitOrderStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected core.OrderStatus
	}{
		{"New", core.StatusNew},
		{"Created", core.StatusNew},
		{"PartiallyFilled", core.StatusPartiallyFilled},
		{"Filled", core.StatusFilled},
		{"Cancelled", core.StatusCanceled},
		{"Rejected", core.StatusRejected},
		{"Deactivated", core.StatusExpired},
		{"unknown", core.StatusNew},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBybitOrderStatus(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseBybitTimeInForce(t *testing.T) {
	tests := []struct {
		input    string
		expected core.TimeInForce
	}{
		{"GTC", core.GTC},
		{"IOC", core.IOC},
		{"FOK", core.FOK},
		{"unknown", core.GTC},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBybitTimeInForce(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
