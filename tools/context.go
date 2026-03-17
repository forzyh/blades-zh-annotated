package tools

import "context"

// ============================================================================
// ToolContext：工具上下文
// ============================================================================

// ToolContext 提供了关于工具调用的元数据，并允许工具通过 SetAction 将控制流信号传递回调用者。
//
// 【是什么】
// ToolContext 是一个接口，它在工具执行期间携带上下文信息。
// 它有两个主要作用：
// 1. 提供工具调用的元数据（ID、名称）
// 2. 提供一个机制（SetAction）让工具可以传递控制流信号给外层调用者
//
// 【为什么需要 ToolContext】
// 1. 身份识别：通过 ID 和 Name 识别是哪个工具被调用
// 2. 控制流通信：工具可以通过 SetAction 告诉外层"发生了什么"
// 3. 状态传递：Actions 可以在线程安全的方式下累积状态
//
// 【使用场景】
//   - ExitTool 通过 SetAction(ActionLoopExit, true) 通知外层循环退出
//   - 工具可以通过 SetAction 记录执行状态，供调用者检查
//   - 调试和日志记录时，可以通过 ToolContext 获取工具信息
//
// 【与 context.Context 的关系】
// ToolContext 不是 context.Context 的替代品，而是补充。
// ToolContext 存储在 context.Context 中，通过 NewContext 和 FromContext 访问。
//
// 【并发安全】
// ToolContext 的所有方法都是并发安全的，可以在多个 goroutine 中使用。
type ToolContext interface {
	// ID 返回工具调用的唯一标识符。
	// 每次工具调用都会生成一个新的 ID，用于追踪和日志记录。
	ID() string

	// Name 返回被调用工具的名称。
	// 这可以帮助调试和日志记录时识别是哪个工具被调用。
	Name() string

	// Actions 返回工具累积的动作映射的副本。
	//
	// 【是什么】
	// Actions 是一个 map[string]any，用于存储工具执行过程中设置的各种动作。
	// 工具可以通过 SetAction 向这个映射添加键值对。
	//
	// 【返回值】
	// 返回一个副本（copy），而不是原始映射的引用。
	// 这样做是为了防止调用者意外修改内部状态。
	//
	// 【使用场景】
	// 外层循环执行器检查 Actions 来判断是否有退出信号：
	//
	//	actions := toolCtx.Actions()
	//	if _, ok := actions[tools.ActionLoopExit]; ok {
	//	    break // 退出循环
	//	}
	Actions() map[string]any

	// SetAction 记录一个控制流动作，调用者（如 LoopAgent、RoutingAgent）
	// 可以在工具执行后通过检查消息的 Actions 来感知。
	//
	// 【参数】
	//   - key: 动作的键名，如 "loop_exit"
	//   - value: 动作的值，可以是任意类型（any）
	//
	// 【使用示例】
	//
	//	// ExitTool 使用示例
	//	tc.SetAction(ActionLoopExit, true)  // 设置退出信号，escalate=true
	//	tc.SetAction("custom_action", data) // 设置自定义动作
	//
	// 【并发安全】
	// SetAction 是并发安全的，多个 goroutine 可以同时调用。
	// 内部实现应该使用互斥锁或其他同步机制保护共享状态。
	//
	// 【注意事项】
	//   - 设置的动作会累积，不会被覆盖（除非使用相同的 key）
	//   - 调用者通过 Actions() 获取的是副本，修改副本不会影响内部状态
	//   - 动作在工具执行结束后传递给调用者
	SetAction(key string, value any)
}

// ============================================================================
// ToolContext 上下文传递函数
// ============================================================================

