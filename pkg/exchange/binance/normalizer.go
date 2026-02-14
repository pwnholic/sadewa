package binance

import (
	"fmt"
	"time"

	"github.com/cockroachdb/apd/v3"

	"sadewa/pkg/core"
)

// binanceTicker represents the raw ticker response from Binance API.
type binanceTicker struct {
	Symbol             string      `json:"symbol"`
	PriceChange        apd.Decimal `json:"priceChange"`
	PriceChangePercent apd.Decimal `json:"priceChangePercent"`
	LastPrice          apd.Decimal `json:"lastPrice"`
	HighPrice          apd.Decimal `json:"highPrice"`
	LowPrice           apd.Decimal `json:"lowPrice"`
	Volume             apd.Decimal `json:"volume"`
	BidPrice           apd.Decimal `json:"bidPrice"`
	AskPrice           apd.Decimal `json:"askPrice"`
}

// binanceOrder represents the raw order response from Binance API.
type binanceOrder struct {
	Symbol        string      `json:"symbol"`
	OrderID       int64       `json:"orderId"`
	ClientOrderID string      `json:"clientOrderId"`
	Price         apd.Decimal `json:"price"`
	OrigQty       apd.Decimal `json:"origQty"`
	ExecutedQty   apd.Decimal `json:"executedQty"`
	Status        string      `json:"status"`
	Type          string      `json:"type"`
	Side          string      `json:"side"`
	TimeInForce   string      `json:"timeInForce"`
	TransactTime  int64       `json:"transactTime"`
	UpdateTime    int64       `json:"updateTime"`
}

// binanceBalance represents a single asset balance from Binance API.
type binanceBalance struct {
	Asset  string      `json:"asset"`
	Free   apd.Decimal `json:"free"`
	Locked apd.Decimal `json:"locked"`
}

// binanceAccount represents the account information response from Binance API.
type binanceAccount struct {
	MakerCommission  int64            `json:"makerCommission"`
	TakerCommission  int64            `json:"takerCommission"`
	BuyerCommission  int64            `json:"buyerCommission"`
	SellerCommission int64            `json:"sellerCommission"`
	CanTrade         bool             `json:"canTrade"`
	CanWithdraw      bool             `json:"canWithdraw"`
	CanDeposit       bool             `json:"canDeposit"`
	Balances         []binanceBalance `json:"balances"`
}

// binanceTrade represents a public trade from Binance API.
type binanceTrade struct {
	ID           int64       `json:"id"`
	Price        apd.Decimal `json:"price"`
	Qty          apd.Decimal `json:"qty"`
	QuoteQty     apd.Decimal `json:"quoteQty"`
	Time         int64       `json:"time"`
	IsBuyerMaker bool        `json:"isBuyerMaker"`
	IsBestMatch  bool        `json:"isBestMatch"`
}

// binanceMyTrade represents a user's trade from Binance API.
type binanceMyTrade struct {
	ID              int64       `json:"id"`
	OrderID         int64       `json:"orderId"`
	Symbol          string      `json:"symbol"`
	Side            string      `json:"side"`
	Price           apd.Decimal `json:"price"`
	Qty             apd.Decimal `json:"qty"`
	QuoteQty        apd.Decimal `json:"quoteQty"`
	Commission      apd.Decimal `json:"commission"`
	CommissionAsset string      `json:"commissionAsset"`
	Time            int64       `json:"time"`
	IsBuyerMaker    bool        `json:"isBuyerMaker"`
	IsBestMatch     bool        `json:"isBestMatch"`
}

// binanceOrderBook represents the order book response from Binance API.
type binanceOrderBook struct {
	LastUpdateID int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}

// binanceKline represents a kline/candlestick data array from Binance API.
type binanceKline []any

// Normalizer converts Binance-specific data structures to canonical core types.
type Normalizer struct{}

// NewNormalizer creates a new Normalizer instance.
func NewNormalizer() *Normalizer {
	return &Normalizer{}
}

// NormalizeTicker converts a Binance ticker response to a canonical Ticker.
func (n *Normalizer) NormalizeTicker(data *binanceTicker) *core.Ticker {
	return &core.Ticker{
		Symbol:    parseSymbol(data.Symbol),
		Bid:       data.BidPrice,
		Ask:       data.AskPrice,
		Last:      data.LastPrice,
		High:      data.HighPrice,
		Low:       data.LowPrice,
		Volume:    data.Volume,
		Timestamp: time.Now(),
	}
}

