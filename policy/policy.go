package policy

import (
	"fmt"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/types"
)

// BuilderPlanner builds graphs using the PolicyBuilder fluent API
type BuilderPlanner struct {
	builder *PolicyBuilder
	bus     types.EventBus
}

// NewBuilderPlanner creates a planner from a PolicyBuilder
func NewBuilderPlanner(builder *PolicyBuilder) *BuilderPlanner {
	return &BuilderPlanner{builder: builder}
}

func (bp *BuilderPlanner) WithBus(bus types.EventBus) *BuilderPlanner {
	bp.bus = bus
	return bp
}

// Build builds a graph from the PolicyBuilder
func (bp *BuilderPlanner) Build(input any) (*types.Graph[any], error) {
	graph, err := bp.builder.BuildGraph()
	nodeCount := 0
	if graph != nil {
		nodeCount = len(graph.Nodes)
	}
	publishPolicyEvent(bp.bus, "policy.graph_built", "", "builder_planner", err == nil, nodeCount, err, 0, boolToErrorCount(err), map[string]any{"input_provided": input != nil})
	return graph, err
}

// GetBuilder returns the underlying PolicyBuilder
func (bp *BuilderPlanner) GetBuilder() *PolicyBuilder {
	return bp.builder
}

// Validate validates the built graph
func (bp *BuilderPlanner) Validate(capRegistry *capability.Registry) *ValidationResult {
	validator := NewPolicyValidator(capRegistry)
	validator.bus = bp.bus
	graph, err := bp.builder.BuildGraph()
	if err != nil {
		result := &ValidationResult{
			Valid: false,
			Errors: []ValidationError{
				{
					Level:   "error",
					NodeID:  "",
					Message: fmt.Sprintf("failed to build graph: %v", err),
				},
			},
		}
		publishPolicyEvent(bp.bus, "policy.graph_build_failed", "", "builder_planner", false, 0, err, 0, 1, nil)
		return result
	}
	result := validator.ValidateGraph(graph, capRegistry)
	warnings, errors := summarizeValidation(result)
	eventName := "policy.validated"
	if !result.Valid {
		eventName = "policy.validation_failed"
	}
	publishPolicyEvent(bp.bus, eventName, "", "builder_planner", result.Valid, len(graph.Nodes), nil, warnings, errors, nil)
	return result
}

func boolToErrorCount(err error) int {
	if err != nil {
		return 1
	}
	return 0
}
