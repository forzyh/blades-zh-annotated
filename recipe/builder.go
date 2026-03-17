package recipe

import (
	"fmt"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/flow"
	"github.com/go-kratos/blades/tools"
)

// BuildOption 配置 Build 过程的函数选项
// 使用函数选项模式（Functional Options Pattern）可以灵活地配置构建过程
// 每个 BuildOption 函数修改 buildOptions 结构体的某个字段
type BuildOption func(*buildOptions)

// buildOptions 构建配置选项的内部结构体
// 这些选项在调用 Build 函数时通过 WithModelRegistry 等函数设置
type buildOptions struct {
	// modelRegistry 模型解析器，用于将 YAML 中的模型名称解析为实际的 ModelProvider
	modelRegistry ModelResolver
	// toolRegistry 工具解析器，用于将 YAML 中的工具名称解析为实际的 Tool
	toolRegistry ToolResolver
	// middlewareRegistry 中间件解析器，用于将 YAML 中的中间件名称解析为实际的 Middleware
	middlewareRegistry MiddlewareResolver
	// params 运行时参数，用于模板渲染
	params map[string]any
}

// WithModelRegistry 设置模型解析器
// 模型解析器用于将 YAML 配置中的模型名称（如 "gpt-4o"）解析为实际的 blades.ModelProvider 实例
// 此选项是必需的，因为配方中的 model 字段只是字符串名称，需要解析器来获取实际的模型提供者
func WithModelRegistry(r ModelResolver) BuildOption {
	return func(o *buildOptions) {
		o.modelRegistry = r
	}
}

// WithToolRegistry 设置工具解析器
// 工具解析器用于将 YAML 配置中的工具名称（如 "search"、"file_read"）解析为实际的 tools.Tool 实例
// 当配方中引用了外部工具时，此选项是必需的
func WithToolRegistry(r ToolResolver) BuildOption {
	return func(o *buildOptions) {
		o.toolRegistry = r
	}
}

// WithMiddlewareRegistry 设置中间件解析器
// 中间件解析器用于将 YAML 配置中的中间件名称解析为实际的 blades.Middleware 实例
// 当配方中引用了中间件时，此选项是必需的
func WithMiddlewareRegistry(r MiddlewareResolver) BuildOption {
	return func(o *buildOptions) {
		o.middlewareRegistry = r
	}
}

// WithParams 设置运行时参数
// 参数用于渲染配方中的 Go template 模板（如 Instruction 和 Prompt 字段）
// 例如，如果 Instruction 包含 "{{.language}} 代码审查"，params 中需要提供 "language" 键
func WithParams(params map[string]any) BuildOption {
	return func(o *buildOptions) {
		o.params = params
	}
}

// Build 从 AgentSpec 构建 blades.Agent
// 这是配方的核心入口点，将 YAML 配置转换为可执行的 Agent 实例
//
// 构建流程：
// 1. 验证配方的有效性（Validate）
// 2. 解析所有 BuildOption 配置
// 3. 合并默认参数和运行时参数
// 4. 根据是否有子 Agent 选择不同的构建策略：
//    - 无子 Agent：构建单个 Agent（buildSingleAgent）
//    - 有子 Agent：根据 Execution 模式构建不同类型的流程 Agent
// 5. 应用 Prompt 注入包装
//
// 使用示例:
//
//	spec, _ := recipe.LoadFromFile("agent.yaml")
//	agent, err := recipe.Build(spec,
//	    recipe.WithModelRegistry(registry),
//	    recipe.WithParams(map[string]any{"language": "go"}),
//	)
func Build(spec *AgentSpec, opts ...BuildOption) (blades.Agent, error) {
	// 第一步：验证配方是否有效
	if err := Validate(spec); err != nil {
		return nil, err
	}
	// 解析构建选项
	o := &buildOptions{}
	for _, opt := range opts {
		opt(o)
	}
	// 模型解析器是必需的，因为每个 Agent 都需要使用模型
	if o.modelRegistry == nil {
		return nil, fmt.Errorf("recipe: model registry is required")
	}

	// 合并默认参数和运行时参数，并验证参数值的有效性
	params := resolveParams(spec.Parameters, o.params)
	if err := ValidateParams(spec, params); err != nil {
		return nil, err
	}

	// 根据是否有子 Agent 选择构建策略
	var (
		agent blades.Agent
		err   error
	)
	if len(spec.SubAgents) == 0 {
		// 无子 Agent：构建单个独立 Agent
		agent, err = buildSingleAgent(spec, params, o)
	} else {
		// 有子 Agent：根据执行模式构建不同类型的流程 Agent
		switch spec.Execution {
		case ExecutionSequential:
			// 顺序执行：子 Agent 依次执行
			agent, err = buildSequentialAgent(spec, params, o)
		case ExecutionParallel:
			// 并行执行：子 Agent 同时执行
			agent, err = buildParallelAgent(spec, params, o)
		case ExecutionLoop:
			// 循环执行：子 Agent 反复执行直到退出条件
			agent, err = buildLoopAgent(spec, params, o)
		case ExecutionTool:
			// 工具模式：子 Agent 作为工具供父 Agent 调用
			agent, err = buildToolAgent(spec, params, o)
		default:
			return nil, fmt.Errorf("recipe: unsupported execution mode %q", spec.Execution)
		}
	}
	if err != nil {
		return nil, err
	}
	// 应用 Prompt 注入包装，如果配置了 Prompt 模板则会在运行时注入
	return withPromptInjection(spec, params, agent)
}

