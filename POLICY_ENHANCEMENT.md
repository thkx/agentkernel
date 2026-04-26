# Policy 增强 - 完整指南

## 📋 概述

与原始的硬编码 Policy 不同，现在支持了以下关键特性：

1. **配置驱动的 DAG** (Config-Driven)
2. **流畅 API 动态构建** (Fluent Builder API)
3. **条件表达式求值** (Condition Expression Evaluation)
4. **策略验证** (Policy Validation)
5. **运行时 DAG 修改** (Runtime Graph Modification)
6. **输入参数化** (Input Parameterization)

---

## 🏗️ 核心组件

### 1. PolicyConfig （`config.go`）

定义了策略配置的完整数据结构：

```go
type PolicyConfig struct {
    Version     string                    // 版本号
    Name        string                    // 策略名称
    Description string                    // 描述
    Start       string                    // 起始节点 ID
    Nodes       map[string]*NodeConfig    // 节点配置
    Global      *GlobalSettings           // 全局设置
    Metadata    map[string]interface{}    // 元数据
}
```

**特点**：
- 支持 JSON/YAML 格式
- 完整的节点配置（优先级、超时、限流、重试策略等）
- 全局设置供所有节点继承
- 边的条件表达式支持

### 2. PolicyBuilder （`builder.go`）

流畅的链式 API，用于动态构建 DAG：

```go
builder := NewPolicyBuilder().
    Start("n1").
    Node("n1").
        Capability("llm").
        Input("task").
        Priority(10).
        NextNode("n2").
        Build().
    Node("n2").
        Capability("tool").
        Input("execute").
        Build()

graph := builder.BuildGraph()
```

**关键功能**：
- `Start(id)`：设置起始节点
- `Node(id)`：创建新节点
- `Capability(name)`：设置能力
- `Input(data)`：设置输入
- `NextNode(id)`：添加简单边
- `NextConditional(id, condition)`：添加条件边
- `Priority(p)` / `Tenant(t)` / `Timeout(d)` / `RateLimit(n)`：配置节点
- `CloneNode(source, target)`：克隆节点
- `InsertNode(before, new, after, ...)`：插入节点
- `RemoveNode(id)`：删除节点
- `UpdateNodeInput(id, input)`：更新节点输入
- `UpdateNodeCapability(id, cap)`：更新节点能力

### 3. ConditionEvaluator （`evaluator.go`）

强大的条件表达式求值器：

```go
evaluator := NewConditionEvaluator()
condition, err := evaluator.BuildCondition("status == 'SUCCESS'")
if err == nil {
    result := condition(types.Result{...})
}
```

**支持的操作符**：
- 比较：`==`, `!=`, `<`, `>`, `<=`, `>=`
- 字符串：`contains`, `startswith`, `endswith`
- 集合：`in` (检查是否在列表中)

**表达式示例**：
```
status == 'SUCCESS'
output.contains('yes')
attempt > 2
code in ['200', '201']
error != ''
```

**特殊字段引用**：
- `status`：执行状态 (SUCCESS/FAILED)
- `output`：输出结果
- `error`：错误消息
- `control`：控制信号
- `attempt`：重试次数
- `retryable`：是否可重试

### 4. PolicyValidator （`validator.go`）

验证 DAG 是否有效：

```go
validator := NewPolicyValidator(registry)
result := validator.ValidateConfig(config, registry)

if !result.Valid {
    for _, err := range result.Errors {
        fmt.Printf("[%s] %s (node: %s)\n", err.Level, err.Message, err.NodeID)
    }
}
```

**验证项**：
- ✓ 起始节点存在
- ✓ 所有边的目标节点存在
- ✓ 能力在注册表中存在
- ✓ 检测无法到达的节点 (警告)
- ✓ 检测环路 (可能是意图的循环)
- ✓ 条件表达式语法有效
- ✓ 重试配置合理

### 5. PolicyLoader （`loader.go`）

从文件加载或保存配置：

```go
loader := NewPolicyLoader()

// 从 JSON 加载
config, err := loader.LoadFromJSON("policy.json")

// 从 YAML 加载
config, err := loader.LoadFromYAML("policy.yaml")

// 从字符串加载
config, err := loader.LoadFromJSONString(jsonStr)

// 保存配置
loader.SaveToJSON(config, "policy_out.json")
```

### 6. 新的 Planner 实现 （`policy.go`）

扩展了原有的 Planner，新增了两个特化实现：

```go
// 1. 从配置创建
planner := NewConfigPlanner(config)
// 或从文件加载
planner, err := NewConfigPlannerFromFile("policy.json", false)

// 2. 从 Builder 创建
builder := NewPolicyBuilder()...
planner := NewBuilderPlanner(builder)

// 验证
validation := planner.Validate(registry)
if !validation.Valid {
    fmt.Println(validation.GetValidationSummary())
}
```

---

## 📝 使用示例

### 示例 1：从 JSON 配置加载

```go
// 1. 加载配置
loader := policy.NewPolicyLoader()
config, err := loader.LoadFromJSON("workflow.json")
if err != nil {
    log.Fatal(err)
}

// 2. 创建 Planner
planner := policy.NewConfigPlanner(config)

// 3. 验证
validator := policy.NewPolicyValidator(registry)
result := validator.ValidateConfig(config, registry)
if !result.Valid {
    log.Fatal(result.GetValidationSummary())
}

// 4. 在 Runtime 中使用
rt, err := runtime.NewRuntime(
    runtime.WithPlanner(planner),
    runtime.WithCapabilityRegistry(registry),
)
if err != nil {
    panic(err)
}
if err := rt.Run(); err != nil {
    panic(err)
}
```

