package policy

import (
	"fmt"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/types"
)

// PolicyValidator validates a policy graph for correctness and completeness
type PolicyValidator struct {
	capRegistry *capability.Registry
}

// NewPolicyValidator creates a new policy validator
func NewPolicyValidator(capRegistry *capability.Registry) *PolicyValidator {
	return &PolicyValidator{
		capRegistry: capRegistry,
	}
}

// ValidationError represents a validation failure
type ValidationError struct {
	Level   string // "error" or "warning"
	NodeID  string
	Message string
}

// ValidationResult contains the result of policy validation
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// ValidateGraph validates a graph for correctness
func (pv *PolicyValidator) ValidateGraph(graph *types.Graph[any], capRegistry *capability.Registry) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: make([]ValidationError, 0),
	}

	if graph == nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Level:   "error",
			Message: "graph is nil",
		})
		return result
	}

	// Validate start node
	if graph.Start == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Level:   "error",
			Message: "start node not specified",
		})
		return result
	}

	nodes := graph.Nodes
	if len(nodes) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Level:   "error",
			Message: "no nodes in graph",
		})
		return result
	}

	// Check start node exists
	if _, exists := nodes[graph.Start]; !exists {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Level:   "error",
			Message: fmt.Sprintf("start node '%s' not found", graph.Start),
		})
		return result
	}

	// Validate each node
	for _, node := range nodes {
		nodeErrors := pv.validateNode(node, nodes)
		result.Errors = append(result.Errors, nodeErrors...)
	}

	// Check for disconnected nodes (warnings)
	reachable := pv.findReachableNodes(nodes, graph.Start)
	for nodeID := range nodes {
		if _, found := reachable[nodeID]; !found && nodeID != graph.Start {
			result.Errors = append(result.Errors, ValidationError{
				Level:   "warning",
				NodeID:  string(nodeID),
				Message: "node is not reachable from start node",
			})
		}
	}

	// Check for cycles
	if hasCycle := pv.hasCycle(nodes, graph.Start); hasCycle {
		result.Errors = append(result.Errors, ValidationError{
			Level:   "warning",
			Message: "graph contains cycles (may be intentional for loops)",
		})
	}

	// Check for dead ends (nodes with no outgoing edges except end nodes)
	for _, node := range nodes {
		if len(node.Next) == 0 {
			// This is an end node, which is fine
			continue
		}
	}

	// Set valid flag
	result.Valid = true
	for _, err := range result.Errors {
		if err.Level == "error" {
			result.Valid = false
			break
		}
	}

	return result
}

// validateNode validates a single node
func (pv *PolicyValidator) validateNode(node *types.Node[any], allNodes map[types.NodeID]*types.Node[any]) []ValidationError {
	errors := make([]ValidationError, 0)

	if node.ID == "" {
		errors = append(errors, ValidationError{
			Level:   "error",
			Message: "node has empty ID",
		})
		return errors
	}

	// Check capability exists
	if err := pv.checkCapability(node.Capability, string(node.ID)); err != nil {
		errors = append(errors, *err)
	}

	// Check all edges point to existing nodes
	for _, edge := range node.Next {
		if edge.To == "" {
			errors = append(errors, ValidationError{
				Level:   "error",
				NodeID:  string(node.ID),
				Message: "edge has no target",
			})
			continue
		}

		if _, exists := allNodes[edge.To]; !exists {
			errors = append(errors, ValidationError{
				Level:   "error",
				NodeID:  string(node.ID),
				Message: fmt.Sprintf("edge points to non-existent node '%s'", edge.To),
			})
		}
	}

	// Check input is not nil
	if node.Input == nil {
		errors = append(errors, ValidationError{
			Level:   "warning",
			NodeID:  string(node.ID),
			Message: "node input is nil",
		})
	}

	return errors
}

// checkCapability checks if a capability exists in the registry
func (pv *PolicyValidator) checkCapability(capName types.CapabilityName, nodeID string) *ValidationError {
	if pv.capRegistry != nil {
		if _, ok := pv.capRegistry.Get(capName); !ok {
			return &ValidationError{
				Level:   "error",
				NodeID:  nodeID,
				Message: fmt.Sprintf("capability '%s' not found in registry", capName),
			}
		}
	}
	return nil
}

