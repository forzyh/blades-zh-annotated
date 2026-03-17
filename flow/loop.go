package flow

import (
	"context"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"
)

// LoopState 捕获循环条件函数（LoopCondition）可观察的状态信息
// 它在每次迭代完成后被传递给条件函数，供其决策是否继续下一轮迭代
//
// 字段说明：
// - Iteration: 刚刚完成的迭代的索引（从 0 开始计数）
//   例如：Iteration=0 表示第一轮迭代完成，Iteration=1 表示第二轮完成
// - Input: 传递给 LoopAgent 的原始输入消息
//   条件函数可以参考原始输入来判断任务是否已完成
// - Output: 当前迭代中产生的最后一条消息
//   条件函数可以检查最新输出来判断是否满足终止条件
//
// 使用示例：
// condition := func(ctx context.Context, state LoopState) (bool, error) {
//     // 检查输出中是否包含"完成"标记
//     if strings.Contains(state.Output.Text(), "任务完成") {
//         return false, nil // 终止循环
//     }
//     return true, nil // 继续下一轮迭代
// }
type LoopState struct {
	// Iteration 是刚刚完成的迭代的索引（从 0 开始）
	Iteration int
	// Input 是传递给 LoopAgent 的原始输入消息
	Input *blades.Message
	// Output 是当前迭代中产生的最后一条消息
	Output *blades.Message
}

// LoopCondition 是循环条件函数类型
// 它在每次完整迭代结束后被调用一次，用于判断是否应该继续下一轮迭代
//
// 参数：
// - ctx: 上下文，用于控制生命周期和取消信号
// - state: LoopState 结构，包含当前迭代的状态信息
//
// 返回值：
// - bool: true 表示继续下一轮迭代，false 表示正常终止循环
// - error: 非 nil 错误表示异常终止
//   特别地，可以返回 blades.ErrLoopEscalated 来触发循环升级错误
//
// 优先级说明：
// LoopCondition 的优先级高于 ExitTool 信号
// 即使子代理请求退出，如果 Condition 返回 true，循环仍会继续
//
// 使用示例：
// // 示例 1：基于最大迭代次数终止
// condition := func(ctx context.Context, state LoopState) (bool, error) {
//     return state.Iteration < 5, nil
// }
//
// // 示例 2：基于输出内容判断是否完成
// condition := func(ctx context.Context, state LoopState) (bool, error) {
//     if strings.Contains(state.Output.Text(), "已完成") {
//         return false, nil
//     }
//     return true, nil
// }
//
// // 示例 3：遇到特定条件时触发升级错误
// condition := func(ctx context.Context, state LoopState) (bool, error) {
//     if state.Iteration >= 3 && !taskCompleted {
//         return false, blades.ErrLoopEscalated // 触发升级错误
//     }
//     return !taskCompleted, nil
// }
type LoopCondition func(ctx context.Context, state LoopState) (bool, error)

// LoopConfig 是循环代理（LoopAgent）的配置结构体
// 用于定义循环执行模式所需的参数
//
// 字段说明：
// - Name: 代理的名称，用于标识和日志记录
// - Description: 代理的描述信息，说明该代理的用途和功能
// - MaxIterations: 最大迭代次数，防止无限循环
//   如果设置为 0 或负数，NewLoopAgent 会自动设置为默认值 10
// - Condition: 循环条件函数，每次迭代后执行，用于判断是否继续
//   该条件优先级高于 ExitTool 信号
// - SubAgents: 子代理列表，这些子代理在每次迭代中按顺序执行
//
// 使用场景：
// - 需要反复尝试直到任务完成的场景
// - 需要逐步 refinement 的任务（如代码迭代优化）
// - 需要多次调用工具才能解决的问题
//
// 配置示例：
// config := flow.LoopConfig{
//     Name: "迭代优化器",
//     Description: "反复优化代码直到满意",
//     MaxIterations: 5,
//     Condition: myCondition,
//     SubAgents: []blades.Agent{optimizer, validator},
// }
type LoopConfig struct {
	Name          string
	Description   string
	MaxIterations int
	// Condition 在每次迭代后被调用，优先级高于 ExitTool 信号
	Condition LoopCondition
	SubAgents []blades.Agent
}

// LoopAgent 是循环代理结构体
// 它负责将多个子代理在循环中反复执行，直到满足终止条件
//
// 核心特性：
// 1. 每次迭代按顺序执行所有子代理
// 2. 支持两种终止机制：ExitTool 信号和 Condition 条件函数
// 3. 有最大迭代次数限制，防止无限循环
// 4. 支持循环升级（escalation）机制，用于处理无法完成的循环
//
// 终止条件（满足任一即终止）：
// - 达到最大迭代次数（MaxIterations）
// - Condition 函数返回 false
// - 子代理通过 ExitTool 设置退出信号且 escalated=false
//
// 错误处理：
// - 子代理执行错误：立即终止并返回错误
// - Condition 函数错误：立即终止并返回错误
// - 循环升级（escalated=true）：返回 blades.ErrLoopEscalated 错误
type LoopAgent struct {
	config LoopConfig
}

// NewLoopAgent 是循环代理的构造函数
//
// 参数：config - LoopConfig 配置结构体
// 返回：blades.Agent 接口，实现标准化的代理运行协议
//
// 特殊处理：
// - 如果 MaxIterations <= 0，自动设置为默认值 10
// 这是为了防止无限循环，确保循环有合理的上限
//
// 使用示例：
// loopAgent := flow.NewLoopAgent(flow.LoopConfig{
//     Name: "代码审查循环",
//     Description: "反复审查和修复代码问题",
//     MaxIterations: 3,
//     Condition: checkCondition,
//     SubAgents: []blades.Agent{reviewer, fixer},
// })
func NewLoopAgent(config LoopConfig) blades.Agent {
	// 防御性编程：确保最大迭代次数有效
	// 防止配置错误导致无限循环
	if config.MaxIterations <= 0 {
		config.MaxIterations = 10
	}
	return &LoopAgent{config: config}
}

