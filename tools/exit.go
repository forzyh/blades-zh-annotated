package tools

import (
	"context"
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
)

// ============================================================================
// ExitTool：退出控制流工具
// ============================================================================

// ActionLoopExit 是 ExitTool 在 ToolContext 上设置的动作键名。
//
// 【是什么】
// 这是一个常量，定义为 "loop_exit"，用于在工具执行过程中传递"退出循环"的信号。
// 当工具被调用时，它会在 ToolContext 的 Actions 映射中设置这个键。
//
// 【为什么】
// 在 blades 框架中，Agent 可能会以循环模式运行（LoopAgent），不断执行工具直到完成任务。
// ExitTool 被调用时，会设置这个键，告知外层循环："应该停止了"。
//
// 【值类型】
// ActionLoopExit 对应的值是一个 bool，表示是否应该升级（escalate）到外层处理器。
//   - false: 正常退出，任务完成
//   - true:  升级处理，当前层无法处理，需要外层介入
//
// 【使用方式】
// 通常不需要直接使用这个常量，LoopAgent 会检查消息的 Actions 来判断是否退出：
//
//	if action, ok := msg.Actions[tools.ActionLoopExit]; ok {
//	    if escalate, _ := action.(bool); escalate {
//	        // 升级到外层处理器
//	    }
//	    break // 退出循环
//	}
const ActionLoopExit = "loop_exit"

// ExitInput 是 ExitTool 的输入参数结构体。
//
// 【是什么】
// ExitInput 定义了调用 ExitTool 时需要传递的参数格式。
// 当 LLM 决定退出循环时，它会生成一个符合此结构的 JSON 对象。
//
// 【字段说明】
//   - Reason: 退出原因
//     类型：string
//     用途：记录为什么要退出，便于调试、日志记录和向用户解释
//     示例："任务已完成，所有目标已达成"
//
//   - Escalate: 是否升级处理
//     类型：bool
//     用途：指示是否需要外层处理器介入
//     值含义：
//       - false (默认): 正常退出，当前 Agent 可以处理
//       - true: 升级退出，需要外层 Agent 或处理器处理
//     使用场景：当子 Agent 遇到超出其能力范围的情况时，可以设置 escalate=true
//
// 【JSON Schema 注解】
// 结构体 tag 中的 jsonschema 注解用于生成 JSON Schema：
//   - json:"reason": JSON 字段名
//   - jsonschema:"...": 字段描述，LLM 会看到这个描述来决定何时使用
//
// 【使用示例】
// LLM 生成的调用参数可能如下：
//
//	{
//	    "reason": "已经收集了足够的信息，可以生成报告了",
//	    "escalate": false
//	}
type ExitInput struct {
	Reason   string `json:"reason"             jsonschema:"Reason for exiting the loop."`
	Escalate bool   `json:"escalate,omitempty" jsonschema:"If true, escalate to the outer handler instead of completing normally."`
}

