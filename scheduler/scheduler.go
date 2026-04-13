package scheduler

import (
	"log"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/event"
	"github.com/thkx/agentkernel/types"
)

type Scheduler struct {
	engine *GraphEngine
	caps   map[string]capability.Capability
	event  *event.Bus
}

func New(engine *GraphEngine, bus *event.Bus) *Scheduler {
	return &Scheduler{
		engine: engine,
		caps:   map[string]capability.Capability{},
		event:  bus,
	}
}

func (s *Scheduler) Register(cap capability.Capability) {
	s.caps[cap.Name()] = cap
}

func (s *Scheduler) Run(start types.NodeID, state map[string]any) {

	current := start

	for {

		node := s.engine.GetNode(current)

		ctx := types.ExecContext{
			NodeID: node.ID,
			State:  state,
		}

		cap, ok := s.caps[node.Capability]
		if !ok {
			log.Printf("capability not found: %s", node.Capability)
			return
		}

		output, err := cap.Invoke(ctx, node.Input)

		result := types.Result{
			Output: output,
			Error:  err,
			Status: "SUCCESS",
		}

		s.event.Publish(event.Event{
			NodeID: string(node.ID),
			Result: result,
		})

		// ===== CONTROL FLOW =====
		switch result.Control {

		case types.NEXT:
			if len(node.Next) > 0 {
				current = node.Next[0].To
				continue
			}
			return

		case types.JUMP:
			current = result.NextNode
			continue

		case types.RETRY:
			continue

		case types.STOP:
			return
		}
	}
}
