package scheduler

import "github.com/thkx/agentkernel/types"

type readOnlyStateView struct {
	store types.ReadOnlyStateAccessor
}

func (r readOnlyStateView) Get(key string) any {
	return r.store.Get(key)
}

func (r readOnlyStateView) Snapshot() map[string]any {
	return r.store.Snapshot()
}
