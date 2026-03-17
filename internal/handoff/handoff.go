// Package handoff 提供了 Agent 交接（handoff）功能的实现。
// 交接是指一个 Agent 将任务转移给另一个更适合的 Agent 处理。
// 本包包含：
// - handoff 工具：用于将请求转移给其他 Agent
// - 指令构建函数：生成包含可用 Agent 列表的指令
//
// 使用场景：
// 当用户请求超出当前 Agent 的能力范围，或有其他更专业的 Agent 可以处理时，
// 可以使用交接功能将任务转移给更合适的 Agent。
package handoff

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-kratos/blades/tools"
	"github.com/google/jsonschema-go/jsonschema"
)

var (
	// ActionHandoffToAgent 是交接给子代理的动作名称。
	// 该常量用于在工具上下文中设置交接目标代理的名称。
	// 当工具执行时，会将此名称设置到上下文中，供外部处理交接逻辑。
	ActionHandoffToAgent = "handoff_to_agent"
)

// handoffTool 是交接工具的具体实现结构体。
// 它是一个空结构体，因为工具不需要内部状态。
//
// 工具功能：
// - 允许当前 Agent 将任务转移给另一个更专业的 Agent
// - 通过工具调用的方式触发交接
type handoffTool struct{}

// NewHandoffTool 创建并返回一个新的交接工具实例。
//
// 返回值：
// - tools.Tool: 交接工具实例
//
// 使用示例：
//
//	handoffTool := handoff.NewHandoffTool()
//	agent := blades.NewAgent("router", blades.WithTools(handoffTool))
func NewHandoffTool() tools.Tool {
	return &handoffTool{}
}

// Name 返回工具的名称。
// 此名称用于 Agent 识别和调用工具。
func (h *handoffTool) Name() string { return "handoff_to_agent" }

// Description 返回工具的描述。
// 描述会提供给 Agent，帮助它理解何时以及如何使用此工具。
//
// 使用时机：
// - 当问题更适合其他专业 Agent 处理时
// - 当需要将任务路由到特定领域的 Agent 时
// - 当多 Agent 协作架构中需要任务分发时
func (h *handoffTool) Description() string {
	return `Transfer the question to another agent.
Use this tool to hand off control to a more suitable agent based on the agents' descriptions.`
}

// InputSchema 返回工具的输入 JSON Schema。
// 该 Schema 用于验证 Agent 传递的工具调用参数。
//
// Schema 结构说明：
// - 类型：对象（object）
// - 必需字段：agentName
// - agentName: 目标代理的名称（字符串）
//
// 使用示例（Agent 调用）：
//
//	{
//	  "agentName": "research-agent"
//	}
func (h *handoffTool) InputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:     "object",
		Required: []string{"agentName"},
		Properties: map[string]*jsonschema.Schema{
			"agentName": {
				Type:        "string",
				Description: "The name of the target agent to hand off the request to.",
			},
		},
	}
}

// OutputSchema 返回工具的输出 JSON Schema。
// 返回 nil 表示此工具没有结构化的输出，只返回空消息。
func (h *handoffTool) OutputSchema() *jsonschema.Schema { return nil }

// Handle 处理工具调用请求，执行交接逻辑。
//
// 参数说明：
// - ctx: 上下文，包含工具执行环境和状态
// - input: JSON 格式的工具输入字符串，包含目标代理名称
//
// 处理流程：
// 1. 解析输入的 JSON 数据，获取目标代理名称
// 2. 验证代理名称非空
// 3. 从上下文中获取工具上下文（ToolContext）
// 4. 将交接动作和目标代理名称设置到上下文中
//
// 关键点：
// - 此工具不直接执行交接，而是将交接信息设置到上下文中
// - 外部系统会读取上下文中的交接信息并执行实际的交接逻辑
// - 返回空字符串表示交接后当前 Agent 的任务结束
//
// 返回值：
// - string: 空字符串（交接后不产生响应内容）
// - error: 处理过程中的错误（JSON 解析失败、代理名为空、工具上下文不存在等）
func (h *handoffTool) Handle(ctx context.Context, input string) (string, error) {
	// 解析输入 JSON 获取参数
	args := map[string]string{}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	// 提取并验证代理名称
	agentName := strings.TrimSpace(args["agentName"])
	if agentName == "" {
		return "", fmt.Errorf("agentName must be a non-empty string")
	}
	// 从上下文中获取工具上下文
	toolCtx, ok := tools.FromContext(ctx)
	if !ok {
		return "", fmt.Errorf("tool context not found in context")
	}
	// 设置交接动作和目标代理名称到上下文
	// 外部系统会读取这些信息并执行实际的交接
	toolCtx.SetAction(ActionHandoffToAgent, agentName)
	return "", nil
}
