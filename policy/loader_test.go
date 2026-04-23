package policy

import (
	"testing"
)

func TestPolicyLoader_LoadFromString(t *testing.T) {
	loader := NewPolicyLoader()

	yamlConfig := `
name: test-policy
start: n1
nodes:
  n1:
    capability: llm
    input: "hello"
  n2:
    capability: tool
    input: "process"
`

	config, err := loader.LoadFromYAMLString(yamlConfig)
	if err != nil {
		t.Fatalf("LoadFromYAMLString failed: %v", err)
	}

	if config.Start != "n1" {
		t.Errorf("expected start 'n1', got '%s'", config.Start)
	}

	if len(config.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(config.Nodes))
	}

	if config.Nodes["n1"].Capability != "llm" {
		t.Errorf("expected n1 capability 'llm', got '%s'", config.Nodes["n1"].Capability)
	}
}

func TestPolicyLoader_LoadFromJSON(t *testing.T) {
	loader := NewPolicyLoader()

	jsonConfig := `{
		"name": "test-policy",
		"start": "n1",
		"nodes": {
			"n1": {
				"capability": "llm",
				"input": "hello"
			}
		}
	}`

	config, err := loader.LoadFromJSONString(jsonConfig)
	if err != nil {
		t.Fatalf("LoadFromJSONString failed: %v", err)
	}

	if config.Start != "n1" {
		t.Errorf("expected start 'n1', got '%s'", config.Start)
	}
}

func TestPolicyLoader_LoadFromInvalidFormat(t *testing.T) {
	loader := NewPolicyLoader()

	_, err := loader.LoadFromYAMLString("invalid: - yaml: content")
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
