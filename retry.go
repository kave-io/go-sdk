package kave

import (
	"context"
	"math/rand/v2"
	"time"
)

// RetryPolicy controls retry behavior for idempotent reads.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts (1 = no retry).
	MaxAttempts int
	// Base is the initial backoff duration.
	Base time.Duration
	// Cap is the maximum backoff duration.
	Cap time.Duration
	// JitterFraction is applied symmetrically (0.2 = ±20%).
	JitterFraction float64
}

// DefaultRetryPolicy retries idempotent reads up to 3 times with exponential
// backoff: 200 ms × 2^n, ±20% jitter, capped at 5 s.
var DefaultRetryPolicy = RetryPolicy{
	MaxAttempts:    3,
	Base:           200 * time.Millisecond,
	Cap:            5 * time.Second,
	JitterFraction: 0.2,
}

// NoRetry disables all automatic retries.
var NoRetry = RetryPolicy{MaxAttempts: 1}

// WithRetry returns an Option that overrides the client retry policy.
func WithRetry(p RetryPolicy) Option {
	return func(o *options) { o.retryPolicy = p }
}

// doWithRetry runs fn up to p.MaxAttempts times, retrying on retriable errors.
// Only use for idempotent operations.
func doWithRetry[T any](ctx context.Context, p RetryPolicy, fn func() (T, error)) (T, error) {
	p = normalizeRetryPolicy(p)
	var (
		zero T
		err  error
		val  T
	)
	for attempt := 0; attempt < p.MaxAttempts; attempt++ {
		val, err = fn()
		if err == nil {
			return val, nil
		}
		if !isRetriable(err) || attempt == p.MaxAttempts-1 {
			return zero, err
		}
		delay := backoff(p, attempt)
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}
	return zero, err
}

func isRetriable(err error) bool {
	return IsUnavailable(err) || IsDeadlineExceeded(err)
}

func normalizeRetryPolicy(p RetryPolicy) RetryPolicy {
	if p.MaxAttempts <= 0 {
		return NoRetry
	}
	if p.Base <= 0 {
		p.Base = DefaultRetryPolicy.Base
	}
	if p.Cap <= 0 {
		p.Cap = DefaultRetryPolicy.Cap
	}
	if p.JitterFraction < 0 {
		p.JitterFraction = 0
	}
	return p
}

func backoff(p RetryPolicy, attempt int) time.Duration {
	p = normalizeRetryPolicy(p)
	d := p.Base * (1 << attempt)
	if d > p.Cap {
		d = p.Cap
	}
	// ±jitter
	jitter := float64(d) * p.JitterFraction * (rand.Float64()*2 - 1)
	return d + time.Duration(jitter)
}
