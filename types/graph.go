package types

type NodeID string

type Node struct {
	ID         NodeID
	Capability string
	Input      any
	Next       []Edge
}

type Edge struct {
	To        NodeID
	Condition func(result Result) bool
}

type Graph struct {
	Start NodeID
	Nodes map[NodeID]*Node
}
