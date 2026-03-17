// Blades 示例：恢复工作流（resume-workflow）
//
// 本示例演示如何从错误中恢复并继续执行工作流。
// 当 Agent 执行过程中遇到错误时，可以保存执行状态，
// 修复问题后从断点处恢复执行，而不需要重新开始。
//
// 适用场景：
// - 长流程任务的错误恢复
// - 外部依赖故障后的重试
// - 多阶段工作流的断点续传
// - 调试和开发过程中的迭代测试
//
// 核心概念：
// 1. ErrInterrupted：表示执行被中断（可由错误触发）
// 2. WithResume：标记是否为恢复执行
// 3. InvocationID：标识一次完整的执行流程
// 4. Session State：在多次执行间保持状态
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
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/flow"
	"github.com/go-kratos/blades/stream"
)

// mockErr 创建一个模拟错误的中间件
// 这是一个教学示例，展示如何模拟错误并演示恢复流程
// 第一次调用返回错误，第二次（恢复时）调用成功
func mockErr() blades.Middleware {
	return func(next blades.Handler) blades.Handler {
		return blades.HandleFunc(func(ctx context.Context, invocation *blades.Invocation) blades.Generator[*blades.Message, error] {
			// 如果不是恢复执行，返回模拟错误
			if !invocation.Resume {
				return stream.Error[*blades.Message](errors.New("[ERROR] Simulated error in ReviewerAgent"))
			}
			// 恢复执行时，正常调用下一个 Handler
			return next.Handle(ctx, invocation)
		})
	}
}

func main() {
	// 步骤 1: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建 WriterAgent（写作 Agent）
	// 负责起草关于气候变化的段落
	writerAgent, err := blades.NewAgent(
		"WriterAgent",
		blades.WithModel(model),
		blades.WithInstruction("Draft a short paragraph on climate change."),
		blades.WithOutputKey("draft"), // 输出保存到状态的键名
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 ReviewerAgent（审核 Agent）
	// 负责审核草稿并提供改进建议
	// 绑定了 mockErr 中间件，第一次调用会失败
	reviewerAgent, err := blades.NewAgent(
		"ReviewerAgent",
		blades.WithModel(model),
		blades.WithInstruction(`Review the draft and suggest improvements.
			Draft: {{.draft}}`), // 使用模板引用 WriterAgent 的输出
		blades.WithOutputKey("review"),
		blades.WithMiddleware(mockErr()), // 模拟错误的中间件
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建 RefactorAgent（重构 Agent）
	// 根据审核意见修改草稿
	refactorAgent, err := blades.NewAgent(
		"RefactorAgent",
		blades.WithModel(model),
		blades.WithInstruction(`Refactor the draft based on the review.
			Draft: {{.draft}}
			Review: {{.review}}`),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 5: 创建顺序工作流
	// flow.NewSequentialAgent 将多个 Agent 串联起来
	// 执行顺序：WriterAgent -> ReviewerAgent -> RefactorAgent
	sequentialAgent := flow.NewSequentialAgent(flow.SequentialConfig{
		Name: "WritingReviewFlow",
		SubAgents: []blades.Agent{
			writerAgent,
			reviewerAgent,
			refactorAgent,
		},
	})

	// 步骤 6: 准备输入和会话
	input := blades.UserMessage("Please write a short paragraph about climate change.")
	ctx := context.Background()
	session := blades.NewSession()
	invocationID := "invocation-001" // 唯一标识这次执行

	// ==================== 第一次运行（会遇到错误） ====================

	runner := blades.NewRunner(sequentialAgent)
	stream := runner.RunStream(
		ctx,
		input,
		blades.WithSession(session),
		blades.WithInvocationID(invocationID),
	)

	log.Println("=== First Run (will encounter error) ===")
	for message, err := range stream {
		if err != nil {
			log.Println(err) // 记录错误
			break
		}
		// 只显示已完成的消息
		if message.Status != blades.StatusCompleted {
			continue
		}
		log.Println("first:", message.Author, message.Text())
	}

	// 此时 ReviewerAgent 失败，工作流中断
	// WriterAgent 的输出已保存到 session 中

	// ==================== 恢复执行 ====================

	resumeRunner := blades.NewRunner(sequentialAgent)
	resumeStream := resumeRunner.RunStream(
		ctx,
		input,
		blades.WithResume(true), // 标记为恢复执行
		blades.WithSession(session),
		blades.WithInvocationID(invocationID),
	)

	log.Println("=== Resume Run (should succeed) ===")
	for message, err := range resumeStream {
		if err != nil {
			log.Println(err)
			break
		}
		if message.Status != blades.StatusCompleted {
			continue
		}
		log.Println("second:", message.Author, message.Text())
	}

	// 预期流程：
	// 第一次运行：
	// 1. WriterAgent 成功执行，生成草稿
	// 2. ReviewerAgent 触发模拟错误，执行中断
	// 3. 日志显示错误信息
	//
	// 恢复运行：
	// 1. WriterAgent 被跳过（因为已有输出）
	// 2. ReviewerAgent 成功执行（mockErr 检测到 Resume=true）
	// 3. RefactorAgent 成功执行
	// 4. 日志显示完整的执行结果
	//
	// 扩展提示：
	// 1. 错误恢复策略：
	//    - 自动重试：对于临时错误，自动恢复执行
	//    - 人工介入：对于需要判断的错误，等待人工修复
	//    - 降级处理：跳过失败步骤，继续执行
	//
	// 2. 状态持久化：
	//    - 将 Session 保存到数据库
	//    - 支持跨进程/服务恢复
	//    - 实现真正的工作流引擎
	//
	// 3. 错误分类：
	//    - 临时错误：网络抖动、服务不可用 -> 自动重试
	//    - 永久错误：数据无效、配置错误 -> 人工修复
	//    - 业务错误：审批拒绝、条件不满足 -> 业务处理
}
