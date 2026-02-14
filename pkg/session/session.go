package session

import (
	"context"
	"fmt"
	"iter"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"sadewa/internal/circuitbreaker"
	"sadewa/internal/ratelimit"
	"sadewa/pkg/core"
	"sadewa/pkg/exchange"
)

type SessionState int32

const (
	SessionStateNew SessionState = iota
	SessionStateActive
	SessionStateClosed
)

func (s SessionState) String() string {
	return [...]string{"NEW", "ACTIVE", "CLOSED"}[s]
}

type Session struct {
	container      *exchange.Container
	config         *core.Config
	ex             exchange.Exchange
	rateLimiter    *ratelimit.RateLimiter
	circuitBreaker *circuitbreaker.Breaker
	cache          *Cache
	logger         zerolog.Logger
	state          atomic.Int32
	mu             sync.RWMutex
}

type Cache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
	ttl   time.Duration
}

type cacheItem struct {
	value     any
	expiresAt time.Time
}

func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		items: make(map[string]*cacheItem),
		ttl:   ttl,
	}
}

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

func (c *Cache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	return nil
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheItem)
}

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
		container:      exchange.NewContainer(),
		config:         config,
		rateLimiter:    rateLimiter,
		circuitBreaker: circuitBreaker,
		cache:          cache,
		logger:         logger,
	}
	session.state.Store(int32(SessionStateNew))

	return session, nil
}

func NewSession(container *exchange.Container, config *core.Config) (*Session, error) {
	if container == nil {
		return nil, fmt.Errorf("container is required")
	}
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

	var cb *circuitbreaker.Breaker
	if config.CircuitBreakerEnabled {
		cb = circuitbreaker.New(circuitbreaker.Config{
			FailThreshold:    config.CircuitBreakerFailThreshold,
			SuccessThreshold: config.CircuitBreakerSuccessThreshold,
			Timeout:          config.CircuitBreakerTimeout,
		})
	}

	var cache *Cache
	if config.CacheEnabled {
		cache = NewCache(config.CacheTTL)
	}

	s := &Session{
		container:      container,
		config:         config,
		rateLimiter:    rateLimiter,
		circuitBreaker: cb,
		cache:          cache,
		logger:         logger,
	}
	s.state.Store(int32(SessionStateNew))

	return s, nil
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache != nil {
		s.cache.Clear()
	}

	s.state.Store(int32(SessionStateClosed))
	return nil
}

func (s *Session) State() SessionState {
	return SessionState(s.state.Load())
}

func (s *Session) SetExchange(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ex, err := s.container.Get(name)
	if err != nil {
		return fmt.Errorf("get exchange: %w", err)
	}

	s.ex = ex

	if SessionState(s.state.Load()) == SessionStateNew {
		s.state.Store(int32(SessionStateActive))
	}

	return nil
}

func (s *Session) RegisterExchange(name string, ex exchange.Exchange) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.container.Register(name, ex)
}

func (s *Session) CurrentExchange() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ex == nil {
		return ""
	}
	return s.ex.Name()
}

func (s *Session) exchange() (exchange.Exchange, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ex == nil {
		return nil, fmt.Errorf("exchange not set")
	}
	return s.ex, nil
}

func (s *Session) GetTicker(ctx context.Context, symbol string, opts ...exchange.Option) (*core.Ticker, error) {
	ex, err := s.exchange()
	if err != nil {
		return nil, err
	}

	if s.circuitBreaker != nil && !s.circuitBreaker.Allow() {
		return nil, core.NewExchangeError(
			ex.Name(),
			core.ErrorTypeServerError,
			503,
			"circuit breaker is open",
		)
	}

	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

	ticker, err := ex.GetTicker(ctx, symbol, opts...)
	if s.circuitBreaker != nil {
		s.circuitBreaker.Record(err == nil)
	}
	return ticker, err
}

func (s *Session) GetOrderBook(ctx context.Context, symbol string, opts ...exchange.Option) (*core.OrderBook, error) {
	ex, err := s.exchange()
	if err != nil {
		return nil, err
	}

	if s.circuitBreaker != nil && !s.circuitBreaker.Allow() {
		return nil, core.NewExchangeError(
			ex.Name(),
			core.ErrorTypeServerError,
			503,
			"circuit breaker is open",
		)
	}

	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

	orderBook, err := ex.GetOrderBook(ctx, symbol, opts...)
	if s.circuitBreaker != nil {
		s.circuitBreaker.Record(err == nil)
	}
	return orderBook, err
}

func (s *Session) GetTrades(ctx context.Context, symbol string, opts ...exchange.Option) iter.Seq2[*core.Trade, error] {
	return func(yield func(*core.Trade, error) bool) {
		ex, err := s.exchange()
		if err != nil {
			yield(nil, err)
			return
		}

		if s.circuitBreaker != nil && !s.circuitBreaker.Allow() {
			yield(nil, core.NewExchangeError(
				ex.Name(),
				core.ErrorTypeServerError,
				503,
				"circuit breaker is open",
			))
			return
		}

		if s.rateLimiter != nil {
			if err := s.rateLimiter.Wait(ctx); err != nil {
				yield(nil, fmt.Errorf("rate limit wait: %w", err))
				return
			}
		}

		var hasError bool
		for trade, err := range ex.GetTrades(ctx, symbol, opts...) {
			if !yield(trade, err) {
				return
			}
			if err != nil {
				hasError = true
			}
		}

		if s.circuitBreaker != nil {
			s.circuitBreaker.Record(!hasError)
		}
	}
}

