package binance

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"sync"

	"github.com/rs/zerolog"
	"resty.dev/v3"

	"sadewa/internal/circuitbreaker"
	httpClient "sadewa/internal/http"
	"sadewa/internal/keyring"
	"sadewa/internal/ratelimit"
	"sadewa/pkg/core"
	"sadewa/pkg/exchange"
)

// BinanceExchange implements the Exchange interface for Binance spot and futures markets.
// It provides rate limiting, circuit breaker, and API key rotation capabilities.
type BinanceExchange struct {
	config         *core.Config
	keyRing        *keyring.KeyRing
	httpClient     *httpClient.Client
	rateLimiter    *ratelimit.RateLimiter
	circuitBreaker *circuitbreaker.Breaker
	logger         zerolog.Logger
	normalizer     *Normalizer
	protocol       *Protocol
	wsClient       *BinanceWSClient
	wsMu           sync.RWMutex
}

// Option is a functional option for configuring the BinanceExchange.
type Option func(*Options)

// Options holds configuration options for the BinanceExchange.
type Options struct {
	KeyRing *keyring.KeyRing
	Logger  zerolog.Logger
}

// WithKeyRing returns an option that sets the API key ring for key rotation.
func WithKeyRing(kr *keyring.KeyRing) Option {
	return func(o *Options) {
		o.KeyRing = kr
	}
}

// WithLogger returns an option that sets the logger for the exchange.
func WithLogger(l zerolog.Logger) Option {
	return func(o *Options) {
		o.Logger = l
	}
}

// New creates a new BinanceExchange instance with the given configuration and options.
// It initializes the HTTP client, rate limiter, and circuit breaker based on the config.
func New(config *core.Config, opts ...Option) (*BinanceExchange, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	options := &Options{
		Logger: zerolog.Nop(),
	}
	for _, opt := range opts {
		opt(options)
	}

	httpClient, err := httpClient.NewClient(&httpClient.Config{
		BaseURL:      baseURL(config),
		Timeout:      config.Timeout,
		MaxRetries:   config.MaxRetries,
		RetryWaitMin: config.RetryWaitMin,
		RetryWaitMax: config.RetryWaitMax,
	})
	if err != nil {
		return nil, fmt.Errorf("create http client: %w", err)
	}

	var rl *ratelimit.RateLimiter
	if config.RateLimitRequests > 0 {
		rl = ratelimit.New(config.RateLimitRequests, config.RateLimitPeriod)
	}

	var cb *circuitbreaker.Breaker
	if config.CircuitBreakerEnabled {
		cb = circuitbreaker.New(circuitbreaker.Config{
			FailThreshold:    config.CircuitBreakerFailThreshold,
			SuccessThreshold: config.CircuitBreakerSuccessThreshold,
			Timeout:          config.CircuitBreakerTimeout,
		})
	}

	return &BinanceExchange{
		config:         config,
		keyRing:        options.KeyRing,
		httpClient:     httpClient,
		rateLimiter:    rl,
		circuitBreaker: cb,
		logger:         options.Logger,
		normalizer:     NewNormalizer(),
		protocol:       NewProtocol(),
	}, nil
}

// baseURL returns the API base URL based on market type and sandbox mode.
func baseURL(config *core.Config) string {
	switch config.MarketType {
	case core.MarketTypeFutures:
		if config.Sandbox {
			return "https://testnet.binancefuture.com"
		}
		return "https://fapi.binance.com"
	default:
		if config.Sandbox {
			return "https://testnet.binance.vision"
		}
		return "https://api.binance.com"
	}
}

// wsURL returns the WebSocket URL based on market type and sandbox mode.
func wsURL(config *core.Config) string {
	switch config.MarketType {
	case core.MarketTypeFutures:
		if config.Sandbox {
			return "wss://stream.binancefuture.com/ws"
		}
		return "wss://fstream.binance.com/ws"
	default:
		if config.Sandbox {
			return "wss://testnet.binance.vision/ws"
		}
		return "wss://stream.binance.com:9443/ws"
	}
}

