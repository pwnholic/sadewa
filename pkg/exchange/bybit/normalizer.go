package bybit

import (
	"fmt"
	"time"

	"github.com/cockroachdb/apd/v3"

	"sadewa/pkg/core"
)

// bybitTicker represents the raw ticker response from Bybit API (v5).
type bybitTicker struct {
	Symbol      string `json:"symbol"`
	LastPrice   string `json:"lastPrice"`
	HighPrice   string `json:"highPrice24h"`
	LowPrice    string `json:"lowPrice24h"`
	Volume      string `json:"volume24h"`
	Bid1Price   string `json:"bid1Price"`
	Bid1Size    string `json:"bid1Size"`
	Ask1Price   string `json:"ask1Price"`
	Ask1Size    string `json:"ask1Size"`
	PriceChange string `json:"price24hPcnt"`
}

// bybitOrder represents the raw order response from Bybit API.
type bybitOrder struct {
	OrderID     string `json:"orderId"`
	OrderLinkID string `json:"orderLinkId"`
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	OrderType   string `json:"orderType"`
	Price       string `json:"price"`
	Qty         string `json:"qty"`
	CumExecQty  string `json:"cumExecQty"`
	OrderStatus string `json:"orderStatus"`
	TimeInForce string `json:"timeInForce"`
	CreatedTime string `json:"createdTime"`
	UpdatedTime string `json:"updatedTime"`
	AvgPrice    string `json:"avgPrice"`
}

// bybitBalance represents a single asset balance from Bybit API.
type bybitBalance struct {
	Coin          string `json:"coin"`
	WalletBalance string `json:"walletBalance"`
	Free          string `json:"availableToWithdraw"`
	Locked        string `json:"cumRealisedPnl"`
}

// bybitCoin represents coin info in wallet.
type bybitCoin struct {
	Coin string `json:"coin"`
}

// bybitAccount represents the account information response from Bybit API.
type bybitAccount struct {
	List []struct {
		Coin []bybitBalance `json:"coin"`
	} `json:"list"`
}

// bybitTrade represents a public trade from Bybit API.
type bybitTrade struct {
	ExecID  string `json:"execId"`
	Symbol  string `json:"symbol"`
	Side    string `json:"side"`
	Price   string `json:"price"`
	Qty     string `json:"qty"`
	Time    string `json:"time"`
	IsMaker bool   `json:"isMaker"`
}

