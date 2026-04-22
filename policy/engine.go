package policy

import (
	"context"
	"fmt"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/types"
)

// PolicyEngine orchestrates policy evaluation and plan generation
type PolicyEngine struct {
	policies   map[string]Policy
	evaluator  *ExpressionEvaluator
	selector   PolicySelector
	validator  *PolicyValidator
	strategies map[string]EvaluationStrategy
}

// PolicySelector determines which policy to use for a given context
type PolicySelector interface {
	Select(ctx context.Context, planCtx *PlanContext, policies map[string]Policy) (string, error)
}

// EvaluationStrategy defines how to evaluate policies
type EvaluationStrategy interface {
	Evaluate(ctx context.Context, policies map[string]Policy, planCtx *PlanContext) (*Plan, error)
}

// DefaultPolicySelector selects the first policy or a specified one
type DefaultPolicySelector struct {
	DefaultPolicyName string
}

// SelectFirst is a common strategy that evaluates policies in order and returns the first valid plan
type SelectFirstStrategy struct {
	selector PolicySelector
}

// NewPolicyEngine creates a new policy engine
func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{
		policies:   make(map[string]Policy),
		evaluator:  NewExpressionEvaluator(),
		strategies: make(map[string]EvaluationStrategy),
		selector: &DefaultPolicySelector{
			DefaultPolicyName: "",
		},
	}
}

// Register registers a policy with the engine
func (pe *PolicyEngine) Register(policy Policy) error {
	if policy == nil {
		return fmt.Errorf("policy cannot be nil")
	}
	if policy.Name() == "" {
		return fmt.Errorf("policy name cannot be empty")
	}
	pe.policies[policy.Name()] = policy
	return nil
}

// Evaluate evaluates policies and returns a plan
func (pe *PolicyEngine) Evaluate(ctx context.Context, planCtx *PlanContext) (*Plan, error) {
	if len(pe.policies) == 0 {
		return nil, fmt.Errorf("no policies registered")
	}

	// Use the default strategy if not specified
	strategy := pe.strategies["default"]
	if strategy == nil {
		strategy = &SelectFirstStrategy{
			selector: pe.selector,
		}
	}

	return strategy.Evaluate(ctx, pe.policies, planCtx)
}

// Select selects a specific policy by name and evaluates it
func (pe *PolicyEngine) Select(ctx context.Context, policyName string, planCtx *PlanContext) (*Plan, error) {
	policy, ok := pe.policies[policyName]
	if !ok {
		return nil, fmt.Errorf("policy %q not found", policyName)
	}
	return policy.Decide(ctx, planCtx)
}

// Validate validates all registered policies
func (pe *PolicyEngine) Validate(capRegistry *capability.Registry) error {
	for name, policy := range pe.policies {
		if err := policy.Validate(capRegistry); err != nil {
			return fmt.Errorf("policy %q validation failed: %w", name, err)
		}
	}
	return nil
}

// SetSelector sets the policy selector
func (pe *PolicyEngine) SetSelector(selector PolicySelector) {
	pe.selector = selector
}

// SetStrategy sets an evaluation strategy
func (pe *PolicyEngine) SetStrategy(name string, strategy EvaluationStrategy) {
	pe.strategies[name] = strategy
	if name == "default" {
		pe.strategies["default"] = strategy
	}
}

// Select implements PolicySelector
func (dps *DefaultPolicySelector) Select(ctx context.Context, planCtx *PlanContext, policies map[string]Policy) (string, error) {
	if dps.DefaultPolicyName != "" {
		if _, ok := policies[dps.DefaultPolicyName]; !ok {
			return "", fmt.Errorf("default policy %q not found", dps.DefaultPolicyName)
		}
		return dps.DefaultPolicyName, nil
	}

	// Return the first policy
	for name := range policies {
		return name, nil
	}

	return "", fmt.Errorf("no policies available")
}

// Evaluate implements EvaluationStrategy
func (sfs *SelectFirstStrategy) Evaluate(ctx context.Context, policies map[string]Policy, planCtx *PlanContext) (*Plan, error) {
	if sfs.selector == nil {
		return nil, fmt.Errorf("selector not configured")
	}

	selectedName, err := sfs.selector.Select(ctx, planCtx, policies)
	if err != nil {
		return nil, err
	}

	policy := policies[selectedName]
	return policy.Decide(ctx, planCtx)
}

// NewConfigDrivenPolicy creates a policy from configuration
func NewConfigDrivenPolicy(name string, config *PolicyConfig) *ConfigDrivenPolicy {
	return &ConfigDrivenPolicy{
		name:   name,
		config: config,
	}
}

// NewProgrammaticPolicy creates a policy from a programmatic graph
func NewProgrammaticPolicy(name string, graph *types.Graph[any]) *ProgrammaticPolicy {
	return &ProgrammaticPolicy{
		name:      name,
		graph:     graph,
		startNode: graph.Start,
	}
}

// Name returns the policy name
func (cdp *ConfigDrivenPolicy) Name() string {
	return cdp.name
}

// Name returns the policy name
func (pp *ProgrammaticPolicy) Name() string {
	return pp.name
}

// Decide makes a decision using the configuration
func (cdp *ConfigDrivenPolicy) Decide(ctx context.Context, planCtx *PlanContext) (*Plan, error) {
	if cdp.config == nil {
		return nil, fmt.Errorf("configuration not set")
	}

	// Build the graph from the configuration
	builder, err := FromConfig(cdp.config)
	if err != nil {
		return nil, fmt.Errorf("failed to build graph from config: %w", err)
	}

	graph := builder.BuildGraph()

	return &Plan{
		Graph:      graph,
		StartNode:  graph.Start,
		PolicyName: cdp.Name(),
		Metadata: map[string]any{
			"source": "config",
			"config": cdp.config.Name,
		},
	}, nil
}

// Validate validates the configuration policy
func (cdp *ConfigDrivenPolicy) Validate(capRegistry *capability.Registry) error {
	if cdp.config == nil {
		return fmt.Errorf("configuration not set")
	}

	validator := NewPolicyValidator(capRegistry)
	result := validator.ValidateConfig(cdp.config, capRegistry)

	if !result.Valid {
		var errMsg string
		for _, validationErr := range result.Errors {
			if validationErr.Level == "error" {
				errMsg += fmt.Sprintf("[ERROR] %s\n", validationErr.Message)
			}
		}
		if errMsg != "" {
			return fmt.Errorf("validation failed:\n%s", errMsg)
		}
	}

	return nil
}

// Decide makes a decision using the programmatic graph
func (pp *ProgrammaticPolicy) Decide(ctx context.Context, planCtx *PlanContext) (*Plan, error) {
	if pp.graph == nil {
		return nil, fmt.Errorf("graph not set")
	}

	return &Plan{
		Graph:      pp.graph,
		StartNode:  pp.startNode,
		PolicyName: pp.Name(),
		Metadata: map[string]any{
			"source": "programmatic",
		},
	}, nil
}

// Validate validates the programmatic policy
func (pp *ProgrammaticPolicy) Validate(capRegistry *capability.Registry) error {
	if pp.graph == nil {
		return fmt.Errorf("graph not set")
	}

	validator := NewPolicyValidator(capRegistry)
	result := validator.ValidateGraph(pp.graph, capRegistry)

	if !result.Valid {
		var errMsg string
		for _, validationErr := range result.Errors {
			if validationErr.Level == "error" {
				errMsg += fmt.Sprintf("[ERROR] %s\n", validationErr.Message)
			}
		}
		if errMsg != "" {
			return fmt.Errorf("validation failed:\n%s", errMsg)
		}
	}

	return nil
}
