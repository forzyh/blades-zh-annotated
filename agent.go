// Package blades 提供了一个用于构建 AI 代理（Agent）的轻量级框架。
// Blades 核心包定义了代理的基本抽象，包括 Agent 接口、消息处理、工具调用、会话管理等。
// 通过组合不同的组件（如 ModelProvider、Tools、Skills、Middleware），可以构建复杂的 AI 应用。
package blades

import (
	"context"
	"fmt"
	"html/template"
	"strings"
	"sync"

	"github.com/go-kratos/blades/skills"
	"github.com/go-kratos/blades/tools"
	"github.com/go-kratos/kit/container/maps"
	"github.com/google/jsonschema-go/jsonschema"
	"golang.org/x/sync/errgroup"
)

// InstructionProvider 是一个函数类型，用于根据上下文动态生成代理的系统指令。
// 为什么需要动态指令？在某些场景下，指令需要根据会话状态、用户配置或外部数据源动态生成，
// 而不是在创建代理时静态指定。这提供了更大的灵活性。
// 例如，可以根据当前会话的状态（State）中的值来定制指令内容。
type InstructionProvider func(ctx context.Context) (string, error)

// AgentOption 是用于配置 Agent 的选项函数类型。
// 采用函数选项模式（Functional Options Pattern）的好处：
// 1. 提供灵活的配置方式，可以有零个或多个选项
// 2. 保持构造函数的简洁，不需要为不同配置组合创建多个构造函数
// 3. 易于扩展，添加新选项时不需要修改现有代码
type AgentOption func(*agent)

// WithModel 设置 Agent 使用的模型提供者。
// 模型提供者负责与实际的 AI 模型（如 Claude、GPT-4 等）进行通信。
// 这是必选选项，创建 Agent 时必须提供。
func WithModel(model ModelProvider) AgentOption {
	return func(a *agent) {
		a.model = model
	}
}

// WithDescription 设置 Agent 的功能描述。
// 描述用于说明代理的用途和能力，通常在将 Agent 作为工具嵌套使用时很重要。
func WithDescription(description string) AgentOption {
	return func(a *agent) {
		a.description = description
	}
}

// WithInstruction 设置 Agent 的静态系统指令。
// 系统指令用于定义代理的行为准则、任务目标和工作方式。
// 例如："你是一个专业的代码审查助手，负责检查代码质量并提供改进建议。"
// 注意：静态指令不支持模板变量，如需动态内容请使用 WithInstructionProvider。
func WithInstruction(instruction string) AgentOption {
	return func(a *agent) {
		a.instruction = instruction
	}
}

// WithInstructionProvider 设置动态指令提供者。
// 当需要根据运行时上下文（如会话状态）动态生成指令时使用。
// 优先级：静态指令 > InstructionProvider > Skills 指令 > 调用时指令
func WithInstructionProvider(p InstructionProvider) AgentOption {
	return func(a *agent) {
		a.instructionProvider = p
	}
}

// WithInputSchema 设置 Agent 的输入 JSON Schema。
// Schema 用于验证和约束输入格式，帮助模型理解预期的输入结构。
// 当 Agent 作为工具被其他 Agent 调用时，Schema 会传递给调用方作为参考。
func WithInputSchema(schema *jsonschema.Schema) AgentOption {
	return func(a *agent) {
		a.inputSchema = schema
	}
}

// WithOutputSchema 设置 Agent 的输出 JSON Schema。
// Schema 用于约束输出格式，确保模型返回结构化的数据。
// 例如，可以定义输出必须包含特定字段，如 {"result": string, "confidence": number}。
func WithOutputSchema(schema *jsonschema.Schema) AgentOption {
	return func(a *agent) {
		a.outputSchema = schema
	}
}

// WithOutputKey 设置 Agent 输出结果在会话状态（Session State）中的存储键名。
// 当需要将 Agent 的输出保存到会话状态以供后续 Agent 使用时非常有用。
// 例如，设置 "search_result" 后，Agent 的输出会自动存储在 session.State()["search_result"]。
func WithOutputKey(key string) AgentOption {
	return func(a *agent) {
		a.outputKey = key
	}
}

