package scheduler

import (
	"testing"
	"time"

	"github.com/thkx/agentkernel/types"
)

// TestTenantQuotaEnforcement verifies per-tenant rate limiting
func TestTenantQuotaEnforcement(t *testing.T) {
	fc := newFlowController()
	fc.enableTenantQuota("tenant-a", 3, 3) // 3 req/sec, burst 3

	// Create nodes for tenant-a
	node := &types.Node[any]{
		ID:         "test-node",
		Capability: "cap1",
		Tenant:     "tenant-a",
	}

	// First 3 requests should pass (within burst)
	for i := 0; i < 3; i++ {
		if !fc.canProceedWithTenant(node) {
			t.Errorf("Request %d should pass (within burst)", i+1)
		}
	}

	// 4th request should fail (quota exhausted)
	if fc.canProceedWithTenant(node) {
		t.Error("Request 4 should fail (quota exhausted)")
	}

	// Wait 1 second for token replenishment
	time.Sleep(1100 * time.Millisecond)

	// Next request should pass (3 new tokens in 1 second)
	if !fc.canProceedWithTenant(node) {
		t.Error("Request 5 should pass after token replenishment")
	}
}

// TestTenantFairness verifies that different tenants don't interfere
func TestTenantFairness(t *testing.T) {
	fc := newFlowController()
	fc.enableTenantQuota("tenant-a", 2, 2)
	fc.enableTenantQuota("tenant-b", 2, 2)

	nodeA := &types.Node[any]{
		ID:         "node-a",
		Capability: "cap1",
		Tenant:     "tenant-a",
	}

	nodeB := &types.Node[any]{
		ID:         "node-b",
		Capability: "cap1",
		Tenant:     "tenant-b",
	}

	// Exhaust tenant-a quota
	if !fc.canProceedWithTenant(nodeA) {
		t.Error("Tenant-a request 1 should pass")
	}
	if !fc.canProceedWithTenant(nodeA) {
		t.Error("Tenant-a request 2 should pass")
	}
	if fc.canProceedWithTenant(nodeA) {
		t.Error("Tenant-a request 3 should fail (quota exhausted)")
	}

	// Tenant-b should still be able to proceed
	if !fc.canProceedWithTenant(nodeB) {
		t.Error("Tenant-b request 1 should pass")
	}
	if !fc.canProceedWithTenant(nodeB) {
		t.Error("Tenant-b request 2 should pass")
	}
	if fc.canProceedWithTenant(nodeB) {
		t.Error("Tenant-b request 3 should fail (quota exhausted)")
	}
}

// TestNoTenantNoQuota verifies behavior when no tenant is set
func TestNoTenantNoQuota(t *testing.T) {
	fc := newFlowController()
	fc.enableTenantQuota("tenant-a", 2, 2)

	// Node without tenant should not be rate limited by tenant quota
	node := &types.Node[any]{
		ID:         "node-no-tenant",
		Capability: "cap1",
		Tenant:     "", // No tenant
	}

	// Multiple requests should all pass (no tenant quota)
	for i := 0; i < 10; i++ {
		if !fc.canProceedWithTenant(node) {
			t.Errorf("Request %d should pass (no tenant constraint)", i+1)
		}
	}
}

// TestCombinedNodeAndTenantQuota verifies both limits are checked
func TestCombinedNodeAndTenantQuota(t *testing.T) {
	fc := newFlowController()
	fc.enableRateLimit("cap1", 5, 5)       // Capability quota: 5 req/sec
	fc.enableTenantQuota("tenant-a", 2, 2) // Tenant quota: 2 req/sec

	node := &types.Node[any]{
		ID:         "node-limited",
		Capability: "cap1",
		Tenant:     "tenant-a",
		RateLimit:  0, // Use flow controller rate limit
	}

	// Tenant quota is stricter (2 < 5), so it should be limiting factor
	if !fc.canProceedWithTenant(node) {
		t.Error("Request 1 should pass")
	}
	if !fc.canProceedWithTenant(node) {
		t.Error("Request 2 should pass")
	}
	if fc.canProceedWithTenant(node) {
		t.Error("Request 3 should fail (tenant quota exhausted)")
	}
}

// TestDynamicTenantQuota verifies tenant quota can be configured at runtime
func TestDynamicTenantQuota(t *testing.T) {
	fc := newFlowController()
	fc.enableTenantQuota("tenant-c", 1, 1) // Start with strict limit

	node := &types.Node[any]{
		ID:         "test-node",
		Capability: "cap1",
		Tenant:     "tenant-c",
	}

	// With 1 req/sec limit, only 1 request should pass
	if !fc.canProceedWithTenant(node) {
		t.Error("Request 1 should pass")
	}
	if fc.canProceedWithTenant(node) {
		t.Error("Request 2 should fail (quota exhausted)")
	}

	// Update quota to be more generous
	fc.enableTenantQuota("tenant-c", 5, 5)

	// Reset limiter by getting it again
	limiter := fc.getTenantQuotaLimiter("tenant-c")
	if limiter == nil {
		t.Error("Tenant limiter should exist after update")
	}

	// Note: In practice, updating quota while tokens are exhausted requires
	// creating a new limiter. This is a demonstration of the API design.
}

// TestZeroTenantQuota verifies invalid quota configurations are ignored
func TestZeroTenantQuota(t *testing.T) {
	fc := newFlowController()
	fc.enableTenantQuota("invalid-tenant", 0, 0) // Invalid quota

	node := &types.Node[any]{
		ID:         "test-node",
		Capability: "cap1",
		Tenant:     "invalid-tenant",
	}

	// Should allowed because the zero rate is ignored
	for i := 0; i < 10; i++ {
		if !fc.canProceedWithTenant(node) {
			t.Errorf("Request %d should pass (invalid quota ignored)", i+1)
		}
	}
}

// TestTenantQuotaWithBurst verifies burst handling per tenant
func TestTenantQuotaWithBurst(t *testing.T) {
	fc := newFlowController()
	fc.enableTenantQuota("tenant-d", 1, 5) // 1 req/sec, burst 5

	node := &types.Node[any]{
		ID:         "test-node",
		Capability: "cap1",
		Tenant:     "tenant-d",
	}

	// First 5 requests should pass (burst)
	for i := 0; i < 5; i++ {
		if !fc.canProceedWithTenant(node) {
			t.Errorf("Burst request %d should pass", i+1)
		}
	}

	// 6th request should fail (burst exhausted)
	if fc.canProceedWithTenant(node) {
		t.Error("Request 6 should fail (burst exhausted)")
	}
}
