// Blades 示例：提示词调用（prompt-invocation）
//
// 本示例演示如何使用底层 Invocation API 直接调用 Agent。
// Invocation 是 Blades 框架中请求执行的核心数据结构，
// 提供了比 Runner 更细粒度的控制。
//
// 适用场景：
// - 需要自定义执行逻辑
// - 构建高级抽象（如 Runner 的替代品）
// - 直接访问 Agent 的流式输出
// - 实现自定义中间件或包装器
//
// 核心概念：
// 1. Invocation（调用）：包含请求消息和执行选项的数据结构
// 2. Agent.Run：直接运行 Agent，返回流式生成器
// 3. Generator：Go 迭代器，用于流式处理响应
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
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建 Agent
	agent, err := blades.NewAgent(
		"Invocation Agent",
		blades.WithModel(model),
		blades.WithInstruction("You are a helpful assistant that provides detailed and accurate information."),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建用户输入消息
	input := blades.UserMessage("What is the capital of France?")

	// 步骤 4: 直接调用 Agent.Run
	// 这是比 Runner.Run 更底层的 API
	// Agent.Run 返回一个 Generator（迭代器），可以遍历流式响应
	//
	// Invocation 结构包含：
	// - Message: 用户输入消息
	// - Resume: 是否从上次中断处恢复
	// - 其他执行选项
	//
	// 使用 range 遍历 Generator：
	// - 每次迭代返回一个 (*Message, error)
	// - 第一个值是部分响应（流式）
	// - 最后一个值是完整响应
	for output, err := range agent.Run(context.Background(), &blades.Invocation{Message: input}) {
		if err != nil {
			log.Fatal(err)
		}
		// 输出每次迭代的响应
		// 对于非流式模型，这可能只有一次迭代
		// 对于流式模型，这里会多次输出
		log.Println(output.Text())
	}

	// 与 Runner 的对比：
	//
	// 1. Runner.Run（高层 API）：
	//    runner := blades.NewRunner(agent)
	//    output, err := runner.Run(ctx, input)
	//    // 等待完整响应后返回
	//
	// 2. Agent.Run（底层 API）：
	//    for output, err := range agent.Run(ctx, &blades.Invocation{Message: input}) {
	//        // 可以处理每个流式片段
	//    }
	//
	// 3. Runner.RunStream（流式高层 API）：
	//    stream := runner.RunStream(ctx, input)
	//    for msg, err := range stream {
	//        // 处理流式片段
	//    }
	//
	// 扩展提示：
	// 1. 自定义 Invocation：
	//    invocation := &blades.Invocation{
	//        Message: input,
	//        Resume:  true, // 从上次中断处恢复
	//    }
	//
	// 2. 处理错误：
	//    for output, err := range agent.Run(ctx, invocation) {
	//        if err != nil {
	//            // 处理错误
	//            log.Printf("Error: %v", err)
	//            break
	//        }
	//        // 处理成功响应
	//    }
	//
	// 3. 构建自定义 Runner：
	//    可以基于 Agent.Run 构建自己的 Runner，
	//    添加日志、指标、缓存等功能
}
