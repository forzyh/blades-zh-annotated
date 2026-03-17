// Blades 示例：工具中间件（tools-middleware）
//
// 本示例演示如何为工具添加中间件。
// 工具中间件允许在工具执行前后添加额外逻辑，
// 如日志记录、权限检查、缓存、限流等。
//
// 适用场景：
// - 工具调用日志记录
// - 权限和访问控制
// - 请求/响应转换
// - 缓存和限流
//
// 核心概念：
// 1. Tool Middleware：包装工具处理函数的装饰器
// 2. WithMiddleware：为工具绑定中间件
// 3. Handler 链：多个中间件按顺序执行
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"log"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/tools"
	"github.com/google/jsonschema-go/jsonschema"
)

// timeHandle 是时间工具的处理函数
// 返回模拟的当前时间
func timeHandle(ctx context.Context, args string) (string, error) {
	log.Println("Time tool called with args:", args)
	return "The current time is 3:04 PM", nil
}

// weatherHandle 是天气工具的处理函数
// 返回模拟的天气预报
func weatherHandle(ctx context.Context, args string) (string, error) {
	log.Println("Weather tool called with args:", args)
	return "Sunny, 25°C", nil
}

// toolLogging 是一个日志记录中间件
// 它记录每个工具调用的请求参数
func toolLogging() tools.Middleware {
	return func(next tools.Handler) tools.Handler {
		return tools.HandleFunc(func(ctx context.Context, req string) (string, error) {
			// 前置处理：记录请求
			log.Println("Request received:", req)

			// 调用下一个处理器
			res, err := next.Handle(ctx, req)

			// 后置处理：记录响应
			if err != nil {
				log.Println("Error:", err)
			} else {
				log.Println("Response:", res)
			}

			return res, err
		})
	}
}

func main() {
	// 步骤 1: 创建时间工具
	// tools.NewTool 创建一个底层工具（相对于 NewFunc 的高级工具）
	// NewTool 使用字符串参数和返回值，更灵活但类型安全性较低
	timeTool := tools.NewTool(
		"get_current_time", // 工具名称
		"Get the current time", // 工具描述
		tools.HandleFunc(timeHandle), // 处理函数
		// 使用 WithInputSchema 设置输入 Schema
		// 这告诉 LLM 如何构造参数
		tools.WithInputSchema(&jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"timezone": {
					Type:        "string",
					Description: "The timezone to get the current time for",
				},
			},
		}),
		// 注意：这里没有添加中间件
	)

	// 步骤 2: 创建天气工具（带中间件）
	weatherTool := tools.NewTool(
		"get_weather",
		"Get the current weather for a given city",
		tools.HandleFunc(weatherHandle),
		tools.WithInputSchema(&jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"country": {
					Type:        "string",
					Description: "The country",
				},
				"current_time": {
					Type:        "string",
					Description: "The current time in the location",
				},
			},
		}),
		// 绑定日志中间件
		tools.WithMiddleware(toolLogging()),
	)

	// 步骤 3: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 4: 创建 Agent 并绑定工具
	agent, err := blades.NewAgent(
		"Weather Agent",
		blades.WithModel(model),
		blades.WithInstruction("You are a helpful assistant that provides weather information."),
		blades.WithTools(timeTool, weatherTool), // 绑定两个工具
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 5: 创建用户输入
	input := blades.UserMessage("What is the weather in New York City?")

	// 步骤 6: 创建 Runner 并使用流式执行
	runner := blades.NewRunner(agent)
	for output, err := range runner.RunStream(context.Background(), input) {
		if err != nil {
			log.Fatal(err)
		}
		// 输出每次迭代的消息
		// Role: 消息角色（user/assistant/tool）
		// Status: 消息状态（streaming/completed）
		// String(): 消息内容
		log.Println(output.Role, output.Status, output.String())
	}

	// 预期输出：
	// 1. Agent 可能会先调用 get_current_time 获取时间
	// 2. 然后调用 get_weather，此时会触发日志中间件：
	//    Request received: {"country": "New York City", ...}
	//    Response: Sunny, 25°C
	// 3. 最后返回整合后的回答
	//
	// 扩展提示：
	// 1. 多层中间件：
	//    tools.WithMiddleware(
	//        toolLogging(),
	//        toolAuth(),
	//        toolCache(),
	//    )
	//
	// 2. 认证中间件示例：
	//    func toolAuth() tools.Middleware {
	//        return func(next tools.Handler) tools.Handler {
	//            return tools.HandleFunc(func(ctx context.Context, req string) (string, error) {
	//                token := extractToken(ctx)
	//                if !validate(token) {
	//                    return "", errors.New("unauthorized")
	//                }
	//                return next.Handle(ctx, req)
	//            })
	//        }
	//    }
	//
	// 3. 缓存中间件示例：
	//    func toolCache() tools.Middleware {
	//        cache := make(map[string]string)
	//        return func(next tools.Handler) tools.Handler {
	//            return tools.HandleFunc(func(ctx context.Context, req string) (string, error) {
	//                if res, ok := cache[req]; ok {
	//                    return res, nil // 缓存命中
	//                }
	//                res, err := next.Handle(ctx, req)
	//                cache[req] = res // 缓存结果
	//                return res, err
	//            })
	//        }
	//    }
	//
	// 4. 限流中间件：
	//    使用 golang.org/x/time/rate 实现
}
