package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/thkx/agentkernel/capability"
	"github.com/thkx/agentkernel/event"
	"github.com/thkx/agentkernel/scheduler"
	"github.com/thkx/agentkernel/types"
)

type Runtime struct {
	sched            types.Scheduler
	bus              types.EventBus
	caps             *capability.Registry
	graph            *types.Graph[any]
	beforeHooks      []types.HookFunc
	afterHooks       []types.HookFunc
	beforeWritable   []types.WritableHookFunc
	afterWritable    []types.WritableHookFunc
	schedulerOptions []scheduler.SchedulerOption
	plugins          []capability.Plugin
	pluginConfigs    map[string]any
	pluginMeta       []capability.PluginMetadata
	lifecyclePlugins []capability.LifecyclePlugin
	status           Status
	initialized      bool
	stopped          bool
	lastErr          error
	lastStartedAt    time.Time
	lastStoppedAt    time.Time
	lastRunAt        time.Time
	mu               sync.Mutex
}

var ErrRuntimeStopped = errors.New("runtime has been stopped")

type Status string

const (
	StatusCreated  Status = "CREATED"
	StatusStarting Status = "STARTING"
	StatusRunning  Status = "RUNNING"
	StatusStopping Status = "STOPPING"
	StatusStopped  Status = "STOPPED"
	StatusFailed   Status = "FAILED"
)

type Health struct {
	Status          Status
	Started         bool
	Stopped         bool
	Healthy         bool
	LastError       string
	PluginCount     int
	CapabilityCount int
	DuplicateCount  int
	Initialized     bool
	DLQSize         int
	TaskSummary     map[types.TaskStatus]int
	Metrics         MetricsSummary
	LastStartedAt   string
	LastStoppedAt   string
	LastRunAt       string
	Uptime          string
}

type MetricsSummary struct {
	Total      int64
	QPS        float64
	AvgLatency string
	Fastest    string
	Slowest    string
}

func NewRuntime(opts ...RuntimeOption) (*Runtime, error) {
	r := &Runtime{
		bus:    event.NewSourcingBus(event.NewInMemoryEventStore()),
		caps:   capability.NewRegistry(),
		status: StatusCreated,
	}

	for _, opt := range opts {
		opt(r)
	}

	if r.bus == nil {
		r.bus = event.NewSourcingBus(event.NewInMemoryEventStore())
	}
	if r.caps == nil {
		r.caps = capability.NewRegistry()
	}
	if err := r.loadPlugins(); err != nil {
		return nil, err
	}

	engine := scheduler.NewGraphEngine(r.graph)
	r.sched = scheduler.New(engine, r.bus, r.caps, r.schedulerOptions...)

	for _, hook := range r.beforeHooks {
		r.sched.RegisterBeforeHook(hook)
	}
	for _, hook := range r.afterHooks {
		r.sched.RegisterAfterHook(hook)
	}
	for _, hook := range r.beforeWritable {
		r.sched.RegisterBeforeWritableHook(hook)
	}
	for _, hook := range r.afterWritable {
		r.sched.RegisterAfterWritableHook(hook)
	}

	r.publishRuntimeEvent("runtime.created", StatusCreated, "")

	return r, nil
}

func MustNewRuntime(opts ...RuntimeOption) *Runtime {
	rt, err := NewRuntime(opts...)
	if err != nil {
		panic(err)
	}
	return rt
}

func (r *Runtime) Run() error {
	if r.graph == nil {
		return nil
	}
	if err := r.Start(); err != nil {
		return err
	}
	r.markRun()
	go r.sched.Run(r.graph.Start, map[string]any{})
	return nil
}

func (r *Runtime) RunWithContext(ctx context.Context) error {
	if r.graph == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := r.StartWithContext(ctx); err != nil {
		return err
	}
	r.markRun()
	go r.sched.RunWithContext(ctx, r.graph.Start, map[string]any{})
	return nil
}

func (r *Runtime) RunSync() error {
	if r.graph == nil {
		return nil
	}
	if err := r.Start(); err != nil {
		return err
	}
	r.markRun()
	r.sched.Run(r.graph.Start, map[string]any{})
	return nil
}

func (r *Runtime) RunSyncWithContext(ctx context.Context) error {
	if r.graph == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := r.StartWithContext(ctx); err != nil {
		return err
	}
	r.markRun()
	r.sched.RunWithContext(ctx, r.graph.Start, map[string]any{})
	return nil
}

func (r *Runtime) Bus() types.EventBus {
	return r.bus
}

func (r *Runtime) Scheduler() types.Scheduler {
	return r.sched
}

func (r *Runtime) StateSnapshot() map[string]any {
	if r.sched == nil {
		return map[string]any{}
	}
	return r.sched.StateSnapshot()
}

func (r *Runtime) TaskStates() map[types.NodeID]types.TaskStatus {
	if r.sched == nil {
		return map[types.NodeID]types.TaskStatus{}
	}
	return r.sched.TaskStates()
}