### 示例 2：使用 Fluent API 动态构建

```go
builder := policy.NewPolicyBuilder().
    Start("analyze").
    Node("analyze").
        Capability("llm").
        Input("Analyze user request").
        Priority(10).
        NextConditional("simple_task", func(r types.Result) bool {
            return strings.Contains(r.Output.(string), "simple")
        }).
        NextConditional("complex_task", func(r types.Result) bool {
            return strings.Contains(r.Output.(string), "complex")
        }).
        Build().
    Node("simple_task").
        Capability("tool").
        Input("Execute task directly").
        Build().
    Node("complex_task").
        Capability("tool").
        Input("Decompose and execute").
        Build()

planner := policy.NewBuilderPlanner(builder)
rt, err := runtime.NewRuntime(
    runtime.WithPlanner(planner),
    runtime.WithCapabilityRegistry(registry),
)
if err != nil {
    panic(err)
}
if err := rt.Run(); err != nil {
    panic(err)
}
```

### 示例 3：运行时修改 DAG

```go
builder := policy.NewPolicyBuilder()...

// 修改现有节点
builder.UpdateNodeInput("n1", "new input")
builder.UpdateNodeCapability("n2", "new_capability")

// 克隆节点
builder.CloneNode("n2", "n2_retry")

// 插入新节点
builder.InsertNode("n1", "validate", "n2", "tool", "Validate")

// 移除节点（如果它没有被引用）
builder.RemoveNode("old_node")

// 构建并验证
planner := policy.NewBuilderPlanner(builder)
validation := planner.Validate(registry)
```

### 示例 4：条件表达式

```go
evaluator := policy.NewConditionEvaluator()

// 基本比较
cond1, _ := evaluator.BuildCondition("status == 'SUCCESS'")

// 字符串操作
cond2, _ := evaluator.BuildCondition("output.contains('yes')")

// 数字比较
cond3, _ := evaluator.BuildCondition("attempt > 2")

// 集合检查
cond4, _ := evaluator.BuildCondition("code in ['200', '201', '202']")

// 使用条件函数
result := cond1(types.Result{Status: types.SUCCESS})  // true
```

---

## 📄 JSON 配置文件格式

完整示例见 `examples/policy_config.json`

```json
{
  "version": "1.0",
  "name": "agent-workflow",
  "description": "Complex agent workflow",
  "start": "analyze",
  "global": {
    "default_timeout": "30s",
    "default_rate_limit": 100,
    "tenant_aware": true,
    "variables": {
      "max_retries": 3
    }
  },
  "nodes": {
    "analyze": {
      "id": "analyze",
      "capability": "llm",
      "input": "Analyze request",
      "output_var": "analysis",
      "config": {
        "priority": 10,
        "timeout": "15s",
        "rate_limit": 50
      },
      "edges": [
        {
          "to": "execute",
          "condition": "status == 'SUCCESS'",
          "priority": 1
        },
        {
          "to": "error_handler",
          "condition": "status == 'FAILED'",
          "priority": 2
        }
      ]
    },
    "execute": {
      "id": "execute",
      "capability": "tool",
      "input": "Execute task",
      "edges": []
    },
    "error_handler": {
      "id": "error_handler",
      "capability": "tool",
      "input": "Handle error",
      "config": {
        "continue_on_error": true,
        "retry": {
          "max_attempts": 3,
          "initial_backoff": "100ms",
          "max_backoff": "10s",
          "multiplier": 2.0
        }
      },
      "edges": []
    }
  }
}
```

---

## 🔍 与旧 Planner 的对比

| 特性 | 旧 Planner | 新 ConfigPlanner | 新 BuilderPlanner |
|------|-----------|-----------------|------------------|
| 硬编码模式 | ✓ (4种预设) | ✗ | ✗ |
| 配置文件 | ✗ | ✓ (JSON/YAML) | ✗ |
| 动态构建 | ✗ | ✗ | ✓ |
| 条件表达式 | ✗ (函数) | ✓ (字符串解析) | ✗ (函数) |
| 运行时修改 | ✗ | ✗ | ✓ |
| 验证支持 | ✗ | ✓ (完整) | ✓ (完整) |
| 参数化 | ✗ | ✓ (全局变量) | ✗ |

---

## 🧪 测试

所有新功能都有对应的测试用例在 `policy_test.go`：

```bash
go test ./policy -v
```

关键测试：
- ✓ Builder 基本功能
- ✓ 条件边处理
- ✓ 节点克隆和插入
- ✓ 条件求值
- ✓ 配置加载
- ✓ 策略验证
- ✓ 错误处理

---

## ⚠️ 限制和未来改进

### 当前限制
1. 条件表达式不支持复杂的逻辑（如 `AND`、`OR`）
2. 输入参数化目前只支持简单的变量替换
3. 没有可视化工具来编辑 DAG

### 未来改进方向
1. 支持更复杂的表达式语言 (Lua/JavaScript)
2. 可视化编辑器 (Web UI)
3. 策略版本管理和差异对比
4. 热重载支持（不重启运行时修改策略）
5. 策略模板和继承

---

## 📚 参考资源

- `examples/policy_config.json` - 完整的配置示例
- `examples/policy_enhanced_main.go` - 使用示例代码
- `policy_test.go` - 单元测试
- 原始文档：查看 `policy/README.md`

---

## 🎯 总结

Policy 增强使得 AgentKernel 从硬编码的预设策略进化为：
- **配置驱动**的灵活方案（适合运维和配置管理）
- **编程构建**的动态方案（适合需要大量自定义的场景）
- **完整验证**的安全方案（在执行前捕获错误）

这为生产环境中的策略管理提供了强大和灵活的工具。