// WithTools 设置 Agent 可以使用的工具列表。
// 工具是 Agent 执行特定任务的能力扩展，如搜索、文件读写、代码执行等。
// 工具会被转换为 Model 可理解的格式，由模型决定何时调用。
func WithTools(tools ...tools.Tool) AgentOption {
	return func(a *agent) {
		a.tools = tools
	}
}

// WithSkills 设置 Agent 的技能列表。
// Skills 是 Blades 框架提供的一种高级抽象，可以组合多个工具形成复合能力。
// Skills 会自动生成系统指令和工具定义，简化 Agent 配置。
func WithSkills(skillList ...skills.Skill) AgentOption {
	return func(a *agent) {
		a.skills = skillList
	}
}

// WithToolsResolver 设置工具解析器，用于动态获取工具。
// 解析器支持从多种来源（如 MCP 服务器、插件系统）懒加载工具。
// 工具在首次使用时才会被解析，这有助于减少启动开销和按需加载。
func WithToolsResolver(r tools.Resolver) AgentOption {
	return func(a *agent) {
		a.toolsResolver = r
	}
}

// WithMiddleware 设置 Agent 的中间件链。
// 中间件用于在请求处理前后执行额外逻辑，如日志记录、错误处理、权限检查等。
// 多个中间件按添加顺序从外到内执行（类似洋葱模型）。
func WithMiddleware(ms ...Middleware) AgentOption {
	return func(a *agent) {
		a.middlewares = ms
	}
}

// WithMaxIterations 设置 Agent 的最大迭代次数。
// 迭代发生在 Agent 调用工具并等待结果的过程中。
// 默认值为 10，防止无限循环。当 Agent 陷入工具调用循环时会达到此限制。
func WithMaxIterations(n int) AgentOption {
	return func(a *agent) {
		a.maxIterations = n
	}
}

// agent 是 Agent 接口的内部实现，包含了代理的所有核心数据和逻辑。
// 使用小写名称表示这是包内私有类型，外部通过 Agent 接口和 NewAgent 函数交互。
// 字段说明：
// - name: 代理的唯一标识名称
// - description: 代理功能描述，用于工具嵌套时向调用方说明
// - instruction: 静态系统指令
// - instructionProvider: 动态指令生成器
// - outputKey: 输出存储键名
// - maxIterations: 最大工具调用迭代次数
// - model: 底层 AI 模型提供者
// - inputSchema/outputSchema: 输入输出数据结构约束
// - middlewares: 中间件链
// - tools: 静态配置的工具列表
// - skills: 技能列表（高级工具抽象）
// - skillToolset: Skills 对应的工具集（内部使用）
// - toolsResolver: 动态工具解析器
type agent struct {
	name                string
	description         string
	instruction         string
	instructionProvider InstructionProvider
	outputKey           string
	maxIterations       int
	model               ModelProvider
	inputSchema         *jsonschema.Schema
	outputSchema        *jsonschema.Schema
	middlewares         []Middleware
	tools               []tools.Tool
	skills              []skills.Skill
	skillToolset        *skills.Toolset
	toolsResolver       tools.Resolver // 可选的动态工具解析器（如 MCP 服务器）
}

// NewAgent 创建一个新的 Agent 实例。
// 参数：
//   - name: 代理的名称，用于标识和日志记录
//   - opts: 可选的配置选项，通过 WithModel、WithInstruction 等函数设置
//
// 返回：
//   - Agent: 创建好的代理实例
//   - error: 创建失败时的错误（如缺少必需的 ModelProvider）
//
// 使用示例：
//
//	agent, err := blades.NewAgent("助手",
//	    blades.WithModel(myModel),
//	    blades.WithInstruction("你是一个有帮助的助手"),
//	    blades.WithTools(searchTool, fileTool),
//	)
func NewAgent(name string, opts ...AgentOption) (Agent, error) {
	a := &agent{
		name:          name,
		maxIterations: 10, // 默认最大迭代次数为 10
	}
	for _, opt := range opts {
		opt(a)
	}
	// 模型提供者是必需的，因为没有模型就无法进行 AI 推理
	if a.model == nil {
		return nil, ErrModelProviderRequired
	}
	// 如果配置了 Skills，需要将其转换为内部工具集
	if len(a.skills) > 0 {
		toolset, err := skills.NewToolset(a.skills)
		if err != nil {
			return nil, err
		}
		a.skillToolset = toolset
	}
	return a, nil
}

