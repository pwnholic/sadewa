package aggregator

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sadewa/pkg/core"
	"sadewa/pkg/session"
)

type mockProtocol struct {
	name            string
	ticker          *core.Ticker
	orderBook       *core.OrderBook
	err             error
	supportedOps    []core.Operation
	mu              sync.Mutex
	callCount       int
	requestedSymbol string
}

func (m *mockProtocol) Name() string {
	return m.name
}

func (m *mockProtocol) Version() string {
	return "1.0"
}

func (m *mockProtocol) BaseURL(sandbox bool) string {
	return "https://api.mock.com"
}

func (m *mockProtocol) BuildRequest(ctx context.Context, op core.Operation, params core.Params) (*core.Request, error) {
	m.mu.Lock()
	m.callCount++
	m.requestedSymbol, _ = params["symbol"].(string)
	m.mu.Unlock()

	if m.err != nil {
		return nil, m.err
	}

	return &core.Request{
		Method: "GET",
		Path:   "/api/v3/ticker/24hr",
		Query:  params,
	}, nil
}

func (m *mockProtocol) ParseResponse(op core.Operation, resp *resty.Response) (any, error) {
	if m.err != nil {
		return nil, m.err
	}

	switch op {
	case core.OpGetTicker:
		if m.ticker != nil {
			return m.ticker, nil
		}
	case core.OpGetOrderBook:
		if m.orderBook != nil {
			return m.orderBook, nil
		}
	}

	return nil, nil
}

func (m *mockProtocol) SignRequest(req *resty.Request, creds core.Credentials) error {
	return nil
}

func (m *mockProtocol) SupportedOperations() []core.Operation {
	if m.supportedOps != nil {
		return m.supportedOps
	}
	return []core.Operation{core.OpGetTicker, core.OpGetOrderBook}
}

func (m *mockProtocol) RateLimits() core.RateLimitConfig {
	return core.RateLimitConfig{RequestsPerSecond: 10}
}

func createTestTicker(bid, ask, volume string, timestamp time.Time) *core.Ticker {
	t := &core.Ticker{
		Symbol:    "BTC/USDT",
		Timestamp: timestamp,
	}
	_, _, _ = t.Bid.SetString(bid)
	_, _, _ = t.Ask.SetString(ask)
	_, _, _ = t.Volume.SetString(volume)
	return t
}

func createTestOrderBook(bids, asks [][2]string, timestamp time.Time) *core.OrderBook {
	ob := &core.OrderBook{
		Symbol:    "BTC/USDT",
		Timestamp: timestamp,
	}

	for _, b := range bids {
		level := core.OrderBookLevel{}
		_, _, _ = level.Price.SetString(b[0])
		_, _, _ = level.Quantity.SetString(b[1])
		ob.Bids = append(ob.Bids, level)
	}

	for _, a := range asks {
		level := core.OrderBookLevel{}
		_, _, _ = level.Price.SetString(a[0])
		_, _, _ = level.Quantity.SetString(a[1])
		ob.Asks = append(ob.Asks, level)
	}

	return ob
}

func TestNewAggregator(t *testing.T) {
	agg := NewAggregator()
	assert.NotNil(t, agg)
	assert.NotNil(t, agg.sessions)
	assert.Empty(t, agg.Sessions())
}

func TestAggregator_AddRemoveSessions(t *testing.T) {
	agg := NewAggregator()

	config := core.DefaultConfig("binance")
	sess, err := session.New(config)
	require.NoError(t, err)
	require.NoError(t, sess.SetProtocol(&mockProtocol{name: "binance"}))

	agg.AddSession("binance", sess)
	assert.Equal(t, []string{"binance"}, agg.Sessions())

	agg.AddSession("coinbase", sess)
	assert.Len(t, agg.Sessions(), 2)

	agg.RemoveSession("binance")
	assert.Equal(t, []string{"coinbase"}, agg.Sessions())
}

