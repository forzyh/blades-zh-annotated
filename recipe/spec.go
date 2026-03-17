package recipe

// ExecutionMode 定义子 Agent 的执行模式
// 在多 Agent 工作流中，可以通过设置 execution 字段来选择不同的执行策略
type ExecutionMode string

const (
	// ExecutionSequential 顺序执行模式
	// 子 Agent 按照定义的顺序依次执行，前一个 Agent 的输出会作为下一个 Agent 的输入
	// 适用于需要严格按步骤处理的场景，如：分析 -> 编写 -> 审查
	ExecutionSequential ExecutionMode = "sequential"
	// ExecutionParallel 并行执行模式
	// 所有子 Agent 同时执行，互不干扰，最后汇总结果
	// 适用于独立任务的同时处理，如：同时检查代码的多个方面
	ExecutionParallel ExecutionMode = "parallel"
	// ExecutionLoop 循环执行模式
	// 子 Agent 反复执行，直到达到 max_iterations 上限或某个子 Agent 通过 loop_exit 工具信号退出
	// 适用于需要迭代优化的场景，如：持续改进直到满足条件
	ExecutionLoop ExecutionMode = "loop"
	// ExecutionTool 工具模式
	// 将每个子 Agent 包装成工具，由父 Agent 根据需要调用
	// 适用于动态决策场景，父 Agent 根据任务情况选择调用哪个子 Agent
	ExecutionTool ExecutionMode = "tool"
)

// ParameterType 定义配方参数的数据类型
// 用于在 YAML 配置中声明参数的类型，支持模板渲染时的类型校验
type ParameterType string

const (
	// ParameterString 字符串类型参数
	ParameterString ParameterType = "string"
	// ParameterNumber 数字类型参数（支持整数和浮点数）
	ParameterNumber ParameterType = "number"
	// ParameterBoolean 布尔类型参数（true/false）
	ParameterBoolean ParameterType = "boolean"
	// ParameterSelect 选择类型参数（从预定义选项中选择一个值）
	ParameterSelect ParameterType = "select"
)

// ParameterRequirement 定义参数的必需性
// 用于声明参数是必须提供还是可选
type ParameterRequirement string

const (
	// ParameterRequired 必需参数，调用时必须提供值
	ParameterRequired ParameterRequirement = "required"
	// ParameterOptional 可选参数，可以不提供，使用默认值
	ParameterOptional ParameterRequirement = "optional"
)

// ContextStrategy 定义上下文管理策略
// 当 Agent 需要处理长对话或多轮交互时，需要管理上下文长度以避免超出 Token 限制
type ContextStrategy string

const (
	// ContextStrategySummarize 摘要压缩策略
	// 使用 LLM 对旧消息进行滚动摘要，保留核心信息同时减少 Token 占用
	// 适用于需要保留历史对话语义的场景
	ContextStrategySummarize ContextStrategy = "summarize"
	// ContextStrategyWindow 滑动窗口策略
	// 保留最近的消息，当超出 Token 或消息数量限制时截断最旧的消息
	// 适用于只关心最近上下文的场景
	ContextStrategyWindow ContextStrategy = "window"
)

// ContextSpec 配置 Agent 的上下文窗口管理
// 通过 YAML 中的 context 字段指定，映射到具体的 ContextManager 实现
//
// YAML 示例（摘要模式）:
//
//	context:
//	  strategy: summarize    # 使用摘要策略
//	  max_tokens: 80000      # Token 预算上限
//	  keep_recent: 10        # 始终保留最近 10 条消息
//	  batch_size: 20         # 每次压缩处理 20 条消息
//	  model: gpt-4o-mini     # 用于摘要的模型（可选，默认使用 Agent 自身模型）
//
// YAML 示例（窗口模式）:
//
//	context:
//	  strategy: window       # 使用窗口策略
//	  max_tokens: 80000      # 最大 Token 数
//	  max_messages: 100      # 最大消息数量
type ContextSpec struct {
	// Strategy 选择上下文管理实现："summarize" 或 "window"
	Strategy ContextStrategy `yaml:"strategy"`
	// MaxTokens 设置 Token 预算上限，超出时会压缩或丢弃旧消息
	// 对于 summarize 策略，触发摘要压缩；对于 window 策略，触发截断
	MaxTokens int64 `yaml:"max_tokens,omitempty"`
	// MaxMessages 最大消息数量限制（仅 window 策略使用）
	// 当消息数量超过此值时，最旧的消息会被丢弃
	MaxMessages int `yaml:"max_messages,omitempty"`
	// KeepRecent 始终保留的最近消息数量（仅 summarize 策略使用，默认 10）
	// 这些消息不会被摘要，保持原样
	KeepRecent int `yaml:"keep_recent,omitempty"`
	// BatchSize 每次摘要压缩处理的消息数量（仅 summarize 策略使用，默认 20）
	// 控制每次处理的批量大小，影响压缩效率
	BatchSize int `yaml:"batch_size,omitempty"`
	// Model 用于摘要的模型名称（仅 summarize 策略使用）
	// 如果省略，则回退到 Agent 自身使用的模型
	Model string `yaml:"model,omitempty"`
}

// MiddlewareSpec 声明要应用到 Agent 的中间件
// 中间件在构建时通过 MiddlewareRegistry 按名称解析，Options 传递给工厂函数
// 中间件用于横切关注点，如日志记录、链路追踪、指标收集等
//
// YAML 示例:
//
//	middlewares:
//	  - name: tracing              # 链路追踪中间件
//	  - name: logging              # 日志中间件
//	    options:                   # 传递给中间件工厂的配置选项
//	      level: info              # 日志级别
type MiddlewareSpec struct {
	// Name 中间件名称，必须在 MiddlewareRegistry 中注册
	Name string `yaml:"name"`
	// Options 中间件配置选项，键值对形式传递给工厂函数
	Options map[string]any `yaml:"options,omitempty"`
}