// Name 返回代理的名称。
// 实现 Agent 接口的方法。
func (a *agent) Name() string {
	return a.name
}

// Description 返回代理的功能描述。
// 实现 Agent 接口的方法。
func (a *agent) Description() string {
	return a.description
}

// resolveTools 合并静态工具和动态解析的工具。
// 工具解析是懒加载的，只在第一次调用时执行。
// 返回的工具列表会被传递给模型，模型根据工具定义决定调用哪个工具。
func (a *agent) resolveTools(ctx context.Context) ([]tools.Tool, error) {
	tools := make([]tools.Tool, 0, len(a.tools))
	// 首先添加静态配置的工具
	if len(a.tools) > 0 {
		tools = append(tools, a.tools...)
	}
	// 如果配置了动态解析器，解析并添加动态工具
	if a.toolsResolver != nil {
		resolved, err := a.toolsResolver.Resolve(ctx)
		if err != nil {
			return nil, err
		}
		tools = append(tools, resolved...)
	}
	return tools, nil
}

// prepareInvocation 在代理执行前准备 Invocation 对象。
// 主要工作：
// 1. 解析工具（静态 + 动态）
// 2. 设置模型名称
// 3. 合并 Skills 生成的指令和工具
// 4. 处理静态指令和动态指令（支持 Go template 语法）
//
// 指令优先级（从高到低）：
// 1. WithInstruction 设置的静态指令
// 2. WithInstructionProvider 设置的动态指令
// 3. Skills 自动生成的指令
// 4. Invocation 自带的指令
//
// 静态指令支持 Go template 语法，可以使用 session.State() 中的变量。
// 例如："当前用户是 {{.username}}，请根据他的偏好回答问题。"
func (a *agent) prepareInvocation(ctx context.Context, invocation *Invocation) error {
	resolvedTools, err := a.resolveTools(ctx)
	if err != nil {
		return err
	}
	invocation.Model = a.model.Name()
	finalTools := resolvedTools
	// 如果配置了 Skills，将其与解析的工具组合
	if a.skillToolset != nil {
		finalTools = a.skillToolset.ComposeTools(resolvedTools)
		// Skills 会生成额外的系统指令
		invocation.Instruction = MergeParts(SystemMessage(a.skillToolset.Instruction()), invocation.Instruction)
	}
	invocation.Tools = append(invocation.Tools, finalTools...)
	// 按优先级顺序处理指令
	if a.instructionProvider != nil {
		instruction, err := a.instructionProvider(ctx)
		if err != nil {
			return err
		}
		invocation.Instruction = MergeParts(SystemMessage(instruction), invocation.Instruction)
	}
	if a.instruction != "" {
		if invocation.Session != nil {
			var buf strings.Builder
			// 使用 Go template 解析指令，支持会话状态变量
			t, err := template.New("instruction").Parse(a.instruction)
			if err != nil {
				return err
			}
			if err := t.Execute(&buf, invocation.Session.State()); err != nil {
				return err
			}
			invocation.Instruction = MergeParts(SystemMessage(buf.String()), invocation.Instruction)
		} else {
			invocation.Instruction = MergeParts(SystemMessage(a.instruction), invocation.Instruction)
		}
	}
	return nil
}

