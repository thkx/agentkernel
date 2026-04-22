package policy

import (
	"fmt"
	"time"

	"github.com/thkx/agentkernel/types"
)

// PolicyBuilder provides a fluent API for building DAGs at runtime
type PolicyBuilder struct {
	nodes   map[types.NodeID]*types.Node[any]
	start   types.NodeID
	nodeMap map[types.NodeID]*NodeBuildContext
}

// NodeBuildContext holds context while building a node
type NodeBuildContext struct {
	node  *types.Node[any]
	edges []types.Edge
}

// NewPolicyBuilder creates a new policy builder
func NewPolicyBuilder() *PolicyBuilder {
	return &PolicyBuilder{
		nodes:   make(map[types.NodeID]*types.Node[any]),
		nodeMap: make(map[types.NodeID]*NodeBuildContext),
	}
}

// Start sets the start node ID
func (pb *PolicyBuilder) Start(nodeID string) *PolicyBuilder {
	pb.start = types.NodeID(nodeID)
	return pb
}

// Node adds a node to the graph
func (pb *PolicyBuilder) Node(id string) *NodeBuilder {
	return &NodeBuilder{
		pb:   pb,
		id:   types.NodeID(id),
		node: &types.Node[any]{ID: types.NodeID(id)},
	}
}

// NodeBuilder provides a fluent interface for building a single node
type NodeBuilder struct {
	pb   *PolicyBuilder
	id   types.NodeID
	node *types.Node[any]
}

// Capability sets the capability for this node
func (nb *NodeBuilder) Capability(name string) *NodeBuilder {
	nb.node.Capability = types.CapabilityName(name)
	return nb
}

// Input sets the input for this node
func (nb *NodeBuilder) Input(input any) *NodeBuilder {
	nb.node.Input = input
	return nb
}

// Next adds edges to the next nodes
func (nb *NodeBuilder) Next(edges ...types.Edge) *NodeBuilder {
	nb.node.Next = append(nb.node.Next, edges...)
	return nb
}

// NextNode is a convenience method to add a single edge
func (nb *NodeBuilder) NextNode(to string) *NodeBuilder {
	nb.node.Next = append(nb.node.Next, types.Edge{To: types.NodeID(to)})
	return nb
}

// NextConditional adds a conditional edge
func (nb *NodeBuilder) NextConditional(to string, condition func(types.Result) bool) *NodeBuilder {
	nb.node.Next = append(nb.node.Next, types.Edge{
		To:        types.NodeID(to),
		Condition: condition,
	})
	return nb
}

// Priority sets the priority for this node
func (nb *NodeBuilder) Priority(p int) *NodeBuilder {
	nb.node.Priority = p
	return nb
}

// Tenant sets the tenant for this node
func (nb *NodeBuilder) Tenant(tenant string) *NodeBuilder {
	nb.node.Tenant = tenant
	return nb
}

// Timeout sets the timeout for this node
func (nb *NodeBuilder) Timeout(timeout time.Duration) *NodeBuilder {
	nb.node.Timeout = timeout
	return nb
}

// RateLimit sets the rate limit for this node
func (nb *NodeBuilder) RateLimit(limit int) *NodeBuilder {
	nb.node.RateLimit = limit
	return nb
}

// CircuitBreakerKey sets the circuit breaker key for this node
func (nb *NodeBuilder) CircuitBreakerKey(key string) *NodeBuilder {
	nb.node.CircuitBreakerKey = key
	return nb
}

// Build adds this node to the graph and returns the builder for chaining
func (nb *NodeBuilder) Build() *PolicyBuilder {
	nb.pb.nodes[nb.id] = nb.node
	nb.pb.nodeMap[nb.id] = &NodeBuildContext{
		node:  nb.node,
		edges: nb.node.Next,
	}
	return nb.pb
}

// BuildGraph creates the final Graph
func (pb *PolicyBuilder) BuildGraph() *types.Graph[any] {
	if pb.start == "" {
		panic("start node not set")
	}
	return types.NewGraph[any](pb.start, pb.nodes)
}

// FromConfig builds a PolicyBuilder from a PolicyConfig
func FromConfig(cfg *PolicyConfig) (*PolicyBuilder, error) {
	if cfg.Start == "" {
		return nil, fmt.Errorf("policy config: start node not specified")
	}

	pb := NewPolicyBuilder().Start(cfg.Start)

	// Build nodes
	for _, nodeCfg := range cfg.Nodes {
		if err := buildNode(pb, nodeCfg); err != nil {
			return nil, err
		}
	}

	return pb, nil
}

// buildNode adds a single node from config to the builder
func buildNode(pb *PolicyBuilder, nodeCfg *NodeConfig) error {
	if nodeCfg.ID == "" {
		return fmt.Errorf("policy config: node ID is empty")
	}

	if nodeCfg.Capability == "" {
		return fmt.Errorf("policy config: node %s has no capability", nodeCfg.ID)
	}

	nb := pb.Node(nodeCfg.ID).
		Capability(nodeCfg.Capability).
		Input(nodeCfg.Input)

	applyNodeConfig(nb, nodeCfg.Config)

	edges, err := buildEdges(nodeCfg.ID, nodeCfg.Edges)
	if err != nil {
		return err
	}

	nb.Next(edges...)
	nb.Build()
	return nil
}

