package blades

import (
	"context"

	"github.com/go-kratos/blades/tools"
	"github.com/google/jsonschema-go/jsonschema"
)

// agentTool 是一个将 Agent 包装为 Tool 的内部类型。
// 这使得 Agent 可以作为工具被其他 Agent 调用，实现 Agent 嵌套和组合。
// 通过这种方式，可以将复杂任务分解为多个专用 Agent 协作完成。
type agentTool struct {
	Agent // 嵌入 Agent 接口，自动实现 Name() 和 Description()
}

// NewAgentTool 创建一个新工具，包装给定的 Agent。
// 这是将 Agent 转换为工具的工厂函数。
//
// 使用场景：
// 1. 将一个专用 Agent 作为工具提供给主 Agent 使用
// 2. 实现 Agent 组合和任务分解
// 3. 构建多层 Agent 架构
//
// 参数：
//   - agent: 要包装的 Agent
//
// 返回：
//   - tools.Tool: 包装后的工具
//
// 使用示例：
//
//	// 创建一个搜索 Agent
//	searchAgent, _ := blades.NewAgent("搜索专家", ...)
//	// 将其转换为工具
//	searchTool := blades.NewAgentTool(searchAgent)
//	// 提供给主 Agent 使用
//	mainAgent, _ := blades.NewAgent("助手",
//	    blades.WithModel(model),
//	    blades.WithTools(searchTool),
//	)
func NewAgentTool(agent Agent) tools.Tool {
	return &agentTool{Agent: agent}
}

// InputSchema 返回底层 Agent 的输入 Schema（如果有）。
// 通过类型断言检查 Agent 是否实现了 InputSchema() 方法。
// 这是 tools.Tool 接口的可选方法，用于描述工具的输入格式。
//
// 返回：
//   - *jsonschema.Schema: 输入 Schema，如果 Agent 不支持则返回 nil
func (a *agentTool) InputSchema() *jsonschema.Schema {
	if agent, ok := a.Agent.(interface {
		InputSchema() *jsonschema.Schema
	}); ok {
		return agent.InputSchema()
	}
	return nil
}

// OutputSchema 返回底层 Agent 的输出 Schema（如果有）。
// 通过类型断言检查 Agent 是否实现了 OutputSchema() 方法。
// 这是 tools.Tool 接口的可选方法，用于描述工具的输出格式。
//
// 返回：
//   - *jsonschema.Schema: 输出 Schema，如果 Agent 不支持则返回 nil
func (a *agentTool) OutputSchema() *jsonschema.Schema {
	if agent, ok := a.Agent.(interface {
		OutputSchema() *jsonschema.Schema
	}); ok {
		return agent.OutputSchema()
	}
	return nil
}

// Handle 使用给定的输入运行底层 Agent 并返回输出。
// 这是 tools.Tool 接口的核心方法。
//
// 执行流程：
// 1. 创建 Invocation，将输入字符串包装为用户消息
// 2. 调用 Agent.Run 执行
// 3. 消费迭代器，获取最终消息
// 4. 返回消息的文本内容
//
// 参数：
//   - ctx: 上下文
//   - input: 输入字符串（会被包装为用户消息）
//
// 返回：
//   - string: Agent 输出的文本内容
//   - error: 执行失败时的错误
func (a *agentTool) Handle(ctx context.Context, input string) (string, error) {
	// 创建 Invocation，将输入作为用户消息
	iter := a.Agent.Run(ctx, &Invocation{Message: UserMessage(input)})
	var (
		err   error
		final *Message
	)
	// 消费迭代器，等待 Agent 完成
	for final, err = range iter {
		if err != nil {
			return "", err
		}
	}
	// 返回最终消息的文本
	if final != nil {
		return final.Text(), nil
	}
	return "", ErrNoFinalResponse
}
