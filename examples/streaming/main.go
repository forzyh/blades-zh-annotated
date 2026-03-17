// Blades 示例：流式输出（streaming）
//
// 本示例演示如何使用 Blades 的流式输出功能。
// 流式输出允许实时接收 LLM 的生成内容，
// 而不需要等待完整响应，提供更好的用户体验。
//
// 适用场景：
// - 聊天机器人实时回复
// - 长文本生成的进度展示
// - 降低感知延迟（首字快速）
// - 实时翻译/转写
//
// 核心概念：
// 1. RunStream：流式执行方法，返回 Generator
// 2. Generator：Go 迭代器，支持 range 遍历
// 3. Message Status：消息状态（streaming/completed）
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
)

func main() {
	// 步骤 1: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建 Agent
	agent, err := blades.NewAgent(
		"Stream Agent",
		blades.WithModel(model),
		blades.WithInstruction("You are a helpful assistant that provides detailed answers."),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建用户输入
	input := blades.UserMessage("What is the capital of France?")

	// 步骤 4: 创建 Runner 并执行流式请求
	runner := blades.NewRunner(agent)

	// RunStream 返回一个 Generator[*blades.Message, error]
	// 这是一个迭代器，可以使用 range 遍历
	// 每次迭代返回一个部分响应（chunk）
	stream := runner.RunStream(context.Background(), input)

	// 步骤 5: 遍历流式响应
	for m, err := range stream {
		if err != nil {
			// 处理错误（网络错误、API 错误等）
			log.Fatal(err)
		}

		// m.Status 表示消息状态：
		// - StatusStreaming: 正在流式传输
		// - StatusCompleted: 流式传输完成
		//
		// m.Text() 返回当前累积的文本内容
		// 注意：每次迭代返回的是完整文本，不是增量
		log.Println(m.Status, m.Text())
	}

	// 预期输出（多次迭代）：
	// streaming The
	// streaming The capital
	// streaming The capital of
	// streaming The capital of France
	// streaming The capital of France is
	// streaming The capital of France is Paris
	// completed The capital of France is Paris.
	//
	// 注意：实际输出取决于 LLM 的流式响应速度
	//
	// 扩展提示：
	// 1. 实时显示（类似打字机效果）：
	//    for m, err := range stream {
	//        if err != nil { log.Fatal(err) }
	//        fmt.Print(m.Text()) // 打印增量
	//    }
	//
	// 2. 处理 SSE（Server-Sent Events）：
	//    w.Header().Set("Content-Type", "text/event-stream")
	//    for m, err := range stream {
	//        if err != nil { break }
	//        fmt.Fprintf(w, "data: %s\n\n", m.Text())
	//        if f, ok := w.(http.Flusher); ok { f.Flush() }
	//    }
	//
	// 3. 收集完整响应：
	//    var buf strings.Builder
	//    for m, err := range stream {
	//        if err != nil { break }
	//        buf.WriteString(m.Text())
	//    }
	//    fullResponse := buf.String()
	//
	// 4. 流式中断：
	//    可以在遍历中根据条件 break 提前终止
	//    例如：检测到敏感内容时停止生成
	//
	// 5. 并发处理：
	//    可以在单独的 goroutine 中处理流式数据
	//    使用 channel 传递给前端
}
