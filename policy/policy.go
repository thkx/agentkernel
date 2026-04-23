package policy

import (
	"fmt"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/types"
)

// BuilderPlanner builds graphs using the PolicyBuilder fluent API
type BuilderPlanner struct {
	builder *PolicyBuilder
}

// NewBuilderPlanner creates a planner from a PolicyBuilder
func NewBuilderPlanner(builder *PolicyBuilder) *BuilderPlanner {
	return &BuilderPlanner{builder: builder}
}

// Build builds a graph from the PolicyBuilder
func (bp *BuilderPlanner) Build(input any) (*types.Graph[any], error) {
	return bp.builder.BuildGraph()
}

// GetBuilder returns the underlying PolicyBuilder
func (bp *BuilderPlanner) GetBuilder() *PolicyBuilder {
	return bp.builder
}

// Validate validates the built graph
func (bp *BuilderPlanner) Validate(capRegistry *capability.Registry) *ValidationResult {
	validator := NewPolicyValidator(capRegistry)
	graph, err := bp.builder.BuildGraph()
	if err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{
					Level:   "error",
					NodeID:  "",
					Message: fmt.Sprintf("failed to build graph: %v", err),
				},
			},
		}
	}
	return validator.ValidateGraph(graph, capRegistry)
}