// applyNodeConfig applies configuration to a node builder
func applyNodeConfig(nb *NodeBuilder, config *NodeSettings) {
	if config == nil {
		return
	}
	if config.Priority > 0 {
		nb.Priority(config.Priority)
	}
	if config.Tenant != "" {
		nb.Tenant(config.Tenant)
	}
	if config.Timeout != nil {
		nb.Timeout(*config.Timeout)
	}
	if config.RateLimit > 0 {
		nb.RateLimit(config.RateLimit)
	}
	if config.CircuitBreakerKey != "" {
		nb.CircuitBreakerKey(config.CircuitBreakerKey)
	}
}

// buildEdges builds edges from config
func buildEdges(fromNodeID string, edgeConfigs []EdgeConfig) ([]types.Edge, error) {
	edges := make([]types.Edge, 0, len(edgeConfigs))
	for _, edgeCfg := range edgeConfigs {
		edge := types.Edge{
			To: types.NodeID(edgeCfg.To),
		}

		if edgeCfg.Condition != "" && edgeCfg.Condition != "*" {
			evaluator := NewConditionEvaluator()
			condFunc, err := evaluator.BuildCondition(edgeCfg.Condition)
			if err != nil {
				return nil, fmt.Errorf("policy config: invalid condition '%s' on edge %s->%s: %w",
					edgeCfg.Condition, fromNodeID, edgeCfg.To, err)
			}
			edge.Condition = condFunc
		}

		edges = append(edges, edge)
	}
	return edges, nil
}

// CloneNode creates a copy of an existing node with a new ID
func (pb *PolicyBuilder) CloneNode(sourceID, newID string) *PolicyBuilder {
	source, exists := pb.nodes[types.NodeID(sourceID)]
	if !exists {
		panic(fmt.Sprintf("source node %s not found", sourceID))
	}

	newNode := &types.Node[any]{
		ID:                types.NodeID(newID),
		Capability:        source.Capability,
		Input:             source.Input,
		Priority:          source.Priority,
		Tenant:            source.Tenant,
		Timeout:           source.Timeout,
		RateLimit:         source.RateLimit,
		CircuitBreakerKey: source.CircuitBreakerKey,
		Next:              make([]types.Edge, len(source.Next)),
	}

	// Update edge references to use new node ID where applicable
	copy(newNode.Next, source.Next)

	pb.nodes[types.NodeID(newID)] = newNode
	return pb
}

// RemoveNode removes a node from the graph
// Returns error if removing the node would break connectivity
func (pb *PolicyBuilder) RemoveNode(nodeID string) error {
	id := types.NodeID(nodeID)

	// Check if it's the start node
	if pb.start == id {
		return fmt.Errorf("cannot remove start node %s", nodeID)
	}

	// Check if any other nodes point to this node
	for _, node := range pb.nodes {
		for _, edge := range node.Next {
			if edge.To == id {
				return fmt.Errorf("cannot remove node %s: referenced by node %s", nodeID, node.ID)
			}
		}
	}

	delete(pb.nodes, id)
	delete(pb.nodeMap, id)
	return nil
}

// UpdateNodeInput updates the input of an existing node
func (pb *PolicyBuilder) UpdateNodeInput(nodeID string, input any) error {
	id := types.NodeID(nodeID)
	node, exists := pb.nodes[id]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	node.Input = input
	return nil
}

// UpdateNodeCapability updates the capability of an existing node
func (pb *PolicyBuilder) UpdateNodeCapability(nodeID string, capability string) error {
	id := types.NodeID(nodeID)
	node, exists := pb.nodes[id]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	node.Capability = types.CapabilityName(capability)
	return nil
}

// InsertNode inserts a new node between two existing nodes
// Returns error if the connection would be invalid
func (pb *PolicyBuilder) InsertNode(beforeNodeID, newNodeID, afterNodeID string,
	capability string, input any) error {

	beforeID := types.NodeID(beforeNodeID)
	newID := types.NodeID(newNodeID)
	afterID := types.NodeID(afterNodeID)

	// Check that fromNode points to toNode
	fromNode, exists := pb.nodes[beforeID]
	if !exists {
		return fmt.Errorf("node %s not found", beforeNodeID)
	}

	// Check that the edge exists
	var foundEdge bool
	for _, edge := range fromNode.Next {
		if edge.To == afterID {
			foundEdge = true
			break
		}
	}
	if !foundEdge {
		return fmt.Errorf("no edge found from %s to %s", beforeNodeID, afterNodeID)
	}

	// Check toNode exists
	if _, exists := pb.nodes[afterID]; !exists {
		return fmt.Errorf("node %s not found", afterNodeID)
	}

	// Create new node
	newNode := &types.Node[any]{
		ID:         newID,
		Capability: types.CapabilityName(capability),
		Input:      input,
		Next: []types.Edge{
			{To: afterID},
		},
	}

	pb.nodes[newID] = newNode

	// Update fromNode to point to new node instead
	for i, edge := range fromNode.Next {
		if edge.To == afterID {
			fromNode.Next[i].To = newID
			break
		}
	}

	return nil
}

// ListNodes returns a list of all node IDs
func (pb *PolicyBuilder) ListNodes() []string {
	nodes := make([]string, 0, len(pb.nodes))
	for id := range pb.nodes {
		nodes = append(nodes, string(id))
	}
	return nodes
}

// GetNode returns a specific node (for inspection)
func (pb *PolicyBuilder) GetNode(nodeID string) (*types.Node[any], error) {
	node, exists := pb.nodes[types.NodeID(nodeID)]
	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}
	return node, nil
}
