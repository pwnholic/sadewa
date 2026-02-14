package core

type MarketType int

const (
	MarketTypeSpot MarketType = iota
	MarketTypeFutures
	MarketTypeOptions
)

func (m MarketType) String() string {
	return [...]string{
		"spot",
		"futures",
		"options",
	}[m]
}