// bybitMyTrade represents a user's trade from Bybit API.
type bybitMyTrade struct {
	ExecID        string `json:"execId"`
	OrderID       string `json:"orderId"`
	OrderLinkID   string `json:"orderLinkId"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Price         string `json:"execPrice"`
	Qty           string `json:"execQty"`
	Fee           string `json:"execFee"`
	FeeCurrencyID string `json:"feeCurrencyId"`
	ExecTime      string `json:"execTime"`
}

// bybitOrderBook represents the order book response from Bybit API.
type bybitOrderBook struct {
	Symbol string     `json:"s"`
	Bids   [][]string `json:"b"`
	Asks   [][]string `json:"a"`
	Time   int64      `json:"ts"`
}

// bybitKline represents a kline/candlestick from Bybit API.
type bybitKline struct {
	Start    int64  `json:"start"`
	End      int64  `json:"end"`
	Interval string `json:"interval"`
	Open     string `json:"open"`
	High     string `json:"high"`
	Low      string `json:"low"`
	Close    string `json:"close"`
	Volume   string `json:"volume"`
	Turnover string `json:"turnover"`
}

// Normalizer converts Bybit-specific data structures to canonical core types.
type Normalizer struct{}

// NewNormalizer creates a new Normalizer instance.
func NewNormalizer() *Normalizer {
	return &Normalizer{}
}

// NormalizeTicker converts a Bybit ticker response to a canonical Ticker.
func (n *Normalizer) NormalizeTicker(data *bybitTicker) *core.Ticker {
	ticker := &core.Ticker{
		Symbol:    parseSymbol(data.Symbol),
		Timestamp: time.Now(),
	}

	parseDecimal(&ticker.Bid, data.Bid1Price)
	parseDecimal(&ticker.Ask, data.Ask1Price)
	parseDecimal(&ticker.Last, data.LastPrice)
	parseDecimal(&ticker.High, data.HighPrice)
	parseDecimal(&ticker.Low, data.LowPrice)
	parseDecimal(&ticker.Volume, data.Volume)

	return ticker
}

// NormalizeOrder converts a Bybit order response to a canonical Order.
// It calculates the remaining quantity from total and filled quantities.
func (n *Normalizer) NormalizeOrder(data *bybitOrder) (*core.Order, error) {
	order := &core.Order{
		ID:            data.OrderID,
		ClientOrderID: data.OrderLinkID,
		Symbol:        parseSymbol(data.Symbol),
		Side:          parseBybitOrderSide(data.Side),
		Type:          parseBybitOrderType(data.OrderType),
		Status:        parseBybitOrderStatus(data.OrderStatus),
		TimeInForce:   parseBybitTimeInForce(data.TimeInForce),
	}

	parseDecimal(&order.Price, data.Price)
	parseDecimal(&order.Quantity, data.Qty)
	parseDecimal(&order.FilledQuantity, data.CumExecQty)

	if data.CreatedTime != "" {
		if ts, err := parseBybitTime(data.CreatedTime); err == nil {
			order.CreatedAt = ts
		}
	}

	if data.UpdatedTime != "" {
		if ts, err := parseBybitTime(data.UpdatedTime); err == nil {
			order.UpdatedAt = ts
		}
	}

	var remaining apd.Decimal
	_, err := apd.BaseContext.Sub(&remaining, &order.Quantity, &order.FilledQuantity)
	if err != nil {
		return nil, fmt.Errorf("calculate remaining: %w", err)
	}
	order.RemainingQty = remaining

	return order, nil
}

// NormalizeBalance converts a Bybit balance to a canonical Balance.
func (n *Normalizer) NormalizeBalance(data *bybitBalance) *core.Balance {
	balance := &core.Balance{
		Asset: data.Coin,
	}

	parseDecimal(&balance.Free, data.Free)
	parseDecimal(&balance.Locked, data.Locked)

	return balance
}

// NormalizeBalances extracts and converts all balances from a Bybit account response.
func (n *Normalizer) NormalizeBalances(account *bybitAccount) []core.Balance {
	var balances []core.Balance

	for _, list := range account.List {
		for _, coin := range list.Coin {
			balances = append(balances, *n.NormalizeBalance(&coin))
		}
	}

	return balances
}

// NormalizeTrade converts a Bybit public trade to a canonical Trade.
func (n *Normalizer) NormalizeTrade(data *bybitTrade) *core.Trade {
	trade := &core.Trade{
		ID:     data.ExecID,
		Symbol: parseSymbol(data.Symbol),
		Side:   parseBybitSide(data.Side),
	}

	parseDecimal(&trade.Price, data.Price)
	parseDecimal(&trade.Quantity, data.Qty)

	if data.Time != "" {
		if ts, err := parseBybitTime(data.Time); err == nil {
			trade.Timestamp = ts
		}
	}

	return trade
}

// NormalizeMyTrade converts a Bybit user trade to a canonical Trade.
func (n *Normalizer) NormalizeMyTrade(data *bybitMyTrade) *core.Trade {
	trade := &core.Trade{
		ID:       data.ExecID,
		OrderID:  data.OrderID,
		Symbol:   parseSymbol(data.Symbol),
		Side:     parseBybitSide(data.Side),
		FeeAsset: data.FeeCurrencyID,
	}

	parseDecimal(&trade.Price, data.Price)
	parseDecimal(&trade.Quantity, data.Qty)
	parseDecimal(&trade.Fee, data.Fee)

	if data.ExecTime != "" {
		if ts, err := parseBybitTime(data.ExecTime); err == nil {
			trade.Timestamp = ts
		}
	}

	return trade
}

// NormalizeKline converts a Bybit kline to a canonical Kline.
func (n *Normalizer) NormalizeKline(data *bybitKline, symbol string) (*core.Kline, error) {
	kline := &core.Kline{
		Symbol:    symbol,
		OpenTime:  time.UnixMilli(data.Start),
		CloseTime: time.UnixMilli(data.End),
	}

	if err := parseDecimal(&kline.Open, data.Open); err != nil {
		return nil, fmt.Errorf("parse open: %w", err)
	}

	if err := parseDecimal(&kline.High, data.High); err != nil {
		return nil, fmt.Errorf("parse high: %w", err)
	}

	if err := parseDecimal(&kline.Low, data.Low); err != nil {
		return nil, fmt.Errorf("parse low: %w", err)
	}

	if err := parseDecimal(&kline.Close, data.Close); err != nil {
		return nil, fmt.Errorf("parse close: %w", err)
	}

	if err := parseDecimal(&kline.Volume, data.Volume); err != nil {
		return nil, fmt.Errorf("parse volume: %w", err)
	}

	if err := parseDecimal(&kline.QuoteVolume, data.Turnover); err != nil {
		kline.QuoteVolume = apd.Decimal{}
	}

	return kline, nil
}

// NormalizeKlines converts multiple Bybit klines to canonical Klines.
func (n *Normalizer) NormalizeKlines(data []bybitKline, symbol string) ([]core.Kline, error) {
	klines := make([]core.Kline, 0, len(data))
	for _, k := range data {
		kline, err := n.NormalizeKline(&k, symbol)
		if err != nil {
			return nil, fmt.Errorf("normalize kline: %w", err)
		}
		klines = append(klines, *kline)
	}
	return klines, nil
}

// NormalizeOrderBook converts a Bybit order book to a canonical OrderBook.
func (n *Normalizer) NormalizeOrderBook(data *bybitOrderBook, symbol string) (*core.OrderBook, error) {
	orderBook := &core.OrderBook{
		Symbol:    symbol,
		Timestamp: time.UnixMilli(data.Time),
	}

	bids, err := n.normalizeOrderBookLevels(data.Bids)
	if err != nil {
		return nil, fmt.Errorf("normalize bids: %w", err)
	}
	orderBook.Bids = bids

	asks, err := n.normalizeOrderBookLevels(data.Asks)
	if err != nil {
		return nil, fmt.Errorf("normalize asks: %w", err)
	}
	orderBook.Asks = asks

	return orderBook, nil
}

func (n *Normalizer) normalizeOrderBookLevels(levels [][]string) ([]core.OrderBookLevel, error) {
	result := make([]core.OrderBookLevel, 0, len(levels))

	for _, level := range levels {
		if len(level) < 2 {
			continue
		}

		var obl core.OrderBookLevel
		if err := parseDecimal(&obl.Price, level[0]); err != nil {
			return nil, fmt.Errorf("parse price: %w", err)
		}

		if err := parseDecimal(&obl.Quantity, level[1]); err != nil {
			return nil, fmt.Errorf("parse quantity: %w", err)
		}

		result = append(result, obl)
	}

	return result, nil
}

// NormalizeTrades converts multiple Bybit trades to canonical Trades.
func (n *Normalizer) NormalizeTrades(data []bybitTrade, symbol string) []core.Trade {
	trades := make([]core.Trade, 0, len(data))
	for _, t := range data {
		trade := n.NormalizeTrade(&t)
		trade.Symbol = symbol
		trades = append(trades, *trade)
	}
	return trades
}

// NormalizeOrders converts multiple Bybit orders to canonical Orders.
func (n *Normalizer) NormalizeOrders(data []bybitOrder) ([]core.Order, error) {
	orders := make([]core.Order, 0, len(data))
	for _, o := range data {
		order, err := n.NormalizeOrder(&o)
		if err != nil {
			return nil, fmt.Errorf("normalize order: %w", err)
		}
		orders = append(orders, *order)
	}
	return orders, nil
}

// OrderParams contains the Bybit-specific parameters for placing an order.
type OrderParams struct {
	Symbol        string
	Side          string
	Type          string
	Quantity      string
	Price         string
	TimeInForce   string
	ClientOrderID string
}

// DenormalizeOrder converts a canonical Order to Bybit-specific order parameters.
func (n *Normalizer) DenormalizeOrder(order *core.Order) OrderParams {
	params := OrderParams{
		Symbol:   formatSymbol(order.Symbol),
		Side:     order.Side.String(),
		Type:     order.Type.String(),
		Quantity: order.Quantity.String(),
	}

	if !order.Price.IsZero() {
		params.Price = order.Price.String()
	}

	if order.TimeInForce != core.GTC && order.Type == core.TypeLimit {
		params.TimeInForce = order.TimeInForce.String()
	}

	if order.ClientOrderID != "" {
		params.ClientOrderID = order.ClientOrderID
	}

	return params
}

func parseDecimal(dest *apd.Decimal, s string) error {
	if s == "" {
		*dest = apd.Decimal{}
		return nil
	}

	_, _, err := apd.BaseContext.SetString(dest, s)
	if err != nil {
		return fmt.Errorf("set decimal from string: %w", err)
	}

	return nil
}

func parseBybitTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	var ms int64
	if _, err := fmt.Sscanf(s, "%d", &ms); err != nil {
		return time.Time{}, fmt.Errorf("parse time: %w", err)
	}

	return time.UnixMilli(ms), nil
}

func parseBybitOrderSide(s string) core.OrderSide {
	switch s {
	case "Buy":
		return core.SideBuy
	case "Sell":
		return core.SideSell
	default:
		return core.SideBuy
	}
}

func parseBybitSide(s string) core.OrderSide {
	switch s {
	case "Buy":
		return core.SideBuy
	case "Sell":
		return core.SideSell
	default:
		return core.SideBuy
	}
}

func parseBybitOrderType(s string) core.OrderType {
	switch s {
	case "Market":
		return core.TypeMarket
	case "Limit":
		return core.TypeLimit
	default:
		return core.TypeMarket
	}
}

func parseBybitOrderStatus(s string) core.OrderStatus {
	switch s {
	case "New", "Created":
		return core.StatusNew
	case "PartiallyFilled":
		return core.StatusPartiallyFilled
	case "Filled":
		return core.StatusFilled
	case "Cancelled":
		return core.StatusCanceled
	case "Rejected":
		return core.StatusRejected
	case "Deactivated":
		return core.StatusExpired
	default:
		return core.StatusNew
	}
}

func parseBybitTimeInForce(s string) core.TimeInForce {
	switch s {
	case "GTC":
		return core.GTC
	case "IOC":
		return core.IOC
	case "FOK":
		return core.FOK
	default:
		return core.GTC
	}
}
