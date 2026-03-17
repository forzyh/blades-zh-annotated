// Blades 示例：中间件链 - 日志记录（logging.go）
//
// 本文件演示如何实现一个自定义日志中间件 Logging，用于记录 Agent 的执行情况。
// 日志中间件可以记录每次请求的耗时、输入、输出和错误信息，
// 对于调试、监控和审计非常有用。
//
// 适用场景：
// - 调试 Agent 行为
// - 性能监控和耗时分析
// - 审计日志（记录谁在什么时候做了什么）
// - 错误追踪和问题排查
//
// 核心概念：
// 1. Middleware（中间件）：包装 Handler 的装饰器
// 2. Generator 迭代：通过 range 遍历流式响应
// 3. 上下文提取：从 ctx 中获取 AgentContext 等元数据
//
// 注意：这是中间件链示例的一部分，与 guardrails.go 和 main.go 一起使用
package main

import (
	"context"
	"log"
	"time"

	"github.com/go-kratos/blades"
)

// Logging 是一个日志记录中间件结构体
// 它实现了 blades.Handler 接口，可以包装其他 Handler 并记录执行日志
type Logging struct {
	next blades.Handler // 下一个处理器（被包装的 Handler）
}

// NewLogging 创建一个新的 Logging 中间件
// 这是中间件工厂函数，符合 blades.Middleware 类型签名
// 参数 next 是被包装的下一个 Handler
// 返回值 blades.Handler 是包装后的 Handler
func NewLogging(next blades.Handler) blades.Handler {
	return &Logging{next}
}

// onError 是错误处理回调
// 当 Agent 执行失败时调用，记录错误信息
// 参数：
//   - start: 请求开始时间，用于计算耗时
//   - agent: Agent 上下文，包含 Agent 名称等元数据
//   - invocation: 调用信息，包含用户输入消息
//   - err: 发生的错误
func (m *Logging) onError(start time.Time, agent blades.AgentContext, invocation *blades.Invocation, err error) {
	log.Printf("logging: agent(%s) prompt(%s) failed after %s: %v",
		agent.Name(),                    // Agent 名称
		invocation.Message.String(),     // 用户输入消息
		time.Since(start),               // 执行耗时
		err,                             // 错误信息
	)
}

// onSuccess 是成功处理回调
// 当 Agent 成功生成响应时调用，记录成功日志
// 参数：
//   - start: 请求开始时间，用于计算耗时
//   - agent: Agent 上下文
//   - invocation: 调用信息，包含用户输入
//   - output: Agent 的输出消息
func (m *Logging) onSuccess(start time.Time, agent blades.AgentContext, invocation *blades.Invocation, output *blades.Message) {
	log.Printf("logging: agent(%s) prompt(%s) succeeded after %s: %s",
		agent.Name(),                    // Agent 名称
		invocation.Message.String(),     // 用户输入消息
		time.Since(start),               // 执行耗时
		output.String(),                 // Agent 输出消息
	)
}

// Handle 是 Logging 中间件的核心方法
// 它包装下一个 Handler 并记录执行日志
// 参数：
//   - ctx: 上下文，包含请求的生命周期信息
//   - invocation: 调用信息，包含用户消息、会话状态等
// 返回值：
//   - blades.Generator[*blades.Message, error]: 流式响应生成器
func (m *Logging) Handle(ctx context.Context, invocation *blades.Invocation) blades.Generator[*blades.Message, error] {
	return func(yield func(*blades.Message, error) bool) {
		// 记录请求开始时间
		start := time.Now()

		// 从上下文中提取 AgentContext
		// blades.FromAgentContext 返回 (AgentContext, ok)
		// 这允许中间件访问 Agent 的元数据（如名称、描述等）
		agent, ok := blades.FromAgentContext(ctx)
		if !ok {
			// 如果没有找到 AgentContext，直接返回错误
			yield(nil, blades.ErrNoAgentContext)
			return
		}

		// 调用下一个 Handler（被包装的 Handler）
		// 这会将请求传递给中间件链中的下一环
		streaming := m.next.Handle(ctx, invocation)

		// 遍历流式响应
		// Generator 是一个迭代器，每次迭代返回一个 (Message, error)
		for msg, err := range streaming {
			if err != nil {
				// 如果发生错误，调用 onError 回调记录错误日志
				m.onError(start, agent, invocation, err)
			} else {
				// 如果成功，调用 onSuccess 回调记录成功日志
				m.onSuccess(start, agent, invocation, msg)
			}

			// yield 将消息传递给上层调用者
			// 如果 yield 返回 false，表示调用者希望停止接收消息，中断循环
			if !yield(msg, err) {
				break
			}
		}
	}
}

// 扩展提示：
// 1. 结构化日志（使用 JSON 格式）：
//    import "encoding/json"
//    type LogEntry struct {
//        Agent     string `json:"agent"`
//        Prompt    string `json:"prompt"`
//        Duration  string `json:"duration_ms"`
//        Status    string `json:"status"`
//    }
//    // 然后使用 json.Marshal 输出结构化日志
//
// 2. 添加日志级别：
//    func (m *Logging) setLevel(level string) { ... }
//    // 支持 DEBUG、INFO、WARN、ERROR 等级别
//
// 3. 集成日志库：
//    可以使用 zap、logrus 等流行日志库替代标准 log
//    import "go.uber.org/zap"
//    logger, _ := zap.NewProduction()
//    logger.Info("agent executed", zap.String("agent", agent.Name()))
//
// 4. 采样日志（减少日志量）：
//    // 只记录 10% 的请求
//    if rand.Intn(100) < 10 {
//        m.onSuccess(...)
//    }
