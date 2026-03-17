// Package summary 提供了基于摘要的上下文管理器实现。
//
// 核心概念：
// 上下文管理器（ContextManager）负责处理 Agent 对话历史的管理，
// 当对话历史过长时，通过摘要（summarization）压缩旧消息，
// 保持上下文在模型 token 限制内。
//
// 工作原理：
// 1. 监控对话历史的 token 数量
// 2. 当超过阈值时，将旧消息压缩为一条摘要
// 3. 使用 LLM 生成语义连贯的摘要
// 4. 保存摘要和压缩偏移量到会话状态
// 5. 下次调用时从上次位置继续压缩
//
// 与 window.ContextManager 的区别：
// - window: 简单截断旧消息，丢失信息
// - summary: 压缩旧消息为摘要，保留关键信息
//
// 使用场景：
// - 长对话场景，需要保留历史信息
// - 多轮对话中需要记住早期决策和事实
// - token 预算有限但需要长上下文
package summary

import (
	"context"
	"fmt"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/internal/counter"
)

// 会话状态键，用于在多次运行之间持久化压缩状态。
// 这些键用于 session.State() 存储，值是原始类型（string 和 int）。
const (
	// stateSummaryOffsetKey 存储已压缩到摘要中的消息数量（int 类型）。
	//  offset 表示从 session.History() 开头到已处理位置的索引。
	//  例如：offset=10 表示前 10 条消息已被压缩到摘要中。
	stateSummaryOffsetKey = "__summary_offset__"

	// stateSummaryContentKey 存储滚动摘要的文本内容（string 类型）。
	//  这是已压缩消息的语义摘要，由 LLM 生成。
	stateSummaryContentKey = "__summary_content__"
)

// 默认配置常量
const (
	// defaultKeepRecent 是默认保留的最近消息数量。
	// 这些消息始终保留原文，不被压缩。
	defaultKeepRecent = 10

	// defaultBatchSize 是每次压缩迭代处理的消息数量。
	// 分批处理避免单次压缩过多消息导致 token 超限。
	defaultBatchSize = 20

	// defaultInstruction 是用于 LLM 生成摘要的默认指令。
	// 它告知模型如何生成高质量的摘要。
	defaultInstruction = "Please provide a concise summary of the following conversation transcript. " +
		"Preserve key facts, decisions, and outcomes. Output only the summary."
)

// buildWorkingView 构建工作视图，将摘要消息插入到消息列表开头。
//
// 参数说明：
// - summaryContent: 滚动摘要的文本内容
// - offset: 已压缩的消息数量（从消息列表开头的偏移量）
// - messages: 原始消息列表
//
// 返回值：
// []*blades.Message - 工作视图，格式为 [摘要消息] + 未压缩的消息
//
// 处理逻辑：
// 1. 如果 offset <= 0 或摘要为空，返回原始消息（无压缩）
// 2. 创建新切片，容量为 1（摘要）+ 剩余消息数
// 3. 将摘要作为第一条消息（Assistant 角色）
// 4. 追加 offset 之后的所有消息
//
// 使用示例：
//
//	messages := [msg0, msg1, msg2, msg3, msg4]
//	summary := "用户询问了 X，Agent 回答了 Y"
//	workingView := buildWorkingView(summary, 2, messages)
//	// 结果：[summaryMsg, msg2, msg3, msg4]
func buildWorkingView(summaryContent string, offset int, messages []*blades.Message) []*blades.Message {
	if offset <= 0 || summaryContent == "" {
		return messages
	}
	// 创建新切片，预留容量避免重复分配
	result := make([]*blades.Message, 0, 1+len(messages)-offset)
	// 将摘要作为第一条消息（使用 Assistant 角色）
	result = append(result, blades.AssistantMessage(summaryContent))
	// 追加未压缩的消息
	result = append(result, messages[offset:]...)
	return result
}

