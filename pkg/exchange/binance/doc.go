// Package binance implements the Binance exchange.
// It provides REST API and WebSocket support for spot and futures trading on Binance.
//
// The package includes:
//   - BinanceExchange: Implements exchange.Exchange interface for trading operations
//   - Protocol: REST API request building, response parsing, and authentication
//   - Normalizer: Conversion between Binance-specific and canonical types
//   - BinanceWSClient: WebSocket streaming for real-time market data
//
// Example usage:
//
//	config := core.DefaultConfig("binance")
//	ex, err := binance.New(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer ex.Close()
//
//	ticker, err := ex.GetTicker(ctx, "BTC/USDT")
//
// For registration with a container:
//
//	container := exchange.NewContainer()
//	err := binance.Register(container, config)
//	ex, _ := container.Get("binance")
package binance
