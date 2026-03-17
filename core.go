// Package blades 提供了用于构建 AI 代理的核心抽象和组件。
package blades

import (
	"context"
	"iter"
	"slices"

	"github.com/go-kratos/blades/tools"
	"github.com/google/uuid"
)

// Invocation 保存当前调用的相关信息。
// Invocation 是 Agent.Run 方法的输入参数，包含了执行所需的所有元数据：
// - ID: 调用的唯一标识符
// - Model: 使用的模型名称
// - Resume: 是否从之前的状态恢复
// - Stream: 是否使用流式模式
// - Session: 会话对象，存储状态和历史消息
// - Instruction: 系统指令（可选）
// - Message: 用户消息（可选）
// - Tools: 可用工具列表
type Invocation struct {
	ID          string
	Model       string
	Resume      bool
	Stream      bool
	Session     Session
	Instruction *Message
	Message     *Message
	Tools       []tools.Tool
}

// Generator 是一个泛型类型，表示一个序列生成器。
// 它依次产生类型为 T 的值或类型为 E 的错误。
// 在 Blades 中，Generator 用于流式返回消息或错误，是 Go 1.23+ iter.Seq2 的别名。
//
// 使用示例：
//
//	// 创建一个 Generator，依次产生消息
//	gen := func(yield func(*Message, error) bool) {
//	    for _, msg := range messages {
//	        if !yield(msg, nil) {
//	            return // 消费者提前终止
//	        }
//	    }
//	}
//	// 消费 Generator
//	for msg, err := range gen {
//	    if err != nil {
//	        // 处理错误
//	    }
//	    // 处理消息
//	}
type Generator[T, E any] = iter.Seq2[T, E]

// Agent 表示一个自主代理，能够处理调用并生成消息序列。
// Agent 是 Blades 框架的核心接口，所有可执行组件都实现此接口。
// 通过统一的 Run 方法，实现解耦、标准化和高组合性。
//
// 如何实现一个 Agent：
// 1. 实现 Name() 返回代理名称
// 2. 实现 Description() 返回功能描述
// 3. 实现 Run(ctx, invocation) 返回消息生成器
//
// 使用示例：
//
//	type MyAgent struct { ... }
//	func (a *MyAgent) Name() string { return "my-agent" }
//	func (a *MyAgent) Description() string { return "这是一个自定义代理" }
//	func (a *MyAgent) Run(ctx context.Context, inv *Invocation) Generator[*Message, error] {
//	    return func(yield func(*Message, error) bool) {
//	        // 处理逻辑，调用 yield 返回消息
//	    }
//	}
type Agent interface {
	// Name 返回代理的名称。
	// 名称用于标识代理，在日志、恢复和工具嵌套时使用。
	Name() string
	// Description 返回代理功能的简要描述。
	// 描述用于说明代理的用途，在将代理作为工具嵌套时传递给模型。
	Description() string
	// Run 处理给定的调用，返回一个生成器，依次产生消息或错误。
	// Run 是 Agent 的核心执行方法，支持流式和非流式两种模式。
	//
	// 参数：
	//   - ctx: 上下文，用于取消和传递值
	//   - invocation: 调用对象，包含消息、会话、工具等
	//
	// 返回：
	//   - Generator[*Message, error]: 消息迭代器
	Run(context.Context, *Invocation) Generator[*Message, error]
}

// NewInvocationID 生成一个新的唯一调用标识符。
// 使用 uuid.NewString() 生成 UUID v4，确保全局唯一性。
// 每次创建新的 Invocation 时都应调用此函数生成独立 ID。
func NewInvocationID() string {
	return uuid.NewString()
}

// Clone 创建 Invocation 的深拷贝。
// 深拷贝确保修改副本不会影响原始对象。
// 注意：Session 是引用类型，不会被复制，只是引用传递。
//
// 复制的字段：
// - ID, Model, Resume, Stream: 值类型，直接复制
// - Message, Instruction: 调用各自的 Clone() 方法
// - Tools: 使用 slices.Clone() 创建新切片
//
// 返回：
//   - *Invocation: 新的 Invocation 实例
func (inv *Invocation) Clone() *Invocation {
	return &Invocation{
		ID:          inv.ID,
		Model:       inv.Model,
		Session:     inv.Session,
		Resume:      inv.Resume,
		Stream:      inv.Stream,
		Message:     inv.Message.Clone(),
		Instruction: inv.Instruction.Clone(),
		Tools:       slices.Clone(inv.Tools),
	}
}
