// Blades 示例：窗口式上下文管理（context-window）
//
// 本示例演示如何使用 window.ContextManager 来管理对话上下文窗口。
// 窗口式策略只保留最近的 N 条消息或固定的 token 预算，超出部分直接从前面丢弃。
// 这是一种简单但有效的上下文管理策略，适用于不需要长期记忆的场景。
//
// 与摘要式上下文管理的区别：
// - 窗口式：直接丢弃旧消息，简单快速，但会丢失历史信息
// - 摘要式：将旧消息压缩成摘要，保留关键信息，但消耗更多 token
//
// 适用场景：
// - 简单的问答场景，不需要长期记忆
// - 对 token 成本敏感的应用
// - 每次对话相对独立，上下文依赖较弱
//
// 核心配置：
// - WithMaxMessages: 最多保留的消息数量
// - WithMaxTokens: 最多保留的 token 数量
// 两者同时设置时，任一条件满足都会触发截断
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
	"github.com/go-kratos/blades/context/window"
	"github.com/go-kratos/blades/contrib/openai"
)

func main() {
	// 步骤 1: 创建 OpenAI 模型提供者
	// openai.NewModel 创建一个实现 blades.ModelProvider 接口的模型实例
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建窗口式上下文管理器
	// window.NewContextManager 创建一个基于滑动窗口的上下文管理器
	// 它只保留最近的 N 条消息，超出部分直接丢弃
	contextManager := window.NewContextManager(
		// WithMaxMessages: 最多保留 4 条消息
		// 当消息数量超过 4 条时，最早的_messages_会被丢弃
		window.WithMaxMessages(4),
		// WithMaxTokens: 设置 2000 token 的预算上限
		// 即使消息数量未超限，token 超限也会触发截断
		window.WithMaxTokens(2000),
	)

	// 步骤 3: 创建 Agent（智能体）
	// - 名称：WindowDemo
	// - 模型：使用上面创建的 OpenAI 模型
	// - 指令：要求助手简洁回答
	agent, err := blades.NewAgent(
		"WindowDemo",
		blades.WithModel(model),
		blades.WithInstruction("You are a helpful assistant. Answer concisely."),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建上下文和会话
	// ctx 用于控制请求的超时和取消
	// session 用于在多次调用间保持对话历史
	ctx := context.Background()
	session := blades.NewSession()

	// 步骤 5: 创建 Runner 并绑定上下文管理器
	// Runner 是执行 Agent 的入口，可以配置各种选项
	runner := blades.NewRunner(
		agent,
		blades.WithContextManager(contextManager), // 绑定窗口式上下文管理器
	)

	// 步骤 6: 定义多轮对话内容
	// 通过 6 轮对话演示窗口式上下文的行为
	// 由于设置了 MaxMessages=4，第 5 轮对话时第 1 条消息会被丢弃
	turns := []string{
		"My favourite colour is blue.",   // 第 1 轮：喜欢的颜色
		"My favourite animal is a dolphin.", // 第 2 轮：喜欢的动物
		"My favourite food is sushi.",    // 第 3 轮：喜欢的食物
		"My favourite sport is tennis.",  // 第 4 轮：喜欢的运动
		"My favourite book is Dune.",     // 第 5 轮：喜欢的书籍（此时第 1 条消息被丢弃）
		// 此时上下文只包含最近 4 条消息（动物、食物、运动、书籍）
		// 第 1 条关于颜色的消息已被丢弃
		"Can you list all the favourite things I mentioned?", // 测试 Agent 的记忆
	}

	// 步骤 7: 执行对话循环
	// 每次调用 runner.Run 都会：
	// 1. 将用户输入添加到会话历史
	// 2. 检查是否超出消息数量或 token 限制
	// 3. 如超出，从前面丢弃旧消息
	// 4. 调用 LLM 生成回复
	// 5. 将回复添加到会话历史
	for _, input := range turns {
		log.Printf("User: %s", input)
		output, err := runner.Run(ctx, blades.UserMessage(input), blades.WithSession(session))
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Agent: %s\n", output.Text())
	}

	// 预期结果：
	// 最后一轮对话时，Agent 可能无法回忆起所有之前提到的内容
	// 因为它只能看到最近的 4 条消息，最早的颜色信息已丢失
	// 这演示了窗口式上下文管理的局限性
}
