// Package window 提供了基于窗口的上下文管理器实现。
//
// 核心概念：
// 上下文管理器（ContextManager）负责处理 Agent 对话历史的管理，
// 当对话历史过长时，通过截断（truncation）丢弃旧消息，
// 保持上下文在模型 token 限制内。
//
// 工作原理：
// 1. 监控对话历史的 token 数量或消息数量
// 2. 当超过阈值时，从开头丢弃旧消息
// 3. 保留最近的消息，确保上下文新鲜度
//
// 与 summary.ContextManager 的区别：
// - window: 简单截断旧消息，快速但丢失信息
// - summary: 压缩旧消息为摘要，保留关键信息但需要 LLM 调用
//
// 使用场景：
// - 简单对话场景，不需要保留早期历史
// - token 预算充足的场景
// - 对性能敏感，不想承担摘要计算开销
// - 测试和开发环境
package window

import (
	"context"

	"github.com/go-kratos/blades"
)

// defaultMaxMessages 是默认的最大消息数量限制。
// 当消息数量超过此值时，会丢弃最旧的消息。
const defaultMaxMessages = 100

// Option 是 window.ContextManager 的配置函数类型。
// 使用函数选项模式（Functional Options Pattern）实现灵活配置。
type Option func(*contextManager)

// WithMaxMessages 设置要保留的最大消息数量。
//
// 参数说明：
// - n: 最大消息数量
//   - 当消息数量超过此值时，丢弃最旧的消息
//   - 值为 0 时禁用消息数量限制
//
// 丢弃策略：
// 当超过限制时，从最旧的消息开始丢弃（FIFO 先进先出）。
// 例如：maxMessages=100，当前有 120 条消息，则丢弃前 20 条，保留最近 100 条。
//
// 使用示例：
//
//	cm := window.NewContextManager(
//	    window.WithMaxMessages(50), // 最多保留 50 条消息
//	)
func WithMaxMessages(n int) Option {
	return func(c *contextManager) {
		c.maxMessages = n
	}
}

// WithMaxTokens 设置最大的 token 预算。
//
// 参数说明：
// - tokens: 最大 token 数量
//   - 当消息的 token 总数超过此值时，丢弃旧消息
//   - 值为 0 时禁用 token 限制（默认行为）
//
// 丢弃策略：
// 从消息列表开头（最旧消息）开始丢弃，直到 token 总数低于预算。
// 使用配置的 TokenCounter 估算每条消息的 token 数。
//
// 使用示例：
//
//	cm := window.NewContextManager(
//	    window.WithMaxTokens(100000), // 超过 10 万 token 时截断
//	)
func WithMaxTokens(tokens int64) Option {
	return func(c *contextManager) {
		c.maxTokens = tokens
	}
}

// WithTokenCounter 设置用于估算 token 使用量的 TokenCounter。
// 默认使用基于字符的计数器（1 token ≈ 4 字符）。
//
// 参数说明：
// - counter: Token 计数器实现
//
// 为什么需要 TokenCounter？
// - 不同模型的 token 计算方式不同
// - 精确的 token 计算需要模型的 tokenizer
// - 默认字符计数器是快速估算，适合大多数场景
//
// 使用示例：
//
//	cm := window.NewContextManager(
//	    window.WithTokenCounter(customCounter),
//	)
func WithTokenCounter(counter blades.TokenCounter) Option {
	return func(c *contextManager) {
		c.counter = counter
	}
}

