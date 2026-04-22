// Package policy provides graph builders and evaluators for dynamic execution plans.
package policy

import (
	"time"
)

// PolicyConfig defines the structure of a policy loaded from config file
type PolicyConfig struct {
	// Version of the policy config
	Version string `json:"version" yaml:"version"`

	// Name of the policy
	Name string `json:"name" yaml:"name"`

	// Description of the policy
	Description string `json:"description" yaml:"description"`

	// Start node ID
	Start string `json:"start" yaml:"start"`

	// Nodes in the DAG
	Nodes map[string]*NodeConfig `json:"nodes" yaml:"nodes"`

	// Global settings
	Global *GlobalSettings `json:"global" yaml:"global"`

	// Metadata for policy management
	Metadata map[string]interface{} `json:"metadata" yaml:"metadata"`
}

// NodeConfig defines a single node in the DAG
type NodeConfig struct {
	// ID of the node
	ID string `json:"id" yaml:"id"`

	// Capability name (e.g., "llm", "tool", "agent")
	Capability string `json:"capability" yaml:"capability"`

	// Input: can be a constant, variable reference ($var), or static data
	Input interface{} `json:"input" yaml:"input"`

	// Output variable name (optional, for state storage)
	OutputVar string `json:"output_var" yaml:"output_var"`

	// Edges to next nodes
	Edges []EdgeConfig `json:"edges" yaml:"edges"`

	// Node-specific settings
	Config *NodeSettings `json:"config" yaml:"config"`
}

// EdgeConfig defines an edge between nodes
type EdgeConfig struct {
	// Destination node ID
	To string `json:"to" yaml:"to"`

	// Condition expression (optional)
	// Examples:
	//   "status == 'success'"
	//   "output.contains('yes')"
	//   "result > 100"
	//   "" or "*" means always follow this edge
	Condition string `json:"condition" yaml:"condition"`

	// Priority of this edge (higher priority evaluated first)
	Priority int `json:"priority" yaml:"priority"`
}

// NodeSettings contains node-level configuration
type NodeSettings struct {
	// Priority of task execution (higher priority runs first)
	Priority int `json:"priority" yaml:"priority"`

	// Tenant ID for multi-tenant isolation
	Tenant string `json:"tenant" yaml:"tenant"`

	// Timeout for node execution
	Timeout *time.Duration `json:"timeout" yaml:"timeout"`

	// Rate limit (requests per second)
	RateLimit int `json:"rate_limit" yaml:"rate_limit"`

	// Circuit breaker key
	CircuitBreakerKey string `json:"circuit_breaker_key" yaml:"circuit_breaker_key"`

	// Retry policy for this node
	Retry *RetryConfig `json:"retry" yaml:"retry"`

	// Whether to continue on error
	ContinueOnError bool `json:"continue_on_error" yaml:"continue_on_error"`

	// Custom metadata
	Metadata map[string]interface{} `json:"metadata" yaml:"metadata"`
}

// RetryConfig defines retry behavior for a node
type RetryConfig struct {
	// Maximum number of attempts
	MaxAttempts int `json:"max_attempts" yaml:"max_attempts"`

	// Initial backoff duration
	InitialBackoff time.Duration `json:"initial_backoff" yaml:"initial_backoff"`

	// Maximum backoff duration
	MaxBackoff time.Duration `json:"max_backoff" yaml:"max_backoff"`

	// Backoff multiplier (exponential backoff)
	Multiplier float64 `json:"multiplier" yaml:"multiplier"`

	// Whether to retry on specific errors (optional)
	RetryOn []string `json:"retry_on" yaml:"retry_on"`
}

// GlobalSettings contains global policy settings
type GlobalSettings struct {
	// Default timeout for all nodes
	DefaultTimeout *time.Duration `json:"default_timeout" yaml:"default_timeout"`

	// Default retry policy
	DefaultRetry *RetryConfig `json:"default_retry" yaml:"default_retry"`

	// Default rate limit
	DefaultRateLimit int `json:"default_rate_limit" yaml:"default_rate_limit"`

	// Enable multi-tenant aware scheduling
	TenantAware bool `json:"tenant_aware" yaml:"tenant_aware"`

	// Custom variables for the policy
	Variables map[string]interface{} `json:"variables" yaml:"variables"`
}

// ConditionExpression represents a condition that can be evaluated
type ConditionExpression struct {
	// Raw expression string
	Expression string

	// Parsed components (populated after parsing)
	Left     string      // variable name
	Operator string      // ==, !=, <, >, <=, >=, contains, in
	Right    interface{} // value or variable name

	// For method calls like field.method(arg)
	IsMethodCall bool        // true if this is a method call
	Method       string      // method name (e.g., "contains")
	Arg          interface{} // method argument
}

// ParsedPolicy is the internal representation after loading and validating
type ParsedPolicy struct {
	Config    *PolicyConfig
	Nodes     map[string]*ParsedNode
	StartNode string
	Edges     map[string][]ParsedEdge
}

// ParsedNode represents a parsed node with resolved configurations
type ParsedNode struct {
	ID              string
	Capability      string
	Input           interface{}
	OutputVar       string
	Config          *NodeSettings
	Next            []ParsedEdge
	InputExpression *InputExpression // For input templating
}

// ParsedEdge represents a parsed edge with condition
type ParsedEdge struct {
	To                string
	Condition         *ConditionExpression
	Priority          int
	OriginalCondition string // For debug/logging
}

// InputExpression represents input that may contain variables or expressions
type InputExpression struct {
	// IsStaticValue: true if input is a simple constant
	IsStaticValue bool
	StaticValue   interface{}

	// IsVariableRef: true if input is "$varName"
	IsVariableRef bool
	VarName       string

	// IsExpression: true if input is a complex expression
	IsExpression bool
	ExprString   string
	ExprAst      interface{} // Parsed AST (future)

	// RawInput: the original input value
	RawInput interface{}
}
