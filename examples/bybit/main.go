package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"

	"sadewa/pkg/core"
	"sadewa/pkg/exchange"
	"sadewa/pkg/exchange/bybit"
	"sadewa/pkg/session"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a default config for Bybit
	config := core.DefaultConfig("bybit")

	// Create container and register Bybit exchange
	container := exchange.NewContainer()
	if err := bybit.Register(container, config); err != nil {
		log.Error().Err(err).Msg("Failed to register bybit")
		os.Exit(1)
	}

	// Create session
	sess, err := session.NewSession(container, config)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create session")
		os.Exit(1)
	}
	defer sess.Close()

	// Set the exchange for this session
	if err := sess.SetExchange("bybit"); err != nil {
		log.Error().Err(err).Msg("Failed to set exchange")
		os.Exit(1)
	}

	// Get ticker for BTC/USDT
	fmt.Println("=== Bybit BTC/USDT Ticker ===")
	ticker, err := sess.GetTicker(ctx, "BTC/USDT")
	if err != nil {
		log.Error().Err(err).Msg("Error getting ticker")
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