// buildSingleAgent 从没有子 Agent 的 AgentSpec 构建单个 blades.Agent
// 这是最基础的构建函数，处理单一 Agent 的所有配置项
func buildSingleAgent(spec *AgentSpec, params map[string]any, o *buildOptions) (blades.Agent, error) {
	// 1. 解析模型：从模型注册表中获取实际的 ModelProvider
	model, err := o.modelRegistry.Resolve(spec.Model)
	if err != nil {
		return nil, err
	}

	// 2. 构建 Agent 选项列表
	agentOpts := []blades.AgentOption{
		blades.WithModel(model),
	}
	// 如果配置了描述，添加描述选项
	if spec.Description != "" {
		agentOpts = append(agentOpts, blades.WithDescription(spec.Description))
	}

	// 3. 渲染指令模板：使用参数渲染 Instruction 中的 Go template
	// 预渲染只在构建时可以确定的部分，运行时部分保留到实际执行
	instruction, err := renderTemplate(spec.Instruction, params)
	if err != nil {
		return nil, fmt.Errorf("recipe %q: failed to render instruction: %w", spec.Name, err)
	}
	agentOpts = append(agentOpts, blades.WithInstruction(instruction))

	// 4. 配置输出键（可选）：将输出存储到会话状态的指定键
	if spec.OutputKey != "" {
		agentOpts = append(agentOpts, blades.WithOutputKey(spec.OutputKey))
	}
	// 5. 配置最大迭代次数（可选）：限制工具调用次数
	if spec.MaxIterations > 0 {
		agentOpts = append(agentOpts, blades.WithMaxIterations(spec.MaxIterations))
	}

	// 6. 解析外部工具：从工具注册表获取实际的工具实例
	resolvedTools, err := resolveTools(spec.Tools, o)
	if err != nil {
		return nil, fmt.Errorf("recipe %q: %w", spec.Name, err)
	}
	if len(resolvedTools) > 0 {
		agentOpts = append(agentOpts, blades.WithTools(resolvedTools...))
	}

	// 7. 解析中间件：从中间件注册表获取实际的中间件实例
	middlewares, err := resolveMiddlewares(spec.Middlewares, o)
	if err != nil {
		return nil, fmt.Errorf("recipe %q: %w", spec.Name, err)
	}
	if len(middlewares) > 0 {
		agentOpts = append(agentOpts, blades.WithMiddleware(middlewares...))
	}

	// 8. 创建 Agent 实例
	agent, err := blades.NewAgent(spec.Name, agentOpts...)
	if err != nil {
		return nil, err
	}
	// 9. 包装上下文管理器：如果配置了 ContextSpec，则包装为 contextAwareAgent
	return wrapWithContextManager(agent, spec.Context, spec.Model, o.modelRegistry)
}

