package aggregator

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"sync"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/rs/zerolog"

	"sadewa/pkg/core"
	"sadewa/pkg/exchange"
	"sadewa/pkg/session"
)

// Aggregator combines market data from multiple exchange sessions.
type Aggregator struct {
	mu         sync.RWMutex
	sessions   map[string]*session.Session
	logger     zerolog.Logger
	lastUpdate time.Time
}

// NewAggregator creates a new aggregator with no sessions registered.
func NewAggregator() *Aggregator {
	return &Aggregator{
		sessions: make(map[string]*session.Session),
		logger:   zerolog.Nop(),
	}
}

// NewAggregatorWithLogger creates a new aggregator with a custom logger.
func NewAggregatorWithLogger(logger zerolog.Logger) *Aggregator {
	return &Aggregator{
		sessions: make(map[string]*session.Session),
		logger:   logger,
	}
}

// AddSession registers a trading session with the aggregator under the given name.
func (a *Aggregator) AddSession(name string, sess *session.Session) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions[name] = sess
	a.lastUpdate = time.Now()
}

// RemoveSession unregisters a trading session from the aggregator.
func (a *Aggregator) RemoveSession(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, name)
	a.lastUpdate = time.Now()
}

// Sessions returns the names of all registered sessions in sorted order.
func (a *Aggregator) Sessions() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	names := make([]string, 0, len(a.sessions))
	for name := range a.sessions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TickerResult holds the ticker data or error from a single exchange.
type TickerResult struct {
	// Exchange is the name of the exchange that provided this result.
	Exchange string `json:"exchange"`
	// Ticker contains the market data, or nil if an error occurred.
	Ticker *core.Ticker `json:"ticker,omitempty"`
	// Error contains any error that occurred while fetching the ticker.
	Error error `json:"error,omitempty"`
}

// GetTickers fetches tickers for a symbol from all registered exchanges concurrently.
func (a *Aggregator) GetTickers(ctx context.Context, symbol string) []TickerResult {
	a.mu.RLock()
	sessions := make(map[string]*session.Session, len(a.sessions))
	maps.Copy(sessions, a.sessions)
	a.mu.RUnlock()

	results := make([]TickerResult, 0, len(sessions))
	resultChan := make(chan TickerResult, len(sessions))
	var wg sync.WaitGroup

	for name, sess := range sessions {
		wg.Add(1)
		go func(exchangeName string, s *session.Session) {
			defer wg.Done()

			result := TickerResult{Exchange: exchangeName}

			select {
			case <-ctx.Done():
				result.Error = ctx.Err()
				resultChan <- result
				return
			default:
			}

			ticker, err := s.GetTicker(ctx, symbol)
			if err != nil {
				result.Error = fmt.Errorf("get ticker: %w", err)
				resultChan <- result
				return
			}

			result.Ticker = ticker
			resultChan <- result
		}(name, sess)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	for r := range resultChan {
		results = append(results, r)
	}

	a.mu.Lock()
	a.lastUpdate = time.Now()
	a.mu.Unlock()

	return results
}

// BestPrice represents the best bid and ask prices across all exchanges.
type BestPrice struct {
	// Symbol is the trading pair symbol.
	Symbol string `json:"symbol"`
	// Bid is the highest bid price found across all exchanges.
	Bid apd.Decimal `json:"bid"`
	// Ask is the lowest ask price found across all exchanges.
	Ask apd.Decimal `json:"ask"`
	// BidExchange is the name of the exchange with the best bid.
	BidExchange string `json:"bid_exchange"`
	// AskExchange is the name of the exchange with the best ask.
	AskExchange string `json:"ask_exchange"`
	// Spread is the difference between the best ask and best bid.
	Spread apd.Decimal `json:"spread"`
	// SpreadPercent is the spread as a percentage of the bid price.
	SpreadPercent apd.Decimal `json:"spread_percent"`
	// Timestamp is the most recent update time from any exchange.
	Timestamp time.Time `json:"timestamp"`
}

// GetBestPrice finds the highest bid and lowest ask for a symbol across all exchanges.
func (a *Aggregator) GetBestPrice(ctx context.Context, symbol string) (*BestPrice, error) {
	tickers := a.GetTickers(ctx, symbol)

	var bestBid apd.Decimal
	var bestAsk apd.Decimal
	var bidExchange string
	var askExchange string
	var timestamp time.Time
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
			timestamp = ticker.Timestamp
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

		if ticker.Timestamp.After(timestamp) {
			timestamp = ticker.Timestamp
		}
	}

	if !hasValidData {
		return nil, fmt.Errorf("no valid ticker data available for symbol: %s", symbol)
	}

	var spread apd.Decimal
	_, err := apd.BaseContext.Sub(&spread, &bestAsk, &bestBid)
	if err != nil {
		return nil, fmt.Errorf("calculate spread: %w", err)
	}

	var spreadPercent apd.Decimal
	if !bestBid.IsZero() {
		var hundred apd.Decimal
		hundred.SetInt64(100)
		_, err = apd.BaseContext.Mul(&spreadPercent, &spread, &hundred)
		if err != nil {
			return nil, fmt.Errorf("calculate spread percent multiply: %w", err)
		}
		_, err = apd.BaseContext.Quo(&spreadPercent, &spreadPercent, &bestBid)
		if err != nil {
			return nil, fmt.Errorf("calculate spread percent divide: %w", err)
		}
	}

	return &BestPrice{
		Symbol:        symbol,
		Bid:           bestBid,
		Ask:           bestAsk,
		BidExchange:   bidExchange,
		AskExchange:   askExchange,
		Spread:        spread,
		SpreadPercent: spreadPercent,
		Timestamp:     timestamp,
	}, nil
}

