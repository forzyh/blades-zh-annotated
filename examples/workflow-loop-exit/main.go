// Blades 示例：循环退出工作流（workflow-loop-exit）
//
// 本示例演示如何使用 ExitTool 实现自主的写审循环。
// Loop Agent 会自动执行"写作 - 审核"循环，直到审核者认为内容达标
// 或达到最大迭代次数。ExitTool 允许审核者决定何时退出循环。
//
// 适用场景：
// - 自主内容创作和编辑
// - 代码生成和审查循环
// - 需要质量把关的迭代任务
// - 自动升级（escalation）到人工作业的场景
//
// 核心概念：
// 1. Loop Agent：自动循环执行子 Agent
// 2. ExitTool：允许 Agent 主动退出循环的工具
// 3. Escalation：当需要人工干预时升级
// 4. ContextManager：管理循环过程中的上下文窗口
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/context/window"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/flow"
	"github.com/go-kratos/blades/tools"
)

// workflow-loop-exit 演示完全自主的写审循环
// 用户提供一次输入，循环自动运行直到审核者满意
//
// 上下文管理：
//   - 跨迭代的历史记录由 Loop 自动累积
//   - 每条子 Agent 的调用上下文会自动注入历史
//   - blades.WithContextManager 在每次模型调用前修剪上下文
//     防止多轮迭代后上下文无限增长
func main() {
	// 步骤 1: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建 WriterAgent（写作 Agent）
	// 负责根据用户输入或审核反馈撰写/修改文章
	writerAgent, err := blades.NewAgent(
		"WriterAgent",
		blades.WithModel(model),
		blades.WithInstruction(`You are a skilled writer.
Write a short paragraph on the topic given by the user.
If conversation history contains reviewer feedback, revise accordingly.`),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 ReviewerAgent（审核 Agent）
	// 负责审查文章质量，决定是否需要修改
	// 绑定了 ExitTool，可以主动退出循环
	reviewerAgent, err := blades.NewAgent(
		"ReviewerAgent",
		blades.WithModel(model),
		blades.WithInstruction(`You are a critical editor.
Review the most recent draft in the conversation history and decide:
- If it meets a high standard, call the exit tool with a brief reason.
- If it needs minor improvement, provide concise feedback as plain text.
- If it has fundamental problems requiring human judgement, call exit with escalate=true.`),
		blades.WithTools(tools.NewExitTool()), // 绑定退出工具
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建 Loop Agent
	// Loop Agent 会自动循环执行子 Agent
	// 执行顺序：WriterAgent -> ReviewerAgent -> WriterAgent -> ...
	loopAgent := flow.NewLoopAgent(flow.LoopConfig{
		Name:          "WritingReviewFlow", // 循环名称
		Description:   "Autonomous write-review loop driven by ExitTool.", // 描述
		MaxIterations: 5, // 最大迭代次数，防止无限循环
		SubAgents:     []blades.Agent{writerAgent, reviewerAgent},
	})

	// 步骤 5: 配置上下文管理器
	// WithContextManager 在 Runner 上配置一次，对 Loop 中所有 Agent 生效
	// 窗口式管理：只保留最近 8 条消息（约 4 个写审轮次）
	runner := blades.NewRunner(loopAgent,
		blades.WithContextManager(window.NewContextManager(window.WithMaxMessages(8))),
	)

	// 步骤 6: 执行循环
	// 使用 RunStream 实时查看循环过程
	stream := runner.RunStream(context.Background(), blades.UserMessage("Write about the impact of climate change on coastal cities."))

	// 步骤 7: 处理循环输出
	for message, err := range stream {
		// 检查是否触发了升级（需要人工介入）
		if errors.Is(err, blades.ErrLoopEscalated) {
			log.Println("Escalated — requires human review.")
			return
		}

		if err != nil {
			log.Fatal(err)
		}

		// 输出每条消息
		// [WriterAgent] 文章内容...
		// [ReviewerAgent] 审核意见或退出原因...
		log.Printf("[%s] %s\n", message.Author, message.Text())
	}

	// 预期流程：
	// 迭代 1:
	//   WriterAgent: 撰写关于气候变化对沿海城市影响的初稿
	//   ReviewerAgent: "需要更多具体数据和案例"
	// 迭代 2:
	//   WriterAgent: 添加数据和案例修改
	//   ReviewerAgent: "退出：文章质量达标"
	// 循环结束
	//
	// 或者（升级场景）：
	// 迭代 3:
	//   ReviewerAgent: "退出（升级）：需要领域专家审核数据准确性"
	// 触发 ErrLoopEscalated 错误
	//
	// 扩展提示：
	// 1. 自定义退出条件：
	//    可以在指令中定义更具体的退出标准
	//    "Exit if: word count > 300, contains 3+ data points, no grammar errors"
	//
	// 2. 多轮反馈：
	//    ReviewerAgent 可以提供结构化反馈
	//    "Feedback: 1) Add data 2) Improve transitions 3) Fix typos"
	//
	// 3. 上下文优化：
	//    - 使用摘要式管理（context-summary）保留更多信息
	//    - 调整窗口大小平衡性能和记忆
	//
	// 4. 迭代控制：
	//    - 增加 MaxIterations 允许更多改进机会
	//    - 减少 MaxIterations 提高执行速度
	//
	// 5. 升级处理：
	//    捕获 ErrLoopEscalated 后触发人工审核流程
	//    - 发送邮件通知
	//    - 创建工单系统任务
	//    - 保存到待审核队列
}
