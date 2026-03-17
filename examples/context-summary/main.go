// Blades 示例：摘要式上下文管理（context-summary）
//
// 本示例演示如何使用 summary.ContextManager 来管理长对话的上下文窗口。
// 当对话内容超出 token 预算时，ContextManager 会自动调用 LLM 将早期对话压缩成摘要，
// 同时保留最近的几条消息原文。这种策略可以在有限的上下文窗口内维持更长的对话。
//
// 适用场景：
// - 需要多轮对话的助手应用
// - 对话历史可能超出模型上下文窗口限制的场景
// - 希望在保持对话连贯性的同时控制 token 消耗
//
// 核心概念：
// 1. ContextManager（上下文管理器）：负责管理对话历史的生命周期
// 2. 摘要压缩：使用 LLM 将旧对话压缩成简洁的摘要
// 3. Token 预算：设置上下文的最大 token 数量，超出时触发压缩
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
	"github.com/go-kratos/blades/context/summary"
	"github.com/go-kratos/blades/contrib/openai"
)

func main() {
	// 步骤 1: 创建 OpenAI 模型提供者
	// openai.NewModel 创建一个实现 blades.ModelProvider 接口的模型实例
	// 需要从环境变量中获取模型名称和 API 密钥
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建摘要式上下文管理器
	// summary.NewContextManager 创建一个能够自动压缩对话历史的上下文管理器
	// 它会在 token 数量超过阈值时，使用 LLM 将旧对话压缩成摘要
	contextManager := summary.NewContextManager(
		// WithSummarizer: 指定用于生成摘要的模型
		// 生产环境中可以使用更便宜/更快的模型专门用于摘要生成
		summary.WithSummarizer(model),
		// WithMaxTokens: 设置上下文的最大 token 预算（约 500 tokens）
		// 当上下文超过这个阈值时，触发摘要压缩
		summary.WithMaxTokens(500),
		// WithKeepRecent: 始终保留最近的 3 条消息原文
		// 这确保最近的对话内容保持完整，不被压缩
		summary.WithKeepRecent(3),
		// WithBatchSize: 每次摘要压缩操作最多处理 5 条消息
		// 较小的批量可以减少单次摘要的 token 消耗
		summary.WithBatchSize(5),
	)

	// 步骤 3: 创建 Agent（智能体）
	// blades.NewAgent 创建一个基本的 Agent 实例
	// - 第一个参数是 Agent 的名称，用于日志和调试
	// - WithModel 指定使用的 LLM 模型
	// - WithInstruction 设置系统指令，定义 Agent 的行为准则
	agent, err := blades.NewAgent(
		"SummaryDemo", // Agent 名称
		blades.WithModel(model), // 使用 OpenAI 模型
		blades.WithInstruction("You are a helpful assistant. Answer concisely."), // 系统指令
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建上下文和会话
	// ctx 是 Go 标准库的 context，用于控制超时和取消
	// session 是 Blades 的会话对象，用于在多次调用间保持状态
	ctx := context.Background()
	session := blades.NewSession()

	// 步骤 5: 创建 Runner（执行器）
	// Runner 负责执行 Agent，可以配置额外的选项
	// WithContextManager 将摘要管理器绑定到 Runner，自动处理上下文压缩
	runner := blades.NewRunner(
		agent,
		blades.WithContextManager(contextManager),
	)

	// 步骤 6: 定义多轮对话
	// 通过多轮对话演示上下文管理器的摘要压缩功能
	// 随着对话进行，早期的消息会被压缩成摘要
	turns := []string{
		"Tell me a one-sentence fact about the Sun.",    // 关于太阳的事实
		"Tell me a one-sentence fact about the Moon.",   // 关于月球的事实
		"Tell me a one-sentence fact about Mars.",       // 关于火星的事实
		"Tell me a one-sentence fact about Jupiter.",    // 关于木星的事实
		"Tell me a one-sentence fact about Saturn.",     // 关于土星的事实
		"Tell me a one-sentence fact about Venus.",      // 关于金星的事实
		// 此时，早期的消息可能已经被压缩成摘要以保持在 token 预算内
		"Which planets have I asked about so far?", // 测试 Agent 是否还记得之前问过的行星
	}

	// 步骤 7: 执行对话循环
	// 每次调用 runner.Run 都会：
	// 1. 将用户输入添加到会话历史
	// 2. 检查上下文是否超出 token 预算，如超出则触发摘要压缩
	// 3. 调用 LLM 生成回复
	// 4. 将回复添加到会话历史
	for _, input := range turns {
		log.Printf("User: %s", input)
		output, err := runner.Run(ctx, blades.UserMessage(input), blades.WithSession(session))
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Agent: %s\n", output.Text())
	}
}
