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

	// Build a simple graph
	builder := policy.NewPolicyBuilder().
		Start("n1").
		Node("n1").
			Capability("llm").
			Input("hello world").
			NextNode("n2").
			Build().
		Node("n2").
			Capability("tool").
			Input("process result").
			Build()

	graph := builder.BuildGraph()

	rt := runtime.NewRuntime(
		runtime.WithGraph(graph),
		runtime.WithCapabilityRegistry(registry),
		runtime.WithBeforeHook(func(ctx types.HookContext) {
			fmt.Printf("HOOK before node=%s trace=%s span=%s snapshot_keys=%d\n", ctx.NodeID, ctx.TraceID, ctx.SpanID, len(ctx.SnapshotState()))
		}),
		runtime.WithAfterWritableHook(func(ctx types.WritableHookContext) {
			ctx.SetState("last_node", string(ctx.NodeID))
			fmt.Printf("HOOK after node=%s trace=%s span=%s status=%s duration=%v\n", ctx.NodeID, ctx.TraceID, ctx.SpanID, ctx.Result.Status, ctx.Result.Output)
		}),
	)

	// Subscribe to events for feedback
	sub := rt.Bus().Subscribe()
	go func() {
		for ev := range sub {
			result := ev.Result.(types.Result)
			fmt.Printf("Event: NodeID=%s trace=%s span=%s Output=%v Status=%s\n", ev.NodeID, ev.TraceID, ev.SpanID, result.Output, result.Status)
		}
	}()

	rt.Run()

	time.Sleep(2 * time.Second)

	fmt.Println("\n--- Replaying Events ---")
	rt.Bus().Replay(func(ev types.Event) {
		result := ev.Result.(types.Result)
		fmt.Printf("Replay: NodeID=%s trace=%s span=%s Output=%v Status=%s\n", ev.NodeID, ev.TraceID, ev.SpanID, result.Output, result.Status)
	})

	fmt.Println("\n--- State Snapshot ---")
	snapshot := rt.Bus().Store().Snapshot()
	for k, v := range snapshot {
		fmt.Printf("Snapshot: %s = %v\n", k, v)
	}

	fmt.Println("\n--- Execution Timeline ---")
	for _, entry := range rt.Bus().Store().Timeline() {
		fmt.Printf("Timeline: node=%s trace=%s span=%s status=%s duration=%v attempt=%d error=%s\n", entry.NodeID, entry.TraceID, entry.SpanID, entry.Status, entry.Duration, entry.Attempt, entry.Error)
	}

	fmt.Println("\n--- Metrics ---")
	metrics := rt.Scheduler().Metrics()
	fmt.Printf("QPS=%.2f avg_latency=%v fastest=%v slowest=%v total=%d\n", metrics.QPS(), metrics.AvgLatency(), metrics.Fastest(), metrics.Slowest(), metrics.Total())
}
