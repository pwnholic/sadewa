# Aggregation Examples

This example demonstrates multi-exchange data aggregation:

- Get tickers from multiple exchanges concurrently
- Find best price across exchanges
- Calculate VWAP (Volume-Weighted Average Price)
- Merge order books from multiple exchanges
- Compare prices across exchanges
- Find arbitrage opportunities

## Usage

```bash
go run main.go
```

## Expected Output

```
=== Multi-Exchange Aggregation ===
Creating sessions...
Sessions created: binance

=== Get Tickers from All Exchanges ===
[binance] Last: 43250.50 | Bid: 43250.00 | Ask: 43251.00

=== Get Best Price Across Exchanges ===
Best Bid: 43250.00 @ binance
Best Ask: 43251.00 @ binance
Spread: 1.00 (0.0023%)
Timestamp: 2024-01-15T10:30:00Z

=== Calculate VWAP ===
VWAP: 43245.75
Total Volume: 1250.50
Number of Levels: 20
Exchanges: [binance]

=== Get Merged Order Book ===
Symbol: BTC/USDT
Exchanges: [binance]

Merged Bids (Top 5):
  1. Price: 43250.00 | Qty: 1.5
  2. Price: 43249.00 | Qty: 2.3
  3. Price: 43248.00 | Qty: 0.8
  4. Price: 43247.50 | Qty: 1.2
  5. Price: 43247.00 | Qty: 3.5

Merged Asks (Top 5):
  1. Price: 43251.00 | Qty: 2.1
  2. Price: 43252.00 | Qty: 1.8
  3. Price: 43253.00 | Qty: 0.5
  4. Price: 43254.00 | Qty: 2.0
  5. Price: 43255.00 | Qty: 1.3

=== Compare Prices Across Exchanges ===
Symbol: BTC/USDT
  [binance] Bid: 43250.00 | Ask: 43251.00
Max Spread: 1.00

=== Find Arbitrage Opportunities ===
No arbitrage opportunities found (need multiple exchanges)

=== Aggregator Stats ===
Total Exchanges: 1
Active Exchanges: 1
Last Update: 2024-01-15T10:30:00Z
```

## Adding More Exchanges

To see full aggregation benefits, add multiple exchanges:

```go
// Add Binance
binanceSession, _ := session.New(core.DefaultConfig("binance"))
binanceSession.SetProtocol(binance.New())
agg.AddSession("binance", binanceSession)

// Add Coinbase (when implemented)
coinbaseSession, _ := session.New(core.DefaultConfig("coinbase"))
coinbaseSession.SetProtocol(coinbase.New())
agg.AddSession("coinbase", coinbaseSession)

// Add Kraken (when implemented)
krakenSession, _ := session.New(core.DefaultConfig("kraken"))
krakenSession.SetProtocol(kraken.New())
agg.AddSession("kraken", krakenSession)
```

## VWAP Formula

```
VWAP = Sum(Price x Volume) / Sum(Volume)
```

## Arbitrage Detection

The aggregator finds arbitrage opportunities where:

- Buy price on exchange A < Sell price on exchange B
- Spread percentage exceeds minimum threshold
