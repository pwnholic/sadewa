package exchange

import (
	"time"

	"sadewa/pkg/core"
)

// Option is a functional option for configuring exchange operations.
type Option func(*Options)

// Options holds configurable parameters for exchange API requests.
type Options struct {
	Limit      int
	Interval   string
	StartTime  time.Time
	EndTime    time.Time
	MarketType core.MarketType
}

// WithLimit returns an option that sets the maximum number of results to return.
func WithLimit(limit int) Option {
	return func(o *Options) {
		o.Limit = limit
	}
}

// WithInterval returns an option that sets the time interval for kline/candlestick data.
func WithInterval(interval string) Option {
	return func(o *Options) {
		o.Interval = interval
	}
}

// WithTimeRange returns an option that sets the start and end time for historical data queries.
func WithTimeRange(start, end time.Time) Option {
	return func(o *Options) {
		o.StartTime = start
		o.EndTime = end
	}
}

// WithMarketType returns an option that specifies whether to use spot or futures market.
func WithMarketType(mt core.MarketType) Option {
	return func(o *Options) {
		o.MarketType = mt
	}
}

// ApplyOptions applies a list of options to a new Options instance and returns the result.
func ApplyOptions(opts ...Option) *Options {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
