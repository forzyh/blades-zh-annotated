// Package deep 提供了 DeepAgent 的核心指令和工具定义。
// 本文件实现了 task 工具，用于派生子代理（subagent）执行独立的复杂任务。
package deep

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"
	"github.com/google/jsonschema-go/jsonschema"
)

// 确保 taskTool 实现了 tools.Tool 接口。
// 这是一个编译时检查，保证 taskTool 实现了所有必需的方法。
var _ tools.Tool = (*taskTool)(nil)

// TaskToolConfig 是 task 工具的配置结构体。
// 用于在创建 task 工具时传递各种参数。
//
// 字段说明：
// - Model: 模型提供者，用于创建通用子代理
// - SubAgents: 预定义的子代理列表，Agent 可以选择使用
// - Tools: 工具列表，会传递给通用子代理
// - Instructions: 指令列表，会传递给通用子代理
// - MaxIterations: 最大迭代次数，限制子代理的执行轮数
// - WithoutGeneralPurposeAgent: 是否禁用通用子代理
//
// 使用示例：
//
//	config := TaskToolConfig{
//	    Model:         model,
//	    SubAgents:     []blades.Agent{researchAgent, codingAgent},
//	    Tools:         tools,
//	    MaxIterations: 50,
//	}
//	taskTool, _, err := NewTaskTool(config)
type TaskToolConfig struct {
	Model                      blades.ModelProvider
	SubAgents                  []blades.Agent
	Tools                      []tools.Tool
	Instructions               []string
	MaxIterations              int
	WithoutGeneralPurposeAgent bool
}

// newGeneralPurposeAgent 创建一个通用子代理。
// 当没有其他专用代理可用时，使用此通用代理处理各种任务。
//
// 参数说明：
// - tc: TaskToolConfig 配置结构体
//
// 创建的通用代理具有以下特点：
// - 名称："general-purpose"
// - 使用配置的模型提供者
// - 使用预定义的通用代理描述
// - 支持配置的指令、工具和最大迭代次数
//
// 返回值：
// - blades.Agent: 创建的通用子代理
// - error: 创建过程中的错误
func newGeneralPurposeAgent(tc TaskToolConfig) (blades.Agent, error) {
	return blades.NewAgent(generalAgentName,
		blades.WithModel(tc.Model),
		blades.WithDescription(generalAgentDescription),
		blades.WithInstruction(strings.Join(tc.Instructions, "\n\n")),
		blades.WithTools(tc.Tools...),
		blades.WithMaxIterations(tc.MaxIterations),
	)
}

// NewTaskTool 创建并返回一个新的 task 工具实例。
//
// 参数说明：
// - tc: TaskToolConfig 配置结构体
//
// 处理流程：
// 1. 创建 taskTool 结构体，初始化子代理列表和映射
// 2. 如果未禁用通用子代理，创建一个并添加到子代理列表
// 3. 构建子代理名称到代理实例的映射，便于快速查找
// 4. 生成工具描述（包含所有可用子代理的信息）
//
// 返回值：
// - tools.Tool: task 工具实例
// - string: 工具的使用提示语（prompt）
// - error: 创建过程中的错误
//
// 使用示例：
//
//	taskTool, prompt, err := deep.NewTaskTool(TaskToolConfig{
//	    Model:     model,
//	    SubAgents: []blades.Agent{researchAgent},
//	})
func NewTaskTool(tc TaskToolConfig) (tools.Tool, string, error) {
	t := &taskTool{
		subAgents:    tc.SubAgents,
		subAgentsMap: make(map[string]blades.Agent),
	}
	// 如果没有禁用通用子代理，创建一个并添加到列表
	if !tc.WithoutGeneralPurposeAgent {
		generalAgent, err := newGeneralPurposeAgent(tc)
		if err != nil {
			return nil, "", err
		}
		t.subAgents = append(t.subAgents, generalAgent)
	}
	// 构建名称到代理实例的映射，O(1) 查找
	for _, a := range t.subAgents {
		t.subAgentsMap[a.Name()] = a
	}
	// 生成工具描述，包含所有子代理的信息
	description, err := t.buildDescription()
	if err != nil {
		return nil, "", err
	}
	t.description = description
	return t, taskPrompt, nil
}