// NormalizeOrder converts a Binance order response to a canonical Order.
// It calculates the remaining quantity from total and filled quantities.
func (n *Normalizer) NormalizeOrder(data *binanceOrder) (*core.Order, error) {
	order := &core.Order{
		ID:             fmt.Sprintf("%d", data.OrderID),
		ClientOrderID:  data.ClientOrderID,
		Symbol:         parseSymbol(data.Symbol),
		Side:           parseOrderSide(data.Side),
		Type:           parseOrderType(data.Type),
		Status:         parseOrderStatus(data.Status),
		TimeInForce:    parseTimeInForce(data.TimeInForce),
		Price:          data.Price,
		Quantity:       data.OrigQty,
		FilledQuantity: data.ExecutedQty,
	}

	if data.TransactTime > 0 {
		order.CreatedAt = time.UnixMilli(data.TransactTime)
	}

	if data.UpdateTime > 0 {
		order.UpdatedAt = time.UnixMilli(data.UpdateTime)
	}

	var remaining apd.Decimal
	_, err := apd.BaseContext.Sub(&remaining, &order.Quantity, &order.FilledQuantity)
	if err != nil {
		return nil, fmt.Errorf("calculate remaining: %w", err)
	}
	order.RemainingQty = remaining

	return order, nil
}

// NormalizeBalance converts a Binance balance to a canonical Balance.
func (n *Normalizer) NormalizeBalance(data *binanceBalance) *core.Balance {
	return &core.Balance{
		Asset:  data.Asset,
		Free:   data.Free,
		Locked: data.Locked,
	}
}

// NormalizeBalances extracts and converts all balances from a Binance account response.
func (n *Normalizer) NormalizeBalances(account *binanceAccount) []core.Balance {
	balances := make([]core.Balance, 0, len(account.Balances))
	for _, b := range account.Balances {
		balances = append(balances, *n.NormalizeBalance(&b))
	}
	return balances
}

// NormalizeTrade converts a Binance public trade to a canonical Trade.
func (n *Normalizer) NormalizeTrade(data *binanceTrade) *core.Trade {
	trade := &core.Trade{
		ID:       fmt.Sprintf("%d", data.ID),
		OrderID:  "",
		Symbol:   "",
		Side:     parseSideFromBuyerMaker(data.IsBuyerMaker),
		Price:    data.Price,
		Quantity: data.Qty,
		Fee:      apd.Decimal{},
		FeeAsset: "",
	}

	if data.Time > 0 {
		trade.Timestamp = time.UnixMilli(data.Time)
	}

	return trade
}

// NormalizeMyTrade converts a Binance user trade to a canonical Trade.
func (n *Normalizer) NormalizeMyTrade(data *binanceMyTrade) *core.Trade {
	trade := &core.Trade{
		ID:       fmt.Sprintf("%d", data.ID),
		OrderID:  fmt.Sprintf("%d", data.OrderID),
		Symbol:   parseSymbol(data.Symbol),
		Side:     parseSideFromBuyerMaker(data.IsBuyerMaker),
		Price:    data.Price,
		Quantity: data.Qty,
		Fee:      data.Commission,
		FeeAsset: data.CommissionAsset,
	}

	if data.Time > 0 {
		trade.Timestamp = time.UnixMilli(data.Time)
	}

	return trade
}

// NormalizeKline converts a Binance kline array to a canonical Kline.
// Returns an error if the kline data has insufficient elements.
func (n *Normalizer) NormalizeKline(data binanceKline, symbol string) (*core.Kline, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("insufficient kline data elements: %d", len(data))
	}

	kline := &core.Kline{
		Symbol: symbol,
	}

	if openTime, ok := data[0].(float64); ok {
		kline.OpenTime = time.UnixMilli(int64(openTime))
	}

	if err := parseDecimalFromAny(&kline.Open, data[1]); err != nil {
		return nil, fmt.Errorf("parse open: %w", err)
	}

	if err := parseDecimalFromAny(&kline.High, data[2]); err != nil {
		return nil, fmt.Errorf("parse high: %w", err)
	}

	if err := parseDecimalFromAny(&kline.Low, data[3]); err != nil {
		return nil, fmt.Errorf("parse low: %w", err)
	}

	if err := parseDecimalFromAny(&kline.Close, data[4]); err != nil {
		return nil, fmt.Errorf("parse close: %w", err)
	}

	if err := parseDecimalFromAny(&kline.Volume, data[5]); err != nil {
		return nil, fmt.Errorf("parse volume: %w", err)
	}

	if closeTime, ok := data[6].(float64); ok {
		kline.CloseTime = time.UnixMilli(int64(closeTime))
	}

	if len(data) > 7 {
		if err := parseDecimalFromAny(&kline.QuoteVolume, data[7]); err != nil {
			kline.QuoteVolume = apd.Decimal{}
		}
	}

	if len(data) > 8 {
		if numTrades, ok := data[8].(float64); ok {
			kline.NumTrades = int64(numTrades)
		}
	}

	return kline, nil
}

