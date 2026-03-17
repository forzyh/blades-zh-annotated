package flow

import (
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/internal/deep"
	"github.com/go-kratos/blades/tools"
)

// DeepConfig 是深度代理（DeepAgent）的配置结构体
// 用于定义高级深度代理模式所需的参数
//
// 字段说明：
// - Name: 代理的名称，用于标识和日志记录
// - Model: 模型提供者，用于 AI 推理的底层模型
//   深度代理使用强大的 AI 模型来处理复杂任务和决策
// - Description: 代理的描述信息，说明该代理的用途和功能
// - Instruction: 代理的指令文本，定义代理的行为准则和任务目标
//   指令会与系统自动生成的指令（如 BaseAgentPrompt）组合使用
// - Tools: 工具列表，代理可以调用的自定义工具
//   工具扩展了代理的能力，使其能够执行特定操作
// - SubAgents: 子代理列表，深度代理可以将任务委托给这些子代理
//   子代理通常是专业化的代理，负责特定领域的任务
// - MaxIterations: 最大迭代次数，限制代理的执行轮数
//   防止代理陷入无限循环，保护系统资源
// - WithoutGeneralPurposeAgent: 是否禁用通用目的代理
//   设置为 true 时，代理不会创建通用的任务处理代理
//   适用于只需要委托给专门子代理的场景
// - Middlewares: 中间件列表，用于在代理执行前后进行拦截和处理
//   可用于日志记录、监控、权限检查等横切关注点
//
// 使用场景：
// - 需要处理复杂、多步骤任务的场景
// - 需要维护任务列表（todos）并跟踪进度的场景
// - 需要将任务分解并委托给专业子代理的场景
// - 需要层次化任务管理和协作的场景
//
// 配置示例：
// config := flow.DeepConfig{
//     Name: "项目助手",
//     Model: openaiProvider,
//     Description: "协助完成项目开发任务",
//     Instruction: "你是一个专业的项目开发助手...",
//     Tools: []tools.Tool{fileTool, searchTool},
//     SubAgents: []blades.Agent{codingAgent, testingAgent},
//     MaxIterations: 10,
//     Middlewares: []blades.Middleware{loggingMiddleware},
// }
type DeepConfig struct {
	Name                       string
	Model                      blades.ModelProvider
	Description                string
	Instruction                string
	Tools                      []tools.Tool
	SubAgents                  []blades.Agent
	MaxIterations              int
	WithoutGeneralPurposeAgent bool
	Middlewares                []blades.Middleware
}

// NewDeepAgent 是深度代理的构造函数
// 它构建并返回一个"深度代理"实例
//
// 什么是深度代理（DeepAgent）：
// 深度代理是一种高级代理，具备以下能力：
// 1. 管理复杂任务：能够处理多步骤、多层次的任务
// 2. 维护任务列表（todos）：可以创建和跟踪待办事项
// 3. 层次化委托：可以将任务分解并委托给专业子代理
// 4. 自主决策：能够决定是自己执行任务还是委托给子代理
//
// 与普通代理的区别：
// - 普通代理：直接执行单一任务
// - 深度代理：可以管理任务列表、分解任务、委托给子代理
//
// 参数：config - DeepConfig 配置结构体
// 返回：blades.Agent 接口和可能的错误
//
// 执行流程详解：
// 1. 初始化 TaskToolConfig 配置
//    - 复制用户配置到任务工具配置
//    - 将用户指令与 BaseAgentPrompt（基础代理提示）组合
// 2. 创建 Todos 工具（WriteTodos）
//    - 使代理能够创建和管理待办事项列表
//    - 将工具和相关指令添加到配置中
// 3. 创建 Task 工具（条件性）
//    - 如果没有禁用通用代理或有子代理，则创建
//    - Task 工具使代理能够委托任务给子代理
// 4. 使用 blades.NewAgent 创建最终代理实例
//    - 组合所有配置选项
//    - 将所有指令用双换行符连接
//
// 错误处理：
// - NewWriteTodosTool 失败时返回错误
// - NewTaskTool 失败时返回错误
// - blades.NewAgent 失败时返回错误
//
// 使用示例：
// agent, err := flow.NewDeepAgent(flow.DeepConfig{
//     Name: "代码审查助手",
//     Model: openaiProvider,
//     Description: "帮助进行代码审查和改进",
//     Instruction: "你是一个专业的代码审查员...",
//     Tools: []tools.Tool{gitTool, lintTool},
//     SubAgents: []blades.Agent{styleChecker, bugDetector},
//     MaxIterations: 5,
// })
// if err != nil {
//     log.Fatal(err)
// }
func NewDeepAgent(config DeepConfig) (blades.Agent, error) {
	// 初始化任务工具配置
	// TaskToolConfig 是 deep 包内部用于配置任务工具的结构
	tc := deep.TaskToolConfig{
		Model:                      config.Model,
		Instructions:               []string{config.Instruction, deep.BaseAgentPrompt},
		Tools:                      config.Tools,
		SubAgents:                  config.SubAgents,
		MaxIterations:              config.MaxIterations,
		WithoutGeneralPurposeAgent: config.WithoutGeneralPurposeAgent,
	}
	// 创建 WriteTodos 工具
	// 这个工具允许代理创建和管理待办事项列表
	// 是实现"深度"功能的核心组件之一
	todosTool, todosInstruction, err := deep.NewWriteTodosTool()
	if err != nil {
		return nil, err
	}
	// 将 Todos 工具和相关指令添加到配置中
	tc.Tools = append(tc.Tools, todosTool)
	tc.Instructions = append(tc.Instructions, todosInstruction)
	// 条件性创建 Task 工具
	// 当满足以下任一条件时创建 Task 工具：
	// 1. 没有禁用通用目的代理（WithoutGeneralPurposeAgent=false）
	// 2. 有子代理可以委托（len(SubAgents)>0）
	//
	// Task 工具使代理能够：
	// - 将任务委托给子代理
	// - 如果没有子代理，使用通用目的代理处理任务
	if !tc.WithoutGeneralPurposeAgent || len(tc.SubAgents) > 0 {
		taskTool, taskInstruction, err := deep.NewTaskTool(tc)
		if err != nil {
			return nil, err
		}
		tc.Tools = append(tc.Tools, taskTool)
		tc.Instructions = append(tc.Instructions, taskInstruction)
	}
	// 使用 blades.NewAgent 创建最终的代理实例
	// 将所有配置选项传递给基础代理构造函数
	return blades.NewAgent(config.Name,
		blades.WithModel(config.Model),                          // 设置 AI 模型
		blades.WithDescription(config.Description),              // 设置描述
		blades.WithInstruction(strings.Join(tc.Instructions, "\n\n")), // 组合所有指令
		blades.WithTools(tc.Tools...),                           // 设置所有工具
		blades.WithMaxIterations(config.MaxIterations),          // 设置最大迭代次数
		blades.WithMiddleware(config.Middlewares...),            // 设置中间件
	)
}
