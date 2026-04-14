package scheduler

import (
	"log"
	"sync"
	"time"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/event"
	"github.com/thkx/agentkernel/retry"
	"github.com/thkx/agentkernel/types"
	"github.com/thkx/agentkernel/worker"
)

type Scheduler struct {
	engine     *GraphEngine[any]
	caps       *capability.Registry
	event      *event.SourcingBus
	taskStates map[types.NodeID]types.TaskStatus
	executed   map[string]bool // Idempotency keys
	dlq        *DeadLetterQueue
	retryPol   *retry.RetryPolicy
}

func New(engine *GraphEngine[any], bus *event.SourcingBus, caps *capability.Registry) *Scheduler {
	return &Scheduler{
		engine:     engine,
		caps:       caps,
		event:      bus,
		taskStates: make(map[types.NodeID]types.TaskStatus),
		executed:   make(map[string]bool),
		dlq:        NewDeadLetterQueue(),
		retryPol:   retry.NewRetryPolicy(3, 100*time.Millisecond),
	}
}

func (s *Scheduler) Register(cap capability.Capability) {
	s.caps.Register(cap)
}

func (s *Scheduler) Run(start types.NodeID, state map[string]any) {
	graph := s.engine.graph
	if graph == nil {
		log.Println("graph is nil")
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
	workerPool := worker.NewPool(10)
	defer workerPool.Close()

	executeNode := func(nodeID types.NodeID) {
		defer wg.Done()

		node := s.engine.GetNode(nodeID)
		if node == nil {
			log.Printf("node not found: %s", nodeID)
			return
		}

		// Idempotency check
		key := string(nodeID) + ":" + string(node.Capability)
		if s.executed[key] {
			log.Printf("node %s already executed, skipping", nodeID)
			close(completions[nodeID])
			return
		}

		// Set status to Running
		s.taskStates[nodeID] = types.Running

		// Wait for dependencies
		for _, dep := range dependencies[nodeID] {
			<-completions[dep]
		}

		workerPool.Submit(func() {
			ctx := types.ExecContext{
				NodeID: nodeID,
				State:  state,
			}

			cap, ok := s.caps.Get(node.Capability)
			if !ok {
				log.Printf("capability not found: %s", node.Capability)
				s.taskStates[nodeID] = types.Failed
				close(completions[nodeID])
				return
			}

			output, err := cap.Invoke(ctx, node.Input)

			result := types.Result{
				Output:  output,
				Error:   err,
				Status:  types.SUCCESS,
				Control: types.NEXT,
			}

			if err != nil {
				result.Status = types.FAILED
				// Retry
				if s.retryNode(nodeID, node, ctx, cap, &result) {
					s.taskStates[nodeID] = types.Done
					s.executed[key] = true
				} else {
					s.taskStates[nodeID] = types.Failed
					// To DLQ
					task := types.Task{
						ID:             string(nodeID),
						Type:           string(node.Capability),
						Input:          node.Input,
						Attempt:        3,
						Status:         types.Failed,
						IdempotencyKey: key,
						NodeID:         nodeID,
					}
					s.dlq.Enqueue(task)
				}
			} else {
				s.taskStates[nodeID] = types.Done
				s.executed[key] = true
			}

			s.event.Publish(event.Event{
				NodeID: string(nodeID),
				Result: result,
			})

			close(completions[nodeID])
		})
	}

	for id := range graph.Nodes {
		wg.Add(1)
		go executeNode(id)
	}

	wg.Wait()
}

// recoverState replays events to restore task states
func (s *Scheduler) recoverState() {
	s.event.Replay(func(ev event.Event) {
		if result, ok := ev.Result.(types.Result); ok {
			nodeID := types.NodeID(ev.NodeID)
			if result.Status == types.SUCCESS {
				s.taskStates[nodeID] = types.Done
				key := ev.NodeID + ":" + string(s.engine.GetNode(nodeID).Capability)
				s.executed[key] = true
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
func (s *Scheduler) retryNode(nodeID types.NodeID, node *types.Node[any], ctx types.ExecContext, cap capability.Capability, result *types.Result) bool {
	for attempt := 1; s.retryPol.ShouldRetry(attempt); attempt++ {
		log.Printf("Retrying node %s, attempt %d", nodeID, attempt)
		time.Sleep(s.retryPol.Next(attempt))

		output, err := cap.Invoke(ctx, node.Input)
		if err == nil {
			result.Output = output
			result.Error = nil
			result.Status = types.SUCCESS
			return true
		}
	}
	return false
}
