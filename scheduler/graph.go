package scheduler

import "github.com/thkx/agentkernel/types"

type GraphEngine struct {
	graph *types.Graph
}

func NewGraphEngine(g *types.Graph) *GraphEngine {
	return &GraphEngine{graph: g}
}

func (g *GraphEngine) GetNode(id types.NodeID) *types.Node {
	return g.graph.Nodes[id]
}
