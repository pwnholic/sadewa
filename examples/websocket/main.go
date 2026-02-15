package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"sadewa/pkg/core"
	"sadewa/pkg/exchange/binance"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	fmt.Println("=== Binance WebSocket Streams ===")
	fmt.Println("Connecting to Binance WebSocket...")

	wsClient := binance.NewBinanceWSClient(false)
	wsClient.SetLogger(logger)

	if err := wsClient.Connect(ctx); err != nil {
		logger.Error().Err(err).Msg("Failed to connect")
		os.Exit(1)
	}
	defer wsClient.Close()

	fmt.Println("Connected! Subscribing to streams...")
	fmt.Println()

	wsClient.SubscribeTicker("BTC/USDT", func(ticker *core.Ticker) {
		fmt.Printf("[TICKER] %s | Bid: %s | Ask: %s | Last: %s | Vol: %s\n",
			ticker.Symbol,
			ticker.Bid.String(),
			ticker.Ask.String(),
			ticker.Last.String(),
			ticker.Volume.String())
	})

	wsClient.SubscribeTrades("BTC/USDT", func(trade *core.Trade) {
		side := "BUY "
		if trade.Side == core.SideSell {
			side = "SELL"
		}
		fmt.Printf("[TRADE] %s | %s %s @ %s\n",
			side,
			trade.Quantity.String(),
			trade.Price.String(),
			trade.Timestamp.Format("15:04:05.000"))
	})

	fmt.Println()
	fmt.Println("Subscribed to streams. Press Ctrl+C to exit.")
	fmt.Println()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
}
