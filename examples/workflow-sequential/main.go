// Blades 示例：顺序工作流（workflow-sequential）
//
// 本示例演示如何使用 Sequential Agent 实现顺序执行的工作流。
// Sequential Agent 将多个 Agent 按顺序串联，前一个 Agent 的输出
// 自动传递给后一个 Agent，适合多阶段处理流程。
//
// 适用场景：
// - 多阶段内容生成（草稿 -> 编辑 -> 审核）
// - 流水线处理任务
// - 链式思考和推理
// - 分步问题解决
//
// 核心概念：
// 1. Sequential Agent：按顺序执行多个子 Agent
// 2. 数据传递：前一个 Agent 的输出传递给下一个
// 3. OutputKey：Agent 输出在状态中的键名
// 4. 模板引用：使用 {{.key}} 引用之前的输出
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
	"github.com/go-kratos/blades/flow"
)

func main() {
	// 步骤 1: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建 WriterAgent（写作 Agent）
	// 负责起草关于气候变化的段落
	// WithOutputKey("draft") 将输出保存到 state["draft"]
	// 这样后续 Agent 可以通过 {{.draft}} 引用
	writerAgent, err := blades.NewAgent(
		"WriterAgent",
		blades.WithModel(model),
		blades.WithInstruction("Draft a short paragraph on climate change."),
		blades.WithOutputKey("draft"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 ReviewerAgent（审核 Agent）
	// 负责审查草稿并提供改进建议
	// 使用 {{.draft}} 引用 WriterAgent 的输出
	reviewerAgent, err := blades.NewAgent(
		"ReviewerAgent",
		blades.WithModel(model),
		blades.WithInstruction(`Review the draft and suggest improvements.
			Draft: {{.draft}}`),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建 Sequential Agent
	// flow.NewSequentialAgent 将多个 Agent 串联起来
	// 执行顺序：WriterAgent -> ReviewerAgent
	sequentialAgent := flow.NewSequentialAgent(flow.SequentialConfig{
		Name: "WritingReviewFlow", // 工作流名称
		SubAgents: []blades.Agent{
			writerAgent,
			reviewerAgent,
		},
	})

	// 步骤 5: 准备用户输入
	input := blades.UserMessage("Please write a short paragraph about climate change.")

	// 步骤 6: 创建 Runner 并执行
	runner := blades.NewRunner(sequentialAgent)
	stream := runner.RunStream(context.Background(), input)

	// 步骤 7: 遍历流式输出
	for message, err := range stream {
		if err != nil {
			log.Fatal(err)
		}
		// 输出每条消息的作者和内容
		// message.Author: Agent 名称（WriterAgent 或 ReviewerAgent）
		// message.Text(): Agent 的输出内容
		log.Println(message.Author, message.Text())
	}

	// 预期执行流程：
	// 1. WriterAgent 执行：
	//    输入：用户请求
	//    输出：草稿内容（保存到 state["draft"]）
	//
	// 2. ReviewerAgent 执行：
	//    输入：{{.draft}} 模板被替换为 WriterAgent 的输出
	//    输出：审核意见和改进建议
	//
	// 输出示例：
	// WriterAgent Climate change is one of the most pressing issues of our time...
	// ReviewerAgent The draft is well-structured. Consider adding: 1) Specific data...
	//
	// 扩展提示：
	// 1. 更多阶段：
	//    sequentialAgent := flow.NewSequentialAgent(flow.SequentialConfig{
	//        SubAgents: []blades.Agent{
	//            writerAgent,    // 写作
	//            editorAgent,    // 编辑
	//            reviewerAgent,  // 审核
	//            formatterAgent, // 格式化
	//        },
	//    })
	//
	// 2. 嵌套流程：
	//    可以在 Sequential 中嵌套 Parallel
	//    参见 workflow-parallel 示例
	//
	// 3. 条件执行：
	//    根据前一个 Agent 的输出决定是否执行后续 Agent
	//    if shouldSkip(output) { continue }
	//
	// 4. 错误处理：
	//    某个 Agent 失败时，可以：
	//    - 中止整个流程
	//    - 跳过失败 Agent
	//    - 使用备用 Agent
	//
	// 5. 状态管理：
	//    使用 Session 在多次运行间保持状态
	//    session := blades.NewSession()
	//    runner.Run(ctx, input, blades.WithSession(session))
	//
	// 6. Recipe 模式：
	//    使用 YAML 配置顺序工作流
	//    参见 recipe-sequential 示例
}
