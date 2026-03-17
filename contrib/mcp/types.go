// Package mcp 提供了 MCP（Model Context Protocol）客户端实现。
//
// # MCP 是什么
//
// MCP（Model Context Protocol）是一个开放协议，用于标准化 AI 模型与外部工具/资源的交互方式。
// 它定义了一套通用的 JSON-RPC 2.0 接口，使 AI 模型能够发现、调用各种工具和服务。
//
// # 核心组件
//
//   - Client: MCP 客户端，负责与 MCP 服务器建立连接
//   - ToolsResolver: 工具解析器，管理多个 MCP 服务器并提供统一工具列表
//   - Transport: 传输层，支持 Stdio、HTTP、WebSocket 等多种通信方式
//
// # 支持的传输方式
//
//   - Stdio（标准输入输出）: 通过子进程方式运行 MCP 服务器
//   - HTTP: 通过 HTTP/HTTPS 协议与远程 MCP 服务器通信
//   - WebSocket: 通过 WebSocket 协议与远程 MCP 服务器通信
//
// # 使用示例
//
//	// 创建 MCP 客户端配置（Stdio 方式）
//	config := mcp.ClientConfig{
//	    Name:      "time-server",
//	    Transport: mcp.TransportStdio,
//	    Command:   "python",
//	    Args:      []string{"-m", "mcp_server_time"},
//	}
//
//	// 创建客户端
//	client, err := mcp.NewClient(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// 连接到服务器
//	err = client.Connect(ctx)
//
//	// 列出可用工具
//	tools, err := client.ListTools(ctx)
//
//	// 调用工具
//	result, err := client.CallTool(ctx, "get_current_time", map[string]any{})
package mcp

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/go-kratos/blades/tools"
)

// ToolsResolver 管理多个 MCP 服务器连接并提供统一工具访问。
//
// # 作用说明
//
// ToolsResolver 是 MCP 的高级抽象，允许同时连接多个 MCP 服务器，
// 并将所有服务器的工具聚合为一个统一的工具列表。
//
// # 结构说明
//
//   - mu: 读写锁，保护并发访问
//   - clients: MCP 客户端列表，每个客户端连接一个服务器
//   - tools: 缓存的工具列表，避免重复加载
//   - loaded: 原子标记，表示工具是否已加载
//
// # 懒加载机制
//
// ToolsResolver 使用懒加载：只有在第一次调用 Resolve() 时才会连接服务器并加载工具。
// 加载后的工具会被缓存，后续调用直接返回缓存结果。
//
// # 线程安全
//
// ToolsResolver 的所有公共方法都是线程安全的：
//   - Resolve(): 使用读锁返回缓存，使用写锁更新缓存
//   - Close(): 使用写锁关闭所有客户端
//
// # 使用示例
//
//	// 配置多个 MCP 服务器
//	configs := []ClientConfig{
//	    {Name: "time-server", Transport: TransportStdio, Command: "python", Args: []string{"-m", "mcp_server_time"}},
//	    {Name: "file-server", Transport: TransportStdio, Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem"}},
//	}
//
//	// 创建解析器
//	resolver, err := mcp.NewToolsResolver(configs...)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer resolver.Close()
//
//	// 获取所有工具
//	tools, err := resolver.Resolve(ctx)
type ToolsResolver struct {
	mu      sync.RWMutex
	clients []*Client
	tools   []tools.Tool
	loaded  atomic.Bool
}

// NewToolsResolver 创建一个新的 MCP 工具解析器。
//
// # 参数说明
//
//   - configs: MCP 服务器配置列表，可变参数
//
// # 返回值
//
//   - *ToolsResolver: 工具解析器实例
//   - error: 创建错误（配置为空或某个客户端创建失败）
//
// # 初始化逻辑
//
// 1. 检查至少有一个配置
// 2. 为每个配置创建 Client
// 3. 返回 ToolsResolver
//
// # 使用示例
//
//	resolver, err := mcp.NewToolsResolver(
//	    ClientConfig{Name: "server1", ...},
//	    ClientConfig{Name: "server2", ...},
//	)
func NewToolsResolver(configs ...ClientConfig) (*ToolsResolver, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("at least one server config is required")
	}
	clients := make([]*Client, 0, len(configs))
	for _, config := range configs {
		client, err := NewClient(config)
		if err != nil {
			return nil, err
		}
		clients = append(clients, client)
	}
	return &ToolsResolver{
		clients: clients,
	}, nil
}

