package main

import (
	"fmt"
	"time"

	"github.com/thkx/agentkernel/capability"
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
}
