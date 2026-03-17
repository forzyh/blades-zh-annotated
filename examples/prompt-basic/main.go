// Blades 示例：基础提示词（prompt-basic）
//
// 本示例演示如何使用 Blades 框架发送基础提示词（Prompt）给 LLM。
// 提示词工程是与 LLM 交互的核心技术，决定了模型的回答质量。
//
// 适用场景：
// - 简单问答
// - 内容生成
// - 信息查询
// - 快速测试模型能力
//
// 核心概念：
// 1. Prompt（提示词）：发送给 LLM 的指令或问题
// 2. UserMessage：表示用户输入的消息类型
// 3. Instruction（指令）：定义 Agent 角色和行为的系统消息
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
	// WithInstruction 设置系统指令，影响 Agent 的整体行为
	// 系统指令对用户不可见，但会指导模型的回答方式
	agent, err := blades.NewAgent(
		"Instructions Agent",
		blades.WithModel(model),
		// 使用模板字符串作为指令
		// {{.style}} 是一个占位符，会在运行时被替换
		blades.WithInstruction("Respond as a {{.style}}."),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建会话并设置状态
	// Session 用于在多次调用间保持状态
	// 这里设置了一个 "style" 变量，用于填充指令模板
	session := blades.NewSession()
	session.SetState("style", "robot") // 设置为机器人风格

	// 步骤 4: 创建用户输入
	input := blades.UserMessage("Tell me a joke.")

	// 步骤 5: 创建 Runner 并执行
	ctx := context.Background()
	runner := blades.NewRunner(agent)
	// WithSession 绑定会话，使 Agent 可以访问会话状态
	message, err := runner.Run(ctx, input, blades.WithSession(session))
	if err != nil {
		panic(err)
	}

	// 步骤 6: 输出结果
	// 显示会话状态（包含 style 变量）
	log.Println(session.State())
	// 显示 Agent 的回答
	log.Println(message.Text())

	// 预期输出：
	// state: map[style:robot]
	// output: [以机器人风格讲述的笑话]
	//
	// 扩展提示：
	// 1. 提示词最佳实践：
	//    - 清晰具体：明确说明需要什么
	//    - 提供上下文：帮助模型理解场景
	//    - 示例演示：给出期望的回答格式
	//    - 分步指导：复杂任务分解为步骤
	//
	// 2. 系统指令示例：
	//    - "You are a helpful coding assistant."
	//    - "You are a friendly customer support agent."
	//    - "You are an expert translator. Translate to {{.target_language}}."
	//
	// 3. 提示词模板：
	//    可以使用 Go 的 text/template 创建复杂模板
	//    参见 prompt-template/main.go 示例
}
