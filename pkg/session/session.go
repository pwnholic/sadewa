package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/rs/zerolog"

	"sadewa/internal/circuitbreaker"
	"sadewa/internal/ratelimit"
	"sadewa/pkg/core"
)

// State represents the lifecycle state of a Session.
type State int

const (
	// StateNew indicates a newly created session that has not yet been activated.
	StateNew State = iota
	// StateActive indicates a session that is ready to process requests.
	StateActive
	// StateClosed indicates a session that has been shut down and can no longer be used.
	StateClosed
)

// String returns the string representation of the State.
func (s State) String() string {
	return [...]string{"NEW", "ACTIVE", "CLOSED"}[s]
}

// Session represents a stateful connection to an exchange.
// It manages authentication, rate limiting, circuit breaking, caching, and request execution.
// Sessions are safe for concurrent use.
type Session struct {
	mu             sync.RWMutex
	config         *core.Config
	protocol       core.Protocol
	credentials    *core.Credentials
	rateLimiter    *ratelimit.RateLimiter
	circuitBreaker *circuitbreaker.Breaker
	cache          *Cache
	logger         zerolog.Logger
	state          State
	createdAt      time.Time
	lastUsed       time.Time
}

// Cache provides a simple in-memory cache with TTL support.
// It is safe for concurrent use.
type Cache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
	ttl   time.Duration
}

type cacheItem struct {
	value     any
	expiresAt time.Time
}

// NewCache creates a new Cache instance with the specified default TTL.
// Items stored without an explicit TTL will use the default TTL provided.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		items: make(map[string]*cacheItem),
		ttl:   ttl,
	}
}

// Get retrieves a value from the cache by key.
// Returns nil if the key does not exist or the item has expired.
func (c *Cache) Get(ctx context.Context, key string) (any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, nil
	}

	if time.Now().After(item.expiresAt) {
		return nil, nil
	}

	return item.value, nil
}

// Set stores a value in the cache with the specified key and TTL.
// If TTL is zero, the cache's default TTL is used.
func (c *Cache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl == 0 {
		ttl = c.ttl
	}

	c.items[key] = &cacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

// Delete removes an item from the cache by key.
// No error is returned if the key does not exist.
func (c *Cache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	return nil
}

// Clear removes all items from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheItem)
}

// New creates a new Session with the provided configuration.
// The configuration is validated before the session is created.
// Returns an error if the configuration is nil or invalid.
func New(config *core.Config) (*Session, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	logger := zerolog.Nop()
	if config.LogLevel != "" {
		level, err := zerolog.ParseLevel(config.LogLevel)
		if err != nil {
			level = zerolog.InfoLevel
		}
		zerolog.SetGlobalLevel(level)
	}

	rateLimiter := ratelimit.New(config.RateLimitRequests, config.RateLimitPeriod)

	var circuitBreaker *circuitbreaker.Breaker
	if config.CircuitBreakerEnabled {
		circuitBreaker = circuitbreaker.New(circuitbreaker.Config{
			FailThreshold:    config.CircuitBreakerFailThreshold,
			SuccessThreshold: config.CircuitBreakerSuccessThreshold,
			Timeout:          config.CircuitBreakerTimeout,
		})
	}

	var cache *Cache
	if config.CacheEnabled {
		cache = NewCache(config.CacheTTL)
	}

	session := &Session{
		config:         config,
		credentials:    config.Credentials,
		rateLimiter:    rateLimiter,
		circuitBreaker: circuitBreaker,
		cache:          cache,
		logger:         logger,
		state:          StateNew,
		createdAt:      time.Now(),
		lastUsed:       time.Now(),
	}

	return session, nil
}

// SetProtocol assigns the exchange protocol to the session.
// The session state transitions to Active if currently in New state.
// Returns an error if the protocol is nil.
func (s *Session) SetProtocol(protocol core.Protocol) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if protocol == nil {
		return fmt.Errorf("protocol is required")
	}

	s.protocol = protocol

	if s.state == StateNew {
		s.state = StateActive
	}

	s.lastUsed = time.Now()

	return nil
}

