package policy

import "github.com/thkx/agentkernel/types"

type Planner struct{}

func (p *Planner) Build(input any) *types.Graph {

	return &types.Graph{
		Start: "n1",
		Nodes: map[types.NodeID]*types.Node{

			"n1": {
				ID:         "n1",
				Capability: "llm",
				Input:      input,
				Next: []types.Edge{
					{To: "n2"},
				},
			},

			"n2": {
				ID:         "n2",
				Capability: "tool",
				Input:      "process",
			},
		},
	}
}
