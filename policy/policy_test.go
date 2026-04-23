package policy

import (
	"testing"
	"time"

	"github.com/thkx/agentkernel/capability"
	llmplugin "github.com/thkx/agentkernel/plugins/llm"
	toolplugin "github.com/thkx/agentkernel/plugins/tool"
	"github.com/thkx/agentkernel/types"
)

func TestPolicyBuilder_Basic(t *testing.T) {
	builder := NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("test").
		NextNode("n2").
		Build().
		Node("n2").
		Capability("tool").
		Input("execute").
		Build()

	graph, err := builder.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}
	if graph == nil {
		t.Fatal("graph should not be nil")
	}
	if graph.Start == "" {
		t.Fatal("start node should be set")
	}
	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(graph.Nodes))
	}
}

func TestPolicyBuilder_WithConditionalEdges(t *testing.T) {
	builder := NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("check").
		NextConditional("n2", func(r types.Result) bool {
			return r.Status == types.SUCCESS
		}).
		NextConditional("n3", func(r types.Result) bool {
			return r.Status == types.FAILED
		}).
		Build().
		Node("n2").
		Capability("tool").
		Input("success").
		Build().
		Node("n3").
		Capability("tool").
		Input("failure").
		Build()

	graph, err := builder.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}
	n1 := graph.Nodes[types.NodeID("n1")]
	if len(n1.Next) != 2 {
		t.Fatalf("expected 2 edges from n1, got %d", len(n1.Next))
	}
}

func TestPolicyBuilder_CloneNode(t *testing.T) {
	builder := NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("test").
		Priority(10).
		Build()

	_, err := builder.CloneNode("n1", "n1_clone")
	if err != nil {
		t.Fatalf("CloneNode failed: %v", err)
	}

	nodeOrig, _ := builder.GetNode("n1")
	nodeClone, _ := builder.GetNode("n1_clone")

	if nodeOrig.Capability != nodeClone.Capability {
		t.Fatal("cloned node should have same capability")
	}
	if nodeOrig.Priority != nodeClone.Priority {
		t.Fatal("cloned node should have same priority")
	}
	if nodeOrig.ID == nodeClone.ID {
		t.Fatal("cloned node should have different ID")
	}
}

func TestPolicyBuilder_InsertNode(t *testing.T) {
	builder := NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("start").
		NextNode("n2").
		Build().
		Node("n2").
		Capability("tool").
		Input("end").
		Build()

	err := builder.InsertNode("n1", "n_middle", "n2", "tool", "middle")
	if err != nil {
		t.Fatalf("insert should succeed: %v", err)
	}

	if len(builder.ListNodes()) != 3 {
		t.Fatalf("expected 3 nodes after insert, got %d", len(builder.ListNodes()))
	}

	n1 := builder.nodes[types.NodeID("n1")]
	if n1.Next[0].To != types.NodeID("n_middle") {
		t.Fatal("n1 should point to n_middle")
	}
}

func TestPolicyBuilder_RemoveNode(t *testing.T) {
	builder := NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("start").
		NextNode("n2").
		Build().
		Node("n2").
		Capability("tool").
		Input("end").
		Build()

	// Cannot remove start node
	err := builder.RemoveNode("n1")
	if err == nil {
		t.Fatal("removing start node should fail")
	}

	// Cannot remove node that is referenced
	err = builder.RemoveNode("n2")
	if err == nil {
		t.Fatal("removing referenced node should fail")
	}
}

func TestConditionEvaluator_SimpleComparisons(t *testing.T) {
	evaluator := NewConditionEvaluator()

	tests := []struct {
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

	for _, tc := range tests {
		condFunc, err := evaluator.BuildCondition(tc.condition)
		if err != nil {
			t.Fatalf("BuildCondition failed: %v", err)
		}

		result := condFunc(tc.result)
		if result != tc.expected {
			t.Errorf("condition %q: expected %v, got %v", tc.condition, tc.expected, result)
		}
	}
}

func TestConditionEvaluator_StringOperations(t *testing.T) {
	evaluator := NewConditionEvaluator()

	tests := []struct {
		condition string
		result    types.Result
		expected  bool
	}{
		{
			condition: "output.contains('yes')",
			result:    types.Result{Output: "yes, proceed"},
			expected:  true,
		},
		{
			condition: "output.contains('no')",
			result:    types.Result{Output: "yes, proceed"},
			expected:  false,
		},
	}

	for _, tc := range tests {
		condFunc, err := evaluator.BuildCondition(tc.condition)
		if err != nil {
			t.Fatalf("BuildCondition failed: %v", err)
		}

		result := condFunc(tc.result)
		if result != tc.expected {
			t.Errorf("condition %q: expected %v, got %v", tc.condition, tc.expected, result)
		}
	}
}

func TestPolicyLoader_ValidJSON(t *testing.T) {
	loader := NewPolicyLoader()

	jsonStr := `{
		"version": "1.0",
		"name": "test-policy",
		"start": "n1",
		"nodes": {
			"n1": {
				"id": "n1",
				"capability": "llm",
				"input": "test",
				"edges": []
			}
		}
	}`

	config, err := loader.LoadFromJSONString(jsonStr)
	if err != nil {
		t.Fatalf("LoadFromJSONString failed: %v", err)
	}

	if config.Name != "test-policy" {
		t.Fatalf("expected name 'test-policy', got %q", config.Name)
	}

	if config.Start != "n1" {
		t.Fatalf("expected start 'n1', got %q", config.Start)
	}

	if len(config.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(config.Nodes))
	}
}