// findResumeMessages 检查是否可以从之前的状态恢复执行。
// 通过查找会话历史中当前调制的已完成消息来判断。
//
// 参数：
//   - invocation: 当前调用对象，包含会话历史和调用 ID
//
// 返回：
//   - []*Message: 需要恢复的消息列表
//   - bool: 是否可以恢复（true 表示找到已完成的助手消息，可以直接返回结果）
//
// 恢复逻辑：
// 1. 只有在 invocation.Resume=true 且 Session 不为 nil 时才尝试恢复
// 2. 遍历会话历史，查找与当前调用 ID 匹配的消息
// 3. 如果找到当前代理（author == a.name）的已完成消息，表示之前已经执行完成，可以恢复
func (a *agent) findResumeMessages(invocation *Invocation) ([]*Message, bool) {
	if !invocation.Resume || invocation.Session == nil {
		return nil, false
	}
	resumeHistory := invocation.Session.History()
	resumeMessages := make([]*Message, 0, len(resumeHistory))
	for _, m := range resumeHistory {
		if m.InvocationID != invocation.ID {
			continue
		}
		if m.Author == a.name {
			resumeMessages = append(resumeMessages, m)
			// 找到已完成的助手消息，表示之前执行已完成，可以直接恢复
			if m.Role == RoleAssistant && m.Status == StatusCompleted {
				return resumeMessages, true
			}
		}
	}
	return resumeMessages, false
}

// Run 是 Agent 的核心执行方法，处理调用并生成消息序列。
// 该方法返回一个 Generator（迭代器），支持流式和非流式两种执行模式。
//
// 执行流程：
// 1. 检查是否可以恢复（findResumeMessages）
//    - 如果可以恢复，直接返回之前保存的消息
// 2. 准备 Invocation（prepareInvocation）
//    - 解析工具、设置模型名称、处理指令
// 3. 将当前 Agent 注入上下文（NewAgentContext）
// 4. 构建 Handler 处理核心逻辑
//    - 构建 ModelRequest，包含工具、消息、Schema 等
//    - 根据是否有恢复消息或用户消息设置请求内容
// 5. 应用中间件链（ChainMiddlewares）
// 6. 执行 Handler 并生成消息流
//
// 参数：
//   - ctx: 上下文，用于取消和传递值
//   - invocation: 调用对象，包含消息、会话、工具等
//
// 返回：
//   - Generator[*Message, error]: 消息迭代器，依次产生消息或错误
func (a *agent) Run(ctx context.Context, invocation *Invocation) Generator[*Message, error] {
	return func(yield func(*Message, error) bool) {
		// 步骤 1: 检查是否可以恢复
		resumeMessages, ok := a.findResumeMessages(invocation)
		if ok {
			// 可以恢复，直接返回之前保存的消息
			for _, resumeMessage := range resumeMessages {
				if !yield(resumeMessage, nil) {
					return
				}
			}
			return
		}
		// 步骤 2: 准备 Invocation
		if err := a.prepareInvocation(ctx, invocation); err != nil {
			yield(nil, err)
			return
		}
		// 步骤 3: 将 Agent 注入上下文
		ctx = NewAgentContext(ctx, a)
		// 步骤 4: 构建 Handler
		handler := Handler(HandleFunc(func(ctx context.Context, invocation *Invocation) Generator[*Message, error] {
			req := &ModelRequest{
				Tools:        invocation.Tools,
				Instruction:  invocation.Instruction,
				InputSchema:  a.inputSchema,
				OutputSchema: a.outputSchema,
			}
			// 根据情况设置请求消息
			switch {
			case len(resumeMessages) > 0:
				// 有恢复消息，使用恢复的历史
				req.Messages = AppendMessages(req.Messages, resumeMessages...)
			case invocation.Message != nil:
				// 有用户消息，添加到请求
				req.Messages = AppendMessages(req.Messages, invocation.Message)
			}
			return a.handle(ctx, invocation, req)
		}))
		// 步骤 5: 应用中间件
		if len(a.middlewares) > 0 {
			handler = ChainMiddlewares(a.middlewares...)(handler)
		}
		// 步骤 6: 执行 Handler 并生成消息流
		stream := handler.Handle(ctx, invocation)
		for m, err := range stream {
			if !yield(m, err) {
				break
			}
		}
	}
}

