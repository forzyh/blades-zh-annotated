// Blades 示例：循环状态工作流（workflow-loop-state）
//
// 本示例演示如何在 Loop Agent 中使用状态管理和条件判断。
// 通过 Condition 函数，可以根据输出内容动态决定是否继续循环，
// 实现更灵活的循环控制逻辑。
//
// 适用场景：
// - 条件驱动的迭代流程
// - 基于内容质量的循环控制
// - 多轮对话和协商
// - 需要动态终止条件的任务
//
// 核心概念：
// 1. Loop Condition：自定义循环继续/终止条件
// 2. OutputKey：Agent 输出在状态中的键名
// 3. 模板引用：使用 {{.key}} 引用状态数据
// 4. 状态驱动：根据状态决定循环行为
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"log"
	"os"
	"strings"

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
		"WriterAgent",
		blades.WithModel(model),
		blades.WithInstruction(`Draft a short paragraph on climate change.
			{{if .suggestions}}
			**Draft**
			{{.draft}}

			Here are the suggestions to consider:
			{{.suggestions}}
			{{end}}
		`),
		// WithOutputKey 指定输出保存到状态的键名
		// 这样后续 Agent 可以通过 {{.draft}} 引用
		blades.WithOutputKey("draft"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 ReviewerAgent（审核 Agent）
	// 负责审查草稿并提供改进建议
	// WithOutputKey("suggestions") 将输出保存到 state["suggestions"]
	reviewerAgent, err := blades.NewAgent(
		"ReviewerAgent",
		blades.WithModel(model),
		blades.WithInstruction(`Review the draft and suggest improvements.
			If the draft is good, respond with "The draft is good".

			**Draft**
			{{.draft}}
		`),
		// WithOutputKey 指定输出保存到状态的键名
		// Condition 函数会通过这个值决定是否继续循环
		blades.WithOutputKey("suggestions"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建 Loop Agent
	// LoopConfig 配置循环行为
	loopAgent := flow.NewLoopAgent(flow.LoopConfig{
		Name:          "WritingReviewFlow", // 循环名称
		Description:   "An agent that loops between writing and reviewing until the draft is good.", // 描述
		MaxIterations: 3, // 最大迭代次数，防止无限循环
		// Condition 函数：自定义循环继续条件
		// 返回 true 继续循环，返回 false 终止循环
		Condition: func(ctx context.Context, state flow.LoopState) (bool, error) {
			// 检查审核意见是否包含通过标记
			// 如果包含"The draft is good"，说明审核通过，终止循环
			if strings.Contains(state.Output.Text(), "The draft is good") {
				return false, nil // 终止循环
			}
			return true, nil // 继续循环
		},
		SubAgents: []blades.Agent{
			writerAgent,
			reviewerAgent,
		},
	})

	// 步骤 5: 创建用户输入
	input := blades.UserMessage("Please write a short paragraph about climate change.")

	// 步骤 6: 创建 Runner 并执行
	runner := blades.NewRunner(loopAgent)
	stream := runner.RunStream(context.Background(), input)

	// 步骤 7: 遍历流式输出
	for message, err := range stream {
		if err != nil {
			log.Fatal(err)
		}
		// 输出每条消息
		// [WriterAgent] 草稿内容...
		// [ReviewerAgent] 审核意见或"The draft is good"...
		log.Println(message.Author, message.Text())
	}

	// 预期流程：
	// 迭代 1:
	//   WriterAgent: 创建初稿（保存到 state["draft"]）
	//   ReviewerAgent: "需要改进句子结构和添加数据"（保存到 state["suggestions"]）
	//   Condition: 返回 true，继续循环
	//
	// 迭代 2:
	//   WriterAgent: 根据建议修改（使用 {{.draft}} 和 {{.suggestions}}）
	//   ReviewerAgent: "The draft is good"
	//   Condition: 返回 false，终止循环
	//
	// 输出：
	// WriterAgent [草稿内容]
	// ReviewerAgent [审核意见]
	// WriterAgent [修改后的草稿]
	// ReviewerAgent The draft is good
	//
	// 扩展提示：
	// 1. 复杂条件判断：
	//    Condition: func(ctx context.Context, state flow.LoopState) (bool, error) {
	//        // 检查质量分数
	//        if state.Output.Score >= 8.0 {
	//            return false, nil
	//        }
	//        // 检查迭代次数
	//        if state.Iteration >= 5 {
	//            return false, nil // 达到最大迭代
	//        }
	//        return true, nil
	//    }
	//
	// 2. 多条件组合：
	//    // 同时检查多个状态值
	//    hasData := strings.Contains(state.State["draft"], "data")
	//    hasCitations := strings.Contains(state.State["draft"], "citation")
	//    if hasData && hasCitations {
	//        return false, nil
	//    }
	//
	// 3. 外部信号：
	//    // 检查是否有外部中断信号
	//    select {
	//    case <-ctx.Done():
	//        return false, ctx.Err()
	//    default:
	//        return true, nil
	//    }
	//
	// 4. 状态持久化：
	//    将 state.State 保存到数据库
	//    支持跨进程/服务继续循环
}
