package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cockroachdb/apd/v3"
	"github.com/rs/zerolog"

	"sadewa/internal/transport"
	"sadewa/pkg/core"
)

type BinanceWSClient struct {
	client    *transport.WSClient
	logger    zerolog.Logger
	requestID atomic.Int64

	tickerCallbacks     map[string]func(*core.Ticker)
	miniTickerCallbacks map[string]func(*core.Ticker)
	orderBookCallbacks  map[string]func(*core.OrderBook)
	tradeCallbacks      map[string]func(*core.Trade)
	klineCallbacks      map[string]func(*core.Kline)
	aggTradeCallbacks   map[string]func(*core.Trade)

	mu                sync.RWMutex
	handlerRegistered bool
}

type BinanceWSConfig struct {
	Sandbox bool
}

func NewBinanceWSClient(sandbox bool) *BinanceWSClient {
	wsURL := "wss://stream.binance.com:9443/ws"
	if sandbox {
		wsURL = "wss://testnet.binance.vision/ws"
	}

	return &BinanceWSClient{
		client: transport.NewWSClient(transport.WSConfig{
			URL:               wsURL,
			ReconnectEnabled:  true,
			ReconnectMaxWait:  30 * time.Second,
			ReconnectBaseWait: 1 * time.Second,
			PingInterval:      3 * time.Minute,
		}),
		logger:              zerolog.Nop(),
		tickerCallbacks:     make(map[string]func(*core.Ticker)),
		miniTickerCallbacks: make(map[string]func(*core.Ticker)),
		orderBookCallbacks:  make(map[string]func(*core.OrderBook)),
		tradeCallbacks:      make(map[string]func(*core.Trade)),
		klineCallbacks:      make(map[string]func(*core.Kline)),
		aggTradeCallbacks:   make(map[string]func(*core.Trade)),
	}
}

func NewBinanceWSClientWithConfig(config BinanceWSConfig) *BinanceWSClient {
	return NewBinanceWSClient(config.Sandbox)
}

func (c *BinanceWSClient) Connect(ctx context.Context) error {
	c.client.SetLogger(c.logger)

	if err := c.client.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	if err := c.client.Subscribe("_router", c.routeMessage); err != nil {
		return fmt.Errorf("register router: %w", err)
	}

	c.mu.Lock()
	c.handlerRegistered = true
	c.mu.Unlock()

	return nil
}

func (c *BinanceWSClient) Close() error {
	return c.client.Close()
}

func (c *BinanceWSClient) IsConnected() bool {
	return c.client.IsConnected()
}

func (c *BinanceWSClient) SetLogger(logger zerolog.Logger) {
	c.logger = logger
}

func (c *BinanceWSClient) routeMessage(data []byte) error {
	var base map[string]any
	if err := json.Unmarshal(data, &base); err != nil {
		c.logger.Debug().Err(err).Msg("failed to parse base message")
		return nil
	}

	event, _ := base["e"].(string)
	switch event {
	case "24hrTicker":
		return c.handleTickerMessage(data)
	case "24hrMiniTicker":
		return c.handleMiniTickerMessage(data)
	case "trade":
		return c.handleTradeMessage(data)
	case "aggTrade":
		return c.handleAggTradeMessage(data)
	case "depthUpdate", "depth":
		return c.handleDepthMessage(data)
	case "kline":
		return c.handleKlineMessage(data)
	default:
		c.logger.Debug().Str("event", event).Msg("unhandled event type")
	}

	return nil
}

func (c *BinanceWSClient) handleTickerMessage(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse ticker: %w", err)
	}

	symbolRaw, _ := raw["s"].(string)
	symbol := parseSymbol(symbolRaw)
	key := strings.ToLower(symbolRaw) + "@ticker"

	c.mu.RLock()
	callback, ok := c.tickerCallbacks[key]
	c.mu.RUnlock()

	if ok && callback != nil {
		ticker := &core.Ticker{
			Symbol:    symbol,
			Timestamp: time.UnixMilli(int64(raw["E"].(float64))),
		}
		if v, ok := raw["b"].(string); ok {
			parseDecimal(&ticker.Bid, v)
		}
		if v, ok := raw["a"].(string); ok {
			parseDecimal(&ticker.Ask, v)
		}
		if v, ok := raw["c"].(string); ok {
			parseDecimal(&ticker.Last, v)
		}
		if v, ok := raw["h"].(string); ok {
			parseDecimal(&ticker.High, v)
		}
		if v, ok := raw["l"].(string); ok {
			parseDecimal(&ticker.Low, v)
		}
		if v, ok := raw["v"].(string); ok {
			parseDecimal(&ticker.Volume, v)
		}
		callback(ticker)
	}

	return nil
}

