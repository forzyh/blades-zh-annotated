package flow

import (
	"context"

	"github.com/go-kratos/blades"
)

// SequentialConfig 是顺序代理（SequentialAgent）的配置结构体
// 用于定义顺序执行模式所需的参数
//
// 字段说明：
// - Name: 代理的名称，用于标识和日志记录
// - Description: 代理的描述信息，说明该代理的用途和功能
// - SubAgents: 子代理列表，这些子代理将按顺序依次执行
//
// 使用场景：当任务需要分阶段处理，且每个阶段依赖前一阶段的输出时使用顺序配置
// 例如：数据收集 -> 数据分析 -> 报告生成的流水线处理
type SequentialConfig struct {
	Name        string
	Description string
	SubAgents   []blades.Agent
}

// SequentialAgent 是顺序代理结构体
// 它负责将多个子代理按配置顺序依次执行
//
// 核心特性：
// 1. 子代理按列表顺序逐个执行，前一个完成后才开始下一个
// 2. 每个子代理有独立的调用上下文（invocation.Clone()）
// 3. 任意子代理出错时，整个流程立即终止
// 4. 所有子代理的输出都会被传递给调用者
//
// 数据传递机制：
// - 每个子代理接收原始输入的克隆副本
// - 子代理之间不直接共享数据，但调用者可以基于输出进行后续处理
type SequentialAgent struct {
	config SequentialConfig
}

// NewSequentialAgent 是顺序代理的构造函数
//
// 参数：config - SequentialConfig 配置结构体，包含代理名称、描述和子代理列表
// 返回：blades.Agent 接口，实现标准化的代理运行协议
//
// 使用示例：
// sequentialAgent := flow.NewSequentialAgent(flow.SequentialConfig{
//     Name: "数据处理流水线",
//     Description: "按顺序执行收集、分析、生成报告",
//     SubAgents: []blades.Agent{collector, analyzer, reporter},
// })
func NewSequentialAgent(config SequentialConfig) blades.Agent {
	return &SequentialAgent{
		config: config,
	}
}

// Name 返回代理的名称
// 实现 blades.Agent 接口的方法
func (a *SequentialAgent) Name() string {
	return a.config.Name
}

// Description 返回代理的描述信息
// 实现 blades.Agent 接口的方法
func (a *SequentialAgent) Description() string {
	return a.config.Description
}

// Run 是顺序代理的核心执行方法
// 它按配置顺序依次运行所有子代理，并将它们的输出流式传递给调用者
//
// 参数：
// - ctx: 上下文，用于控制生命周期和取消信号
// - input: 调用参数，包含输入消息和执行上下文
//
// 返回：Generator[*blades.Message, error] - 一个生成器函数
// 生成器会逐步产出每个子代理产生的 Message 或 error
//
// 执行流程详解：
// 1. 外层循环遍历配置中的子代理列表
// 2. 对每个子代理，创建独立的 invocation 副本（Clone()）
// 3. 内层循环运行子代理，接收其所有输出消息
// 4. 遇到错误时立即 yield 错误并终止整个流程
// 5. 遇到调用者取消（yield 返回 false）时也立即终止
//
// 边界情况处理：
// - 空 SubAgents 列表：循环不执行，直接返回，不产生任何输出
// - 子代理错误：立即终止，不再执行后续子代理
// - 调用者提前终止：通过 yield 返回值检测，立即退出
//
// 使用示例：
// for message, err := range sequentialAgent.Run(ctx, invocation) {
//     if err != nil {
//         log.Printf("执行错误：%v", err)
//         break
//     }
//     processMessage(message)
// }
func (a *SequentialAgent) Run(ctx context.Context, input *blades.Invocation) blades.Generator[*blades.Message, error] {
	// 返回一个生成器函数，这是 Go 中实现协程迭代器的模式
	// yield 函数由调用者提供，用于接收每个输出结果
	return func(yield func(*blades.Message, error) bool) {
		// 外层循环：按顺序遍历所有子代理
		for _, agent := range a.config.SubAgents {
			var (
				err        error
				message    *blades.Message
				invocation = input.Clone() // 为每个子代理创建独立的调用副本
			)
			// 内层循环：运行当前子代理，接收其所有输出
			// 子代理可能产生多个消息（流式输出）
			for message, err = range agent.Run(ctx, invocation) {
				if err != nil {
					// 子代理执行出错，将错误传递给调用者并终止整个顺序流程
					yield(nil, err)
					return
				}
				// 将消息传递给调用者
				// 如果 yield 返回 false，说明调用者希望提前终止
				if !yield(message, nil) {
					return
				}
			}
			// 当前子代理执行完成，继续执行下一个子代理
		}
		// 所有子代理执行完成
	}
}
