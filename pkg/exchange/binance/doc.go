// Package binance implements the Binance exchange protocol.
// It provides REST API and WebSocket support for spot trading on Binance.
//
// The package includes:
//   - Protocol: REST API request building, response parsing, and authentication
//   - Normalizer: Conversion between Binance-specific and canonical types
//   - BinanceWSClient: WebSocket streaming for real-time market data
//
// Example usage:
//
//	protocol := binance.New()
//	req, err := protocol.BuildRequest(ctx, core.OpGetTicker, core.Params{"symbol": "BTC/USDT"})
package binance
