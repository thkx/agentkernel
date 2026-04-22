package types

import (
	"context"
	"testing"

	"github.com/thkx/agentkernel/state"
)

func TestNewExecContextUsesSnapshotAndStore(t *testing.T) {
	store := state.NewStoreFromMap(map[string]any{
		"counter": 1,
		"payload": map[string]any{"nested": "value"},
	})

	ctx := NewExecContext(context.Background(), "trace", "span", "node-1", store)
	ctx.State["payload"].(map[string]any)["nested"] = "changed"

	if got := ctx.GetState("counter"); got != 1 {
		t.Fatalf("expected shared store value, got %v", got)
	}
	if got := store.Get("payload").(map[string]any)["nested"]; got != "value" {
		t.Fatalf("expected snapshot mutation to stay isolated, got %v", got)
	}
}

type readOnlyStore struct {
	data map[string]any
}

func (r readOnlyStore) Get(key string) any {
	return state.CloneValue(r.data[key])
}

func (r readOnlyStore) Snapshot() map[string]any {
	snapshot := make(map[string]any, len(r.data))
	for k, v := range r.data {
		snapshot[k] = state.CloneValue(v)
	}
	return snapshot
}

func TestNewHookContextUsesSnapshotAndStore(t *testing.T) {
	store := state.NewStoreFromMap(map[string]any{
		"status": "ready",
	})

	hookCtx := NewHookContext("trace", "span", "node-1", "cap", "input", 2, store, Result{Status: SUCCESS})
	if got := hookCtx.GetState("status"); got != "ready" {
		t.Fatalf("expected hook context to read shared store, got %v", got)
	}
	if _, ok := hookCtx.StateStore.(StateAccessor); ok {
		t.Fatal("expected plain hook context to expose read-only state access")
	}
}

func TestNewWritableHookContextUsesWritableStore(t *testing.T) {
	store := state.NewStoreFromMap(map[string]any{
		"status": "ready",
	})

	hookCtx := NewWritableHookContext("trace", "span", "node-1", "cap", "input", 2, store, Result{Status: SUCCESS})
	hookCtx.SetState("status", "done")
	hookCtx.DeleteState("missing")
	if got := store.Get("status"); got != "done" {
		t.Fatalf("expected writable hook context writes to update store, got %v", got)
	}
	if hookCtx.StateStore == nil {
		t.Fatal("expected writable hook context to expose writable state store")
	}
	if _, ok := hookCtx.HookContext.StateStore.(StateAccessor); ok {
		t.Fatal("expected embedded plain hook context to remain read-only")
	}
}

func TestContextsSupportReadOnlyStateStore(t *testing.T) {
	store := readOnlyStore{
		data: map[string]any{"status": "ready"},
	}

	execCtx := NewExecContext(context.Background(), "trace", "span", "node-1", store)
	if got := execCtx.GetState("status"); got != "ready" {
		t.Fatalf("expected exec context to read from read-only store, got %v", got)
	}
	execCtx.SetState("status", "local")
	if got := execCtx.State["status"]; got != "local" {
		t.Fatalf("expected local snapshot update when store is read-only, got %v", got)
	}

	hookCtx := NewHookContext("trace", "span", "node-1", "cap", "input", 1, store, Result{Status: SUCCESS})
	if got := hookCtx.GetState("status"); got != "ready" {
		t.Fatalf("expected read-only hook context to leave backing store intact, got %v", got)
	}
}