// saveOutputState 将 Agent 的输出保存到会话状态中。
// 当设置了 outputKey 且消息是已完成的助手消息时，将输出文本存储到 session.State()[outputKey]。
// 这使得后续 Agent 可以访问之前 Agent 的输出结果，实现数据传递。
func (a *agent) saveOutputState(ctx context.Context, invocation *Invocation, message *Message) error {
	// 保存输出到会话状态的条件：
	// 1. outputKey 不为空（显式配置了存储键）
	// 2. Session 存在
	// 3. 消息是助手角色且已完成
	if a.outputKey != "" &&
		invocation.Session != nil &&
		message.Role == RoleAssistant &&
		message.Status == StatusCompleted {
		// 将消息文本存储在指定的键下
		invocation.Session.SetState(a.outputKey, message.Text())
	}
	return nil
}

// handleTools 查找并执行指定的工具，返回工具响应。
// 遍历所有可用工具（静态 + 动态），找到名称匹配的工具并调用其 Handle 方法。
//
// 参数：
//   - ctx: 上下文
//   - invocation: 当前调用，包含工具列表
//   - part: 工具调用部分，包含工具名称和请求参数
//
// 返回：
//   - ToolPart: 更新后的工具部分，包含响应结果
//   - error: 工具未找到或执行失败时的错误
func (a *agent) handleTools(ctx context.Context, invocation *Invocation, part ToolPart) (ToolPart, error) {
	// 遍历所有可用工具
	for _, tool := range invocation.Tools {
		if tool.Name() == part.Name {
			response, err := tool.Handle(ctx, part.Request)
			if err != nil {
				return part, err
			}
			part.Response = response
			return part, nil
		}
	}
	return part, fmt.Errorf("agent: tool %s not found", part.Name)
}

// executeTools 并发执行消息中指定的所有工具调用。
// 当模型返回的消息包含 ToolPart（工具调用）时，该方法负责执行这些工具并填充响应。
//
// 实现细节：
// 1. 使用 sync.Mutex 保护并发修改 message.Parts 和 message.Actions
// 2. 使用 errgroup 并发执行所有工具调用，提高效率
// 3. 为每个工具调用创建独立的上下文，包含工具元数据（ID、名称、Actions）
// 4. 执行完成后标记 ToolPart.Completed = true
//
// 参数：
//   - ctx: 上下文
//   - invocation: 当前调用
//   - message: 包含工具调用的消息
//
// 返回：
//   - *Message: 更新后的消息（工具响应已填充）
//   - error: 任一工具执行失败时的错误
func (a *agent) executeTools(ctx context.Context, invocation *Invocation, message *Message) (*Message, error) {
	var (
		m sync.Mutex // 保护并发修改
	)
	actions := maps.New(message.Actions)
	// errgroup 用于并发执行工具调用，并等待所有任务完成
	eg, ctx := errgroup.WithContext(ctx)
	for i, part := range message.Parts {
		switch v := any(part).(type) {
		case ToolPart:
			eg.Go(func() error {
				// 已完成的工具跳过
				if v.Completed {
					return nil
				}
				// 创建工具上下文，包含工具 ID、名称和 Actions 映射
				toolCtx := tools.NewContext(ctx, &toolContext{
					id:      v.ID,
					name:    v.Name,
					actions: actions,
				})
				// 执行工具
				part, err := a.handleTools(toolCtx, invocation, v)
				if err != nil {
					return err
				}
				// 标记为已完成
				part.Completed = true
				// 更新消息（需要加锁）
				m.Lock()
				message.Parts[i] = part
				message.Actions = MergeActions(message.Actions, actions.ToMap())
				m.Unlock()
				return nil
			})
		}
	}
	// 等待所有工具调用完成
	return message, eg.Wait()
}

// messageFromResponse 将 ModelResponse 转换为 Message。
// 处理空响应的情况，返回适当的错误。
func messageFromResponse(response *ModelResponse) (*Message, error) {
	if response == nil || response.Message == nil {
		return nil, ErrNoFinalResponse
	}
	return response.Message, nil
}

