// Blades 示例：顺序 Recipe（recipe-sequential）
//
// 本示例演示如何使用 Recipe 定义顺序执行的多 Agent 管道。
// 顺序模式（sequential）允许将多个 Agent 串联起来，
// 前一个 Agent 的输出自动作为后一个 Agent 的输入。
//
// 适用场景：
// - 多阶段处理流程（草稿 -> 编辑 -> 审核）
// - 专业分工（翻译 -> 润色 -> 格式化）
// - 质量控制（生成 -> 验证 -> 修正）
// - 复杂任务分解
//
// 核心概念：
// 1. Sequential Recipe：顺序执行多个子 Agent
// 2. Pipeline（管道）：数据在 Agent 间流动
// 3. 流式输出：实时查看每个阶段的输出
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_API_KEY 环境变量
//       确保当前目录存在 agent.yaml 文件
package main

import (
	"context"
	"log"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/recipe"
)

func main() {
	// 步骤 1: 创建模型注册表并注册模型
	registry := recipe.NewModelRegistry()
	registry.Register("gpt-4o", openai.NewModel("gpt-4o", openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	}))

	// 步骤 2: 从 YAML 文件加载 Recipe
	// agent.yaml 定义了一个顺序执行的 Agent 管道
	// 示例结构：
	// ---
	// mode: sequential
	// name: CodeReviewPipeline
	// agents:
	//   - name: Linter
	//     model: gpt-4o
	//     instruction: Check for syntax errors and style issues.
	//   - name: Reviewer
	//     model: gpt-4o
	//     instruction: Review for logic errors and best practices.
	//   - name: Summarizer
	//     model: gpt-4o
	//     instruction: Summarize the feedback in bullet points.
	spec, err := recipe.LoadFromFile("agent.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 根据 Recipe 构建顺序 Agent 管道
	// Build 方法会：
	// 1. 按顺序创建每个子 Agent
	// 2. 配置 Agent 间的输入输出传递
	// 3. 应用模板参数（如 {{.language}}）
	agent, err := recipe.Build(spec,
		recipe.WithModelRegistry(registry),
		recipe.WithParams(map[string]any{"language": "go"}),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建 Runner 并执行
	runner := blades.NewRunner(agent)

	// 步骤 5: 使用流式输出查看每个阶段的处理结果
	// RunStream 返回一个流式生成器
	// 可以实时看到每个 Agent 的输出
	stream := runner.RunStream(context.Background(), blades.UserMessage(`
		Review this code:
		func divide(a, b int) int {
			return a / b
		}
	`))

	// 遍历流式响应
	for message, err := range stream {
		if err != nil {
			log.Fatal(err)
		}
		// message.Author 是 Agent/阶段名称
		// message.Text() 是该阶段的输出
		log.Printf("[%s] %s\n", message.Author, message.Text())
	}

	// 预期输出：
	// [Linter] No syntax or style issues found.
	// [Reviewer] The function doesn't handle division by zero...
	// [Summarizer] - Add division by zero check
	//              - Consider adding input validation
	//
	// 扩展提示：
	// 1. agent.yaml 完整示例（顺序模式）：
	//    ---
	//    mode: sequential
	//    name: WritingPipeline
	//    agents:
	//      - name: Writer
	//        model: gpt-4o
	//        instruction: Write a draft about {{.topic}}.
	//      - name: Editor
	//        model: gpt-4o
	//        instruction: Edit the draft for clarity and coherence.
	//      - name: Proofreader
	//        model: gpt-4o
	//        instruction: Check for grammar and spelling errors.
	//    params:
	//      topic: climate change
	//
	// 2. 数据传递：
	//    每个 Agent 的输出会自动传递给下一个 Agent
	//    也可以在 instruction 中使用 {{.previous_agent}} 引用前一阶段的输出
	//
	// 3. 条件执行：
	//    可以在 Agent 中添加逻辑，根据输入决定跳过某些阶段
	//
	// 4. 错误处理：
	//    如果某个阶段失败，整个管道会停止
	//    可以添加重试中间件提高可靠性
}