// VWAPResult holds the volume-weighted average price calculation results.
type VWAPResult struct {
	// Symbol is the trading pair symbol.
	Symbol string `json:"symbol"`
	// VWAP is the calculated volume-weighted average price.
	VWAP apd.Decimal `json:"vwap"`
	// Volume is the total volume included in the calculation.
	Volume apd.Decimal `json:"volume"`
	// NumTrades is the count of order book levels included.
	NumTrades int `json:"num_trades"`
	// Exchanges lists the names of exchanges that contributed data.
	Exchanges []string `json:"exchanges"`
}

// GetVWAP calculates the volume-weighted average price across all exchanges.
// The depth parameter limits the number of order book levels to include per exchange.
func (a *Aggregator) GetVWAP(ctx context.Context, symbol string, depth int) (*VWAPResult, error) {
	a.mu.RLock()
	sessions := make(map[string]*session.Session, len(a.sessions))
	maps.Copy(sessions, a.sessions)
	a.mu.RUnlock()

	type orderBookResult struct {
		exchange string
		ob       *core.OrderBook
		err      error
	}

	resultChan := make(chan orderBookResult, len(sessions))
	var wg sync.WaitGroup

	opts := make([]exchange.Option, 0)
	if depth > 0 {
		opts = append(opts, exchange.WithLimit(depth))
	}

	for name, sess := range sessions {
		wg.Add(1)
		go func(exchangeName string, s *session.Session) {
			defer wg.Done()

			result := orderBookResult{exchange: exchangeName}

			select {
			case <-ctx.Done():
				result.err = ctx.Err()
				resultChan <- result
				return
			default:
			}

			ob, err := s.GetOrderBook(ctx, symbol, opts...)
			if err != nil {
				result.err = fmt.Errorf("get order book: %w", err)
				resultChan <- result
				return
			}

			result.ob = ob
			resultChan <- result
		}(name, sess)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var totalValue apd.Decimal
	var totalVolume apd.Decimal
	numTrades := 0
	exchanges := make([]string, 0)

	for r := range resultChan {
		if r.err != nil || r.ob == nil {
			continue
		}

		exchanges = append(exchanges, r.exchange)

		for _, level := range r.ob.Bids {
			var value apd.Decimal
			_, err := apd.BaseContext.Mul(&value, &level.Price, &level.Quantity)
			if err != nil {
				continue
			}
			_, err = apd.BaseContext.Add(&totalValue, &totalValue, &value)
			if err != nil {
				continue
			}
			_, err = apd.BaseContext.Add(&totalVolume, &totalVolume, &level.Quantity)
			if err != nil {
				continue
			}
			numTrades++
		}

		for _, level := range r.ob.Asks {
			var value apd.Decimal
			_, err := apd.BaseContext.Mul(&value, &level.Price, &level.Quantity)
			if err != nil {
				continue
			}
			_, err = apd.BaseContext.Add(&totalValue, &totalValue, &value)
			if err != nil {
				continue
			}
			_, err = apd.BaseContext.Add(&totalVolume, &totalVolume, &level.Quantity)
			if err != nil {
				continue
			}
			numTrades++
		}
	}

	if totalVolume.IsZero() {
		return nil, fmt.Errorf("no valid order book data available for symbol: %s", symbol)
	}

	var vwap apd.Decimal
	_, err := apd.BaseContext.Quo(&vwap, &totalValue, &totalVolume)
	if err != nil {
		return nil, fmt.Errorf("calculate vwap: %w", err)
	}

	a.mu.Lock()
	a.lastUpdate = time.Now()
	a.mu.Unlock()

	return &VWAPResult{
		Symbol:    symbol,
		VWAP:      vwap,
		Volume:    totalVolume,
		NumTrades: numTrades,
		Exchanges: exchanges,
	}, nil
}

// MergedOrderBook combines order book levels from multiple exchanges.
type MergedOrderBook struct {
	// Symbol is the trading pair symbol.
	Symbol string `json:"symbol"`
	// Bids are aggregated bid levels sorted by price descending.
	Bids []core.OrderBookLevel `json:"bids"`
	// Asks are aggregated ask levels sorted by price ascending.
	Asks []core.OrderBookLevel `json:"asks"`
	// Timestamp is the most recent update time from any exchange.
	Timestamp time.Time `json:"timestamp"`
	// Exchanges lists the names of exchanges that contributed data.
	Exchanges []string `json:"exchanges"`
}

// GetMergedOrderBook aggregates order book levels from all exchanges.
// Levels at the same price are combined by summing their quantities.
func (a *Aggregator) GetMergedOrderBook(ctx context.Context, symbol string, depth int) (*MergedOrderBook, error) {
	a.mu.RLock()
	sessions := make(map[string]*session.Session, len(a.sessions))
	maps.Copy(sessions, a.sessions)
	a.mu.RUnlock()

	type orderBookResult struct {
		exchange string
		ob       *core.OrderBook
		err      error
	}

	resultChan := make(chan orderBookResult, len(sessions))
	var wg sync.WaitGroup

	opts := make([]exchange.Option, 0)
	if depth > 0 {
		opts = append(opts, exchange.WithLimit(depth))
	}

	for name, sess := range sessions {
		wg.Add(1)
		go func(exchangeName string, s *session.Session) {
			defer wg.Done()

			result := orderBookResult{exchange: exchangeName}

			select {
			case <-ctx.Done():
				result.err = ctx.Err()
				resultChan <- result
				return
			default:
			}

			ob, err := s.GetOrderBook(ctx, symbol, opts...)
			if err != nil {
				result.err = fmt.Errorf("get order book: %w", err)
				resultChan <- result
				return
			}

			result.ob = ob
			resultChan <- result
		}(name, sess)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	bidMap := make(map[string]*apd.Decimal)
	askMap := make(map[string]*apd.Decimal)
	var timestamp time.Time
	exchanges := make([]string, 0)

	for r := range resultChan {
		if r.err != nil || r.ob == nil {
			continue
		}

		exchanges = append(exchanges, r.exchange)

		if r.ob.Timestamp.After(timestamp) {
			timestamp = r.ob.Timestamp
		}

		for _, level := range r.ob.Bids {
			priceStr := level.Price.String()
			if existing, ok := bidMap[priceStr]; ok {
				_, err := apd.BaseContext.Add(existing, existing, &level.Quantity)
				if err != nil {
					continue
				}
			} else {
				var qty apd.Decimal
				qty.Set(&level.Quantity)
				bidMap[priceStr] = &qty
			}
		}

		for _, level := range r.ob.Asks {
			priceStr := level.Price.String()
			if existing, ok := askMap[priceStr]; ok {
				_, err := apd.BaseContext.Add(existing, existing, &level.Quantity)
				if err != nil {
					continue
				}
			} else {
				var qty apd.Decimal
				qty.Set(&level.Quantity)
				askMap[priceStr] = &qty
			}
		}
	}

	if len(bidMap) == 0 && len(askMap) == 0 {
		return nil, fmt.Errorf("no valid order book data available for symbol: %s", symbol)
	}

	bids := make([]core.OrderBookLevel, 0, len(bidMap))
	for priceStr, qty := range bidMap {
		var price apd.Decimal
		_, _, err := price.SetString(priceStr)
		if err != nil {
			a.logger.Debug().Err(err).Str("price", priceStr).Msg("failed to parse bid price")
			continue
		}
		bids = append(bids, core.OrderBookLevel{
			Price:    price,
			Quantity: *qty,
		})
	}

	asks := make([]core.OrderBookLevel, 0, len(askMap))
	for priceStr, qty := range askMap {
		var price apd.Decimal
		_, _, err := price.SetString(priceStr)
		if err != nil {
			a.logger.Debug().Err(err).Str("price", priceStr).Msg("failed to parse ask price")
			continue
		}
		asks = append(asks, core.OrderBookLevel{
			Price:    price,
			Quantity: *qty,
		})
	}

	sort.Slice(bids, func(i, j int) bool {
		return bids[i].Price.Cmp(&bids[j].Price) > 0
	})

	sort.Slice(asks, func(i, j int) bool {
		return asks[i].Price.Cmp(&asks[j].Price) < 0
	})

	if depth > 0 {
		if len(bids) > depth {
			bids = bids[:depth]
		}
		if len(asks) > depth {
			asks = asks[:depth]
		}
	}

	a.mu.Lock()
	a.lastUpdate = time.Now()
	a.mu.Unlock()

	return &MergedOrderBook{
		Symbol:    symbol,
		Bids:      bids,
		Asks:      asks,
		Timestamp: timestamp,
		Exchanges: exchanges,
	}, nil
}

// ExchangePrice holds bid and ask prices from a single exchange.
type ExchangePrice struct {
	// Exchange is the name of the exchange.
	Exchange string `json:"exchange"`
	// Bid is the current bid price.
	Bid apd.Decimal `json:"bid"`
	// Ask is the current ask price.
	Ask apd.Decimal `json:"ask"`
}

// PriceComparison holds price data from multiple exchanges for comparison.
type PriceComparison struct {
	// Symbol is the trading pair symbol.
	Symbol string `json:"symbol"`
	// Exchanges contains price data from each exchange.
	Exchanges []ExchangePrice `json:"exchanges"`
	// MaxSpread is the largest bid-ask spread across all exchanges.
	MaxSpread apd.Decimal `json:"max_spread"`
}

// ComparePrices fetches and compares prices for a symbol across all exchanges.
func (a *Aggregator) ComparePrices(ctx context.Context, symbol string) (*PriceComparison, error) {
	tickers := a.GetTickers(ctx, symbol)

	exchangePrices := make([]ExchangePrice, 0)
	var maxSpread apd.Decimal

	for _, result := range tickers {
		if result.Error != nil || result.Ticker == nil {
			continue
		}

		ticker := result.Ticker
		exchangePrices = append(exchangePrices, ExchangePrice{
			Exchange: result.Exchange,
			Bid:      ticker.Bid,
			Ask:      ticker.Ask,
		})

		var spread apd.Decimal
		_, err := apd.BaseContext.Sub(&spread, &ticker.Ask, &ticker.Bid)
		if err != nil {
			continue
		}

		if spread.Cmp(&maxSpread) > 0 {
			maxSpread = spread
		}
	}

	if len(exchangePrices) == 0 {
		return nil, fmt.Errorf("no valid ticker data available for symbol: %s", symbol)
	}

	return &PriceComparison{
		Symbol:    symbol,
		Exchanges: exchangePrices,
		MaxSpread: maxSpread,
	}, nil
}

// ArbitrageOpportunity represents a potential cross-exchange arbitrage trade.
type ArbitrageOpportunity struct {
	// Symbol is the trading pair symbol.
	Symbol string `json:"symbol"`
	// BuyExchange is the exchange to buy from (lowest ask).
	BuyExchange string `json:"buy_exchange"`
	// SellExchange is the exchange to sell to (highest bid).
	SellExchange string `json:"sell_exchange"`
	// BuyPrice is the ask price on the buy exchange.
	BuyPrice apd.Decimal `json:"buy_price"`
	// SellPrice is the bid price on the sell exchange.
	SellPrice apd.Decimal `json:"sell_price"`
	// Spread is the difference between sell and buy prices.
	Spread apd.Decimal `json:"spread"`
	// SpreadPercent is the spread as a percentage of the buy price.
	SpreadPercent apd.Decimal `json:"spread_percent"`
	// PotentialProfit is the estimated profit per unit traded.
	PotentialProfit apd.Decimal `json:"potential_profit"`
}

// FindArbitrage identifies arbitrage opportunities where buying on one exchange
// and selling on another yields a spread above the minimum threshold.
func (a *Aggregator) FindArbitrage(ctx context.Context, symbol string, minSpreadPercent apd.Decimal) ([]ArbitrageOpportunity, error) {
	tickers := a.GetTickers(ctx, symbol)

	validTickers := make([]TickerResult, 0)
	for _, result := range tickers {
		if result.Error == nil && result.Ticker != nil {
			validTickers = append(validTickers, result)
		}
	}

	if len(validTickers) < 2 {
		return nil, fmt.Errorf("need at least 2 exchanges with valid data for arbitrage detection")
	}

	opportunities := make([]ArbitrageOpportunity, 0)
	minSpread := minSpreadPercent

	for i, buyResult := range validTickers {
		for j, sellResult := range validTickers {
			if i == j {
				continue
			}

			buyPrice := buyResult.Ticker.Ask
			sellPrice := sellResult.Ticker.Bid

			if buyPrice.IsZero() {
				continue
			}

			var spread apd.Decimal
			_, err := apd.BaseContext.Sub(&spread, &sellPrice, &buyPrice)
			if err != nil {
				continue
			}

			var spreadPercent apd.Decimal
			var hundred apd.Decimal
			hundred.SetInt64(100)
			_, err = apd.BaseContext.Mul(&spreadPercent, &spread, &hundred)
			if err != nil {
				continue
			}
			_, err = apd.BaseContext.Quo(&spreadPercent, &spreadPercent, &buyPrice)
			if err != nil {
				continue
			}

			if spreadPercent.Cmp(&minSpread) < 0 {
				continue
			}

			opportunities = append(opportunities, ArbitrageOpportunity{
				Symbol:        symbol,
				BuyExchange:   buyResult.Exchange,
				SellExchange:  sellResult.Exchange,
				BuyPrice:      buyPrice,
				SellPrice:     sellPrice,
				Spread:        spread,
				SpreadPercent: spreadPercent,
			})
		}
	}

	sort.Slice(opportunities, func(i, j int) bool {
		return opportunities[i].SpreadPercent.Cmp(&opportunities[j].SpreadPercent) > 0
	})

	return opportunities, nil
}

// AggregateStats contains statistics about the aggregator's registered sessions.
type AggregateStats struct {
	// TotalExchanges is the count of all registered sessions.
	TotalExchanges int `json:"total_exchanges"`
	// ActiveExchanges is the count of sessions in active state.
	ActiveExchanges int `json:"active_exchanges"`
	// LastUpdate is the timestamp of the most recent data refresh.
	LastUpdate time.Time `json:"last_update"`
}

// GetStats returns statistics about the aggregator's current state.
func (a *Aggregator) GetStats() *AggregateStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	total := len(a.sessions)
	active := 0

	for _, sess := range a.sessions {
		if sess.State() == session.SessionStateActive {
			active++
		}
	}

	return &AggregateStats{
		TotalExchanges:  total,
		ActiveExchanges: active,
		LastUpdate:      a.lastUpdate,
	}
}
