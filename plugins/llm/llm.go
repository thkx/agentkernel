package llm

import (
	"fmt"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/types"
)

type LLM struct{}

func (l *LLM) Name() types.CapabilityName { return "llm" }

func (l *LLM) Invoke(ctx types.ExecContext, input any) (any, error) {
	return fmt.Sprintf("LLM(%s)", input), nil
}

type Plugin struct{}

func (p *Plugin) Register(registry *capability.Registry) {
	registry.Register(&LLM{})
}

func (p *Plugin) Metadata() capability.PluginMetadata {
	return capability.PluginMetadata{
		Name:         "llm",
		Version:      "v1",
		Description:  "Registers the built-in llm capability.",
		Capabilities: []types.CapabilityName{"llm"},
	}
}
