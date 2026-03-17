// Package deep 提供了 DeepAgent 的核心指令和工具定义。
// 本文件实现了 write_todos 工具，用于管理复杂任务的任务列表。
package deep

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-kratos/blades/tools"
	"github.com/google/jsonschema-go/jsonschema"
)

// 确保 writeTodosTool 实现了 tools.Tool 接口。
// 这是一个编译时检查：如果 writeTodosTool 没有实现 tools.Tool 的所有方法，编译将失败。
var _ tools.Tool = (*writeTodosTool)(nil)

// NewWriteTodosTool 创建并返回一个新的 write_todos 工具实例。
//
// 返回值说明：
// - tools.Tool: write_todos 工具实例，实现了 tools.Tool 接口
// - string: 工具的使用提示语（prompt），指导 Agent 如何使用此工具
// - error: 创建过程中的错误（此实现中始终为 nil）
//
// 使用示例：
//
//	tool, prompt, err := deep.NewWriteTodosTool()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// 将 tool 注册到 Agent 中
//	agent := blades.NewAgent("my-agent", blades.WithTools(tool))
func NewWriteTodosTool() (tools.Tool, string, error) {
	return &writeTodosTool{}, writeTodosToolPrompt, nil
}

// writeTodosTool 是 write_todos 工具的具体实现结构体。
// 它是一个空结构体，因为工具的状态都通过方法参数传递，不需要内部状态。
//
// 工具功能：
// - 创建和管理结构化的任务列表
// - 跟踪每个任务的状态（pending、in_progress、completed）
// - 帮助用户了解复杂任务的进展
type writeTodosTool struct{}

// Name 返回工具的名称。
// 此名称用于 Agent 识别和调用工具。
func (t *writeTodosTool) Name() string { return "write_todos" }

// Description 返回工具的描述。
// 描述会提供给 Agent，帮助它理解何时以及如何使用此工具。
func (t *writeTodosTool) Description() string {
	return writeTodosToolDescription
}

// TODO 表示任务列表中的单个任务项。
//
// 字段说明：
// Content - 任务的描述内容，说明需要完成什么工作
// Status  - 任务的当前状态，只能是以下值之一：
//   - "pending": 任务尚未开始
//   - "in_progress": 任务正在进行中
//   - "completed": 任务已完成
//
// 使用示例：
//
//	todo := TODO{
//	    Content: "编写用户认证模块",
//	    Status:  "in_progress",
//	}
type TODO struct {
	Content string `json:"content"`
	Status  string `json:"status" jsonschema:"enum=pending,enum=in_progress,enum=completed"`
}

// writeTodosRequest 是 write_todos 工具的请求结构体。
// 它封装了工具调用时传递的参数。
type writeTodosRequest struct {
	Todos []TODO `json:"todos"`
}

// InputSchema 返回工具的输入 JSON Schema。
// 该 Schema 用于验证 Agent 传递的工具调用参数。
//
// Schema 结构说明：
// - 类型：对象（object）
// - 必需字段：todos
// - todos 字段：
//   - 类型：数组（array）
//   - 描述：更新后的任务列表
//   - 数组元素：TODO 对象，包含 content 和 status 字段
//
// JSON Schema 的作用：
// 1. 告诉 Agent 如何正确调用工具
// 2. 在运行时验证输入参数的格式
// 3. 提供字段描述和枚举值约束
func (t *writeTodosTool) InputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:     "object",
		Required: []string{"todos"},
		Properties: map[string]*jsonschema.Schema{
			"todos": {
				Type:        "array",
				Description: "The updated todo list",
				Items: &jsonschema.Schema{
					Type:     "object",
					Required: []string{"content", "status"},
					Properties: map[string]*jsonschema.Schema{
						"content": {
							Type:        "string",
							Description: "The task description",
						},
						"status": {
							Type:        "string",
							Description: "The task status",
							Enum:        []any{"pending", "in_progress", "completed"},
						},
					},
				},
			},
		},
	}
}

// OutputSchema 返回工具的输出 JSON Schema。
// 返回 nil 表示此工具没有结构化的输出，只返回文本消息。
//
// 为什么返回 nil？
// write_todos 工具的执行结果是一个简单的确认消息，
// 不需要结构化的 JSON 输出，因此不需要定义输出 Schema。
func (t *writeTodosTool) OutputSchema() *jsonschema.Schema { return nil }

// Handle 处理工具调用请求。
//
// 参数说明：
// - ctx: 上下文，用于控制取消和超时
// - input: JSON 格式的工具输入字符串
//
// 处理流程：
// 1. 将输入 JSON 字符串解析为 writeTodosRequest 结构体
// 2. 将任务列表序列化为 JSON 字符串（用于响应）
// 3. 返回确认消息
//
// 返回值：
// - string: 工具执行的响应消息
// - error: 处理过程中的错误
//
// 注意：
// 此实现只是简单地返回确认消息，没有实际存储任务列表。
// 实际应用中可能需要将任务列表保存到会话状态或数据库中。
func (t *writeTodosTool) Handle(ctx context.Context, input string) (string, error) {
	// 解析输入的 JSON 数据
	req := new(writeTodosRequest)
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", err
	}
	// 将任务列表序列化为 JSON
	todos, err := json.Marshal(req.Todos)
	if err != nil {
		return "", err
	}
	// 返回确认消息，包含更新后的任务列表 JSON
	return fmt.Sprintf("Updated todo list to %s", todos), nil
}
