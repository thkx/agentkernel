# Policy 事件链路集成指南

## 概述

Policy 事件流已完全集成到 Event Model 中，形成从 **Policy 设计 → Runtime 执行** 的完整可观察链。

## 事件链路全图

```
┌─────────────────────────────────────────────────────────────────────┐
│                         POLICY EVENTS                               │
├─────────────────────────────────────────────────────────────────────┤
│ 1. policy.config_loaded ──┐                                         │
│    (PolicyLoader)         ├─→  Graph 构建阶段                       │
│                           │                                         │
│ 2. policy.graph_built ────┐                                         │
│    (BuilderPlanner)       ├─→  Graph 验证阶段                       │
│                           │                                         │
│ 3. policy.validated ──────┐                                         │
│    (PolicyValidator)      ├─→  引擎注册阶段                         │
│                           │                                         │
│ 4. policy.registered ─────┐                                         │
│    (PolicyEngine.Register)├─→  策略选择阶段                         │
│                           │                                         │
│ 5. policy.selected ───────┐                                         │
│    (SelectFirstStrategy)  ├─→  策略评估阶段                         │
│                           │                                         │
│ 6. policy.evaluated ──────┘                                         │
│    (PolicyEngine.Evaluate/Select)                                   │
└─────────────────────────────────────────────────────────────────────┘
                                    ↓
┌─────────────────────────────────────────────────────────────────────┐
│                        RUNTIME EVENTS                               │
├─────────────────────────────────────────────────────────────────────┤
│ • runtime.started                                                   │
│ • runtime.executing                                                 │
│ • runtime.completed                                                 │
│ • runtime.stopped                                                   │
└─────────────────────────────────────────────────────────────────────┘
                                    ↓
┌─────────────────────────────────────────────────────────────────────┐
│                       EXECUTION EVENTS                              │
├─────────────────────────────────────────────────────────────────────┤
│ • node.started                                                      │
│ • node.capability_invoked                                           │
│ • node.completed                                                    │
│ • node.failed (retry)                                               │
│ • node.succeeded                                                    │
└─────────────────────────────────────────────────────────────────────┘
```

## Policy 事件详细说明

### 1. policy.config_loaded / policy.config_load_failed
**发起者**: `PolicyLoader`
```go
loader := policy.NewPolicyLoader().WithBus(bus)
config, err := loader.LoadFromJSON("policy.json")
// Events published:
// - policy.config_loaded (成功)
// - policy.config_load_failed (失败)
```

**事件载荷**:
```
PolicyEvent {
  Name:       "policy.config_loaded/failed",
  PolicyName: "workflow_v1",
  Source:     "json_file" | "yaml_file" | "json_string" | "yaml_string",
  Valid:      true/false,
  NodeCount:  3,
  Error:      (if failed),
  Metadata:   {"filename": "policy.json"}
}
```

### 2. policy.graph_built / policy.graph_build_failed
**发起者**: `BuilderPlanner`
```go
builder := policy.NewPolicyBuilder()...
planner := policy.NewBuilderPlanner(builder).WithBus(bus)
graph, err := planner.Build(inputData)
// Events:
// - policy.graph_built (成功)
// - policy.graph_build_failed (失败)
```

**事件载荷**:
```
PolicyEvent {
  Name:      "policy.graph_built",
  Source:    "builder_planner",
  Valid:     true,
  NodeCount: 5,       // 图中的节点数
  Metadata:  {"input_provided": true}
}
```

### 3. policy.validated / policy.validation_failed
**发起者**: `PolicyValidator`
```go
validator := policy.NewPolicyValidator(registry).WithBus(bus)
result := validator.ValidateGraph(graph, registry)
// Events:
// - policy.validated (所有检查通过)
// - policy.validation_failed (发现错误/警告)
```

**事件载荷**:
```
PolicyEvent {
  Name:     "policy.validated",
  Source:   "validator_graph",
  Valid:    true,
  NodeCount: 5,
  Warnings: 0,
  Errors:   0
}
```

### 4. policy.registered
**发起者**: `PolicyEngine.Register()`
```go
engine := policy.NewPolicyEngine().WithBus(bus)
policy_obj := policy.NewConfigDrivenPolicy("workflow", config)
err := engine.Register(policy_obj)
// Event: policy.registered
```

**事件载荷**:
```
PolicyEvent {
  Name:       "policy.registered",
  PolicyName: "workflow",
  Source:     "policy_engine",
  Valid:      true
}
```

### 5. policy.selected
**发起者**: `SelectFirstStrategy` 或 `PolicyEngine.Select()`
```go
plan, err := engine.Evaluate(ctx, planCtx)
// Events:
// - policy.selected (选择成功)
// - policy.select_failed (选择失败)
```

**事件载荷**:
```
PolicyEvent {
  Name:       "policy.selected",
  PolicyName: "workflow",
  Source:     "select_first_strategy" | "policy_engine",
  Valid:      true,
  Metadata:   {"strategy": "select_first"}
}
```

