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

用于：

* Debug
* 审计
* 可视化

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
| **v0.6** | 可观测性        | Observability               | 1. Trace（TraceID / Span）<br>2. Structured Logging<br>3. Metrics（QPS / latency）<br>4. Execution Timeline（执行时间线）<br>5. Debug Replay UI<br>6. Hook System（before/after exec）                           | 达到“可调试、可分析”              |

