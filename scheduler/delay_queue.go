package scheduler

import (
	"container/heap"
	"time"

	"github.com/thkx/agentkernel/types"
)

type delayedTask struct {
	nodeID  types.NodeID
	readyAt time.Time
	index   int
}

type delayedTaskHeap []*delayedTask

func (h delayedTaskHeap) Len() int { return len(h) }

func (h delayedTaskHeap) Less(i, j int) bool {
	return h[i].readyAt.Before(h[j].readyAt)
}

func (h delayedTaskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *delayedTaskHeap) Push(x any) {
	item := x.(*delayedTask)
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *delayedTaskHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

type delayedTaskQueue struct {
	items delayedTaskHeap
}

func newDelayedTaskQueue() *delayedTaskQueue {
	q := &delayedTaskQueue{
		items: make(delayedTaskHeap, 0),
	}
	heap.Init(&q.items)
	return q
}

func (q *delayedTaskQueue) push(nodeID types.NodeID, readyAt time.Time) {
	heap.Push(&q.items, &delayedTask{nodeID: nodeID, readyAt: readyAt})
}

func (q *delayedTaskQueue) popReady(now time.Time) (types.NodeID, bool) {
	if len(q.items) == 0 {
		return "", false
	}
	item := q.items[0]
	if item.readyAt.After(now) {
		return "", false
	}
	heap.Pop(&q.items)
	return item.nodeID, true
}

func (q *delayedTaskQueue) nextReadyAt() (time.Time, bool) {
	if len(q.items) == 0 {
		return time.Time{}, false
	}
	return q.items[0].readyAt, true
}

func (q *delayedTaskQueue) reset() {
	q.items = q.items[:0]
}

func (q *delayedTaskQueue) len() int {
	return len(q.items)
}
