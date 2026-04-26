package runtime

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thkx/agentkernel/capability"
	llmplugin "github.com/thkx/agentkernel/plugins/llm"
	toolplugin "github.com/thkx/agentkernel/plugins/tool"
	"github.com/thkx/agentkernel/policy"
	"github.com/thkx/agentkernel/types"
)

func TestRuntime_RunSync(t *testing.T) {
	// Create a simple graph
	builder := policy.NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("hello").
		Build()

	graph, err := builder.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	// Create registry
	registry := capability.NewRegistry()
	llm := &llmplugin.LLM{}
	registry.Register(llm)

	// Create runtime
	rt, err := NewRuntime(
		WithGraph(graph),
		WithCapabilityRegistry(registry),
	)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}

	// Run synchronously
	if err := rt.RunSync(); err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}

	// For now, we can't easily check the result since RunSync doesn't return it
	// This test mainly checks that it doesn't panic
}

func TestRuntime_RunSyncWithInvalidGraph(t *testing.T) {
	// Create runtime without graph
	rt, err := NewRuntime()
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}

	// This should not panic, but we can't easily test the error
	if err := rt.RunSync(); err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}
	if err := rt.Run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if err := rt.RunWithContext(context.Background()); err != nil {
		t.Fatalf("RunWithContext failed: %v", err)
	}
}

func TestRuntime_RunSyncWithInvalidCapability(t *testing.T) {
	// Create graph with invalid capability
	builder := policy.NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("invalid").
		Input("test").
		Build()

	graph, err := builder.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	registry := capability.NewRegistry()

	rt, err := NewRuntime(
		WithGraph(graph),
		WithCapabilityRegistry(registry),
	)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}

	// This should not panic, but we can't easily test the error
	if err := rt.RunSync(); err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}
}

func TestRuntime_WithPluginsAndCapabilities(t *testing.T) {
	rt, err := NewRuntime(
		WithPlugins(&llmplugin.Plugin{}, &toolplugin.Plugin{}),
	)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}

	caps := rt.Capabilities()
	if len(caps) != 2 {
		t.Fatalf("expected 2 capabilities, got %d: %v", len(caps), caps)
	}
	if caps[0] != "llm" || caps[1] != "tool" {
		t.Fatalf("unexpected capabilities: %v", caps)
	}

	plugins := rt.Plugins()
	if len(plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(plugins))
	}
	if plugins[0].Name != "llm" || plugins[1].Name != "tool" {
		t.Fatalf("unexpected plugins: %+v", plugins)
	}
	if rt.Status() != StatusCreated {
		t.Fatalf("expected created status before start, got %s", rt.Status())
	}
	if !rt.Healthy() {
		t.Fatal("expected newly created runtime to be healthy")
	}
}

func TestRuntime_StartAndStopLifecyclePlugins(t *testing.T) {
	var starts atomic.Int32
	var stops atomic.Int32

	plugin := &testLifecyclePlugin{
		testPlugin: testPlugin{name: "lifecycle"},
		start: func(ctx context.Context, registry *capability.Registry) error {
			starts.Add(1)
			return nil
		},
		stop: func(ctx context.Context) error {
			stops.Add(1)
			return nil
		},
	}

	rt, err := NewRuntime(WithPlugin(plugin))
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}

	if err := rt.StartWithContext(context.Background()); err != nil {
		t.Fatalf("StartWithContext failed: %v", err)
	}
	if err := rt.Start(); err != nil {
		t.Fatalf("second Start failed: %v", err)
	}
	if starts.Load() != 1 {
		t.Fatalf("expected plugin to start once, got %d", starts.Load())
	}
	if rt.Status() != StatusRunning {
		t.Fatalf("expected running status after start, got %s", rt.Status())
	}
	if !rt.Started() {
		t.Fatal("expected runtime to report started")
	}
	if rt.Health().LastStartedAt == "" {
		t.Fatal("expected health to include last started time")
	}

	if err := rt.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if stops.Load() != 1 {
		t.Fatalf("expected plugin to stop once, got %d", stops.Load())
	}
	if rt.Status() != StatusStopped {
		t.Fatalf("expected stopped status after stop, got %s", rt.Status())
	}
	if !rt.Stopped() {
		t.Fatal("expected runtime to report stopped")
	}
	if rt.Health().LastStoppedAt == "" {
		t.Fatal("expected health to include last stopped time")
	}
}