func TestBestPriceAlgorithm(t *testing.T) {
	now := time.Now()

	tickers := []TickerResult{
		{
			Exchange: "binance",
			Ticker:   createTestTicker("50000", "50020", "100", now),
		},
		{
			Exchange: "coinbase",
			Ticker:   createTestTicker("50005", "50015", "200", now),
		},
		{
			Exchange: "kraken",
			Ticker:   createTestTicker("49995", "50025", "150", now),
		},
	}

	var bestBid apd.Decimal
	var bestAsk apd.Decimal
	var bidExchange string
	var askExchange string
	hasValidData := false

	for _, result := range tickers {
		if result.Error != nil || result.Ticker == nil {
			continue
		}

		ticker := result.Ticker
		if !hasValidData {
			bestBid = ticker.Bid
			bestAsk = ticker.Ask
			bidExchange = result.Exchange
			askExchange = result.Exchange
			hasValidData = true
			continue
		}

		if ticker.Bid.Cmp(&bestBid) > 0 {
			bestBid = ticker.Bid
			bidExchange = result.Exchange
		}

		if ticker.Ask.Cmp(&bestAsk) < 0 {
			bestAsk = ticker.Ask
			askExchange = result.Exchange
		}
	}

	assert.True(t, hasValidData)

	var expectedBid apd.Decimal
	_, _, _ = expectedBid.SetString("50005")
	assert.Equal(t, 0, bestBid.Cmp(&expectedBid))
	assert.Equal(t, "coinbase", bidExchange)

	var expectedAsk apd.Decimal
	_, _, _ = expectedAsk.SetString("50015")
	assert.Equal(t, 0, bestAsk.Cmp(&expectedAsk))
	assert.Equal(t, "coinbase", askExchange)
}

func TestVWAPCalculation(t *testing.T) {
	now := time.Now()

	orderBooks := []*core.OrderBook{
		createTestOrderBook(
			[][2]string{{"50000", "1"}, {"49900", "2"}},
			[][2]string{{"50100", "1"}, {"50200", "2"}},
			now,
		),
		createTestOrderBook(
			[][2]string{{"50050", "1"}},
			[][2]string{{"50150", "1"}},
			now,
		),
	}

	ctx := apd.Context{
		Precision:   34,
		MaxExponent: apd.MaxExponent,
		MinExponent: apd.MinExponent,
		Rounding:    apd.RoundHalfUp,
	}

	var totalValue apd.Decimal
	var totalVolume apd.Decimal

	for _, ob := range orderBooks {
		for _, level := range ob.Bids {
			var value apd.Decimal
			_, err := ctx.Mul(&value, &level.Price, &level.Quantity)
			require.NoError(t, err)
			_, err = ctx.Add(&totalValue, &totalValue, &value)
			require.NoError(t, err)
			_, err = ctx.Add(&totalVolume, &totalVolume, &level.Quantity)
			require.NoError(t, err)
		}

		for _, level := range ob.Asks {
			var value apd.Decimal
			_, err := ctx.Mul(&value, &level.Price, &level.Quantity)
			require.NoError(t, err)
			_, err = ctx.Add(&totalValue, &totalValue, &value)
			require.NoError(t, err)
			_, err = ctx.Add(&totalVolume, &totalVolume, &level.Quantity)
			require.NoError(t, err)
		}
	}

	var vwap apd.Decimal
	_, err := ctx.Quo(&vwap, &totalValue, &totalVolume)
	require.NoError(t, err)

	var expectedVolume apd.Decimal
	_, _, _ = expectedVolume.SetString("8")
	assert.Equal(t, 0, totalVolume.Cmp(&expectedVolume))

	var expectedValue apd.Decimal
	_, _, _ = expectedValue.SetString("400500")
	assert.Equal(t, 0, totalValue.Cmp(&expectedValue))

	var expectedVWAP apd.Decimal
	_, _, _ = expectedVWAP.SetString("50062.5")
	assert.Equal(t, 0, vwap.Cmp(&expectedVWAP))
}

