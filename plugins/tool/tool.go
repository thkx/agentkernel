package tool

import (
	"fmt"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/types"
)

type Tool struct{}

func (t *Tool) Name() types.CapabilityName { return "tool" }

func (t *Tool) Invoke(ctx types.ExecContext, input any) (any, error) {
	return fmt.Sprintf("TOOL(%v)", input), nil
}

type Plugin struct{}

func (p *Plugin) Register(registry *capability.Registry) {
	registry.Register(&Tool{})
}

func (p *Plugin) Metadata() capability.PluginMetadata {
	return capability.PluginMetadata{
		Name:         "tool",
		Version:      "v1",
		Description:  "Registers the built-in tool capability.",
		Capabilities: []types.CapabilityName{"tool"},
	}
}
