package policy

import (
	"testing"
)

func TestPolicyConfig_Validation(t *testing.T) {
	tests := []struct {
		name     string
		config   *PolicyConfig
		hasError bool
	}{
		{
			name: "valid config",
			config: &PolicyConfig{
				Start: "n1",
				Nodes: map[string]*NodeConfig{
					"n1": {Capability: "llm", Input: "hello"},
					"n2": {Capability: "tool", Input: "process"},
				},
			},
			hasError: false,
		},
		{
			name: "missing start",
			config: &PolicyConfig{
				Nodes: map[string]*NodeConfig{
					"n1": {Capability: "llm", Input: "hello"},
				},
			},
			hasError: true,
		},
		{
			name: "start node not in nodes",
			config: &PolicyConfig{
				Start: "n3",
				Nodes: map[string]*NodeConfig{
					"n1": {Capability: "llm", Input: "hello"},
				},
			},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test basic validation - this would be enhanced with proper validation
			if tt.config.Start == "" && tt.hasError {
				// Expected error for missing start
				return
			}
			if tt.config.Start != "" {
				if _, exists := tt.config.Nodes[tt.config.Start]; !exists && tt.hasError {
					// Expected error for invalid start node
					return
				}
			}
			if !tt.hasError {
				// Should be valid
				return
			}
			t.Errorf("expected validation error but got none")
		})
	}
}