// AgentSpec 是配方的顶层声明式规范
// YAML 配置文件会被解析成此结构，然后通过 Build 函数构建成 blades.Agent
// 配方（Recipe）是一种声明式的方式来定义 Agent 行为，无需编写代码即可配置复杂的 Agent 工作流
type AgentSpec struct {
	// Version 配方版本号，用于标识配置格式版本（必需字段）
	Version string `yaml:"version"`
	// Agent 名称，用于识别和日志记录（必需字段）
	Name string `yaml:"name"`
	// Description 描述信息，说明 Agent 的用途和能力
	Description string `yaml:"description"`
	// Model 使用的模型名称，必须在 ModelRegistry 中注册
	// 当有子 Agent 时，此模型作为子 Agent 的默认模型（可被子 Agent 覆盖）
	Model string `yaml:"model,omitempty"`
	// Instruction 系统指令，告诉 Agent 应该做什么
	// 支持 Go template 语法，可以使用 {{.param}} 引用参数
	Instruction string `yaml:"instruction"`
	// Prompt 可选的运行时 Prompt 模板，在每次调用时动态注入
	// 与 Instruction 不同，Prompt 是在运行时（而非构建时）渲染的
	Prompt string `yaml:"prompt,omitempty"`
	// Parameters 参数列表，定义此配方可接受的配置参数
	// 用于模板渲染和参数校验
	Parameters []ParameterSpec `yaml:"parameters,omitempty"`
	// SubAgents 子 Agent 列表，定义多步工作流中的子 Agent
	// 当定义子 Agent 时，必须同时指定 Execution 执行模式
	SubAgents []SubAgentSpec `yaml:"sub_agents,omitempty"`
	// Execution 执行模式，当有子 Agent 时必需指定
	// 可选值：sequential（顺序）、parallel（并行）、loop（循环）、tool（工具）
	Execution ExecutionMode `yaml:"execution,omitempty"`
	// Tools 外部工具列表，引用在 ToolRegistry 中注册的工具名称
	// 这些工具会添加到 Agent 的工具集中供调用
	Tools []string `yaml:"tools,omitempty"`
	// OutputKey 输出键，指定将 Agent 输出存储到会话状态的哪个键
	// 用于在多个 Agent 之间共享数据
	OutputKey string `yaml:"output_key,omitempty"`
	// MaxIterations 最大迭代次数，限制 Agent 可以调用工具的最大次数
	// 防止无限循环或过度调用工具
	MaxIterations int `yaml:"max_iterations,omitempty"`
	// Context 上下文管理配置，当需要长对话或多轮交互时指定
	Context *ContextSpec `yaml:"context,omitempty"`
	// Middlewares 中间件列表，应用于此 Agent 的横切关注点
	Middlewares []MiddlewareSpec `yaml:"middlewares,omitempty"`
}

// SubAgentSpec 定义配方中的子 Agent
// 子 Agent 是父 Agent 工作流的一部分，可以有自己的配置
// 子 Agent 会继承父 Agent 的参数，也可以定义自己的参数
type SubAgentSpec struct {
	// Name 子 Agent 名称，用于识别和日志记录（必需字段）
	Name string `yaml:"name"`
	// Description 描述信息，说明子 Agent 的用途
	Description string `yaml:"description,omitempty"`
	// Model 使用的模型名称，如果省略则继承父 Agent 的模型
	Model string `yaml:"model,omitempty"`
	// Instruction 系统指令，定义子 Agent 的职责
	// 支持 Go template 语法，可以使用 {{.param}} 引用参数
	Instruction string `yaml:"instruction"`
	// Prompt 可选的运行时 Prompt 模板
	Prompt string `yaml:"prompt,omitempty"`
	// Parameters 参数列表，定义子 Agent 可接受的参数
	Parameters []ParameterSpec `yaml:"parameters,omitempty"`
	// Tools 外部工具列表，引用在 ToolRegistry 中注册的工具名称
	Tools []string `yaml:"tools,omitempty"`
	// OutputKey 输出键，指定将子 Agent 输出存储到会话状态的哪个键
	OutputKey string `yaml:"output_key,omitempty"`
	// MaxIterations 最大迭代次数，限制子 Agent 调用工具的最大次数
	MaxIterations int `yaml:"max_iterations,omitempty"`
	// Context 上下文管理配置
	Context *ContextSpec `yaml:"context,omitempty"`
	// Middlewares 中间件列表
	Middlewares []MiddlewareSpec `yaml:"middlewares,omitempty"`
}

// ParameterSpec 定义配方的可配置参数
// 参数用于模板渲染，使配方可以动态生成指令
// 参数在 YAML 中声明，在运行时通过 WithParams 传入具体值
type ParameterSpec struct {
	// Name 参数名称，用于在模板中引用（必需字段）
	Name string `yaml:"name"`
	// Type 参数类型，决定如何校验参数值
	// 可选值：string（字符串）、number（数字）、boolean（布尔）、select（选择）
	Type ParameterType `yaml:"type"`
	// Description 参数描述，说明参数的用途
	Description string `yaml:"description"`
	// Default 默认值，当用户未提供参数时使用此值
	// 类型必须与 Type 字段匹配
	Default any `yaml:"default,omitempty"`
	// Required 是否必需，决定调用时是否必须提供此参数
	// 可选值：required（必需）、optional（可选）
	Required ParameterRequirement `yaml:"required,omitempty"`
	// Options 可选项列表，仅当 Type 为 select 时使用
	// 参数值必须是列表中的一个
	Options []string `yaml:"options,omitempty"`
}