// taskTool 是 task 工具的具体实现结构体。
//
// 字段说明：
// - description: 工具的描述，包含所有可用子代理的信息
// - subAgents: 可用的子代理列表
// - subAgentsMap: 子代理名称到实例的映射，用于快速查找
//
// 工作原理：
// task 工具允许主 Agent 派生子代理来执行独立的复杂任务。
// 子代理有自己的上下文窗口，与主线程隔离，适合处理：
// - 需要大量 Token 的任务
// - 可以并行执行的独立任务
// - 只需要最终结果、不需要中间过程的任务
type taskTool struct {
	description  string
	subAgents    []blades.Agent
	subAgentsMap map[string]blades.Agent
}

// Name 返回工具的名称。
// 此名称用于 Agent 识别和调用工具。
func (t *taskTool) Name() string { return "task" }

// buildDescription 构建并返回工具的描述字符串。
//
// 处理流程：
// 1. 遍历所有子代理，生成"名称：描述"格式的列表
// 2. 使用 Go 模板引擎将子代理列表注入到预定义的模板中
// 3. 返回完整的工具描述
//
// 模板变量：
// - SubAgents: 子代理描述列表，格式为 "- name: description"
//
// 返回值：
// - string: 完整的工具描述
// - error: 模板执行过程中的错误
func (t *taskTool) buildDescription() (string, error) {
	// 构建子代理描述列表
	descs := make([]string, 0, len(t.subAgents))
	for _, a := range t.subAgents {
		descs = append(descs, fmt.Sprintf("- %s: %s", a.Name(), a.Description()))
	}
	// 使用模板生成完整描述
	var sb strings.Builder
	if err := taskToolDescriptionTmpl.Execute(&sb, map[string]any{
		"SubAgents": strings.Join(descs, "\n"),
	}); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// Description 返回工具的描述。
// 描述会提供给 Agent，帮助它理解何时以及如何使用此工具。
func (t *taskTool) Description() string {
	return t.description
}

// taskToolRequest 是 task 工具的请求结构体。
// 它封装了工具调用时传递的参数。
type taskToolRequest struct {
	// SubagentType 指定要使用的子代理类型（名称）
	SubagentType string `json:"subagent_type"`
	// Description 任务的详细描述，包括指令和预期输出
	Description string `json:"description"`
}

// InputSchema 返回工具的输入 JSON Schema。
// 该 Schema 用于验证 Agent 传递的工具调用参数。
//
// Schema 结构说明：
// - 类型：对象（object）
// - 必需字段：subagent_type, description
// - subagent_type: 子代理类型名称（字符串）
// - description: 任务描述（字符串）
func (t *taskTool) InputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:     "object",
		Required: []string{"subagent_type", "description"},
		Properties: map[string]*jsonschema.Schema{
			"subagent_type": {
				Type:        "string",
				Description: "The type of subagent to use",
			},
			"description": {
				Type:        "string",
				Description: "A short description of the task",
			},
		},
	}
}

// OutputSchema 返回工具的输出 JSON Schema。
// 返回 nil 表示此工具没有结构化的输出，只返回文本消息。
func (t *taskTool) OutputSchema() *jsonschema.Schema { return nil }

// Handle 处理工具调用请求，派生子代理执行任务。
//
// 参数说明：
// - ctx: 上下文，用于控制取消和超时
// - input: JSON 格式的工具输入字符串
//
// 处理流程：
// 1. 解析输入的 JSON 数据，获取子代理类型和任务描述
// 2. 根据子代理类型查找对应的代理实例
// 3. 使用 blades.NewAgentTool 创建代理工具并执行
//
// 返回值：
// - string: 子代理执行的结果
// - error: 处理过程中的错误（子代理不存在、执行失败等）
//
// 错误处理：
// - 如果指定的子代理类型不存在，返回错误
// - 如果 JSON 解析失败，返回错误
// - 如果子代理执行失败，返回错误
func (t *taskTool) Handle(ctx context.Context, input string) (string, error) {
	// 解析输入 JSON
	var req taskToolRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", err
	}
	// 查找指定的子代理
	agent, ok := t.subAgentsMap[req.SubagentType]
	if !ok {
		return "", fmt.Errorf("subagent type %s not found", req.SubagentType)
	}
	// 创建代理工具并执行任务
	return blades.NewAgentTool(agent).Handle(ctx, req.Description)
}
