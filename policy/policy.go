package policy

import "github.com/thkx/agentkernel/types"

type PolicyEngine struct{}

func (p *PolicyEngine) Plan(input any) []types.Task {
	return []types.Task{
		{
			ID:    "task-1",
			Type:  "llm",
			Input: input,
		},
		{
			ID:    "task-2",
			Type:  "tool",
			Input: "process result",
		},
	}
}
