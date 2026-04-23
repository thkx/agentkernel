package policy

import (
	"testing"

	"github.com/thkx/agentkernel/capability"
	llmplugin "github.com/thkx/agentkernel/plugins/llm"
	toolplugin "github.com/thkx/agentkernel/plugins/tool"
	"github.com/thkx/agentkernel/types"
)

func TestPolicyValidator_ValidateGraph(t *testing.T) {
	registry := capability.NewRegistry()
	llm := &llmplugin.LLM{}
	tool := &toolplugin.Tool{}
	registry.Register(llm)
	registry.Register(tool)

	validator := NewPolicyValidator(registry)

	// Create a valid graph
	builder := NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("hello").
		NextNode("n2").Build().Node("n2").
		Capability("tool").
		Input("process").
		Build()

	graph, err := builder.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	result := validator.ValidateGraph(graph, registry)
	if !result.Valid {
		t.Errorf("expected valid graph, got errors: %v", result.Errors)
	}

	// Test invalid capability
	builder2 := NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("invalid").
		Input("test").
		Build()

	graph2, err := builder2.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	result2 := validator.ValidateGraph(graph2, registry)
	if result2.Valid {
		t.Error("expected invalid graph due to unknown capability")
	}
}

func TestPolicyValidator_ValidateStartNode(t *testing.T) {
	registry := capability.NewRegistry()
	validator := NewPolicyValidator(registry)

	// Graph with missing start node
	nodes := map[types.NodeID]*types.Node[any]{
		"n1": {ID: "n1", Capability: "llm"},
	}

	graph := types.NewGraph[any]("missing", nodes)
	result := validator.ValidateGraph(graph, registry)

	if result.Valid {
		t.Error("expected invalid graph due to missing start node")
	}
}
