# Basic Examples

This example demonstrates basic market data operations:

- Get Ticker (current price, bid/ask, volume)
- Get Order Book (bids and asks)
- Get Recent Trades
- Get Klines (candlestick/OHLCV data)

## Usage

```bash
go run main.go
```

## Expected Output

```
=== Get Ticker ===
Symbol:    BTC/USDT
Bid:       43250.00
Ask:       43251.00
Last:      43250.50
High 24h:  44100.00
Low 24h:   42800.00
Volume:    12500.5
Timestamp: 2024-01-15T10:30:00Z

=== Get Order Book ===
Symbol: BTC/USDT

Bids (Top 5):
  1. Price: 43250.00  Qty: 1.5
  2. Price: 43249.00  Qty: 2.3
  ...

=== Get Recent Trades ===
  1. [BUY ] 0.005 @ 43250.00 (Fee: 0.00001 BTC)
  2. [SELL] 0.010 @ 43249.50 (Fee: 0.00002 BTC)
  ...

=== Get Klines ===
  1. O:43100.00 H:43250.00 L:43050.00 C:43200.00 V:125.5
  ...
```
