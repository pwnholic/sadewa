package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides rate limiting with support for global and per-bucket limits.
type RateLimiter struct {
	global   *rate.Limiter
	buckets  sync.Map
	requests int
	period   time.Duration
	metrics  *Metrics
}

// Metrics tracks statistics about rate limiter usage.
type Metrics struct {
	totalRequests   atomic.Int64
	allowedRequests atomic.Int64
	deniedRequests  atomic.Int64
	bucketCount     atomic.Int32
}

// New creates a new RateLimiter with the specified number of requests allowed per period.
func New(requests int, period time.Duration) *RateLimiter {
	rps := float64(requests) / period.Seconds()
	return &RateLimiter{
		global:   rate.NewLimiter(rate.Limit(rps), requests),
		requests: requests,
		period:   period,
		metrics:  &Metrics{},
	}
}

// Wait blocks until the global rate limiter allows a request or the context is cancelled.
func (r *RateLimiter) Wait(ctx context.Context) error {
	r.metrics.totalRequests.Add(1)
	err := r.global.Wait(ctx)
	if err != nil {
		r.metrics.deniedRequests.Add(1)
		return err
	}
	r.metrics.allowedRequests.Add(1)
	return nil
}

// WaitBucket blocks until the named bucket's rate limiter allows a request or the context is cancelled.
// Buckets are created on-demand with the default rate limit.
func (r *RateLimiter) WaitBucket(ctx context.Context, bucket string) error {
	r.metrics.totalRequests.Add(1)
	limiter := r.getBucket(bucket)
	err := limiter.Wait(ctx)
	if err != nil {
		r.metrics.deniedRequests.Add(1)
		return err
	}
	r.metrics.allowedRequests.Add(1)
	return nil
}

// Allow returns true if the global rate limiter permits a request immediately.
func (r *RateLimiter) Allow() bool {
	r.metrics.totalRequests.Add(1)
	allowed := r.global.Allow()
	if allowed {
		r.metrics.allowedRequests.Add(1)
	} else {
		r.metrics.deniedRequests.Add(1)
	}
	return allowed
}

// AllowBucket returns true if the named bucket's rate limiter permits a request immediately.
// Buckets are created on-demand with the default rate limit.
func (r *RateLimiter) AllowBucket(bucket string) bool {
	r.metrics.totalRequests.Add(1)
	limiter := r.getBucket(bucket)
	allowed := limiter.Allow()
	if allowed {
		r.metrics.allowedRequests.Add(1)
	} else {
		r.metrics.deniedRequests.Add(1)
	}
	return allowed
}

func (r *RateLimiter) getBucket(bucket string) *rate.Limiter {
	if v, ok := r.buckets.Load(bucket); ok {
		return v.(*rate.Limiter)
	}

	rps := float64(r.requests) / r.period.Seconds()
	limiter := rate.NewLimiter(rate.Limit(rps), r.requests)
	actual, loaded := r.buckets.LoadOrStore(bucket, limiter)
	if !loaded {
		r.metrics.bucketCount.Add(1)
	}
	return actual.(*rate.Limiter)
}

// Reserve returns a reservation for the global rate limiter allowing future planning of request timing.
func (r *RateLimiter) Reserve() *rate.Reservation {
	return r.global.Reserve()
}

// ReserveBucket returns a reservation for the named bucket's rate limiter.
// Buckets are created on-demand with the default rate limit.
func (r *RateLimiter) ReserveBucket(bucket string) *rate.Reservation {
	limiter := r.getBucket(bucket)
	return limiter.Reserve()
}

// SetLimit updates the global rate limit to the specified requests per period.
func (r *RateLimiter) SetLimit(requests int, period time.Duration) {
	r.requests = requests
	r.period = period
	rps := float64(requests) / period.Seconds()
	r.global.SetLimit(rate.Limit(rps))
}

// SetBucketLimit updates the rate limit for a specific bucket.
// The bucket is created if it does not exist.
func (r *RateLimiter) SetBucketLimit(bucket string, requests int, period time.Duration) {
	rps := float64(requests) / period.Seconds()
	limiter := r.getBucket(bucket)
	limiter.SetLimit(rate.Limit(rps))
}

// Metrics returns a snapshot of the current rate limiter statistics.
func (r *RateLimiter) Metrics() MetricsSnapshot {
	return MetricsSnapshot{
		TotalRequests:   r.metrics.totalRequests.Load(),
		AllowedRequests: r.metrics.allowedRequests.Load(),
		DeniedRequests:  r.metrics.deniedRequests.Load(),
		BucketCount:     r.metrics.bucketCount.Load(),
	}
}

// MetricsSnapshot is a point-in-time capture of rate limiter statistics.
type MetricsSnapshot struct {
	// TotalRequests is the total number of rate limit checks performed.
	TotalRequests int64
	// AllowedRequests is the number of requests that were allowed.
	AllowedRequests int64
	// DeniedRequests is the number of requests that were denied.
	DeniedRequests int64
	// BucketCount is the number of rate limit buckets in use.
	BucketCount int32
}
