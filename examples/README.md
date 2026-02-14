# Sadewa Examples

This directory contains runnable examples demonstrating the Sadewa multi-exchange trading library.

## Directory Structure

```
examples/
├── basic/          # Basic market data operations
├── order/          # Order management examples
├── websocket/      # Real-time WebSocket streams
└── aggregation/    # Multi-exchange aggregation
```

## Running Examples

### Basic Example

Market data operations without authentication:

```bash
cd examples/basic
go run main.go
```

Features demonstrated:

- Get Ticker (price, bid/ask, volume)
- Get Order Book
- Get Recent Trades
- Get Klines (candlesticks)

### Order Example

Order management with authentication:

```bash
export BINANCE_API_KEY="your-api-key"
export BINANCE_SECRET_KEY="your-secret-key"

cd examples/order
go run main.go
```

Features demonstrated:

- Get Account Balance
- Place Limit Order
- Query Order Status
- Cancel Order
- Get Open Orders

### WebSocket Example

Real-time market data streaming:

```bash
cd examples/websocket
go run main.go
```

Features demonstrated:

- Ticker Stream
- Mini Ticker Stream
- Order Book Depth Stream
- Trade Stream
- Aggregated Trade Stream
- Kline Stream

### Aggregation Example

Multi-exchange data aggregation:

```bash
cd examples/aggregation
go run main.go
```

Features demonstrated:

- Concurrent ticker fetching
- Best price discovery
- VWAP calculation
- Merged order book
- Price comparison
- Arbitrage detection

## Environment Variables

| Variable             | Description        | Required For   |
| -------------------- | ------------------ | -------------- |
| `BINANCE_API_KEY`    | Binance API key    | Order examples |
| `BINANCE_SECRET_KEY` | Binance secret key | Order examples |

## Notes

- Order examples use sandbox/testnet mode by default
- WebSocket examples run until interrupted (Ctrl+C)
- Aggregation benefits are more visible with multiple exchanges configured
