// Blades 示例：指令模板（prompt-instructions）
//
// 本示例演示如何在 Agent 指令中使用模板变量。
// 通过模板，可以动态调整 Agent 的行为和风格，实现灵活的配置。
//
// 适用场景：
// - 多风格助手（正式/休闲/专业等）
// - 多语言支持
// - 可配置的角色扮演
// - A/B 测试不同指令效果
//
// 核心概念：
// 1. Instruction Template（指令模板）：包含占位符的系统指令
// 2. Session State（会话状态）：存储模板变量的值
// 3. 模板渲染：运行时将占位符替换为实际值
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

	// 步骤 2: 创建 Agent，使用带模板的指令
	// 指令中的 {{.style}} 是一个占位符
	// 运行时会被 Session 中的 "style" 变量值替换
	agent, err := blades.NewAgent(
		"Instructions Agent",
		blades.WithModel(model),
		blades.WithInstruction("Respond as a {{.style}}."), // 模板指令
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建会话并设置模板变量
	// Session 不仅保持对话历史，还存储模板变量的值
	session := blades.NewSession()
	// 设置 "style" 变量为 "robot"
	// Agent 的指令会被渲染为："Respond as a robot."
	session.SetState("style", "robot")

	// 步骤 4: 创建用户输入
	input := blades.UserMessage("Tell me a joke.")

	// 步骤 5: 创建 Runner 并执行
	ctx := context.Background()
	runner := blades.NewRunner(agent)
	// WithSession 绑定会话，使 Agent 可以：
	// 1. 访问模板变量（用于渲染指令）
	// 2. 保持对话历史（多轮对话）
	message, err := runner.Run(ctx, input, blades.WithSession(session))
	if err != nil {
		panic(err)
	}

	// 步骤 6: 输出结果
	// 显示会话状态，包含所有变量
	log.Println(session.State())
	// 显示 Agent 的回答（应该是以机器人口吻讲的笑话）
	log.Println(message.Text())

	// 预期输出：
	// map[style:robot]
	// Beep boop... Here's a joke from my database:
	// Why did the robot go on vacation? Because his circuits were fried!
	//
	// 扩展提示：
	// 1. 多变量模板：
	//    blades.WithInstruction("You are a {{.role}} speaking {{.tone}} to a {{.audience}}.")
	//    session.SetState("role", "teacher")
	//    session.SetState("tone", "enthusiastically")
	//    session.SetState("audience", "students")
	//
	// 2. 动态切换风格：
	//    // 第一轮：休闲风格
	//    session.SetState("style", "friend")
	//    runner.Run(ctx, input, blades.WithSession(session))
	//    // 第二轮：正式风格
	//    session.SetState("style", "professor")
	//    runner.Run(ctx, input2, blades.WithSession(session))
	//
	// 3. 模板语法：
	//    - {{.var}}: 简单变量
	//    - {{if .cond}}...{{end}}: 条件渲染
	//    - {{range .items}}...{{end}}: 循环
	//    完整语法参考 Go 的 text/template 包
}
