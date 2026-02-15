package bybit

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

	"sadewa/internal/ws"
	"sadewa/pkg/core"
)

type BybitWSClient struct {
	client    *ws.WSClient
	logger    zerolog.Logger
	requestID atomic.Int64

	tickerCallbacks    map[string]func(*core.Ticker)
	orderBookCallbacks map[string]func(*core.OrderBook)
	tradeCallbacks     map[string]func(*core.Trade)
	klineCallbacks     map[string]func(*core.Kline)

	mu                sync.RWMutex
	handlerRegistered bool
	sandbox           bool
}

type BybitWSConfig struct {
	Sandbox bool
}

func NewBybitWSClient(sandbox bool) *BybitWSClient {
	wsURL := "wss://stream.bybit.com/v5/public/spot"
	if sandbox {
		wsURL = "wss://stream-testnet.bybit.com/v5/public/spot"
	}

	return &BybitWSClient{
		client: ws.NewWSClient(ws.WSConfig{
			URL:               wsURL,
			ReconnectEnabled:  true,
			ReconnectMaxWait:  30 * time.Second,
			ReconnectBaseWait: 1 * time.Second,
			PingInterval:      20 * time.Second,
		}),
		logger:            zerolog.Nop(),
		sandbox:           sandbox,
		tickerCallbacks:   make(map[string]func(*core.Ticker)),
		orderBookCallbacks: make(map[string]func(*core.OrderBook)),
		tradeCallbacks:    make(map[string]func(*core.Trade)),
		klineCallbacks:    make(map[string]func(*core.Kline)),
	}
}

func NewBybitWSClientWithConfig(config BybitWSConfig) *BybitWSClient {
	return NewBybitWSClient(config.Sandbox)
}

