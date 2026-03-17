package tools

import "context"

// ============================================================================
// Resolver：工具解析器接口
// ============================================================================

// Resolver 定义了动态解析工具的接口。
//
// 【是什么】
// Resolver 是一个用于"动态获取工具列表"的组件。
// 与静态注册的工具不同，Resolver 允许在运行时从各种来源动态获取工具。
//
// 【为什么需要 Resolver】
// 1. 动态工具源：工具可能不在代码中硬编码，而是来自外部服务
// 2. 按需加载：不需要在启动时加载所有工具，可以按需解析
// 3. 可扩展性：新的工具源只需实现 Resolver 接口即可接入
//
// 【使用场景】
//   - MCP 服务器：从 Model Context Protocol 服务器获取可用工具
//   - 插件系统：从插件目录动态加载工具
//   - 远程服务：从 HTTP API 获取工具列表
//   - 条件工具：根据运行时条件返回不同的工具
//
// 【与静态工具的区别】
//   - 静态工具：在代码中创建并注册，编译期确定
//   - 动态工具（Resolver）：运行时解析，可以变化
//
// 【怎么实现】
// 实现 Resolver 接口需要：
// 1. 定义一个结构体（如 MCPServerResolver）
// 2. 实现 Resolve 方法，返回工具列表
// 3. 在方法内部可以从任何来源获取工具
//
// 【使用示例】
//
//	// 从 MCP 服务器解析工具
//	type MCPServerResolver struct {
//	    server *mcp.Server
//	}
//
//	func (r *MCPServerResolver) Resolve(ctx context.Context) ([]Tool, error) {
//	    // 从服务器获取工具列表
//	    mcpTools := r.server.ListTools()
//	    // 转换为 blades 的 Tool 接口
//	    return convertToBladesTools(mcpTools), nil
//	}
//
// 【工作流程】
// 1. Agent 准备执行时，调用所有注册的 Resolver.Resolve()
// 2. 每个 Resolver 返回它管理的工具列表
// 3. Agent 合并所有工具，提供给 LLM 选择
//
// 【注意事项】
//   - Resolve 可能被多次调用，应该保证幂等性
//   - 应该处理错误情况，返回有意义的 error
//   - 上下文 ctx 可用于超时控制和取消操作
type Resolver interface {
	// Resolve 返回当前可用的工具列表。
	//
	// 【参数】
	//   - ctx: 上下文，用于超时控制和取消操作
	//
	// 【返回值】
	//   - []Tool: 工具列表，可以为空但不能为 nil（建议返回空切片）
	//   - error: 如果解析失败（如网络错误、服务不可用），返回错误
	//
	// 【调用时机】
	//   - Agent 初始化时
	//   - 每次执行前（取决于实现）
	//   - 显式刷新时
	//
	// 【性能考虑】
	//   - 如果解析开销大，可以考虑缓存结果
	//   - 可以使用 ctx 控制超时，避免长时间阻塞
	Resolve(ctx context.Context) ([]Tool, error)
}
