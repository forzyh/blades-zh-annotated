package tools

// ============================================================================
// Middleware：工具中间件
// ============================================================================

// Middleware 是一个函数类型，它接收一个 Handler，返回一个新的 Handler。
//
// 【是什么】
// Middleware（中间件）是一种"包装器"模式，允许你在工具执行前后添加额外逻辑。
// 这种设计类似于 Web 框架（如 Gin、Echo）中的中间件机制。
//
// 【为什么需要中间件】
// 1. 横切关注点：日志、监控、权限验证等逻辑可以统一处理
// 2. 代码复用：多个工具可以共享相同的中间件
// 3. 组合灵活：可以按需组合多个中间件
// 4. 不侵入业务：中间件逻辑与业务逻辑分离
//
// 【工作原理】
// 中间件接收一个 Handler，返回一个新的 Handler：
//   - 新 Handler 可以在调用原 Handler 之前做某事（如记录日志）
//   - 新 Handler 可以在调用原 Handler 之后做某事（如统计耗时）
//   - 新 Handler 可以修改输入或输出
//   - 新 Handler 可以决定是否调用原 Handler（如权限验证失败时阻断）
//
// 【使用场景】
//   - 日志记录：记录每次工具调用的参数和结果
//   - 性能监控：统计工具执行耗时
//   - 权限验证：检查调用者是否有权使用此工具
//   - 输入验证：在业务逻辑前验证输入合法性
//   - 错误处理：统一处理错误，转换为友好格式
//   - 重试逻辑：失败时自动重试
//   - 超时控制：为工具执行设置超时
//
// 【使用示例】
//
//	// 日志中间件
//	func LoggingMiddleware() Middleware {
//	    return func(next Handler) Handler {
//	        return HandleFunc(func(ctx context.Context, input string) (string, error) {
//	            log.Printf("调用工具，输入：%s", input)
//	            result, err := next.Handle(ctx, input)
//	            log.Printf("工具返回：result=%s, err=%v", result, err)
//	            return result, err
//	        })
//	    }
//	}
//
//	// 使用中间件创建工具
//	tool := NewTool("mytool", "描述", handler,
//	    WithMiddleware(LoggingMiddleware()))
//
// 【执行顺序】
// 当有多个中间件时，它们按"洋葱模型"执行：
//   - Middleware 1 开始执行
//   - Middleware 2 开始执行
//   - Middleware 3 开始执行
//   - 原始 Handler 执行
//   - Middleware 3 继续执行（后处理）
//   - Middleware 2 继续执行（后处理）
//   - Middleware 1 继续执行（后处理）
type Middleware func(Handler) Handler

// ChainMiddlewares 将多个中间件组合成一个。
//
// 【是什么】
// ChainMiddlewares 是一个工具函数，它将多个 Middleware 组合成一个，
// 使得可以一次性应用多个中间件。
//
// 【参数】
//   - mws: 可变数量的 Middleware
//
// 【返回值】
// 返回一个新的 Middleware，它是所有输入中间件的组合。
//
// 【执行顺序】
// 第一个传入的中间件会成为"最外层"，最后一个成为"最内层"。
// 执行顺序是：mws[0] -> mws[1] -> ... -> Handler -> ... -> mws[1] -> mws[0]
//
// 【实现原理】
// 代码从后向前遍历中间件列表：
//
//	h := next
//	for i := len(mws) - 1; i >= 0; i-- {
//	    h = mws[i](h)  // 用 mws[i] 包装 h
//	}
//
// 这样做的结果是：
//   - 最后一次迭代（i=0）：h = mws[0](h)，mws[0] 成为最外层
//   - 第一次迭代（i=len-1）：h = mws[len-1](next)，mws[len-1] 成为最内层
//
// 【为什么倒序遍历】
// 这是一个常见的函数组合技巧。假设我们有 mw1, mw2, mw3：
//   - 我们希望执行顺序是：mw1 -> mw2 -> mw3 -> Handler
//   - 倒序遍历时：先 mw3 包装 Handler，再 mw2 包装结果，最后 mw1 包装结果
//   - 最终：mw1(mw2(mw3(Handler)))
//   - 调用时：mw1 先执行，然后 mw2，然后 mw3，最后 Handler
//
// 【使用示例】
//
//	// 单独使用
//	tool := NewTool("mytool", "描述", handler,
//	    WithMiddleware(LoggingMiddleware()))
//
//	// 组合使用
//	tool := NewTool("mytool", "描述", handler,
//	    WithMiddleware(ChainMiddlewares(
//	        LoggingMiddleware(),
//	        AuthMiddleware(),
//	        MetricsMiddleware(),
//	    )))
//
// 【注意事项】
//   - 中间件列表为空时，返回的 Middleware 不做任何处理
//   - 中间件可以修改输入输出，使用时注意顺序
func ChainMiddlewares(mws ...Middleware) Middleware {
	return func(next Handler) Handler {
		h := next
		// 倒序遍历，确保 mws[0] 成为最外层包装
		// 这样调用时 mws[0] 先执行，符合直觉
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](h)
		}
		return h
	}
}
