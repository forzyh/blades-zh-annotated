package recipe

import (
	"context"
	"fmt"

	"github.com/go-kratos/blades"
)

// promptInjectedAgent 是一个 Agent 包装器，用于在运行时注入 Prompt
// 它包装基础 Agent，在每次调用时将配置的 Prompt 注入到请求中
//
// Prompt 注入与 Instruction 的区别：
// - Instruction 在构建时渲染，是 Agent 的固定系统指令
// - Prompt 在运行时渲染，是每次调用时动态注入的用户级指令
type promptInjectedAgent struct {
	// 被包装的基础 Agent
	base blades.Agent
	// 要注入的 Prompt 内容
	prompt string
}

// Name 返回 Agent 的名称（委托给基础 Agent）
func (a *promptInjectedAgent) Name() string {
	return a.base.Name()
}

// Description 返回 Agent 的描述（委托给基础 Agent）
func (a *promptInjectedAgent) Description() string {
	return a.base.Description()
}

// Run 执行 Agent 调用，在调用前注入 Prompt
// 如果 invocation 为空或没有配置 Prompt，则直接调用基础 Agent
func (a *promptInjectedAgent) Run(ctx context.Context, inv *blades.Invocation) blades.Generator[*blades.Message, error] {
	// 没有有效的 invocation 或没有配置 Prompt，直接调用基础 Agent
	if inv == nil || a.prompt == "" {
		return a.base.Run(ctx, inv)
	}
	// 克隆 invocation，避免修改原始对象
	next := inv.Clone()
	// 创建用户消息形式的 Prompt
	promptMessage := blades.UserMessage(a.prompt)
	if next.Message == nil {
		// 如果没有消息，将 Prompt 作为用户消息
		next.Message = promptMessage
	} else {
		// 如果有消息，将 Prompt 作为系统指令合并到现有指令中
		next.Instruction = blades.MergeParts(blades.SystemMessage(a.prompt), next.Instruction)
	}
	// 使用注入后的 invocation 调用基础 Agent
	return a.base.Run(ctx, next)
}

// withPromptInjection 从 AgentSpec 构建 Prompt 注入包装
// 如果配置了 Prompt 模板，则渲染模板并包装 Agent 为 promptInjectedAgent
func withPromptInjection(spec *AgentSpec, params map[string]any, base blades.Agent) (blades.Agent, error) {
	// 委托给 withPromptTemplate 处理具体的模板渲染和包装逻辑
	return withPromptTemplate(base, fmt.Sprintf("recipe %q", spec.Name), spec.Prompt, params)
}

// withPromptTemplate 渲染 Prompt 模板并包装 Agent
// scope 用于错误报告中标识当前处理的配方或子 Agent
func withPromptTemplate(base blades.Agent, scope string, promptTemplate string, params map[string]any) (blades.Agent, error) {
	// 没有配置 Prompt 模板，直接返回原 Agent
	if promptTemplate == "" {
		return base, nil
	}
	// 渲染 Prompt 模板，替换 {{.param}} 占位符
	prompt, err := renderTemplate(promptTemplate, params)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to render prompt: %w", scope, err)
	}
	// 用渲染后的 Prompt 包装 Agent
	return &promptInjectedAgent{
		base:   base,
		prompt: prompt,
	}, nil
}