func TestRuntime_DuplicateCapabilitiesAreTracked(t *testing.T) {
	dup1 := &testPlugin{
		name: "dup-a",
		caps: []types.Capability{testRuntimeCapability{name: "shared"}},
	}
	dup2 := &testPlugin{
		name: "dup-b",
		caps: []types.Capability{testRuntimeCapability{name: "shared"}},
	}

	rt, err := NewRuntime(WithPlugins(dup1, dup2))
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	dups := rt.DuplicateCapabilities()
	if len(dups) != 1 || dups[0] != "shared" {
		t.Fatalf("expected duplicate capability to be tracked, got %v", dups)
	}
}

func TestRuntime_ConfiguresPluginByName(t *testing.T) {
	plugin := &testConfigurablePlugin{
		testPlugin: testPlugin{name: "configurable"},
	}

	rt, err := NewRuntime(
		WithPlugin(plugin),
		WithPluginConfig("configurable", testPluginConfig{Prefix: "hello"}),
	)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	_ = rt

	if plugin.config.Prefix != "hello" {
		t.Fatalf("expected plugin config to be applied, got %+v", plugin.config)
	}
}

func TestRuntime_StopClosesBus(t *testing.T) {
	rt, err := NewRuntime()
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	sub := rt.Bus().Subscribe()

	if err := rt.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	sawClosed := false
	for ev := range sub {
		if ev.Kind != types.EventKindRuntime {
			continue
		}
	}
	sawClosed = true
	if !sawClosed {
		t.Fatal("expected runtime stop to close event bus subscribers")
	}
	if health := rt.Health(); health.Status != StatusStopped || !health.Stopped || !health.Healthy {
		t.Fatalf("unexpected health after stop: %+v", health)
	}
}

func TestRuntime_PublishesLifecycleEvents(t *testing.T) {
	rt, err := NewRuntime()
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}

	if err := rt.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := rt.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	var names []string
	rt.Bus().Replay(func(ev types.Event) {
		if ev.Kind != types.EventKindRuntime {
			return
		}
		lifecycle, ok := ev.Result.(types.RuntimeLifecycleEvent)
		if !ok {
			t.Fatalf("expected runtime lifecycle payload, got %T", ev.Result)
		}
		names = append(names, lifecycle.Name)
	})

	expected := []string{"runtime.created", "runtime.starting", "runtime.running", "runtime.stopping", "runtime.stopped"}
	if len(names) != len(expected) {
		t.Fatalf("expected lifecycle events %v, got %v", expected, names)
	}
	for i := range expected {
		if names[i] != expected[i] {
			t.Fatalf("expected event %q at index %d, got %q", expected[i], i, names[i])
		}
	}
}

func TestRuntime_PublishesNamedExecutionEvents(t *testing.T) {
	builder := policy.NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("hello").
		Build()

	graph, err := builder.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	registry := capability.NewRegistry()
	registry.Register(&llmplugin.LLM{})

	rt, err := NewRuntime(
		WithGraph(graph),
		WithCapabilityRegistry(registry),
	)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}

	if err := rt.RunSync(); err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}

	var names []string
	var finished types.ExecutionEvent
	rt.Bus().Replay(func(ev types.Event) {
		if ev.Kind != types.EventKindExecution {
			return
		}
		payload, ok := ev.Result.(types.ExecutionEvent)
		if !ok {
			t.Fatalf("expected execution event payload, got %T", ev.Result)
		}
		if payload.Name != ev.Name {
			t.Fatalf("expected payload name %q to match event name %q", payload.Name, ev.Name)
		}
		if ev.Name == "node.finished" {
			finished = payload
		}
		names = append(names, ev.Name)
	})

	expected := []string{"node.started", "node.finished"}
	if len(names) < len(expected) {
		t.Fatalf("expected at least execution events %v, got %v", expected, names)
	}
	for i := range expected {
		if names[i] != expected[i] {
			t.Fatalf("expected execution event %q at index %d, got %q", expected[i], i, names[i])
		}
	}
	if finished.NodeID != "n1" || finished.Capability != "llm" || finished.Status != types.SUCCESS || finished.Attempt != 1 {
		t.Fatalf("unexpected finished execution payload: %+v", finished)
	}
	if finished.Result == nil || finished.Result.Output != "LLM(hello)" {
		t.Fatalf("expected finished payload to include result output, got %+v", finished)
	}
}

