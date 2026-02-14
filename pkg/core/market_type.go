package core

// MarketType represents the type of trading market on an exchange.
type MarketType int

// Market type constants define the available trading market categories.
const (
	// MarketTypeSpot indicates spot trading where assets are exchanged immediately.
	MarketTypeSpot MarketType = iota
	// MarketTypeFutures indicates futures trading with contracts for future delivery.
	MarketTypeFutures
	// MarketTypeOptions indicates options trading with contracts for optional future trades.
	MarketTypeOptions
)

// String returns the string representation of the market type ("spot", "futures", or "options").
func (m MarketType) String() string {
	return [...]string{
		"spot",
		"futures",
		"options",
	}[m]
}
