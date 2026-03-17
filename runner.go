package blades

import (
	"context"

	"github.com/go-kratos/blades/stream"
)

// RunOption 是用于配置 Runner 运行选项的函数类型。
// 采用函数选项模式，允许灵活设置运行参数。
type RunOption func(*RunOptions)

// WithSession 设置自定义会话。
// 会话用于存储状态和历史消息，不设置时会创建新的空会话。
//
// 使用示例：
//
//	session := blades.NewSession()
//	runner.Run(ctx, msg, blades.WithSession(session))
func WithSession(session Session) RunOption {
	return func(r *RunOptions) {
		r.Session = session
	}
}

// WithResume 设置是否从上一个会话状态恢复执行。
// 当 Resume=true 时，Runner 会查找会话历史中之前的执行结果并直接返回。
// 这适用于中断后恢复或避免重复执行的场景。
func WithResume(resume bool) RunOption {
	return func(r *RunOptions) {
		r.Resume = resume
	}
}

// WithInvocationID 设置自定义调用 ID。
// 不设置时会自动生成一个新的 UUID。
// 自定义 ID 适用于需要追踪特定调用的场景。
func WithInvocationID(invocationID string) RunOption {
	return func(r *RunOptions) {
		r.InvocationID = invocationID
	}
}

// RunOptions 保存运行 Agent 时的配置选项。
// 通过 WithSession、WithResume、WithInvocationID 等函数设置。
type RunOptions struct {
	Session      Session // 会话对象
	Resume       bool    // 是否恢复执行
	InvocationID string  // 调用 ID
}

// RunnerOption 是用于配置 Runner 构造函数的选项类型。
type RunnerOption func(*Runner)

// WithContextManager 设置上下文管理器。
// ContextManager 会在 Runner 执行的每个 Agent 的每次模型调用前被调用。
// 它被注入到执行上下文中，因此管道中的所有 Agent（包括循环子 Agent）都能受益，
// 无需每个 Agent 单独配置。
//
// 使用示例：
//
//	manager := &SlidingWindowManager{maxTokens: 10000}
//	runner := NewRunner(agent, WithContextManager(manager))
func WithContextManager(contextManager ContextManager) RunnerOption {
	return func(r *Runner) {
		r.contextManager = contextManager
	}
}

// Runner 负责在会话上下文中执行 Agent。
// Runner 是 Agent 的执行入口，提供 Run（非流式）和 RunStream（流式）两种方法。
//
// 字段说明：
// - rootAgent: 根 Agent，是执行的入口 Agent
// - contextManager: 上下文管理器，可选，用于管理上下文窗口
type Runner struct {
	rootAgent      Agent
	contextManager ContextManager
}

