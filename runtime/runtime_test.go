package runtime

import (
	"testing"

	"github.com/thkx/agentkernel/capability"
	llmplugin "github.com/thkx/agentkernel/plugins/llm"
	"github.com/thkx/agentkernel/policy"
)

func TestRuntime_RunSync(t *testing.T) {
	// Create a simple graph
	builder := policy.NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("hello").
		Build()

	graph, err := builder.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	// Create registry
	registry := capability.NewRegistry()
	llm := &llmplugin.LLM{}
	registry.Register(llm)

	// Create runtime
	rt := NewRuntime(
		WithGraph(graph),
		WithCapabilityRegistry(registry),
	)

	// Run synchronously
	rt.RunSync()

	// For now, we can't easily check the result since RunSync doesn't return it
	// This test mainly checks that it doesn't panic
}

func TestRuntime_RunSyncWithInvalidGraph(t *testing.T) {
	// Create runtime without graph
	rt := NewRuntime()

	// This should not panic, but we can't easily test the error
	rt.RunSync()
}

func TestRuntime_RunSyncWithInvalidCapability(t *testing.T) {
	// Create graph with invalid capability
	builder := policy.NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("invalid").
		Input("test").
		Build()

	graph, err := builder.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	registry := capability.NewRegistry()

	rt := NewRuntime(
		WithGraph(graph),
		WithCapabilityRegistry(registry),
	)

	// This should not panic, but we can't easily test the error
	rt.RunSync()
}
