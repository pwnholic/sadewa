package stream

import (
	"context"
	"time"

	"sadewa/internal/ws"
)

type ConnState = ws.ConnState

const (
	StateDisconnected = ws.StateDisconnected
	StateConnecting   = ws.StateConnecting
	StateConnected    = ws.StateConnected
	StateReconnecting = ws.StateReconnecting
	StateClosed       = ws.StateClosed
)

type Stream interface {
	Connect(ctx context.Context) error
	Close() error
	State() ConnState
}

type ReconnectConfig struct {
	Enabled     bool
	MaxAttempts int
	BaseWait    time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

func DefaultReconnectConfig() ReconnectConfig {
	return ReconnectConfig{
		Enabled:     true,
		MaxAttempts: 10,
		BaseWait:    1 * time.Second,
		MaxWait:     30 * time.Second,
		Multiplier:  2.0,
	}
}

type BaseConfig struct {
	URL          string
	Reconnect    ReconnectConfig
	PingInterval time.Duration
	PongWait     time.Duration
	BufferSize   int
}

func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		Reconnect:    DefaultReconnectConfig(),
		PingInterval: 10 * time.Second,
		PongWait:     20 * time.Second,
		BufferSize:   100,
	}
}
