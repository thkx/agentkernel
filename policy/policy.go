package policy

import "github.com/thkx/agentkernel/types"

type Planner struct{}

func (p *Planner) Build(input any) *types.Graph[any] {
	return types.NewGraph[any](
		"n1",
		map[types.NodeID]*types.Node[any]{
			"n1": {
				ID:         "n1",
				Capability: "llm",
				Input:      input,
				Next:       []types.Edge{{To: "n2"}},
			},
			"n2": {
				ID:         "n2",
				Capability: "tool",
				Input:      "process",
			},
		},
	)
}
