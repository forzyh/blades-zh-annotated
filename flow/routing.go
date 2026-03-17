package flow

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/internal/handoff"
)

// RoutingConfig 是路由代理（RoutingAgent）的配置结构体
// 用于定义路由分发模式所需的参数
//
// 字段说明：
// - Name: 路由代理的名称，用于标识和日志记录
// - Description: 路由代理的描述信息，说明该代理的用途和功能
// - Model: 模型提供者，用于路由决策的 AI 模型
//   路由代理使用 AI 模型来理解用户输入并决定路由到哪个子代理
// - SubAgents: 子代理列表，这些子代理是路由的潜在目标
//   路由代理会根据任务类型将请求分发给合适的子代理
// - Middlewares: 中间件列表，用于在代理执行前后进行拦截和处理
//   可用于日志记录、监控、权限检查等横切关注点
//
// 使用场景：
// - 需要根据用户请求类型分发到不同专业代理的场景
// - 构建多代理协作系统，由路由代理担任"调度员"角色
// - 实现智能任务分配，提高整体处理效率
//
// 配置示例：
// config := flow.RoutingConfig{
//     Name: "任务调度器",
//     Description: "根据任务类型路由到专业代理",
//     Model: myModelProvider,
//     SubAgents: []blades.Agent{codingAgent, testingAgent, docsAgent},
//     Middlewares: []blades.Middleware{loggingMiddleware},
// }
type RoutingConfig struct {
	Name        string
	Description string
	Model       blades.ModelProvider
	SubAgents   []blades.Agent
	Middlewares []blades.Middleware
}

// RoutingAgent 是路由代理结构体
// 它负责接收用户请求，通过 AI 模型分析后路由到合适的子代理执行
//
// 核心特性：
// 1. 嵌入 blades.Agent 接口，具有标准代理的所有功能
// 2. 内部维护一个 target 映射，将代理名称映射到代理实例
// 3. 使用 handoff 机制实现代理间的任务移交
//
// 路由流程：
// 1. 根代理（rootAgent）首先执行，分析用户输入
// 2. 根代理决定是否需要路由到其他子代理
// 3. 如果需要路由，通过 handoff 动作指定目标代理
// 4. 路由代理查找目标代理并执行它
// 5. 返回目标代理的执行结果
//
// 数据结构：
// - Agent: 嵌入的根代理，负责任务分析和路由决策
// - targets: 代理名称到代理实例的映射表，用于快速查找目标代理
type RoutingAgent struct {
	blades.Agent
	targets map[string]blades.Agent
}

// NewRoutingAgent 是路由代理的构造函数
//
// 参数：config - RoutingConfig 配置结构体
// 返回：blades.Agent 接口，实现标准化的代理运行协议
//
// 执行流程：
// 1. 调用 handoff.BuildInstruction 构建路由指令
//    该指令告诉 AI 模型如何根据任务类型选择合适的子代理
// 2. 创建根代理（rootAgent）
//    根代理配置了 handoff 工具，可以输出路由决策
// 3. 构建 targets 映射表
//    将子代理名称（去除首尾空格）映射到代理实例
// 4. 返回 RoutingAgent 实例
//
// 错误处理：
// - handoff.BuildInstruction 失败时返回错误
// - blades.NewAgent 失败时返回错误
//
// 使用示例：
// routingAgent, err := flow.NewRoutingAgent(flow.RoutingConfig{
//     Name: "智能路由器",
//     Description: "根据任务类型路由到专业代理",
//     Model: openaiProvider,
//     SubAgents: []blades.Agent{codingAgent, testingAgent},
// })
// if err != nil {
//     log.Fatal(err)
// }
func NewRoutingAgent(config RoutingConfig) (blades.Agent, error) {
	// 构建路由指令
	// handoff.BuildInstruction 会根据子代理列表生成指令
	// 指令内容包含每个子代理的名称、描述等信息
	// 用于指导 AI 模型理解何时路由到哪个子代理
	instruction, err := handoff.BuildInstruction(config.SubAgents)
	if err != nil {
		return nil, err
	}
	// 创建根代理（路由决策代理）
	// 根代理不直接执行任务，而是分析输入并决定路由到哪个子代理
	rootAgent, err := blades.NewAgent(
		config.Name,
		blades.WithModel(config.Model),
		blades.WithDescription(config.Description),
		blades.WithInstruction(instruction),
		blades.WithTools(handoff.NewHandoffTool()), // 配置 handoff 工具，用于输出路由决策
		blades.WithMiddleware(config.Middlewares...),
	)
	if err != nil {
		return nil, err
	}
	// 构建 targets 映射表
	// key: 代理名称（去除首尾空格）
	// value: 代理实例
	// 使用映射表可以在 O(1) 时间内查找目标代理
	targets := make(map[string]blades.Agent)
	for _, agent := range config.SubAgents {
		targets[strings.TrimSpace(agent.Name())] = agent
	}
	// 返回 RoutingAgent 实例
	// 嵌入根代理作为父类，同时持有 targets 映射表
	return &RoutingAgent{
		Agent:   rootAgent,
		targets: targets,
	}, nil
}

