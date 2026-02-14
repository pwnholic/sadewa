package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_New(t *testing.T) {
	limiter := New(10, time.Second)

	assert.NotNil(t, limiter)
}

func TestRateLimiter_Allow(t *testing.T) {
	limiter := New(5, time.Second)

	for i := 0; i < 5; i++ {
		assert.True(t, limiter.Allow(), "request %d should be allowed", i+1)
	}

	assert.False(t, limiter.Allow(), "request 6 should be blocked")
}

func TestRateLimiter_Wait(t *testing.T) {
	limiter := New(5, 100*time.Millisecond)

	for i := 0; i < 5; i++ {
		err := limiter.Wait(context.Background())
		assert.NoError(t, err)
	}
}

func TestRateLimiter_Wait_ContextCancellation(t *testing.T) {
	limiter := New(1, time.Second)

	err := limiter.Wait(context.Background())
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = limiter.Wait(ctx)
	assert.Error(t, err)
}

func TestRateLimiter_Bucket(t *testing.T) {
	limiter := New(5, time.Second)

	for i := 0; i < 5; i++ {
		assert.True(t, limiter.AllowBucket("bucket1"), "bucket1 request %d should be allowed", i+1)
	}
	assert.False(t, limiter.AllowBucket("bucket1"), "bucket1 request 6 should be blocked")

	assert.True(t, limiter.AllowBucket("bucket2"), "bucket2 request 1 should be allowed")
}

func TestRateLimiter_WaitBucket(t *testing.T) {
	limiter := New(5, 100*time.Millisecond)

	for i := 0; i < 5; i++ {
		err := limiter.WaitBucket(context.Background(), "bucket1")
		assert.NoError(t, err)
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	limiter := New(100, time.Second)

	var wg sync.WaitGroup
	successCount := make(chan bool, 200)

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			successCount <- limiter.Allow()
		}()
	}

	wg.Wait()
	close(successCount)

	allowed := 0
	for success := range successCount {
		if success {
			allowed++
		}
	}

	assert.LessOrEqual(t, allowed, 100, "should not allow more than 100 requests")
}

func TestRateLimiter_Reserve(t *testing.T) {
	limiter := New(10, time.Second)

	reservation := limiter.Reserve()
	assert.NotNil(t, reservation)
}

func TestRateLimiter_SetLimit(t *testing.T) {
	limiter := New(1, time.Minute)

	assert.True(t, limiter.Allow())
	assert.False(t, limiter.Allow())

	limiter.SetLimit(1000, time.Second)

	time.Sleep(100 * time.Millisecond)
	assert.True(t, limiter.Allow(), "should allow after limit increase and time passage")
}
