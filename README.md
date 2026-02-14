# Sadewa

A high-performance, type-safe multi-exchange trading library for Go 1.25+. Built with explicit design principles, compile-time safety, and arbitrary-precision decimal arithmetic for financial accuracy.

## Features

### Core Types

| Type        | Description                                                  |
| ----------- | ------------------------------------------------------------ |
| `Ticker`    | Real-time market data with bid, ask, last, high, low, volume |
| `Order`     | Exchange order with full lifecycle tracking                  |
| `Balance`   | Account balance with free and locked amounts                 |
| `Trade`     | Executed trade with fee information                          |
| `Kline`     | Candlestick/OHLCV data                                       |
| `OrderBook` | Order book snapshot with bid/ask levels                      |

### Order Management

- **OrderSide**: Buy, Sell
- **OrderType**: Market, Limit, StopLoss, StopLossLimit, TakeProfit, TakeProfitLimit
- **OrderStatus**: New, PartiallyFilled, Filled, Canceling, Canceled, Rejected, Expired
- **TimeInForce**: GTC (Good Till Cancel), IOC (Immediate Or Cancel), FOK (Fill Or Kill)

### Market Types

- **MarketTypeSpot**: Spot trading
- **MarketTypeFutures**: Perpetual and dated futures
- **MarketTypeOptions**: Options trading

### Infrastructure

- **Rate Limiting**: Token bucket algorithm with global and per-bucket limits
- **Circuit Breaker**: Three-state pattern (Closed, Open, HalfOpen) for fault tolerance
- **Caching**: Configurable TTL caching via `eko/gocache`
- **WebSocket**: Automatic reconnection with exponential backoff
- **API Key Rotation**: KeyRing with multiple rotation strategies

### Go 1.25+ Features

- **Iterator Pattern**: `iter.Seq2` for streaming trades and klines
- **Channel-Based Streaming**: Type-safe channels instead of callbacks
- **Atomic State**: `atomic.Int32` for connection state management
- **slices/maps packages**: Modern collection operations

### Multi-Exchange Aggregation

- Best price discovery across exchanges
- VWAP (Volume-Weighted Average Price) calculation
- Merged order book aggregation
- Cross-exchange arbitrage detection

### Dependency Injection

- **Container**: Type-safe exchange registry
- **DI Pattern**: No global state, explicit dependencies

### Error Handling

- **Error Codes**: Programmatic error classification
- **Typed Errors**: `ErrorCode` constants for reliable handling

---

## Architecture

Sadewa implements a 6-layer architecture with Go 1.25+ features.

```mermaid
graph TB
    subgraph "Layer 6: Order Management"
        OM[Order Manager]
        OB[Order Builder]
    end

    subgraph "Layer 5: Aggregation"
        AG[Aggregator]
        BP[BestPrice]
        VW[VWAP]
        AR[Arbitrage]
    end

    subgraph "Layer 4: Stream"
        TS[TickerStream]
        TR[TradeStream]
        OS[OrderBookStream]
    end

    subgraph "Layer 3: Session"
        S[Session]
        RL[RateLimiter]
        CB[CircuitBreaker]
        CA[Cache]
    end

    subgraph "Layer 2: Exchange Interface"
        E[Exchange Interface]
        B[BinanceExchange]
        C[CoinbaseExchange]
    end

    subgraph "Layer 1: Internal"
        HTTP[HTTP Client]
        WS[WebSocket Client]
        KR[KeyRing]
        N[Normalizer]
    end

    OM --> S
    AG --> S
    S --> E
    E --> B
    E --> C
    B --> HTTP
    B --> WS
    B --> KR
    B --> N
    S --> RL
    S --> CB
    S --> CA
    TS --> WS
    TR --> WS
    OS --> WS
```

### Request Flow

