package scheduler

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/observability"
	"github.com/thkx/agentkernel/retry"
	"github.com/thkx/agentkernel/types"
	"github.com/thkx/agentkernel/worker"
)

type Scheduler struct {
	engine      *GraphEngine[any]
	caps        *capability.Registry
	event       types.EventBus
	taskStates  map[types.NodeID]types.TaskStatus
	executed    map[string]bool // Idempotency keys
	dlq         *DeadLetterQueue
	retryPol    *retry.RetryPolicy
	metrics     types.MetricsRecorder
	beforeHooks []types.HookFunc
	afterHooks  []types.HookFunc
	mu          sync.Mutex
}

func New(engine *GraphEngine[any], bus types.EventBus, caps *capability.Registry) *Scheduler {
	return &Scheduler{
		engine:      engine,
		caps:        caps,
		event:       bus,
		taskStates:  make(map[types.NodeID]types.TaskStatus),
		executed:    make(map[string]bool),
		dlq:         NewDeadLetterQueue(),
		retryPol:    retry.NewRetryPolicy(3, 100*time.Millisecond),
		metrics:     observability.NewMetrics(),
		beforeHooks: make([]types.HookFunc, 0),
		afterHooks:  make([]types.HookFunc, 0),
	}
}

func (s *Scheduler) Register(cap types.Capability) {
	s.caps.Register(cap)
}

func (s *Scheduler) RegisterBeforeHook(hook types.HookFunc) {
	s.beforeHooks = append(s.beforeHooks, hook)
}

func (s *Scheduler) RegisterAfterHook(hook types.HookFunc) {
	s.afterHooks = append(s.afterHooks, hook)
}

func (s *Scheduler) Metrics() types.MetricsRecorder {
	return s.metrics
}

