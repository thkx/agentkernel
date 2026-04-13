package runtime

import (
	"github.com/thkx/agentkernel/event"
	"github.com/thkx/agentkernel/policy"
	"github.com/thkx/agentkernel/scheduler"
	"github.com/thkx/agentkernel/worker"
)

type Runtime struct {
	policy *policy.PolicyEngine
	sched  *scheduler.Scheduler
}

func NewRuntime() *Runtime {
	bus := event.NewBus()

	s := scheduler.NewScheduler(bus)

	s.SetWorkers([]*worker.Worker{
		{ID: 1},
	})

	return &Runtime{
		policy: &policy.PolicyEngine{},
		sched:  s,
	}
}

func (r *Runtime) Start(input any) {
	tasks := r.policy.Plan(input)

	go r.sched.Run()

	for _, t := range tasks {
		r.sched.Submit(t)
	}
}