// Do executes an operation against the exchange.
// It handles rate limiting, circuit breaker protection, caching, and authentication automatically.
// Returns the parsed response on success, or an error if the operation fails.
func (s *Session) Do(ctx context.Context, op core.Operation, params core.Params) (any, error) {
	s.mu.Lock()
	if s.protocol == nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("protocol not set")
	}
	s.lastUsed = time.Now()
	s.mu.Unlock()

	req, err := s.protocol.BuildRequest(ctx, op, params)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if req.CacheKey != "" && s.cache != nil {
		cached, err := s.cache.Get(ctx, req.CacheKey)
		if err != nil {
			s.logger.Warn().Err(err).Str("cache_key", req.CacheKey).Msg("cache get error")
		}
		if cached != nil {
			s.logger.Debug().Str("cache_key", req.CacheKey).Msg("cache hit")
			return cached, nil
		}
	}

	if s.circuitBreaker != nil {
		if !s.circuitBreaker.Allow() {
			return nil, core.NewExchangeError(
				s.config.Exchange,
				core.ErrorTypeServerError,
				503,
				"circuit breaker is open",
			)
		}
	}

	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

	baseURL := s.protocol.BaseURL(s.config.Sandbox)
	restyReq := s.buildRestyRequest(ctx, req, baseURL)

	if req.RequireAuth && s.credentials != nil {
		if err := s.protocol.SignRequest(restyReq, *s.credentials); err != nil {
			return nil, fmt.Errorf("sign request: %w", err)
		}
	}

	var resp *resty.Response
	switch req.Method {
	case "GET":
		resp, err = restyReq.Get(req.Path)
	case "POST":
		resp, err = restyReq.Post(req.Path)
	case "PUT":
		resp, err = restyReq.Put(req.Path)
	case "DELETE":
		resp, err = restyReq.Delete(req.Path)
	case "PATCH":
		resp, err = restyReq.Patch(req.Path)
	default:
		return nil, fmt.Errorf("unsupported method: %s", req.Method)
	}

	success := err == nil && resp != nil && resp.IsSuccess()
	if s.circuitBreaker != nil {
		s.circuitBreaker.Record(success)
	}

	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.IsError() {
		excErr := core.NewExchangeError(
			s.config.Exchange,
			s.mapStatusCodeToErrorType(resp.StatusCode()),
			resp.StatusCode(),
			string(resp.Body()),
		)
		return nil, excErr
	}

	result, err := s.protocol.ParseResponse(op, resp)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if req.CacheKey != "" && s.cache != nil && result != nil {
		cacheTTL := req.CacheTTL
		if cacheTTL == 0 {
			cacheTTL = s.config.CacheTTL
		}
		if err := s.cache.Set(ctx, req.CacheKey, result, cacheTTL); err != nil {
			s.logger.Warn().Err(err).Str("cache_key", req.CacheKey).Msg("cache set error")
		}
	}

	return result, nil
}

func (s *Session) buildRestyRequest(ctx context.Context, req *core.Request, baseURL string) *resty.Request {
	client := resty.New()
	client.SetBaseURL(baseURL)
	client.SetTimeout(s.config.Timeout)

	r := client.R().SetContext(ctx)

	for k, v := range req.Headers {
		r.SetHeader(k, v)
	}

	if req.Query != nil {
		query := make(map[string]string)
		for k, v := range req.Query {
			query[k] = fmt.Sprintf("%v", v)
		}
		r.SetQueryParams(query)
	}

	if req.Body != nil {
		r.SetBody(req.Body)
	}

	return r
}

