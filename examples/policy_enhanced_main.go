package main

import (
	"fmt"
	"time"

	"github.com/thkx/agentkernel/capability"
	llmplugin "github.com/thkx/agentkernel/plugins/llm"
	toolplugin "github.com/thkx/agentkernel/plugins/tool"
	"github.com/thkx/agentkernel/policy"
	"github.com/thkx/agentkernel/runtime"
	"github.com/thkx/agentkernel/types"
)

// Example demonstrates the enhanced Policy features:
// 1. Config-driven DAG from JSON files
// 2. Fluent API for dynamic DAG building
// 3. Policy validation
// 4. Runtime DAG modification
func mainEnhanced() {
	fmt.Println("=== AgentKernel Policy Enhancement Demo ===")

	// Setup capability registry
	registry := capability.NewRegistry()
	registry.Load(&llmplugin.Plugin{})
	registry.Load(&toolplugin.Plugin{})

	// Example 1: Load policy from JSON config file
	fmt.Println("Example 1: Loading policy from JSON config")
	fmt.Println("-------------------------------------------")
	loadFromConfigExample(registry)

	time.Sleep(1 * time.Second)

	// Example 2: Build policy dynamically using fluent API
	fmt.Println("\n\nExample 2: Building policy dynamically with fluent API")
	fmt.Println("------------------------------------------------------")
	fluentAPIExample(registry)

	time.Sleep(1 * time.Second)

	// Example 3: Validate and modify policy at runtime
	fmt.Println("\n\nExample 3: Validating and modifying policy at runtime")
	fmt.Println("-----------------------------------------------------")
	runtimeModificationExample(registry)

	// Example 4: Evaluate conditions
	fmt.Println("\n\nExample 4: Condition evaluation")
	fmt.Println("------------------------------")
	conditionEvaluationExample()
}

// Example 1: Load policy from JSON config file
func loadFromConfigExample(registry *capability.Registry) {
	loader := policy.NewPolicyLoader()

	// Load policy config from JSON file
	config, err := loader.LoadFromJSON("examples/policy_config.json")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		fmt.Println("(This is expected if running from a different directory)")
		// Use inline JSON instead
		jsonStr := `{
			"version": "1.0",
			"name": "simple-workflow",
			"description": "A simple workflow",
			"start": "step1",
			"nodes": {
				"step1": {
					"id": "step1",
					"capability": "llm",
					"input": "Analyze request",
					"edges": [
						{"to": "step2", "condition": "status == 'SUCCESS'"}
					]
				},
				"step2": {
					"id": "step2",
					"capability": "tool",
					"input": "Execute task",
					"edges": []
				}
			}
		}`
		config, err = loader.LoadFromJSONString(jsonStr)
		if err != nil {
			fmt.Printf("Error loading from string: %v\n", err)
			return
		}
	}

	fmt.Printf("✓ Loaded policy: %s\n", config.Name)
	fmt.Printf("  Description: %s\n", config.Description)
	fmt.Printf("  Start node: %s\n", config.Start)
	fmt.Printf("  Node count: %d\n", len(config.Nodes))

	// Validate the policy
	validator := policy.NewPolicyValidator(registry)
	validation := validator.ValidateConfig(config, registry)

	if validation.Valid {
		fmt.Println("✓ Policy validation passed")
	} else {
		fmt.Println("✗ Policy validation failed:")
		for _, err := range validation.Errors {
			fmt.Printf("  - [%s] %s (node: %s)\n", err.Level, err.Message, err.NodeID)
		}
	}

	builder, err := policy.FromConfig(config)
	if err != nil {
		fmt.Printf("FromConfig failed: %v", err)
	}

	// Build and execute
	graph, err := builder.BuildGraph()
	if err != nil {
		fmt.Printf("BuildGraph failed: %v\n", err)
		return
	}
	rt := runtime.NewRuntime(
		runtime.WithGraph(graph),
		runtime.WithCapabilityRegistry(registry),
		runtime.WithBeforeHook(func(ctx types.HookContext) {
			fmt.Printf("  [EXEC] Node: %s, Capability: %s\n", ctx.NodeID, ctx.Capability)
		}),
	)

	rt.Run()
	time.Sleep(500 * time.Millisecond)
}