// buildSubAgent 从 SubAgentSpec 构建 blades.Agent
// parentModel 是父配方的模型名称，用作子 Agent 的默认模型（当子 Agent 未指定模型时）
func buildSubAgent(sub *SubAgentSpec, parentModel string, params map[string]any, o *buildOptions) (blades.Agent, error) {
	// 1. 确定模型：优先使用子 Agent 自己的模型，否则继承父 Agent 的模型
	modelName := sub.Model
	if modelName == "" {
		modelName = parentModel
	}
	if modelName == "" {
		return nil, fmt.Errorf("recipe: sub_agent %q has no model and parent has no model", sub.Name)
	}

	// 2. 从注册表解析模型
	model, err := o.modelRegistry.Resolve(modelName)
	if err != nil {
		return nil, err
	}

	// 3. 构建 Agent 选项
	agentOpts := []blades.AgentOption{
		blades.WithModel(model),
	}
	if sub.Description != "" {
		agentOpts = append(agentOpts, blades.WithDescription(sub.Description))
	}

	// 4. 合并参数：子 Agent 参数与父 Agent 参数合并
	subParams := resolveParams(sub.Parameters, params)
	// 验证子 Agent 参数值的有效性
	if err := validateParamValues(fmt.Sprintf("sub_agent %q", sub.Name), sub.Parameters, subParams); err != nil {
		return nil, err
	}

	// 5. 渲染指令模板
	// 保留未知键（如 {{.syntax_report}}），以便运行时从会话状态解析
	// 这样可以支持在构建时未知但在运行时可用的动态数据
	instruction := sub.Instruction
	if hasTemplateActions(instruction) {
		rendered, err := renderTemplatePreservingUnknown(instruction, subParams)
		if err != nil {
			return nil, fmt.Errorf("sub_agent %q: failed to render instruction: %w", sub.Name, err)
		}
		instruction = rendered
	}
	agentOpts = append(agentOpts, blades.WithInstruction(instruction))

	// 6. 配置可选选项
	if sub.OutputKey != "" {
		agentOpts = append(agentOpts, blades.WithOutputKey(sub.OutputKey))
	}
	if sub.MaxIterations > 0 {
		agentOpts = append(agentOpts, blades.WithMaxIterations(sub.MaxIterations))
	}

	// 7. 解析工具和中间件
	resolvedTools, err := resolveTools(sub.Tools, o)
	if err != nil {
		return nil, fmt.Errorf("sub_agent %q: %w", sub.Name, err)
	}
	if len(resolvedTools) > 0 {
		agentOpts = append(agentOpts, blades.WithTools(resolvedTools...))
	}

	// 解析中间件
	middlewares, err := resolveMiddlewares(sub.Middlewares, o)
	if err != nil {
		return nil, fmt.Errorf("sub_agent %q: %w", sub.Name, err)
	}
	if len(middlewares) > 0 {
		agentOpts = append(agentOpts, blades.WithMiddleware(middlewares...))
	}

	// 8. 创建子 Agent 实例
	agent, err := blades.NewAgent(sub.Name, agentOpts...)
	if err != nil {
		return nil, err
	}
	// 9. 应用 Prompt 模板包装
	agent, err = withPromptTemplate(agent, fmt.Sprintf("sub_agent %q", sub.Name), sub.Prompt, subParams)
	if err != nil {
		return nil, err
	}
	// 10. 包装上下文管理器
	return wrapWithContextManager(agent, sub.Context, modelName, o.modelRegistry)
}

// buildSequentialAgent 从子 Agent 列表构建顺序执行的流程 Agent
// 顺序执行意味着子 Agent 一个接一个地执行，前一个的输出可能影响下一个的执行
func buildSequentialAgent(spec *AgentSpec, params map[string]any, o *buildOptions) (blades.Agent, error) {
	// 1. 依次构建所有子 Agent
	subAgents := make([]blades.Agent, 0, len(spec.SubAgents))
	for i := range spec.SubAgents {
		agent, err := buildSubAgent(&spec.SubAgents[i], spec.Model, params, o)
		if err != nil {
			return nil, fmt.Errorf("recipe %q: %w", spec.Name, err)
		}
		subAgents = append(subAgents, agent)
	}
	// 2. 创建 SequentialAgent 流程，按顺序执行所有子 Agent
	return flow.NewSequentialAgent(flow.SequentialConfig{
		Name:        spec.Name,
		Description: spec.Description,
		SubAgents:   subAgents,
	}), nil
}

// buildParallelAgent 从子 Agent 列表构建并行执行的流程 Agent
// 并行执行意味着所有子 Agent 同时开始执行，最后汇总结果
func buildParallelAgent(spec *AgentSpec, params map[string]any, o *buildOptions) (blades.Agent, error) {
	// 1. 依次构建所有子 Agent
	subAgents := make([]blades.Agent, 0, len(spec.SubAgents))
	for i := range spec.SubAgents {
		agent, err := buildSubAgent(&spec.SubAgents[i], spec.Model, params, o)
		if err != nil {
			return nil, fmt.Errorf("recipe %q: %w", spec.Name, err)
		}
		subAgents = append(subAgents, agent)
	}
	// 2. 创建 ParallelAgent 流程，同时执行所有子 Agent
	return flow.NewParallelAgent(flow.ParallelConfig{
		Name:        spec.Name,
		Description: spec.Description,
		SubAgents:   subAgents,
	}), nil
}

// buildLoopAgent 从子 Agent 列表构建循环执行的流程 Agent
// 循环执行会反复执行子 Agent，直到达到 max_iterations 上限或某个子 Agent 通过 loop_exit 工具信号退出
// LoopCondition 不在配方 YAML 中支持，退出条件通过工具调用来实现
func buildLoopAgent(spec *AgentSpec, params map[string]any, o *buildOptions) (blades.Agent, error) {
	// 1. 依次构建所有子 Agent
	subAgents := make([]blades.Agent, 0, len(spec.SubAgents))
	for i := range spec.SubAgents {
		agent, err := buildSubAgent(&spec.SubAgents[i], spec.Model, params, o)
		if err != nil {
			return nil, fmt.Errorf("recipe %q: %w", spec.Name, err)
		}
		subAgents = append(subAgents, agent)
	}
	// 2. 创建 LoopAgent 流程，循环执行子 Agent 直到退出条件
	return flow.NewLoopAgent(flow.LoopConfig{
		Name:          spec.Name,
		Description:   spec.Description,
		MaxIterations: spec.MaxIterations,
		SubAgents:     subAgents,
	}), nil
}

