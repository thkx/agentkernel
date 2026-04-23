package runtime

import (
	"context"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/event"
	"github.com/thkx/agentkernel/scheduler"
	"github.com/thkx/agentkernel/types"
)

type Runtime struct {
	sched            types.Scheduler
	bus              types.EventBus
	caps             *capability.Registry
	graph            *types.Graph[any]
	beforeHooks      []types.HookFunc
	afterHooks       []types.HookFunc
	beforeWritable   []types.WritableHookFunc
	afterWritable    []types.WritableHookFunc
	schedulerOptions []scheduler.SchedulerOption
}

func NewRuntime(opts ...RuntimeOption) *Runtime {
	r := &Runtime{
		bus:  event.NewSourcingBus(event.NewInMemoryEventStore()),
		caps: capability.NewRegistry(),
	}

	for _, opt := range opts {
		opt(r)
	}

	if r.bus == nil {
		r.bus = event.NewSourcingBus(event.NewInMemoryEventStore())
	}
	if r.caps == nil {
		r.caps = capability.NewRegistry()
	}

	engine := scheduler.NewGraphEngine(r.graph)
	r.sched = scheduler.New(engine, r.bus, r.caps, r.schedulerOptions...)

	for _, hook := range r.beforeHooks {
		r.sched.RegisterBeforeHook(hook)
	}
	for _, hook := range r.afterHooks {
		r.sched.RegisterAfterHook(hook)
	}
	for _, hook := range r.beforeWritable {
		r.sched.RegisterBeforeWritableHook(hook)
	}
	for _, hook := range r.afterWritable {
		r.sched.RegisterAfterWritableHook(hook)
	}

	return r
}

func (r *Runtime) Run() {
	go r.sched.Run(r.graph.Start, map[string]any{})
}

func (r *Runtime) RunWithContext(ctx context.Context) {
	go r.sched.RunWithContext(ctx, r.graph.Start, map[string]any{})
}

func (r *Runtime) RunSync() {
	if r.graph == nil {
		// Log error or handle gracefully
		return
	}
	r.sched.Run(r.graph.Start, map[string]any{})
}

func (r *Runtime) RunSyncWithContext(ctx context.Context) {
	r.sched.RunWithContext(ctx, r.graph.Start, map[string]any{})
}

func (r *Runtime) Bus() types.EventBus {
	return r.bus
}

func (r *Runtime) Scheduler() types.Scheduler {
	return r.sched
}
