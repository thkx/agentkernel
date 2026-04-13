package runtime

import (
	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/event"
	"github.com/thkx/agentkernel/policy"
	"github.com/thkx/agentkernel/scheduler"
)

type Runtime struct {
	sched *scheduler.Scheduler
	pol   *policy.Planner
	bus   *event.Bus
}

func NewRuntime() *Runtime {

	bus := event.NewBus()

	pl := &policy.Planner{}

	graph := pl.Build("hello v0.2")

	engine := scheduler.NewGraphEngine(graph)

	s := scheduler.New(engine, bus)

	s.Register(&capability.LLM{})
	s.Register(&capability.Tool{})

	return &Runtime{
		sched: s,
		pol:   pl,
		bus:   bus,
	}
}

func (r *Runtime) Run() {
	go r.sched.Run("n1", map[string]any{})
}