// Name returns the exchange identifier "binance".
func (e *BinanceExchange) Name() string {
	return "binance"
}

// Version returns the Binance API version.
func (e *BinanceExchange) Version() string {
	return "3"
}

// Close releases resources used by the exchange, including the HTTP client.
func (e *BinanceExchange) Close() error {
	if e.httpClient != nil {
		return e.httpClient.Close()
	}
	return nil
}

// GetTicker retrieves the current ticker for the specified symbol.
func (e *BinanceExchange) GetTicker(ctx context.Context, symbol string, opts ...exchange.Option) (*core.Ticker, error) {
	options := exchange.ApplyOptions(opts...)

	params := core.Params{
		"symbol": symbol,
	}

	req, err := e.protocol.BuildRequest(ctx, core.OpGetTicker, params)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if options.MarketType == core.MarketTypeFutures {
		req.Path = "/fapi/v1/ticker/24hr"
	}

	resp, err := e.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	result, err := e.protocol.ParseResponse(core.OpGetTicker, resp)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	ticker, ok := result.(*core.Ticker)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return ticker, nil
}

// GetOrderBook retrieves the order book for the specified symbol.
func (e *BinanceExchange) GetOrderBook(ctx context.Context, symbol string, opts ...exchange.Option) (*core.OrderBook, error) {
	options := exchange.ApplyOptions(opts...)

	params := core.Params{
		"symbol": symbol,
	}
	if options.Limit > 0 {
		params["limit"] = options.Limit
	}

	req, err := e.protocol.BuildRequest(ctx, core.OpGetOrderBook, params)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if options.MarketType == core.MarketTypeFutures {
		req.Path = "/fapi/v1/depth"
	}

	resp, err := e.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	result, err := e.protocol.ParseResponse(core.OpGetOrderBook, resp)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	orderBook, ok := result.(*core.OrderBook)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	orderBook.Symbol = symbol
	return orderBook, nil
}

// GetTrades retrieves recent trades for the specified symbol as an iterator.
func (e *BinanceExchange) GetTrades(ctx context.Context, symbol string, opts ...exchange.Option) iter.Seq2[*core.Trade, error] {
	return func(yield func(*core.Trade, error) bool) {
		options := exchange.ApplyOptions(opts...)

		params := core.Params{
			"symbol": symbol,
		}
		if options.Limit > 0 {
			params["limit"] = options.Limit
		}

		req, err := e.protocol.BuildRequest(ctx, core.OpGetTrades, params)
		if err != nil {
			yield(nil, fmt.Errorf("build request: %w", err))
			return
		}

		if options.MarketType == core.MarketTypeFutures {
			req.Path = "/fapi/v1/trades"
		}

		resp, err := e.doRequest(ctx, req)
		if err != nil {
			yield(nil, err)
			return
		}

		result, err := e.protocol.ParseResponse(core.OpGetTrades, resp)
		if err != nil {
			yield(nil, fmt.Errorf("parse response: %w", err))
			return
		}

		trades, ok := result.([]core.Trade)
		if !ok {
			yield(nil, fmt.Errorf("unexpected response type: %T", result))
			return
		}

		for i := range trades {
			trade := &trades[i]
			trade.Symbol = symbol
			if !yield(trade, nil) {
				return
			}
		}
	}
}

// GetKlines retrieves candlestick/kline data for the specified symbol.
func (e *BinanceExchange) GetKlines(ctx context.Context, symbol string, opts ...exchange.Option) ([]core.Kline, error) {
	options := exchange.ApplyOptions(opts...)

	params := core.Params{
		"symbol": symbol,
	}
	if options.Interval != "" {
		params["interval"] = options.Interval
	}
	if options.Limit > 0 {
		params["limit"] = options.Limit
	}

	req, err := e.protocol.BuildRequest(ctx, core.OpGetKlines, params)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if options.MarketType == core.MarketTypeFutures {
		req.Path = "/fapi/v1/klines"
	}

	resp, err := e.doRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	result, err := e.protocol.ParseResponse(core.OpGetKlines, resp)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	klines, ok := result.([]core.Kline)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	for i := range klines {
		klines[i].Symbol = symbol
	}

	return klines, nil
}

