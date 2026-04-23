package policy

import (
	"testing"
)

func TestInputEvaluator_EvaluateInput(t *testing.T) {
	ie := NewInputEvaluator(map[string]interface{}{
		"user": "john",
		"age":  25,
	})

	tests := []struct {
		name     string
		input    interface{}
		state    map[string]interface{}
		expected interface{}
		hasError bool
	}{
		{
			name:     "static string",
			input:    "hello",
			state:    map[string]interface{}{},
			expected: "hello",
			hasError: false,
		},
		{
			name:     "simple variable",
			input:    "$user",
			state:    map[string]interface{}{},
			expected: "john",
			hasError: false,
		},
		{
			name:     "template string",
			input:    "user: $user, age: $age",
			state:    map[string]interface{}{},
			expected: "user: john, age: 25",
			hasError: false,
		},
		{
			name:     "state variable",
			input:    "$count",
			state:    map[string]interface{}{"count": 42},
			expected: 42,
			hasError: false,
		},
		{
			name:     "undefined variable",
			input:    "$missing",
			state:    map[string]interface{}{},
			expected: nil,
			hasError: true,
		},
		{
			name:     "nil input",
			input:    nil,
			state:    map[string]interface{}{},
			expected: nil,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ie.EvaluateInput(tt.input, tt.state)
			if tt.hasError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.hasError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.hasError && result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
