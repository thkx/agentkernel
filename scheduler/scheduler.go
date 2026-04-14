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
	engine *GraphEngine[any]
	caps   *capability.Registry
	event  *event.SourcingBus
}

func New(engine *GraphEngine[any], bus *event.SourcingBus, caps *capability.Registry) *Scheduler {
	return &Scheduler{
		engine: engine,
		caps:   caps,
		event:  bus,
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

	// Build dependency map: node -> list of nodes that depend on it (outgoing)
	dependencies := s.buildDependencies(graph)

	// Completion channels
	completions := make(map[types.NodeID]chan struct{})
	for id := range graph.Nodes {
		completions[id] = make(chan struct{})
	}

	var wg sync.WaitGroup

	// Worker pool
	workerPool := worker.NewPool(10) // max 10 concurrent workers
	defer workerPool.Close()

	// Function to execute a node
	executeNode := func(nodeID types.NodeID) {
		defer wg.Done()

		node := s.engine.GetNode(nodeID)
		if node == nil {
			log.Printf("node not found: %s", nodeID)
			return
		}

		// Wait for dependencies (incoming edges)
		for _, dep := range dependencies[nodeID] {
			<-completions[dep]
		}

		// Submit to worker pool
		workerPool.Submit(func() {
			ctx := types.ExecContext{
				NodeID: nodeID,
				State:  state,
			}

			cap, ok := s.caps.Get(node.Capability)
			if !ok {
				log.Printf("capability not found: %s", node.Capability)
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
				// Retry logic
				s.retryNode(nodeID, node, ctx, cap, &result)
			}

			s.event.Publish(event.Event{
				NodeID: string(nodeID),
				Result: result,
			})

			// Signal completion
			close(completions[nodeID])
		})
	}

	// Start all nodes (they will wait for dependencies)
	for id := range graph.Nodes {
		wg.Add(1)
		go executeNode(id)
	}

	wg.Wait()
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
func (s *Scheduler) retryNode(nodeID types.NodeID, node *types.Node[any], ctx types.ExecContext, cap capability.Capability, result *types.Result) {
	policy := &retry.RetryPolicy{}
	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Printf("Retrying node %s, attempt %d", nodeID, attempt)
		time.Sleep(policy.Next(attempt))

		output, err := cap.Invoke(ctx, node.Input)
		if err == nil {
			result.Output = output
			result.Error = nil
			result.Status = types.SUCCESS
			return
		}
	}
	// After max retries, keep failed status
}
