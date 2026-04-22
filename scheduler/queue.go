package scheduler

import (
	"container/heap"
	"sync"
	"time"

	"github.com/thkx/agentkernel/types"
)

// taskItem represents a queued task
type taskItem struct {
	nodeID    types.NodeID
	priority  int
	tenant    string
	createdAt time.Time
	index     int
}

// taskHeap implements heap.Interface for priority queue
type taskHeap []*taskItem

func (h taskHeap) Len() int { return len(h) }

func (h taskHeap) Less(i, j int) bool {
	if h[i].priority != h[j].priority {
		return h[i].priority > h[j].priority
	}
	return h[i].createdAt.Before(h[j].createdAt)
}

func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *taskHeap) Push(x any) {
	item := x.(*taskItem)
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *taskHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

func (h *taskHeap) Reset() {
	*h = (*h)[:0]
}

// RejectionPolicy defines queue rejection behavior
type RejectionPolicy int

const (
	RejectPolicyWait RejectionPolicy = iota
	RejectPolicyDiscard
	RejectPolicyQueue
)

// taskQueue manages task queueing with priority and tenant awareness
type taskQueue struct {
	mu            sync.Mutex
	cond          *sync.Cond
	queue         taskHeap
	tenantQueues  map[string]*taskHeap
	tenantOrder   []string
	currentTenant int
	totalSize     int
	maxQueueSize  int
	rejectionPol  RejectionPolicy
}

// newTaskQueue creates a new task queue with configurable rejection policy
func newTaskQueue(maxQueueSize int, rejectionPolicy RejectionPolicy) *taskQueue {
	tq := &taskQueue{
		queue:        make(taskHeap, 0),
		tenantQueues: make(map[string]*taskHeap),
		maxQueueSize: maxQueueSize,
		rejectionPol: rejectionPolicy,
	}
	tq.cond = sync.NewCond(&tq.mu)
	return tq
}

// enqueueResult indicates whether enqueue succeeded
type enqueueResult struct {
	success bool
	dropped bool
}

// enqueue adds a task to the queue, respecting rejection policy
// Returns (success, dropped). Success=true means accepted. Dropped=true means rejected and task lost.
func (tq *taskQueue) enqueue(item *taskItem, tenantAware bool) enqueueResult {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	queueLen := tq.lengthLocked()
	if queueLen >= tq.maxQueueSize {
		switch tq.rejectionPol {
		case RejectPolicyDiscard:
			return enqueueResult{success: false, dropped: true}
		case RejectPolicyWait:
			for tq.lengthLocked() >= tq.maxQueueSize {
				tq.cond.Wait()
			}
		case RejectPolicyQueue:
			// Allow queue to grow beyond maxQueueSize by 50%
			if queueLen >= int(float64(tq.maxQueueSize)*1.5) {
				return enqueueResult{success: false, dropped: true}
			}
		}
	}

	if tenantAware && item.tenant != "" {
		heap.Push(tq.getTenantQueueLocked(item.tenant), item)
	} else {
		heap.Push(&tq.queue, item)
	}
	tq.totalSize++

	tq.cond.Signal()
	return enqueueResult{success: true, dropped: false}
}

// dequeue pops the next task based on priority and tenant fairness
func (tq *taskQueue) dequeue(tenantAware bool) *taskItem {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	if tq.lengthLocked() == 0 {
		return nil
	}

	if tenantAware && len(tq.tenantOrder) > 0 {
		for i := 0; i < len(tq.tenantOrder); i++ {
			idx := (tq.currentTenant + i) % len(tq.tenantOrder)
			tenant := tq.tenantOrder[idx]
			q := tq.tenantQueues[tenant]
			if q != nil && q.Len() > 0 {
				tq.currentTenant = (idx + 1) % len(tq.tenantOrder)
				item := heap.Pop(q).(*taskItem)
				tq.totalSize--
				if q.Len() == 0 {
					tq.removeTenantQueueLocked(tenant)
				}
				tq.cond.Signal()
				return item
			}
		}
	}

	if tq.queue.Len() > 0 {
		item := heap.Pop(&tq.queue).(*taskItem)
		tq.totalSize--
		tq.cond.Signal()
		return item
	}

	return nil
}

// len returns the total queue length (for internal use)
func (tq *taskQueue) lengthLocked() int {
	return tq.totalSize
}

// getTenantQueueLocked lazily creates and returns a tenant queue
func (tq *taskQueue) getTenantQueueLocked(tenant string) *taskHeap {
	q, ok := tq.tenantQueues[tenant]
	if !ok {
		q = &taskHeap{}
		heap.Init(q)
		tq.tenantQueues[tenant] = q
		tq.tenantOrder = append(tq.tenantOrder, tenant)
	}
	return q
}

func (tq *taskQueue) removeTenantQueueLocked(tenant string) {
	delete(tq.tenantQueues, tenant)
	for i, current := range tq.tenantOrder {
		if current != tenant {
			continue
		}
		tq.tenantOrder = append(tq.tenantOrder[:i], tq.tenantOrder[i+1:]...)
		if len(tq.tenantOrder) == 0 {
			tq.currentTenant = 0
		} else if tq.currentTenant >= len(tq.tenantOrder) {
			tq.currentTenant %= len(tq.tenantOrder)
		}
		return
	}
}

// signal notifies waiters about queue changes
func (tq *taskQueue) signal() {
	tq.cond.Signal()
}

func (tq *taskQueue) reset() {
	tq.mu.Lock()
	defer tq.mu.Unlock()

	tq.queue.Reset()
	tq.tenantQueues = make(map[string]*taskHeap)
	tq.tenantOrder = nil
	tq.currentTenant = 0
	tq.totalSize = 0
	tq.cond.Broadcast()
}