func TestMergedOrderBookAlgorithm(t *testing.T) {
	now := time.Now()

	orderBooks := []*core.OrderBook{
		createTestOrderBook(
			[][2]string{{"50000", "1"}, {"49900", "2"}},
			[][2]string{{"50100", "1"}, {"50200", "2"}},
			now,
		),
		createTestOrderBook(
			[][2]string{{"50000", "0.5"}, {"49800", "1"}},
			[][2]string{{"50100", "1.5"}, {"50300", "3"}},
			now,
		),
	}

	bidMap := make(map[string]*apd.Decimal)
	askMap := make(map[string]*apd.Decimal)

	for _, ob := range orderBooks {
		for _, level := range ob.Bids {
			priceStr := level.Price.String()
			if existing, ok := bidMap[priceStr]; ok {
				_, err := apd.BaseContext.Add(existing, existing, &level.Quantity)
				require.NoError(t, err)
			} else {
				var qty apd.Decimal
				qty.Set(&level.Quantity)
				bidMap[priceStr] = &qty
			}
		}

		for _, level := range ob.Asks {
			priceStr := level.Price.String()
			if existing, ok := askMap[priceStr]; ok {
				_, err := apd.BaseContext.Add(existing, existing, &level.Quantity)
				require.NoError(t, err)
			} else {
				var qty apd.Decimal
				qty.Set(&level.Quantity)
				askMap[priceStr] = &qty
			}
		}
	}

	assert.Len(t, bidMap, 3)
	assert.Len(t, askMap, 3)

	var expectedMergedQty apd.Decimal
	_, _, _ = expectedMergedQty.SetString("1.5")
	assert.Equal(t, 0, bidMap["50000"].Cmp(&expectedMergedQty))
}

func TestArbitrageDetection(t *testing.T) {
	now := time.Now()

	tickers := []TickerResult{
		{
			Exchange: "binance",
			Ticker:   createTestTicker("50000", "50010", "100", now),
		},
		{
			Exchange: "coinbase",
			Ticker:   createTestTicker("50020", "50030", "200", now),
		},
	}

	ctx := apd.Context{
		Precision:   34,
		MaxExponent: apd.MaxExponent,
		MinExponent: apd.MinExponent,
		Rounding:    apd.RoundHalfUp,
	}

	opportunities := make([]ArbitrageOpportunity, 0)

	for i, buyResult := range tickers {
		for j, sellResult := range tickers {
			if i == j {
				continue
			}

			buyPrice := buyResult.Ticker.Ask
			sellPrice := sellResult.Ticker.Bid

			if buyPrice.IsZero() {
				continue
			}

			var spread apd.Decimal
			_, err := ctx.Sub(&spread, &sellPrice, &buyPrice)
			require.NoError(t, err)

			var spreadPercent apd.Decimal
			var hundred apd.Decimal
			hundred.SetInt64(100)
			_, err = ctx.Mul(&spreadPercent, &spread, &hundred)
			require.NoError(t, err)
			_, err = ctx.Quo(&spreadPercent, &spreadPercent, &buyPrice)
			require.NoError(t, err)

			opportunities = append(opportunities, ArbitrageOpportunity{
				Symbol:        "BTC/USDT",
				BuyExchange:   buyResult.Exchange,
				SellExchange:  sellResult.Exchange,
				BuyPrice:      buyPrice,
				SellPrice:     sellPrice,
				Spread:        spread,
				SpreadPercent: spreadPercent,
			})
		}
	}

	assert.Len(t, opportunities, 2)

	var found bool
	for _, opp := range opportunities {
		if opp.BuyExchange == "binance" && opp.SellExchange == "coinbase" {
			found = true

			var expectedBuyPrice apd.Decimal
			_, _, _ = expectedBuyPrice.SetString("50010")
			assert.Equal(t, 0, opp.BuyPrice.Cmp(&expectedBuyPrice))

			var expectedSellPrice apd.Decimal
			_, _, _ = expectedSellPrice.SetString("50020")
			assert.Equal(t, 0, opp.SellPrice.Cmp(&expectedSellPrice))

			assert.True(t, opp.SpreadPercent.Cmp(&apd.Decimal{}) > 0)
			break
		}
	}
	assert.True(t, found)
}

