package capability

import "github.com/thkx/agentkernel/types"

type Capability interface {
	Name() types.CapabilityName
	Invoke(ctx types.ExecContext, input any) (any, error)
}

type Plugin interface {
	Register(registry *Registry)
}

type Registry struct {
	caps map[types.CapabilityName]Capability
}

func NewRegistry() *Registry {
	return &Registry{
		caps: map[types.CapabilityName]Capability{},
	}
}

func (r *Registry) Register(cap Capability) {
	r.caps[cap.Name()] = cap
}

func (r *Registry) Get(name types.CapabilityName) (Capability, bool) {
	cap, ok := r.caps[name]
	return cap, ok
}

func (r *Registry) Load(plugin Plugin) {
	plugin.Register(r)
}

type LLMPlugin struct{}

func (p *LLMPlugin) Register(registry *Registry) {
	registry.Register(&LLM{})
}

type ToolPlugin struct{}

func (p *ToolPlugin) Register(registry *Registry) {
	registry.Register(&Tool{})
}
