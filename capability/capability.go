package capability

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"

	"github.com/thkx/agentkernel/types"
)

// Capability is a runtime-managed execution unit.
//
// Implementations should treat ExecContext.State as a read-only snapshot and
// use ExecContext.GetState/SetState/DeleteState for shared mutable state.
type Capability interface {
	Name() types.CapabilityName
	Invoke(ctx types.ExecContext, input any) (any, error)
}

type Plugin interface {
	Register(registry *Registry)
}

type PluginMetadata struct {
	Name         string
	Version      string
	Description  string
	Capabilities []types.CapabilityName
}

type DescribedPlugin interface {
	Plugin
	Metadata() PluginMetadata
}

type LifecyclePlugin interface {
	Plugin
	Start(ctx context.Context, registry *Registry) error
	Stop(ctx context.Context) error
}

type ConfigurablePlugin interface {
	Plugin
	Configure(config any) error
}

type Registry struct {
	mu         sync.RWMutex
	caps       map[types.CapabilityName]Capability
	duplicates map[types.CapabilityName]int
}

func NewRegistry() *Registry {
	return &Registry{
		caps:       map[types.CapabilityName]Capability{},
		duplicates: map[types.CapabilityName]int{},
	}
}

func (r *Registry) Register(cap Capability) {
	if cap == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	name := cap.Name()
	if _, exists := r.caps[name]; exists {
		r.duplicates[name]++
		return
	}
	r.caps[name] = cap
}

func (r *Registry) Get(name types.CapabilityName) (Capability, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cap, ok := r.caps[name]
	return cap, ok
}

func (r *Registry) Load(plugin Plugin) {
	plugin.Register(r)
}

func (r *Registry) Names() []types.CapabilityName {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]types.CapabilityName, 0, len(r.caps))
	for name := range r.caps {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return names[i] < names[j]
	})
	return names
}

func (r *Registry) DuplicateNames() []types.CapabilityName {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]types.CapabilityName, 0, len(r.duplicates))
	for name := range r.duplicates {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return names[i] < names[j]
	})
	return names
}

func PluginKey(plugin Plugin) string {
	if plugin == nil {
		return ""
	}
	if described, ok := plugin.(DescribedPlugin); ok {
		meta := described.Metadata()
		if meta.Name != "" {
			return meta.Name
		}
	}
	t := reflect.TypeOf(plugin)
	if t == nil {
		return ""
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Name() != "" {
		return t.Name()
	}
	return fmt.Sprintf("%T", plugin)
}
