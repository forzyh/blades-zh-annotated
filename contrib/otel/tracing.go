// Package otel 提供了 OpenTelemetry 链路追踪集成。
//
// # OpenTelemetry 是什么
//
// OpenTelemetry 是一个可观测性框架，用于生成、收集和传输遥测数据（链路追踪、指标、日志）。
// 它提供了标准的 API 和 SDK，支持多种后端（如 Jaeger、Zipkin、OTLP 等）。
//
// # 在 blades 中的使用
//
// 本包提供了一个 blades.Middleware，可以为所有 Agent 调用自动添加链路追踪：
//   - 记录 Agent 名称、描述
//   - 记录使用的模型
//   - 记录 Token 用量（输入/输出）
//   - 记录结束原因（finish reason）
//   - 记录错误信息
//
// # 使用示例
//
//	// 创建带追踪的 Agent
//	agent := blades.NewAgent(
//	    blades.WithMiddleware(otel.Tracing(
//	        otel.WithSystem("openai"),
//	        otel.WithTracerProvider(tp),
//	    )),
//	)
//
//	// 调用 Agent
//	ctx := context.Background()
//	for msg, err := range agent.GenerateStream(ctx, request) {
//	    // 处理响应
//	}
//
// # Span 属性
//
// 每个 Agent 调用会创建一个 Span，包含以下属性：
//   - gen_ai.operation.name: invoke_agent
//   - gen_ai.system: AI 系统名称（如 "openai", "claude", "gemini"）
//   - gen_ai.agent.name: Agent 名称
//   - gen_ai.agent.description: Agent 描述
//   - gen_ai.request.model: 请求的模型
//   - gen_ai.conversation.id: 会话 ID
//   - gen_ai.response.finish_reasons: 结束原因
//   - gen_ai.usage.input_tokens: 输入 token 数
//   - gen_ai.usage.output_tokens: 输出 token 数
package otel

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/go-kratos/blades"
)

const (
	// traceScope 定义追踪的作用域名称。
	// 用于标识 traces 来自 blades 框架。
	traceScope = "blades"
)

// TraceOption 定义追踪中间件的配置选项函数。
//
// # 作用说明
//
// 使用函数选项模式（Functional Options Pattern）配置 tracing 中间件。
// 这种模式在 Go 中很常见，允许灵活地配置可选参数。
//
// # 可用选项
//
//   - WithSystem(system string): 设置 AI 系统名称
//   - WithTracerProvider(tr trace.TracerProvider): 设置自定义 TracerProvider
//
// # 使用示例
//
//	opts := []TraceOption{
//	    otel.WithSystem("openai"),
//	    otel.WithTracerProvider(myTracerProvider),
//	}
//	middleware := otel.Tracing(opts...)
type TraceOption func(*tracing)

// tracing 持有追踪中间件的配置。
//
// # 结构说明
//
//   - system: AI 系统名称（如 "openai", "claude", "gemini", "_OTHER"）
//   - tracer: OpenTelemetry Tracer，用于创建 Span
//   - next: 下一个处理器（被追踪的目标）
//
// # 默认值
//
// 如果未设置 system，默认为 "_OTHER"。
type tracing struct {
	system string // e.g., "openai", "claude", "gemini"
	tracer trace.Tracer
	next   blades.Handler
}

// WithSystem 设置 AI 系统名称，如 "openai", "claude", "gemini"。
//
// # 参数说明
//
//   - system: AI 系统名称
//
// # 返回值
//
// TraceOption: 配置函数
//
// # 作用说明
//
// 设置的 system 值会被记录到 Span 的 gen_ai.system 属性中。
// 这有助于在追踪后端中按 Provider 过滤和分析请求。
//
// # 使用示例
//
//	middleware := otel.Tracing(otel.WithSystem("openai"))
func WithSystem(system string) TraceOption {
	return func(t *tracing) {
		t.system = system
	}
}

// WithTracerProvider 设置自定义的 TracerProvider。
//
// # 参数说明
//
//   - tr: OpenTelemetry TracerProvider
//
// # 返回值
//
// TraceOption: 配置函数
//
// # 作用说明
//
// 默认情况下，中间件使用 otel.GetTracerProvider() 获取全局 TracerProvider。
// 如果应用配置了自定义的 TracerProvider（如配置了 OTLP 导出器），
// 可以使用此选项注入。
//
// # 使用示例
//
//	// 创建 OTLP Trace 导出器
//	exporter, _ := otlptrace.New(ctx, otlptracegrpc.NewClient())
//
//	// 创建 TracerProvider
//	tp := trace.NewTracerProvider(
//	    trace.WithBatcher(exporter),
//	)
//	defer tp.Shutdown(ctx)
//
//	// 使用自定义 TracerProvider
//	agent := blades.NewAgent(
//	    blades.WithMiddleware(otel.Tracing(
//	        otel.WithTracerProvider(tp),
//	    )),
//	)
func WithTracerProvider(tr trace.TracerProvider) TraceOption {
	return func(t *tracing) {
		t.tracer = tr.Tracer(traceScope)
	}
}

