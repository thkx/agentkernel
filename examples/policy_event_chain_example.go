package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/event"
	examplesllm "github.com/thkx/agentkernel/examples/llm"
	examplestool "github.com/thkx/agentkernel/examples/tool"
	"github.com/thkx/agentkernel/policy"
	"github.com/thkx/agentkernel/runtime"
	"github.com/thkx/agentkernel/types"
)

// demonstratePolicyEventChain showcases the complete event chain:
// policy.config_loaded → policy.graph_built → policy.validated →
// policy.registered → policy.selected → policy.evaluated →
// RUNTIME lifecycle → EXECUTION lifecycle
func demonstratePolicyEventChain() {
	fmt.Println("=== Policy Event Chain Integration Demo ===\n")

	// Setup
	registry := capability.NewRegistry()
	registry.Register(&examplesllm.LLM{})
	registry.Register(&examplestool.Tool{})

	// Create a shared event bus for the entire flow
	eventStore := event.NewInMemoryEventStore()
	bus := event.NewSourcingBus(eventStore)

	// Step 1: Load policy configuration
	fmt.Println("Step 1: Loading policy configuration...")
	fmt.Println("----------------------------------------")

	loader := policy.NewPolicyLoader().WithBus(bus)
	configJSON := `{
		"version": "1.0",
		"name": "event-chain-demo",
		"description": "Demonstrates complete event chain",
		"start": "analyze",
		"nodes": {
			"analyze": {
				"id": "analyze",
				"capability": "llm",
				"input": "Analyze the request",
				"outputVar": "analysis",
				"edges": [
					{"to": "execute", "condition": "analysis != null"}
				]
			},
			"execute": {
				"id": "execute",
				"capability": "tool",
				"input": "Execute based on analysis",
				"edges": []
			}
		}
	}`

	config, err := loader.LoadFromJSONString(configJSON)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	fmt.Printf("✓ Policy config loaded: %s\n\n", config.Name)

	// Step 2: Build the policy graph
	fmt.Println("Step 2: Building policy graph...")
	fmt.Println("--------------------------------")

	builder, _ := policy.FromConfig(config)
	builderPlanner := policy.NewBuilderPlanner(builder).WithBus(bus)

	graph, err := builderPlanner.Build(nil)
	if err != nil {
		log.Fatalf("Failed to build: %v", err)
	}
	fmt.Printf("✓ Policy graph built with %d nodes\n\n", len(graph.Nodes))

	// Step 3: Validate the policy
	fmt.Println("Step 3: Validating policy...")
	fmt.Println("----------------------------")

	validator := policy.NewPolicyValidator(registry).WithBus(bus)
	validationResult := validator.ValidateGraph(graph, registry)
	if !validationResult.Valid {
		log.Fatalf("Policy validation failed")
	}
	fmt.Printf("✓ Policy validation passed\n\n")

	// Step 4: Create policies and register with engine
	fmt.Println("Step 4: Registering policies with engine...")
	fmt.Println("------------------------------------------")

	engine := policy.NewPolicyEngine().WithBus(bus)
	policyObj := policy.NewConfigDrivenPolicy("event-chain-demo", config)

	if err := engine.Register(policyObj); err != nil {
		log.Fatalf("Failed to register policy: %v", err)
	}
	fmt.Printf("✓ Policy registered\n\n")

	// Step 5: Select and evaluate policy
	fmt.Println("Step 5: Selecting and evaluating policy...")
	fmt.Println("-----------------------------------------")

	ctx := context.Background()
	planCtx := &policy.PlanContext{
		Variables: map[string]any{"user_input": "test request"},
	}

	selectedPlan, err := engine.Select(ctx, "event-chain-demo", planCtx)
	if err != nil {
		log.Fatalf("Failed to select policy: %v", err)
	}
	fmt.Printf("✓ Policy selected and evaluated: %s\n\n", selectedPlan.PolicyName)

	// Step 6: Create runtime with the selected plan and same event bus
	fmt.Println("Step 6: Executing with Runtime...")
	fmt.Println("--------------------------------")

	rt, err := runtime.NewRuntime(
		runtime.WithGraph(selectedPlan.Graph),
		runtime.WithCapabilityRegistry(registry),
		runtime.WithBus(bus),
	)
	if err != nil {
		log.Fatalf("Failed to create runtime: %v", err)
	}

	if err := rt.Run(); err != nil {
		log.Fatalf("Runtime execution failed: %v", err)
	}
	fmt.Printf("✓ Runtime executed successfully\n\n")

	// Step 7: Replay all events to show the complete chain
	fmt.Println("Step 7: Complete Event Chain Replay")
	fmt.Println("===================================\n")

	eventTimings := make(map[string]time.Time)

	bus.Replay(func(ev types.Event) {
		eventTimings[ev.Name] = ev.Timestamp

		switch ev.Kind {
		case types.EventKindPolicy:
			payload := ev.Result.(types.PolicyEvent)
			fmt.Printf("[POLICY] %s\n", ev.Name)
			fmt.Printf("  → PolicyName: %s\n", payload.PolicyName)
			fmt.Printf("  → Source: %s\n", payload.Source)
			fmt.Printf("  → Valid: %v\n", payload.Valid)
			fmt.Printf("  → Nodes: %d\n", payload.NodeCount)
			if payload.Error != "" {
				fmt.Printf("  → Error: %s\n", payload.Error)
			}
			fmt.Printf("  → Timestamp: %s\n\n", ev.Timestamp.Format("15:04:05.000"))

		case types.EventKindRuntime:
			lifecycle := ev.Result.(types.RuntimeLifecycleEvent)
			fmt.Printf("[RUNTIME] %s\n", ev.Name)
			fmt.Printf("  → Status: %s\n", lifecycle.Status)
			if lifecycle.Previous != "" {
				fmt.Printf("  → Previous: %s\n", lifecycle.Previous)
			}
			if lifecycle.Error != "" {
				fmt.Printf("  → Error: %s\n", lifecycle.Error)
			}
			fmt.Printf("  → Timestamp: %s\n\n", ev.Timestamp.Format("15:04:05.000"))

		case types.EventKindExecution:
			execEvent := ev.Result.(types.ExecutionEvent)
			fmt.Printf("[EXECUTION] %s\n", ev.Name)
			fmt.Printf("  → Node: %s\n", execEvent.NodeID)
			fmt.Printf("  → Capability: %s\n", execEvent.Capability)
			fmt.Printf("  → Status: %s\n", execEvent.Status)
			fmt.Printf("  → Attempt: %d\n", execEvent.Attempt)
			fmt.Printf("  → Duration: %v\n", execEvent.Duration)
			if execEvent.Error != "" {
				fmt.Printf("  → Error: %s\n", execEvent.Error)
			}
			if execEvent.Result != nil && execEvent.Result.Output != nil {
				fmt.Printf("  → Output: %v\n", execEvent.Result.Output)
			}
			fmt.Printf("  → Timestamp: %s\n\n", ev.Timestamp.Format("15:04:05.000"))
		}
	})

	// Summary
	fmt.Println("\n=== Event Chain Summary ===\n")
	eventSequence := []string{
		"policy.config_loaded",
		"policy.graph_built",
		"policy.validated",
		"policy.registered",
		"policy.selected",
		"policy.evaluated",
	}

	fmt.Println("Policy Events:")
	for _, name := range eventSequence {
		if t, ok := eventTimings[name]; ok {
			fmt.Printf("  ✓ %s @ %s\n", name, t.Format("15:04:05.000"))
		}
	}

	fmt.Println("\nRuntime & Execution Events recorded:")
	fmt.Printf("  Total events: %d\n", len(eventStore.Events()))

	// Statistics
	eventCounts := make(map[string]int)
	for _, ev := range eventStore.Events() {
		eventCounts[string(ev.Kind)]++
	}

	fmt.Printf("\nEvent breakdown:\n")
	for kind, count := range eventCounts {
		fmt.Printf("  %s: %d\n", kind, count)
	}

	fmt.Println("\n✓ Complete event chain successfully demonstrated!")
	fmt.Println("  From policy design → runtime execution with full observability")
}
