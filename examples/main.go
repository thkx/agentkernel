package main

import (
	"fmt"
	"time"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/event"
	"github.com/thkx/agentkernel/policy"
	"github.com/thkx/agentkernel/runtime"
	"github.com/thkx/agentkernel/types"
)

func main() {
	registry := capability.NewRegistry()
	registry.Load(&capability.LLMPlugin{})
	registry.Load(&capability.ToolPlugin{})

	rt := runtime.NewRuntime(
		runtime.WithPlanner(&policy.Planner{}),
		runtime.WithCapabilityRegistry(registry),
	)

	// Subscribe to events for feedback
	sub := rt.Bus().Subscribe()
	go func() {
		for ev := range sub {
			result := ev.Result.(types.Result)
			fmt.Printf("Event: NodeID=%s, Output=%v, Status=%s\n", ev.NodeID, result.Output, result.Status)
		}
	}()

	rt.Run()

	time.Sleep(2 * time.Second)

	fmt.Println("\n--- Replaying Events ---")
	rt.Bus().Replay(func(ev event.Event) {
		result := ev.Result.(types.Result)
		fmt.Printf("Replay: NodeID=%s, Output=%v, Status=%s\n", ev.NodeID, result.Output, result.Status)
	})

	fmt.Println("\n--- State Snapshot ---")
	if store, ok := rt.Bus().Store().(*event.InMemoryEventStore); ok {
		snapshot := store.Snapshot()
		for k, v := range snapshot {
			fmt.Printf("Snapshot: %s = %v\n", k, v)
		}
	}

	fmt.Println("\n--- Dead Letter Queue ---")
	// Note: In a real implementation, access DLQ through scheduler
	// For demo, assume no failed tasks
	fmt.Printf("DLQ Size: 0\n")
}
