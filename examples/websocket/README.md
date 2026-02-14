# WebSocket Examples

This example demonstrates real-time market data streaming via WebSocket:

- Ticker Stream (24-hour ticker updates)
- Mini Ticker Stream (simplified ticker)
- Order Book Depth Stream (real-time order book)
- Trade Stream (individual trades)
- Aggregated Trade Stream (combined trades)
- Kline Stream (candlestick updates)

## Usage

```bash
go run main.go
```

## Expected Output

```
=== Binance WebSocket Streams ===
Connecting to Binance WebSocket...
Connected! Subscribing to streams...

Subscribed to streams. Press Ctrl+C to exit.

[TICKER] BTC/USDT | Bid: 43250.00 | Ask: 43251.00 | Last: 43250.50 | Vol: 12500.5
[MINI] ETH/USDT | Last: 2280.50 | Vol: 8500.2
[ORDERBOOK] BTC/USDT | Best Bid: 43250.00 x 1.5 | Best Ask: 43251.00 x 2.3
[TRADE] BUY  0.005 @ 43250.00 | 10:30:15.123
[AGGTRADE] SELL 0.010 @ 2280.50 | 10:30:15.456
[KLINE] BTC/USDT | O:43200.00 H:43250.00 L:43180.00 C:43240.00 V:125.5
...

^C
Shutting down...
```

## Available Streams

| Stream           | Method                                                       | Description                   |
| ---------------- | ------------------------------------------------------------ | ----------------------------- |
| Ticker           | `SubscribeTicker(symbol, callback)`                          | 24-hour ticker with all stats |
| Mini Ticker      | `SubscribeMiniTicker(symbol, callback)`                      | Simplified ticker             |
| Order Book       | `SubscribeOrderBook(symbol, callback)`                       | Full order book updates       |
| Order Book Depth | `SubscribeOrderBookDepth(symbol, levels, speedMs, callback)` | Custom depth                  |
| Trades           | `SubscribeTrades(symbol, callback)`                          | Individual trade events       |
| AggTrades        | `SubscribeAggTrades(symbol, callback)`                       | Aggregated trades             |
| Klines           | `SubscribeKlines(symbol, interval, callback)`                | Candlestick updates           |
| All Mini Tickers | `SubscribeAllMarketMiniTicker(callback)`                     | All symbols                   |

## Kline Intervals

- Minutes: `1m`, `3m`, `5m`, `15m`, `30m`
- Hours: `1h`, `2h`, `4h`, `6h`, `8h`, `12h`
- Days: `1d`, `3d`
- Weeks: `1w`
- Months: `1M`