func (s *Scheduler) DLQSize() int {
	return s.dlq.Size()
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (s *Scheduler) Run(start types.NodeID, state map[string]any) {
	graph := s.engine.graph
	if graph == nil {
		log.Println("level=error msg=graph_nil")
		return
	}

	// Crash recovery: replay events to restore state
	s.recoverState()

	dependencies := s.buildDependencies(graph)
	completions := make(map[types.NodeID]chan struct{})
	for id := range graph.Nodes {
		completions[id] = make(chan struct{})
	}

	var wg sync.WaitGroup
	var taskWg sync.WaitGroup
	workerPool := worker.NewPool(10)
	defer workerPool.Close()

	executeNode := func(nodeID types.NodeID) {
		defer wg.Done()

		node := s.engine.GetNode(nodeID)
		if node == nil {
			log.Printf("level=error msg=node_not_found node=%s", nodeID)
			return
		}

		traceID := generateID()
		spanID := generateID()
		key := string(nodeID) + ":" + string(node.Capability)

		s.mu.Lock()
		if s.executed[key] {
			s.mu.Unlock()
			log.Printf("level=info msg=skip_already_executed node=%s trace=%s span=%s", nodeID, traceID, spanID)
			close(completions[nodeID])
			return
		}
		s.taskStates[nodeID] = types.Running
		s.mu.Unlock()

		for _, dep := range dependencies[nodeID] {
			<-completions[dep]
		}

		taskWg.Add(1)
		workerPool.Submit(func() {
			defer taskWg.Done()
			startTime := time.Now()
			ctx := types.ExecContext{
				TraceID: traceID,
				SpanID:  spanID,
				NodeID:  nodeID,
				State:   state,
			}

			for _, hook := range s.beforeHooks {
				hook(types.HookContext{
					TraceID:    traceID,
					SpanID:     spanID,
					NodeID:     nodeID,
					Capability: node.Capability,
					Input:      node.Input,
					Attempt:    0,
					State:      state,
				})
			}

			cap, ok := s.caps.Get(node.Capability)
			if !ok {
				log.Printf("level=error msg=capability_not_found node=%s capability=%s trace=%s span=%s", nodeID, node.Capability, traceID, spanID)
				s.mu.Lock()
				s.taskStates[nodeID] = types.Failed
				s.mu.Unlock()
				close(completions[nodeID])
				return
			}

			output, err := cap.Invoke(ctx, node.Input)
			status := types.SUCCESS
			if err != nil {
				status = types.FAILED
			}

			result := types.Result{
				Output:  output,
				Error:   err,
				Status:  status,
				Control: types.NEXT,
			}

			attempt := 1
			if err != nil {
				var retryCount int
				var retryOK bool
				retryOK, retryCount = s.retryNode(nodeID, node, ctx, cap, &result)
				attempt = retryCount
				if retryOK {
					s.mu.Lock()
					s.taskStates[nodeID] = types.Done
					s.executed[key] = true
					s.mu.Unlock()
				} else {
					s.mu.Lock()
					s.taskStates[nodeID] = types.Failed
					s.mu.Unlock()
					task := types.Task{
						ID:             string(nodeID),
						Type:           string(node.Capability),
						Input:          node.Input,
						Attempt:        s.retryPol.MaxAttempts,
						Status:         types.Failed,
						IdempotencyKey: key,
						NodeID:         nodeID,
					}
					s.dlq.Enqueue(task)
				}
			} else {
				s.mu.Lock()
				s.taskStates[nodeID] = types.Done
				s.executed[key] = true
				s.mu.Unlock()
			}

			endTime := time.Now()
			duration := endTime.Sub(startTime)
			s.metrics.Record(duration)

			timeline := types.ExecutionTimelineEntry{
				TraceID:    traceID,
				SpanID:     spanID,
				NodeID:     nodeID,
				Capability: node.Capability,
				Status:     result.Status,
				StartTime:  startTime,
				EndTime:    endTime,
				Duration:   duration,
				Attempt:    attempt,
				Error:      fmt.Sprintf("%v", result.Error),
			}
			s.event.Store().AppendTimeline(timeline)

			s.event.Publish(types.Event{
				NodeID:  string(nodeID),
				TraceID: traceID,
				SpanID:  spanID,
				Result:  result,
			})

			for _, hook := range s.afterHooks {
				hook(types.HookContext{
					TraceID:    traceID,
					SpanID:     spanID,
					NodeID:     nodeID,
					Capability: node.Capability,
					Input:      node.Input,
					Attempt:    attempt,
					State:      state,
					Result:     result,
				})
			}

			log.Printf("level=info msg=exec_finished node=%s capability=%s trace=%s span=%s status=%s latency_ms=%d", nodeID, node.Capability, traceID, spanID, result.Status, duration.Milliseconds())
			close(completions[nodeID])
		})
	}

	for id := range graph.Nodes {
		wg.Add(1)
		go executeNode(id)
	}

	wg.Wait()
	taskWg.Wait()
}

// recoverState replays events to restore task states
func (s *Scheduler) recoverState() {
	s.event.Replay(func(ev types.Event) {
		if result, ok := ev.Result.(types.Result); ok {
			nodeID := types.NodeID(ev.NodeID)
			s.mu.Lock()
			defer s.mu.Unlock()
			if result.Status == types.SUCCESS {
				s.taskStates[nodeID] = types.Done
				if node := s.engine.GetNode(nodeID); node != nil {
					key := ev.NodeID + ":" + string(node.Capability)
					s.executed[key] = true
				}
			} else {
				s.taskStates[nodeID] = types.Failed
			}
		}
	})
}

// buildDependencies builds a map of node -> list of predecessors
func (s *Scheduler) buildDependencies(graph *types.Graph[any]) map[types.NodeID][]types.NodeID {
	deps := make(map[types.NodeID][]types.NodeID)
	for id, node := range graph.Nodes {
		for _, edge := range node.Next {
			deps[edge.To] = append(deps[edge.To], id)
		}
	}
	return deps
}

// retryNode implements retry logic
func (s *Scheduler) retryNode(nodeID types.NodeID, node *types.Node[any], ctx types.ExecContext, cap capability.Capability, result *types.Result) (bool, int) {
	for attempt := 2; s.retryPol.ShouldRetry(attempt); attempt++ {
		log.Printf("level=warn msg=retrying node=%s attempt=%d", nodeID, attempt)
		time.Sleep(s.retryPol.Next(attempt - 1))

		output, err := cap.Invoke(ctx, node.Input)
		if err == nil {
			result.Output = output
			result.Error = nil
			result.Status = types.SUCCESS
			return true, attempt
		}
		result.Error = err
	}
	return false, s.retryPol.MaxAttempts
}
