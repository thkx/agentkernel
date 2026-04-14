package retry

import (
	"math"
	"time"
)

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

func NewRetryPolicy(maxAttempts int, baseDelay time.Duration) *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts: maxAttempts,
		BaseDelay:   baseDelay,
		MaxDelay:    time.Minute,
	}
}

func (r *RetryPolicy) Next(attempt int) time.Duration {
	if attempt >= r.MaxAttempts {
		return 0
	}
	delay := time.Duration(float64(r.BaseDelay) * math.Pow(2, float64(attempt-1)))
	if delay > r.MaxDelay {
		delay = r.MaxDelay
	}
	return delay
}

func (r *RetryPolicy) ShouldRetry(attempt int) bool {
	return attempt < r.MaxAttempts
}