// handle 是 Agent 的核心处理方法，负责与模型交互并处理工具调用循环。
// 该方法实现了 Agent 的主要执行逻辑，支持流式和非流式两种模式。
//
// 执行流程（主循环）：
// 1. 应用 ContextManager（如果有）准备消息上下文
//    - 用于处理上下文窗口限制，如滑动窗口截断或摘要压缩
// 2. 调用模型生成响应
//    - 非流式：一次性获取完整响应
//    - 流式：逐步接收响应片段
// 3. 处理响应消息
//    - 保存输出状态（如果配置了 outputKey）
//    - 生成消息到输出流
// 4. 检查是否需要调用工具（Role == RoleTool）
//    - 执行工具调用（executeTools）
//    - 将工具响应添加到消息历史
//    - 继续下一次迭代
// 5. 如果不是工具调用，表示完成，返回
//
// 循环终止条件：
// - 模型返回非工具消息（任务完成）
// - 达到最大迭代次数（ErrMaxIterationsExceeded）
// - 发生错误
//
// 参数：
//   - ctx: 上下文
//   - invocation: 当前调用
//   - req: 模型请求，包含消息、工具、Schema 等
//
// 返回：
//   - Generator[*Message, error]: 消息迭代器
func (a *agent) handle(ctx context.Context, invocation *Invocation, req *ModelRequest) Generator[*Message, error] {
	return func(yield func(*Message, error) bool) {
		contextManager, _ := ContextManagerFromContext(ctx)
		// 主循环：处理工具调用迭代
		for i := 0; i < a.maxIterations; i++ {
			// 应用上下文窗口管理（每次迭代前都执行）
			// 这 handles 初始历史和工具调用过程中累积的消息
			if contextManager != nil {
				prepared, err := contextManager.Prepare(ctx, req.Messages)
				if err != nil {
					yield(nil, err)
					return
				}
				req.Messages = prepared
			}
			var finalMessage *Message
			// 非流式模式：一次性获取完整响应
			if !invocation.Stream {
				finalResponse, err := a.model.Generate(ctx, req)
				if err != nil {
					yield(nil, err)
					return
				}
				finalMessage, err = messageFromResponse(finalResponse)
				if err != nil {
					yield(nil, err)
					return
				}
				// 设置作者和调用 ID
				if finalMessage.Author == "" {
					finalMessage.Author = a.name
				}
				finalMessage.InvocationID = invocation.ID
				// 跳过保存工具中间状态
				if finalMessage.Role == RoleAssistant {
					if err := a.saveOutputState(ctx, invocation, finalMessage); err != nil {
						yield(nil, err)
						return
					}
					if !yield(finalMessage, nil) {
						return
					}
				}
			} else {
				// 流式模式：逐步接收响应
				streaming := a.model.NewStreaming(ctx, req)
				for response, err := range streaming {
					if err != nil {
						yield(nil, err)
						return
					}
					finalMessage, err = messageFromResponse(response)
					if err != nil {
						yield(nil, err)
						return
					}
					if finalMessage.Author == "" {
						finalMessage.Author = a.name
					}
					finalMessage.InvocationID = invocation.ID
					// 跳过工具响应中间状态
					if finalMessage.Role == RoleTool && finalMessage.Status == StatusCompleted {
						continue
					}
					if err := a.saveOutputState(ctx, invocation, finalMessage); err != nil {
						yield(nil, err)
						return
					}
					if !yield(finalMessage, nil) {
						return // 早期终止
					}
				}
			}
			// 检查是否获取到有效消息
			if finalMessage == nil {
				yield(nil, ErrNoFinalResponse)
				return
			}
			if invocation.Stream && finalMessage.Status != StatusCompleted {
				yield(nil, ErrNoFinalResponse)
				return
			}
			// 如果是工具调用，执行工具并继续下一次迭代
			if finalMessage.Role == RoleTool {
				toolMessage, err := a.executeTools(ctx, invocation, finalMessage)
				if err != nil {
					yield(nil, err)
					return
				}
				if !yield(toolMessage, nil) {
					return
				}
				// 将工具响应添加到历史，供下一次迭代使用
				req.Messages = append(req.Messages, toolMessage)
				continue // 继续下一次迭代
			}
			// 不是工具调用，任务完成
			return
		}
		// 超过最大迭代次数
		yield(nil, ErrMaxIterationsExceeded)
	}
}