// GetBalance retrieves account balances for all assets.
func (e *BinanceExchange) GetBalance(ctx context.Context, opts ...exchange.Option) ([]core.Balance, error) {
	options := exchange.ApplyOptions(opts...)

	req, err := e.protocol.BuildRequest(ctx, core.OpGetBalance, core.Params{})
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if options.MarketType == core.MarketTypeFutures {
		req.Path = "/fapi/v2/balance"
	}

	resp, err := e.doSignedRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	result, err := e.protocol.ParseResponse(core.OpGetBalance, resp)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	balances, ok := result.([]core.Balance)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return balances, nil
}

// PlaceOrder submits a new order to the exchange.
func (e *BinanceExchange) PlaceOrder(ctx context.Context, req *exchange.OrderRequest, opts ...exchange.Option) (*core.Order, error) {
	options := exchange.ApplyOptions(opts...)

	params := core.Params{
		"symbol":   req.Symbol,
		"side":     req.Side.String(),
		"type":     req.Type.String(),
		"quantity": req.Quantity.String(),
	}

	if !req.Price.IsZero() {
		params["price"] = req.Price.String()
	}
	if req.TimeInForce != core.GTC {
		params["time_in_force"] = req.TimeInForce.String()
	}
	if req.ClientOrderID != "" {
		params["client_order_id"] = req.ClientOrderID
	}

	coreReq, err := e.protocol.BuildRequest(ctx, core.OpPlaceOrder, params)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if options.MarketType == core.MarketTypeFutures {
		coreReq.Path = "/fapi/v1/order"
	}

	resp, err := e.doSignedRequest(ctx, coreReq)
	if err != nil {
		return nil, err
	}

	result, err := e.protocol.ParseResponse(core.OpPlaceOrder, resp)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	order, ok := result.(*core.Order)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return order, nil
}

// CancelOrder cancels an existing order on the exchange.
func (e *BinanceExchange) CancelOrder(ctx context.Context, req *exchange.CancelRequest, opts ...exchange.Option) (*core.Order, error) {
	options := exchange.ApplyOptions(opts...)

	params := core.Params{
		"symbol": req.Symbol,
	}
	if req.OrderID != "" {
		params["order_id"] = req.OrderID
	}

	coreReq, err := e.protocol.BuildRequest(ctx, core.OpCancelOrder, params)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if options.MarketType == core.MarketTypeFutures {
		coreReq.Path = "/fapi/v1/order"
	}

	resp, err := e.doSignedRequest(ctx, coreReq)
	if err != nil {
		return nil, err
	}

	result, err := e.protocol.ParseResponse(core.OpCancelOrder, resp)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	order, ok := result.(*core.Order)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return order, nil
}

// GetOrder retrieves the current status of an order.
func (e *BinanceExchange) GetOrder(ctx context.Context, req *exchange.OrderQuery, opts ...exchange.Option) (*core.Order, error) {
	options := exchange.ApplyOptions(opts...)

	params := core.Params{
		"symbol": req.Symbol,
	}
	if req.OrderID != "" {
		params["order_id"] = req.OrderID
	}

	coreReq, err := e.protocol.BuildRequest(ctx, core.OpGetOrder, params)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if options.MarketType == core.MarketTypeFutures {
		coreReq.Path = "/fapi/v1/order"
	}

	resp, err := e.doSignedRequest(ctx, coreReq)
	if err != nil {
		return nil, err
	}

	result, err := e.protocol.ParseResponse(core.OpGetOrder, resp)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	order, ok := result.(*core.Order)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return order, nil
}

