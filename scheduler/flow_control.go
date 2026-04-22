package scheduler

import (
	"sync"
	"time"

	"github.com/thkx/agentkernel/types"
)

// RateLimitConfig configures rate limiting per capability
type RateLimitConfig struct {
	Rate  int
	Burst int
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	FailureThreshold int
	ResetTimeout     time.Duration
}

// circuitBreakerState represents the state of a circuit breaker
type circuitBreakerState int

const (
	cbClosed circuitBreakerState = iota
	cbOpen
	cbHalfOpen
)

// circuitBreaker implements a simple circuit breaker pattern
type circuitBreaker struct {
	mu           sync.Mutex
	state        circuitBreakerState
	failureCount int
	lastFailure  time.Time
	config       CircuitBreakerConfig
}

// newCircuitBreaker creates a new circuit breaker
func newCircuitBreaker(config CircuitBreakerConfig) *circuitBreaker {
	return &circuitBreaker{
		state:  cbClosed,
		config: config,
	}
}

// Allow checks if the circuit breaker allows a request
func (cb *circuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case cbOpen:
		if time.Since(cb.lastFailure) >= cb.config.ResetTimeout {
			cb.state = cbHalfOpen
			return true
		}
		return false
	case cbHalfOpen, cbClosed:
		return true
	default:
		return true
	}
}

// Record records success/failure to update circuit breaker state
func (cb *circuitBreaker) Record(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if success {
		cb.failureCount = 0
		cb.state = cbClosed
		return
	}

	cb.failureCount++
	cb.lastFailure = time.Now()
	if cb.failureCount >= cb.config.FailureThreshold {
		cb.state = cbOpen
	}
}

// rateLimiter implements token bucket-based rate limiting
type rateLimiter struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

// newRateLimiter creates a new rate limiter
func newRateLimiter(rate int, burst int) *rateLimiter {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = rate
	}
	return &rateLimiter{
		rate:   float64(rate),
		burst:  float64(burst),
		tokens: float64(burst),
		last:   time.Now(),
	}
}

// Allow checks if a request is allowed under rate limit
func (r *rateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	delta := now.Sub(r.last).Seconds()
	r.last = now
	r.tokens += delta * r.rate
	if r.tokens > r.burst {
		r.tokens = r.burst
	}
	if r.tokens < 1 {
		return false
	}
	r.tokens--
	return true
}

// TenantQuotaConfig configures per-tenant rate limiting
type TenantQuotaConfig struct {
	Rate  int // requests per second
	Burst int // maximum burst
}

// flowController manages rate limiting and circuit breaking for nodes
type flowController struct {
	mu               sync.Mutex
	rateLimiters     map[types.CapabilityName]*rateLimiter
	cbConfigs        map[string]CircuitBreakerConfig
	circuitBreakers  map[string]*circuitBreaker
	tenantQuotas     map[string]*rateLimiter      // tenant -> rate limiter
	tenantQuotaConfs map[string]TenantQuotaConfig // tenant -> config
}

// newFlowController creates a new flow controller
func newFlowController() *flowController {
	return &flowController{
		rateLimiters:     make(map[types.CapabilityName]*rateLimiter),
		cbConfigs:        make(map[string]CircuitBreakerConfig),
		circuitBreakers:  make(map[string]*circuitBreaker),
		tenantQuotas:     make(map[string]*rateLimiter),
		tenantQuotaConfs: make(map[string]TenantQuotaConfig),
	}
}

// enableRateLimit configures rate limiting for a capability
func (fc *flowController) enableRateLimit(name types.CapabilityName, rate, burst int) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if rate > 0 {
		fc.rateLimiters[name] = newRateLimiter(rate, burst)
	}
}

// enableCircuitBreaker configures circuit breaker for a key
func (fc *flowController) enableCircuitBreaker(key string, config CircuitBreakerConfig) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if key != "" && config.FailureThreshold > 0 && config.ResetTimeout > 0 {
		fc.cbConfigs[key] = config
	}
}

// enableTenantQuota configures rate limiting for a tenant
func (fc *flowController) enableTenantQuota(tenant string, rate, burst int) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if tenant != "" && rate > 0 {
		fc.tenantQuotaConfs[tenant] = TenantQuotaConfig{Rate: rate, Burst: burst}
		fc.tenantQuotas[tenant] = newRateLimiter(rate, burst)
	}
}

// getTenantQuotaLimiter lazily initializes and returns a tenant's rate limiter
func (fc *flowController) getTenantQuotaLimiter(tenant string) *rateLimiter {
	if tenant == "" {
		return nil
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	limiter, exists := fc.tenantQuotas[tenant]
	if !exists {
		// Check if tenant has a configured quota
		conf, ok := fc.tenantQuotaConfs[tenant]
		if !ok {
			return nil // No tenant quota configured
		}
		limiter = newRateLimiter(conf.Rate, conf.Burst)
		fc.tenantQuotas[tenant] = limiter
	}
	return limiter
}

// canProceedWithTenant checks both node and tenant rate limiting
func (fc *flowController) canProceedWithTenant(node *types.Node[any]) bool {
	// Check node-level rate limit
	if !fc.canProceed(node) {
		return false
	}

	// Check tenant-level quota if tenant is set
	if node.Tenant != "" {
		limiter := fc.getTenantQuotaLimiter(node.Tenant)
		if limiter != nil && !limiter.Allow() {
			return false
		}
	}

	return true
}

// canProceed checks rate limiting for a node
func (fc *flowController) canProceed(node *types.Node[any]) bool {
	fc.mu.Lock()
	limiter := fc.rateLimiters[node.Capability]
	if limiter == nil && node.RateLimit > 0 {
		limiter = newRateLimiter(node.RateLimit, node.RateLimit)
		fc.rateLimiters[node.Capability] = limiter
	}
	fc.mu.Unlock()

	if limiter == nil {
		return true
	}
	return limiter.Allow()
}

// isOpen checks if circuit breaker is open
func (fc *flowController) isOpen(node *types.Node[any]) bool {
	cb := fc.getBreaker(fc.breakerKey(node))
	if cb == nil {
		return false
	}
	return !cb.Allow()
}

// recordResult records success/failure for circuit breaker
func (fc *flowController) recordResult(node *types.Node[any], success bool) {
	cb := fc.getBreaker(fc.breakerKey(node))
	if cb != nil {
		cb.Record(success)
	}
}

func (fc *flowController) breakerKey(node *types.Node[any]) string {
	if node == nil {
		return ""
	}
	if node.CircuitBreakerKey != "" {
		return node.CircuitBreakerKey
	}
	return string(node.Capability)
}

// getBreaker lazily initializes and returns a circuit breaker
func (fc *flowController) getBreaker(key string) *circuitBreaker {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	cb, exists := fc.circuitBreakers[key]
	if !exists {
		config, ok := fc.cbConfigs[key]
		if !ok {
			return nil
		}
		cb = newCircuitBreaker(config)
		fc.circuitBreakers[key] = cb
	}
	return cb
}
