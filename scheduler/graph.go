package scheduler

import "github.com/thkx/agentkernel/types"

type GraphEngine[T any] struct {
	graph *types.Graph[T]
}

func NewGraphEngine[T any](g *types.Graph[T]) *GraphEngine[T] {
	return &GraphEngine[T]{graph: g}
}

func (g *GraphEngine[T]) GetNode(id types.NodeID) *types.Node[T] {
	if g == nil || g.graph == nil {
		return nil
	}
	return g.graph.Nodes[id]
}
