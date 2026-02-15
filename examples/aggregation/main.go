package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/rs/zerolog"

	aggregator "sadewa/pkg/aggregate"
	"sadewa/pkg/core"
	"sadewa/pkg/exchange"
	"sadewa/pkg/exchange/binance"
	"sadewa/pkg/session"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("=== Multi-Exchange Aggregation ===")
	fmt.Println("Creating sessions...")

	agg := aggregator.NewAggregator()

	binanceConfig := core.DefaultConfig("binance")

	container := exchange.NewContainer()
	if err := binance.Register(container, binanceConfig); err != nil {
		log.Error().Err(err).Msg("Failed to register binance")
		os.Exit(1)
	}

	binanceSession, err := session.NewSession(container, binanceConfig)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Binance session")
		os.Exit(1)
	}
	defer binanceSession.Close()

	if err := binanceSession.SetExchange("binance"); err != nil {
		log.Error().Err(err).Msg("Failed to set exchange")
		os.Exit(1)
	}

	agg.AddSession("binance", binanceSession)

	fmt.Println("Sessions created: binance")
	fmt.Println()

	fmt.Println("=== Get Tickers from All Exchanges ===")
	tickers := agg.GetTickers(ctx, "BTC/USDT")
	for _, t := range tickers {
		if t.Error != nil {
			fmt.Printf("[%s] Error: %v\n", t.Exchange, t.Error)
		} else {
			fmt.Printf("[%s] Last: %s | Bid: %s | Ask: %s\n",
				t.Exchange,
				t.Ticker.Last.String(),
				t.Ticker.Bid.String(),
				t.Ticker.Ask.String())
		}
	}

	fmt.Println()
	fmt.Println("=== Get Best Price Across Exchanges ===")
	bestPrice, err := agg.GetBestPrice(ctx, "BTC/USDT")
	if err != nil {
		log.Error().Err(err).Msg("Error getting best price")
	} else {
		fmt.Printf("Best Bid: %s @ %s\n", bestPrice.Bid.String(), bestPrice.BidExchange)
		fmt.Printf("Best Ask: %s @ %s\n", bestPrice.Ask.String(), bestPrice.AskExchange)
		fmt.Printf("Spread: %s (%s%%)\n", bestPrice.Spread.String(), bestPrice.SpreadPercent.String())
		fmt.Printf("Timestamp: %s\n", bestPrice.Timestamp.Format(time.RFC3339))
	}

	fmt.Println()
	fmt.Println("=== Calculate VWAP ===")
	vwap, err := agg.GetVWAP(ctx, "BTC/USDT", 20)
	if err != nil {
		log.Error().Err(err).Msg("Error calculating VWAP")
	} else {
		fmt.Printf("VWAP: %s\n", vwap.VWAP.String())
		fmt.Printf("Total Volume: %s\n", vwap.Volume.String())
		fmt.Printf("Number of Levels: %d\n", vwap.NumTrades)
		fmt.Printf("Exchanges: %v\n", vwap.Exchanges)
	}

	fmt.Println()
	fmt.Println("=== Get Merged Order Book ===")
	mergedBook, err := agg.GetMergedOrderBook(ctx, "BTC/USDT", 5)
	if err != nil {
		log.Error().Err(err).Msg("Error getting merged order book")
	} else {
		fmt.Printf("Symbol: %s\n", mergedBook.Symbol)
		fmt.Printf("Exchanges: %v\n", mergedBook.Exchanges)

		fmt.Println("\nMerged Bids (Top 5):")
		for i, bid := range mergedBook.Bids {
			if i >= 5 {
				break
			}
			fmt.Printf("  %d. Price: %s | Qty: %s\n", i+1, bid.Price.String(), bid.Quantity.String())
		}

		fmt.Println("\nMerged Asks (Top 5):")
		for i, ask := range mergedBook.Asks {
			if i >= 5 {
				break
			}
			fmt.Printf("  %d. Price: %s | Qty: %s\n", i+1, ask.Price.String(), ask.Quantity.String())
		}
	}

	fmt.Println()
	fmt.Println("=== Compare Prices Across Exchanges ===")
	comparison, err := agg.ComparePrices(ctx, "BTC/USDT")
	if err != nil {
		log.Error().Err(err).Msg("Error comparing prices")
	} else {
		fmt.Printf("Symbol: %s\n", comparison.Symbol)
		for _, ep := range comparison.Exchanges {
			fmt.Printf("  [%s] Bid: %s | Ask: %s\n", ep.Exchange, ep.Bid.String(), ep.Ask.String())
		}
		fmt.Printf("Max Spread: %s\n", comparison.MaxSpread.String())
	}

	fmt.Println()
	fmt.Println("=== Find Arbitrage Opportunities ===")
	var minSpread apd.Decimal
	minSpread.SetInt64(0)

	opportunities, err := agg.FindArbitrage(ctx, "BTC/USDT", minSpread)
	if err != nil {
		log.Error().Err(err).Msg("Error finding arbitrage")
	} else if len(opportunities) == 0 {
		fmt.Println("No arbitrage opportunities found (need multiple exchanges)")
	} else {
		for _, opp := range opportunities {
			fmt.Printf("Buy on %s @ %s\n", opp.BuyExchange, opp.BuyPrice.String())
			fmt.Printf("Sell on %s @ %s\n", opp.SellExchange, opp.SellPrice.String())
			fmt.Printf("Spread: %s (%s%%)\n", opp.Spread.String(), opp.SpreadPercent.String())
			fmt.Println()
		}
	}

	stats := agg.GetStats()
	fmt.Println()
	fmt.Println("=== Aggregator Stats ===")
	fmt.Printf("Total Exchanges: %d\n", stats.TotalExchanges)
	fmt.Printf("Active Exchanges: %d\n", stats.ActiveExchanges)
	fmt.Printf("Last Update: %s\n", stats.LastUpdate.Format(time.RFC3339))
}