func (r *Runtime) Capabilities() []types.CapabilityName {
	if r.caps == nil {
		return nil
	}
	return r.caps.Names()
}

func (r *Runtime) Plugins() []capability.PluginMetadata {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]capability.PluginMetadata, len(r.pluginMeta))
	copy(out, r.pluginMeta)
	return out
}

func (r *Runtime) DuplicateCapabilities() []types.CapabilityName {
	if r.caps == nil {
		return nil
	}
	return r.caps.DuplicateNames()
}

func (r *Runtime) Start() error {
	return r.StartWithContext(context.Background())
}

func (r *Runtime) StartWithContext(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	if r.stopped {
		r.lastErr = ErrRuntimeStopped
		r.status = StatusStopped
		r.mu.Unlock()
		r.publishRuntimeEvent("runtime.start_rejected", StatusStopped, ErrRuntimeStopped.Error())
		return ErrRuntimeStopped
	}
	if r.initialized {
		r.mu.Unlock()
		return nil
	}
	plugins := append([]capability.LifecyclePlugin(nil), r.lifecyclePlugins...)
	prevStatus := r.status
	r.status = StatusStarting
	r.lastErr = nil
	r.mu.Unlock()
	r.publishRuntimeEventTransition("runtime.starting", prevStatus, StatusStarting, "")

	for _, plugin := range plugins {
		if err := plugin.Start(ctx, r.caps); err != nil {
			r.mu.Lock()
			prevStatus = r.status
			r.status = StatusFailed
			r.lastErr = err
			r.mu.Unlock()
			r.publishRuntimeEventTransition("runtime.failed", prevStatus, StatusFailed, err.Error())
			return err
		}
	}

	r.mu.Lock()
	prevStatus = r.status
	r.initialized = true
	r.status = StatusRunning
	r.lastErr = nil
	r.lastStartedAt = time.Now()
	r.mu.Unlock()
	r.publishRuntimeEventTransition("runtime.running", prevStatus, StatusRunning, "")
	return nil
}

// Initialize is kept as a compatibility alias for StartWithContext.
func (r *Runtime) Initialize(ctx context.Context) error {
	return r.StartWithContext(ctx)
}

func (r *Runtime) Stop() error {
	return r.StopWithContext(context.Background())
}

func (r *Runtime) StopWithContext(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	r.stopped = true
	prevStatus := r.status
	r.status = StatusStopping
	if !r.initialized {
		r.status = StatusStopped
		r.mu.Unlock()
		r.publishRuntimeEventTransition("runtime.stopping", prevStatus, StatusStopping, "")
		r.mu.Lock()
		r.lastErr = nil
		r.mu.Unlock()
		r.publishRuntimeEventTransition("runtime.stopped", StatusStopping, StatusStopped, "")
		err := r.closeBus()
		if err != nil {
			r.mu.Lock()
			r.status = StatusFailed
			r.lastErr = err
			r.mu.Unlock()
			return err
		}
		return nil
	}
	plugins := append([]capability.LifecyclePlugin(nil), r.lifecyclePlugins...)
	r.initialized = false
	r.mu.Unlock()
	r.publishRuntimeEventTransition("runtime.stopping", prevStatus, StatusStopping, "")

	var firstErr error
	for i := len(plugins) - 1; i >= 0; i-- {
		if err := plugins[i].Stop(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	r.mu.Lock()
	prevStatus = r.status
	if firstErr != nil {
		r.status = StatusFailed
		r.lastErr = firstErr
	} else {
		r.status = StatusStopped
		r.lastErr = nil
	}
	r.lastStoppedAt = time.Now()
	r.mu.Unlock()
	if firstErr != nil {
		r.publishRuntimeEventTransition("runtime.failed", prevStatus, StatusFailed, firstErr.Error())
	} else {
		r.publishRuntimeEventTransition("runtime.stopped", prevStatus, StatusStopped, "")
	}
	if err := r.closeBus(); err != nil && firstErr == nil {
		r.mu.Lock()
		r.status = StatusFailed
		r.lastErr = err
		r.mu.Unlock()
		return err
	}
	return firstErr
}

func (r *Runtime) loadPlugins() error {
	seenConfigs := make(map[string]struct{}, len(r.pluginConfigs))
	for _, plugin := range r.plugins {
		if plugin == nil {
			continue
		}
		key := capability.PluginKey(plugin)
		if configurable, ok := plugin.(capability.ConfigurablePlugin); ok {
			if config, exists := r.pluginConfigs[key]; exists {
				if err := configurable.Configure(config); err != nil {
					return fmt.Errorf("configure plugin %q: %w", key, err)
				}
				seenConfigs[key] = struct{}{}
			}
		} else if _, exists := r.pluginConfigs[key]; exists {
			return fmt.Errorf("plugin %q does not accept configuration", key)
		}
		before := toCapabilitySet(r.caps.Names())
		plugin.Register(r.caps)
		after := r.caps.Names()
		r.pluginMeta = append(r.pluginMeta, describePlugin(plugin, before, after))
		if lifecycle, ok := plugin.(capability.LifecyclePlugin); ok {
			r.lifecyclePlugins = append(r.lifecyclePlugins, lifecycle)
		}
	}
	for key := range r.pluginConfigs {
		if _, ok := seenConfigs[key]; ok {
			continue
		}
		found := false
		for _, plugin := range r.plugins {
			if capability.PluginKey(plugin) == key {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("plugin config provided for unknown plugin %q", key)
		}
	}
	return nil
}

func describePlugin(plugin capability.Plugin, before map[types.CapabilityName]struct{}, after []types.CapabilityName) capability.PluginMetadata {
	added := make([]types.CapabilityName, 0)
	for _, name := range after {
		if _, exists := before[name]; exists {
			continue
		}
		added = append(added, name)
	}

	if described, ok := plugin.(capability.DescribedPlugin); ok {
		meta := described.Metadata()
		if len(meta.Capabilities) == 0 {
			meta.Capabilities = added
		}
		return meta
	}
	return capability.PluginMetadata{
		Name:         "anonymous",
		Capabilities: added,
	}
}

func toCapabilitySet(names []types.CapabilityName) map[types.CapabilityName]struct{} {
	set := make(map[types.CapabilityName]struct{}, len(names))
	for _, name := range names {
		set[name] = struct{}{}
	}
	return set
}

func (r *Runtime) closeBus() error {
	if r.bus == nil {
		return nil
	}
	return r.bus.Close()
}

func (r *Runtime) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *Runtime) Started() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.initialized
}

func (r *Runtime) Stopped() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopped
}