```mermaid
sequenceDiagram
    participant C as Client
    participant S as Session
    participant RL as RateLimiter
    participant CB as CircuitBreaker
    participant E as Exchange
    participant KR as KeyRing
    participant N as Normalizer

    C->>S: GetTicker(ctx, "BTC/USDT")

    alt Cache Hit
        S-->>C: Cached Ticker
    else Cache Miss
        S->>CB: Allow()
        alt Circuit Open
            CB-->>S: Rejected
            S-->>C: Error
        else Circuit Closed
            CB-->>S: Allowed
            S->>RL: Wait(ctx)
            RL-->>S: Token Acquired
            S->>E: GetTicker(ctx, symbol, opts)
            E->>KR: Current()
            KR-->>E: APIKey
            E->>E: BuildRequest()
            E->>E: HTTP Request
            E-->>E: Response
            E->>N: NormalizeTicker(raw)
            N-->>E: *Ticker
            E-->>S: *Ticker
            S->>CB: Record(success)
            S->>S: Cache Result
            S-->>C: *Ticker
        end
    end
```

---

## Supported Exchanges

### Binance (Spot)

| Feature           | REST API | WebSocket |
| ----------------- | :------: | :-------: |
| Get Ticker        |   Yes    |    Yes    |
| Get Order Book    |   Yes    |    Yes    |
| Get Trades        |   Yes    |    Yes    |
| Get Klines        |   Yes    |    Yes    |
| Get Balance       |   Yes    |     -     |
| Place Order       |   Yes    |     -     |
| Cancel Order      |   Yes    |     -     |
| Get Order         |   Yes    |     -     |
| Get Open Orders   |   Yes    |     -     |
| Get Order History |   Yes    |     -     |

### Planned Exchanges

| Exchange | Priority | Status  |
| -------- | :------: | :-----: |
| Coinbase |   High   | Planned |
| Kraken   |   High   | Planned |
| Bybit    |   High   | Planned |
| OKX      |   High   | Planned |
| Huobi    |  Medium  | Planned |
| KuCoin   |  Medium  | Planned |
| Gate.io  |  Medium  | Planned |
| Bitfinex |  Medium  | Planned |

---

## Installation

```bash
go get github.com/pwhholic/sadewa
```

### Dependencies

| Package                                  | Purpose                                |
| ---------------------------------------- | -------------------------------------- |
| `github.com/cockroachdb/apd/v3`          | Arbitrary-precision decimal arithmetic |
| `github.com/go-resty/resty/v3`           | HTTP client with retries               |
| `github.com/bytedance/sonic`             | High-performance JSON serialization    |
| `github.com/rs/zerolog`                  | Zero-allocation structured logging     |
| `github.com/lxzan/gws`                   | High-performance WebSocket             |
| `github.com/eko/gocache/v4`              | Flexible caching library               |
| `github.com/go-playground/validator/v10` | Struct validation                      |
| `golang.org/x/time`                      | Rate limiting                          |
| `golang.org/x/sync/errgroup`             | Concurrent error handling              |

---

## Quick Start

### Basic Ticker Fetch (with DI Container)

```go
package main

import (
    "context"
    "fmt"
    "time"

    "sadewa/pkg/core"
    "sadewa/pkg/exchange"
    "sadewa/pkg/exchange/binance"
    "sadewa/pkg/session"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    cfg := &binance.Config{
        Sandbox: false,
    }

    container := exchange.NewContainer()
    binance.Register(container, cfg)

    sess, err := session.New(container, &session.Config{
        Exchange: "binance",
    })
    if err != nil {
        panic(err)
    }
    defer sess.Close()

    ticker, err := sess.GetTicker(ctx, "BTC/USDT")
    if err != nil {
        panic(err)
    }

    fmt.Printf("BTC/USDT: Bid=%s Ask=%s Last=%s\n",
        ticker.Bid.String(),
        ticker.Ask.String(),
        ticker.Last.String())
}
```

### Place Order