// buildToolAgent 构建工具模式的 Agent
// 工具模式下，子 Agent 被包装成工具，由父 Agent 根据需要动态调用
// 这适用于需要决策能力的场景，父 Agent 可以智能选择调用哪个子 Agent
func buildToolAgent(spec *AgentSpec, params map[string]any, o *buildOptions) (blades.Agent, error) {
	// 1. 解析父 Agent 的模型（工具模式需要父 Agent 有模型来做决策）
	model, err := o.modelRegistry.Resolve(spec.Model)
	if err != nil {
		return nil, err
	}

	// 2. 将每个子 Agent 构建后包装成工具
	agentTools := make([]tools.Tool, 0, len(spec.SubAgents))
	for i := range spec.SubAgents {
		subAgent, err := buildSubAgent(&spec.SubAgents[i], spec.Model, params, o)
		if err != nil {
			return nil, fmt.Errorf("recipe %q: %w", spec.Name, err)
		}
		// 将子 Agent 包装成工具，供父 Agent 调用
		agentTools = append(agentTools, blades.NewAgentTool(subAgent))
	}

	// 3. 解析额外的外部工具（如果有）
	externalTools, err := resolveTools(spec.Tools, o)
	if err != nil {
		return nil, fmt.Errorf("recipe %q: %w", spec.Name, err)
	}
	// 合并子 Agent 工具和外部工具
	allTools := append(agentTools, externalTools...)

	// 4. 渲染指令模板
	instruction, err := renderTemplate(spec.Instruction, params)
	if err != nil {
		return nil, fmt.Errorf("recipe %q: failed to render instruction: %w", spec.Name, err)
	}

	// 5. 构建 Agent 选项
	agentOpts := []blades.AgentOption{
		blades.WithModel(model),
		blades.WithInstruction(instruction),
		blades.WithTools(allTools...),
	}
	if spec.Description != "" {
		agentOpts = append(agentOpts, blades.WithDescription(spec.Description))
	}
	if spec.OutputKey != "" {
		agentOpts = append(agentOpts, blades.WithOutputKey(spec.OutputKey))
	}
	if spec.MaxIterations > 0 {
		agentOpts = append(agentOpts, blades.WithMaxIterations(spec.MaxIterations))
	}

	// 6. 解析中间件
	middlewares, err := resolveMiddlewares(spec.Middlewares, o)
	if err != nil {
		return nil, fmt.Errorf("recipe %q: %w", spec.Name, err)
	}
	if len(middlewares) > 0 {
		agentOpts = append(agentOpts, blades.WithMiddleware(middlewares...))
	}

	// 7. 创建工具模式 Agent
	return blades.NewAgent(spec.Name, agentOpts...)
}

// resolveTools 将工具名称列表解析为实际的 tools.Tool 实例
// 此函数从 ToolRegistry 中按名称查找工具，如果找不到则返回错误
func resolveTools(names []string, o *buildOptions) ([]tools.Tool, error) {
	if len(names) == 0 {
		return nil, nil
	}
	// 当配方引用了工具时，工具注册表是必需的
	if o.toolRegistry == nil {
		return nil, fmt.Errorf("tool registry is required when tools are referenced")
	}
	// 逐个解析工具名称
	resolved := make([]tools.Tool, 0, len(names))
	for _, name := range names {
		t, err := o.toolRegistry.Resolve(name)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, t)
	}
	return resolved, nil
}

// resolveMiddlewares 将 MiddlewareSpec 列表解析为实际的 blades.Middleware 实例
// 每个 MiddlewareSpec 包含中间件名称和选项，选项会传递给工厂函数
func resolveMiddlewares(specs []MiddlewareSpec, o *buildOptions) ([]blades.Middleware, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	// 当配方引用了中间件时，中间件注册表是必需的
	if o.middlewareRegistry == nil {
		return nil, fmt.Errorf("middleware registry is required when middlewares are referenced")
	}
	// 逐个解析中间件，每个中间件使用其配置的选项
	resolved := make([]blades.Middleware, 0, len(specs))
	for _, spec := range specs {
		mw, err := o.middlewareRegistry.Resolve(spec.Name, spec.Options)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, mw)
	}
	return resolved, nil
}