// ExitTool 是一个特殊的控制流工具，用于通知 enclosing loop（外层循环）停止执行。
//
// 【是什么】
// ExitTool 不是用来执行具体业务逻辑的工具，而是一个"控制流"工具。
// 它的作用类似于编程中的 break 或 return 语句，用于提前结束 Agent 的执行循环。
//
// 【为什么需要 ExitTool】
// 1. 显式退出：让 LLM 能够主动决定何时完成任务，而不是盲目继续执行
// 2. Escalation 机制：当遇到无法处理的情况时，可以请求外层处理器介入
// 3. 统一退出方式：提供标准化的退出机制，便于框架统一管理
// 4. 可观察性：通过 ToolContext 传递退出信号，外层可以观察到退出原因
//
// 【使用场景】
//   - 任务已完成：LLM 判断已经收集了足够的信息或完成了所有步骤
//   - 遇到错误：遇到无法恢复的错误，需要提前终止
//   - 满足条件：某个条件满足，无需继续执行
//   - 需要升级：当前 Agent 无法处理，需要外层 Agent 介入
//
// 【注册方式】
// 通过 blades.WithTools 注册到子 Agent 上：
//
//	agent := blades.NewAgent(
//	    blades.WithTools(tools.NewExitTool()),
//	)
//
// 【调用流程】
// 1. LLM 决定退出，调用 exit 工具
// 2. ExitTool.Handle 被调用，解析输入参数
// 3. 从上下文获取 ToolContext
// 4. 在 ToolContext 上设置 ActionLoopExit 动作
// 5. 返回成功响应
// 6. Agent 运行时将动作附加到消息上
// 7. LoopAgent 检查消息的 Actions，发现退出信号
// 8. LoopAgent 退出执行循环
//
// 【注意事项】
//   - 如果在没有 ToolContext 的环境中调用（即不在循环中），此调用会是 no-op（无操作）
//   - ExitTool 不执行任何业务逻辑，只负责传递退出信号
//   - Escalate 为 true 时，外层处理器需要决定如何处理
type ExitTool struct {
	// inputSchema 是 ExitInput 的 JSON Schema，由构造函数自动生成
	// 用于告诉 LLM 调用 exit 工具时需要传递什么参数
	inputSchema *jsonschema.Schema
}

// NewExitTool 创建并返回一个可用的 ExitTool 实例。
//
// 【返回值】
// *ExitTool: 初始化好的 ExitTool 实例
//
// 【使用示例】
//
//	exitTool := tools.NewExitTool()
//	// 注册到 Agent
//	agent := blades.NewAgent(blades.WithTools(exitTool))
//
// 【底层实现】
// 使用 jsonschema.For 根据 ExitInput 结构体自动生成 JSON Schema。
// jsonschema.For 会分析结构体的字段和 tag，生成对应的 JSON Schema。
// 这样 LLM 就知道调用 exit 工具时需要传递包含 reason 和 escalate 字段的对象。
//
// 【为什么不处理错误】
// jsonschema.For[ExitInput](nil) 几乎不会失败，因为 ExitInput 是简单的结构体。
// 如果真失败，说明有更严重的问题，panic 是合理的。
func NewExitTool() *ExitTool {
	// 根据 ExitInput 类型生成 JSON Schema
	// 第二个参数 nil 表示使用默认的 Schema 生成配置
	schema, _ := jsonschema.For[ExitInput](nil)
	return &ExitTool{inputSchema: schema}
}

// Name 返回工具名称 "exit"。
//
// 【用途】
// 名称是工具的唯一标识符，LLM 通过名称来调用工具。
// 在 Agent 的工具列表中，这个工具会以 "exit" 的名称出现。
func (t *ExitTool) Name() string { return "exit" }

// InputSchema 返回 ExitTool 的输入 JSON Schema。
//
// 【用途】
// LLM 会根据此 Schema 生成包含 reason 和 escalate 字段的 JSON 对象。
// Schema 会告诉 LLM：
//   - 需要什么类型的参数（object）
//   - 需要哪些字段（reason, escalate）
//   - 每个字段的类型和含义
func (t *ExitTool) InputSchema() *jsonschema.Schema { return t.inputSchema }

// OutputSchema 返回 nil，因为 ExitTool 不返回结构化数据。
//
// 【为什么返回 nil】
// ExitTool 的返回值只是一个简单的成功确认 `{"ok":true}`，
// 不需要复杂的结构，所以不定义 OutputSchema。
//
// 【调用者如何处理】
// 调用者通常不关心 ExitTool 的返回值，只关心它设置的 Action。
func (t *ExitTool) OutputSchema() *jsonschema.Schema { return nil }

