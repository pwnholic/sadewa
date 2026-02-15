package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"sadewa/pkg/core"
	"sadewa/pkg/exchange"
	"sadewa/pkg/exchange/bybit"
	"sadewa/pkg/session"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a default config for Bybit
	config := core.DefaultConfig("bybit")

	// Create container and register Bybit exchange
	container := exchange.NewContainer()
	if err := bybit.Register(container, config); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register bybit: %v\n", err)
		os.Exit(1)
	}

	// Create session
	sess, err := session.NewSession(container, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create session: %v\n", err)
		os.Exit(1)
	}
	defer sess.Close()

	// Set the exchange for this session
	if err := sess.SetExchange("bybit"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set exchange: %v\n", err)
		os.Exit(1)
	}

	// Get ticker for BTC/USDT
	fmt.Println("=== Bybit BTC/USDT Ticker ===")
	ticker, err := sess.GetTicker(ctx, "BTC/USDT")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting ticker: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Symbol:    %s\n", ticker.Symbol)
	fmt.Printf("Bid:       %s\n", ticker.Bid.String())
	fmt.Printf("Ask:       %s\n", ticker.Ask.String())
	fmt.Printf("Last:      %s\n", ticker.Last.String())
	fmt.Printf("High 24h:  %s\n", ticker.High.String())
	fmt.Printf("Low 24h:   %s\n", ticker.Low.String())
	fmt.Printf("Volume:    %s\n", ticker.Volume.String())
	fmt.Printf("Timestamp: %s\n", ticker.Timestamp.Format(time.RFC3339))
}
