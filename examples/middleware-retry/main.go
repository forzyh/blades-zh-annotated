// Blades 示例：重试中间件（middleware-retry）
//
// 本示例演示如何使用重试中间件（Retry Middleware）自动处理临时性错误。
// 重试中间件可以捕获失败并自动重新执行操作，提高系统的容错能力。
//
// 适用场景：
// - 处理网络抖动或临时服务不可用
// - 应对 API 限流（Rate Limit）
// - 处理偶发性错误
// - 提高外部依赖的可靠性
//
// 核心概念：
// 1. Retry Middleware：自动重试失败的 Handler
// 2. 最大重试次数：防止无限重试
// 3. 模拟错误：演示重试行为的示例
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/middleware"
	"github.com/go-kratos/blades/stream"
)

// mockRetry 创建一个模拟重试场景的中间件
// 这是一个教学示例，展示如何编写自定义中间件
// 第一次调用返回错误，第二次调用成功
func mockRetry() blades.Middleware {
	attempts := 0 // 使用闭包保存状态，记录调用次数
	return func(next blades.Handler) blades.Handler {
		// 返回一个 Handler，使用 HandleFunc 简化创建
		return blades.HandleFunc(func(ctx context.Context, invocation *blades.Invocation) blades.Generator[*blades.Message, error] {
			// 第一次调用时返回模拟错误
			if attempts == 0 {
				attempts++
				// stream.Error 创建一个立即返回错误的 Generator
				return stream.Error[*blades.Message](errors.New("mock error"))
			}
			// 第二次调用时，将请求传递给下一个 Handler（正常执行）
			return next.Handle(ctx, invocation)
		})
	}
}

func main() {
	// 步骤 1: 创建 OpenAI 模型提供者
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建 Agent 并绑定中间件链
	// 中间件执行顺序：mockRetry -> middleware.Retry
	// 1. mockRetry 先包装 Agent，第一次调用返回错误
	// 2. Retry(2) 再包装 mockRetry，捕获错误并重试（最多 2 次）
	//
	// 执行流程：
	// - 第 1 次：mockRetry 返回错误 -> Retry 捕获并重试
	// - 第 2 次：mockRetry 成功，调用 Agent -> 返回结果
	agent, err := blades.NewAgent(
		"RetryAgent",
		blades.WithModel(model),
		blades.WithMiddleware(
			mockRetry(),     // 模拟错误的中间件（第一次调用失败）
			middleware.Retry(2), // 重试中间件，最多重试 2 次
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	msg, err := runner.Run(context.Background(), blades.UserMessage("What is the capital of France?"))
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 输出结果
	fmt.Println(msg)

	// 预期输出：
	// 尽管 mockRetry 第一次返回错误，但重试中间件会自动重试
	// 最终应该成功获得 Agent 的回复
	//
	// 如果没有重试中间件，程序会在第一次调用时失败
	//
	// 扩展提示：
	// 1. 配置重试策略：
	//    - 固定间隔重试：middleware.Retry(3)
	//    - 指数退避：middleware.RetryWithBackoff(3, time.Second)
	//    - 自定义重试条件：middleware.RetryIf(errCondition)
	//
	// 2. 只重试特定错误：
	//    middleware.RetryIf(func(err error) bool {
	//        return errors.Is(err, networkErr) // 只重试网络错误
	//    })
	//
	// 3. 添加重试日志：
	//    可以在中间件中添加日志记录重试次数和原因
}