// Option 是 summary.ContextManager 的配置函数类型。
// 使用函数选项模式（Functional Options Pattern）实现灵活配置。
type Option func(*contextManager)

// WithMaxTokens 设置触发压缩的 token 预算阈值。
//
// 参数说明：
// - tokens: 最大 token 数量
//   - 当消息列表的 token 总数超过此值时，触发压缩
//   - 值为 0 时禁用压缩（无操作）
//
// 使用示例：
//
//	cm := summary.NewContextManager(
//	    summary.WithMaxTokens(100000), // 超过 10 万 token 时压缩
//	)
func WithMaxTokens(tokens int64) Option {
	return func(c *contextManager) {
		c.maxTokens = tokens
	}
}

// WithSummarizer 设置用于生成摘要的 ModelProvider。
//
// 参数说明：
// - model: 模型提供者，用于调用 LLM 生成摘要
//
// 为什么需要单独的 summarizer？
// - 生成摘要需要 LLM 能力，不是简单的文本拼接
// - 可以使用与主 Agent 不同的模型（如更便宜/更快的模型）
//
// 使用示例：
//
//	cm := summary.NewContextManager(
//	    summary.WithSummarizer(model),
//	)
func WithSummarizer(model blades.ModelProvider) Option {
	return func(c *contextManager) {
		c.summarizer = model
	}
}

// WithTokenCounter 设置用于估算 token 使用量的 TokenCounter。
// 默认使用基于字符的计数器（1 token ≈ 4 字符）。
//
// 参数说明：
// - counter: Token 计数器实现
//
// 使用示例：
//
//	cm := summary.NewContextManager(
//	    summary.WithTokenCounter(customCounter),
//	)
func WithTokenCounter(counter blades.TokenCounter) Option {
	return func(c *contextManager) {
		c.counter = counter
	}
}

// WithKeepRecent 设置始终保留原文的最近消息数量。
// 默认值为 10。
//
// 参数说明：
// - n: 保留的最近消息数
//
// 为什么保留最近消息？
// - 最近消息通常包含当前话题的关键信息
// - 避免压缩正在进行的对话内容
// - 提高摘要质量，保留上下文连贯性
//
// 使用示例：
//
//	cm := summary.NewContextManager(
//	    summary.WithKeepRecent(20), // 保留最近 20 条消息
//	)
func WithKeepRecent(n int) Option {
	return func(c *contextManager) {
		c.keepRecent = n
	}
}

// WithBatchSize 设置每次压缩迭代处理的消息数量。
// 默认值为 20。
//
// 参数说明：
// - n: 每次压缩的消息数
//
// 为什么分批压缩？
// - 避免单次处理过多消息导致 token 超限
// -  incremental 压缩更易控制质量
// - 可以在达到 token 预算后停止
//
// 使用示例：
//
//	cm := summary.NewContextManager(
//	    summary.WithBatchSize(50), // 每次压缩 50 条消息
//	)
func WithBatchSize(n int) Option {
	return func(c *contextManager) {
		c.batchSize = n
	}
}

// contextManager 通过滚动摘要压缩旧消息实现 ContextManager 接口。
// 当 token 数量超过配置限制时，使用 LLM 将旧消息压缩为语义摘要。
// 压缩状态（滚动摘要内容和已压缩偏移量）在会话中持久化，
// 避免在多次运行之间重复处理已压缩的消息。
//
// 字段说明：
// - maxTokens: 触发压缩的 token 阈值
// - counter: Token 计数器，用于估算消息的 token 使用量
// - summarizer: LLM 模型提供者，用于生成摘要
// - keepRecent: 始终保留原文的最近消息数量
// - batchSize: 每次压缩迭代处理的消息数量
// - instruction: 生成摘要时使用的指令
//
// 压缩算法流程：
// 1. 从会话状态读取已保存的 offset 和摘要内容
// 2. 构建工作视图：[摘要消息] + messages[offset:]
// 3. 检查 token 总数是否超过 maxTokens
// 4. 如果超过，取下一批 batchSize 条消息进行压缩
// 5. 调用 LLM 扩展摘要
// 6. 更新 offset 和摘要内容
// 7. 重复步骤 3-6 直到低于预算或无消息可压缩
// 8. 将新状态保存到会话
type contextManager struct {
	maxTokens   int64
	counter     blades.TokenCounter
	summarizer  blades.ModelProvider
	keepRecent  int
	batchSize   int
	instruction string
}

