// Blades 示例：并行工作流（workflow-parallel）
//
// 本示例演示如何使用 Parallel Agent 实现并行执行。
// Parallel Agent 可以同时执行多个独立的子 Agent，
// 然后将结果汇聚，适合处理可并行化的任务。
//
// 适用场景：
// - 多版本生成和比较
// - 并行编辑（语法、风格、结构）
// - 多角度分析
// - 批量独立任务处理
//
// 核心概念：
// 1. Parallel Agent：并行执行多个子 Agent
// 2. 独立执行：每个子 Agent 独立运行，互不依赖
// 3. 结果汇聚：所有子 Agent 完成后继续后续流程
// 4. 嵌套组合：Parallel 可以嵌套在 Sequential 中
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
	writerAgent, err := blades.NewAgent(
		"writerAgent",
		blades.WithModel(model),
		blades.WithInstruction("Draft a short paragraph on climate change."),
		blades.WithOutputKey("draft"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建两个并行编辑 Agent

	// 语法编辑 Agent
	// 负责检查语法错误
	editorAgent1, err := blades.NewAgent(
		"editorAgent1",
		blades.WithModel(model),
		blades.WithInstruction(`Edit the paragraph for grammar.
			**Paragraph:**
			{{.draft}}
		`),
		blades.WithOutputKey("grammar_edit"), // 输出保存到 state["grammar_edit"]
	)
	if err != nil {
		log.Fatal(err)
	}

	// 风格编辑 Agent
	// 负责优化写作风格
	// 注意：这里名称是"editorAgent1"，应该是"editorAgent2"（示例代码原有问题）
	editorAgent2, err := blades.NewAgent(
		"editorAgent1",
		blades.WithModel(model),
		blades.WithInstruction(`Edit the paragraph for style.
			**Paragraph:**
			{{.draft}}
		`),
		blades.WithOutputKey("style_edit"), // 输出保存到 state["style_edit"]
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建最终审核 Agent
	// 负责整合语法和风格编辑的结果
	reviewerAgent, err := blades.NewAgent(
		"finalReviewerAgent",
		blades.WithModel(model),
		blades.WithInstruction(`Consolidate the grammar and style edits into a final version.
			**Draft:**
			{{.draft}}

			**Grammar Edit:**
			{{.grammar_edit}}

			**Style Edit:**
			{{.style_edit}}
		`),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 5: 创建 Parallel Agent
	// Parallel Agent 并行执行 editorAgent1 和 editorAgent2
	// 两个编辑互不依赖，可以同时进行
	parallelAgent := flow.NewParallelAgent(flow.ParallelConfig{
		Name:        "EditorParallelAgent",
		Description: "Edits the drafted paragraph in parallel for grammar and style.",
		SubAgents: []blades.Agent{
			editorAgent1,
			editorAgent2,
		},
	})

	// 步骤 6: 创建 Sequential Agent
	// 将 Writer -> Parallel(Editors) -> Reviewer 串联起来
	sequentialAgent := flow.NewSequentialAgent(flow.SequentialConfig{
		Name:        "WritingSequenceAgent",
		Description: "Drafts, edits, and reviews a paragraph about climate change.",
		SubAgents: []blades.Agent{
			writerAgent,    // 第一步：写作
			parallelAgent,  // 第二步：并行编辑（语法 + 风格）
			reviewerAgent,  // 第三步：整合审核
		},
	})

	// 步骤 7: 创建会话
	// Session 用于在 Agent 间传递状态
	session := blades.NewSession()

	// 步骤 8: 准备用户输入
	input := blades.UserMessage("Please write a short paragraph about climate change.")

	// 步骤 9: 执行工作流
	ctx := context.Background()
	runner := blades.NewRunner(sequentialAgent)
	stream := runner.RunStream(ctx, input, blades.WithSession(session))

	// 步骤 10: 遍历流式输出
	for message, err := range stream {
		if err != nil {
			log.Fatal(err)
		}
		// 只显示已完成的消息
		if message.Status != blades.StatusCompleted {
			continue
		}
		log.Println(message.Author, message.Text())
	}

	// 预期执行流程：
	// 1. writerAgent 执行：
	//    输出草稿 -> state["draft"]
	//
	// 2. parallelAgent 执行（并行）：
	//    - editorAgent1 语法编辑 -> state["grammar_edit"]
	//    - editorAgent2 风格编辑 -> state["style_edit"]
	//    （两个编辑同时执行，提高效率）
	//
	// 3. reviewerAgent 执行：
	//    使用 {{.draft}}、{{.grammar_edit}}、{{.style_edit}}
	//    整合为最终版本
	//
	// 输出示例：
	// writerAgent [草稿内容]
	// editorAgent1 [语法编辑结果]
	// editorAgent1 [风格编辑结果]
	// finalReviewerAgent [最终整合版本]
	//
	// 扩展提示：
	// 1. 更多并行任务：
	//    parallelAgent := flow.NewParallelAgent(flow.ParallelConfig{
	//        SubAgents: []blades.Agent{agent1, agent2, agent3, agent4},
	//    })
	//
	// 2. 嵌套流程：
	//    Sequential(
	//        agent1,
	//        Parallel(agent2, agent3),
	//        Sequential(agent4, agent5),
	//        Parallel(agent6, agent7),
	//    )
	//
	// 3. 并行 vs 串行：
	//    | 特性 | 并行 | 串行 |
	//    |------|------|------|
	//    | 速度 | 快   | 慢   |
	//    | 依赖 | 无   | 有   |
	//    | 成本 | 高   | 低   |
	//
	// 4. 结果合并：
	//    可以自定义合并逻辑
	//    mergeResults(results []string) string
	//
	// 5. 错误隔离：
	//    一个子 Agent 失败不影响其他 Agent
	//    可以收集所有成功结果
}
