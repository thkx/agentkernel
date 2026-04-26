package scheduler

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/event"
	"github.com/thkx/agentkernel/types"
)

func TestPriorityQueue(t *testing.T) {
	q := newTaskQueue(1000, RejectPolicyWait)
	base := time.Now()

	rst1 := q.enqueue(&taskItem{nodeID: "low", priority: 1, createdAt: base}, false)
	rst2 := q.enqueue(&taskItem{nodeID: "high", priority: 10, createdAt: base}, false)
	rst3 := q.enqueue(&taskItem{nodeID: "mid", priority: 5, createdAt: base}, false)

	if !rst1.success || !rst2.success || !rst3.success {
		t.Fatalf("Failed to enqueue tasks")
	}

	first := q.dequeue(false)
	if first.nodeID != "high" {
		t.Fatalf("Expected high priority first, got %q", first.nodeID)
	}

	second := q.dequeue(false)
	if second.nodeID != "mid" {
		t.Fatalf("Expected mid priority second, got %q", second.nodeID)
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := newRateLimiter(10, 10)
	for i := 0; i < 10; i++ {
		if !limiter.Allow() {
			t.Fatalf("Request %d denied prematurely", i)
		}
	}
	if limiter.Allow() {
		t.Fatal("11th request should be denied")
	}
}

func TestCircuitBreaker(t *testing.T) {
	cb := newCircuitBreaker(CircuitBreakerConfig{FailureThreshold: 2, ResetTimeout: 100 * time.Millisecond})

	if !cb.Allow() {
		t.Fatal("Circuit breaker should allow when closed")
	}

	cb.Record(false)
	cb.Record(false)

	if cb.Allow() {
		t.Fatal("Circuit breaker should be open")
	}

	time.Sleep(150 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("Circuit breaker should allow after reset")
	}
}

func TestRejectionPolicyDiscard(t *testing.T) {
	q := newTaskQueue(2, RejectPolicyDiscard)

	// Fill queue
	q.enqueue(&taskItem{nodeID: "t1", priority: 1, createdAt: time.Now()}, false)
	q.enqueue(&taskItem{nodeID: "t2", priority: 1, createdAt: time.Now()}, false)

	// Next should be rejected
	rst := q.enqueue(&taskItem{nodeID: "t3", priority: 1, createdAt: time.Now()}, false)
	if rst.success || !rst.dropped {
		t.Fatal("Expected task to be rejected with discard policy")
	}
}

func TestRejectionPolicyQueue(t *testing.T) {
	q := newTaskQueue(2, RejectPolicyQueue)

	// Fill queue and beyond (allows 10% overflow)
	for i := 0; i < 2; i++ {
		rst := q.enqueue(&taskItem{nodeID: types.NodeID(fmt.Sprintf("t%d", i)), priority: 1, createdAt: time.Now()}, false)
		if !rst.success {
			t.Fatalf("Failed to enqueue task %d", i)
		}
	}

	// Should still accept a bit more (elastic)
	rst := q.enqueue(&taskItem{nodeID: "t_elastic", priority: 1, createdAt: time.Now()}, false)
	if !rst.success {
		t.Fatal("Elastic queue should accept more than maxQueueSize")
	}
}

func TestFlowControl(t *testing.T) {
	fc := newFlowController()
	fc.enableRateLimit("test_cap", 5, 5)
	fc.enableCircuitBreaker("test_cap", CircuitBreakerConfig{FailureThreshold: 2, ResetTimeout: 100 * time.Millisecond})

	node := &types.Node[any]{ID: "n1", Capability: "test_cap"}

	if !fc.canProceed(node) {
		t.Fatal("Should proceed initially")
	}

	fc.recordResult(node, false)
	fc.recordResult(node, false)

	if !fc.isOpen(node) {
		t.Fatal("Circuit should be open")
	}
}

func TestFlowControlUsesCircuitBreakerKey(t *testing.T) {
	fc := newFlowController()
	fc.enableCircuitBreaker("custom-breaker", CircuitBreakerConfig{FailureThreshold: 1, ResetTimeout: 100 * time.Millisecond})

	node := &types.Node[any]{
		ID:                "n1",
		Capability:        "test_cap",
		CircuitBreakerKey: "custom-breaker",
	}

	fc.recordResult(node, false)

	if !fc.isOpen(node) {
		t.Fatal("Circuit should use node-specific breaker key")
	}
}

func TestRunWithContextStartsFromReachableSubgraph(t *testing.T) {
	store := event.NewInMemoryEventStore()
	bus := event.NewSourcingBus(store)
	registry := capability.NewRegistry()

	var (
		mu       sync.Mutex
		executed []string
	)

	registry.Register(testCapability{
		name: "cap",
		fn: func(ctx types.ExecContext, input any) (any, error) {
			mu.Lock()
			executed = append(executed, input.(string))
			mu.Unlock()
			return input, nil
		},
	})

	graph := NewGraphEngine(types.NewGraph[any]("start", map[types.NodeID]*types.Node[any]{
		"start": {
			ID:         "start",
			Capability: "cap",
			Input:      "start",
			Next:       []types.Edge{{To: "child"}},
		},
		"child": {
			ID:         "child",
			Capability: "cap",
			Input:      "child",
		},
		"other-root": {
			ID:         "other-root",
			Capability: "cap",
			Input:      "other-root",
		},
	}))

	s := New(graph, bus, registry)
	s.RunWithContext(context.Background(), "start", map[string]any{})

	mu.Lock()
	defer mu.Unlock()
	if len(executed) != 2 {
		t.Fatalf("expected 2 reachable nodes to execute, got %d: %v", len(executed), executed)
	}
	for _, node := range executed {
		if node == "other-root" {
			t.Fatalf("unexpected execution of unrelated root node: %v", executed)
		}
	}
}

func TestRunWithContextHandlesRateLimitedRequeue(t *testing.T) {
	store := event.NewInMemoryEventStore()
	bus := event.NewSourcingBus(store)
	registry := capability.NewRegistry()

	var (
		mu       sync.Mutex
		executed []string
	)

	registry.Register(testCapability{
		name: "cap",
		fn: func(ctx types.ExecContext, input any) (any, error) {
			mu.Lock()
			executed = append(executed, input.(string))
			mu.Unlock()
			return input, nil
		},
	})

	graph := NewGraphEngine(types.NewGraph[any]("start", map[types.NodeID]*types.Node[any]{
		"start": {
			ID:         "start",
			Capability: "cap",
			Input:      "start",
			Next:       []types.Edge{{To: "child"}},
		},
		"child": {
			ID:         "child",
			Capability: "cap",
			Input:      "child",
		},
	}))

	s := New(graph, bus, registry, WithCapabilityRateLimit("cap", 1, 1))
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()

	s.RunWithContext(ctx, "start", map[string]any{})

	mu.Lock()
	defer mu.Unlock()
	if len(executed) != 2 {
		t.Fatalf("expected both rate-limited tasks to execute, got %d: %v", len(executed), executed)
	}
}

func TestSchedulerRunStateResetsBetweenRuns(t *testing.T) {
	store := event.NewInMemoryEventStore()
	bus := event.NewSourcingBus(store)
	registry := capability.NewRegistry()

	var (
		mu     sync.Mutex
		counts = map[string]int{}
	)

	registry.Register(testCapability{
		name: "cap",
		fn: func(ctx types.ExecContext, input any) (any, error) {
			key := input.(string)
			mu.Lock()
			counts[key]++
			mu.Unlock()
			return input, nil
		},
	})

	graph := NewGraphEngine(types.NewGraph[any]("start", map[types.NodeID]*types.Node[any]{
		"start": {
			ID:         "start",
			Capability: "cap",
			Input:      "start",
			Next:       []types.Edge{{To: "child"}},
		},
		"child": {
			ID:         "child",
			Capability: "cap",
			Input:      "child",
		},
	}))

	s := New(graph, bus, registry, WithCapabilityRateLimit("cap", 1, 1))

	firstCtx, firstCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer firstCancel()
	s.RunWithContext(firstCtx, "start", map[string]any{})

	secondCtx, secondCancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer secondCancel()
	s.RunWithContext(secondCtx, "start", map[string]any{})

	mu.Lock()
	defer mu.Unlock()
	if counts["start"] != 1 {
		t.Fatalf("expected start node to execute once, got %d", counts["start"])
	}
	if counts["child"] != 1 {
		t.Fatalf("expected child node to execute once after rerun, got %d", counts["child"])
	}
}

func TestExecContextStateStoreIsSharedSafely(t *testing.T) {
	store := event.NewInMemoryEventStore()
	bus := event.NewSourcingBus(store)
	registry := capability.NewRegistry()

	registry.Register(testCapability{
		name: "cap",
		fn: func(ctx types.ExecContext, input any) (any, error) {
			key := input.(string)
			ctx.SetState(key, true)
			if ctx.GetState("seed") != "value" {
				t.Fatalf("expected seeded state to be visible, got %v", ctx.GetState("seed"))
			}
			return key, nil
		},
	})

	graph := NewGraphEngine(types.NewGraph[any]("start", map[types.NodeID]*types.Node[any]{
		"start": {
			ID:         "start",
			Capability: "cap",
			Input:      "start",
			Next: []types.Edge{
				{To: "left"},
				{To: "right"},
			},
		},
		"left": {
			ID:         "left",
			Capability: "cap",
			Input:      "left",
		},
		"right": {
			ID:         "right",
			Capability: "cap",
			Input:      "right",
		},
	}))

	s := New(graph, bus, registry)
	s.RunWithContext(context.Background(), "start", map[string]any{"seed": "value"})

	snapshot := store.Snapshot()
	if snapshot["start"] != "start" {
		t.Fatalf("expected event snapshot for start node, got %v", snapshot["start"])
	}
}

func TestHookContextReceivesSharedStateSnapshot(t *testing.T) {
	store := event.NewInMemoryEventStore()
	bus := event.NewSourcingBus(store)
	registry := capability.NewRegistry()

	registry.Register(testCapability{
		name: "cap",
		fn: func(ctx types.ExecContext, input any) (any, error) {
			ctx.SetState("written", input)
			return input, nil
		},
	})

	graph := NewGraphEngine(types.NewGraph[any]("start", map[types.NodeID]*types.Node[any]{
		"start": {
			ID:         "start",
			Capability: "cap",
			Input:      "payload",
		},
	}))

	var seen any
	s := New(graph, bus, registry)
	s.RegisterAfterHook(func(ctx types.HookContext) {
		seen = ctx.GetState("written")
		if ctx.StateStore == nil {
			t.Fatal("expected hook context to receive state store")
		}
	})
	s.RunWithContext(context.Background(), "start", map[string]any{})

	if seen != "payload" {
		t.Fatalf("expected hook to observe shared state snapshot, got %v", seen)
	}
}

func TestExecContextStateSnapshotDoesNotLeakNestedMutation(t *testing.T) {
	store := event.NewInMemoryEventStore()
	bus := event.NewSourcingBus(store)
	registry := capability.NewRegistry()

	registry.Register(testCapability{
		name: "cap",
		fn: func(ctx types.ExecContext, input any) (any, error) {
			ctx.State["payload"].(map[string]any)["count"] = 99
			if ctx.GetState("payload").(map[string]any)["count"] != 1 {
				t.Fatalf("expected shared state to remain isolated from snapshot mutation")
			}
			ctx.SetState("confirmed", true)
			return input, nil
		},
	})

	graph := NewGraphEngine(types.NewGraph[any]("start", map[types.NodeID]*types.Node[any]{
		"start": {
			ID:         "start",
			Capability: "cap",
			Input:      "payload",
		},
	}))

	s := New(graph, bus, registry)
	s.RunWithContext(context.Background(), "start", map[string]any{
		"payload": map[string]any{"count": 1},
	})
}

func TestHookContextCanUpdateSharedState(t *testing.T) {
	store := event.NewInMemoryEventStore()
	bus := event.NewSourcingBus(store)
	registry := capability.NewRegistry()

	registry.Register(testCapability{
		name: "cap",
		fn: func(ctx types.ExecContext, input any) (any, error) {
			return input, nil
		},
	})

	graph := NewGraphEngine(types.NewGraph[any]("start", map[types.NodeID]*types.Node[any]{
		"start": {
			ID:         "start",
			Capability: "cap",
			Input:      "payload",
		},
	}))

	var seen any
	s := New(graph, bus, registry)
	s.RegisterAfterWritableHook(func(ctx types.WritableHookContext) {
		ctx.SetState("hook_written", "ok")
		seen = ctx.GetState("hook_written")
	})
	s.RunWithContext(context.Background(), "start", map[string]any{})

	if seen != "ok" {
		t.Fatalf("expected hook to update shared state, got %v", seen)
	}
}

func TestExecAndHookContextCanDeleteSharedState(t *testing.T) {
	store := event.NewInMemoryEventStore()
	bus := event.NewSourcingBus(store)
	registry := capability.NewRegistry()

	registry.Register(testCapability{
		name: "cap",
		fn: func(ctx types.ExecContext, input any) (any, error) {
			if ctx.GetState("seed") != "value" {
				t.Fatalf("expected seeded state before delete, got %v", ctx.GetState("seed"))
			}
			ctx.DeleteState("seed")
			if ctx.GetState("seed") != nil {
				t.Fatalf("expected exec context delete to remove shared state, got %v", ctx.GetState("seed"))
			}
			return input, nil
		},
	})

	graph := NewGraphEngine(types.NewGraph[any]("start", map[types.NodeID]*types.Node[any]{
		"start": {
			ID:         "start",
			Capability: "cap",
			Input:      "payload",
		},
	}))

	s := New(graph, bus, registry)
	s.RegisterAfterWritableHook(func(ctx types.WritableHookContext) {
		ctx.SetState("hook_key", "present")
		ctx.DeleteState("hook_key")
		if ctx.GetState("hook_key") != nil {
			t.Fatalf("expected hook delete to remove shared state, got %v", ctx.GetState("hook_key"))
		}
	})
	s.RunWithContext(context.Background(), "start", map[string]any{"seed": "value"})
}

func TestHooksAreReadOnlyByDefault(t *testing.T) {
	store := event.NewInMemoryEventStore()
	bus := event.NewSourcingBus(store)
	registry := capability.NewRegistry()

	registry.Register(testCapability{
		name: "cap",
		fn: func(ctx types.ExecContext, input any) (any, error) {
			return input, nil
		},
	})

	graph := NewGraphEngine(types.NewGraph[any]("start", map[types.NodeID]*types.Node[any]{
		"start": {
			ID:         "start",
			Capability: "cap",
			Input:      "payload",
		},
	}))

	s := New(graph, bus, registry)
	s.RegisterAfterHook(func(ctx types.HookContext) {
		if ctx.GetState("hook_written") != nil {
			t.Fatalf("expected read-only hook to observe nil shared state, got %v", ctx.GetState("hook_written"))
		}
		if _, ok := ctx.StateStore.(types.StateAccessor); ok {
			t.Fatal("expected default hook state store to be read-only")
		}
	})
	s.RunWithContext(context.Background(), "start", map[string]any{})
}

func TestWritableHooksCanMutateSharedStateWithoutGlobalFlag(t *testing.T) {
	store := event.NewInMemoryEventStore()
	bus := event.NewSourcingBus(store)
	registry := capability.NewRegistry()

	registry.Register(testCapability{
		name: "cap",
		fn: func(ctx types.ExecContext, input any) (any, error) {
			return input, nil
		},
	})

	graph := NewGraphEngine(types.NewGraph[any]("start", map[types.NodeID]*types.Node[any]{
		"start": {
			ID:         "start",
			Capability: "cap",
			Input:      "payload",
		},
	}))

	var seen any
	s := New(graph, bus, registry)
	s.RegisterAfterWritableHook(func(ctx types.WritableHookContext) {
		ctx.SetState("hook_written", "ok")
		seen = ctx.GetState("hook_written")
		if ctx.StateStore == nil {
			t.Fatal("expected writable hook state store to be available")
		}
	})
	s.RunWithContext(context.Background(), "start", map[string]any{})

	if seen != "ok" {
		t.Fatalf("expected writable hook to update shared state, got %v", seen)
	}
}

func TestRunWithContextSupportsControlResultJump(t *testing.T) {
	store := event.NewInMemoryEventStore()
	bus := event.NewSourcingBus(store)
	registry := capability.NewRegistry()

	var (
		mu       sync.Mutex
		executed []string
	)

	registry.Register(testCapability{
		name: "cap",
		fn: func(ctx types.ExecContext, input any) (any, error) {
			label := input.(string)
			mu.Lock()
			executed = append(executed, label)
			mu.Unlock()
			if label == "start" {
				return types.Result{
					Output:   "jumping",
					Status:   types.SUCCESS,
					Control:  types.JUMP,
					NextNode: "target",
				}, nil
			}
			return label, nil
		},
	})

	graph := NewGraphEngine(types.NewGraph[any]("start", map[types.NodeID]*types.Node[any]{
		"start": {
			ID:         "start",
			Capability: "cap",
			Input:      "start",
			Next:       []types.Edge{{To: "skipped"}},
		},
		"skipped": {
			ID:         "skipped",
			Capability: "cap",
			Input:      "skipped",
		},
		"target": {
			ID:         "target",
			Capability: "cap",
			Input:      "target",
		},
	}))

	s := New(graph, bus, registry)
	s.RunWithContext(context.Background(), "start", map[string]any{})

	mu.Lock()
	defer mu.Unlock()
	if len(executed) != 2 {
		t.Fatalf("expected 2 nodes to execute after jump, got %d: %v", len(executed), executed)
	}
	if executed[0] != "start" || executed[1] != "target" {
		t.Fatalf("expected jump to execute start then target, got %v", executed)
	}
}

func TestRunWithContextPersistsSharedStateSnapshot(t *testing.T) {
	store := event.NewInMemoryEventStore()
	bus := event.NewSourcingBus(store)
	registry := capability.NewRegistry()

	registry.Register(testCapability{
		name: "cap",
		fn: func(ctx types.ExecContext, input any) (any, error) {
			ctx.SetState("last_output", input)
			return input, nil
		},
	})

	graph := NewGraphEngine(types.NewGraph[any]("start", map[types.NodeID]*types.Node[any]{
		"start": {
			ID:         "start",
			Capability: "cap",
			Input:      "payload",
		},
	}))

	s := New(graph, bus, registry)
	s.RunWithContext(context.Background(), "start", map[string]any{"seed": "value"})

	snapshot := s.StateSnapshot()
	if snapshot["seed"] != "value" {
		t.Fatalf("expected initial state to survive in scheduler snapshot, got %v", snapshot["seed"])
	}
	if snapshot["last_output"] != "payload" {
		t.Fatalf("expected runtime state to include capability writes, got %v", snapshot["last_output"])
	}

	states := s.TaskStates()
	if states["start"] != types.Done {
		t.Fatalf("expected task state to be done, got %s", states["start"])
	}
}

type testCapability struct {
	name types.CapabilityName
	fn   func(ctx types.ExecContext, input any) (any, error)
}

func (t testCapability) Name() types.CapabilityName {
	return t.name
}

func (t testCapability) Invoke(ctx types.ExecContext, input any) (any, error) {
	return t.fn(ctx, input)
}