func (s *Session) mapStatusCodeToErrorType(statusCode int) core.ErrorType {
	switch {
	case statusCode >= 500:
		return core.ErrorTypeServerError
	case statusCode == 429:
		return core.ErrorTypeRateLimit
	case statusCode == 401 || statusCode == 403:
		return core.ErrorTypeAuthentication
	case statusCode == 400:
		return core.ErrorTypeBadRequest
	case statusCode == 404:
		return core.ErrorTypeNotFound
	default:
		return core.ErrorTypeUnknown
	}
}

// Close shuts down the session and releases resources.
// It clears the cache and transitions the session to Closed state.
// After closing, the session should not be used for further operations.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache != nil {
		s.cache.Clear()
	}

	s.state = StateClosed
	return nil
}

// State returns the current lifecycle state of the session.
func (s *Session) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Protocol returns the exchange protocol assigned to the session.
// Returns nil if no protocol has been set.
func (s *Session) Protocol() core.Protocol {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.protocol
}

// Config returns the configuration used to create the session.
func (s *Session) Config() *core.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// CreatedAt returns the timestamp when the session was created.
func (s *Session) CreatedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.createdAt
}

// LastUsed returns the timestamp of the last request executed by the session.
func (s *Session) LastUsed() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastUsed
}

// SetCredentials updates the API credentials used for authenticated requests.
func (s *Session) SetCredentials(creds *core.Credentials) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.credentials = creds
}

// ClearCache removes all cached items from the session's cache.
// If caching is disabled, this method does nothing.
func (s *Session) ClearCache() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cache != nil {
		s.cache.Clear()
	}
}

// GetTicker fetches the 24-hour ticker for a symbol.
// Returns the ticker with current bid, ask, last price, and volume information.
func (s *Session) GetTicker(ctx context.Context, symbol string) (*core.Ticker, error) {
	result, err := s.Do(ctx, core.OpGetTicker, core.Params{"symbol": symbol})
	if err != nil {
		return nil, err
	}
	ticker, ok := result.(*core.Ticker)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}
	return ticker, nil
}

// GetOrderBook fetches the order book for a symbol with the specified depth limit.
// Returns bids sorted by price descending and asks sorted by price ascending.
func (s *Session) GetOrderBook(ctx context.Context, symbol string, limit int) (*core.OrderBook, error) {
	result, err := s.Do(ctx, core.OpGetOrderBook, core.Params{
		"symbol": symbol,
		"limit":  limit,
	})
	if err != nil {
		return nil, err
	}
	orderBook, ok := result.(*core.OrderBook)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}
	orderBook.Symbol = symbol
	return orderBook, nil
}

// GetTrades fetches recent trades for a symbol.
// Returns trades sorted by time descending with a maximum of limit results.
func (s *Session) GetTrades(ctx context.Context, symbol string, limit int) ([]core.Trade, error) {
	result, err := s.Do(ctx, core.OpGetTrades, core.Params{
		"symbol": symbol,
		"limit":  limit,
	})
	if err != nil {
		return nil, err
	}
	trades, ok := result.([]core.Trade)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}
	for i := range trades {
		trades[i].Symbol = symbol
	}
	return trades, nil
}

