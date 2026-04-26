package main

import (
	"context"
	"fmt"
	"log"

	"github.com/thkx/agentkernel/capability"
	llmplugin "github.com/thkx/agentkernel/plugins/llm"
	toolplugin "github.com/thkx/agentkernel/plugins/tool"
	"github.com/thkx/agentkernel/policy"
	"github.com/thkx/agentkernel/runtime"
	"github.com/thkx/agentkernel/types"
)

func mainRedesign() {
	// Example 1: Using PolicyEngine with Programmatic Policy
	fmt.Println("=== Example 1: Programmatic Policy ===")
	example1ProgrammaticPolicy()

	// Example 2: Using PolicyEngine with Config-Driven Policy
	fmt.Println("\n=== Example 2: Config-Driven Policy ===")
	example2ConfigPolicy()

	// Example 3: Policy Selection and Evaluation
	fmt.Println("\n=== Example 3: Policy Selection ===")
	example3PolicySelection()
}

// Example 1: Build a policy programmatically and use PolicyEngine
func example1ProgrammaticPolicy() {
	// Create a graph with PolicyBuilder
	builder := policy.NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("analyze user request").
		NextNode("n2").
		Priority(10).
		Build().
		Node("n2").
		Capability("tool").
		Input("execute action").
		Build()

	graph, err := builder.BuildGraph()
	if err != nil {
		log.Fatalf("BuildGraph failed: %v", err)
	}

	// Create a programmatic policy from the graph
	programmaticPolicy := policy.NewProgrammaticPolicy("my_workflow", graph)

	// Create engine and register policy
	engine := policy.NewPolicyEngine()
	if err := engine.Register(programmaticPolicy); err != nil {
		fmt.Printf("Error registering policy: %v\n", err)
		return
	}

	// Evaluate the policy
	ctx := context.Background()
	planCtx := &policy.PlanContext{
		Variables: map[string]any{
			"user_input": "What is the weather?",
		},
	}

	plan, err := engine.Evaluate(ctx, planCtx)
	if err != nil {
		fmt.Printf("Error evaluating policy: %v\n", err)
		return
	}

	fmt.Printf("Generated Plan:\n")
	fmt.Printf("  Policy: %s\n", plan.PolicyName)
	fmt.Printf("  Start Node: %s\n", plan.StartNode)
	fmt.Printf("  Nodes: %d\n", len(plan.Graph.Nodes))
	for nodeID, node := range plan.Graph.Nodes {
		fmt.Printf("    - %s: %s (input: %v)\n", nodeID, node.Capability, node.Input)
	}
}

// Example 2: Create a config-driven policy with YAML/JSON
func example2ConfigPolicy() {
	// Define a policy configuration
	config := &policy.PolicyConfig{
		Version:     "1.0",
		Name:        "workflow_v1",
		Description: "A complete workflow",
		Start:       "init",
		Nodes: map[string]*policy.NodeConfig{
			"init": {
				ID:         "init",
				Capability: "llm",
				Input:      "$user_input",
				OutputVar:  "decision",
				Edges: []policy.EdgeConfig{
					{
						To:        "process",
						Condition: "status == 'SUCCESS'",
						Priority:  1,
					},
					{
						To:        "error_handler",
						Condition: "status == 'FAILED'",
						Priority:  2,
					},
				},
			},
			"process": {
				ID:         "process",
				Capability: "tool",
				Input:      "process with $user_input",
			},
			"error_handler": {
				ID:         "error_handler",
				Capability: "llm",
				Input:      "handle error",
			},
		},
		Global: &policy.GlobalSettings{
			TenantAware: true,
			Variables: map[string]interface{}{
				"default_timeout": "30s",
			},
		},
	}

	// Create config-driven policy
	configPolicy := policy.NewConfigDrivenPolicy("config_workflow", config)

	// Create engine and register policy
	engine := policy.NewPolicyEngine()
	if err := engine.Register(configPolicy); err != nil {
		fmt.Printf("Error registering policy: %v\n", err)
		return
	}

	// Evaluate the policy
	ctx := context.Background()
	planCtx := &policy.PlanContext{
		Variables: map[string]any{
			"user_input": "Summarize this article",
		},
	}

	plan, err := engine.Evaluate(ctx, planCtx)
	if err != nil {
		fmt.Printf("Error evaluating policy: %v\n", err)
		return
	}

	fmt.Printf("Generated Plan from Config:\n")
	fmt.Printf("  Policy: %s\n", plan.PolicyName)
	fmt.Printf("  Start Node: %s\n", plan.StartNode)
	fmt.Printf("  Total Nodes: %d\n", len(plan.Graph.Nodes))
}

// Example 3: Policy selection and evaluation
func example3PolicySelection() {
	// Create multiple policies
	policy1 := policy.NewProgrammaticPolicy("policy_simple",
		types.NewGraph[any](
			"n1",
			map[types.NodeID]*types.Node[any]{
				"n1": {
					ID:         "n1",
					Capability: "llm",
					Input:      "simple task",
				},
			},
		),
	)

	policy2 := policy.NewProgrammaticPolicy("policy_complex",
		types.NewGraph[any](
			"n1",
			map[types.NodeID]*types.Node[any]{
				"n1": {
					ID:         "n1",
					Capability: "llm",
					Input:      "complex task",
					Next: []types.Edge{
						{To: "n2"},
					},
				},
				"n2": {
					ID:         "n2",
					Capability: "tool",
					Input:      "process result",
				},
			},
		),
	)

	// Create engine with multiple policies
	engine := policy.NewPolicyEngine()
	engine.Register(policy1)
	engine.Register(policy2)

	// Set default selector
	selector := &policy.DefaultPolicySelector{
		DefaultPolicyName: "policy_complex",
	}
	engine.SetSelector(selector)

	// Evaluate using the selector
	ctx := context.Background()
	planCtx := &policy.PlanContext{
		Variables: map[string]any{
			"task_complexity": "high",
		},
	}

	plan, err := engine.Evaluate(ctx, planCtx)
	if err != nil {
		fmt.Printf("Error evaluating: %v\n", err)
		return
	}

	fmt.Printf("Selected Policy: %s\n", plan.PolicyName)
	fmt.Printf("Graph has %d nodes\n", len(plan.Graph.Nodes))
}

// Example 4: Integration with Runtime
func example4RuntimeIntegration() {
	// Setup
	registry := capability.NewRegistry()
	registry.Load(&llmplugin.Plugin{})
	registry.Load(&toolplugin.Plugin{})

	// Create a simple policy
	builder := policy.NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("Start").
		NextNode("n2").
		Build().
		Node("n2").
		Capability("tool").
		Input("Execute").
		Build()

	graph, err := builder.BuildGraph()
	if err != nil {
		log.Fatalf("BuildGraph failed: %v", err)
	}

	// Create runtime with the policy
	rt, err := runtime.NewRuntime(
		runtime.WithGraph(graph),
		runtime.WithCapabilityRegistry(registry),
	)
	if err != nil {
		log.Fatalf("NewRuntime failed: %v", err)
	}

	// Run the workflow
	if err := rt.Run(); err != nil {
		log.Fatalf("Run failed: %v", err)
	}

	fmt.Println("Runtime integration example completed")
}
