# AGENTS.md - Project Development Guidelines

## Project Info

**Project:** Multi-Exchange Trading Library  
**Language:** Go 1.25+  
**Philosophy:** Explicit, Type-Safe, High-Performance, Zero-`any`

---

## CRITICAL: Use context7 MCP

Always query context7 MCP before implementing ANY feature:

- For dependencies: "gws websocket", "sonic json", "apd decimal", "resty v3"
- For patterns: "golang concurrency patterns", "golang error handling"
- Never assume API signatures - always verify

---

## Core Principles

### 1. Explicit Over Implicit

```go
// GOOD
var price apd.Decimal
price.SetString("100.50")

// BAD
price := someFunction() // What type?
```

### 2. Compile-Time Safety

```go
// GOOD
type OrderStatus int
const (StatusNew OrderStatus = iota; StatusFilled)

// BAD
const StatusNew = "new"
```

### 3. Fail Fast

```go
// GOOD
func PlaceOrder(order *Order) error {
    if order.Symbol == "" { return ErrInvalidSymbol }
    // ...
}
```

---

## Type System

| Rule              | GOOD                           | BAD                 |
| ----------------- | ------------------------------ | ------------------- |
| Financial numbers | `apd.Decimal`                  | `float64`, `string` |
| Enumerations      | `type Status int` + iota       | string constants    |
| JSON              | Always add `json:"field"` tags | missing tags        |
| Interface         | `any` (not `interface{}`)      | `interface{}`       |
| Returns           | Concrete types                 | `any`               |

---

## Error Handling

```go
// GOOD: Typed + wrapped
type Error struct {
    Code      ErrorCode
    Message   string
    Exchange  string
}

func Do() error {
    if err := validate(); err != nil {
        return fmt.Errorf("validation: %w", err)
    }
    return nil
}

// BAD
return errors.New("failed")
```

---

## Naming Conventions

| Element   | Convention          | Example                   |
| --------- | ------------------- | ------------------------- |
| Package   | lowercase, singular | `binance`, `session`      |
| Interface | noun/verb+er        | `Protocol`, `RateLimiter` |
| Function  | Verb+Noun           | `GetTicker`, `PlaceOrder` |
| Constants | PascalCase          | `DefaultTimeout`          |
| Files     | snake_case          | `circuit_breaker.go`      |

---

## Dependencies

```go
// Required
"github.com/cockroachdb/apd/v3"           // Decimals
"github.com/go-resty/resty/v3"            // HTTP (v3!)
"github.com/bytedance/sonic"               // JSON
"github.com/rs/zerolog"                    // Logging
"github.com/lxzan/gws"                     // WebSocket
"github.com/eko/gocache/v4"                // Caching
"github.com/go-playground/validator/v10"   // Validation
"golang.org/x/sync/errgroup"               // Concurrency
"golang.org/x/time/rate"                   // Rate limiting
```

### Forbidden Patterns

```go
// NEVER
var price float64                    // Use apd.Decimal
import "encoding/json"               // Use sonic
fmt.Println("log")                   // Use zerolog
func Get() any                       // Return concrete type
var registry = make(map[string]Exchange)  // Use DI
func Subscribe(callback func())      // Use channels
```

---

## Go 1.25+ Features

### Iterator Pattern

```go
func (e *Exchange) Trades(ctx context.Context, symbol string) iter.Seq2[*Trade, error] {
    return func(yield func(*Trade, error) bool) {
        for _, t := range trades {
            if !yield(t, nil) { return }
        }
    }
}

// Usage
for trade, err := range exchange.Trades(ctx, "BTC/USDT") { ... }
```

### slices/maps packages

```go
slices.SortFunc(bids, cmp.Compare)
slices.Contains(symbols, "BTC/USDT")
maps.Copy(dst, src)
```

### Atomic State

```go
type ConnState int32
const (StateDisconnected ConnState = iota; StateConnecting; StateConnected)

type Conn struct { state atomic.Int32 }

func (c *Conn) State() ConnState { return ConnState(c.state.Load()) }
```

---

## Exchange Interface

