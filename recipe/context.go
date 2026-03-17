package recipe

import (
	"context"
	"fmt"
	"iter"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/context/summary"
	"github.com/go-kratos/blades/context/window"
)

// buildContextManager 从 ContextSpec 构建 blades.ContextManager
// ContextManager 负责管理 Agent 的对话上下文，防止上下文过长超出 Token 限制
// fallbackModelName 当 ContextSpec.Model 为空时用作摘要模型
//
// 上下文管理的两种策略：
// 1. Summarize（摘要）：使用 LLM 对旧消息进行摘要，保留核心信息
// 2. Window（窗口）：保留最近的消息，截断最旧的消息
func buildContextManager(spec *ContextSpec, reg ModelResolver, fallbackModelName string) (blades.ContextManager, error) {
	// 如果没有配置上下文规范，返回 nil 表示不使用上下文管理
	if spec == nil {
		return nil, nil
	}
	// 根据策略类型选择不同的上下文管理器实现
	switch spec.Strategy {
	case ContextStrategySummarize:
		// 摘要策略：使用 LLM 滚动摘要压缩旧消息
		opts := []summary.Option{}
		// 配置最大 Token 数，超出时触发摘要
		if spec.MaxTokens > 0 {
			opts = append(opts, summary.WithMaxTokens(spec.MaxTokens))
		}
		// 配置始终保留的最近消息数量
		if spec.KeepRecent > 0 {
			opts = append(opts, summary.WithKeepRecent(spec.KeepRecent))
		}
		// 配置每次摘要处理的批量大小
		if spec.BatchSize > 0 {
			opts = append(opts, summary.WithBatchSize(spec.BatchSize))
		}
		// 确定摘要使用的模型：优先使用配置的模型，否则回退到 Agent 的模型
		modelName := spec.Model
		if modelName == "" {
			modelName = fallbackModelName
		}
		if modelName != "" {
			// 从模型注册表解析摘要模型
			model, err := reg.Resolve(modelName)
			if err != nil {
				return nil, fmt.Errorf("recipe: context model: %w", err)
			}
			opts = append(opts, summary.WithSummarizer(model))
		}
		// 创建摘要上下文管理器
		return summary.NewContextManager(opts...), nil

	case ContextStrategyWindow:
		// 窗口策略：滑动窗口截断旧消息
		opts := []window.Option{}
		// 配置最大 Token 数
		if spec.MaxTokens > 0 {
			opts = append(opts, window.WithMaxTokens(spec.MaxTokens))
		}
		// 配置最大消息数量
		if spec.MaxMessages > 0 {
			opts = append(opts, window.WithMaxMessages(spec.MaxMessages))
		}
		// 创建窗口上下文管理器
		return window.NewContextManager(opts...), nil

	default:
		// 未知策略类型，返回错误
		return nil, fmt.Errorf("recipe: unknown context strategy %q", spec.Strategy)
	}
}

// contextAwareAgent 是一个 Agent 包装器，为 Agent 注入 ContextManager
// 它使得每个 Agent 可以有自己独立的上下文管理策略，而不依赖于 Runner 级别的全局 ContextManager
// 这种设计允许不同的 Agent 使用不同的上下文管理方式
type contextAwareAgent struct {
	// 被包装的基础 Agent
	blades.Agent
	// 上下文管理器，负责管理对话上下文的压缩或截断
	cm blades.ContextManager
}

// Run 执行 Agent 调用，在执行前将 ContextManager 注入到执行上下文中
// 这样 Agent 在执行过程中可以通过上下文获取管理后的历史消息
func (a *contextAwareAgent) Run(ctx context.Context, inv *blades.Invocation) iter.Seq2[*blades.Message, error] {
	// 将 ContextManager 注入到上下文中，使 Agent 可以访问
	ctx = blades.NewContextManagerContext(ctx, a.cm)
	// 调用被包装的 Agent 的 Run 方法
	return a.Agent.Run(ctx, inv)
}

// wrapWithContextManager 当 spec 非 nil 时，用 contextAwareAgent 包装 agent
// 如果 spec 为 nil，则原样返回 agent，不做任何包装
// fallbackModelName 当 ContextSpec.Model 为空时用作摘要模型
func wrapWithContextManager(agent blades.Agent, spec *ContextSpec, fallbackModelName string, reg ModelResolver) (blades.Agent, error) {
	// 没有配置上下文管理，直接返回原 Agent
	if spec == nil {
		return agent, nil
	}
	// 构建上下文管理器
	cm, err := buildContextManager(spec, reg, fallbackModelName)
	if err != nil {
		return nil, err
	}
	// 用 contextAwareAgent 包装原 Agent，注入上下文管理能力
	return &contextAwareAgent{Agent: agent, cm: cm}, nil
}