// GetKlines fetches candlestick/kline data for a symbol.
// Interval specifies the time frame (e.g., "1m", "5m", "1h", "1d").
func (s *Session) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]core.Kline, error) {
	result, err := s.Do(ctx, core.OpGetKlines, core.Params{
		"symbol":   symbol,
		"interval": interval,
		"limit":    limit,
	})
	if err != nil {
		return nil, err
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

// GetBalance fetches account balances for all assets.
// Requires authenticated session with valid API credentials.
func (s *Session) GetBalance(ctx context.Context) ([]core.Balance, error) {
	result, err := s.Do(ctx, core.OpGetBalance, nil)
	if err != nil {
		return nil, err
	}
	balances, ok := result.([]core.Balance)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}
	return balances, nil
}

// PlaceOrder submits a new order to the exchange.
// Requires authenticated session with valid API credentials.
// Returns the created order with exchange-assigned ID and initial status.
func (s *Session) PlaceOrder(ctx context.Context, order *core.Order) (*core.Order, error) {
	params := core.Params{
		"symbol":   order.Symbol,
		"side":     order.Side.String(),
		"type":     order.Type.String(),
		"quantity": order.Quantity.String(),
	}

	if !order.Price.IsZero() {
		params["price"] = order.Price.String()
	}

	if order.TimeInForce != core.GTC {
		params["time_in_force"] = order.TimeInForce.String()
	}

	if order.ClientOrderID != "" {
		params["client_order_id"] = order.ClientOrderID
	}

	result, err := s.Do(ctx, core.OpPlaceOrder, params)
	if err != nil {
		return nil, err
	}

	placedOrder, ok := result.(*core.Order)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return placedOrder, nil
}

// CancelOrder cancels an existing order by its exchange-assigned ID.
// Requires authenticated session with valid API credentials.
func (s *Session) CancelOrder(ctx context.Context, symbol string, orderID string) (*core.Order, error) {
	result, err := s.Do(ctx, core.OpCancelOrder, core.Params{
		"symbol":   symbol,
		"order_id": orderID,
	})
	if err != nil {
		return nil, err
	}

	order, ok := result.(*core.Order)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return order, nil
}

// CancelOrderByClientID cancels an existing order by its client-assigned ID.
// Requires authenticated session with valid API credentials.
func (s *Session) CancelOrderByClientID(ctx context.Context, symbol string, clientOrderID string) (*core.Order, error) {
	result, err := s.Do(ctx, core.OpCancelOrder, core.Params{
		"symbol":          symbol,
		"client_order_id": clientOrderID,
	})
	if err != nil {
		return nil, err
	}

	order, ok := result.(*core.Order)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return order, nil
}

// GetOrder fetches an order by its exchange-assigned ID.
// Requires authenticated session with valid API credentials.
func (s *Session) GetOrder(ctx context.Context, symbol string, orderID string) (*core.Order, error) {
	result, err := s.Do(ctx, core.OpGetOrder, core.Params{
		"symbol":   symbol,
		"order_id": orderID,
	})
	if err != nil {
		return nil, err
	}

	order, ok := result.(*core.Order)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return order, nil
}

// GetOrderByClientID fetches an order by its client-assigned ID.
// Requires authenticated session with valid API credentials.
func (s *Session) GetOrderByClientID(ctx context.Context, symbol string, clientOrderID string) (*core.Order, error) {
	result, err := s.Do(ctx, core.OpGetOrder, core.Params{
		"symbol":          symbol,
		"client_order_id": clientOrderID,
	})
	if err != nil {
		return nil, err
	}

	order, ok := result.(*core.Order)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return order, nil
}

// GetOpenOrders fetches all open orders for a symbol.
// If symbol is empty, fetches open orders for all symbols.
// Requires authenticated session with valid API credentials.
func (s *Session) GetOpenOrders(ctx context.Context, symbol string) ([]core.Order, error) {
	params := core.Params{}
	if symbol != "" {
		params["symbol"] = symbol
	}

	result, err := s.Do(ctx, core.OpGetOpenOrders, params)
	if err != nil {
		return nil, err
	}

	orders, ok := result.([]core.Order)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return orders, nil
}

// GetOrderHistory fetches order history for a symbol.
// Requires authenticated session with valid API credentials.
func (s *Session) GetOrderHistory(ctx context.Context, symbol string, startTime, endTime int64, limit int) ([]core.Order, error) {
	params := core.Params{
		"symbol": symbol,
	}

	if startTime > 0 {
		params["start_time"] = startTime
	}

	if endTime > 0 {
		params["end_time"] = endTime
	}

	if limit > 0 {
		params["limit"] = limit
	}

	result, err := s.Do(ctx, core.OpGetOrderHistory, params)
	if err != nil {
		return nil, err
	}

	orders, ok := result.([]core.Order)
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	return orders, nil
}