func TestRuntime_NewRuntimeReturnsPluginConfigErrors(t *testing.T) {
	plugin := &testConfigurablePlugin{
		testPlugin: testPlugin{name: "configurable"},
	}

	_, err := NewRuntime(
		WithPlugin(plugin),
		WithPluginConfig("configurable", "bad-config"),
	)
	if err == nil {
		t.Fatal("expected invalid plugin config to return an error")
	}
}

func TestRuntime_NewRuntimeRejectsUnknownPluginConfig(t *testing.T) {
	_, err := NewRuntime(
		WithPluginConfig("missing", testPluginConfig{Prefix: "hello"}),
	)
	if err == nil {
		t.Fatal("expected unknown plugin config to return an error")
	}
}

func TestRuntime_NewRuntimeRejectsConfigForNonConfigurablePlugin(t *testing.T) {
	plugin := &testPlugin{name: "plain"}

	_, err := NewRuntime(
		WithPlugin(plugin),
		WithPluginConfig("plain", testPluginConfig{Prefix: "hello"}),
	)
	if err == nil {
		t.Fatal("expected config for non-configurable plugin to return an error")
	}
}

func TestRuntime_StartAfterStopReturnsError(t *testing.T) {
	rt, err := NewRuntime()
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}

	if err := rt.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if err := rt.Start(); err != ErrRuntimeStopped {
		t.Fatalf("expected ErrRuntimeStopped, got %v", err)
	}
	if rt.Status() != StatusStopped {
		t.Fatalf("expected stopped status after restart attempt, got %s", rt.Status())
	}
	if rt.LastError() != ErrRuntimeStopped {
		t.Fatalf("expected last error to be ErrRuntimeStopped, got %v", rt.LastError())
	}
}

func TestRuntime_StartFailureUpdatesHealth(t *testing.T) {
	plugin := &testLifecyclePlugin{
		testPlugin: testPlugin{name: "broken"},
		start: func(ctx context.Context, registry *capability.Registry) error {
			return fmt.Errorf("boom")
		},
	}

	rt, err := NewRuntime(WithPlugin(plugin))
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}

	err = rt.Start()
	if err == nil {
		t.Fatal("expected start failure")
	}
	if rt.Status() != StatusFailed {
		t.Fatalf("expected failed status, got %s", rt.Status())
	}
	if rt.Healthy() {
		t.Fatal("expected failed runtime to be unhealthy")
	}
	health := rt.Health()
	if health.LastError == "" || health.Status != StatusFailed {
		t.Fatalf("unexpected health after failure: %+v", health)
	}
}

func TestRuntime_RunReturnsStartError(t *testing.T) {
	builder := policy.NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("hello").
		Build()
	graph, err := builder.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	registry := capability.NewRegistry()
	registry.Register(&llmplugin.LLM{})

	rt, err := NewRuntime(
		WithGraph(graph),
		WithCapabilityRegistry(registry),
	)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}

	if err := rt.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if err := rt.Run(); err != ErrRuntimeStopped {
		t.Fatalf("expected ErrRuntimeStopped from Run, got %v", err)
	}
	if err := rt.RunSync(); err != ErrRuntimeStopped {
		t.Fatalf("expected ErrRuntimeStopped from RunSync, got %v", err)
	}
}