func (c *BinanceWSClient) handleMiniTickerMessage(data []byte) error {
	var msg wsTickerMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("parse mini ticker: %w", err)
	}

	symbol := parseSymbol(msg.Symbol)
	key := strings.ToLower(msg.Symbol) + "@miniticker"

	c.mu.RLock()
	callback, ok := c.miniTickerCallbacks[key]
	c.mu.RUnlock()

	if ok && callback != nil {
		ticker := &core.Ticker{
			Symbol:    symbol,
			Bid:       msg.BidPrice,
			Ask:       msg.AskPrice,
			Last:      msg.LastPrice,
			High:      msg.HighPrice,
			Low:       msg.LowPrice,
			Volume:    msg.Volume,
			Timestamp: time.UnixMilli(msg.EventTime),
		}
		callback(ticker)
	}

	return nil
}

func (c *BinanceWSClient) handleTradeMessage(data []byte) error {
	var msg wsTradeMessage
	if err := sonic.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("parse trade: %w", err)
	}

	symbol := parseSymbol(msg.Symbol)
	key := strings.ToLower(msg.Symbol) + "@trade"

	c.mu.RLock()
	callback, ok := c.tradeCallbacks[key]
	c.mu.RUnlock()

	if ok && callback != nil {
		trade := &core.Trade{
			ID:        fmt.Sprintf("%d", msg.TradeID),
			Symbol:    symbol,
			Side:      parseSideFromBuyerMaker(msg.IsBuyerMaker),
			Price:     msg.Price,
			Quantity:  msg.Quantity,
			Timestamp: time.UnixMilli(msg.EventTime),
		}
		callback(trade)
	}

	return nil
}

func (c *BinanceWSClient) handleAggTradeMessage(data []byte) error {
	var msg wsAggTradeMessage
	if err := sonic.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("parse aggTrade: %w", err)
	}

	symbol := parseSymbol(msg.Symbol)
	key := strings.ToLower(msg.Symbol) + "@aggTrade"

	c.mu.RLock()
	callback, ok := c.aggTradeCallbacks[key]
	c.mu.RUnlock()

	if ok && callback != nil {
		trade := &core.Trade{
			ID:        fmt.Sprintf("%d", msg.AggTradeID),
			Symbol:    symbol,
			Side:      parseSideFromBuyerMaker(msg.IsBuyerMaker),
			Price:     msg.Price,
			Quantity:  msg.Quantity,
			Timestamp: time.UnixMilli(msg.EventTime),
		}
		callback(trade)
	}

	return nil
}

func (c *BinanceWSClient) handleDepthMessage(data []byte) error {
	var msg wsDepthMessage
	if err := sonic.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("parse depth: %w", err)
	}

	symbol := parseSymbol(msg.Symbol)
	key := strings.ToLower(msg.Symbol) + "@depth"

	c.mu.RLock()
	callback, ok := c.orderBookCallbacks[key]
	c.mu.RUnlock()

	if ok && callback != nil {
		orderBook := &core.OrderBook{
			Symbol:    symbol,
			Timestamp: time.UnixMilli(msg.EventTime),
		}

		for _, bid := range msg.Bids {
			if len(bid) >= 2 {
				var price, qty apd.Decimal
				if err := parseDecimal(&price, bid[0]); err == nil {
					if err := parseDecimal(&qty, bid[1]); err == nil {
						orderBook.Bids = append(orderBook.Bids, core.OrderBookLevel{
							Price:    price,
							Quantity: qty,
						})
					}
				}
			}
		}

		for _, ask := range msg.Asks {
			if len(ask) >= 2 {
				var price, qty apd.Decimal
				if err := parseDecimal(&price, ask[0]); err == nil {
					if err := parseDecimal(&qty, ask[1]); err == nil {
						orderBook.Asks = append(orderBook.Asks, core.OrderBookLevel{
							Price:    price,
							Quantity: qty,
						})
					}
				}
			}
		}

		callback(orderBook)
	}

	return nil
}

