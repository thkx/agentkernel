package scheduler

import (
	"fmt"

	"github.com/thkx/agentkernel/event"
	"github.com/thkx/agentkernel/types"
	"github.com/thkx/agentkernel/worker"
)

type Scheduler struct {
	tasks    chan types.Task
	workers  []*worker.Worker
	eventBus *event.Bus
}

func NewScheduler(bus *event.Bus) *Scheduler {
	return &Scheduler{
		tasks:    make(chan types.Task, 100),
		eventBus: bus,
	}
}

func (s *Scheduler) Submit(task types.Task) {
	s.tasks <- task
}

func (s *Scheduler) Run() {
	for task := range s.tasks {

		w := s.workers[0]

		result := w.Do(task)

		fmt.Println("executed:", task.ID, result.Output)

		s.eventBus.Publish(event.Event{
			TaskID: task.ID,
			Result: result,
		})
	}
}

func (s *Scheduler) SetWorkers(ws []*worker.Worker) {
	s.workers = ws
}