// Name 返回代理的名称
// 实现 blades.Agent 接口的方法
func (a *LoopAgent) Name() string        { return a.config.Name }

// Description 返回代理的描述信息
// 实现 blades.Agent 接口的方法
func (a *LoopAgent) Description() string { return a.config.Description }

// Run 是循环代理的核心执行方法
// 它在循环中运行所有子代理，直到满足终止条件
//
// 参数：
// - ctx: 上下文，用于控制生命周期和取消信号
// - input: 调用参数，包含输入消息和执行上下文
//
// 返回：Generator[*blades.Message, error] - 一个生成器函数
// 生成器会逐步产出每个子代理产生的 Message 或 error
//
// 执行流程详解：
// 1. 初始化 LoopState 用于跟踪迭代状态
// 2. 外层 for 循环：控制迭代次数，最多执行 MaxIterations 次
// 3. 内层 for 循环：按顺序执行所有子代理
// 4. 每次子代理输出后，检查是否有 ActionLoopExit 信号
// 5. 每轮迭代完成后，如果配置了 Condition 则调用它判断是否继续
// 6. 根据退出信号和 escalated 标志决定终止方式
//
// 退出机制（三种方式）：
// 1. ExitTool 信号：子代理通过 message.Actions[tools.ActionLoopExit] 设置退出标志
//    - escalated=false：正常退出，不返回错误
//    - escalated=true：升级退出，返回 blades.ErrLoopEscalated 错误
// 2. Condition 函数：返回 false 时正常终止，返回错误时异常终止
// 3. 最大迭代次数：达到 MaxIterations 后自动终止
//
// 优先级说明：
// - Condition 条件的优先级高于 ExitTool 信号
// - 即使设置了退出信号，如果 Condition 返回 true，循环仍会继续
//
// 上下文管理：
// - 跨迭代的上下文管理由 Runner 配置的 ContextManager 处理
// - 通过 blades.WithContextManager 选项配置
//
// 使用示例：
// for message, err := range loopAgent.Run(ctx, invocation) {
//     if err == blades.ErrLoopEscalated {
//         log.Println("循环升级，需要人工介入")
//         break
//     }
//     if err != nil {
//         log.Printf("执行错误：%v", err)
//         break
//     }
//     processMessage(message)
// }
func (a *LoopAgent) Run(ctx context.Context, input *blades.Invocation) blades.Generator[*blades.Message, error] {
	// 返回一个生成器函数，这是 Go 中实现协程迭代器的模式
	// yield 函数由调用者提供，用于接收每个输出结果
	return func(yield func(*blades.Message, error) bool) {
		// 初始化循环状态
		state := LoopState{}
		// 外层循环：控制迭代次数
		// state.Iteration 从 0 开始递增，直到达到最大迭代次数
		for state.Iteration = 0; state.Iteration < a.config.MaxIterations; state.Iteration++ {
			// exitRequested 标记是否有子代理请求退出
			// escalated 标记退出是否需要升级（返回错误）
			exitRequested := false
			escalated := false
			// 内层循环：按顺序执行当前迭代中的所有子代理
			for _, agent := range a.config.SubAgents {
				var (
					err        error
					message    *blades.Message
					invocation = input.Clone() // 为每个子代理创建独立的调用副本
				)
				// 运行子代理，接收其所有输出消息
				for message, err = range agent.Run(ctx, invocation) {
					if err != nil {
						// 子代理执行出错，立即将错误传递给调用者并终止整个循环
						yield(nil, err)
						return
					}
					// 将消息传递给调用者
					// 如果 yield 返回 false，说明调用者希望提前终止
					if !yield(message, nil) {
						return
					}
					// 检查消息中是否包含 ActionLoopExit 信号
					// 这是子代理通过 ExitTool 设置的退出标志
					if exit, ok := message.Actions[tools.ActionLoopExit]; ok {
						// 检查退出标志的类型是否为 bool
						// bool 值表示是否需要升级（escalated）
						if exitEscalated, ok := exit.(bool); ok {
							exitRequested = true
							escalated = exitEscalated
						}
					}
				}
				// 更新循环状态，供 Condition 函数使用
				state.Input = input.Message   // 原始输入消息
				state.Output = message        // 当前子代理的最后输出
			}
			// 一轮迭代完成，检查是否有 Condition 条件函数
			if a.config.Condition != nil {
				// 调用条件函数，判断是否继续下一轮迭代
				shouldContinue, err := a.config.Condition(ctx, state)
				if err != nil {
					// Condition 函数返回错误，立即终止并传递错误
					yield(nil, err)
					return
				}
				// 如果条件函数返回 false，正常终止循环
				if !shouldContinue {
					return
				}
				// 条件函数返回 true，继续下一轮迭代（跳过下面的 ExitTool 检查）
				continue
			}
			// Condition 条件未配置时，检查 ExitTool 退出信号
			if exitRequested {
				if escalated {
					// escalated=true 表示需要升级，返回循环升级错误
					yield(nil, blades.ErrLoopEscalated)
					return
				}
				// escalated=false 表示正常退出，不返回错误
				return
			}
			// 既没有 Condition 也没有 ExitTool 信号，继续下一轮迭代
		}
		// 达到最大迭代次数，循环自然终止
	}
}
