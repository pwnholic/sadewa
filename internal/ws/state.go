package ws

import "sync/atomic"

type ConnState int32

const (
	StateDisconnected ConnState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateClosed
)

func (s ConnState) String() string {
	return [...]string{
		"disconnected",
		"connecting",
		"connected",
		"reconnecting",
		"closed",
	}[s]
}

type State struct {
	state atomic.Int32
}

func (s *State) Load() ConnState {
	return ConnState(s.state.Load())
}

func (s *State) Store(state ConnState) {
	s.state.Store(int32(state))
}

func (s *State) CompareAndSwap(old, new ConnState) bool {
	return s.state.CompareAndSwap(int32(old), int32(new))
}