// findReachableNodes finds all nodes reachable from a start node
func (pv *PolicyValidator) findReachableNodes(nodes map[types.NodeID]*types.Node[any], start types.NodeID) map[types.NodeID]bool {
	reachable := make(map[types.NodeID]bool)
	visited := make(map[types.NodeID]bool)

	var dfs func(id types.NodeID)
	dfs = func(id types.NodeID) {
		if visited[id] {
			return
		}
		visited[id] = true
		reachable[id] = true

		if node, exists := nodes[id]; exists {
			for _, edge := range node.Next {
				dfs(edge.To)
			}
		}
	}

	dfs(start)
	return reachable
}

// hasCycle detects if the graph has cycles
func (pv *PolicyValidator) hasCycle(nodes map[types.NodeID]*types.Node[any], start types.NodeID) bool {
	visited := make(map[types.NodeID]bool)
	recStack := make(map[types.NodeID]bool)

	var dfs func(id types.NodeID) bool
	dfs = func(id types.NodeID) bool {
		visited[id] = true
		recStack[id] = true

		if node, exists := nodes[id]; exists {
			for _, edge := range node.Next {
				if !visited[edge.To] {
					if dfs(edge.To) {
						return true
					}
				} else if recStack[edge.To] {
					return true
				}
			}
		}

		recStack[id] = false
		return false
	}

	return dfs(start)
}

// ValidateConfig validates a PolicyConfig
func (pv *PolicyValidator) ValidateConfig(config *PolicyConfig, capRegistry *capability.Registry) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: make([]ValidationError, 0),
	}

	if config == nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Level:   "error",
			Message: "config is nil",
		})
		return result
	}

	if config.Start == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Level:   "error",
			Message: "start node not specified in config",
		})
		return result
	}

	if len(config.Nodes) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Level:   "error",
			Message: "no nodes in config",
		})
		return result
	}

	// Check start node exists
	if _, exists := config.Nodes[config.Start]; !exists {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Level:   "error",
			Message: fmt.Sprintf("start node '%s' not found in config", config.Start),
		})
		return result
	}

	// Validate each node
	for _, nodeConfig := range config.Nodes {
		nodeErrors := pv.validateNodeConfig(nodeConfig, config.Nodes)
		result.Errors = append(result.Errors, nodeErrors...)
	}

	// Set valid flag
	result.Valid = true
	for _, err := range result.Errors {
		if err.Level == "error" {
			result.Valid = false
			break
		}
	}

	return result
}

// validateNodeConfig validates a single NodeConfig
func (pv *PolicyValidator) validateNodeConfig(nodeConfig *NodeConfig, allNodes map[string]*NodeConfig) []ValidationError {
	errors := make([]ValidationError, 0)

	if nodeConfig.ID == "" {
		errors = append(errors, ValidationError{
			Level:   "error",
			Message: "node has empty ID",
		})
		return errors
	}

	if nodeConfig.Capability == "" {
		errors = append(errors, ValidationError{
			Level:   "error",
			NodeID:  nodeConfig.ID,
			Message: "node has no capability specified",
		})
	}

	// Check capability exists
	if nodeConfig.Capability != "" {
		if err := pv.checkCapability(types.CapabilityName(nodeConfig.Capability), nodeConfig.ID); err != nil {
			errors = append(errors, *err)
		}
	}

	// Check edges
	for _, edge := range nodeConfig.Edges {
		if edge.To == "" {
			errors = append(errors, ValidationError{
				Level:   "error",
				NodeID:  nodeConfig.ID,
				Message: "edge has no target node",
			})
			continue
		}

		if _, exists := allNodes[edge.To]; !exists {
			errors = append(errors, ValidationError{
				Level:   "error",
				NodeID:  nodeConfig.ID,
				Message: fmt.Sprintf("edge points to non-existent node '%s'", edge.To),
			})
		}
	}

	return errors
}

// GetValidationSummary returns a summary string of the validation result
func (vr *ValidationResult) GetValidationSummary() string {
	errorCount := 0
	warningCount := 0

	for _, err := range vr.Errors {
		if err.Level == "error" {
			errorCount++
		} else if err.Level == "warning" {
			warningCount++
		}
	}

	if vr.Valid {
		return "Validation passed"
	}

	return fmt.Sprintf("Validation failed: %d error(s), %d warning(s)",
		errorCount, warningCount)
}
