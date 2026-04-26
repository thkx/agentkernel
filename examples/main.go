package main

import (
	"fmt"
	"log"
	"time"

	"github.com/thkx/agentkernel/capability"
	examplesllm "github.com/thkx/agentkernel/examples/llm"
	examplestool "github.com/thkx/agentkernel/examples/tool"
	"github.com/thkx/agentkernel/policy"
	"github.com/thkx/agentkernel/runtime"
	"github.com/thkx/agentkernel/types"
)

func main() {
	registry := capability.NewRegistry()
	registry.Register(&examplesllm.LLM{})
	registry.Register(&examplestool.Tool{})

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

	graph, err := builder.BuildGraph()
	if err != nil {
		log.Fatalf("BuildGraph failed: %v", err)
	}

	rt, err := runtime.NewRuntime(
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
	if err != nil {
		log.Fatalf("NewRuntime failed: %v", err)
	}

	// Subscribe to events for feedback
	sub := rt.Bus().Subscribe()
	go func() {
		for ev := range sub {
			switch ev.Kind {
			case types.EventKindRuntime:
				lifecycle := ev.Result.(types.RuntimeLifecycleEvent)
				fmt.Printf("RuntimeEvent: name=%s status=%s previous=%s error=%s at=%s\n", ev.Name, lifecycle.Status, lifecycle.Previous, lifecycle.Error, ev.Timestamp.Format(time.RFC3339Nano))
			default:
				payload := ev.Result.(types.ExecutionEvent)
				if payload.Result == nil {
					fmt.Printf("ExecutionEvent: name=%s node=%s capability=%s trace=%s span=%s at=%s\n", ev.Name, payload.NodeID, payload.Capability, payload.TraceID, payload.SpanID, ev.Timestamp.Format(time.RFC3339Nano))
					continue
				}
				fmt.Printf("ExecutionEvent: name=%s node=%s capability=%s trace=%s span=%s output=%v status=%s attempt=%d duration=%s error=%s at=%s\n", ev.Name, payload.NodeID, payload.Capability, payload.TraceID, payload.SpanID, payload.Result.Output, payload.Status, payload.Attempt, payload.Duration, payload.Error, ev.Timestamp.Format(time.RFC3339Nano))
			}
		}
	}()

	fmt.Printf("Runtime status before run: %s\n", rt.Status())
	if err := rt.Run(); err != nil {
		log.Fatalf("Run failed: %v", err)
	}

	time.Sleep(2 * time.Second)

	fmt.Printf("Runtime health after run: %+v\n", rt.Health())

	fmt.Println("\n--- Replaying Events ---")
	rt.Bus().Replay(func(ev types.Event) {
		switch ev.Kind {
		case types.EventKindRuntime:
			lifecycle := ev.Result.(types.RuntimeLifecycleEvent)
			fmt.Printf("ReplayRuntime: name=%s status=%s previous=%s error=%s at=%s\n", ev.Name, lifecycle.Status, lifecycle.Previous, lifecycle.Error, ev.Timestamp.Format(time.RFC3339Nano))
		default:
			payload := ev.Result.(types.ExecutionEvent)
			if payload.Result == nil {
				fmt.Printf("ReplayExecution: name=%s node=%s capability=%s trace=%s span=%s at=%s\n", ev.Name, payload.NodeID, payload.Capability, payload.TraceID, payload.SpanID, ev.Timestamp.Format(time.RFC3339Nano))
				return
			}
			fmt.Printf("ReplayExecution: name=%s node=%s capability=%s trace=%s span=%s output=%v status=%s attempt=%d duration=%s error=%s at=%s\n", ev.Name, payload.NodeID, payload.Capability, payload.TraceID, payload.SpanID, payload.Result.Output, payload.Status, payload.Attempt, payload.Duration, payload.Error, ev.Timestamp.Format(time.RFC3339Nano))
		}
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
