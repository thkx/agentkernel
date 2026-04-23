package capability

import (
	"fmt"
	"testing"

	"github.com/thkx/agentkernel/types"
)

type testLLM struct{}

func (l *testLLM) Name() types.CapabilityName { return "test-llm" }

func (l *testLLM) Invoke(ctx types.ExecContext, input any) (any, error) {
	return fmt.Sprintf("LLM(%s)", input), nil
}

type testTool struct{}

func (t *testTool) Name() types.CapabilityName { return "test-tool" }

func (t *testTool) Invoke(ctx types.ExecContext, input any) (any, error) {
	return fmt.Sprintf("TOOL(%v)", input), nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	registry := NewRegistry()

	llm := &testLLM{}
	registry.Register(llm)

	retrieved, exists := registry.Get("test-llm")
	if !exists {
		t.Fatal("expected to retrieve registered capability")
	}

	if retrieved != llm {
		t.Error("retrieved capability should be the same instance")
	}

	// Test non-existent capability
	_, exists = registry.Get("missing")
	if exists {
		t.Error("expected nil for non-existent capability")
	}
}

func TestLLM_Invoke(t *testing.T) {
	llm := &testLLM{}

	ctx := types.ExecContext{
		NodeID:  "test-node",
		TraceID: "test-trace",
		SpanID:  "test-span",
		State:   nil, // StateAccessor interface, can be nil for basic tests
	}

	result, err := llm.Invoke(ctx, "hello")
	if err != nil {
		t.Fatalf("LLM invoke failed: %v", err)
	}

	if result == nil {
		t.Error("expected non-nil result")
	}

	// LLM returns formatted output
	expected := "LLM(hello)"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestTool_Invoke(t *testing.T) {
	tool := &testTool{}

	ctx := types.ExecContext{
		NodeID:  "test-node",
		TraceID: "test-trace",
		SpanID:  "test-span",
		State:   nil, // StateAccessor interface, can be nil for basic tests
	}

	result, err := tool.Invoke(ctx, "execute")
	if err != nil {
		t.Fatalf("Tool invoke failed: %v", err)
	}

	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestLLM_Name(t *testing.T) {
	llm := &testLLM{}
	if llm.Name() != "test-llm" {
		t.Errorf("expected name 'test-llm', got '%s'", llm.Name())
	}
}

func TestTool_Name(t *testing.T) {
	tool := &testTool{}
	if tool.Name() != "test-tool" {
		t.Errorf("expected name 'test-tool', got '%s'", tool.Name())
	}
}