// NormalizeKlines converts multiple Binance klines to canonical Klines.
func (n *Normalizer) NormalizeKlines(data []binanceKline, symbol string) ([]core.Kline, error) {
	klines := make([]core.Kline, 0, len(data))
	for _, k := range data {
		kline, err := n.NormalizeKline(k, symbol)
		if err != nil {
			return nil, fmt.Errorf("normalize kline: %w", err)
		}
		klines = append(klines, *kline)
	}
	return klines, nil
}

// NormalizeOrderBook converts a Binance order book to a canonical OrderBook.
func (n *Normalizer) NormalizeOrderBook(data *binanceOrderBook, symbol string) (*core.OrderBook, error) {
	orderBook := &core.OrderBook{
		Symbol:    symbol,
		Timestamp: time.Now(),
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

// NormalizeTrades converts multiple Binance trades to canonical Trades.
func (n *Normalizer) NormalizeTrades(data []binanceTrade, symbol string) []core.Trade {
	trades := make([]core.Trade, 0, len(data))
	for _, t := range data {
		trade := n.NormalizeTrade(&t)
		trade.Symbol = symbol
		trades = append(trades, *trade)
	}
	return trades
}

// NormalizeOrders converts multiple Binance orders to canonical Orders.
func (n *Normalizer) NormalizeOrders(data []binanceOrder) ([]core.Order, error) {
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

// OrderParams contains the Binance-specific parameters for placing an order.
type OrderParams struct {
	Symbol        string
	Side          string
	Type          string
	Quantity      string
	Price         string
	TimeInForce   string
	ClientOrderID string
}

// DenormalizeOrder converts a canonical Order to Binance-specific order parameters.
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

func parseDecimalFromAny(dest *apd.Decimal, val any) error {
	switch v := val.(type) {
	case string:
		return parseDecimal(dest, v)
	case float64:
		_, _, err := apd.BaseContext.SetString(dest, fmt.Sprintf("%v", v))
		return err
	default:
		return fmt.Errorf("unsupported type for decimal: %T", val)
	}
}

func parseOrderSide(s string) core.OrderSide {
	switch s {
	case "BUY":
		return core.SideBuy
	case "SELL":
		return core.SideSell
	default:
		return core.SideBuy
	}
}

func parseOrderType(s string) core.OrderType {
	switch s {
	case "MARKET":
		return core.TypeMarket
	case "LIMIT":
		return core.TypeLimit
	case "STOP_LOSS":
		return core.TypeStopLoss
	case "STOP_LOSS_LIMIT":
		return core.TypeStopLossLimit
	case "TAKE_PROFIT":
		return core.TypeTakeProfit
	case "TAKE_PROFIT_LIMIT":
		return core.TypeTakeProfitLimit
	default:
		return core.TypeMarket
	}
}

func parseOrderStatus(s string) core.OrderStatus {
	switch s {
	case "NEW":
		return core.StatusNew
	case "PARTIALLY_FILLED":
		return core.StatusPartiallyFilled
	case "FILLED":
		return core.StatusFilled
	case "PENDING_CANCEL":
		return core.StatusCanceling
	case "CANCELED":
		return core.StatusCanceled
	case "REJECTED":
		return core.StatusRejected
	case "EXPIRED":
		return core.StatusExpired
	default:
		return core.StatusNew
	}
}

func parseTimeInForce(s string) core.TimeInForce {
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

func parseSideFromBuyerMaker(isBuyerMaker bool) core.OrderSide {
	if isBuyerMaker {
		return core.SideSell
	}
	return core.SideBuy
}
