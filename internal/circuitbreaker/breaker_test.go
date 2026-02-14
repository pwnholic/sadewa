package circuitbreaker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestState_String(t *testing.T) {
	tests := []struct {
		name  string
		state State
		want  string
	}{
		{"closed", StateClosed, "CLOSED"},
		{"open", StateOpen, "OPEN"},
		{"half_open", StateHalfOpen, "HALF_OPEN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.state.String())
		})
	}
}

func TestBreaker_New(t *testing.T) {
	config := Config{
		FailThreshold:    5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
	}

	breaker := New(config)

	assert.NotNil(t, breaker)
	assert.Equal(t, StateClosed, breaker.State())
}

func TestBreaker_Allow_Closed(t *testing.T) {
	breaker := New(Config{
		FailThreshold:    3,
		SuccessThreshold: 2,
		Timeout:          time.Second,
	})

	assert.True(t, breaker.Allow())
}

func TestBreaker_TransitionToOpen(t *testing.T) {
	breaker := New(Config{
		FailThreshold:    3,
		SuccessThreshold: 2,
		Timeout:          time.Second,
	})

	breaker.Record(false)
	assert.Equal(t, StateClosed, breaker.State())

	breaker.Record(false)
	assert.Equal(t, StateClosed, breaker.State())

	breaker.Record(false)
	assert.Equal(t, StateOpen, breaker.State())

	assert.False(t, breaker.Allow())
}

func TestBreaker_TransitionToHalfOpen(t *testing.T) {
	breaker := New(Config{
		FailThreshold:    2,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	})

	breaker.Record(false)
	breaker.Record(false)
	assert.Equal(t, StateOpen, breaker.State())

	time.Sleep(150 * time.Millisecond)

	assert.True(t, breaker.Allow())
	breaker.Record(true)
	assert.Equal(t, StateHalfOpen, breaker.State())
}

func TestBreaker_TransitionToClosed(t *testing.T) {
	breaker := New(Config{
		FailThreshold:    2,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	})

	breaker.Record(false)
	breaker.Record(false)
	assert.Equal(t, StateOpen, breaker.State())

	time.Sleep(150 * time.Millisecond)

	breaker.Record(true)
	assert.Equal(t, StateHalfOpen, breaker.State())

	breaker.Record(true)
	assert.Equal(t, StateClosed, breaker.State())
}

func TestBreaker_HalfOpenToFails(t *testing.T) {
	breaker := New(Config{
		FailThreshold:    2,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	})

	breaker.Record(false)
	breaker.Record(false)
	assert.Equal(t, StateOpen, breaker.State())

	time.Sleep(150 * time.Millisecond)

	breaker.Record(true)
	assert.Equal(t, StateHalfOpen, breaker.State())

	breaker.Record(false)
	assert.Equal(t, StateOpen, breaker.State())
}

func TestBreaker_Reset(t *testing.T) {
	breaker := New(Config{
		FailThreshold:    2,
		SuccessThreshold: 2,
		Timeout:          time.Second,
	})

	breaker.Record(false)
	breaker.Record(false)
	assert.Equal(t, StateOpen, breaker.State())

	breaker.Reset()

	assert.Equal(t, StateClosed, breaker.State())
	assert.Equal(t, 0, breaker.Failures())
	assert.Equal(t, 0, breaker.Successes())
}

func TestBreaker_FailuresAndSuccesses(t *testing.T) {
	breaker := New(Config{
		FailThreshold:    10,
		SuccessThreshold: 5,
		Timeout:          time.Second,
	})

	breaker.Record(false)
	breaker.Record(false)
	breaker.Record(true)

	assert.Equal(t, 0, breaker.Failures())
	assert.Equal(t, 0, breaker.Successes())
}

func TestBreaker_SuccessResetsFailures(t *testing.T) {
	breaker := New(Config{
		FailThreshold:    5,
		SuccessThreshold: 2,
		Timeout:          time.Second,
	})

	breaker.Record(false)
	breaker.Record(false)
	breaker.Record(false)
	assert.Equal(t, 3, breaker.Failures())

	breaker.Record(true)
	assert.Equal(t, 0, breaker.Failures())
}