func (c *BinanceWSClient) handleKlineMessage(data []byte) error {
	var msg wsKlineMessage
	if err := sonic.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("parse kline: %w", err)
	}

	symbol := parseSymbol(msg.Symbol)
	key := strings.ToLower(msg.Symbol) + "@kline_" + strings.ToLower(msg.Kline.Interval)

	c.mu.RLock()
	callback, ok := c.klineCallbacks[key]
	c.mu.RUnlock()

	if ok && callback != nil {
		kline := &core.Kline{
			Symbol:      symbol,
			OpenTime:    time.UnixMilli(msg.Kline.StartTime),
			CloseTime:   time.UnixMilli(msg.Kline.EndTime),
			Open:        msg.Kline.Open,
			High:        msg.Kline.High,
			Low:         msg.Kline.Low,
			Close:       msg.Kline.Close,
			Volume:      msg.Kline.Volume,
			QuoteVolume: msg.Kline.QuoteVolume,
			NumTrades:   msg.Kline.NumTrades,
		}
		callback(kline)
	}

	return nil
}

func (c *BinanceWSClient) subscribeStream(stream string, callback any) error {
	if c.IsConnected() {
		c.sendSubscribe([]string{stream})
	}
	return nil
}

func (c *BinanceWSClient) unsubscribeStream(stream string) {
	if c.IsConnected() {
		c.sendUnsubscribe([]string{stream})
	}
}

func (c *BinanceWSClient) sendSubscribe(streams []string) {
	if !c.IsConnected() {
		return
	}

	req := wsRequest{
		Method: "SUBSCRIBE",
		Params: streams,
		ID:     c.requestID.Add(1),
	}

	if err := c.client.SendJSON(req); err != nil {
		c.logger.Error().Err(err).Strs("streams", streams).Msg("failed to send subscribe")
	}
	c.logger.Debug().Strs("streams", streams).Msg("sent subscribe")

	c.logger.Debug().Strs("streams", streams).Msg("subscribed to streams")
}

func (c *BinanceWSClient) sendUnsubscribe(streams []string) {
	if !c.IsConnected() {
		return
	}

	req := wsRequest{
		Method: "UNSUBSCRIBE",
		Params: streams,
		ID:     c.requestID.Add(1),
	}

	if err := c.client.SendJSON(req); err != nil {
		c.logger.Error().Err(err).Strs("streams", streams).Msg("failed to send unsubscribe")
	}
}

func (c *BinanceWSClient) SubscribeTicker(symbol string, callback func(*core.Ticker)) error {
	stream := buildStreamName(symbol, "ticker")
	key := stream

	c.mu.Lock()
	c.tickerCallbacks[key] = callback
	c.mu.Unlock()

	return c.subscribeStream(stream, callback)
}

func (c *BinanceWSClient) UnsubscribeTicker(symbol string) {
	stream := buildStreamName(symbol, "ticker")

	c.mu.Lock()
	delete(c.tickerCallbacks, stream)
	c.mu.Unlock()

	c.unsubscribeStream(stream)
}

func (c *BinanceWSClient) SubscribeMiniTicker(symbol string, callback func(*core.Ticker)) error {
	stream := buildStreamName(symbol, "miniTicker")
	key := stream

	c.mu.Lock()
	c.miniTickerCallbacks[key] = callback
	c.mu.Unlock()

	return c.subscribeStream(stream, callback)
}

func (c *BinanceWSClient) UnsubscribeMiniTicker(symbol string) {
	stream := buildStreamName(symbol, "miniTicker")

	c.mu.Lock()
	delete(c.miniTickerCallbacks, stream)
	c.mu.Unlock()

	c.unsubscribeStream(stream)
}

func (c *BinanceWSClient) SubscribeOrderBook(symbol string, callback func(*core.OrderBook)) error {
	stream := buildStreamName(symbol, "depth")

	c.mu.Lock()
	c.orderBookCallbacks[stream] = callback
	c.mu.Unlock()

	return c.subscribeStream(stream, callback)
}

func (c *BinanceWSClient) UnsubscribeOrderBook(symbol string) {
	stream := buildStreamName(symbol, "depth")

	c.mu.Lock()
	delete(c.orderBookCallbacks, stream)
	c.mu.Unlock()

	c.unsubscribeStream(stream)
}

func (c *BinanceWSClient) SubscribeOrderBookDepth(symbol string, depth int, speed int, callback func(*core.OrderBook)) error {
	stream := fmt.Sprintf("%s@depth%d@%dms", strings.ToLower(formatSymbol(symbol)), depth, speed)
	key := stream

	c.mu.Lock()
	c.orderBookCallbacks[key] = callback
	c.mu.Unlock()

	return c.subscribeStream(stream, callback)
}