func (c *BybitWSClient) Connect(ctx context.Context) error {
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

func (c *BybitWSClient) Close() error {
	return c.client.Close()
}

func (c *BybitWSClient) IsConnected() bool {
	return c.client.IsConnected()
}

func (c *BybitWSClient) SetLogger(logger zerolog.Logger) {
	c.logger = logger
}

func (c *BybitWSClient) routeMessage(data []byte) error {
	var base struct {
		Topic string `json:"topic"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal(data, &base); err != nil {
		c.logger.Debug().Err(err).Msg("failed to parse base message")
		return nil
	}

	// Bybit uses topic-based routing
	if strings.Contains(base.Topic, "tickers") {
		return c.handleTickerMessage(data)
	}
	if strings.Contains(base.Topic, "orderbook") {
		return c.handleDepthMessage(data)
	}
	if strings.Contains(base.Topic, "publicTrade") {
		return c.handleTradeMessage(data)
	}
	if strings.Contains(base.Topic, "kline") {
		return c.handleKlineMessage(data)
	}

	c.logger.Debug().Str("topic", base.Topic).Msg("unhandled topic type")
	return nil
}

func (c *BybitWSClient) handleTickerMessage(data []byte) error {
	var msg struct {
		Topic string `json:"topic"`
		Type  string `json:"type"`
		Data  struct {
			Symbol          string `json:"symbol"`
			LastPrice       string `json:"lastPrice"`
			HighPrice24h    string `json:"highPrice24h"`
			LowPrice24h     string `json:"lowPrice24h"`
			Volume24h       string `json:"volume24h"`
			Bid1Price       string `json:"bid1Price"`
			Ask1Price       string `json:"ask1Price"`
		} `json:"data"`
		Ts int64 `json:"ts"`
	}

	if err := sonic.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("parse ticker: %w", err)
	}

	symbol := parseSymbol(msg.Data.Symbol)
	key := "tickers." + strings.ToLower(msg.Data.Symbol)

	c.mu.RLock()
	callback, ok := c.tickerCallbacks[key]
	c.mu.RUnlock()

	if ok && callback != nil {
		ticker := &core.Ticker{
			Symbol:    symbol,
			Timestamp: time.UnixMilli(msg.Ts),
		}
		parseDecimal(&ticker.Bid, msg.Data.Bid1Price)
		parseDecimal(&ticker.Ask, msg.Data.Ask1Price)
		parseDecimal(&ticker.Last, msg.Data.LastPrice)
		parseDecimal(&ticker.High, msg.Data.HighPrice24h)
		parseDecimal(&ticker.Low, msg.Data.LowPrice24h)
		parseDecimal(&ticker.Volume, msg.Data.Volume24h)
		callback(ticker)
	}

	return nil
}

func (c *BybitWSClient) handleTradeMessage(data []byte) error {
	var msg struct {
		Topic string `json:"topic"`
		Type  string `json:"type"`
		Data  []struct {
			ExecID  string `json:"i"`
			Price   string `json:"p"`
			Qty     string `json:"v"`
			Side    string `json:"S"`
			Time    int64  `json:"T"`
			IsMaker bool   `json:"m"`
		} `json:"data"`
		Ts int64 `json:"ts"`
	}

	if err := sonic.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("parse trade: %w", err)
	}

	if len(msg.Data) == 0 {
		return nil
	}

	// Extract symbol from topic: "publicTrade.BTCUSDT"
	symbolRaw := strings.TrimPrefix(msg.Topic, "publicTrade.")
	symbol := parseSymbol(symbolRaw)
	key := msg.Topic

	c.mu.RLock()
	callback, ok := c.tradeCallbacks[key]
	c.mu.RUnlock()

	if ok && callback != nil {
		for _, trade := range msg.Data {
			t := &core.Trade{
				ID:        trade.ExecID,
				Symbol:    symbol,
				Side:      parseBybitSide(trade.Side),
				Timestamp: time.UnixMilli(trade.Time),
			}
			parseDecimal(&t.Price, trade.Price)
			parseDecimal(&t.Quantity, trade.Qty)
			callback(t)
		}
	}

	return nil
}

func (c *BybitWSClient) handleDepthMessage(data []byte) error {
	var msg struct {
		Topic string `json:"topic"`
		Type  string `json:"type"`
		Data  struct {
			Symbol string     `json:"s"`
			Bids   [][]string `json:"b"`
			Asks   [][]string `json:"a"`
		} `json:"data"`
		Ts int64 `json:"ts"`
	}

	if err := sonic.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("parse depth: %w", err)
	}

	symbol := parseSymbol(msg.Data.Symbol)
	key := "orderbook." + strings.ToLower(msg.Data.Symbol) + ".1"

	c.mu.RLock()
	callback, ok := c.orderBookCallbacks[key]
	c.mu.RUnlock()

	if ok && callback != nil {
		orderBook := &core.OrderBook{
			Symbol:    symbol,
			Timestamp: time.UnixMilli(msg.Ts),
		}

		for _, bid := range msg.Data.Bids {
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

		for _, ask := range msg.Data.Asks {
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

func (c *BybitWSClient) handleKlineMessage(data []byte) error {
	var msg struct {
		Topic string `json:"topic"`
		Type  string `json:"type"`
		Data  []struct {
			Start    int64  `json:"start"`
			End      int64  `json:"end"`
			Interval string `json:"interval"`
			Open     string `json:"open"`
			High     string `json:"high"`
			Low      string `json:"low"`
			Close    string `json:"close"`
			Volume   string `json:"volume"`
			Turnover string `json:"turnover"`
		} `json:"data"`
		Ts int64 `json:"ts"`
	}

	if err := sonic.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("parse kline: %w", err)
	}

	if len(msg.Data) == 0 {
		return nil
	}

	// Extract symbol and interval from topic: "kline.1.BTCUSDT"
	parts := strings.Split(msg.Topic, ".")
	if len(parts) < 3 {
		return nil
	}
	interval := parts[1]
	symbolRaw := parts[2]
	symbol := parseSymbol(symbolRaw)
	key := msg.Topic

	c.mu.RLock()
	callback, ok := c.klineCallbacks[key]
	c.mu.RUnlock()

	if ok && callback != nil {
		for _, k := range msg.Data {
			kline := &core.Kline{
				Symbol:      symbol,
				OpenTime:    time.UnixMilli(k.Start),
				CloseTime:   time.UnixMilli(k.End),
				NumTrades:   0,
			}
			parseDecimal(&kline.Open, k.Open)
			parseDecimal(&kline.High, k.High)
			parseDecimal(&kline.Low, k.Low)
			parseDecimal(&kline.Close, k.Close)
			parseDecimal(&kline.Volume, k.Volume)
			parseDecimal(&kline.QuoteVolume, k.Turnover)
			_ = interval
			callback(kline)
		}
	}

	return nil
}

func (c *BybitWSClient) subscribeTopic(topic string, _ any) error {
	if c.IsConnected() {
		c.sendSubscribe(topic)
	}
	return nil
}

func (c *BybitWSClient) unsubscribeTopic(topic string) {
	if c.IsConnected() {
		c.sendUnsubscribe(topic)
	}
}

func (c *BybitWSClient) sendSubscribe(topic string) {
	if !c.IsConnected() {
		return
	}

	req := wsRequest{
		Op:   "subscribe",
		Args: []string{topic},
		ReqID: c.requestID.Add(1),
	}

	if err := c.client.SendJSON(req); err != nil {
		c.logger.Error().Err(err).Str("topic", topic).Msg("failed to send subscribe")
	}
	c.logger.Debug().Str("topic", topic).Msg("sent subscribe")
}

func (c *BybitWSClient) sendUnsubscribe(topic string) {
	if !c.IsConnected() {
		return
	}

	req := wsRequest{
		Op:   "unsubscribe",
		Args: []string{topic},
		ReqID: c.requestID.Add(1),
	}

	if err := c.client.SendJSON(req); err != nil {
		c.logger.Error().Err(err).Str("topic", topic).Msg("failed to send unsubscribe")
	}
}

func (c *BybitWSClient) SubscribeTicker(symbol string, callback func(*core.Ticker)) error {
	topic := "tickers." + formatSymbol(symbol)
	key := topic

	c.mu.Lock()
	c.tickerCallbacks[key] = callback
	c.mu.Unlock()

	return c.subscribeTopic(topic, callback)
}

func (c *BybitWSClient) UnsubscribeTicker(symbol string) {
	topic := "tickers." + formatSymbol(symbol)

	c.mu.Lock()
	delete(c.tickerCallbacks, topic)
	c.mu.Unlock()

	c.unsubscribeTopic(topic)
}

func (c *BybitWSClient) SubscribeOrderBook(symbol string, callback func(*core.OrderBook)) error {
	topic := "orderbook." + formatSymbol(symbol) + ".1"
	key := topic

	c.mu.Lock()
	c.orderBookCallbacks[key] = callback
	c.mu.Unlock()

	return c.subscribeTopic(topic, callback)
}

func (c *BybitWSClient) UnsubscribeOrderBook(symbol string) {
	topic := "orderbook." + formatSymbol(symbol) + ".1"

	c.mu.Lock()
	delete(c.orderBookCallbacks, topic)
	c.mu.Unlock()

	c.unsubscribeTopic(topic)
}

func (c *BybitWSClient) SubscribeTrades(symbol string, callback func(*core.Trade)) error {
	topic := "publicTrade." + formatSymbol(symbol)

	c.mu.Lock()
	c.tradeCallbacks[topic] = callback
	c.mu.Unlock()

	return c.subscribeTopic(topic, callback)
}

func (c *BybitWSClient) UnsubscribeTrades(symbol string) {
	topic := "publicTrade." + formatSymbol(symbol)

	c.mu.Lock()
	delete(c.tradeCallbacks, topic)
	c.mu.Unlock()

	c.unsubscribeTopic(topic)
}

func (c *BybitWSClient) SubscribeKlines(symbol, interval string, callback func(*core.Kline)) error {
	// Bybit interval format: 1, 3, 5, 15, 30, 60, 120, 240, 360, 720, D, W, M
	bybitInterval := interval
	switch interval {
	case "1m":
		bybitInterval = "1"
	case "3m":
		bybitInterval = "3"
	case "5m":
		bybitInterval = "5"
	case "15m":
		bybitInterval = "15"
	case "30m":
		bybitInterval = "30"
	case "1h":
		bybitInterval = "60"
	case "2h":
		bybitInterval = "120"
	case "4h":
		bybitInterval = "240"
	case "6h":
		bybitInterval = "360"
	case "12h":
		bybitInterval = "720"
	case "1d", "1D":
		bybitInterval = "D"
	case "1w", "1W":
		bybitInterval = "W"
	case "1M":
		bybitInterval = "M"
	}

	topic := "kline." + bybitInterval + "." + formatSymbol(symbol)

	c.mu.Lock()
	c.klineCallbacks[topic] = callback
	c.mu.Unlock()

	return c.subscribeTopic(topic, callback)
}

func (c *BybitWSClient) UnsubscribeKlines(symbol, interval string) {
	bybitInterval := interval
	switch interval {
	case "1m":
		bybitInterval = "1"
	case "3m":
		bybitInterval = "3"
	case "5m":
		bybitInterval = "5"
	case "15m":
		bybitInterval = "15"
	case "30m":
		bybitInterval = "30"
	case "1h":
		bybitInterval = "60"
	case "2h":
		bybitInterval = "120"
	case "4h":
		bybitInterval = "240"
	case "6h":
		bybitInterval = "360"
	case "12h":
		bybitInterval = "720"
	case "1d", "1D":
		bybitInterval = "D"
	case "1w", "1W":
		bybitInterval = "W"
	case "1M":
		bybitInterval = "M"
	}

	topic := "kline." + bybitInterval + "." + formatSymbol(symbol)

	c.mu.Lock()
	delete(c.klineCallbacks, topic)
	c.mu.Unlock()

	c.unsubscribeTopic(topic)
}

func (c *BybitWSClient) Subscriptions() []string {
	return c.client.Subscriptions()
}

type wsRequest struct {
	Op    string   `json:"op"`
	Args  []string `json:"args"`
	ReqID int64    `json:"reqId"`
}
