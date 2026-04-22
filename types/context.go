package types

import (
	"context"

	"github.com/thkx/agentkernel/state"
)

type ReadOnlyStateAccessor interface {
	Get(key string) any
	Snapshot() map[string]any
}

type StateAccessor interface {
	ReadOnlyStateAccessor
	Set(key string, value any)
	Delete(key string)
}

type readOnlyStateView struct {
	store ReadOnlyStateAccessor
}

func (r readOnlyStateView) Get(key string) any {
	return r.store.Get(key)
}

func (r readOnlyStateView) Snapshot() map[string]any {
	return r.store.Snapshot()
}

type ExecContext struct {
	Ctx     context.Context
	TraceID string
	SpanID  string
	NodeID  NodeID
	// Deprecated: State is a point-in-time snapshot for read-only inspection.
	// Use GetState/SetState/DeleteState/SnapshotState for stateful logic.
	State      map[string]any
	StateStore ReadOnlyStateAccessor
}

type HookContext struct {
	TraceID    string
	SpanID     string
	NodeID     NodeID
	Capability CapabilityName
	Input      any
	Attempt    int
	// Deprecated: State is a point-in-time snapshot for read-only inspection.
	// Use GetState/SetState/DeleteState/SnapshotState for stateful logic.
	State      map[string]any
	StateStore ReadOnlyStateAccessor
	Result     Result
}

type HookFunc func(HookContext)
type WritableHookContext struct {
	HookContext
	StateStore StateAccessor
}

type WritableHookFunc func(WritableHookContext)

func NewExecContext(ctx context.Context, traceID, spanID string, nodeID NodeID, store ReadOnlyStateAccessor) ExecContext {
	stateSnapshot := map[string]any{}
	if store != nil {
		stateSnapshot = store.Snapshot()
	}
	return ExecContext{
		Ctx:        ctx,
		TraceID:    traceID,
		SpanID:     spanID,
		NodeID:     nodeID,
		State:      stateSnapshot,
		StateStore: store,
	}
}

func NewHookContext(traceID, spanID string, nodeID NodeID, capability CapabilityName, input any, attempt int, store ReadOnlyStateAccessor, result Result) HookContext {
	stateSnapshot := map[string]any{}
	readOnlyStore := store
	if store != nil {
		stateSnapshot = store.Snapshot()
		readOnlyStore = readOnlyStateView{store: store}
	}
	return HookContext{
		TraceID:    traceID,
		SpanID:     spanID,
		NodeID:     nodeID,
		Capability: capability,
		Input:      input,
		Attempt:    attempt,
		State:      stateSnapshot,
		StateStore: readOnlyStore,
		Result:     result,
	}
}

func NewWritableHookContext(traceID, spanID string, nodeID NodeID, capability CapabilityName, input any, attempt int, store StateAccessor, result Result) WritableHookContext {
	base := NewHookContext(traceID, spanID, nodeID, capability, input, attempt, store, result)
	return WritableHookContext{
		HookContext: base,
		StateStore:  store,
	}
}

func (c ExecContext) GetState(key string) any {
	if c.StateStore != nil {
		return c.StateStore.Get(key)
	}
	return c.State[key]
}

func (c ExecContext) SetState(key string, value any) {
	if writable, ok := c.StateStore.(StateAccessor); ok {
		writable.Set(key, value)
		c.State[key] = state.CloneValue(value)
		return
	}
	c.State[key] = state.CloneValue(value)
}

func (c ExecContext) DeleteState(key string) {
	if writable, ok := c.StateStore.(StateAccessor); ok {
		writable.Delete(key)
	}
	delete(c.State, key)
}

func (c ExecContext) SnapshotState() map[string]any {
	if c.StateStore != nil {
		return c.StateStore.Snapshot()
	}
	snapshot := make(map[string]any, len(c.State))
	for k, v := range c.State {
		snapshot[k] = state.CloneValue(v)
	}
	return snapshot
}

func (c HookContext) GetState(key string) any {
	if c.StateStore != nil {
		return c.StateStore.Get(key)
	}
	return c.State[key]
}

func (c HookContext) SnapshotState() map[string]any {
	if c.StateStore != nil {
		return c.StateStore.Snapshot()
	}
	snapshot := make(map[string]any, len(c.State))
	for k, v := range c.State {
		snapshot[k] = state.CloneValue(v)
	}
	return snapshot
}

// SetState mutates shared runtime state for writable hooks only.
func (c WritableHookContext) SetState(key string, value any) {
	if c.StateStore != nil {
		c.StateStore.Set(key, value)
		c.State[key] = state.CloneValue(value)
		return
	}
	c.State[key] = state.CloneValue(value)
}

// DeleteState removes shared runtime state for writable hooks only.
func (c WritableHookContext) DeleteState(key string) {
	if c.StateStore != nil {
		c.StateStore.Delete(key)
	}
	delete(c.State, key)
}
