package blades

import (
	"context"
)

// Handler 定义了一个处理 Invocation 并返回消息生成器的接口。
// Handler 是 Agent 内部执行的核心抽象，用于封装消息处理逻辑。
// 通过 Handler 接口，可以在 Agent 执行前后添加中间件逻辑。
type Handler interface {
	// Handle 处理调用并返回消息生成器。
	//
	// 参数：
	//   - ctx: 上下文
	//   - invocation: 调用对象
	//
	// 返回：
	//   - Generator[*Message, error]: 消息迭代器
	Handle(context.Context, *Invocation) Generator[*Message, error]
}

// HandleFunc 是一个适配器类型，允许普通函数实现 Handler 接口。
// 这是 Go 中常见的适配器模式，类似于 http.HandlerFunc。
// 通过 HandleFunc，可以直接使用函数作为 Handler，无需定义新类型。
//
// 使用示例：
//
//	handler := blades.HandleFunc(func(ctx context.Context, inv *blades.Invocation) blades.Generator[*blades.Message, error] {
//	    return func(yield func(*blades.Message, error) bool) {
//	        // 处理逻辑
//	        yield(response, nil)
//	    }
//	})
type HandleFunc func(context.Context, *Invocation) Generator[*Message, error]

// Handle 为 HandleFunc 实现 Handler 接口。
// 这使得 HandleFunc 可以直接作为 Handler 使用。
func (f HandleFunc) Handle(ctx context.Context, invocation *Invocation) Generator[*Message, error] {
	return f(ctx, invocation)
}

// Middleware 是中间件类型，用于包装 Handler 并添加额外行为。
// 中间件可以：
// 1. 在请求处理前执行逻辑（如日志记录、权限检查）
// 2. 在请求处理后执行逻辑（如结果转换、错误处理）
// 3. 修改或替换 Handler 的行为
// 4. 短路请求（如缓存命中时直接返回）
//
// 中间件通过 ChainMiddlewares 组合成链，按顺序执行（类似洋葱模型）。
//
// 使用示例：
//
//	// 日志中间件
//	func LoggingMiddleware(next blades.Handler) blades.Handler {
//	    return blades.HandleFunc(func(ctx context.Context, inv *blades.Invocation) blades.Generator[*blades.Message, error] {
//	        log.Printf("开始处理调用：%s", inv.ID)
//	        defer log.Printf("调用处理完成")
//	        return next.Handle(ctx, inv)
//	    })
//	}
type Middleware func(Handler) Handler

// ChainMiddlewares 将多个中间件组合成一个，按顺序应用。
// 第一个中间件成为最外层包装器（最先执行）。
//
// 执行顺序说明：
// 假设有中间件 M1, M2, M3，调用 ChainMiddlewares(M1, M2, M3)(handler)：
// 1. M1 最先执行（最外层）
// 2. M2 其次执行
// 3. M3 最后执行（最内层，最接近原始 handler）
//
// 这符合中间件的自然思维顺序：M1 -> M2 -> M3 -> handler
//
// 参数：
//   - mws: 中间件列表，按执行顺序排列
//
// 返回：
//   - Middleware: 组合后的中间件
//
// 使用示例：
//
//	// 组合多个中间件
//	combined := blades.ChainMiddlewares(
//	    LoggingMiddleware,      // 最外层，最先执行
//	    AuthMiddleware,         // 第二层
//	    MetricsMiddleware,      // 最内层，最后执行
//	)
//	agent := blades.NewAgent("助手",
//	    blades.WithModel(model),
//	    blades.WithMiddleware(combined),
//	)
func ChainMiddlewares(mws ...Middleware) Middleware {
	return func(next Handler) Handler {
		h := next
		// 从后向前应用中间件，使得 mws[0] 成为最外层
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](h)
		}
		return h
	}
}
