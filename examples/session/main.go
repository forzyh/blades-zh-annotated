// Blades 示例：会话管理（session）
//
// 本示例演示如何使用 Blades 的 Session（会话）功能。
// Session 用于在多轮对话中保持上下文和状态，
// 使 Agent 能够记住之前的对话内容和自定义状态。
//
// 适用场景：
// - 多轮对话机器人
// - 有状态的交互流程
// - 用户偏好和上下文保持
// - 跨请求的数据共享
//
// 核心概念：
// 1. Session（会话）：保持对话历史和自定义状态
// 2. Message History：自动维护的对话历史
// 3. State：用户自定义的键值对状态
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
		"History Tutor",
		blades.WithModel(model),
		blades.WithInstruction("You are a knowledgeable history tutor. Provide detailed and accurate information on historical events."),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建用户输入
	input := blades.UserMessage("Can you tell me about the causes of World War II?")

	// 步骤 4: 创建会话
	// NewSession 创建一个空会话
	// 会话会自动维护对话历史（用户输入和 Agent 输出）
	session := blades.NewSession()

	// 步骤 5: （可选）设置自定义状态
	// 可以在会话中存储任意键值对数据
	// 这些数据可以在 Agent 指令模板中使用，或在工具中访问
	session.SetState("user_level", "advanced")
	session.SetState("preferred_language", "en")

	// 步骤 6: 创建 Runner 并执行
	ctx := context.Background()
	runner := blades.NewRunner(agent)
	// WithSession 绑定会话到本次执行
	output, err := runner.Run(ctx, input, blades.WithSession(session))
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 7: 输出结果
	log.Println(output.Text())

	// 步骤 8: 查看会话状态
	// 会话现在包含了完整的对话历史
	log.Println("Session state:", session.State())

	// 多轮对话示例：
	// 可以在同一会话上继续调用 Run
	// followup := blades.UserMessage("What were the major battles?")
	// output2, _ := runner.Run(ctx, followup, blades.WithSession(session))
	// Agent 会记得之前的对话内容

	// 预期输出：
	// Agent 会回答关于二战起因的问题
	// Session state 包含对话历史和自定义状态
	//
	// 扩展提示：
	// 1. 对话历史管理：
	//    - 会话自动保存所有对话历史
	//    - 使用 ContextManager 控制历史长度
	//    - 参见 context-window 和 context-summary 示例
	//
	// 2. 状态持久化：
	//    - 将会话状态保存到数据库
	//    - 支持跨服务/进程恢复会话
	//    - 实现用户级别的长期记忆
	//
	// 3. 会话模板：
	//    session := blades.NewSession(map[string]any{
	//        "user_name": "Alice",
	//        "topic": "history",
	//    })
	//    // 在指令中使用：You are tutoring {{.user_name}} in {{.topic}}.
	//
	// 4. 会话隔离：
	//    - 每个用户/对话创建独立会话
	//    - 使用会话 ID 标识和检索
	//    - 注意并发访问安全
	//
	// 5. 会话序列化：
	//    - 使用 JSON 编码保存会话
	//    - 支持断点续传和迁移
	//    - 注意敏感数据脱敏
}
