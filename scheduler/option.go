package scheduler

import (
	"time"

	"github.com/thkx/agentkernel/types"
)

type SchedulerOption func(*Scheduler)

func WithWorkerCount(count int) SchedulerOption {
	return func(s *Scheduler) {
		if count > 0 {
			s.workerCount = count
		}
	}
}

func WithMaxQueueSize(size int) SchedulerOption {
	return func(s *Scheduler) {
		if size > 0 {
			s.queue.maxQueueSize = size
		}
	}
}

func WithRejectionPolicy(policy RejectionPolicy) SchedulerOption {
	return func(s *Scheduler) {
		s.queue.rejectionPol = policy
	}
}

func WithDefaultTimeout(timeout time.Duration) SchedulerOption {
	return func(s *Scheduler) {
		if timeout > 0 {
			s.defaultTimeout = timeout
		}
	}
}

func WithCapabilityRateLimit(name types.CapabilityName, rate, burst int) SchedulerOption {
	return func(s *Scheduler) {
		s.flowCtl.enableRateLimit(name, rate, burst)
	}
}

func WithTenantQuota(tenant string, rate, burst int) SchedulerOption {
	return func(s *Scheduler) {
		s.flowCtl.enableTenantQuota(tenant, rate, burst)
	}
}

func WithCircuitBreaker(key string, config CircuitBreakerConfig) SchedulerOption {
	return func(s *Scheduler) {
		s.flowCtl.enableCircuitBreaker(key, config)
	}
}

func WithTenantScheduling(enabled bool) SchedulerOption {
	return func(s *Scheduler) {
		s.tenantAware = enabled
	}
}
