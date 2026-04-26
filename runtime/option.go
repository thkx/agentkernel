package runtime

import (
	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/scheduler"
	"github.com/thkx/agentkernel/types"
)

type RuntimeOption func(*Runtime)

func WithSchedulerOption(opt scheduler.SchedulerOption) RuntimeOption {
	return func(r *Runtime) {
		r.schedulerOptions = append(r.schedulerOptions, opt)
	}
}

func WithSchedulerOptions(opts ...scheduler.SchedulerOption) RuntimeOption {
	return func(r *Runtime) {
		r.schedulerOptions = append(r.schedulerOptions, opts...)
	}
}

func WithBus(bus types.EventBus) RuntimeOption {
	return func(r *Runtime) {
		r.bus = bus
	}
}

func WithCapabilityRegistry(reg *capability.Registry) RuntimeOption {
	return func(r *Runtime) {
		r.caps = reg
	}
}

func WithGraph(graph *types.Graph[any]) RuntimeOption {
	return func(r *Runtime) {
		r.graph = graph
	}
}

func WithPlugin(plugin capability.Plugin) RuntimeOption {
	return func(r *Runtime) {
		if plugin == nil {
			return
		}
		r.plugins = append(r.plugins, plugin)
	}
}

func WithPlugins(plugins ...capability.Plugin) RuntimeOption {
	return func(r *Runtime) {
		for _, plugin := range plugins {
			if plugin == nil {
				continue
			}
			r.plugins = append(r.plugins, plugin)
		}
	}
}

func WithPluginConfig(name string, config any) RuntimeOption {
	return func(r *Runtime) {
		if name == "" {
			return
		}
		if r.pluginConfigs == nil {
			r.pluginConfigs = make(map[string]any)
		}
		r.pluginConfigs[name] = config
	}
}

func WithPluginConfigs(configs map[string]any) RuntimeOption {
	return func(r *Runtime) {
		if len(configs) == 0 {
			return
		}
		if r.pluginConfigs == nil {
			r.pluginConfigs = make(map[string]any, len(configs))
		}
		for name, config := range configs {
			if name == "" {
				continue
			}
			r.pluginConfigs[name] = config
		}
	}
}

func WithBeforeHook(hook types.HookFunc) RuntimeOption {
	return func(r *Runtime) {
		r.beforeHooks = append(r.beforeHooks, hook)
	}
}

func WithAfterHook(hook types.HookFunc) RuntimeOption {
	return func(r *Runtime) {
		r.afterHooks = append(r.afterHooks, hook)
	}
}

// WithBeforeWritableHook registers a before-hook that can mutate shared runtime state.
func WithBeforeWritableHook(hook types.WritableHookFunc) RuntimeOption {
	return func(r *Runtime) {
		r.beforeWritable = append(r.beforeWritable, hook)
	}
}

// WithAfterWritableHook registers an after-hook that can mutate shared runtime state.
func WithAfterWritableHook(hook types.WritableHookFunc) RuntimeOption {
	return func(r *Runtime) {
		r.afterWritable = append(r.afterWritable, hook)
	}
}