// NewContextManager 创建并返回一个新的摘要上下文管理器。
//
// 参数说明：
// - opts: 配置选项，可设置 maxTokens、summarizer 等
//
// 默认配置：
// - instruction: 默认摘要指令
// - keepRecent: 10 条消息
// - batchSize: 20 条消息
// - counter: 基于字符的计数器
//
// 压缩状态持久化：
// 当上下文存在于 Session 中时，压缩状态（滚动摘要文本和
// 已压缩消息偏移量）通过 session.State() 在多次运行之间持久化。
// 这避免了每次调用时重新处理已压缩的消息。
//
// 返回值：
// blades.ContextManager - 摘要上下文管理器实例
//
// 使用示例：
//
//	cm := summary.NewContextManager(
//	    summary.WithMaxTokens(100000),
//	    summary.WithSummarizer(model),
//	    summary.WithKeepRecent(15),
//	)
//	agent := blades.NewAgent("my-agent", blades.WithContextManager(cm))
func NewContextManager(opts ...Option) blades.ContextManager {
	cm := &contextManager{
		instruction: defaultInstruction,
		keepRecent:  defaultKeepRecent,
		batchSize:   defaultBatchSize,
		counter:     counter.NewCharBasedCounter(),
	}
	for _, opt := range opts {
		opt(cm)
	}
	return cm
}

// ensureSession 从上下文中获取 Session。
// 如果上下文中不存在 Session，则创建一个临时内存 Session。
//
// 参数说明：
// - ctx: 上下文
//
// 返回值：
// blades.Session - 会话对象
//
// 设计说明：
// 此方法确保 Prepare 方法始终有一个 Session 可用，
// 无需在每次调用时检查 Session 是否存在。
// - 如果上下文中存在 Session，其状态会在多次调用之间持久化
// - 如果不存在，使用临时 Session，状态在当前调用结束后丢弃
//
// 这实现了统一的行为：
// - 有 Session 时：增量压缩，状态持久化
// - 无 Session 时：每次从头压缩，状态不保存
func (s *contextManager) ensureSession(ctx context.Context) blades.Session {
	if session, ok := blades.FromSessionContext(ctx); ok {
		return session
	}
	// 创建临时 Session，状态不持久化
	return blades.NewSession()
}