```go
package main

import (
    "context"
    "fmt"

    "sadewa/pkg/core"
    "sadewa/pkg/exchange"
    "sadewa/pkg/exchange/binance"
    "sadewa/pkg/order"
    "sadewa/pkg/session"
)

func main() {
    ctx := context.Background()

    cfg := &binance.Config{
        Credentials: &core.Credentials{
            APIKey:    "your-api-key",
            SecretKey: "your-secret-key",
        },
    }

    container := exchange.NewContainer()
    binance.Register(container, cfg)

    sess, _ := session.New(container, &session.Config{
        Exchange: "binance",
    })
    defer sess.Close()

    req := &order.Request{
        Symbol:        "BTC/USDT",
        Side:          core.SideBuy,
        Type:          core.TypeLimit,
        Price:         mustDecimal("50000"),
        Quantity:      mustDecimal("0.001"),
        TimeInForce:   core.GTC,
        ClientOrderID: "my-order-001",
    }

    ord, err := sess.PlaceOrder(ctx, req)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Order placed: ID=%s\n", ord.ID)
}

func mustDecimal(s string) apd.Decimal {
    var d apd.Decimal
    if _, _, err := d.SetString(s); err != nil {
        panic(err)
    }
    return d
}
```

### Channel-Based WebSocket Streaming

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "sadewa/pkg/core"
    "sadewa/pkg/exchange"
    "sadewa/pkg/exchange/binance"
    "sadewa/pkg/stream"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    cfg := &binance.Config{Sandbox: false}

    container := exchange.NewContainer()
    binance.Register(container, cfg)

    strm := stream.New(container, &stream.Config{Exchange: "binance"})

    tickerCh, errCh := strm.SubscribeTicker(ctx, "BTC/USDT")

    go func() {
        for {
            select {
            case ticker := <-tickerCh:
                fmt.Printf("[%s] Bid: %s | Ask: %s\n",
                    ticker.Timestamp.Format("15:04:05"),
                    ticker.Bid.String(),
                    ticker.Ask.String())
            case err := <-errCh:
                fmt.Printf("Error: %v\n", err)
                return
            case <-ctx.Done():
                return
            }
        }
    }()

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
}
```

### Iterator Pattern (Go 1.25+)

```go
package main

import (
    "context"
    "fmt"

    "sadewa/pkg/exchange"
    "sadewa/pkg/exchange/binance"
    "sadewa/pkg/session"
)

func main() {
    ctx := context.Background()

    container := exchange.NewContainer()
    binance.Register(container, &binance.Config{})

    sess, _ := session.New(container, &session.Config{Exchange: "binance"})

    for trade, err := range sess.Trades(ctx, "BTC/USDT") {
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            break
        }
        fmt.Printf("Trade: %s @ %s\n", trade.Quantity.String(), trade.Price.String())
    }
}
```

### Multi-Exchange Aggregation

```go
package main

import (
    "context"
    "fmt"

    "sadewa/pkg/aggregate"
    "sadewa/pkg/core"
    "sadewa/pkg/exchange"
    "sadewa/pkg/exchange/binance"
    "sadewa/pkg/session"
)

func main() {
    ctx := context.Background()

    container := exchange.NewContainer()
    binance.Register(container, &binance.Config{})

    binanceSess, _ := session.New(container, &session.Config{Exchange: "binance"})

    agg := aggregate.New()
    agg.AddSession("binance", binanceSess)

    bestPrice, err := agg.GetBestPrice(ctx, "BTC/USDT")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Best Bid: %s @ %s\n", bestPrice.Bid.String(), bestPrice.BidExchange)
    fmt.Printf("Best Ask: %s @ %s\n", bestPrice.Ask.String(), bestPrice.AskExchange)

    vwap, err := agg.GetVWAP(ctx, "BTC/USDT", 20)
    if err != nil {
        panic(err)
    }
    fmt.Printf("VWAP: %s (Volume: %s)\n", vwap.VWAP.String(), vwap.Volume.String())
}
```

### API Key Rotation

```go
package main

