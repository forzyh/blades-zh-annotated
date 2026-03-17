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
	"fmt"
	"time"
)

// TransportType 定义 MCP 服务器的通信方式。
//
// # 支持的传输类型
//
//   - TransportStdio: 标准输入输出，通过子进程运行服务器
//   - TransportHTTP: HTTP/HTTPS，通过 HTTP 协议通信
//   - TransportWebSocket: WebSocket，通过 WebSocket 协议通信
//
// # 选择指南
//
//   - 本地服务器：使用 Stdio，无需网络，安全性高
//   - 远程服务器：使用 HTTP 或 WebSocket，支持分布式部署
type TransportType string

const (
	// TransportStdio 使用标准输入/输出进行通信。
	// 适用于本地运行的 MCP 服务器进程。
	TransportStdio TransportType = "stdio"
	// TransportHTTP 使用 HTTP 进行通信。
	// 适用于远程 MCP 服务器，支持 SSE（Server-Sent Events）。
	TransportHTTP TransportType = "http"
	// TransportWebSocket 使用 WebSocket 进行通信。
	// 适用于需要双向实时通信的场景。
	TransportWebSocket TransportType = "websocket"
)

// ClientConfig 配置 MCP 服务器连接。
//
// # 必填字段
//
//   - Name: 服务器唯一标识符
//   - Transport: 通信方式
//
// # 传输方式特定配置
//
// == Stdio 传输 (Transport = TransportStdio) ==
//
//   - Command: 可执行文件路径（如 "python", "node", "npx"）
//   - Args: 命令行参数
//   - Env: 环境变量
//   - WorkDir: 工作目录
//
// == HTTP/WebSocket 传输 (Transport = TransportHTTP/WebSocket) ==
//
//   - Endpoint: 服务器 URL（如 "https://mcp.example.com/sse"）
//   - Headers: 自定义 HTTP 请求头
//   - Timeout: 请求超时
//
// # 使用示例
//
//	// Stdio 方式运行 Python MCP 服务器
//	stdioConfig := ClientConfig{
//	    Name:      "time-server",
//	    Transport: TransportStdio,
//	    Command:   "python",
//	    Args:      []string{"-m", "mcp_server_time"},
//	    Env:       map[string]string{"TZ": "UTC"},
//	}
//
//	// HTTP 方式连接远程服务器
//	httpConfig := ClientConfig{
//	    Name:      "remote-server",
//	    Transport: TransportHTTP,
//	    Endpoint:  "https://mcp.example.com/sse",
//	    Headers:   map[string]string{"Authorization": "Bearer token"},
//	    Timeout:   60 * time.Second,
//	}
type ClientConfig struct {
	// Name 是 MCP 服务器的唯一标识符
	Name string
	// Transport 指定通信方式
	Transport TransportType
	// === Stdio 传输配置（当 Transport = TransportStdio 时）===
	// Command 是要运行的可执行文件（如 "python", "node", "npx"）
	Command string
	// Args 是命令行参数（如 ["-m", "mcp_server_time"]）
	Args []string
	// Env 包含子进程的环境变量
	Env map[string]string
	// WorkDir 是子进程的工作目录
	WorkDir string
	// === HTTP 配置（当 Transport = TransportHTTP 时）===
	// Endpoint 是 MCP 服务器 URL
	Endpoint string
	// Headers 是请求中包含的自定义 HTTP 头
	Headers map[string]string
	// Timeout 是请求超时时长
	Timeout time.Duration
}

// validate 检查配置是否有效。
//
// # 验证规则
//
//   - Stdio 传输：必须有 Command（要执行的命令）
//   - HTTP/WebSocket 传输：必须有 Endpoint（服务器 URL）
//   - Transport 必须是支持的类型
//
// # 返回值
//
// error: 配置无效时返回错误
func (c *ClientConfig) validate() error {
	switch c.Transport {
	case TransportStdio:
		// Stdio 传输必须有命令
		if c.Command == "" {
			return fmt.Errorf("mcp: invalid config: command is required for stdio transport")
		}
	case TransportHTTP, TransportWebSocket:
		// HTTP/WebSocket 传输必须有 URL
		if c.Endpoint == "" {
			return fmt.Errorf("mcp: invalid config: URL is required for HTTP/WebSocket transport")
		}
	default:
		return fmt.Errorf("mcp: invalid config: unsupported transport type: %s", c.Transport)
	}
	return nil
}