func TestArbitrageMinimumSpread(t *testing.T) {
	now := time.Now()

	tickers := []TickerResult{
		{
			Exchange: "binance",
			Ticker:   createTestTicker("50000", "50001", "100", now),
		},
		{
			Exchange: "coinbase",
			Ticker:   createTestTicker("50000", "50001", "200", now),
		},
	}

	var minSpreadPercent apd.Decimal
	_, _, err := minSpreadPercent.SetString("5.0")
	require.NoError(t, err)

	ctx := apd.Context{
		Precision:   34,
		MaxExponent: apd.MaxExponent,
		MinExponent: apd.MinExponent,
		Rounding:    apd.RoundHalfUp,
	}

	opportunities := make([]ArbitrageOpportunity, 0)

	for i, buyResult := range tickers {
		for j, sellResult := range tickers {
			if i == j {
				continue
			}

			buyPrice := buyResult.Ticker.Ask
			sellPrice := sellResult.Ticker.Bid

			if buyPrice.IsZero() {
				continue
			}

			var spread apd.Decimal
			_, err := ctx.Sub(&spread, &sellPrice, &buyPrice)
			require.NoError(t, err)

			var spreadPercent apd.Decimal
			var hundred apd.Decimal
			hundred.SetInt64(100)
			_, err = ctx.Mul(&spreadPercent, &spread, &hundred)
			require.NoError(t, err)
			_, err = ctx.Quo(&spreadPercent, &spreadPercent, &buyPrice)
			require.NoError(t, err)

			if spreadPercent.Cmp(&minSpreadPercent) < 0 {
				continue
			}

			opportunities = append(opportunities, ArbitrageOpportunity{
				Symbol:        "BTC/USDT",
				BuyExchange:   buyResult.Exchange,
				SellExchange:  sellResult.Exchange,
				BuyPrice:      buyPrice,
				SellPrice:     sellPrice,
				Spread:        spread,
				SpreadPercent: spreadPercent,
			})
		}
	}

	assert.Empty(t, opportunities)
}

func TestGetStats(t *testing.T) {
	agg := NewAggregator()

	stats := agg.GetStats()
	assert.Equal(t, 0, stats.TotalExchanges)
	assert.Equal(t, 0, stats.ActiveExchanges)

	config := core.DefaultConfig("binance")
	sess, err := session.New(config)
	require.NoError(t, err)
	require.NoError(t, sess.SetProtocol(&mockProtocol{name: "binance"}))
	agg.AddSession("binance", sess)

	stats = agg.GetStats()
	assert.Equal(t, 1, stats.TotalExchanges)
	assert.Equal(t, 1, stats.ActiveExchanges)
	assert.False(t, stats.LastUpdate.IsZero())
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	select {
	case <-ctx.Done():
		assert.Error(t, ctx.Err())
	default:
		t.Error("context should be cancelled")
	}
}

func TestSpreadCalculation(t *testing.T) {
	var bid, ask apd.Decimal
	_, _, _ = bid.SetString("50000")
	_, _, _ = ask.SetString("50010")

	ctx := apd.Context{
		Precision:   34,
		MaxExponent: apd.MaxExponent,
		MinExponent: apd.MinExponent,
		Rounding:    apd.RoundHalfUp,
	}

	var spread apd.Decimal
	_, err := ctx.Sub(&spread, &ask, &bid)
	require.NoError(t, err)

	var spreadPercent apd.Decimal
	var hundred apd.Decimal
	hundred.SetInt64(100)
	_, err = ctx.Mul(&spreadPercent, &spread, &hundred)
	require.NoError(t, err)
	_, err = ctx.Quo(&spreadPercent, &spreadPercent, &bid)
	require.NoError(t, err)

	var expectedSpread apd.Decimal
	_, _, _ = expectedSpread.SetString("10")
	assert.Equal(t, 0, spread.Cmp(&expectedSpread))

	var expectedSpreadPercent apd.Decimal
	_, _, _ = expectedSpreadPercent.SetString("0.02")
	assert.Equal(t, 0, spreadPercent.Cmp(&expectedSpreadPercent))
}

func TestPriceComparisonMaxSpread(t *testing.T) {
	now := time.Now()

	tickers := []TickerResult{
		{
			Exchange: "binance",
			Ticker:   createTestTicker("50000", "50020", "100", now),
		},
		{
			Exchange: "coinbase",
			Ticker:   createTestTicker("50005", "50015", "200", now),
		},
	}

	var maxSpread apd.Decimal

	for _, result := range tickers {
		if result.Error != nil || result.Ticker == nil {
			continue
		}

		ticker := result.Ticker
		var spread apd.Decimal
		_, err := apd.BaseContext.Sub(&spread, &ticker.Ask, &ticker.Bid)
		require.NoError(t, err)

		if spread.Cmp(&maxSpread) > 0 {
			maxSpread = spread
		}
	}

	var expectedMaxSpread apd.Decimal
	_, _, _ = expectedMaxSpread.SetString("20")
	assert.Equal(t, 0, maxSpread.Cmp(&expectedMaxSpread))
}
