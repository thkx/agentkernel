package retry

import "time"

type RetryPolicy struct{}

func (r *RetryPolicy) Next(attempt int) time.Duration {
	return time.Duration(attempt) * 100 * time.Millisecond
}
