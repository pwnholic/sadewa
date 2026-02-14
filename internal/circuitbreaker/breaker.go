package circuitbreaker

import (
	"sync"
	"sync/atomic"
	"time"
)

// State represents the operational state of a circuit breaker.
type State int32

// Circuit breaker states.
const (
	// StateClosed indicates the circuit breaker is allowing all requests through.
	StateClosed State = iota
	// StateOpen indicates the circuit breaker is blocking all requests.
	StateOpen
	// StateHalfOpen indicates the circuit breaker is testing if the service has recovered.
	StateHalfOpen
)

// String returns the string representation of the circuit breaker state.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// Config holds configuration options for a circuit breaker.
type Config struct {
	// FailThreshold is the number of consecutive failures required to open the circuit.
	FailThreshold int `json:"fail_threshold"`
	// SuccessThreshold is the number of consecutive successes required to close the circuit from half-open.
	SuccessThreshold int `json:"success_threshold"`
	// Timeout is the duration to wait in open state before transitioning to half-open.
	Timeout time.Duration `json:"timeout"`
}

// Breaker implements the circuit breaker pattern for fault tolerance.
type Breaker struct {
	state            atomic.Int32
	failures         atomic.Int32
	successes        atomic.Int32
	failThreshold    int
	successThreshold int
	timeout          time.Duration
	lastFailTime     atomic.Int64
	mu               sync.Mutex
	metrics          *Metrics
}

// Metrics tracks statistics about circuit breaker operations.
type Metrics struct {
	totalRequests   atomic.Int64
	successRequests atomic.Int64
	failedRequests  atomic.Int64
	stateChanges    atomic.Int32
}

// New creates a new circuit breaker with the given configuration.
func New(config Config) *Breaker {
	b := &Breaker{
		failThreshold:    config.FailThreshold,
		successThreshold: config.SuccessThreshold,
		timeout:          config.Timeout,
		metrics:          &Metrics{},
	}
	b.state.Store(int32(StateClosed))
	return b
}

// Allow returns true if a request should be permitted based on the current circuit state.
// In open state, it returns false until the timeout has elapsed.
func (b *Breaker) Allow() bool {
	b.metrics.totalRequests.Add(1)
	state := State(b.state.Load())

	switch state {
	case StateClosed:
		return true
	case StateOpen:
		lastFail := time.Unix(0, b.lastFailTime.Load())
		if time.Since(lastFail) >= b.timeout {
			b.transitionTo(StateHalfOpen)
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

// Record reports the outcome of a request to the circuit breaker.
// Positive outcomes may close the circuit, while negative outcomes may open it.
func (b *Breaker) Record(success bool) {
	state := State(b.state.Load())

	switch state {
	case StateClosed:
		if success {
			b.failures.Store(0)
			b.metrics.successRequests.Add(1)
		} else {
			b.failures.Add(1)
			b.metrics.failedRequests.Add(1)
			if int(b.failures.Load()) >= b.failThreshold {
				b.lastFailTime.Store(time.Now().UnixNano())
				b.transitionTo(StateOpen)
			}
		}
	case StateOpen:
		b.mu.Lock()
		lastFail := time.Unix(0, b.lastFailTime.Load())
		if time.Since(lastFail) >= b.timeout {
			b.transitionTo(StateHalfOpen)
			b.failures.Store(0)
			b.successes.Store(0)
			if success {
				b.successes.Add(1)
				b.metrics.successRequests.Add(1)
				if int(b.successes.Load()) >= b.successThreshold {
					b.transitionTo(StateClosed)
				}
			} else {
				b.lastFailTime.Store(time.Now().UnixNano())
				b.transitionTo(StateOpen)
				b.metrics.failedRequests.Add(1)
			}
		}
		b.mu.Unlock()
	case StateHalfOpen:
		if success {
			b.successes.Add(1)
			b.metrics.successRequests.Add(1)
			if int(b.successes.Load()) >= b.successThreshold {
				b.transitionTo(StateClosed)
				b.failures.Store(0)
				b.successes.Store(0)
			}
		} else {
			b.lastFailTime.Store(time.Now().UnixNano())
			b.transitionTo(StateOpen)
			b.successes.Store(0)
			b.metrics.failedRequests.Add(1)
		}
	}
}

func (b *Breaker) transitionTo(newState State) {
	b.state.Store(int32(newState))
	b.metrics.stateChanges.Add(1)
}

// State returns the current operational state of the circuit breaker.
func (b *Breaker) State() State {
	return State(b.state.Load())
}

// Reset immediately transitions the circuit breaker to closed state and clears all counters.
func (b *Breaker) Reset() {
	b.state.Store(int32(StateClosed))
	b.failures.Store(0)
	b.successes.Store(0)
}

// Failures returns the current count of consecutive failures.
func (b *Breaker) Failures() int {
	return int(b.failures.Load())
}

// Successes returns the current count of consecutive successes in half-open state.
func (b *Breaker) Successes() int {
	return int(b.successes.Load())
}

// Metrics returns a snapshot of the current circuit breaker statistics.
func (b *Breaker) Metrics() MetricsSnapshot {
	return MetricsSnapshot{
		TotalRequests:   b.metrics.totalRequests.Load(),
		SuccessRequests: b.metrics.successRequests.Load(),
		FailedRequests:  b.metrics.failedRequests.Load(),
		StateChanges:    b.metrics.stateChanges.Load(),
		CurrentState:    b.State().String(),
	}
}

// MetricsSnapshot is a point-in-time capture of circuit breaker statistics.
type MetricsSnapshot struct {
	// TotalRequests is the total number of Allow calls made.
	TotalRequests int64
	// SuccessRequests is the number of requests that reported success.
	SuccessRequests int64
	// FailedRequests is the number of requests that reported failure.
	FailedRequests int64
	// StateChanges is the number of times the circuit breaker has changed state.
	StateChanges int32
	// CurrentState is the string representation of the current state.
	CurrentState string
}