// GetOpenOrders retrieves all open orders for the account, optionally filtered by symbol.
func (e *BinanceExchange) GetOpenOrders(ctx context.Context, symbol string, opts ...exchange.Option) ([]core.Order, error) {
	options := exchange.ApplyOptions(opts...)

	params := core.Params{}
	if symbol != "" {
		params["symbol"] = symbol
	}

	req, err := e.protocol.BuildRequest(ctx, core.OpGetOpenOrders, params)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if options.MarketType == core.MarketTypeFutures {
		req.Path = "/fapi/v1/openOrders"
	}

	resp, err := e.doSignedRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	result, err := e.protocol.ParseResponse(core.OpGetOpenOrders, resp)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	orders, ok := result.([]core.Order)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return orders, nil
}

// SubscribeTicker subscribes to real-time ticker updates for the specified symbol.
// Returns channels for ticker updates and errors respectively.
func (e *BinanceExchange) SubscribeTicker(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.Ticker, <-chan error) {
	tickerCh := make(chan *core.Ticker, 100)
	errCh := make(chan error, 100)

	go func() {
		defer close(tickerCh)
		defer close(errCh)

		ws, err := e.getWSClient()
		if err != nil {
			errCh <- err
			return
		}

		if err := ws.Connect(ctx); err != nil {
			errCh <- fmt.Errorf("connect websocket: %w", err)
			return
		}

		if err := ws.SubscribeTicker(symbol, func(t *core.Ticker) {
			select {
			case tickerCh <- t:
			default:
			}
		}); err != nil {
			errCh <- fmt.Errorf("subscribe ticker: %w", err)
		}
	}()

	return tickerCh, errCh
}

// SubscribeTrades subscribes to real-time trade updates for the specified symbol.
// Returns channels for trade updates and errors respectively.
func (e *BinanceExchange) SubscribeTrades(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.Trade, <-chan error) {
	tradeCh := make(chan *core.Trade, 100)
	errCh := make(chan error, 100)

	go func() {
		defer close(tradeCh)
		defer close(errCh)

		ws, err := e.getWSClient()
		if err != nil {
			errCh <- err
			return
		}

		if err := ws.Connect(ctx); err != nil {
			errCh <- fmt.Errorf("connect websocket: %w", err)
			return
		}

		if err := ws.SubscribeAggTrades(symbol, func(t *core.Trade) {
			select {
			case tradeCh <- t:
			default:
			}
		}); err != nil {
			errCh <- fmt.Errorf("subscribe trades: %w", err)
		}
	}()

	return tradeCh, errCh
}

// SubscribeOrderBook subscribes to real-time order book updates for the specified symbol.
// Returns channels for order book updates and errors respectively.
func (e *BinanceExchange) SubscribeOrderBook(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.OrderBook, <-chan error) {
	bookCh := make(chan *core.OrderBook, 100)
	errCh := make(chan error, 100)

	go func() {
		defer close(bookCh)
		defer close(errCh)

		ws, err := e.getWSClient()
		if err != nil {
			errCh <- err
			return
		}

		if err := ws.Connect(ctx); err != nil {
			errCh <- fmt.Errorf("connect websocket: %w", err)
			return
		}

		if err := ws.SubscribeOrderBook(symbol, func(ob *core.OrderBook) {
			ob.Symbol = symbol
			select {
			case bookCh <- ob:
			default:
			}
		}); err != nil {
			errCh <- fmt.Errorf("subscribe orderbook: %w", err)
		}
	}()

	return bookCh, errCh
}

func (e *BinanceExchange) getWSClient() (*BinanceWSClient, error) {
	e.wsMu.RLock()
	if e.wsClient != nil {
		e.wsMu.RUnlock()
		return e.wsClient, nil
	}
	e.wsMu.RUnlock()

	e.wsMu.Lock()
	defer e.wsMu.Unlock()

	if e.wsClient != nil {
		return e.wsClient, nil
	}

	wsConfig := BinanceWSConfig{
		Sandbox: e.config.Sandbox,
	}
	e.wsClient = NewBinanceWSClientWithConfig(wsConfig)
	e.wsClient.SetLogger(e.logger)

	return e.wsClient, nil
}

