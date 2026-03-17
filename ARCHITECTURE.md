# Blades 架构设计文档

> **Blades** 是一个基于 Go 语言构建的多模态 AI Agent 框架，提供灵活的 Agent 编排、工具调用、工作流引擎等能力。

## 目录

- [1. 整体架构](#1-整体架构)
  - [1.1 架构愿景](#11-架构愿景)
  - [1.2 设计原则](#12-设计原则)
  - [1.3 架构图](#13-架构图)
  - [1.4 核心组件概览](#14-核心组件概览)
- [2. 核心模块设计](#2-核心模块设计)
  - [2.1 Agent 模块](#21-agent-模块)
  - [2.2 ModelProvider 模块](#22-modelprovider-模块)
  - [2.3 Tool 模块](#23-tool-模块)
  - [2.4 Runner 模块](#24-runner-模块)
  - [2.5 Middleware 模块](#25-middleware-模块)
  - [2.6 Session & Memory 模块](#26-session--memory-模块)
  - [2.7 Flow 工作流模块](#27-flow-工作流模块)
  - [2.8 Graph 图引擎模块](#28-graph-图引擎模块)
  - [2.9 Recipe 配方系统](#29-recipe-配方系统)
  - [2.10 Skills 技能系统](#210-skills-技能系统)
- [3. 模块间关系和调用流程](#3-模块间关系和调用流程)
  - [3.1 模块依赖关系](#31-模块依赖关系)
  - [3.2 典型调用流程](#32-典型调用流程)
  - [3.3 数据流分析](#33-数据流分析)
- [4. 关键技术选型和设计理念](#4-关键技术选型和设计理念)
  - [4.1 Go 语言特性利用](#41-go-语言特性利用)
  - [4.2 设计模式应用](#42-设计模式应用)
  - [4.3 扩展性设计](#43-扩展性设计)
  - [4.4 错误处理策略](#44-错误处理策略)

---

## 1. 整体架构

### 1.1 架构愿景

Blades 的目标是提供一个**Go 语言原生的 AI Agent 框架**，让 Go 开发者能够以熟悉的编程范式构建复杂的 AI 应用。框架的设计灵感来源于游戏《战神》中奎托斯的标志性武器——混沌之刃，象征着强大的执行力和灵活性。

**核心定位：**
- 多模态 AI 交互（文本、图像、音频）
- 可组合的 Agent 编排
- 声明式工作流定义
- 插件化的模型和工具支持

### 1.2 设计原则

| 原则 | 说明 |
|------|------|
| **Go Idiomatic** | 完全遵循 Go 的设计哲学，代码风格符合 Go 开发者的习惯 |
| **接口抽象** | 通过统一的接口（Agent、ModelProvider、Tool）实现高度解耦 |
| **组合优于继承** | 通过组合不同的组件构建复杂功能，而非继承层次 |
| **函数式选项** | 使用 Functional Options 模式提供灵活的配置方式 |
| **流式处理** | 基于 Go 1.23+ 的 iter.Seq2 实现生成器模式，支持流式输出 |
| **中间件机制** | 借鉴 Web 框架的中间件设计，提供横切关注点的统一处理 |

### 1.3 架构图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Application Layer                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Recipe    │  │    Flow     │  │    Graph    │  │   Direct Agent      │ │
│  │  (YAML 配置) │  │  (工作流)   │  │  (图引擎)   │  │     使用            │ │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘ │
├─────────┼────────────────┼────────────────┼────────────────────┼────────────┤
│         │                │                │                    │            │
│  ┌──────▼────────────────▼────────────────▼────────────────────▼──────────┐ │
│  │                         Runner (执行入口)                               │ │
│  │  ┌─────────────────────────────────────────────────────────────────┐   │ │
│  │  │                    Session & Memory (会话与记忆)                  │   │ │
│  │  └─────────────────────────────────────────────────────────────────┘   │ │
│  └─────────────────────────────────────────────────────────────────────────┘ │
│         │
│  ┌──────▼────────────────────────────────────────────────────────────────┐  │
│  │                         Agent (核心抽象)                               │  │
│  │  ┌───────────────┐  ┌───────────────┐  ┌───────────────────────────┐ │  │
│  │  │   Middleware  │  │     Tools     │  │       Skills              │ │  │
│  │  │   (中间件链)   │  │    (工具调用)  │  │      (技能系统)           │ │  │
│  │  └───────────────┘  └───────────────┘  └───────────────────────────┘ │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│         │
│  ┌──────▼────────────────────────────────────────────────────────────────┐  │
│  │                      ModelProvider (模型抽象层)                         │  │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────────────┐  │  │
│  │  │  OpenAI   │  │  Anthropic│  │  Gemini   │  │   Custom Provider │  │  │
│  │  └───────────┘  └───────────┘  └───────────┘  └───────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│         │
│  ┌──────▼────────────────────────────────────────────────────────────────┐  │
│  │                      External Services (外部服务)                       │  │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────────────┐  │  │
│  │  │    MCP    │  │  Database │  │   Files   │  │   Third-party API │  │  │
│  │  └───────────┘  └───────────┘  └───────────┘  └───────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.4 核心组件概览

| 组件 | 职责 | 关键接口/结构 |
|------|------|--------------|
| **Agent** | AI 代理核心抽象，执行任务的基本单元 | `Agent` 接口 |
| **ModelProvider** | LLM 模型适配器，统一不同模型的 API | `ModelProvider` 接口 |
| **Tool** | 外部能力封装，扩展 Agent 能力边界 | `tools.Tool` 接口 |
| **Runner** | Agent 执行入口，管理会话和上下文 | `Runner` 结构体 |
| **Middleware** | 横切关注点处理（日志、监控、限流） | `Middleware` 类型 |
| **Session** | 会话状态和历史管理 | `Session` 接口 |
| **Memory** | 长期记忆存储和检索 | `Memory` 接口 |
| **Flow** | 工作流编排（顺序、并行、循环、路由） | `SequentialAgent`, `ParallelAgent`, `LoopAgent`, `RoutingAgent` |
| **Graph** | 有向图执行引擎，复杂工作流编排 | `Graph`, `Executor` |
| **Recipe** | 声明式 YAML 配置系统 | `AgentSpec`, `Build` |
| **Skills** | 技能包系统，可组合的工具集合 | `Skill`, `Toolset` |

---

## 2. 核心模块设计

### 2.1 Agent 模块

#### 2.1.1 核心接口

```go
// Agent 表示一个自主代理，能够处理调用并生成消息序列
type Agent interface {
    Name() string                                    // 返回代理名称
    Description() string                             // 返回功能描述
    Run(context.Context, *Invocation) Generator[*Message, error]
}
```

#### 2.1.2 设计要点

**1. 统一的执行协议**

所有可执行组件（基础 Agent、Flow 组合 Agent、Graph 节点）都实现 `Agent` 接口，这使得它们可以：
- 无缝组合和嵌套
- 被 Runner 统一调度
- 作为工具被其他 Agent 调用

**2. 生成器模式**

`Run` 方法返回 `Generator[*Message, error]`（即 `iter.Seq2[*Message, error]`），这是一种惰性迭代器：
- 支持流式输出，调用者可以实时接收消息
- 支持早期终止，调用者可以中途退出
- 简化并发控制，生产者 - 消费者模式自然解耦

**3. Invocation 上下文**

`Invocation` 结构体携带完整的执行上下文：
```go
type Invocation struct {
    ID          string          // 调用唯一标识
    Model       string          // 使用的模型名称
    Resume      bool            // 是否从历史恢复
    Stream      bool            // 是否流式模式
    Session     Session         // 会话对象
    Instruction *Message        // 系统指令
    Message     *Message        // 用户消息
    Tools       []tools.Tool    // 可用工具列表
}
```

#### 2.1.3 内部实现结构

```go
type agent struct {
    name                string
    description         string
    instruction         string                // 静态指令
    instructionProvider InstructionProvider   // 动态指令生成器
    outputKey           string                // 输出存储键
    maxIterations       int                   // 最大迭代次数
    model               ModelProvider         // 模型提供者
    inputSchema         *jsonschema.Schema    // 输入 Schema
    outputSchema        *jsonschema.Schema    // 输出 Schema
    middlewares         []Middleware          // 中间件链
    tools               []tools.Tool          // 静态工具
    skills              []skills.Skill        // 技能列表
    skillToolset        *skills.Toolset       // 技能工具集
    toolsResolver       tools.Resolver        // 动态工具解析器
}
```

#### 2.1.4 执行流程

```
┌─────────────────────────────────────────────────────────────────┐
│                     Agent.Run() 执行流程                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. findResumeMessages() ────► 检查是否可恢复 ──► 可恢复？      │
│     │                                              │           │
│     │ 否                                           │ 是        │
│     ▼                                              ▼           │
│  2. prepareInvocation()                       直接返回历史     │
│     │  - 解析工具（静态 + 动态）                      消息      │
│     │  - 设置模型名称                                  │       │
│     │  - 合并指令（优先级处理）                       │        │
│     │                                                │         │
│  3. NewAgentContext() ◄─────────────────────────────┘         │
│     │ 注入 Agent 到上下文                                      │
│     │                                                          │
│  4. 构建 Handler                                                 │
│     │  - 创建 ModelRequest                                      │
│     │  - 设置消息历史                                           │
│     │                                                          │
│  5. ChainMiddlewares()                                          │
│     │ 应用中间件链（洋葱模型）                                   │
│     │                                                          │
│  6. handler.Handle() ──► handle() 主循环                        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

handle() 主循环（处理工具调用迭代）:
┌──────────────────────────────────────────────────────────────┐
│  for i := 0; i < maxIterations; i++ {                        │
│      │                                                       │
│      ├─► ContextManager.Prepare() 管理上下文窗口             │
│      │                                                       │
│      ├─► Model.Generate() / NewStreaming() 调用模型          │
│      │                                                       │
│      ├─► 处理响应消息                                        │
│      │   │                                                   │
│      │   ├─► Role == RoleTool ?                              │
│      │   │   │                                               │
│      │   │   ├─► 是：executeTools() 并发执行工具              │
│      │   │   │        req.Messages = append(toolMessage)     │
│      │   │   │        continue (下一次迭代)                   │
│      │   │   │                                               │
│      │   │   └─► 否：return (完成)                           │
│      │   │                                                   │
│      │   └─► saveOutputState() 保存输出到会话                 │
│      │                                                       │
│      └─► 达到 maxIterations ──► ErrMaxIterationsExceeded     │
│  }                                                           │
└──────────────────────────────────────────────────────────────┘
```

#### 2.1.5 功能选项

```go
// 核心配置选项
WithModel(model ModelProvider)           // 必需：设置模型提供者
WithInstruction(instruction string)      // 设置静态系统指令
WithInstructionProvider(provider)        // 设置动态指令生成器
WithTools(tools...)                      // 设置工具列表
WithSkills(skills...)                    // 设置技能列表
WithMiddleware(ms...)                    // 设置中间件链
WithMaxIterations(n int)                 // 设置最大迭代次数（默认 10）

// Schema 约束
WithInputSchema(schema *jsonschema.Schema)   // 输入 Schema
WithOutputSchema(schema *jsonschema.Schema)  // 输出 Schema

// 状态管理
WithOutputKey(key string)  // 输出存储到会话的键名
```

---

### 2.2 ModelProvider 模块

#### 2.2.1 核心接口

```go
// ModelProvider 是用于多模态对话式模型的接口
type ModelProvider interface {
    Name() string  // 返回模型名称

    // Generate 执行完整生成请求，一次性返回结果
    // 适用于不需要实时反馈的场景
    Generate(context.Context, *ModelRequest) (*ModelResponse, error)

    // NewStreaming 执行流式生成请求
    // 返回生成器，允许逐步接收模型输出
    // 适用于构建实时对话应用
    NewStreaming(context.Context, *ModelRequest) Generator[*ModelResponse, error]
}
```

#### 2.2.2 请求/响应结构

```go
// ModelRequest 是多模态对话式请求
type ModelRequest struct {
    Tools        []tools.Tool       // 可用工具列表
    Messages     []*Message         // 消息历史
    Instruction  *Message           // 系统指令
    InputSchema  *jsonschema.Schema // 输入 Schema 约束
    OutputSchema *jsonschema.Schema // 输出 Schema 约束
}

// ModelResponse 是单次生成的助手消息结果
type ModelResponse struct {
    Message *Message  // 助手消息
}
```

#### 2.2.3 Provider 实现

Blades 提供了多个开箱即用的 Provider 实现：

| Provider | 文件 | 支持的模型 |
|----------|------|-----------|
| **OpenAI** | `contrib/openai/chat.go` | GPT-4, GPT-4o, o1 等 |
| **Anthropic** | `contrib/anthropic/claude.go` | Claude 3/3.5/4 系列 |
| **Gemini** | `contrib/gemini/gemini.go` | Gemini 1.5/2.0 系列 |

**Provider 适配器模式：**

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Blades        │     │   Provider      │     │   External      │
│   ModelRequest  │────►│   Adapter       │────►│   LLM API       │
│   (标准格式)    │     │   (转换层)      │     │   (原生格式)    │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                                │
                                ▼
                        ┌─────────────────┐
                        │   Blades        │
                        │   ModelResponse │
                        │   (标准格式)    │
                        └─────────────────┘
```

---

### 2.3 Tool 模块

#### 2.3.1 核心接口

```go
// Tool 是 Agent 可调用的外部能力抽象
type Tool interface {
    Name() string         // 工具名称
    Description() string  // 工具描述

    // InputSchema 返回输入 JSON Schema（可选）
    // 用于指导 LLM 生成正确的调用参数
    InputSchema() *jsonschema.Schema

    // OutputSchema 返回输出 JSON Schema（可选）
    OutputSchema() *jsonschema.Schema

    // Handle 执行工具调用
    Handle(ctx context.Context, input string) (string, error)
}
```

#### 2.3.2 工具类型

**1. 函数工具 (Function Tool)**

通过 `tools.NewFunc` 创建，将 Go 函数包装为 Tool：

```go
// 示例：天气查询工具
weatherTool := tools.NewFunc(
    "get_weather",
    "查询指定城市的天气",
    weatherInputSchema,
    func(ctx context.Context, input *WeatherInput) (*WeatherOutput, error) {
        // 实现天气查询逻辑
    },
)
```

**2. Agent 工具 (Agent as Tool)**

通过 `blades.NewAgentTool` 将 Agent 包装为 Tool：

```go
// 将搜索 Agent 作为工具提供给主 Agent
searchAgent, _ := blades.NewAgent("搜索专家", ...)
searchTool := blades.NewAgentTool(searchAgent)
```

**3. MCP 工具 (Model Context Protocol)**

通过 MCP 服务器动态获取工具：

```go
// 配置 MCP 工具解析器
resolver := mcp.NewToolResolver(client)
agent, _ := blades.NewAgent("助手",
    blades.WithToolsResolver(resolver),
)
```

#### 2.3.3 工具执行流程

```
┌──────────────────────────────────────────────────────────────┐
│                    工具调用执行流程                          │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Agent.handle() 检测到 RoleTool 消息                          │
│       │                                                      │
│       ▼                                                      │
│  executeTools() 并发执行所有工具调用                           │
│       │                                                      │
│       ├─► 为每个 ToolPart 创建 goroutine                      │
│       │     │                                                │
│       │     ▼                                                │
│       │   handleTools() 查找并执行工具                        │
│       │     │                                                │
│       │     ├─► 遍历 invocation.Tools                         │
│       │     │   查找名称匹配的 Tool                           │
│       │     │                                                │
│       │     ├─► tool.Handle(ctx, request) 执行               │
│       │     │                                                │
│       │     └─► 返回 ToolPart 填充 Response                    │
│       │                                                      │
│       ├─► 更新 message.Parts[i] = part                       │
│       ├─► 合并 message.Actions                                │
│       │                                                      │
│       ▼                                                      │
│  eg.Wait() 等待所有工具完成                                    │
│       │                                                      │
│       ▼                                                      │
│  返回更新后的 Message（包含工具响应）                            │
│       │                                                      │
│       ▼                                                      │
│  添加到 req.Messages，继续下一次模型调用                        │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

#### 2.3.4 ToolContext

工具执行时可以访问的上下文信息：

```go
type ToolContext interface {
    ID() string                    // 工具调用唯一标识
    Name() string                  // 工具名称
    Actions() map[string]any       // Actions 映射（跨工具共享状态）
    SetAction(key string, value any) // 设置 Action
}
```

---

### 2.4 Runner 模块

#### 2.4.1 核心结构

```go
// Runner 负责在会话上下文中执行 Agent
type Runner struct {
    rootAgent      Agent          // 根 Agent（执行入口）
    contextManager ContextManager // 上下文管理器（可选）
}
```

#### 2.4.2 执行方法

**Run (非流式)**
```go
func (r *Runner) Run(ctx context.Context, message *Message, opts ...RunOption) (*Message, error)
```
- 等待 Agent 完成所有迭代
- 返回最终的助手消息
- 适用于批处理和不需要实时反馈的场景

**RunStream (流式)**
```go
func (r *Runner) RunStream(ctx context.Context, message *Message, opts ...RunOption) Generator[*Message, error]
```
- 逐步产生消息，支持实时输出
- 适用于聊天界面等需要即时反馈的场景

#### 2.4.3 运行选项

```go
// Session 配置
WithSession(session Session)      // 自定义会话
WithResume(resume bool)           // 是否从历史恢复

// Invocation 配置
WithInvocationID(id string)       // 自定义调用 ID

// Runner 配置
WithContextManager(manager ContextManager)  // 上下文管理器
```

#### 2.4.4 执行流程

```
┌─────────────────────────────────────────────────────────────┐
│                    Runner.Run() 流程                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. 初始化 RunOptions                                        │
│     - 创建新 Session（如果未提供）                          │
│     - 生成新 InvocationID（如果未提供）                     │
│                                                             │
│  2. buildInvocation()                                       │
│     - 构建 Invocation 对象                                   │
│     - 将用户消息添加到会话历史                              │
│                                                             │
│  3. 创建执行上下文                                           │
│     runCtx = NewSessionContext(ctx, session)               │
│     if contextManager != nil:                               │
│         runCtx = NewContextManagerContext(runCtx, manager)  │
│                                                             │
│  4. rootAgent.Run(runCtx, invocation)                       │
│     - 调用 Agent 的 Run 方法                                  │
│     - 消费迭代器获取消息                                    │
│                                                             │
│  5. 保存已完成的消息到会话历史                               │
│     session.Append(message)                                 │
│                                                             │
│  6. 返回最终消息                                             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

### 2.5 Middleware 模块

#### 2.5.1 核心类型

```go
// Handler 处理 Invocation 并返回消息生成器
type Handler interface {
    Handle(context.Context, *Invocation) Generator[*Message, error]
}

// Middleware 包装 Handler 并添加额外行为
type Middleware func(Handler) Handler

// ChainMiddlewares 将多个中间件组合成一个（按顺序应用）
func ChainMiddlewares(mws ...Middleware) Middleware
```

#### 2.5.2 洋葱模型执行

```
中间件链：Logging -> Auth -> Metrics -> Handler

执行流程（请求）:
┌───────────────────────────────────────────────────────────┐
│  LoggingMiddleware (外层)                                  │
│  │  1. 记录开始时间                                        │
│  │  ▼                                                      │
│  │  AuthMiddleware                                         │
│  │  │  2. 验证权限                                          │
│  │  │  ▼                                                    │
│  │  │  MetricsMiddleware                                    │
│  │  │  │  3. 记录指标                                        │
│  │  │  │  ▼                                                  │
│  │  │  │  Handler (内层)                                    │
│  │  │  │  │  4. 实际处理                                      │
│  │  │  │  ▲                                                  │
│  │  │  5. 记录响应时间                                        │
│  │  │  ▲                                                    │
│  │  6. 记录认证结果                                          │
│  │  ▲                                                      │
│  7. 记录完成                                                │
└───────────────────────────────────────────────────────────┘
```

#### 2.5.3 中间件示例

**日志中间件：**
```go
func LoggingMiddleware(next Handler) Handler {
    return HandleFunc(func(ctx context.Context, inv *Invocation) Generator[*Message, error] {
        log.Printf("[开始] 调用 ID: %s", inv.ID)
        defer log.Printf("[结束] 调用 ID: %s", inv.ID)
        return next.Handle(ctx, inv)
    })
}
```

**重试中间件：**
```go
func RetryMiddleware(maxRetries int) Middleware {
    return func(next Handler) Handler {
        return HandleFunc(func(ctx context.Context, inv *Invocation) Generator[*Message, error] {
            return func(yield func(*Message, error) bool) {
                for attempt := 0; attempt < maxRetries; attempt++ {
                    for msg, err := range next.Handle(ctx, inv) {
                        if err == nil {
                            yield(msg, nil)
                            return
                        }
                        if attempt == maxRetries-1 {
                            yield(nil, err)
                            return
                        }
                    }
                }
            }
        })
    }
}
```

---

### 2.6 Session & Memory 模块

#### 2.6.1 Session 接口

```go
// Session 管理单轮或多轮对话的上下文
type Session interface {
    // Append 添加消息到历史
    Append(ctx context.Context, msg *Message) error

    // History 返回完整的消息历史
    History() []*Message

    // State 返回会话状态（键值对）
    State() map[string]any

    // SetState 设置状态值
    SetState(key string, value any)

    // Save 持久化会话（可选）
    Save(ctx context.Context) error
}
```

#### 2.6.2 Memory 接口

```go
// Memory 提供长期记忆能力
type Memory interface {
    // AddMemory 添加记忆
    AddMemory(ctx context.Context, memory *Memory) error

    // SearchMemory 搜索相关记忆
    SearchMemory(ctx context.Context, query string) ([]*Memory, error)

    // SaveSession 持久化会话
    SaveSession(ctx context.Context, session Session) error
}
```

#### 2.6.3 InMemory 实现

```go
// InMemoryStore 是内存中的会话存储实现
type InMemoryStore struct {
    mu       sync.RWMutex
    sessions map[string]*sessionInMemory  // sessionID -> Session
    histories map[string][]*Message       // sessionID -> History
    states   map[string]map[string]any    // sessionID -> State
}
```

---

### 2.7 Flow 工作流模块

Flow 模块提供四种基本的工作流编排模式：

#### 2.7.1 Sequential (顺序执行)

```go
// SequentialConfig 顺序执行配置
type SequentialConfig struct {
    Name        string
    Description string
    SubAgents   []blades.Agent  // 按顺序执行的子代理列表
}

// 执行流程：Agent1 -> Agent2 -> Agent3 -> ...
// 每个 Agent 完成后才执行下一个
```

**使用场景：**
- 数据处理流水线（收集 -> 分析 -> 报告）
- 多步骤任务分解

#### 2.7.2 Parallel (并行执行)

```go
// ParallelConfig 并行执行配置
type ParallelConfig struct {
    Name        string
    Description string
    SubAgents   []blades.Agent  // 并行执行的子代理列表
}

// 执行流程：
// Agent1 ─┐
// Agent2 ─┼──► 汇总输出
// Agent3 ─┘
// 所有 Agent 同时启动，输出汇总
```

**使用场景：**
- 多数据源并行采集
- 独立任务并行处理

#### 2.7.3 Loop (循环执行)

```go
// LoopConfig 循环执行配置
type LoopConfig struct {
    Name          string
    Description   string
    MaxIterations int           // 最大迭代次数
    Condition     LoopCondition // 循环条件函数
    SubAgents     []blades.Agent
}

// 循环条件函数
type LoopCondition func(ctx context.Context, state LoopState) (bool, error)

// LoopState 包含每次迭代的状态
type LoopState struct {
    Iteration int           // 当前迭代次数
    Input     *Message      // 原始输入
    Output    *Message      // 当前迭代输出
}
```

**终止机制：**
1. 达到 `MaxIterations`
2. `Condition` 返回 `false`
3. 子代理通过 `ExitTool` 设置退出信号

**退出信号：**
```go
// escalated=false：正常退出
// escalated=true：升级退出（返回 ErrLoopEscalated）
message.Actions[tools.ActionLoopExit] = escalated
```

#### 2.7.4 Routing (路由分发)

```go
// RoutingConfig 路由配置
type RoutingConfig struct {
    Name        string
    Description string
    RouteFn     RouteFn       // 路由函数
    SubAgents   map[string]blades.Agent
}

// RouteFn 根据输入决定路由到哪个子代理
type RouteFn func(ctx context.Context, input *Message) (string, error)
```

**使用场景：**
- 意图识别后路由到专业 Agent
- 基于内容类型的分发

---

### 2.8 Graph 图引擎模块

Graph 模块提供有向无环图（DAG）形式的工作流编排。

#### 2.8.1 核心结构

```go
// Graph 表示一个有向图
type Graph struct {
    nodes       map[string]Handler              // 节点名 -> Handler
    edges       map[string][]conditionalEdge    // 节点名 -> 出边列表
    entryPoint  string                          // 入口节点
    finishPoint string                          // 出口节点
    parallel    bool                            // 是否并行执行
    middlewares []Middleware                    // 全局中间件
}

// Handler 节点处理函数
type Handler func(ctx context.Context, state State) (State, error)

// State 图执行状态
type State interface {
    Get(key string) any
    Set(key string, value any)
    Delete(key string)
}
```

#### 2.8.2 边和条件

```go
// 无条件边：总是执行
graph.AddEdge("node1", "node2")

// 条件边：根据状态决定是否执行
graph.AddEdge("router", "branch_a",
    WithEdgeCondition(func(ctx context.Context, state State) bool {
        return state.Get("type") == "A"
    }),
)
```

#### 2.8.3 编译和执行

```go
// 1. 创建图
graph := graph.New(
    graph.WithParallel(true),
    graph.WithMiddleware(loggingMiddleware),
)

// 2. 添加节点
graph.AddNode("start", startHandler)
graph.AddNode("process", processHandler)
graph.AddNode("end", endHandler)

// 3. 添加边
graph.AddEdge("start", "process")
graph.AddEdge("process", "end")

// 4. 设置入口和出口
graph.SetEntryPoint("start")
graph.SetFinishPoint("end")

// 5. 编译（验证图的正确性）
executor, err := graph.Compile(
    graph.WithCheckpointer(checkpointer),
)

// 6. 执行
finalState, err := executor.Run(ctx, initialState)
```

#### 2.8.4 图验证

编译时会进行以下验证：
1. **入口/出口点存在性**：必须设置且节点存在
2. **环检测**：使用 DFS 检测有向环（不支持循环）
3. **可达性**：从入口必须能到达出口
4. **结构完整性**：非结束节点必须有出边，结束节点不能有出边
5. **边类型一致性**：同一节点不能混用条件边和无条件边

---

### 2.9 Recipe 配方系统

Recipe 提供了一种声明式的 YAML 配置方式来定义 Agent 和工作流。

#### 2.9.1 AgentSpec 结构

```go
// AgentSpec 定义单个 Agent 的规范
type AgentSpec struct {
    Name          string            `yaml:"name"`           // Agent 名称
    Description   string            `yaml:"description"`    // 功能描述
    Instruction   string            `yaml:"instruction"`    // 系统指令
    Model         string            `yaml:"model"`          // 模型标识
    Tools         []string          `yaml:"tools"`          // 工具列表
    SubAgents     []SubAgentSpec    `yaml:"sub_agents"`     // 子代理
    Context       *ContextSpec      `yaml:"context"`        // 上下文管理
    Middlewares   []string          `yaml:"middlewares"`    // 中间件
    OutputKey     string            `yaml:"output_key"`     // 输出存储键
    MaxIterations int               `yaml:"max_iterations"` // 最大迭代
}

// ExecutionMode 执行模式
type ExecutionMode string
const (
    ModeSequential ExecutionMode = "sequential"  // 顺序
    ModeParallel   ExecutionMode = "parallel"    // 并行
    ModeLoop       ExecutionMode = "loop"        // 循环
    ModeTool       ExecutionMode = "tool"        // 工具模式
)
```

#### 2.9.2 YAML 示例

```yaml
name: 代码审查助手
description: 审查代码并提供改进建议
model: gpt-4o
instruction: |
  你是一个专业的代码审查专家，请检查代码的质量问题并提供改进建议。

params:
  language: go
  focus_areas:
    - 性能
    - 可读性
    - 安全性

tools:
  - file_read
  - github_api

context:
  mode: window
  max_tokens: 8000

sub_agents:
  - name: 语法检查器
    instruction: 检查代码语法错误
  - name: 性能分析器
    instruction: 分析代码性能问题
```

#### 2.9.3 构建流程

```go
// 1. 创建注册表
registry := recipe.NewModelRegistry()
registry.Register("gpt-4o", openaiModel)

// 2. 加载配方
spec, err := recipe.LoadFromFile("agent.yaml")

// 3. 构建 Agent
agent, err := recipe.Build(spec,
    recipe.WithModelRegistry(registry),
    recipe.WithParams(map[string]any{"language": "go"}),
)

// 4. 执行
runner := blades.NewRunner(agent)
output, err := runner.Run(ctx, blades.UserMessage("审查这段代码"))
```

---

### 2.10 Skills 技能系统

Skills 是一种高级抽象，将多个工具组合成可复用的技能包。

#### 2.10.1 技能结构

```go
// Skill 表示一个可复用的技能
type Skill interface {
    Name() string                    // 技能名称
    Description() string             // 技能描述
    SystemInstruction() string       // 系统指令
    Resources() []Resource           // 资源文件
    Tools() []Tool                   // 技能包含的工具
    Validate() error                 // 验证技能有效性
}
```

#### 2.10.2 技能目录结构

```
my-skill/
├── skill.yaml          # 技能元数据
├── prompts/            # 提示词模板
│   └── system.md
├── resources/          # 资源文件
│   └── knowledge.txt
└── scripts/            # 脚本文件
    └── process.py
```

#### 2.10.3 Toolset 工具集

```go
// Toolset 将 Skills 组合为统一的工具集
type Toolset struct {
    skills []Skill
    instruction string      // 聚合的系统指令
    tools []Tool           // 聚合的工具列表
}

// ComposeTools 将 Skills 的工具与外部工具组合
func (t *Toolset) ComposeTools(externalTools []Tool) []Tool

// Instruction 获取聚合的系统指令
func (t *Toolset) Instruction() string
```

---

## 3. 模块间关系和调用流程

### 3.1 模块依赖关系

```
┌─────────────────────────────────────────────────────────────────┐
│                        应用层 (Application)                      │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌─────────────┐  │
│  │  Recipe   │  │   Flow    │  │   Graph   │  │   Direct    │  │
│  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘  └──────┬──────┘  │
├────────┼──────────────┼──────────────┼───────────────┼─────────┤
│        │              │              │               │         │
│  ┌─────▼──────────────▼──────────────▼───────────────▼──────┐  │
│  │                    Runner                                │  │
│  │  ┌────────────────────────────────────────────────────┐  │  │
│  │  │              Session & Memory                      │  │  │
│  │  └────────────────────────────────────────────────────┘  │  │
│  └────────────────────────┬─────────────────────────────────┘  │
│                           │                                     │
│  ┌────────────────────────▼─────────────────────────────────┐  │
│  │                    Agent 核心                             │  │
│  │  ┌───────────────┐  ┌───────────────┐  ┌─────────────┐  │  │
│  │  │  Middleware   │  │    Tools      │  │   Skills    │  │  │
│  │  └───────────────┘  └───────────────┘  └─────────────┘  │  │
│  └────────────────────────┬─────────────────────────────────┘  │
│                           │                                     │
│  ┌────────────────────────▼─────────────────────────────────┐  │
│  │                 ModelProvider                            │  │
│  │  ┌───────────┐  ┌───────────┐  ┌───────────┐            │  │
│  │  │  OpenAI   │  │ Anthropic │  │  Gemini   │  ...       │  │
│  │  └───────────┘  └───────────┘  └───────────┘            │  │
│  └────────────────────────┬─────────────────────────────────┘  │
│                           │                                     │
│  ┌────────────────────────▼─────────────────────────────────┐  │
│  │                 External Services                        │  │
│  │  ┌───────────┐  ┌───────────┐  ┌─────────────────────┐  │  │
│  │  │    MCP    │  │  Files    │  │   Third-party API   │  │  │
│  │  └───────────┘  └───────────┘  └─────────────────────┘  │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 3.2 典型调用流程

```
用户请求
   │
   ▼
┌──────────────────────────────────────────────────────────────┐
│ 1. Runner.Run(ctx, UserMessage("..."))                       │
│    - 创建 Session                                             │
│    - 生成 InvocationID                                        │
│    - 注入 ContextManager                                      │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│ 2. Agent.Run(ctx, invocation)                                │
│    - checkResume() 检查是否可恢复                              │
│    - prepareInvocation() 准备调用                             │
│      ├─ resolveTools() 解析工具                               │
│      ├─ 合并 Skills 指令                                       │
│      └─ 处理模板指令                                          │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│ 3. 应用中间件链                                                │
│    ChainMiddlewares(mws...)(handler)                         │
│    Logging -> Auth -> Metrics -> Handler                     │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│ 4. handler.Handle() ──► handle() 主循环                       │
│                                                                │
│    for i := 0; i < maxIterations; i++ {                       │
│                                                                │
│      a. ContextManager.Prepare() 管理上下文窗口                │
│                                                                │
│      b. Model.Generate() 调用 LLM                              │
│         ┌───────────────────────────────────────────┐         │
│         │ ModelProvider.Generate()                  │         │
│         │   ├─ OpenAI / Claude / Gemini             │         │
│         │   └─ 转换请求/响应格式                     │         │
│         └───────────────────────────────────────────┘         │
│                                                                │
│      c. 处理响应                                               │
│         ├─ saveOutputState() 保存输出到会话                     │
│         └─ yield(message) 输出消息                             │
│                                                                │
│      d. Role == RoleTool ?                                     │
│         ├─ 是：executeTools() 并发执行工具                       │
│         │   ├─ handleTools() 查找并执行工具                     │
│         │   │   ┌─────────────────────────────────┐           │
│         │   │   │ Tool.Handle()                   │           │
│         │   │   │ ├─ Function Tool                │           │
│         │   │   │ ├─ Agent Tool                   │           │
│         │   │   │ └─ MCP Tool                     │           │
│         │   │   └─────────────────────────────────┘           │
│         │   └─ append(toolMessage) 添加到历史                  │
│         │   └─ continue (下一次迭代)                           │
│         │                                                     │
│         └─ 否：return (完成)                                   │
│    }                                                           │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│ 5. Runner 保存消息到 Session                                  │
│    session.Append(message)                                   │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
                    返回最终消息
```

### 3.3 数据流分析

**Invocation 数据流：**

```
┌─────────────────────────────────────────────────────────────┐
│                     Invocation 生命周期                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Runner.buildInvocation()                                   │
│       │                                                     │
│       ├─► ID = UUID (或用户指定)                            │
│       ├─► Session = NewSession() (或用户指定)               │
│       ├─► Message = UserMessage                             │
│       └─► Stream = false/true                               │
│                                                             │
│       ▼                                                     │
│  Agent.prepareInvocation()                                  │
│       │                                                     │
│       ├─► Model = agent.model.Name()                        │
│       ├─► Tools = resolveTools()                            │
│       │   ├─ 静态工具                                        │
│       │   ├─ Skills 工具                                     │
│       │   └─ 动态解析工具 (MCP)                              │
│       │                                                     │
│       ├─► Instruction = 合并指令                            │
│       │   优先级：静态 > Provider > Skills > Invocation     │
│       │                                                     │
│       └─► 支持 Go template 模板变量                           │
│                                                             │
│       ▼                                                     │
│  Agent.handle() 主循环                                       │
│       │                                                     │
│       ├─► ModelRequest                                      │
│       │   ├─ Tools = invocation.Tools                       │
│       │   ├─ Messages = 会话历史                            │
│       │   ├─ Instruction = invocation.Instruction           │
│       │   └─ Schema = input/outputSchema                    │
│       │                                                     │
│       └─► 迭代更新                                           │
│           └─► req.Messages = append(toolMessage)            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Session 数据流：**

```
┌─────────────────────────────────────────────────────────────┐
│                      Session 数据流                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Session 结构                                               │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  History: []*Message                                │   │
│  │    ├─ Message 1 (User)                              │   │
│  │    ├─ Message 2 (Assistant)                         │   │
│  │    ├─ Message 3 (Tool)                              │   │
│  │    └─ Message 4 (Assistant - Final)                 │   │
│  │                                                     │   │
│  │  State: map[string]any                              │   │
│  │    ├─ "search_result": "..."                        │   │
│  │    ├─ "analysis": "..."                             │   │
│  │    └─ "final_output": "..."                         │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
│  数据流向:                                                  │
│                                                             │
│  1. Runner 添加用户消息                                       │
│     session.Append(userMessage)                             │
│                                                             │
│  2. Agent 执行过程中                                         │
│     ├─ saveOutputState() 保存输出到 State                   │
│     │   session.SetState(outputKey, message.Text())        │
│     │                                                       │
│     └─ 工具执行可能读取 State                                │
│         value := session.State()["previous_result"]        │
│                                                             │
│  3. 完成后保存助手消息                                        │
│     session.Append(assistantMessage)                        │
│                                                             │
│  4. 可选持久化                                                │
│     memory.SaveSession(ctx, session)                        │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 4. 关键技术选型和设计理念

### 4.1 Go 语言特性利用

#### 4.1.1 接口抽象

Go 的接口设计使得 Blades 能够实现高度解耦：

```go
// 小接口，大用处
type Agent interface {
    Name() string
    Description() string
    Run(context.Context, *Invocation) Generator[*Message, error]
}

// 任何实现此接口的类型都可以：
// - 被 Runner 执行
// - 作为 Flow 的子代理
// - 作为 Graph 的节点
// - 被包装为 Tool
```

**优势：**
- **解耦**：调用方只依赖接口，不依赖具体实现
- **可测试**：轻松创建 mock 实现进行单元测试
- **可扩展**：添加新实现无需修改现有代码

#### 4.1.2 函数选项模式 (Functional Options)

```go
func NewAgent(name string, opts ...AgentOption) (Agent, error) {
    a := &agent{
        name:          name,
        maxIterations: 10,  // 默认值
    }
    for _, opt := range opts {
        opt(a)  // 应用选项
    }
    // ...
}

// 使用
agent, _ := NewAgent("助手",
    WithModel(model),
    WithInstruction("你是有帮助的助手"),
    WithTools(searchTool, fileTool),
    WithMiddleware(loggingMiddleware),
)
```

**优势：**
- 灵活的配置方式，零个或多个选项
- 保持构造函数简洁
- 易于扩展新选项

#### 4.1.3 生成器模式 (Go 1.23+ iter.Seq2)

```go
type Generator[T, E any] = iter.Seq2[T, E]

func (a *agent) Run(ctx, inv *Invocation) Generator[*Message, error] {
    return func(yield func(*Message, error) bool) {
        // 产生消息
        if !yield(message, nil) {
            return  // 消费者提前终止
        }
    }
}
```

**优势：**
- 惰性求值，按需生成
- 支持流式输出
- 支持早期终止
- 自然的并发控制

#### 4.1.4 上下文 (context) 管理

```go
// 所有执行方法都接受 context.Context
func Run(ctx context.Context, invocation *Invocation) Generator[*Message, error]

// 使用场景：
// 1. 超时控制
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// 2. 取消信号
ctx, cancel := context.WithCancel(context.Background())
// ... 在其他 goroutine 中调用 cancel()

// 3. 传递值
ctx = NewAgentContext(ctx, agent)
ctx = NewSessionContext(ctx, session)
```

### 4.2 设计模式应用

#### 4.2.1 策略模式 (Strategy Pattern)

ModelProvider 是典型的策略模式：

```go
// 策略接口
type ModelProvider interface {
    Generate(context.Context, *ModelRequest) (*ModelResponse, error)
    NewStreaming(context.Context, *ModelRequest) Generator[*ModelResponse, error]
}

// 具体策略
type openAIModel struct{ ... }   // OpenAI 实现
type claudeModel struct{ ... }   // Claude 实现
type geminiModel struct{ ... }   // Gemini 实现
```

#### 4.2.2 适配器模式 (Adapter Pattern)

Tool 接口适配各种外部能力：

```go
// 函数适配器
func NewFunc(name, description string, schema *jsonschema.Schema, fn interface{}) Tool

// Agent 适配器
func NewAgentTool(agent Agent) Tool

// MCP 工具适配器
func NewMCPTool(client *mcp.Client, tool mcp.Tool) Tool
```

#### 4.2.3 责任链模式 (Chain of Responsibility)

Middleware 链：

```go
func ChainMiddlewares(mws ...Middleware) Middleware {
    return func(next Handler) Handler {
        h := next
        for i := len(mws) - 1; i >= 0; i-- {
            h = mws[i](h)  // 从内向外包装
        }
        return h
    }
}
```

#### 4.2.4 建造者模式 (Builder Pattern)

Graph 构建：

```go
graph := graph.New()
    .AddNode("start", startHandler)
    .AddNode("process", processHandler)
    .AddNode("end", endHandler)
    .AddEdge("start", "process")
    .AddEdge("process", "end")
    .SetEntryPoint("start")
    .SetFinishPoint("end")
```

### 4.3 扩展性设计

#### 4.3.1 插件化 ModelProvider

添加新的模型提供者只需实现 `ModelProvider` 接口：

```go
type MyCustomModel struct {
    apiKey string
    // ...
}

func (m *MyCustomModel) Name() string { return "my-model" }

func (m *MyCustomModel) Generate(ctx context.Context, req *ModelRequest) (*ModelResponse, error) {
    // 实现与自定义模型的交互逻辑
    // 转换为标准 ModelResponse
}

func (m *MyCustomModel) NewStreaming(ctx context.Context, req *ModelRequest) Generator[*ModelResponse, error] {
    // 实现流式生成
}
```

#### 4.3.2 动态 Tool 解析

通过 `tools.Resolver` 实现动态工具加载：

```go
type Resolver interface {
    Resolve(ctx context.Context) ([]Tool, error)
}

// MCP 解析器实现
type mcpResolver struct {
    client *mcp.Client
}

func (r *mcpResolver) Resolve(ctx context.Context) ([]Tool, error) {
    // 从 MCP 服务器动态获取工具列表
}
```

#### 4.3.3 自定义 Flow 模式

实现 `Agent` 接口即可创建新的 Flow 模式：

```go
type TreeAgent struct {
    config TreeConfig
}

func (t *TreeAgent) Name() string { return t.config.Name }
func (t *TreeAgent) Description() string { return t.config.Description }
func (t *TreeAgent) Run(ctx context.Context, inv *Invocation) Generator[*Message, error] {
    // 实现树形工作流逻辑
}
```

### 4.4 错误处理策略

#### 4.4.1 错误类型定义

```go
var (
    ErrModelProviderRequired  = errors.New("blades: model provider is required")
    ErrNoFinalResponse        = errors.New("blades: no final response received")
    ErrMaxIterationsExceeded  = errors.New("blades: maximum iterations exceeded")
    ErrLoopEscalated          = errors.New("blades: loop escalated")
)
```

#### 4.4.2 错误传播机制

```go
// Agent 执行错误
for message, err := range agent.Run(ctx, invocation) {
    if err != nil {
        // 错误通过 Generator 传递
        yield(nil, err)
        return
    }
    yield(message, nil)
}

// 工具执行错误
func (a *agent) executeTools(...) (*Message, error) {
    eg, ctx := errgroup.WithContext(ctx)
    for _, part := range message.Parts {
        eg.Go(func() error {
            // 工具执行错误会终止整个 eg
            if err := handleTool(...); err != nil {
                return err
            }
            return nil
        })
    }
    return message, eg.Wait()  // 等待并返回第一个错误
}
```

#### 4.4.3 错误恢复机制

```go
// 重试中间件
func RetryMiddleware(maxRetries int) Middleware {
    return func(next Handler) Handler {
        return HandleFunc(func(ctx context.Context, inv *Invocation) Generator[*Message, error] {
            for attempt := 0; attempt < maxRetries; attempt++ {
                for msg, err := range next.Handle(ctx, inv) {
                    if err == nil {
                        yield(msg, nil)
                        return
                    }
                    // 最后一次尝试失败才返回错误
                    if attempt == maxRetries-1 {
                        yield(nil, err)
                        return
                    }
                    // 否则继续重试
                }
            }
        })
    }
}
```

---

## 附录

### A. 包结构

```
blades/
├── blades.go              # 核心包（Agent、ModelProvider 接口）
├── agent.go               # Agent 实现
├── runner.go              # Runner 执行器
├── message.go             # 消息类型定义
├── session.go             # 会话管理
├── middleware.go          # 中间件机制
├── tool.go                # Agent as Tool
│
├── flow/                  # 工作流模块
│   ├── sequential.go      # 顺序执行
│   ├── parallel.go        # 并行执行
│   ├── loop.go            # 循环执行
│   └── routing.go         # 路由分发
│
├── graph/                 # 图引擎模块
│   ├── graph.go           # 图定义
│   ├── executor.go        # 执行器
│   ├── state.go           # 状态管理
│   ├── checkpoint.go      # 检查点
│   └── middleware.go      # 图中间件
│
├── recipe/                # 配方系统
│   ├── spec.go            # AgentSpec 定义
│   ├── builder.go         # 构建器
│   ├── loader.go          # 加载器
│   └── registry.go        # 注册表
│
├── skills/                # 技能系统
│   ├── loader.go          # 技能加载
│   ├── toolset.go         # 工具集
│   └── models.go          # 数据模型
│
├── tools/                 # 工具系统
│   ├── tool.go            # Tool 实现
│   ├── handler.go         # Handler 适配器
│   ├── resolver.go        # 工具解析器
│   └── middleware.go      # 工具中间件
│
├── contrib/               # 第三方贡献
│   ├── openai/            # OpenAI Provider
│   ├── anthropic/         # Anthropic Provider
│   ├── gemini/            # Gemini Provider
│   ├── mcp/               # MCP 客户端
│   └── otel/              # OpenTelemetry 追踪
│
├── memory/                # 记忆系统
│   ├── memory.go          # Memory 接口
│   └── in_memory_store.go # 内存实现
│
├── context/               # 上下文管理
│   ├── summary/           # 摘要压缩
│   └── window/            # 滑动窗口
│
├── internal/              # 内部包
│   ├── counter/           # Token 计数
│   ├── deep/              # DeepAgent 支持
│   └── handoff/           # Agent 交接
│
└── examples/              # 示例代码
    ├── model-claude/      # Claude 使用示例
    ├── workflow-parallel/ # 并行工作流示例
    └── ...                # 更多示例
```

### B. 版本兼容性

| Blades 版本 | Go 版本 | 特性 |
|------------|---------|------|
| v0.1.x | Go 1.21+ | 基础 Agent 功能 |
| v0.2.x | Go 1.22+ | Flow 工作流 |
| v0.3.x | Go 1.23+ | 生成器模式（iter.Seq2） |
| v0.4.x | Go 1.23+ | Graph 图引擎、Recipe 配方 |

### C. 相关资源

- **GitHub**: https://github.com/go-kratos/blades
- **GoDoc**: https://pkg.go.dev/github.com/go-kratos/blades
- **Kratos**: https://github.com/go-kratos/kratos (Blades 的设计灵感来源之一)
