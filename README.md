# Agent System（V7）

一个**可控执行系统（Controlled Execution System）**，用于统一调度：

* LLM
* Tools
* Workflows
* External systems

---

# 1. 核心定义

```text id="core_def"
Agent = Runtime + Capability + Policy + UI
```

但关键是：

> ❗所有能力必须被 Runtime 约束执行

---

# 2. 设计核心原则

---

## 🔴 Principle 1：Runtime is the only executor

```text id="p1"
只有 Runtime 可以执行任何任务
```

---

## 🔴 Principle 2：LLM is NOT an agent

LLM：

* ❌ 不控制流程
* ❌ 不循环调用
* ❌ 不持有状态

👉 只做：

```text id="llm_role"
单次决策函数
```

---

## 🔴 Principle 3：Capability is stateless function

Tool / LLM：

```text id="cap"
input → output
```

无控制权

---

## 🔴 Principle 4：Policy only decides, never executes

Policy：

* 生成 Plan
* 选择 Capability

❌ 不执行

Policy 增强，可参考 [POLICY_ENHANCEMENT.md](./POLICY_ENHANCEMENT.md)。
---

## 🔴 Principle 5：UI is event consumer only

UI：

* 订阅 EventBus
* 不参与执行

---

# 3. 执行模型（核心）

```text id="exec_model"
User Input
   ↓
Policy (Plan)
   ↓
Runtime Scheduler
   ↓
Executor
   ↓
Capability (LLM / Tool)
   ↓
Result
   ↓
State Update
   ↓
EventBus → UI
```

---

# 4. Result 控制流模型

```go id="result"
type Result struct {
    Output any

    Status ExecStatus

    Control ControlSignal   // NEXT / REPEAT / STOP / JUMP

    NextNode string

    Error error

    Meta map[string]any
}
```

---

# 5. Scheduler（核心心脏）

Scheduler 是：

> 状态机驱动执行引擎

能力包括：

* Task Queue
* Worker Pool
* Retry
* Backoff
* ACK
* FSM

---

# 6. LLM 强约束模型（关键）

LLM：

```text id="llm_rule"
必须是单次调用能力（One-shot function）
```

限制：

* ❌ 不允许循环调用
* ❌ 不允许控制 flow
* ❌ 不允许递归工具调用

---

# 7. Capability 系统

所有能力统一接口：

```go id="cap"
Invoke(ctx, Request) → Response
```

分类：

* LLM Capability
* Tool Capability
* System Capability

---

# 8. UI 系统（完全解耦）

UI 基于事件流：

```text id="ui_flow"
Runtime → Hook → EventBus → UI
```

UI 类型：

* Web UI
* CLI UI
* Debug UI

---

# 9. State 系统

支持：

* Snapshot
* Diff
* Replay
* Shared runtime state accessors

用于：

* Debug
* 审计
* 可视化

状态访问约定：

* `ExecContext.State` / `HookContext.State` 是只读快照，用于调试和观察
* 共享状态读写统一走 `GetState` / `SetState` / `DeleteState`
* 需要完整副本时，使用 `SnapshotState()`

示例：

```go
func (c *MyCapability) Invoke(ctx types.ExecContext, input any) (any, error) {
    retries := ctx.GetState("retries")
    _ = retries

    ctx.SetState("last_input", input)
    ctx.DeleteState("temporary")

    snapshot := ctx.SnapshotState()
    _ = snapshot

    return input, nil
}
```

Hook 也遵循同样的规则：

```go
rt := runtime.NewRuntime(
    runtime.WithAfterWritableHook(func(ctx types.WritableHookContext) {
        ctx.SetState("last_node", string(ctx.NodeID))
    }),
)
```

默认情况下，hook 只拿到只读状态访问能力；只有通过 `WithBeforeWritableHook` / `WithAfterWritableHook` 注册并接收 `types.WritableHookContext` 的 hook，写入才会落到共享状态。普通 `HookContext` 不再提供写方法。

---

# 10. 系统定位

这是一个：

> 🔥 Controlled Execution Kernel（受控执行内核）

不是：

* ❌ Agent应用
* ❌ LLM SDK

---

# 11. 对标系统

* Temporal
* Kubernetes
* LangChain

---

# 12. 最终目标

构建一个：

> 可控、可恢复、可观测的 AI 执行基础设施


| 版本       | 核心主题        | 关键能力                        | 需要实现的条目                                                                                                                                                                                             | 目标效果                     |
| -------- | ----------- | --------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------ |
| **v0.7** | 调度系统升级      | Advanced Scheduler          | 1. Priority Queue（优先级调度）<br>2. Backpressure（限流）<br>3. Rate Limit（能力级限流）<br>4. Timeout 控制<br>5. Circuit Breaker（熔断）<br>6. 多队列调度（multi-tenant）                                                        | 达到“生产级调度器”               |
