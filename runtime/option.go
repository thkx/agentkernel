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
		if r.caps == nil {
			r.caps = capability.NewRegistry()
		}
		r.caps.Load(plugin)
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