func TestPolicyValidator_BasicValidation(t *testing.T) {
	registry := capability.NewRegistry()
	registry.Load(&llmplugin.Plugin{})
	registry.Load(&toolplugin.Plugin{})

	config := &PolicyConfig{
		Version: "1.0",
		Name:    "test",
		Start:   "n1",
		Nodes: map[string]*NodeConfig{
			"n1": {
				ID:         "n1",
				Capability: "llm",
				Input:      "test",
				Edges: []EdgeConfig{
					{To: "n2"},
				},
			},
			"n2": {
				ID:         "n2",
				Capability: "tool",
				Input:      "execute",
				Edges:      []EdgeConfig{},
			},
		},
	}

	validator := NewPolicyValidator(registry)
	result := validator.ValidateConfig(config, registry)

	if !result.Valid {
		t.Fatalf("validation failed: %s", result.GetValidationSummary())
	}
}

func TestPolicyValidator_MissingCapability(t *testing.T) {
	registry := capability.NewRegistry()

	config := &PolicyConfig{
		Version: "1.0",
		Name:    "test",
		Start:   "n1",
		Nodes: map[string]*NodeConfig{
			"n1": {
				ID:         "n1",
				Capability: "nonexistent",
				Input:      "test",
				Edges:      []EdgeConfig{},
			},
		},
	}

	validator := NewPolicyValidator(registry)
	result := validator.ValidateConfig(config, registry)

	if result.Valid {
		t.Fatal("validation should fail for missing capability")
	}

	hasError := false
	for _, err := range result.Errors {
		if err.Level == "error" && err.NodeID == "n1" {
			hasError = true
			break
		}
	}

	if !hasError {
		t.Fatal("expected error about missing capability")
	}
}

func TestPolicyValidator_InvalidEdgeTarget(t *testing.T) {
	registry := capability.NewRegistry()
	registry.Load(&llmplugin.Plugin{})

	config := &PolicyConfig{
		Version: "1.0",
		Name:    "test",
		Start:   "n1",
		Nodes: map[string]*NodeConfig{
			"n1": {
				ID:         "n1",
				Capability: "llm",
				Input:      "test",
				Edges: []EdgeConfig{
					{To: "nonexistent"},
				},
			},
		},
	}

	validator := NewPolicyValidator(registry)
	result := validator.ValidateConfig(config, registry)

	if result.Valid {
		t.Fatal("validation should fail for invalid edge target")
	}
}

func TestFromConfig_BuildsGraphCorrectly(t *testing.T) {
	config := &PolicyConfig{
		Version: "1.0",
		Name:    "test",
		Start:   "n1",
		Nodes: map[string]*NodeConfig{
			"n1": {
				ID:         "n1",
				Capability: "llm",
				Input:      "start",
				Edges: []EdgeConfig{
					{To: "n2", Condition: "status == 'SUCCESS'"},
				},
			},
			"n2": {
				ID:         "n2",
				Capability: "tool",
				Input:      "execute",
				Edges:      []EdgeConfig{},
			},
		},
	}

	builder, err := FromConfig(config)
	if err != nil {
		t.Fatalf("FromConfig failed: %v", err)
	}

	graph, err := builder.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}
	if graph == nil {
		t.Fatal("graph should not be nil")
	}

	if len(graph.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(graph.Nodes))
	}

	n1 := graph.Nodes[types.NodeID("n1")]
	if len(n1.Next) != 1 {
		t.Fatalf("expected 1 edge from n1, got %d", len(n1.Next))
	}

	if n1.Next[0].Condition == nil {
		t.Fatal("condition should not be nil")
	}
}

func TestNodeBuilder_ChainableAPI(t *testing.T) {
	builder := NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("test").
		Priority(5).
		Tenant("tenant-a").
		Timeout(30 * time.Second).
		RateLimit(100).
		CircuitBreakerKey("cb-key").
		Build()

	node, err := builder.GetNode("n1")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if node.Capability != types.CapabilityName("llm") {
		t.Fatal("capability mismatch")
	}
	if node.Priority != 5 {
		t.Fatal("priority mismatch")
	}
	if node.Tenant != "tenant-a" {
		t.Fatal("tenant mismatch")
	}
	if node.Timeout != 30*time.Second {
		t.Fatal("timeout mismatch")
	}
	if node.RateLimit != 100 {
		t.Fatal("rate limit mismatch")
	}
	if node.CircuitBreakerKey != "cb-key" {
		t.Fatal("circuit breaker key mismatch")
	}
}