// Example 2: Build policy dynamically using fluent API
func fluentAPIExample(registry *capability.Registry) {
	builder := policy.NewPolicyBuilder().
		Start("request_parser").
		Node("request_parser").
		Capability("llm").
		Input("Parse user request").
		Priority(10).
		NextNode("decide_action").
		Build().
		Node("decide_action").
		Capability("tool").
		Input("Decide on action").
		NextConditional("execute_search", func(r types.Result) bool {
			return r.Status == types.SUCCESS
		}).
		NextConditional("handle_error", func(r types.Result) bool {
			return r.Status == types.FAILED
		}).
		Build().
		Node("execute_search").
		Capability("tool").
		Input("Execute search").
		NextNode("format_result").
		Build().
		Node("handle_error").
		Capability("tool").
		Input("Handle error gracefully").
		NextNode("format_result").
		Build().
		Node("format_result").
		Capability("llm").
		Input("Format final result").
		Build()

	fmt.Printf("✓ Built policy with %d nodes\n", len(builder.ListNodes()))
	fmt.Printf("  Nodes: %v\n", builder.ListNodes())

	// Validate
	planner := policy.NewBuilderPlanner(builder)
	validation := planner.Validate(registry)
	if validation.Valid {
		fmt.Println("✓ Policy validation passed")
	} else {
		fmt.Println("✗ Validation errors:")
		for _, err := range validation.Errors {
			fmt.Printf("  - %s\n", err.Message)
		}
	}

	// Execute
	graph, err := builder.BuildGraph()
	if err != nil {
		fmt.Printf("BuildGraph failed: %v\n", err)
		return
	}
	rt := runtime.NewRuntime(
		runtime.WithGraph(graph),
		runtime.WithCapabilityRegistry(registry),
		runtime.WithBeforeHook(func(ctx types.HookContext) {
			fmt.Printf("  [EXEC] Node: %s, Capability: %s\n", ctx.NodeID, ctx.Capability)
		}),
	)

	rt.Run()
	time.Sleep(500 * time.Millisecond)
}

// Example 3: Validate and modify policy at runtime
func runtimeModificationExample(registry *capability.Registry) {
	builder := policy.NewPolicyBuilder().
		Start("step1").
		Node("step1").
		Capability("llm").
		Input("Step 1").
		NextNode("step2").
		Build().
		Node("step2").
		Capability("tool").
		Input("Step 2").
		NextNode("step3").
		Build().
		Node("step3").
		Capability("tool").
		Input("Step 3").
		Build()

	fmt.Println("Initial policy:")
	fmt.Printf("  Nodes: %v\n", builder.ListNodes())

	// Clone a node
	fmt.Println("\n✓ Cloning step2 as step2_retry")
	_, err := builder.CloneNode("step2", "step2_retry")
	if err != nil {
		fmt.Printf("  CloneNode failed: %v\n", err)
		return
	}
	fmt.Printf("  Nodes: %v\n", builder.ListNodes())

	// Insert a new node between step1 and step2
	fmt.Println("\n✓ Inserting validation node between step1 and step2")
	err = builder.InsertNode("step1", "validate", "step2", "tool", "Validate input")
	if err == nil {
		fmt.Printf("  Nodes: %v\n", builder.ListNodes())
	} else {
		fmt.Printf("  Error: %v\n", err)
	}

	// Update node input
	fmt.Println("\n✓ Updating step2 input")
	err = builder.UpdateNodeInput("step2", "Step 2 - Updated Input")
	if err == nil {
		node, _ := builder.GetNode("step2")
		fmt.Printf("  Updated input: %v\n", node.Input)
	}

	// Validate modified policy
	planner := policy.NewBuilderPlanner(builder)
	validation := planner.Validate(registry)
	fmt.Printf("\n✓ Validation after modifications: %s\n", validation.GetValidationSummary())
}

// Example 4: Condition evaluation
func conditionEvaluationExample() {
	evaluator := policy.NewConditionEvaluator()

	testCases := []struct {
		condition string
		result    types.Result
		expected  bool
	}{
		{
			condition: "status == 'SUCCESS'",
			result:    types.Result{Status: types.SUCCESS},
			expected:  true,
		},
		{
			condition: "status == 'FAILED'",
			result:    types.Result{Status: types.FAILED},
			expected:  true,
		},
		{
			condition: "output.contains('yes')",
			result:    types.Result{Output: "yes, proceed"},
			expected:  true,
		},
		{
			condition: "attempt > 2",
			result:    types.Result{Attempt: 3},
			expected:  true,
		},
		{
			condition: "attempt <= 2",
			result:    types.Result{Attempt: 2},
			expected:  true,
		},
	}

	fmt.Println("Testing condition evaluation:")
	for _, tc := range testCases {
		condFunc, err := evaluator.BuildCondition(tc.condition)
		if err != nil {
			fmt.Printf("  ✗ Error: %v\n", err)
			continue
		}

		result := condFunc(tc.result)
		status := "✓"
		if result != tc.expected {
			status = "✗"
		}

		fmt.Printf("  %s Condition: %q → %v (expected: %v)\n",
			status, tc.condition, result, tc.expected)
	}
}