// Description 返回工具的描述，帮助 LLM 理解何时使用此工具。
//
// 【描述内容】
// "Signal that the current loop should stop." - 告诉当前循环应该停止
// "Call this when the task is complete or when escalation is required." - 任务完成或需要升级时调用
//
// 【为什么描述重要】
// LLM 会根据 Description 决定何时调用此工具。
// 清晰的描述可以提高 LLM 的决策准确性。
//
// 【编写建议】
//   - 用简洁的英语说明工具的用途
//   - 说明使用时机
//   - 避免技术细节，LLM 更关心"何时用"而不是"怎么用"
func (t *ExitTool) Description() string {
	return "Signal that the current loop should stop. Call this when the task is complete or when escalation is required."
}

// Handle 是 ExitTool 的执行逻辑，由 Agent 运行时在 LLM 调用此工具时执行。
//
// 【是什么】
// Handle 方法实现了 Tool 接口，是工具的实际执行入口。
// 当 LLM 决定调用 exit 工具时，Agent 会调用这个方法。
//
// 【执行流程】
// 1. 将输入的 JSON 字符串反序列化为 ExitInput 结构体
//    - 如果 JSON 格式错误，返回错误
// 2. 从上下文中获取 ToolContext
//    - 使用 FromContext 函数
//    - 如果不存在 ToolContext，跳过设置动作（no-op）
// 3. 在 ToolContext 上设置 ActionLoopExit 动作
//    - 键：ActionLoopExit ("loop_exit")
//    - 值：req.Escalate (bool)
// 4. 返回成功响应 `{"ok":true}`
//
// 【参数说明】
//   - ctx: 上下文
//     包含调用信息和可能的 ToolContext
//     ToolContext 用于传递动作信号给外层循环
//
//   - input: JSON 字符串
//     LLM 生成的工具调用参数
//     示例：`{"reason": "任务完成", "escalate": false}`
//
// 【返回值】
//   - string: JSON 字符串，固定返回 `{"ok":true}`
//     表示退出信号已成功记录
//
//   - error: 执行错误
//     可能的错误来源：
//     - JSON 解析失败：LLM 生成的参数格式错误
//
// 【关键点】
//   - FromContext 返回 (ToolContext, bool)，只有在上下文中存在时才设置动作
//   - 使用 tc.SetAction 而不是直接修改，因为 ToolContext 需要保证并发安全
//   - 即使没有 ToolContext，也不会返回错误，因为这不是工具本身的错误
//
// 【并发安全】
// ToolContext.SetAction 被设计为并发安全的（注释中说明"Safe for concurrent use"），
// 多个 goroutine 同时调用 SetAction 不会导致数据竞争。
//
// 【示例调用】
// 假设 LLM 生成以下调用：
//
//	{
//	    "name": "exit",
//	    "arguments": {"reason": "信息收集完成", "escalate": false}
//	}
//
// Handle 方法会被这样调用：
//
//	tool.Handle(ctx, `{"reason": "信息收集完成", "escalate": false}`)
//
// 执行后，ToolContext.Actions["loop_exit"] = false
func (t *ExitTool) Handle(ctx context.Context, input string) (string, error) {
	var req ExitInput
	// 将 JSON 字符串解析为 ExitInput 结构体
	// 如果输入格式不匹配 ExitInput，这里会返回错误
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", err
	}
	// 尝试从上下文中获取 ToolContext
	// FromContext 返回 (ToolContext, bool)
	//   - ok=true: 上下文中存在 ToolContext，可以设置动作
	//   - ok=false: 上下文中没有 ToolContext（不在循环中），设置动作会被跳过
	if tc, ok := FromContext(ctx); ok {
		// 设置退出动作
		// 键：ActionLoopExit ("loop_exit")
		// 值：req.Escalate (bool)，表示是否升级到外层处理器
		//
		// 外层循环执行器会检查这个值来决定：
		//   - false: 正常结束，任务完成
		//   - true:  将控制权交还给外层处理器，继续处理
		tc.SetAction(ActionLoopExit, req.Escalate)
	}
	// 返回成功响应
	// 返回值会被 Agent 记录，但通常不被使用
	return `{"ok":true}`, nil
}
