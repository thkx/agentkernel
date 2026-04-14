package runtime

import (
	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/event"
	"github.com/thkx/agentkernel/policy"
	"github.com/thkx/agentkernel/scheduler"
	"github.com/thkx/agentkernel/types"
)

type RuntimeOption func(*Runtime)

type Runtime struct {
	sched *scheduler.Scheduler
	pol   *policy.Planner
	bus   *event.SourcingBus
	caps  *capability.Registry
	graph *types.Graph[any]
}

func NewRuntime(opts ...RuntimeOption) *Runtime {
	r := &Runtime{
		bus:  event.NewSourcingBus(event.NewInMemoryEventStore()),
		pol:  &policy.Planner{},
		caps: capability.NewRegistry(),
	}

	for _, opt := range opts {
		opt(r)
	}

	if r.pol == nil {
		r.pol = &policy.Planner{}
	}
	if r.bus == nil {
		r.bus = event.NewSourcingBus(event.NewInMemoryEventStore())
	}
	if r.caps == nil {
		r.caps = capability.NewRegistry()
	}
	if r.graph == nil {
		r.graph = r.pol.Build("hello v0.5")
	}

	engine := scheduler.NewGraphEngine(r.graph)
	r.sched = scheduler.New(engine, r.bus, r.caps)

	return r
}

func WithPlanner(pl *policy.Planner) RuntimeOption {
	return func(r *Runtime) {
		r.pol = pl
	}
}

func WithBus(bus *event.SourcingBus) RuntimeOption {
	return func(r *Runtime) {
		r.bus = bus
	}
}

func WithCapabilityRegistry(reg *capability.Registry) RuntimeOption {
	return func(r *Runtime) {
		r.caps = reg
	}
}

func WithGraph(graph *types.Graph[any]) RuntimeOption {
	return func(r *Runtime) {
		r.graph = graph
	}
}

func WithPlugin(plugin capability.Plugin) RuntimeOption {
	return func(r *Runtime) {
		if r.caps == nil {
			r.caps = capability.NewRegistry()
		}
		r.caps.Load(plugin)
	}
}

func (r *Runtime) Run() {
	go r.sched.Run(r.graph.Start, map[string]any{})
}

func (r *Runtime) Bus() *event.SourcingBus {
	return r.bus
}