func (e *BinanceExchange) doRequest(ctx context.Context, req *core.Request) (*resty.Response, error) {
	if e.rateLimiter != nil {
		if err := e.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit: %w", err)
		}
	}

	if e.circuitBreaker != nil && !e.circuitBreaker.Allow() {
		return nil, fmt.Errorf("circuit breaker open")
	}

	var resp *resty.Response
	var err error

	switch req.Method {
	case http.MethodGet:
		resp, err = e.httpClient.Get(ctx, req.Path, e.buildRequestOptions(req)...)
	case http.MethodPost:
		resp, err = e.httpClient.Post(ctx, req.Path, req.Body, e.buildRequestOptions(req)...)
	case http.MethodDelete:
		resp, err = e.httpClient.Delete(ctx, req.Path, e.buildRequestOptions(req)...)
	default:
		return nil, fmt.Errorf("unsupported method: %s", req.Method)
	}

	if e.circuitBreaker != nil {
		e.circuitBreaker.Record(err == nil)
	}

	return resp, err
}

func (e *BinanceExchange) doSignedRequest(ctx context.Context, req *core.Request) (*resty.Response, error) {
	if e.keyRing == nil && e.config.Credentials == nil {
		return nil, fmt.Errorf("no credentials configured")
	}

	var creds core.Credentials
	if e.keyRing != nil {
		key := e.keyRing.Current()
		if key == nil {
			return nil, fmt.Errorf("no available API key")
		}
		creds = core.Credentials{
			APIKey:     key.Key,
			SecretKey:  key.Secret,
			Passphrase: key.Passphrase,
		}
		e.keyRing.MarkUsed()
	} else {
		creds = *e.config.Credentials
	}

	restyReq := e.httpClient.Request().SetContext(ctx)

	for k, v := range req.Headers {
		restyReq.SetHeader(k, v)
	}

	for k, v := range req.Query {
		restyReq.SetQueryParam(k, fmt.Sprint(v))
	}

	if err := e.protocol.SignRequest(restyReq, creds); err != nil {
		return nil, fmt.Errorf("sign request: %w", err)
	}

	if e.rateLimiter != nil {
		if err := e.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit: %w", err)
		}
	}

	if e.circuitBreaker != nil && !e.circuitBreaker.Allow() {
		return nil, fmt.Errorf("circuit breaker open")
	}

	var resp *resty.Response
	var err error

	switch req.Method {
	case http.MethodGet:
		resp, err = restyReq.Get(req.Path)
	case http.MethodPost:
		resp, err = restyReq.Post(req.Path)
	case http.MethodDelete:
		resp, err = restyReq.Delete(req.Path)
	default:
		return nil, fmt.Errorf("unsupported method: %s", req.Method)
	}

	if e.circuitBreaker != nil {
		e.circuitBreaker.Record(err == nil)
	}

	if e.keyRing != nil && err != nil {
		e.keyRing.OnError(err)
	}

	return resp, err
}

func (e *BinanceExchange) buildRequestOptions(req *core.Request) []httpClient.RequestOption {
	var opts []httpClient.RequestOption

	for k, v := range req.Headers {
		opts = append(opts, httpClient.WithHeader(k, v))
	}

	for k, v := range req.Query {
		opts = append(opts, httpClient.WithQueryParam(k, fmt.Sprint(v)))
	}

	return opts
}

// Register creates a BinanceExchange and registers it with the container.
// This is a convenience function for dependency injection setup.
func Register(container *exchange.Container, config *core.Config, opts ...Option) error {
	ex, err := New(config, opts...)
	if err != nil {
		return fmt.Errorf("create binance exchange: %w", err)
	}
	container.Register("binance", ex)
	return nil
}
