// Blades 示例：工具流式输出（tools-streaming）
//
// 本示例演示如何在工具调用中使用流式输出。
// 流式输出允许实时查看工具调用的过程，
// 对于长时间运行的工具特别有用。
//
// 适用场景：
// - 长时间运行的工具（如数据分析）
// - 需要进度反馈的操作
// - 流式数据处理
// - 实时日志和调试
//
// 核心概念：
// 1. RunStream：流式执行方法
// 2. Tool Call 消息：LLM 调用工具时的消息类型
// 3. Tool Result 消息：工具执行结果的消息类型
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

func main() {
	// 步骤 1: 创建时间工具
	timeTool := tools.NewTool(
		"get_current_time",
		"Get the current time",
		tools.HandleFunc(timeHandle),
		tools.WithInputSchema(&jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"timezone": {
					Type:        "string",
					Description: "The timezone to get the current time for",
				},
			},
		}),
	)

	// 步骤 2: 创建天气工具
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
			},
		}),
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
		blades.WithTools(timeTool, weatherTool),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 5: 创建用户输入
	input := blades.UserMessage("What is the weather in New York City?")

	// 步骤 6: 创建 Runner 并使用流式执行
	runner := blades.NewRunner(agent)

	// RunStream 返回一个 Generator，可以遍历所有消息
	// 包括：
	// - 用户输入消息
	// - LLM 思考过程（ToolCall）
	// - 工具执行结果（ToolResult）
	// - 最终回答（Assistant）
	for output, err := range runner.RunStream(context.Background(), input) {
		if err != nil {
			log.Fatal(err)
		}

		// 输出每条消息的详细信息：
		// Role: 消息角色
		//   - user: 用户消息
		//   - assistant: Agent 回复
		//   - tool: 工具调用/结果
		//
		// Status: 消息状态
		//   - streaming: 正在流式传输
		//   - completed: 传输完成
		//
		// String(): 消息内容
		log.Println(output.Role, output.Status, output.String())
	}

	// 预期输出流程：
	// 1. assistant streaming: [LLM 决定调用工具]
	// 2. tool streaming: get_weather({"country": "New York City"})
	// 3. tool completed: Sunny, 25°C
	// 4. assistant streaming: The current weather in New York City is...
	// 5. assistant completed: ...Sunny, 25°C.
	//
	// 扩展提示：
	// 1. 过滤特定消息类型：
	//    for output, err := range stream {
	//        if output.Role == "tool" {
	//            log.Println("Tool called:", output.String())
	//        }
	//    }
	//
	// 2. 只显示完成消息：
	//    for output, err := range stream {
	//        if output.Status == blades.StatusCompleted {
	//            log.Println(output.Text())
	//        }
	//    }
	//
	// 3. 收集工具调用：
	//    var toolCalls []string
	//    for output, err := range stream {
	//        if output.Role == "tool" {
	//            toolCalls = append(toolCalls, output.String())
	//        }
	//    }
	//
	// 4. 实时进度显示：
	//    for output, err := range stream {
	//        fmt.Printf("[%s] %s\n", output.Role, output.Text())
	//    }
	//
	// 5. 前端集成（SSE）：
	//    for output, err := range stream {
	//        fmt.Fprintf(w, "data: %s\n\n", output.String())
	//        if f, ok := w.(http.Flusher); ok {
	//            f.Flush()
	//        }
	//    }
}