// Run 是路由代理的核心执行方法
// 它先执行根代理进行路由决策，然后执行选中的目标代理
//
// 参数：
// - ctx: 上下文，用于控制生命周期和取消信号
// - invocation: 调用参数，包含输入消息和执行上下文
//
// 返回：Generator[*blades.Message, error] - 一个生成器函数
// 生成器会逐步产出来自根代理或目标代理的 Message 或 error
//
// 执行流程详解：
// 1. 执行根代理（路由决策代理）
//    - 根代理分析用户输入
//    - 决定是否需要路由到其他子代理
//    - 通过 handoff 动作输出路由决策
// 2. 解析根代理的输出，提取目标代理名称
// 3. 在 targets 映射表中查找目标代理
// 4. 如果找到目标代理，执行它并返回其输出
// 5. 如果未找到目标代理，返回根代理的输出或错误
//
// 边界情况处理：
// - 未找到目标代理且根代理有输出：返回根代理的输出
// - 未找到目标代理且根代理无输出：返回错误
// - 目标代理执行出错：将错误传递给调用者
//
// 使用示例：
// for message, err := range routingAgent.Run(ctx, invocation) {
//     if err != nil {
//         log.Printf("执行错误：%v", err)
//         break
//     }
//     fmt.Println("收到消息:", message.Text())
// }
func (a *RoutingAgent) Run(ctx context.Context, invocation *blades.Invocation) blades.Generator[*blades.Message, error] {
	// 返回一个生成器函数，这是 Go 中实现协程迭代器的模式
	// yield 函数由调用者提供，用于接收每个输出结果
	return func(yield func(*blades.Message, error) bool) {
		var (
			err         error
			targetAgent string // 存储路由决策确定的目标代理名称
			message     *blades.Message
		)
		// 克隆调用参数，确保根代理的执行不影响原始 invocation
		routerInvocation := invocation.Clone()
		// 第一步：执行根代理（路由决策代理）
		// 根代理会分析输入并可能输出 handoff 动作
		for message, err = range a.Agent.Run(ctx, routerInvocation) {
			if err != nil {
				// 根代理执行出错，将错误传递给调用者并终止
				yield(nil, err)
				return
			}
			// 检查消息中是否包含 handoff 动作
			// ActionHandoffToAgent 是 handoff 工具定义的路由动作键
			if target, ok := message.Actions[handoff.ActionHandoffToAgent]; ok {
				// 提取目标代理名称
				targetAgent, _ = target.(string)
				// 找到路由决策后，停止根代理的执行
				break
			}
		}
		// 第二步：在 targets 映射表中查找目标代理
		agent, ok := a.targets[targetAgent]
		if !ok {
			// 未找到目标代理的处理逻辑
			// 情况 1：根代理有有效输出，返回根代理的输出
			if message != nil && message.Text() != "" {
				yield(message, nil)
				return
			}
			// 情况 2：根代理无有效输出，返回错误
			yield(nil, fmt.Errorf("target agent not found: %s", targetAgent))
			return
		}
		// 第三步：执行目标代理
		// 再次克隆调用参数，确保目标代理有独立的上下文
		targetInvocation := invocation.Clone()
		// 运行目标代理，将其所有输出传递给调用者
		for message, err := range agent.Run(ctx, targetInvocation) {
			if !yield(message, err) {
				// 调用者希望提前终止
				return
			}
		}
	}
}