// ctxToolKey 是用于在 context.Context 中存储 ToolContext 的键。
//
// 【为什么使用自定义类型】
// 使用自定义类型（而不是 string）作为 context 的键可以避免键名冲突。
// 如果使用 string，其他包可能碰巧使用相同的字符串作为键，导致意外覆盖。
// 使用未导出的 struct 类型确保只有本包可以访问这个键。
//
// 【工作原理】
// context.Context 是一个键值存储，键的类型是 any（接口）。
// 当使用 ctx.Value(key) 时，会使用 equals 比较键。
// 由于 ctxToolKey 是未导出的 struct，其他包无法创建相同的实例，
// 因此不会发生键冲突。
type ctxToolKey struct{}

// NewContext 返回一个子上下文，其中携带了给定的 ToolContext。
//
// 【是什么】
// NewContext 是一个工具函数，用于将 ToolContext 存储到 context.Context 中。
// 这样在工具调用的下游代码中，可以通过 FromContext 获取 ToolContext。
//
// 【参数】
//   - ctx: 父上下文，通常是工具调用时传入的 context.Context
//   - tool: 要存储的 ToolContext 实例
//
// 【返回值】
// 返回一个新的 context.Context，它是 ctx 的子上下文，携带了 ToolContext。
//
// 【使用示例】
//
//	// 创建 ToolContext
//	tc := NewToolContext("tool-123", "my_tool")
//
//	// 将 ToolContext 存入上下文
//	ctx := tools.NewContext(parentCtx, tc)
//
//	// 在下游代码中获取
//	if tc, ok := tools.FromContext(ctx); ok {
//	    tc.SetAction("done", true)
//	}
//
// 【典型使用场景】
// 在 Agent 运行时中，当调用工具的 Handle 方法时：
//
// 1. 创建 ToolContext（包含工具 ID 和名称）
// 2. 使用 NewContext 将其存入上下文
// 3. 调用 tool.Handle(ctx, input)，其中 ctx 已包含 ToolContext
// 4. 工具内部可以通过 FromContext 获取 ToolContext 并设置动作
//
// 【context 链】
// 返回的上下文是 ctx 的子节点，继承了 ctx 的所有值。
// 如果 ctx 被取消或超时，返回的上下文也会被取消或超时。
func NewContext(ctx context.Context, tool ToolContext) context.Context {
	return context.WithValue(ctx, ctxToolKey{}, tool)
}

// FromContext 检索由 NewContext 存储的 ToolContext。
//
// 【是什么】
// FromContext 是 NewContext 的配对函数，用于从 context.Context 中提取 ToolContext。
//
// 【参数】
//   - ctx: 上下文，可能由 NewContext 创建
//
// 【返回值】
//   - ToolContext: 如果 ctx 中存在 ToolContext，返回它；否则返回 nil
//   - bool: 如果成功获取 ToolContext，返回 true；否则返回 false
//
// 【使用示例】
//
//	// 典型用法
//	if tc, ok := tools.FromContext(ctx); ok {
//	    // ctx 中包含 ToolContext，可以使用
//	    tc.SetAction("done", true)
//	} else {
//	    // ctx 中不包含 ToolContext，可能是直接调用
//	    // 根据业务逻辑决定如何处理
//	}
//
// 【为什么返回 (ToolContext, bool)】
// 1. 区分"不存在"和"nil 值"：如果只返回 ToolContext，无法区分这两种情况
// 2. 避免 panic：如果 ctx 中没有 ToolContext，直接返回 nil 而不是 panic
// 3. Go 习惯用法：这是 Go 中常见的"comma ok"模式
//
// 【典型使用场景】
// 在工具内部，需要设置动作时：
//
//	func (t *MyTool) Handle(ctx context.Context, input string) (string, error) {
//	    // 尝试获取 ToolContext
//	    if tc, ok := FromContext(ctx); ok {
//	        // 设置动作（如果不在循环中，这会是 no-op）
//	        tc.SetAction(ActionLoopExit, false)
//	    }
//	    // 继续执行工具逻辑...
//	}
func FromContext(ctx context.Context) (ToolContext, bool) {
	tool, ok := ctx.Value(ctxToolKey{}).(ToolContext)
	return tool, ok
}
