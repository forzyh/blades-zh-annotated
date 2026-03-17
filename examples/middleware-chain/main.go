// Blades 示例：中间件链（middleware-chain）
//
// 本示例演示如何将多个中间件组合成链式结构，按顺序处理请求。
// 中间件链允许在不修改核心业务逻辑的情况下，横向扩展功能（如日志、监控、安全等）。
//
// 适用场景：
// - 多层安全防护（认证 + 授权 + 内容过滤）
// - 全链路监控（日志 + 指标 + 追踪）
// - 请求/响应转换（压缩、加密、格式化）
// - 限流和熔断
//
// 核心概念：
// 1. Middleware Chain（中间件链）：多个中间件按顺序包装执行
// 2. 执行顺序：请求从外向内传递，响应从内向外返回
// 3. WithMiddleware：Agent 配置选项，支持多个中间件
//
// 中间件链执行流程：
// 请求：Logging -> Guardrails -> Agent
// 响应：Agent -> Guardrails -> Logging -> Client
//
// 使用方法：
// go run .
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
	// 步骤 1: 创建 OpenAI 模型提供者
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建 Agent 并绑定中间件链
	// WithMiddleware 接受多个 Middleware 工厂函数
	// 中间件按声明顺序依次执行（Logging -> Guardrails）
	//
	// 中间件链的包装过程：
	// 1. Logging 包装 Guardrails
	// 2. Guardrails 包装 Agent 的默认 Handler
	// 3. 最终形成：Logging -> Guardrails -> Agent
	agent, err := blades.NewAgent(
		"History Tutor", // Agent 名称
		blades.WithModel(model),
		blades.WithInstruction("You are a helpful assistant that provides detailed and accurate information."),
		blades.WithMiddleware(
			NewLogging,   // 第一层：日志记录中间件
			NewGuardrails, // 第二层：防护栏中间件
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建用户输入消息
	input := blades.UserMessage("What is the capital of France?")

	// 步骤 4: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	output, err := runner.Run(context.Background(), input)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 5: 输出结果
	log.Println("runner:", output.Text())

	// 预期日志输出：
	// 1. 首先打印："Applying guardrails to the prompt (streaming)"
	// 2. 然后打印："Streaming with guardrails applied: [响应内容]"
	// 3. 最后打印："logging: agent(History Tutor) prompt(...) succeeded after ...: ..."
	//
	// 注意日志顺序反映了中间件的执行流程：
	// - 请求阶段：Logging 先记录开始，然后 Guardrails 应用检查
	// - 响应阶段：Guardrails 先观察流式响应，然后 Logging 记录结果
}
