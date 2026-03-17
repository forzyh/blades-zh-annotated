package blades

import "context"

// TokenCounter 是令牌计数器接口，用于估算消息的 token 数量。
// Token 是 AI 模型处理文本的基本单位，了解 token 数量有助于：
// 1. 控制成本（模型按 token 计费）
// 2. 避免超出上下文窗口限制
// 3. 实现智能截断策略
//
// 实现可以使用模型特定的 tokenizer 进行精确计数，
// 或使用启发式近似方法（如按字符数估算）。
type TokenCounter interface {
	// Count 返回给定消息的 token 数量估算值。
	// 参数是可变的，可以一次性计算多条消息的总 token 数。
	Count(messages ...*Message) int64
}

// ContextManager 管理每次模型调用前的消息上下文窗口。
// 由于 AI 模型的上下文窗口有限（如 128K tokens），当对话历史过长时，
// 需要策略性地管理消息，以适应窗口限制。
//
// 常见的上下文管理策略：
// 1. 滑动窗口：保留最近 N 条消息，丢弃旧消息
// 2. LLM 摘要：使用模型压缩旧消息为简短摘要
// 3. 重要性排序：保留关键消息，丢弃次要消息
//
// ContextManager 在 Agent 循环的每次模型调用前被调用，
// 允许灵活实现上述策略。
type ContextManager interface {
	// Prepare 过滤、截断或压缩消息以适应上下文窗口。
	// 系统/指令内容通过 ModelRequest.Instruction 单独处理，
	// 永远不会传递给 Prepare，因此实现时无需考虑指令消息。
	//
	// 参数：
	//   - ctx: 上下文
	//   - messages: 原始消息列表
	//
	// 返回：
	//   - []*Message: 处理后的消息列表
	//   - error: 处理失败时的错误
	Prepare(ctx context.Context, messages []*Message) ([]*Message, error)
}

// ctxManagerKey 是用于在上下文中存储 ContextManager 的私有键类型。
type ctxManagerKey struct{}

// NewContextManagerContext 返回一个携带 ContextManager 的子上下文。
// 由 Runner 调用，将配置的 ContextManager 注入执行上下文。
// 这样，所有 Agent（包括循环中的子 Agent）都可以使用同一个 ContextManager。
//
// 参数：
//   - ctx: 父上下文
//   - m: ContextManager 实例
//
// 返回：
//   - context.Context: 携带 ContextManager 的子上下文
func NewContextManagerContext(ctx context.Context, m ContextManager) context.Context {
	return context.WithValue(ctx, ctxManagerKey{}, m)
}

// ContextManagerFromContext 从上下文中检索 Runner 存储的 ContextManager。
// 如果上下文中不存在 ContextManager，返回 (nil, false)。
//
// 返回：
//   - ContextManager: 上下文管理器（如果存在）
//   - bool: 是否成功获取
func ContextManagerFromContext(ctx context.Context) (ContextManager, bool) {
	m, ok := ctx.Value(ctxManagerKey{}).(ContextManager)
	return m, ok
}