### 6. policy.evaluated / policy.evaluated_failed
**发起者**: `PolicyEngine.Evaluate()` 或 `PolicyEngine.Select()`
```go
plan, err := engine.Evaluate(ctx, planCtx)
// Events:
// - policy.evaluated (评估成功)
// - policy.evaluated_failed (评估失败)
```

**事件载荷**:
```
PolicyEvent {
  Name:       "policy.evaluated",
  PolicyName: "workflow",
  Source:     "select_first_strategy",
  Valid:      true,
  NodeCount:  5,
  Metadata:   {"start_node": "analyze"}
}
```

## API 使用指南

### 启用 Policy 事件

所有 Policy 组件都支持通过 `WithBus()` 方法启用事件：

```go
// 1. Loader
loader := policy.NewPolicyLoader().WithBus(bus)
config, _ := loader.LoadFromJSON("policy.json")

// 2. BuilderPlanner
planner := policy.NewBuilderPlanner(builder).WithBus(bus)
graph, _ := planner.Build(nil)

// 3. Validator
validator := policy.NewPolicyValidator(registry).WithBus(bus)
result := validator.ValidateGraph(graph, registry)

// 4. PolicyEngine
engine := policy.NewPolicyEngine().WithBus(bus)
engine.Register(policy_obj)
plan, _ := engine.Evaluate(ctx, planCtx)
```

### 完整集成示例

```go
// 创建共享事件总线
eventStore := event.NewInMemoryEventStore()
bus := event.NewSourcingBus(eventStore)

// Policy 流程
loader := policy.NewPolicyLoader().WithBus(bus)
config, _ := loader.LoadFromJSON("policy.json")

builder, _ := policy.FromConfig(config)
planner := policy.NewBuilderPlanner(builder).WithBus(bus)
graph, _ := planner.Build(nil)

validator := policy.NewPolicyValidator(registry).WithBus(bus)
validator.ValidateGraph(graph, registry)

engine := policy.NewPolicyEngine().WithBus(bus)
engine.Register(policy.NewConfigDrivenPolicy("my_policy", config))
plan, _ := engine.Evaluate(ctx, planCtx)

// Runtime 使用相同的事件总线
runtime, _ := runtime.NewRuntime(
  runtime.WithGraph(plan.Graph),
  runtime.WithEventBus(bus),
  runtime.WithCapabilityRegistry(registry),
)
runtime.Run()

// 所有事件在同一个时间线上
bus.Replay(func(ev types.Event) {
  // POLICY → RUNTIME → EXECUTION 完整链路
})
```

## 错误处理事件

所有 Policy 操作都发送相应的失败事件：

| 操作 | 失败事件 |
|------|---------|
| LoadFromJSON | `policy.config_load_failed` |
| BuildGraph | `policy.graph_build_failed` |
| ValidateGraph | `policy.validation_failed` |
| Select | `policy.select_failed` |
| Evaluate | `policy.evaluated_failed` |

## 事件存储和回放

```go
// 存储
eventStore := event.NewInMemoryEventStore()
bus := event.NewSourcingBus(eventStore)

// 所有事件自动存储
// ...执行 policy 和 runtime 操作...

// 回放
bus.Replay(func(ev types.Event) {
  if ev.Kind == types.EventKindPolicy {
    payload := ev.Result.(types.PolicyEvent)
    // 分析 policy 事件
  }
})

// 获取完整时间线
timeline := eventStore.Timeline()  // ExecutionTimelineEntry
```

## 事件关联

所有事件共享一致性属性：

- **TraceID**: 跨越 Policy → Runtime → Execution 的全局追踪 ID
- **SpanID**: 单个操作的 Span ID
- **Timestamp**: 毫秒级精度时间戳
- **Kind**: 事件分类 (POLICY / RUNTIME / EXECUTION)
- **Name**: 具体事件名称

## 观测和监控

使用事件流实现端到端观测：

```go
// 1. 实时订阅
sub := bus.Subscribe()
go func() {
  for ev := range sub {
    log.Printf("[%s] %s @ %s\n", ev.Kind, ev.Name, ev.Timestamp)
  }
}()

// 2. 历史回放
bus.Replay(func(ev types.Event) {
  // 生成审计日志
  // 生成指标
  // 生成追踪
})

// 3. 快照
snapshot := bus.Store().Snapshot()
// 恢复系统状态
```

## 最佳实践

1. **及早注册 Bus**: 在创建任何 Policy 组件前注册事件总线
2. **共享 Bus**: Policy、Runtime、Execution 使用同一个 EventBus
3. **异步处理**: 在 goroutine 中订阅和处理事件
4. **错误处理**: 监听 `_failed` 事件进行错误处理
5. **可观测性**: 结合使用 Replay 和 Timeline 进行完整的审计

---

**参考示例**: `examples/policy_event_chain_example.go`
