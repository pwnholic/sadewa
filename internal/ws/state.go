package ws

import "sync/atomic"

// ConnState represents the current connection state of a websocket.
type ConnState int32

// Connection states for websocket lifecycle management.
const (
	// StateDisconnected indicates the websocket is not connected.
	StateDisconnected ConnState = iota
	// StateConnecting indicates the websocket is attempting to establish a connection.
	StateConnecting
	// StateConnected indicates the websocket has an active connection.
	StateConnected
	// StateReconnecting indicates the websocket is attempting to reconnect after a disconnect.
	StateReconnecting
	// StateClosed indicates the websocket has been permanently closed.
	StateClosed
)

// String returns the string representation of the connection state.
func (s ConnState) String() string {
	return [...]string{
		"disconnected",
		"connecting",
		"connected",
		"reconnecting",
		"closed",
	}[s]
}

// State provides thread-safe atomic access to a ConnState value.
type State struct {
	state atomic.Int32
}

// Load returns the current connection state.
func (s *State) Load() ConnState {
	return ConnState(s.state.Load())
}

// Store sets the connection state to the given value.
func (s *State) Store(state ConnState) {
	s.state.Store(int32(state))
}

// CompareAndSwap atomically compares the current state with old and swaps to new if equal.
// It returns true if the swap was performed.
func (s *State) CompareAndSwap(old, new ConnState) bool {
	return s.state.CompareAndSwap(int32(old), int32(new))
}
