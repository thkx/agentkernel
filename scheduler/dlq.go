package scheduler

import (
	"sync"

	"github.com/thkx/agentkernel/types"
)

type DeadLetterQueue struct {
	mu    sync.RWMutex
	queue []types.Task
}

func NewDeadLetterQueue() *DeadLetterQueue {
	return &DeadLetterQueue{
		queue: make([]types.Task, 0),
	}
}

func (dlq *DeadLetterQueue) Enqueue(task types.Task) {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	dlq.queue = append(dlq.queue, task)
}

func (dlq *DeadLetterQueue) Dequeue() (types.Task, bool) {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	if len(dlq.queue) == 0 {
		return types.Task{}, false
	}
	task := dlq.queue[0]
	dlq.queue = dlq.queue[1:]
	return task, true
}

func (dlq *DeadLetterQueue) Size() int {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	return len(dlq.queue)
}