func (s *Session) GetKlines(ctx context.Context, symbol string, opts ...exchange.Option) ([]core.Kline, error) {
	ex, err := s.exchange()
	if err != nil {
		return nil, err
	}

	if s.circuitBreaker != nil && !s.circuitBreaker.Allow() {
		return nil, core.NewExchangeError(
			ex.Name(),
			core.ErrorTypeServerError,
			503,
			"circuit breaker is open",
		)
	}

	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

	klines, err := ex.GetKlines(ctx, symbol, opts...)
	if s.circuitBreaker != nil {
		s.circuitBreaker.Record(err == nil)
	}
	return klines, err
}

func (s *Session) GetBalance(ctx context.Context, opts ...exchange.Option) ([]core.Balance, error) {
	ex, err := s.exchange()
	if err != nil {
		return nil, err
	}

	if s.circuitBreaker != nil && !s.circuitBreaker.Allow() {
		return nil, core.NewExchangeError(
			ex.Name(),
			core.ErrorTypeServerError,
			503,
			"circuit breaker is open",
		)
	}

	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

	balances, err := ex.GetBalance(ctx, opts...)
	if s.circuitBreaker != nil {
		s.circuitBreaker.Record(err == nil)
	}
	return balances, err
}

func (s *Session) PlaceOrder(ctx context.Context, req *exchange.OrderRequest, opts ...exchange.Option) (*core.Order, error) {
	ex, err := s.exchange()
	if err != nil {
		return nil, err
	}

	if s.circuitBreaker != nil && !s.circuitBreaker.Allow() {
		return nil, core.NewExchangeError(
			ex.Name(),
			core.ErrorTypeServerError,
			503,
			"circuit breaker is open",
		)
	}

	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

	order, err := ex.PlaceOrder(ctx, req, opts...)
	if s.circuitBreaker != nil {
		s.circuitBreaker.Record(err == nil)
	}
	return order, err
}

func (s *Session) CancelOrder(ctx context.Context, req *exchange.CancelRequest, opts ...exchange.Option) (*core.Order, error) {
	ex, err := s.exchange()
	if err != nil {
		return nil, err
	}

	if s.circuitBreaker != nil && !s.circuitBreaker.Allow() {
		return nil, core.NewExchangeError(
			ex.Name(),
			core.ErrorTypeServerError,
			503,
			"circuit breaker is open",
		)
	}

	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

	order, err := ex.CancelOrder(ctx, req, opts...)
	if s.circuitBreaker != nil {
		s.circuitBreaker.Record(err == nil)
	}
	return order, err
}

func (s *Session) GetOrder(ctx context.Context, req *exchange.OrderQuery, opts ...exchange.Option) (*core.Order, error) {
	ex, err := s.exchange()
	if err != nil {
		return nil, err
	}

	if s.circuitBreaker != nil && !s.circuitBreaker.Allow() {
		return nil, core.NewExchangeError(
			ex.Name(),
			core.ErrorTypeServerError,
			503,
			"circuit breaker is open",
		)
	}

	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

	order, err := ex.GetOrder(ctx, req, opts...)
	if s.circuitBreaker != nil {
		s.circuitBreaker.Record(err == nil)
	}
	return order, err
}

func (s *Session) GetOpenOrders(ctx context.Context, symbol string, opts ...exchange.Option) ([]core.Order, error) {
	ex, err := s.exchange()
	if err != nil {
		return nil, err
	}

	if s.circuitBreaker != nil && !s.circuitBreaker.Allow() {
		return nil, core.NewExchangeError(
			ex.Name(),
			core.ErrorTypeServerError,
			503,
			"circuit breaker is open",
		)
	}

	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

	orders, err := ex.GetOpenOrders(ctx, symbol, opts...)
	if s.circuitBreaker != nil {
		s.circuitBreaker.Record(err == nil)
	}
	return orders, err
}

func (s *Session) SubscribeTicker(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.Ticker, <-chan error) {
	ex, err := s.exchange()
	if err != nil {
		tickerCh := make(chan *core.Ticker)
		errCh := make(chan error, 1)
		errCh <- err
		close(tickerCh)
		close(errCh)
		return tickerCh, errCh
	}
	return ex.SubscribeTicker(ctx, symbol, opts...)
}

func (s *Session) SubscribeTrades(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.Trade, <-chan error) {
	ex, err := s.exchange()
	if err != nil {
		tradeCh := make(chan *core.Trade)
		errCh := make(chan error, 1)
		errCh <- err
		close(tradeCh)
		close(errCh)
		return tradeCh, errCh
	}
	return ex.SubscribeTrades(ctx, symbol, opts...)
}

func (s *Session) SubscribeOrderBook(ctx context.Context, symbol string, opts ...exchange.Option) (<-chan *core.OrderBook, <-chan error) {
	ex, err := s.exchange()
	if err != nil {
		obCh := make(chan *core.OrderBook)
		errCh := make(chan error, 1)
		errCh <- err
		close(obCh)
		close(errCh)
		return obCh, errCh
	}
	return ex.SubscribeOrderBook(ctx, symbol, opts...)
}

func (s *Session) Config() *core.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *Session) ClearCache() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cache != nil {
		s.cache.Clear()
	}
}