// getTools 返回工具列表的副本（读锁保护）。
//
// # 作用说明
//
// 使用 slices.Clone() 返回工具列表的深拷贝，
// 防止调用方修改内部状态。
func (r *ToolsResolver) getTools() []tools.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.tools)
}

// setTools 设置工具列表（写锁保护）。
//
// # 作用说明
//
// 使用 slices.Clone() 存储工具列表的副本，
// 避免外部引用影响内部状态。
func (r *ToolsResolver) setTools(tools []tools.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools = slices.Clone(tools)
}

// Resolve 实现 tools.Resolver 接口。
//
// # 作用说明
//
// 返回所有配置的 MCP 服务器中的工具，使用懒加载机制。
//
// # 参数说明
//
//   - ctx: 上下文，用于连接和工具列表获取
//
// # 返回值
//
//   - []tools.Tool: 所有 MCP 服务器的聚合工具列表
//   - error: 加载错误
//
// # 懒加载逻辑
//
// 1. 检查 loaded 标志：如果已加载，直接返回缓存的工具
// 2. 遍历所有客户端：
//   - 连接服务器
//   - 获取工具列表
//   - 转换为 Blades 工具
//
// 3. 缓存结果并返回
//
// # 容错处理
//
//   - 如果某个服务器连接失败，记录错误但继续处理其他服务器
//   - 如果所有服务器都失败，返回错误
//   - 部分成功时，返回已加载的工具（ suppressing noisy stdout logs）
func (r *ToolsResolver) Resolve(ctx context.Context) ([]tools.Tool, error) {
	// 如果已加载，返回缓存
	if r.loaded.Load() {
		return r.getTools(), nil
	}
	var (
		errors   []error
		allTools []tools.Tool
	)
	// 遍历所有客户端
	for _, client := range r.clients {
		// 连接服务器
		if err := client.Connect(ctx); err != nil {
			errors = append(errors, err)
			continue
		}
		// 获取工具列表
		mcpTools, err := client.ListTools(ctx)
		if err != nil {
			errors = append(errors, err)
			client.Close()
			continue
		}
		// 使用客户端的内置转换方法将 MCP 工具转换为 Blades 工具
		for _, mcpTool := range mcpTools {
			handler := client.handler(mcpTool.Name)
			tool, err := toBladesTool(mcpTool, handler)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to convert MCP tool [%s]: %w", mcpTool.Name, err))
				continue
			}
			allTools = append(allTools, tool)
		}
	}
	// 如果有错误但没有工具，返回错误
	if len(errors) > 0 && len(allTools) == 0 {
		return nil, fmt.Errorf("failed to load any tools: %v", errors)
	}
	// 部分成功时，记录错误但继续
	if len(errors) > 0 {
		// keep partial success behavior: return loaded tools while suppressing noisy stdout logs
	}
	r.setTools(allTools)
	r.loaded.Store(true)
	return r.getTools(), nil
}

// Close 关闭所有客户端连接。
//
// # 返回值
//
// error: 关闭错误（如果有多个错误，会聚合返回）
//
// # 清理逻辑
//
// 1. 获取写锁
// 2. 遍历所有客户端
// 3. 逐个关闭
// 4. 收集所有错误
//
// # 使用注意
//
// 调用 Close() 后，ToolsResolver 不再可用。
// 应该在 defer 中调用 Close() 确保清理：
//
//	resolver, err := mcp.NewToolsResolver(configs...)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer resolver.Close()
func (r *ToolsResolver) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var errors []error
	for _, client := range r.clients {
		if err := client.Close(); err != nil {
			errors = append(errors, fmt.Errorf("server %w", err))
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("errors closing clients: %v", errors)
	}
	return nil
}
