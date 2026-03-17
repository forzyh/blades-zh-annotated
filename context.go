// Package blades 提供了用于构建 AI 代理的核心抽象和组件。
package blades

import (
	"context"

	"github.com/go-kratos/blades/tools"
	"github.com/go-kratos/kit/container/maps"
)

// AgentContext 提供关于代理的元数据接口。
// 通过上下文可以获取当前执行代理的名称和描述，
// 这在中间件、日志记录或工具执行时非常有用。
type AgentContext interface {
	// Name 返回代理的名称
	Name() string
	// Description 返回代理的功能描述
	Description() string
}

// ToolContext 是 tools.ToolContext 的类型别名。
// 保持向后兼容，使现有代码中调用 blades.ToolContext 或 blades.FromToolContext 仍能正常工作。
// ToolContext 提供了工具执行时的上下文信息，包括工具 ID、名称和 Actions 映射。
type ToolContext = tools.ToolContext

// ctxAgentKey 是用于在上下文中存储 AgentContext 的私有键类型。
// 使用私有类型可以防止外部包意外修改或冲突。
type ctxAgentKey struct{}

// NewAgentContext 返回一个新的上下文，携带给定的 AgentContext。
// 在 Agent 执行时调用，将当前代理注入上下文，供后续处理使用。
//
// 参数：
//   - ctx: 父上下文
//   - agent: 要注入的代理（实现 Agent 接口）
//
// 返回：
//   - context.Context: 携带代理信息的子上下文
func NewAgentContext(ctx context.Context, agent Agent) context.Context {
	return context.WithValue(ctx, ctxAgentKey{}, agent)
}

// FromAgentContext 从上下文中检索 AgentContext。
// 如果上下文中不存在 AgentContext，返回 (nil, false)。
// 通常在中间件或工具处理中调用，以获取当前代理的信息。
//
// 返回：
//   - AgentContext: 代理上下文（如果存在）
//   - bool: 是否成功获取（false 表示上下文中没有存储 AgentContext）
func FromAgentContext(ctx context.Context) (AgentContext, bool) {
	agent, ok := ctx.Value(ctxAgentKey{}).(AgentContext)
	return agent, ok
}

// toolContext 是 ToolContext 接口的内部实现。
// 存储工具执行时的元数据：
// - id: 工具调用的唯一标识符
// - name: 工具名称
// - actions: 工具可以读写的动态映射，用于传递额外信息
type toolContext struct {
	id      string
	name    string
	actions *maps.Map[string, any]
}

// ID 返回工具调用的唯一标识符。
func (t *toolContext) ID() string {
	return t.id
}

// Name 返回工具的名称。
func (t *toolContext) Name() string {
	return t.name
}

// Actions 返回工具的动作映射（拷贝）。
// Actions 允许工具在执行过程中设置或获取额外信息，
// 如进度更新、中间结果等。
func (t *toolContext) Actions() map[string]any {
	return t.actions.ToMap()
}

// SetAction 设置一个动作键值对。
// 工具可以通过此方法存储状态或信号，供其他组件参考。
func (t *toolContext) SetAction(key string, value any) {
	t.actions.Store(key, value)
}