func (c *BinanceWSClient) UnsubscribeOrderBookDepth(symbol string, depth int, speed int) {
	stream := fmt.Sprintf("%s@depth%d@%dms", strings.ToLower(formatSymbol(symbol)), depth, speed)

	c.mu.Lock()
	delete(c.orderBookCallbacks, stream)
	c.mu.Unlock()

	c.unsubscribeStream(stream)
}

func (c *BinanceWSClient) SubscribeTrades(symbol string, callback func(*core.Trade)) error {
	stream := buildStreamName(symbol, "trade")

	c.mu.Lock()
	c.tradeCallbacks[stream] = callback
	c.mu.Unlock()

	return c.subscribeStream(stream, callback)
}

func (c *BinanceWSClient) UnsubscribeTrades(symbol string) {
	stream := buildStreamName(symbol, "trade")

	c.mu.Lock()
	delete(c.tradeCallbacks, stream)
	c.mu.Unlock()

	c.unsubscribeStream(stream)
}

func (c *BinanceWSClient) SubscribeAggTrades(symbol string, callback func(*core.Trade)) error {
	stream := buildStreamName(symbol, "aggTrade")

	c.mu.Lock()
	c.aggTradeCallbacks[stream] = callback
	c.mu.Unlock()

	return c.subscribeStream(stream, callback)
}

func (c *BinanceWSClient) UnsubscribeAggTrades(symbol string) {
	stream := buildStreamName(symbol, "aggTrade")

	c.mu.Lock()
	delete(c.aggTradeCallbacks, stream)
	c.mu.Unlock()

	c.unsubscribeStream(stream)
}

func (c *BinanceWSClient) SubscribeKlines(symbol, interval string, callback func(*core.Kline)) error {
	stream := buildStreamName(symbol, "kline_"+interval)

	c.mu.Lock()
	c.klineCallbacks[stream] = callback
	c.mu.Unlock()

	return c.subscribeStream(stream, callback)
}

func (c *BinanceWSClient) UnsubscribeKlines(symbol, interval string) {
	stream := buildStreamName(symbol, "kline_"+interval)

	c.mu.Lock()
	delete(c.klineCallbacks, stream)
	c.mu.Unlock()

	c.unsubscribeStream(stream)
}

func (c *BinanceWSClient) Subscriptions() []string {
	return c.client.Subscriptions()
}

func buildStreamName(symbol, stream string) string {
	return strings.ToLower(formatSymbol(symbol)) + "@" + stream
}

type wsRequest struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	ID     int64    `json:"id"`
}

type wsBaseMessage struct {
	Event string `json:"e"`
}

type wsTickerMessage struct {
	Event       string      `json:"e"`
	EventTime   int64       `json:"E"`
	Symbol      string      `json:"s"`
	PriceChange apd.Decimal `json:"p"`
	LastPrice   apd.Decimal `json:"c"`
	HighPrice   apd.Decimal `json:"h"`
	LowPrice    apd.Decimal `json:"l"`
	Volume      apd.Decimal `json:"v"`
	BidPrice    apd.Decimal `json:"b"`
	AskPrice    apd.Decimal `json:"a"`
}

type wsTradeMessage struct {
	Event        string      `json:"e"`
	EventTime    int64       `json:"E"`
	Symbol       string      `json:"s"`
	TradeID      int64       `json:"t"`
	Price        apd.Decimal `json:"p"`
	Quantity     apd.Decimal `json:"q"`
	IsBuyerMaker bool        `json:"m"`
}

type wsAggTradeMessage struct {
	Event        string      `json:"e"`
	EventTime    int64       `json:"E"`
	Symbol       string      `json:"s"`
	AggTradeID   int64       `json:"a"`
	Price        apd.Decimal `json:"p"`
	Quantity     apd.Decimal `json:"q"`
	IsBuyerMaker bool        `json:"m"`
}

type wsDepthMessage struct {
	Event     string     `json:"e"`
	EventTime int64      `json:"E"`
	Symbol    string     `json:"s"`
	Bids      [][]string `json:"b"`
	Asks      [][]string `json:"a"`
}

type wsKlineMessage struct {
	Event     string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	Kline     struct {
		StartTime   int64       `json:"t"`
		EndTime     int64       `json:"T"`
		Interval    string      `json:"i"`
		Open        apd.Decimal `json:"o"`
		Close       apd.Decimal `json:"c"`
		High        apd.Decimal `json:"h"`
		Low         apd.Decimal `json:"l"`
		Volume      apd.Decimal `json:"v"`
		QuoteVolume apd.Decimal `json:"q"`
		NumTrades   int64       `json:"n"`
	} `json:"k"`
}
