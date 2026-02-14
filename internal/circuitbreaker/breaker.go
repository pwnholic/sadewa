package circuitbreaker

import (
	"sync"
	"sync/atomic"
	"time"
)

type State int32

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

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

type Config struct {
	FailThreshold    int           `json:"fail_threshold"`
	SuccessThreshold int           `json:"success_threshold"`
	Timeout          time.Duration `json:"timeout"`
}

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

type Metrics struct {
	totalRequests   atomic.Int64
	successRequests atomic.Int64
	failedRequests  atomic.Int64
	stateChanges    atomic.Int32
}

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

func (b *Breaker) State() State {
	return State(b.state.Load())
}

func (b *Breaker) Reset() {
	b.state.Store(int32(StateClosed))
	b.failures.Store(0)
	b.successes.Store(0)
}

func (b *Breaker) Failures() int {
	return int(b.failures.Load())
}

func (b *Breaker) Successes() int {
	return int(b.successes.Load())
}

func (b *Breaker) Metrics() MetricsSnapshot {
	return MetricsSnapshot{
		TotalRequests:   b.metrics.totalRequests.Load(),
		SuccessRequests: b.metrics.successRequests.Load(),
		FailedRequests:  b.metrics.failedRequests.Load(),
		StateChanges:    b.metrics.stateChanges.Load(),
		CurrentState:    b.State().String(),
	}
}

type MetricsSnapshot struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	StateChanges    int32
	CurrentState    string
}
