package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"sadewa/pkg/core"
	"sadewa/pkg/exchange/binance"
	"sadewa/pkg/session"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config := core.DefaultConfig("binance")

	sess, err := session.New(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create session: %v\n", err)
		os.Exit(1)
	}
	defer sess.Close()

	sess.SetProtocol(binance.New())

	fmt.Println("=== Get Ticker ===")
	ticker, err := sess.GetTicker(ctx, "BTC/USDT")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	} else {
		printTicker(ticker)
	}

	fmt.Println("\n=== Get Order Book ===")
	orderBook, err := sess.GetOrderBook(ctx, "BTC/USDT", 5)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	} else {
		printOrderBook(orderBook)
	}

	fmt.Println("\n=== Get Recent Trades ===")
	trades, err := sess.GetTrades(ctx, "BTC/USDT", 5)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	} else {
		printTrades(trades)
	}

	fmt.Println("\n=== Get Klines ===")
	klines, err := sess.GetKlines(ctx, "BTC/USDT", "1h", 5)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	} else {
		printKlines(klines)
	}
}

func printTicker(t *core.Ticker) {
	fmt.Printf("Symbol:    %s\n", t.Symbol)
	fmt.Printf("Bid:       %s\n", t.Bid.String())
	fmt.Printf("Ask:       %s\n", t.Ask.String())
	fmt.Printf("Last:      %s\n", t.Last.String())
	fmt.Printf("High 24h:  %s\n", t.High.String())
	fmt.Printf("Low 24h:   %s\n", t.Low.String())
	fmt.Printf("Volume:    %s\n", t.Volume.String())
	fmt.Printf("Timestamp: %s\n", t.Timestamp.Format(time.RFC3339))
}

func printOrderBook(ob *core.OrderBook) {
	fmt.Printf("Symbol: %s\n", ob.Symbol)
	fmt.Printf("\nBids (Top 5):")
	for i, bid := range ob.Bids {
		if i >= 5 {
			break
		}
		fmt.Printf("  %d. Price: %s  Qty: %s\n", i+1, bid.Price.String(), bid.Quantity.String())
	}
	fmt.Printf("\nAsks (Top 5):")
	for i, ask := range ob.Asks {
		if i >= 5 {
			break
		}
		fmt.Printf("  %d. Price: %s  Qty: %s\n", i+1, ask.Price.String(), ask.Quantity.String())
	}
}

func printTrades(trades []core.Trade) {
	for i, t := range trades {
		side := "BUY "
		if t.Side == core.SideSell {
			side = "SELL"
		}
		fmt.Printf("  %d. [%s] %s @ %s (Fee: %s %s)\n",
			i+1, side, t.Quantity.String(), t.Price.String(),
			t.Fee.String(), t.FeeAsset)
	}
}

func printKlines(klines []core.Kline) {
	for i, k := range klines {
		fmt.Printf("  %d. O:%s H:%s L:%s C:%s V:%s\n",
			i+1, k.Open.String(), k.High.String(),
			k.Low.String(), k.Close.String(), k.Volume.String())
	}
}