// Prepare 当消息的 token 总数超过 MaxTokens 时压缩旧消息。
// 当 ctx 中存在 Session 时，它读取并写入两个原始类型状态键
// 以在多次运行之间持久化增量压缩状态。
//
// 参数说明：
// - ctx: 上下文，可能包含 Session
// - messages: 完整的消息历史列表
//
// 返回值：
// []*blades.Message - 压缩后的消息列表
// error - 压缩过程中的错误
//
// 压缩流程详解：
// 1. 获取 Session（持久化或临时）
// 2. 读取已保存的压缩状态（offset 和摘要内容）
// 3. 安全检查：offset 不能超过当前消息长度
// 4. 构建工作视图
// 5. 循环压缩直到 token 数低于阈值：
//    a. 计算可压缩边界（总长度 - keepRecent）
//    b. 如果 offset 已达边界，退出循环
//    c. 取下一批消息（batchSize 条）
//    d. 调用 LLM 扩展摘要
//    e. 更新 offset 和摘要
//    f. 重建工作视图
// 6. 保存新状态到 Session
// 7. 返回压缩后的消息列表
//
// 使用示例：
//
//	cm := summary.NewContextManager(summary.WithMaxTokens(100000))
//	compressed, err := cm.Prepare(ctx, messages)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// compressed 包含摘要和最近消息
func (s *contextManager) Prepare(ctx context.Context, messages []*blades.Message) ([]*blades.Message, error) {
	// 边界情况：空消息列表或禁用压缩
	if len(messages) == 0 || s.maxTokens == 0 {
		return messages, nil
	}

	// 获取 Session（持久化或临时）
	session := s.ensureSession(ctx)

	// 从 Session 读取已保存的压缩状态（仅原始类型：int 和 string）
	offset := 0
	summaryContent := ""
	if v, ok := session.State()[stateSummaryOffsetKey]; ok {
		if n, ok := v.(int); ok {
			offset = n
		}
	}
	if v, ok := session.State()[stateSummaryContentKey]; ok {
		if c, ok := v.(string); ok {
			summaryContent = c
		}
	}
	// 安全检查：如果外部重置了会话历史，offset 可能越界
	if offset > len(messages) {
		offset = 0
		summaryContent = ""
	}

	// 使用已保存的摘要和 offset 构建初始工作视图
	workingView := buildWorkingView(summaryContent, offset, messages)

	// 当工作视图的 token 数超过预算时，继续压缩
	for s.counter.Count(workingView...) > s.maxTokens {
		// 计算可压缩边界：保留最近 keepRecent 条消息
		boundary := len(messages) - s.keepRecent
		if offset >= boundary {
			break // 所有可压缩消息已折叠到摘要中
		}

		// 计算下一批次的结束位置
		end := min(offset+s.batchSize, boundary)
		batch := messages[offset:end]

		// 调用 LLM 扩展摘要
		newSummary, err := s.extendSummary(ctx, summaryContent, batch)
		if err != nil {
			return nil, fmt.Errorf("context manager: summarization failed: %w", err)
		}

		// 更新状态
		offset = end
		summaryContent = newSummary
		workingView = buildWorkingView(summaryContent, offset, messages)
	}

	// 保存更新后的状态（原始类型：int 和 string）
	session.SetState(stateSummaryOffsetKey, offset)
	session.SetState(stateSummaryContentKey, summaryContent)

	return workingView, nil
}

// extendSummary 调用 LLM 生成一个新的摘要，覆盖现有摘要和提供的消息批次。
//
// 参数说明：
// - ctx: 上下文
// - existing: 现有的摘要文本（可以为空）
// - batch: 要压缩到摘要中的消息批次
//
// 处理流程：
// 1. 构建摘要指令：
//    - 基础指令
//    - 如有现有摘要，追加到指令中
// 2. 创建模型请求：
//    - Messages: 要压缩的消息批次
//    - Instruction: 系统指令，告知如何生成摘要
// 3. 调用 LLM 生成新摘要
// 4. 返回摘要文本
//
// 返回值：
// - string: 新生成的摘要文本
// - error: LLM 调用过程中的错误
//
// 摘要指令示例：
//
//	Please provide a concise summary of the following conversation transcript.
//	Preserve key facts, decisions, and outcomes. Output only the summary.
//
//	Existing summary:
//	用户询问了项目结构，Agent 解释了主要目录...
//
// 这样 LLM 会将新消息与现有摘要合并，生成连贯的滚动摘要。
func (s *contextManager) extendSummary(ctx context.Context, existing string, batch []*blades.Message) (string, error) {
	instruction := s.instruction
	// 如果有现有摘要，追加到指令中
	if existing != "" {
		instruction += "\n\nExisting summary:\n" + existing
	}
	// 构建模型请求
	req := &blades.ModelRequest{
		Messages:    batch,
		Instruction: blades.SystemMessage(instruction),
	}
	// 调用 LLM 生成摘要
	resp, err := s.summarizer.Generate(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Message.Text(), nil
}
