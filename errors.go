package blades

import (
	"errors"
)

var (
	// ErrNoSessionContext 当上下文中缺少 Session 时返回此错误。
	// 通常在调用 FromSessionContext 时未找到 Session 时发生。
	ErrNoSessionContext = errors.New("session not found in context")

	// ErrNoAgentContext 当上下文中缺少 Agent 时返回此错误。
	// 通常在调用 FromAgentContext 时未找到 Agent 时发生。
	ErrNoAgentContext = errors.New("agent not found in context")

	// ErrNoInvocationContext 当上下文中缺少 Invocation 时返回此错误。
	// 在某些需要 Invocation 上下文的处理中可能会检查此错误。
	ErrNoInvocationContext = errors.New("invocation not found in context")

	// ErrModelProviderRequired 当缺少必需的 ModelProvider 时返回此错误。
	// 在创建 Agent 时，WithModel 是必选选项，否则会返回此错误。
	ErrModelProviderRequired = errors.New("model provider is required")

	// ErrMaxIterationsExceeded 当 Agent 超过最大迭代次数时返回此错误。
	// 这通常发生在 Agent 陷入工具调用循环或复杂任务中。
	// 可以通过 WithMaxIterations(n) 调整最大迭代次数。
	ErrMaxIterationsExceeded = errors.New("maximum iterations exceeded in agent execution")

	// ErrNoFinalResponse 当 Agent 流结束但没有返回最终响应时返回此错误。
	// 可能发生在 Agent 异常退出或没有生成任何消息时。
	ErrNoFinalResponse = errors.New("stream ended without a final response")

	// ErrInterrupted 当执行被中断时返回此错误。
	// 通常由上下文取消或外部中断信号触发。
	ErrInterrupted = errors.New("execution was interrupted")

	// ErrLoopEscalated 当循环条件信号需要升级到外部处理程序时返回此错误。
	// 这用于在循环执行模式下处理无法在循环内解决的问题。
	ErrLoopEscalated = errors.New("loop escalated to outer handler")
)
