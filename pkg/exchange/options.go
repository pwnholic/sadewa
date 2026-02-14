package exchange

import (
	"time"

	"sadewa/pkg/core"
)

type Option func(*Options)

type Options struct {
	Limit      int
	Interval   string
	StartTime  time.Time
	EndTime    time.Time
	MarketType core.MarketType
}

func WithLimit(limit int) Option {
	return func(o *Options) {
		o.Limit = limit
	}
}

func WithInterval(interval string) Option {
	return func(o *Options) {
		o.Interval = interval
	}
}

func WithTimeRange(start, end time.Time) Option {
	return func(o *Options) {
		o.StartTime = start
		o.EndTime = end
	}
}

func WithMarketType(mt core.MarketType) Option {
	return func(o *Options) {
		o.MarketType = mt
	}
}

func ApplyOptions(opts ...Option) *Options {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