```go
type Exchange interface {
    // Market Data - all typed, NO any
    GetTicker(ctx context.Context, symbol string, opts ...Option) (*Ticker, error)
    GetOrderBook(ctx context.Context, symbol string, opts ...Option) (*OrderBook, error)
    GetTrades(ctx context.Context, symbol string, opts ...Option) iter.Seq2[*Trade, error]
    GetKlines(ctx context.Context, symbol string, opts ...Option) ([]Kline, error)

    // Account
    GetBalance(ctx context.Context, opts ...Option) ([]Balance, error)

    // Trading
    PlaceOrder(ctx context.Context, req *OrderRequest, opts ...Option) (*Order, error)
    CancelOrder(ctx context.Context, req *CancelRequest, opts ...Option) (*Order, error)
    GetOrder(ctx context.Context, req *OrderQuery, opts ...Option) (*Order, error)

    // Streaming - channels, NOT callbacks
    SubscribeTicker(ctx context.Context, symbol string) (<-chan *Ticker, <-chan error)
    SubscribeTrades(ctx context.Context, symbol string) (<-chan *Trade, <-chan error)
}
```

### Options Pattern

```go
type Option func(*Options)

type Options struct {
    Limit      int
    Interval   string
    MarketType MarketType  // Spot/Futures
}

func WithLimit(limit int) Option { return func(o *Options) { o.Limit = limit } }
func WithMarketType(mt MarketType) Option { return func(o *Options) { o.MarketType = mt } }

// Usage
ticker, _ := exchange.GetTicker(ctx, "BTC/USDT", WithLimit(100), WithMarketType(MarketTypeSpot))
```

---

## Channel-Based Streaming

```go
// GOOD: Channels
tickerCh, errCh := exchange.SubscribeTicker(ctx, "BTC/USDT")
for {
    select {
    case t := <-tickerCh: process(t)
    case err := <-errCh: log.Error(err); return
    case <-ctx.Done(): return
    }
}

// BAD: Callbacks
Subscribe(func(t *Ticker) { ... })
```

---

## Dependency Injection

```go
// GOOD: DI Container
type Container struct {
    mu    sync.RWMutex
    exs   map[string]Exchange
}

func (c *Container) Register(name string, ex Exchange)
func (c *Container) Get(name string) (Exchange, error)

// Usage
container := exchange.NewContainer()
container.Register("binance", binance.New(cfg))
session := session.New(container)

// BAD: Global
var registry = make(map[string]Exchange)
```

---

## API Key Rotation

```go
type KeyRing struct {
    keys     []*APIKey
    current  int
    strategy RotationStrategy  // RoundRobin, OnError, OnRateLimit
}

func (k *KeyRing) Current() *APIKey
func (k *KeyRing) Rotate()
func (k *KeyRing) OnError(err error)

// Usage
keyRing := exchange.NewKeyRing([]*exchange.APIKey{
    {ID: "key1", Key: "xxx", Secret: "yyy"},
    {ID: "key2", Key: "aaa", Secret: "bbb"},
}, exchange.RotationOnRateLimit)
```

---

## Error Codes

```go
type ErrorCode string

const (
    ErrCodeNetwork       ErrorCode = "NETWORK_ERROR"
    ErrCodeRateLimit     ErrorCode = "RATE_LIMIT"
    ErrCodeAuth          ErrorCode = "AUTH_ERROR"
    ErrCodeInvalidSymbol ErrorCode = "INVALID_SYMBOL"
)

func IsErrorCode(err error, code ErrorCode) bool

// Usage
if IsErrorCode(err, ErrCodeRateLimit) { retry() }
```

---

## Resty v3

```go
import "github.com/go-resty/resty/v3"

client := resty.New()
defer client.Close()  // v3 requires Close()

client.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error { ... })
client.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error { ... })

resp, _ := client.R().SetContext(ctx).SetQueryParams(params).Get("/api/v3/ticker/price")
```

---

## Testing

```go
// Naming: Test + StructName + MethodName
func TestSession_PlaceOrder(t *testing.T) {}

// Table-driven
tests := []struct{ name, input, want string }{ ... }
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        assert.Equal(t, tt.want, got)
    })
}

// Coverage: 80% min, 100% for critical paths
go test -cover ./...
```

---

## Checklist

Before commit:

- [ ] `go build ./...` passes
- [ ] `go test ./...` passes
- [ ] No `any` returns in Exchange interface
- [ ] `apd.Decimal` for all money fields
- [ ] Context propagated correctly
- [ ] Errors wrapped with context
- [ ] No credentials in logs