// Tracing 返回一个 OpenTelemetry 追踪中间件。
//
// # 参数说明
//
//   - opts: 配置选项
//
// # 返回值
//
// blades.Middleware: 追踪中间件
//
// # 中间件工作原理
//
// 1. 创建 tracing 实例，应用配置选项
// 2. 返回 Middleware 工厂函数
// 3. 当 Agent 处理请求时，tracing.Handle() 会被调用
// 4. 创建 Span，记录请求信息，结束 Span
//
// # 默认配置
//
//   - system: "_OTHER"
//   - tracer: otel.GetTracerProvider().Tracer(traceScope)
//
// # 使用示例
//
//	// 基本用法
//	agent := blades.NewAgent(
//	    blades.WithMiddleware(otel.Tracing()),
//	)
//
//	// 自定义系统名称
//	agent := blades.NewAgent(
//	    blades.WithMiddleware(otel.Tracing(
//	        otel.WithSystem("claude"),
//	    )),
//	)
func Tracing(opts ...TraceOption) blades.Middleware {
	t := &tracing{
		system: "_OTHER",
		tracer: otel.GetTracerProvider().Tracer(traceScope),
	}
	// 应用配置选项
	for _, o := range opts {
		o(t)
	}
	return func(next blades.Handler) blades.Handler {
		t.next = next
		return t
	}
}

// Start 开始一个新的追踪 Span。
//
// # 参数说明
//
//   - ctx: 上下文
//   - agent: Agent 上下文，包含 Agent 信息
//   - invocation: 调用信息，包含模型、会话等
//
// # 返回值
//
//   - context.Context: 携带 Span 的上下文
//   - trace.Span: 创建的 Span
//
// # Span 名称
//
// Span 名称格式："invoke_agent {agentName}"
//
// # Span 属性
//
//   - gen_ai.operation.name: invoke_agent（操作类型）
//   - gen_ai.system: AI 系统名称
//   - gen_ai.agent.name: Agent 名称
//   - gen_ai.agent.description: Agent 描述
//   - gen_ai.request.model: 请求的模型
//   - gen_ai.conversation.id: 会话 ID
func (t *tracing) Start(ctx context.Context, agent blades.AgentContext, invocation *blades.Invocation) (context.Context, trace.Span) {
	var sessionID string
	if invocation.Session != nil {
		sessionID = invocation.Session.ID()
	}
	// 创建 Span
	ctx, span := t.tracer.Start(ctx, fmt.Sprintf("invoke_agent %s", agent.Name()))
	// 设置 Span 属性（使用 OpenTelemetry 语义约定）
	span.SetAttributes(
		semconv.GenAIOperationNameInvokeAgent,
		semconv.GenAISystemKey.String(t.system),
		semconv.GenAIAgentName(agent.Name()),
		semconv.GenAIAgentDescription(agent.Description()),
		semconv.GenAIRequestModel(invocation.Model),
		semconv.GenAIConversationID(sessionID),
	)
	return ctx, span
}

// Handle 以流式方式处理提示，并在调用前后添加 OpenTelemetry 追踪。
//
// # 参数说明
//
//   - ctx: 上下文
//   - invocation: 调用信息
//
// # 返回值
//
// blades.Generator[*blades.Message, error]: 流式响应生成器
//
// # 处理流程
//
// 1. 从上下文获取 Agent 信息
// 2. 调用 Start() 创建 Span
// 3. 调用下一个处理器（实际 Agent）
// 4. 遍历流式响应，yield 每个消息
// 5. 调用 End() 结束 Span
//
// # 错误处理
//
// 如果流式处理中发生错误：
//   - 记录错误到 Span
//   - 设置 Span 状态为 Error
//   - 仍然调用 End() 确保 Span 结束
func (t *tracing) Handle(ctx context.Context, invocation *blades.Invocation) blades.Generator[*blades.Message, error] {
	agent, ok := blades.FromAgentContext(ctx)
	if !ok {
		// 如果无法获取 Agent 上下文，直接传递给下一个处理器
		return t.next.Handle(ctx, invocation)
	}
	return func(yield func(*blades.Message, error) bool) {
		var (
			err     error
			message *blades.Message
		)
		// 开始追踪
		ctx, span := t.Start(ctx, agent, invocation)
		// 调用下一个处理器
		streaming := t.next.Handle(ctx, invocation)
		// 遍历流式响应
		for message, err = range streaming {
			if err != nil {
				yield(nil, err)
				break
			}
			if !yield(message, nil) {
				// 用户取消
				break
			}
		}
		// 结束追踪
		t.End(span, message, err)
	}
}

// End 结束追踪 Span 并记录响应信息。
//
// # 参数说明
//
//   - span: 要结束的 Span
//   - msg: 最终的响应消息
//   - err: 处理过程中发生的错误
//
// # 处理逻辑
//
// 1. 确保 Span 结束（使用 defer）
// 2. 如果有错误：
//   - 记录错误
//   - 设置 Span 状态为 Error
//
// 3. 如果无错误：
//   - 设置 Span 状态为 Ok
//
// 4. 记录响应属性（如果有消息）：
//   - finish_reasons: 结束原因
//   - input_tokens: 输入 token 数
//   - output_tokens: 输出 token 数
func (t *tracing) End(span trace.Span, msg *blades.Message, err error) {
	defer span.End()
	if err != nil {
		// 记录错误
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		// 设置成功状态
		span.SetStatus(codes.Ok, codes.Ok.String())
	}
	if msg == nil {
		return
	}
	// 记录结束原因
	if msg.FinishReason != "" {
		span.SetAttributes(semconv.GenAIResponseFinishReasons(msg.FinishReason))
	}
	// 记录 Token 用量
	if msg.TokenUsage.InputTokens > 0 {
		span.SetAttributes(semconv.GenAIUsageInputTokens(int(msg.TokenUsage.InputTokens)))
	}
	if msg.TokenUsage.OutputTokens > 0 {
		span.SetAttributes(semconv.GenAIUsageOutputTokens(int(msg.TokenUsage.OutputTokens)))
	}
}