import (
    "context"

    "sadewa/pkg/core"
    "sadewa/pkg/exchange"
    "sadewa/pkg/exchange/binance"
    "sadewa/pkg/session"
)

func main() {
    ctx := context.Background()

    keyRing := exchange.NewKeyRing([]*exchange.APIKey{
        {ID: "key1", Key: "xxx", Secret: "yyy"},
        {ID: "key2", Key: "aaa", Secret: "bbb"},
        {ID: "key3", Key: "ccc", Secret: "ddd"},
    }, exchange.RotationOnRateLimit)

    cfg := &binance.Config{
        KeyRing: keyRing,
    }

    container := exchange.NewContainer()
    binance.Register(container, cfg)

    sess, _ := session.New(container, &session.Config{Exchange: "binance"})
    defer sess.Close()

    _, _ = sess.GetTicker(ctx, "BTC/USDT")
}
```

### Options Pattern

```go
package main

import (
    "context"
    "fmt"

    "sadewa/pkg/core"
)

func main() {
    ctx := context.Background()

    ticker, err := sess.GetTicker(ctx, "BTC/USDT",
        core.WithLimit(100),
        core.WithMarketType(core.MarketTypeSpot),
    )

    orderBook, err := sess.GetOrderBook(ctx, "BTC/USDT",
        core.WithLimit(50),
        core.WithMarketType(core.MarketTypeFutures),
    )
}
```

---

## Configuration

```go
type Config struct {
    Exchange    string        `json:"exchange"`
    Sandbox     bool          `json:"sandbox"`
    Credentials *Credentials  `json:"credentials"`

    Timeout       time.Duration `json:"timeout"`
    MaxRetries    int           `json:"max_retries"`
    RetryWaitMin  time.Duration `json:"retry_wait_min"`
    RetryWaitMax  time.Duration `json:"retry_wait_max"`

    RateLimitRequests int           `json:"rate_limit_requests"`
    RateLimitPeriod   time.Duration `json:"rate_limit_period"`

    CacheEnabled bool          `json:"cache_enabled"`
    CacheTTL     time.Duration `json:"cache_ttl"`

    CircuitBreakerEnabled          bool          `json:"circuit_breaker_enabled"`
    CircuitBreakerFailThreshold    int           `json:"circuit_breaker_fail_threshold"`
    CircuitBreakerSuccessThreshold int           `json:"circuit_breaker_success_threshold"`
    CircuitBreakerTimeout          time.Duration `json:"circuit_breaker_timeout"`

    LogLevel string `json:"log_level"`
}
```

### Default Values

| Setting                        | Default |
| ------------------------------ | ------- |
| Timeout                        | 10s     |
| MaxRetries                     | 3       |
| RetryWaitMin                   | 100ms   |
| RetryWaitMax                   | 1s      |
| RateLimitRequests              | 1200    |
| RateLimitPeriod                | 1m      |
| CacheEnabled                   | true    |
| CacheTTL                       | 1s      |
| CircuitBreakerFailThreshold    | 5       |
| CircuitBreakerSuccessThreshold | 2       |
| CircuitBreakerTimeout          | 30s     |

### Config Builder

```go
config := core.DefaultConfig("binance").
    WithCredentials(&core.Credentials{APIKey: "...", SecretKey: "..."}).
    WithSandbox(true).
    WithTimeout(30 * time.Second).
    WithRateLimit(600, time.Minute).
    WithCache(true, 2*time.Second)
