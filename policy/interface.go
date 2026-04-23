// Package policy provides the Policy interface and engine for decision-making in controlled execution.
package policy

import (
	"context"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/types"
)

// Policy is the core interface for making execution decisions.
// A Policy only decides, never executes.
type Policy interface {
	// Name returns the name of this policy
	Name() string

	// Decide evaluates the current context and returns a decision (Plan)
	Decide(ctx context.Context, planCtx *PlanContext) (*Plan, error)

	// Validate checks if the policy is valid for the given capability registry
	Validate(capRegistry *capability.Registry) error
}

// PlanContext contains contextual information for making decisions
type PlanContext struct {
	// Current execution context
	NodeID       types.NodeID
	Graph        *types.Graph[any]
	CurrentState map[string]any
	TraceID      string
	SpanID       string

	// Previous execution result (if coming from a prior node)
	PreviousResult *types.Result

	// User input or task description
	UserInput any

	// Capability registry for validation
	CapabilityRegistry *capability.Registry

	// Variables available to the policy
	Variables map[string]any
}

// Plan represents a decision made by a policy
type Plan struct {
	// The graph that should be executed
	Graph *types.Graph[any]

	// The starting node for execution
	StartNode types.NodeID

	// The name of the policy that made this decision
	PolicyName string

	// Metadata about the plan
	Metadata map[string]any
}

// ConfigDrivenPolicy represents a policy built from configuration
type ConfigDrivenPolicy struct {
	name   string
	config *PolicyConfig
}

// ProgrammaticPolicy represents a policy built programmatically
type ProgrammaticPolicy struct {
	name      string
	graph     *types.Graph[any]
	startNode types.NodeID
}