// contextManager 是 window 包中 ContextManager 接口的具体实现。
// 它通过消息数量限制和 token 预算限制来管理上下文窗口大小。
//
// 字段说明：
// - maxMessages: 最大消息数量限制
//   - 默认值：100
//   - 值为 0 时禁用此限制
//
// - maxTokens: 最大 token 预算
//   - 默认值：0（禁用）
//   - 当超过预算时，从开头丢弃消息
//
// - counter: Token 计数器，用于估算消息的 token 使用量
//   - 默认：基于字符的计数器
//
// 截断策略：
// 1. 首先应用消息数量限制（如果启用）
// 2. 然后应用 token 预算限制（如果启用）
// 3. 总是从最旧的消息开始丢弃
//
// 为什么先按消息数量再按 token 截断？
// - 消息数量限制是快速路径，避免处理大量消息
// - token 限制是精确控制，确保不超过模型限制
// - 两层保护提供更好的控制粒度
type contextManager struct {
	maxMessages int
	maxTokens   int64
	counter     blades.TokenCounter
}

// NewContextManager 创建并返回一个新的窗口上下文管理器。
//
// 参数说明：
// - opts: 配置选项，可设置 maxMessages、maxTokens、counter 等
//
// 默认配置：
// - maxMessages: 100 条消息
// - maxTokens: 0（禁用）
// - counter: 基于字符的计数器
//
// 返回值：
// blades.ContextManager - 窗口上下文管理器实例
//
// 使用示例：
//
//	// 基本用法：只限制消息数量
//	cm := window.NewContextManager()
//
//	// 高级用法：同时限制消息数量和 token 预算
//	cm := window.NewContextManager(
//	    window.WithMaxMessages(50),
//	    window.WithMaxTokens(50000),
//	)
//
//	// 将上下文管理器应用到 Agent
//	agent := blades.NewAgent("my-agent", blades.WithContextManager(cm))
func NewContextManager(opts ...Option) blades.ContextManager {
	// 初始化默认配置
	cm := &contextManager{maxMessages: defaultMaxMessages}
	// 应用用户提供的选项
	for _, opt := range opts {
		opt(cm)
	}
	return cm
}

// Prepare 保留符合配置限制的最新消息。
//
// 参数说明：
// - ctx: 上下文（此实现中未使用，为了符合接口定义而保留）
// - messages: 完整的消息历史列表
//
// 返回值：
// []*blades.Message - 截断后的消息列表
// error - 处理过程中的错误（此实现始终返回 nil）
//
// 处理流程：
// 1. 边界检查：空消息列表直接返回
// 2. 消息数量限制（如果启用）：
//    - 计算超出数量
//    - 切片保留最近的消息
// 3. Token 预算限制（如果启用）：
//    - 计算总 token 数
//    - 循环丢弃最旧消息直到低于预算
// 4. 返回处理后的消息列表
//
// 截断示例：
//
//	// 假设有 150 条消息，maxMessages=100
//	messages := [msg0, msg1, ..., msg149]
//	result := Prepare(ctx, messages)
//	// 结果：[msg50, msg51, ..., msg149] (保留最近 100 条)
//
//	// 假设 token 总数为 150000，maxTokens=100000
//	// 每条约 1000 token
//	result := Prepare(ctx, messages)
//	// 结果：丢弃约 50 条最旧消息，保留约 100 条
func (w *contextManager) Prepare(_ context.Context, messages []*blades.Message) ([]*blades.Message, error) {
	// 边界情况：空消息列表直接返回
	if len(messages) == 0 {
		return messages, nil
	}

	result := messages

	// 步骤 1：应用消息数量限制
	if w.maxMessages > 0 && len(result) > w.maxMessages {
		// 计算需要保留的消息起始索引
		// 例如：150 条消息，maxMessages=100，则从索引 50 开始
		result = result[len(result)-w.maxMessages:]
	}

	// 步骤 2：应用 token 预算限制
	if w.maxTokens > 0 {
		// 计算当前总 token 数
		total := w.counter.Count(result...)
		// 循环丢弃最旧消息直到低于预算
		for len(result) > 1 && total > w.maxTokens {
			// 减去被丢弃消息的 token 数
			total -= w.counter.Count(result[0])
			// 丢弃第一条消息（最旧）
			result = result[1:]
		}
	}

	return result, nil
}