```

---

## Package Structure

```
sadewa/
├── pkg/                          # Public API
│   ├── core/                     # Types, interfaces, config
│   │   ├── types.go
│   │   ├── config.go
│   │   ├── operation.go
│   │   ├── errors.go
│   │   ├── error_codes.go
│   │   ├── market_type.go
│   │   └── request.go
│   │
│   ├── exchange/                 # Exchange interface & DI
│   │   ├── exchange.go           # Exchange interface
│   │   ├── options.go            # Options pattern
│   │   ├── container.go          # DI container
│   │   └── binance/
│   │       ├── exchange.go       # BinanceExchange
│   │       ├── protocol.go       # Protocol implementation
│   │       ├── normalizer.go     # Data normalization
│   │       ├── websocket.go      # WebSocket streams
│   │       └── doc.go            # Package docs
│   │
│   ├── session/                  # Session management
│   │   └── session.go
│   │
│   ├── stream/                   # Channel-based streaming
│   │   ├── stream.go
│   │   ├── ticker.go
│   │   ├── trade.go
│   │   └── orderbook.go
│   │
│   ├── order/                    # Order lifecycle
│   │   ├── manager.go
│   │   └── builder.go
│   │
│   └── aggregate/                # Multi-exchange aggregation
│       └── aggregator.go
│
└── internal/                     # Private implementation
    ├── http/
    │   └── client.go             # Resty v3 wrapper
    ├── ws/
    │   ├── state.go              # Atomic connection state
    │   └── websocket.go          # WebSocket client
    ├── ratelimit/
    │   └── limiter.go            # Token bucket
    ├── keyring/
    │   └── keyring.go            # API key rotation
    └── circuitbreaker/
        └── breaker.go            # Circuit breaker
```

---

## Exchange Interface

```go
type Exchange interface {
    GetTicker(ctx context.Context, symbol string, opts ...Option) (*Ticker, error)
    GetOrderBook(ctx context.Context, symbol string, opts ...Option) (*OrderBook, error)
    GetTrades(ctx context.Context, symbol string, opts ...Option) iter.Seq2[*Trade, error]
    GetKlines(ctx context.Context, symbol string, opts ...Option) ([]Kline, error)

    GetBalance(ctx context.Context, opts ...Option) ([]Balance, error)

    PlaceOrder(ctx context.Context, req *OrderRequest, opts ...Option) (*Order, error)
    CancelOrder(ctx context.Context, req *CancelRequest, opts ...Option) (*Order, error)
    GetOrder(ctx context.Context, req *OrderQuery, opts ...Option) (*Order, error)

    SubscribeTicker(ctx context.Context, symbol string) (<-chan *Ticker, <-chan error)
    SubscribeTrades(ctx context.Context, symbol string) (<-chan *Trade, <-chan error)
}
```

---

## Error Handling

### Error Codes

```go
type ErrorCode string

const (
    ErrCodeNetwork       ErrorCode = "NETWORK_ERROR"
    ErrCodeRateLimit     ErrorCode = "RATE_LIMIT"
    ErrCodeAuth          ErrorCode = "AUTH_ERROR"
    ErrCodeInvalidSymbol ErrorCode = "INVALID_SYMBOL"
)
```

### Programmatic Error Handling

```go
if exchange.IsErrorCode(err, exchange.ErrCodeRateLimit) {
    keyRing.Rotate()
    retry()
}

if exchange.IsErrorCode(err, exchange.ErrCodeAuth) {
    log.Fatal("invalid credentials")
}
```

### Error Types

| Type                         | Description                 | Retryable |
| ---------------------------- | --------------------------- | :-------: |
| `ErrorTypeNetwork`           | Network connectivity issues |    Yes    |
| `ErrorTypeTimeout`           | Request timeout             |    Yes    |
| `ErrorTypeRateLimit`         | Rate limit exceeded         |    Yes    |
| `ErrorTypeAuthentication`    | Invalid credentials         |    No     |
| `ErrorTypeBadRequest`        | Invalid request parameters  |    No     |
| `ErrorTypeNotFound`          | Resource not found          |    No     |
| `ErrorTypeServerError`       | Exchange server error       |    Yes    |
| `ErrorTypeInsufficientFunds` | Not enough balance          |    No     |
| `ErrorTypeInvalidOrder`      | Order validation failed     |    No     |

### Helper Functions

```go
if core.IsNetworkError(err) {
}