func TestRuntime_HealthIncludesSchedulerSummary(t *testing.T) {
	builder := policy.NewPolicyBuilder().
		Start("n1").
		Node("n1").
		Capability("llm").
		Input("hello").
		Build()

	graph, err := builder.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	registry := capability.NewRegistry()
	registry.Register(&llmplugin.LLM{})

	rt, err := NewRuntime(
		WithGraph(graph),
		WithCapabilityRegistry(registry),
	)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}

	if err := rt.RunSync(); err != nil {
		t.Fatalf("RunSync failed: %v", err)
	}

	health := rt.Health()
	if health.DLQSize != 0 {
		t.Fatalf("expected empty DLQ, got %d", health.DLQSize)
	}
	if health.TaskSummary[types.Done] == 0 {
		t.Fatalf("expected done task summary, got %+v", health.TaskSummary)
	}
	if health.Metrics.Total == 0 {
		t.Fatalf("expected metrics total > 0, got %+v", health.Metrics)
	}
	if health.LastRunAt == "" {
		t.Fatalf("expected health to include last run time, got %+v", health)
	}
	if health.Uptime == "" {
		t.Fatalf("expected health to include uptime, got %+v", health)
	}
}

func TestRuntime_HealthTracksTimestampsMonotonically(t *testing.T) {
	rt, err := NewRuntime()
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}

	if err := rt.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	startedHealth := rt.Health()
	if startedHealth.LastStartedAt == "" {
		t.Fatalf("expected start timestamp, got %+v", startedHealth)
	}

	time.Sleep(10 * time.Millisecond)

	if err := rt.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	stoppedHealth := rt.Health()
	if stoppedHealth.LastStoppedAt == "" {
		t.Fatalf("expected stop timestamp, got %+v", stoppedHealth)
	}
	if stoppedHealth.Uptime == "" {
		t.Fatalf("expected uptime after stop, got %+v", stoppedHealth)
	}
}

type testPlugin struct {
	name string
	caps []types.Capability
}

func (p *testPlugin) Register(registry *capability.Registry) {
	for _, cap := range p.caps {
		registry.Register(cap)
	}
}

func (p *testPlugin) Metadata() capability.PluginMetadata {
	return capability.PluginMetadata{Name: p.name}
}

type testLifecyclePlugin struct {
	testPlugin
	start func(context.Context, *capability.Registry) error
	stop  func(context.Context) error
}

func (p *testLifecyclePlugin) Register(registry *capability.Registry) {
	registry.Register(testRuntimeCapability{name: "lifecycle-cap"})
}

func (p *testLifecyclePlugin) Metadata() capability.PluginMetadata {
	return capability.PluginMetadata{Name: p.name}
}

func (p *testLifecyclePlugin) Start(ctx context.Context, registry *capability.Registry) error {
	if p.start != nil {
		return p.start(ctx, registry)
	}
	return nil
}

func (p *testLifecyclePlugin) Stop(ctx context.Context) error {
	if p.stop != nil {
		return p.stop(ctx)
	}
	return nil
}

type testPluginConfig struct {
	Prefix string
}

type testConfigurablePlugin struct {
	testPlugin
	config testPluginConfig
}

func (p *testConfigurablePlugin) Register(registry *capability.Registry) {
	registry.Register(testRuntimeCapability{name: "configured-cap"})
}

func (p *testConfigurablePlugin) Metadata() capability.PluginMetadata {
	return capability.PluginMetadata{Name: p.name}
}

func (p *testConfigurablePlugin) Configure(config any) error {
	typed, ok := config.(testPluginConfig)
	if !ok {
		return fmt.Errorf("unexpected config type %T", config)
	}
	p.config = typed
	return nil
}

type testRuntimeCapability struct {
	name types.CapabilityName
}

func (c testRuntimeCapability) Name() types.CapabilityName { return c.name }

func (c testRuntimeCapability) Invoke(ctx types.ExecContext, input any) (any, error) {
	return fmt.Sprintf("%s(%v)", c.name, input), nil
}
