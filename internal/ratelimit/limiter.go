package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	global   *rate.Limiter
	buckets  sync.Map
	requests int
	period   time.Duration
	metrics  *Metrics
}

type Metrics struct {
	totalRequests   atomic.Int64
	allowedRequests atomic.Int64
	deniedRequests  atomic.Int64
	bucketCount     atomic.Int32
}

func New(requests int, period time.Duration) *RateLimiter {
	rps := float64(requests) / period.Seconds()
	return &RateLimiter{
		global:   rate.NewLimiter(rate.Limit(rps), requests),
		requests: requests,
		period:   period,
		metrics:  &Metrics{},
	}
}

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

func (r *RateLimiter) Reserve() *rate.Reservation {
	return r.global.Reserve()
}

func (r *RateLimiter) ReserveBucket(bucket string) *rate.Reservation {
	limiter := r.getBucket(bucket)
	return limiter.Reserve()
}

func (r *RateLimiter) SetLimit(requests int, period time.Duration) {
	r.requests = requests
	r.period = period
	rps := float64(requests) / period.Seconds()
	r.global.SetLimit(rate.Limit(rps))
}

func (r *RateLimiter) SetBucketLimit(bucket string, requests int, period time.Duration) {
	rps := float64(requests) / period.Seconds()
	limiter := r.getBucket(bucket)
	limiter.SetLimit(rate.Limit(rps))
}

func (r *RateLimiter) Metrics() MetricsSnapshot {
	return MetricsSnapshot{
		TotalRequests:   r.metrics.totalRequests.Load(),
		AllowedRequests: r.metrics.allowedRequests.Load(),
		DeniedRequests:  r.metrics.deniedRequests.Load(),
		BucketCount:     r.metrics.bucketCount.Load(),
	}
}

type MetricsSnapshot struct {
	TotalRequests   int64
	AllowedRequests int64
	DeniedRequests  int64
	BucketCount     int32
}