if core.IsRateLimitError(err) {
}

if core.IsTerminalError(err) {
}
```

---

## Performance

### Targets (p95)

| Operation       | Target  |
| --------------- | ------- |
| REST API        | < 100ms |
| WebSocket       | < 10ms  |
| Cache Hit       | < 1ms   |
| Order Placement | < 150ms |

### Throughput

| Metric             | Target     |
| ------------------ | ---------- |
| REST Requests      | 100-1000/s |
| WebSocket Messages | 10,000/s   |
| Concurrent Orders  | 1,000/s    |

### Memory

| Component            | Usage    |
| -------------------- | -------- |
| Base Session         | 10-20 MB |
| Per Order            | ~1 KB    |
| Cache (1000 entries) | 5-10 MB  |
| WebSocket Connection | 2-5 MB   |

---

## Design Principles

### 1. Explicit Over Implicit

All operations and types are explicitly defined. No magic behavior or hidden state.

### 2. Compile-Time Safety

Typed enumerations and static registration catch errors at compile time, not runtime.

### 3. Arbitrary-Precision Decimals

All financial numbers use `apd.Decimal` to avoid floating-point precision issues.

### 4. Zero-Allocation Hot Paths

Critical paths minimize allocations using sync pools and efficient serialization.

### 5. Fail Fast and Loud

Errors are returned immediately with full context. No silent failures or default values.

### 6. No Global State

All dependencies are explicit via DI container. No global registries or singletons.

---

## Roadmap

### Phase 1: Core Infrastructure - Completed

- Core types and interfaces
- Rate limiting
- Circuit breaker
- HTTP transport

### Phase 2: Session Management - Completed

- Session lifecycle
- Cache integration
- Error handling

### Phase 3: Binance Protocol - Completed

- REST API implementation
- WebSocket streams
- Type normalization

### Phase 4: WebSocket Support - Completed

- Generic WebSocket client
- Reconnection logic
- Binance stream subscriptions

### Phase 5: Order Management - Completed

- Order builder pattern
- Lifecycle tracking
- Channel-based updates

### Phase 6: Aggregation - Completed

- Multi-exchange aggregation
- VWAP calculation
- Arbitrage detection

### Future: Phase 7+

| Feature                   | Description                       |
| ------------------------- | --------------------------------- |
| Additional Exchanges      | Coinbase, Kraken, Bybit, OKX      |
| Futures Support           | Perpetual and dated futures       |
| Margin Trading            | Margin order types                |
| Order Book Depth Analysis | Level 2/3 data processing         |
| Historical Data           | Backtesting support               |
| Batch Operations          | Bulk order placement/cancellation |
| Position Tracking         | Portfolio management              |

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Ensure tests pass: `go test ./...`
4. Ensure linting passes: `golangci-lint run`
5. Submit a pull request

### Adding a New Exchange

1. Create `pkg/exchange/<exchange>/`
2. Implement `core.Protocol` interface
3. Implement normalizer methods
4. Add WebSocket support if applicable
5. Register via `Register(container, config)`
6. Write tests with 80%+ coverage

---

## License

MIT License

---

## Acknowledgments

Built with high-quality open source libraries:

- [cockroachdb/apd](https://github.com/cockroachdb/apd) - Arbitrary-precision decimals
- [go-resty/resty](https://github.com/go-resty/resty) - HTTP client
- [bytedance/sonic](https://github.com/bytedance/sonic) - JSON serialization
- [rs/zerolog](https://github.com/rs/zerolog) - Structured logging
- [lxzan/gws](https://github.com/lxzan/gws) - WebSocket client
- [eko/gocache](https://github.com/eko/gocache) - Caching
- [go-playground/validator](https://github.com/go-playground/validator) - Validation
