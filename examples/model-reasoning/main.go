// Blades 示例：基础 Agent（model-reasoning）
//
// 本示例演示如何使用 Blades 框架创建一个基础的 Agent。
// 这是最简单的 Agent 使用方式，适合入门学习。
//
// 适用场景：
// - 快速原型开发
// - 简单的问答应用
// - 学习和测试 Blades 框架
// - 基础对话机器人
//
// 核心概念：
// 1. Agent（智能体）：Blades 的核心组件，封装了与 LLM 的交互
// 2. ModelProvider（模型提供者）：抽象的模型接口，支持多种 LLM 后端
// 3. Runner（执行器）：负责执行 Agent，可以配置额外选项
// 4. Message（消息）：用户输入和 Agent 输出的数据结构
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
	// 步骤 1: 创建模型提供者
	// openai.NewModel 创建一个实现 blades.ModelProvider 接口的 OpenAI 模型
	// 参数：
	//   - 模型名称：如 "gpt-4o", "gpt-4-turbo", "gpt-3.5-turbo"
	//   - Config：配置选项
	//     - APIKey: OpenAI API 密钥（从环境变量读取）
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建 Agent
	// blades.NewAgent 创建一个 Agent 实例
	// 参数：
	//   - 第一个参数：Agent 名称，用于日志和调试
	//   - WithModel：指定使用的模型
	//   - WithInstruction：设置系统指令，定义 Agent 的角色和行为准则
	agent, err := blades.NewAgent(
		"Basic Agent", // Agent 名称
		blades.WithModel(model), // 使用 OpenAI 模型
		blades.WithInstruction("You are a helpful assistant that provides detailed and accurate information."),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建用户输入消息
	// blades.UserMessage 创建一个用户消息
	// 可以传入字符串或复杂的内容部分（文本、图像、文件等）
	input := blades.UserMessage("What is the capital of France?")

	// 步骤 4: 创建 Runner
	// Runner 负责执行 Agent，可以配置会话、上下文管理等选项
	runner := blades.NewRunner(agent)

	// 步骤 5: 运行 Agent
	// Run 方法发送请求到 LLM 并等待完整响应
	// 参数：
	//   - context.Background(): Go 上下文，用于控制超时和取消
	//   - input: 用户输入消息
	// 返回值：
	//   - output: Agent 的输出消息
	//   - err: 可能的错误
	output, err := runner.Run(context.Background(), input)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 6: 输出结果
	// output.Text() 提取模型的文本回复
	log.Println(output.Text())

	// 扩展提示：
	// 1. 使用 Session 保持对话历史：
	//    session := blades.NewSession()
	//    runner.Run(ctx, input, blades.WithSession(session))
	//
	// 2. 流式输出（实时显示响应）：
	//    stream := runner.RunStream(ctx, input)
	//    for msg, err := range stream {
	//        fmt.Print(msg.Text())
	//    }
	//
	// 3. 添加工具支持：
	//    agent, _ := blades.NewAgent("...",
	//        blades.WithTools(myTool),
	//    )
	//
	// 4. 使用其他模型提供商：
	//    - anthropic.NewModel(...)  // Claude
	//    - gemini.NewModel(...)     // Google Gemini
}
