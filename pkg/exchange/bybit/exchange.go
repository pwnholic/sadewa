package bybit

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

// BybitExchange implements the Exchange interface for Bybit spot and futures markets.
// It provides rate limiting, circuit breaker, and API key rotation capabilities.
type BybitExchange struct {
	config         *core.Config
	keyRing        *keyring.KeyRing
	httpClient     *httpClient.Client
	rateLimiter    *ratelimit.RateLimiter
	circuitBreaker *circuitbreaker.Breaker
	logger         zerolog.Logger
	normalizer     *Normalizer
	protocol       *Protocol
	wsClient       *BybitWSClient
	wsMu           sync.RWMutex
}

// Option is a functional option for configuring the BybitExchange.
type Option func(*Options)

// Options holds configuration options for the BybitExchange.
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

// New creates a new BybitExchange instance with the given configuration and options.
// It initializes the HTTP client, rate limiter, and circuit breaker based on the config.
func New(config *core.Config, opts ...Option) (*BybitExchange, error) {
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
		BaseURL:      getBaseURL(config),
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

	return &BybitExchange{
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
func getBaseURL(config *core.Config) string {
	if config.Sandbox {
		return "https://api-testnet.bybit.com"
	}
	return "https://api.bybit.com"
}

// wsURL returns the WebSocket URL based on market type and sandbox mode.
func getWebsocketURL(config *core.Config) string {
	if config.Sandbox {
		return "wss://stream-testnet.bybit.com/v5/public/spot"
	}
	return "wss://stream.bybit.com/v5/public/spot"
}

// Name returns the exchange identifier "bybit".
func (e *BybitExchange) Name() string {
	return "bybit"
}

// Version returns the Bybit API version.
func (e *BybitExchange) Version() string {
	return "5"
}

// Close releases resources used by the exchange, including the HTTP client.
func (e *BybitExchange) Close() error {
	if e.httpClient != nil {
		return e.httpClient.Close()
	}
	return nil
}

// GetTicker retrieves the current ticker for the specified symbol.
func (e *BybitExchange) GetTicker(ctx context.Context, symbol string, opts ...exchange.Option) (*core.Ticker, error) {
	options := exchange.ApplyOptions(opts...)

	category := "spot"
	if options.MarketType == core.MarketTypeFutures {
		category = "linear"
	}

	params := core.Params{
		"symbol":   symbol,
		"category": category,
	}

	req, err := e.protocol.BuildRequest(ctx, core.OpGetTicker, params)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
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
func (e *BybitExchange) GetOrderBook(ctx context.Context, symbol string, opts ...exchange.Option) (*core.OrderBook, error) {
	options := exchange.ApplyOptions(opts...)

	category := "spot"
	if options.MarketType == core.MarketTypeFutures {
		category = "linear"
	}

	params := core.Params{
		"symbol":   symbol,
		"category": category,
	}
	if options.Limit > 0 {
		params["limit"] = options.Limit
	}

	req, err := e.protocol.BuildRequest(ctx, core.OpGetOrderBook, params)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
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
func (e *BybitExchange) GetTrades(ctx context.Context, symbol string, opts ...exchange.Option) iter.Seq2[*core.Trade, error] {
	return func(yield func(*core.Trade, error) bool) {
		options := exchange.ApplyOptions(opts...)

		category := "spot"
		if options.MarketType == core.MarketTypeFutures {
			category = "linear"
		}

		params := core.Params{
			"symbol":   symbol,
			"category": category,
		}
		if options.Limit > 0 {
			params["limit"] = options.Limit
		}

		req, err := e.protocol.BuildRequest(ctx, core.OpGetTrades, params)
		if err != nil {
			yield(nil, fmt.Errorf("build request: %w", err))
			return
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
func (e *BybitExchange) GetKlines(ctx context.Context, symbol string, opts ...exchange.Option) ([]core.Kline, error) {
	options := exchange.ApplyOptions(opts...)

	category := "spot"
	if options.MarketType == core.MarketTypeFutures {
		category = "linear"
	}

	params := core.Params{
		"symbol":   symbol,
		"category": category,
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
func (e *BybitExchange) GetBalance(ctx context.Context, opts ...exchange.Option) ([]core.Balance, error) {
	_ = exchange.ApplyOptions(opts...)

	req, err := e.protocol.BuildRequest(ctx, core.OpGetBalance, core.Params{})
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
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
func (e *BybitExchange) PlaceOrder(ctx context.Context, req *exchange.OrderRequest, opts ...exchange.Option) (*core.Order, error) {
	options := exchange.ApplyOptions(opts...)

	category := "spot"
	if options.MarketType == core.MarketTypeFutures {
		category = "linear"
	}

	params := core.Params{
		"symbol":   req.Symbol,
		"side":     req.Side.String(),
		"type":     req.Type.String(),
		"quantity": req.Quantity.String(),
		"category": category,
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
func (e *BybitExchange) CancelOrder(ctx context.Context, req *exchange.CancelRequest, opts ...exchange.Option) (*core.Order, error) {
	options := exchange.ApplyOptions(opts...)

	category := "spot"
	if options.MarketType == core.MarketTypeFutures {
		category = "linear"
	}

	params := core.Params{
		"symbol":   req.Symbol,
		"category": category,
	}
	if req.OrderID != "" {
		params["order_id"] = req.OrderID
	}

	coreReq, err := e.protocol.BuildRequest(ctx, core.OpCancelOrder, params)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
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
func (e *BybitExchange) GetOrder(ctx context.Context, req *exchange.OrderQuery, opts ...exchange.Option) (*core.Order, error) {
	options := exchange.ApplyOptions(opts...)

	category := "spot"
	if options.MarketType == core.MarketTypeFutures {
		category = "linear"
	}

	params := core.Params{
		"symbol":   req.Symbol,
		"category": category,
	}
	if req.OrderID != "" {
		params["order_id"] = req.OrderID
	}

	coreReq, err := e.protocol.BuildRequest(ctx, core.OpGetOrder, params)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
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
func (e *BybitExchange) GetOpenOrders(ctx context.Context, symbol string, opts ...exchange.Option) ([]core.Order, error) {
	options := exchange.ApplyOptions(opts...)

	category := "spot"
	if options.MarketType == core.MarketTypeFutures {
		category = "linear"
	}

	params := core.Params{
		"category": category,
	}
	if symbol != "" {
		params["symbol"] = symbol
	}

	req, err := e.protocol.BuildRequest(ctx, core.OpGetOpenOrders, params)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
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
func (e *BybitExchange) SubscribeTicker(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.Ticker, <-chan error) {
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
			case <-ctx.Done():
			default:
				e.logger.Warn().Str("symbol", symbol).Msg("ticker channel full, message dropped")
			}
		}); err != nil {
			errCh <- fmt.Errorf("subscribe ticker: %w", err)
			return
		}

		// Wait for context cancellation
		<-ctx.Done()
		ws.UnsubscribeTicker(symbol)
	}()

	return tickerCh, errCh
}

// SubscribeTrades subscribes to real-time trade updates for the specified symbol.
// Returns channels for trade updates and errors respectively.
func (e *BybitExchange) SubscribeTrades(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.Trade, <-chan error) {
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

		if err := ws.SubscribeTrades(symbol, func(t *core.Trade) {
			select {
			case tradeCh <- t:
			case <-ctx.Done():
			default:
				e.logger.Warn().Str("symbol", symbol).Msg("trade channel full, message dropped")
			}
		}); err != nil {
			errCh <- fmt.Errorf("subscribe trades: %w", err)
			return
		}

		// Wait for context cancellation
		<-ctx.Done()
		ws.UnsubscribeTrades(symbol)
	}()

	return tradeCh, errCh
}

// SubscribeOrderBook subscribes to real-time order book updates for the specified symbol.
// Returns channels for order book updates and errors respectively.
func (e *BybitExchange) SubscribeOrderBook(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.OrderBook, <-chan error) {
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
			case <-ctx.Done():
			default:
				e.logger.Warn().Str("symbol", symbol).Msg("orderbook channel full, message dropped")
			}
		}); err != nil {
			errCh <- fmt.Errorf("subscribe orderbook: %w", err)
			return
		}

		// Wait for context cancellation
		<-ctx.Done()
		ws.UnsubscribeOrderBook(symbol)
	}()

	return bookCh, errCh
}

// SubscribeKlines subscribes to real-time kline updates for the specified symbol.
// Returns channels for kline updates and errors respectively.
func (e *BybitExchange) SubscribeKlines(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.Kline, <-chan error) {
	klineCh := make(chan *core.Kline, 100)
	errCh := make(chan error, 100)

	go func() {
		defer close(klineCh)
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

		if err := ws.SubscribeKlines(symbol, "1", func(k *core.Kline) {
			select {
			case klineCh <- k:
			case <-ctx.Done():
			default:
				e.logger.Warn().Str("symbol", symbol).Msg("kline channel full, message dropped")
			}
		}); err != nil {
			errCh <- fmt.Errorf("subscribe klines: %w", err)
			return
		}

		// Wait for context cancellation
		<-ctx.Done()
		ws.UnsubscribeKlines(symbol, "1")
	}()

	return klineCh, errCh
}

func (e *BybitExchange) getWSClient() (*BybitWSClient, error) {
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

	wsConfig := BybitWSConfig{
		Sandbox: e.config.Sandbox,
	}
	e.wsClient = NewBybitWSClientWithConfig(wsConfig)
	e.wsClient.SetLogger(e.logger)

	return e.wsClient, nil
}

func (e *BybitExchange) doRequest(ctx context.Context, req *core.Request) (*resty.Response, error) {
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

func (e *BybitExchange) doSignedRequest(ctx context.Context, req *core.Request) (*resty.Response, error) {
	if e.keyRing == nil && e.config.Credentials == nil {
		return nil, core.NewExchangeError(e.Name(), core.ErrorTypeAuthentication, 401,
			"no credentials configured").WithCode(core.ErrCodeNoCredentials)
	}

	var creds core.Credentials
	if e.keyRing != nil {
		key := e.keyRing.Current()
		if key == nil {
			return nil, core.NewExchangeError(e.Name(), core.ErrorTypeAuthentication, 401,
				"no available API key").WithCode(core.ErrCodeNoAPIKey)
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

func (e *BybitExchange) buildRequestOptions(req *core.Request) []httpClient.RequestOption {
	var opts []httpClient.RequestOption

	for k, v := range req.Headers {
		opts = append(opts, httpClient.WithHeader(k, v))
	}

	for k, v := range req.Query {
		opts = append(opts, httpClient.WithQueryParam(k, fmt.Sprint(v)))
	}

	return opts
}

// Register creates a BybitExchange and registers it with the container.
// This is a convenience function for dependency injection setup.
func Register(container *exchange.Container, config *core.Config, opts ...Option) error {
	ex, err := New(config, opts...)
	if err != nil {
		return fmt.Errorf("create bybit exchange: %w", err)
	}
	container.Register("bybit", ex)
	return nil
}