// NewRunner 创建一个新的 Runner。
//
// 参数：
//   - rootAgent: 根 Agent，执行的入口
//   - opts: 可选的 Runner 选项，如 WithContextManager
//
// 返回：
//   - *Runner: 新的 Runner 实例
//
// 使用示例：
//
//	agent, _ := blades.NewAgent("助手", blades.WithModel(model))
//	runner := blades.NewRunner(agent)
//	result, err := runner.Run(ctx, blades.UserMessage("你好"))
func NewRunner(rootAgent Agent, opts ...RunnerOption) *Runner {
	r := &Runner{
		rootAgent: rootAgent,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// buildInvocation 为给定的消息和选项构建 Invocation 对象。
//
// 参数：
//   - ctx: 上下文
//   - message: 用户消息
//   - stream: 是否流式模式
//   - o: 运行选项
//
// 返回：
//   - *Invocation: 构建好的调用对象
//   - error: 构建失败时的错误
func (r *Runner) buildInvocation(ctx context.Context, message *Message, stream bool, o *RunOptions) (*Invocation, error) {
	invocation := &Invocation{
		ID:      o.InvocationID,
		Session: o.Session,
		Resume:  o.Resume,
		Stream:  stream,
		Message: message,
	}
	if message != nil {
		message.Author = "user"
		// 将消息添加到会话历史
		if err := r.appendNewMessage(ctx, invocation, message); err != nil {
			return nil, err
		}
	}
	return invocation, nil
}

// appendNewMessage 将新消息追加到会话历史中。
// 如果 Session 或 message 为 nil，则直接返回（无操作）。
//
// 参数：
//   - ctx: 上下文
//   - invocation: 当前调用
//   - message: 要追加的消息
func (r *Runner) appendNewMessage(ctx context.Context, invocation *Invocation, message *Message) error {
	if invocation.Session == nil || message == nil {
		return nil
	}
	message.InvocationID = invocation.ID
	return invocation.Session.Append(ctx, message)
}

// Run 在会话上下文中执行 Agent，返回最终消息。
// 这是非流式执行方法，会等待 Agent 完成所有迭代后返回结果。
//
// 执行流程：
// 1. 初始化 RunOptions（创建新 Session 和 InvocationID）
// 2. 应用用户提供的选项
// 3. 构建 Invocation 对象
// 4. 创建执行上下文（注入 Session 和 ContextManager）
// 5. 调用 Agent.Run 执行
// 6. 保存完成的消息到会话历史
// 7. 返回最终消息
//
// 参数：
//   - ctx: 上下文
//   - message: 用户消息（可选，可以为 nil）
//   - opts: 可选的运行选项
//
// 返回：
//   - *Message: 最终的助手消息
//   - error: 执行失败时的错误
//
// 使用示例：
//
//	result, err := runner.Run(ctx, blades.UserMessage("请帮我分析这个文件"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result.Text())
func (r *Runner) Run(ctx context.Context, message *Message, opts ...RunOption) (*Message, error) {
	o := &RunOptions{
		Session:      NewSession(),
		InvocationID: NewInvocationID(),
	}
	for _, opt := range opts {
		opt(o)
	}
	var (
		err    error
		output *Message
	)
	invocation, err := r.buildInvocation(ctx, message, false, o)
	if err != nil {
		return nil, err
	}
	// 创建执行上下文，注入 Session
	runCtx := NewSessionContext(ctx, o.Session)
	// 如果配置了 ContextManager，也注入上下文
	if r.contextManager != nil {
		runCtx = NewContextManagerContext(runCtx, r.contextManager)
	}
	iter := r.rootAgent.Run(runCtx, invocation)
	// 消费迭代器，获取消息
	for output, err = range iter {
		if err != nil {
			return nil, err
		}
		// 只保存已完成的消息到会话历史
		if output.Status == StatusCompleted {
			if err := r.appendNewMessage(ctx, invocation, output); err != nil {
				return nil, err
			}
		}
	}
	if output == nil {
		return nil, ErrNoFinalResponse
	}
	return output, nil
}

// RunStream 以流式方式执行 Agent，逐步产生消息。
// 与 Run 不同，RunStream 返回一个 Generator，调用者可以实时接收消息。
// 这适用于需要实时显示进度的场景，如聊天界面。
//
// 参数：
//   - ctx: 上下文
//   - message: 用户消息（可选）
//   - opts: 可选的运行选项
//
// 返回：
//   - Generator[*Message, error]: 消息迭代器
//
// 使用示例：
//
//	stream := runner.RunStream(ctx, blades.UserMessage("你好"))
//	for msg, err := range stream {
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    fmt.Println(msg.Text()) // 实时输出消息
//	}
func (r *Runner) RunStream(ctx context.Context, message *Message, opts ...RunOption) Generator[*Message, error] {
	o := &RunOptions{
		Session:      NewSession(),
		InvocationID: NewInvocationID(),
	}
	for _, opt := range opts {
		opt(o)
	}
	invocation, err := r.buildInvocation(ctx, message, true, o)
	if err != nil {
		return stream.Error[*Message](err)
	}
	runCtx := NewSessionContext(ctx, o.Session)
	if r.contextManager != nil {
		runCtx = NewContextManagerContext(runCtx, r.contextManager)
	}
	return func(yield func(*Message, error) bool) {
		iter := r.rootAgent.Run(runCtx, invocation)
		for output, err := range iter {
			if err != nil {
				yield(nil, err)
				return
			}
			if output.Status == StatusCompleted {
				if err := r.appendNewMessage(ctx, invocation, output); err != nil {
					yield(nil, err)
					return
				}
			}
			if !yield(output, nil) {
				return // 消费者提前终止
			}
		}
	}
}