func (r *Runtime) Healthy() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status == StatusCreated || r.status == StatusRunning || r.status == StatusStopped
}

func (r *Runtime) LastError() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastErr
}

func (r *Runtime) Health() Health {
	r.mu.Lock()
	status := r.status
	started := r.initialized
	stopped := r.stopped
	lastErr := r.lastErr
	lastStartedAt := r.lastStartedAt
	lastStoppedAt := r.lastStoppedAt
	lastRunAt := r.lastRunAt
	r.mu.Unlock()

	health := Health{
		Status:          status,
		Started:         started,
		Stopped:         stopped,
		Healthy:         status == StatusCreated || status == StatusRunning || status == StatusStopped,
		PluginCount:     len(r.pluginMeta),
		CapabilityCount: len(r.Capabilities()),
		DuplicateCount:  len(r.DuplicateCapabilities()),
		Initialized:     started,
	}
	if lastErr != nil {
		health.LastError = lastErr.Error()
	}
	health.LastStartedAt = formatTimestamp(lastStartedAt)
	health.LastStoppedAt = formatTimestamp(lastStoppedAt)
	health.LastRunAt = formatTimestamp(lastRunAt)
	if !lastStartedAt.IsZero() {
		end := lastStoppedAt
		if started || end.IsZero() {
			end = time.Now()
		}
		if end.After(lastStartedAt) {
			health.Uptime = end.Sub(lastStartedAt).String()
		}
	}
	if r.sched != nil {
		health.DLQSize = r.sched.DLQSize()
		health.TaskSummary = summarizeTaskStates(r.sched.TaskStates())
		metrics := r.sched.Metrics()
		if metrics != nil {
			health.Metrics = MetricsSummary{
				Total:      metrics.Total(),
				QPS:        metrics.QPS(),
				AvgLatency: metrics.AvgLatency().String(),
				Fastest:    metrics.Fastest().String(),
				Slowest:    metrics.Slowest().String(),
			}
		}
	}
	return health
}

func (r *Runtime) markRun() {
	r.mu.Lock()
	r.lastRunAt = time.Now()
	r.mu.Unlock()
}

func summarizeTaskStates(states map[types.NodeID]types.TaskStatus) map[types.TaskStatus]int {
	summary := make(map[types.TaskStatus]int)
	for _, status := range states {
		summary[status]++
	}
	return summary
}

func formatTimestamp(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339Nano)
}

func (r *Runtime) publishRuntimeEvent(name string, status Status, errMsg string) {
	r.publishRuntimeEventTransition(name, "", status, errMsg)
}

func (r *Runtime) publishRuntimeEventTransition(name string, previous, current Status, errMsg string) {
	if r.bus == nil {
		return
	}
	r.bus.Publish(types.Event{
		Kind:      types.EventKindRuntime,
		Name:      name,
		Timestamp: time.Now(),
		Result: types.RuntimeLifecycleEvent{
			Name:      name,
			Status:    string(current),
			Previous:  string(previous),
			Error:     errMsg,
			Timestamp: time.Now(),
		},
	})
}
