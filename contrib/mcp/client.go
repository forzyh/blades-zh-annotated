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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 确保 Client 实现了 tools.Resolver 接口
var _ tools.Resolver = (*Client)(nil)

// Client 封装了官方 MCP SDK 客户端，用于单个服务器连接。
//
// # 结构说明
//
//   - config: 客户端配置，包含连接参数
//   - client: MCP SDK 客户端实例
//   - session: 活动会话，用于实际通信
//   - connected: 原子布尔，标记连接状态
//   - connectMutex: 保护并发连接的互斥锁
//   - connectCtx/cancel: 用于控制重连的上下文
//   - reconnecting: 原子布尔，标记是否正在重连
//
// # 连接管理
//
// Client 支持自动重连功能。当连接断开时，会在后台尝试重新连接，
// 使用指数退避策略（从 1 秒开始，最大 30 秒）。
//
// # 线程安全
//
// Client 的以下方法是线程安全的：
//   - Connect(): 使用互斥锁保护并发连接
//   - CallTool(): 自动检查连接状态
//   - Close(): 安全关闭连接
//
// # 使用示例
//
//	client, err := mcp.NewClient(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	err = client.Connect(ctx)
//	tools, err := client.ListTools(ctx)
type Client struct {
	config        ClientConfig
	client        *mcp.Client
	session       *mcp.ClientSession
	connected     atomic.Bool
	connectMutex  sync.Mutex
	connectCtx    context.Context
	connectCancel context.CancelFunc
	reconnecting  atomic.Bool
}

// NewClient 创建一个新的 MCP 客户端。
//
// # 参数说明
//
//   - config: 客户端配置，必须指定 Transport 类型及相应参数
//
// # 返回值
//
//   - *Client: MCP 客户端实例
//   - error: 配置验证失败或初始化错误
//
// # 初始化逻辑
//
// 1. 设置默认超时（30 秒），如果未指定
// 2. 验证配置有效性
// 3. 创建 MCP SDK 客户端
// 4. 初始化连接上下文
//
// # 配置验证规则
//
//   - Stdio 传输：必须有 Command
//   - HTTP/WebSocket 传输：必须有 Endpoint（URL）
func NewClient(config ClientConfig) (*Client, error) {
	// 设置默认超时
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	// 验证配置
	if err := config.validate(); err != nil {
		return nil, err
	}
	// 创建 MCP SDK 客户端
	client := mcp.NewClient(&mcp.Implementation{
		Name:    config.Name,
		Version: blades.Version,
	}, nil)
	c := &Client{
		config: config,
		client: client,
	}
	// 初始化连接上下文
	c.connectCtx, c.connectCancel = context.WithCancel(context.Background())
	return c, nil
}

// Connect 建立与 MCP 服务器的连接。
//
// # 参数说明
//
//   - ctx: 上下文，用于控制连接超时和取消
//
// # 返回值
//
// error: 连接失败错误
//
// # 连接流程
//
// 1. 获取互斥锁，防止并发连接
// 2. 检查是否已连接，已连接则直接返回
// 3. 根据 Transport 类型创建传输层
// 4. 调用 SDK Connect 方法建立会话
// 5. 启动后台重连协程
//
// # 幂等性
//
// 多次调用 Connect() 是安全的，如果已经连接，后续调用会直接返回。
func (c *Client) Connect(ctx context.Context) error {
	return c.connect(ctx, true)
}

// connect 是内部连接方法，支持是否启动重连的配置。
//
// # 参数说明
//
//   - ctx: 上下文
//   - startReconnect: 是否启动后台重连协程
//
// # 传输层创建
//
// 根据配置 Transport 类型：
//   - TransportStdio: 创建 CommandTransport（子进程）
//   - TransportHTTP/WebSocket: 创建 StreamableClientTransport
//
// # 错误处理
//
// 连接失败时返回详细错误信息，包含服务器名称和错误原因：
//   - create_transport: 传输层创建失败
//   - connect: 连接建立失败
func (c *Client) connect(ctx context.Context, startReconnect bool) error {
	// 确保一次只有一个连接尝试
	c.connectMutex.Lock()
	defer c.connectMutex.Unlock()
	// 重新创建上下文（如果之前的已取消）
	if c.connectCtx == nil || c.connectCtx.Err() != nil {
		c.connectCtx, c.connectCancel = context.WithCancel(context.Background())
	}
	// 如果已经连接，直接返回
	if c.connected.Load() {
		return nil
	}
	var (
		err       error
		transport mcp.Transport
	)
	// 根据配置创建传输层
	switch c.config.Transport {
	case TransportStdio:
		transport, err = c.createStdioTransport()
	case TransportHTTP, TransportWebSocket:
		// HTTP 和 WebSocket 都使用 StreamableClientTransport
		// 实际传输方式由 URL 协议决定（http/https vs ws/wss）
		transport, err = c.createStreamableTransport()
	default:
		return fmt.Errorf("mcp: invalid config: unsupported transport: %s", c.config.Transport)
	}
	if err != nil {
		return fmt.Errorf("mcp [%s] create_transport: %w", c.config.Name, err)
	}
	// 连接到服务器
	session, err := c.client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("mcp [%s] connect: %w", c.config.Name, err)
	}
	c.session = session
	c.connected.Store(true)
	// 启动后台重连协程
	if startReconnect && c.reconnecting.CompareAndSwap(false, true) {
		go c.reconnect(c.connectCtx)
	}
	return nil
}

// createStdioTransport 创建用于标准输入输出通信的 CommandTransport。
//
// # 作用说明
//
// Stdio 传输方式通过子进程运行 MCP 服务器，通过 stdin/stdout 进行通信。
// 适用于本地运行的 MCP 服务器（如 Python 脚本、Node.js 应用等）。
//
// # 配置说明
//
//   - Command: 可执行文件路径（如 "python", "node", "npx"）
//   - Args: 命令行参数（如 ["-m", "mcp_server_time"]）
//   - Env: 环境变量，会添加到进程环境
//   - WorkDir: 工作目录，子进程的运行目录
//
// # 环境变量处理
//
// 1. 继承当前进程的所有环境变量（os.Environ()）
// 2. 添加配置的 Env 变量，覆盖同名变量
//
// # 使用示例
//
//	// 运行 Python MCP 服务器
//	config := ClientConfig{
//	    Command: "python",
//	    Args:    []string{"-m", "mcp_server_time"},
//	    Env:     map[string]string{"TZ": "UTC"},
//	    WorkDir: "/opt/mcp",
//	}
func (c *Client) createStdioTransport() (mcp.Transport, error) {
	// 创建子进程命令
	cmd := exec.Command(c.config.Command, c.config.Args...)
	// 设置环境变量
	cmd.Env = os.Environ()
	if len(c.config.Env) > 0 {
		for k, v := range c.config.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	// 设置工作目录
	if c.config.WorkDir != "" {
		cmd.Dir = c.config.WorkDir
	}
	return &mcp.CommandTransport{
		Command: cmd,
	}, nil
}

// createStreamableTransport 创建用于 HTTP/WebSocket 通信的 StreamableClientTransport。
//
// # 作用说明
//
// HTTP/WebSocket 传输方式通过网络与远程 MCP 服务器通信。
// 支持 HTTP/HTTPS 和 WebSocket 协议，由 URL 协议自动决定。
//
// # URL 协议规则
//
//   - http:// 或 https://: 使用 HTTP 长轮询（SSE）
//   - ws:// 或 wss://: 使用 WebSocket
//
// # 自定义 HTTP 客户端
//
// 如果配置了 Headers，会创建自定义 http.Client，添加请求头：
//   - 使用 headerRoundTripper 包装基础 Transport
//   - 在每次请求前添加指定 headers
//
// # 使用示例
//
//	// HTTP MCP 服务器
//	config := ClientConfig{
//	    Transport: TransportHTTP,
//	    Endpoint:  "https://mcp.example.com/sse",
//	    Headers:   map[string]string{"Authorization": "Bearer token"},
//	}
func (c *Client) createStreamableTransport() (mcp.Transport, error) {
	transport := &mcp.StreamableClientTransport{
		Endpoint: c.config.Endpoint,
	}
	// 如果配置了自定义 headers，包装 HTTP 客户端
	if len(c.config.Headers) > 0 {
		baseTransport := http.DefaultTransport
		httpClient := &http.Client{
			Transport: newHeaderRoundTripper(c.config.Headers, baseTransport),
		}
		transport.HTTPClient = httpClient
	}
	return transport, nil
}

// ListTools 列出服务器上所有可用的工具。
//
// # 参数说明
//
//   - ctx: 上下文
//
// # 返回值
//
//   - []*mcp.Tool: 工具定义列表
//   - error: 请求错误
//
// # 自动连接
//
// 如果尚未连接，会自动调用 Connect() 建立连接。
func (c *Client) ListTools(ctx context.Context) ([]*mcp.Tool, error) {
	if !c.connected.Load() {
		if err := c.Connect(ctx); err != nil {
			return nil, err
		}
	}
	// 调用 SDK ListTools 方法
	result, err := c.session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp [%s] list_tools: %w", c.config.Name, err)
	}
	return result.Tools, nil
}

// Resolve 实现 tools.Resolver 接口。
//
// # 作用说明
//
// 将 MCP 服务器的工具转换为 Blades 框架的工具列表。
// 这是 Blades 框架统一工具加载机制的入口。
//
// # 参数说明
//
//   - ctx: 上下文，会添加超时控制
//
// # 返回值
//
//   - []tools.Tool: Blades 工具列表
//   - error: 解析错误
//
// # 处理流程
//
// 1. 创建带超时的上下文
// 2. 调用 ListTools 获取 MCP 工具
// 3. 逐个转换为 Blades 工具（toBladesTool）
// 4. 返回转换后的工具列表
//
// # 超时配置
//
// 使用 ClientConfig.Timeout 设置超时，默认 30 秒。
func (c *Client) Resolve(ctx context.Context) ([]tools.Tool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	mcpTools, err := c.ListTools(ctx)
	if err != nil {
		return nil, err
	}
	var res []tools.Tool
	for _, mcpTool := range mcpTools {
		// 为每个 MCP 工具创建 handler
		handler := c.handler(mcpTool.Name)
		tool, err := toBladesTool(mcpTool, handler)
		if err != nil {
			return nil, fmt.Errorf("failed to convert MCP tool [%s]: %w", mcpTool.Name, err)
		}
		res = append(res, tool)
	}
	return res, nil
}

// handler 返回一个工具处理函数，用于调用 MCP 工具。
//
// # 作用说明
//
// 创建一个闭包函数，将 Blades 框架的工具调用请求转发到 MCP 服务器。
// 这是 MCP 工具与 Blades 框架之间的桥梁。
//
// # 参数说明
//
//   - name: MCP 工具名称
//
// # 返回值
//
// tools.HandleFunc: 工具执行函数
//
// # 执行流程
//
// 1. 解析输入 JSON 为 map[string]any
// 2. 调用 CallTool 执行 MCP 工具
// 3. 格式化结果为 JSON 字符串
//
// # 错误处理
//
//   - JSON 解析失败：返回 "invalid input JSON" 错误
//   - 工具执行失败：返回格式化的错误信息
func (c *Client) handler(name string) tools.HandleFunc {
	return func(ctx context.Context, input string) (string, error) {
		var arguments map[string]any
		// 解析输入 JSON
		if err := json.Unmarshal([]byte(input), &arguments); err != nil {
			return "", fmt.Errorf("invalid input JSON: %w", err)
		}
		// 调用 MCP 工具
		result, err := c.CallTool(ctx, name, arguments)
		if err != nil {
			return "", err
		}
		// 格式化结果
		output, err := formatToolResult(result)
		if err != nil {
			return "", fmt.Errorf("failed to format tool result: %w", err)
		}
		return output, nil
	}
}

// CallTool 调用服务器上的 MCP 工具。
//
// # 参数说明
//
//   - ctx: 上下文
//   - name: 工具名称
//   - arguments: 工具参数，键值对 map
//
// # 返回值
//
//   - *mcp.CallToolResult: 工具执行结果
//   - error: 调用错误
//
// # 超时控制
//
// 使用 ClientConfig.Timeout 设置超时。
//
// # 自动连接
//
// 如果尚未连接，会自动调用 Connect() 建立连接。
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]any) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	if !c.connected.Load() {
		if err := c.Connect(ctx); err != nil {
			return nil, err
		}
	}
	// 调用 SDK CallTool 方法
	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("mcp [%s] call_tool: %w", c.config.Name, err)
	}
	return result, nil
}

// Close 关闭客户端连接。
//
// # 返回值
//
// error: 关闭错误
//
// # 清理操作
//
// 1. 取消连接上下文（停止重连）
// 2. 关闭会话
// 3. 重置连接状态
//
// # 幂等性
//
// 多次调用 Close() 是安全的。
func (c *Client) Close() error {
	if c.connectCancel != nil {
		c.connectCancel()
	}
	c.connectMutex.Lock()
	session := c.session
	c.session = nil
	c.connected.Store(false)
	c.connectMutex.Unlock()
	// 关闭会话
	if session != nil {
		if err := session.Close(); err != nil {
			return fmt.Errorf("mcp [%s] close: %w", c.config.Name, err)
		}
	}
	return nil
}

// reconnect 是后台重连协程的主循环。
//
// # 重连策略
//
// 使用指数退避算法：
//   - 初始等待：1 秒
//   - 每次失败后：等待时间翻倍
//   - 最大等待：30 秒
//
// # 重连流程
//
// 1. 等待当前会话结束（session.Wait()）
// 2. 标记断开连接
// 3. 循环尝试重连：
//   - 尝试连接
//   - 成功：重置退避时间，退出内层循环
//   - 失败：等待后退避时间后重试
//
// 4. 外层循环：等待会话再次断开
//
// # 退出条件
//
// 当连接上下文被取消（调用 Close 时），重连协程会退出。
func (c *Client) reconnect(ctx context.Context) {
	defer c.reconnecting.Store(false)
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		c.connectMutex.Lock()
		session := c.session
		connected := c.connected.Load()
		c.connectMutex.Unlock()
		// 等待会话结束
		if session != nil && connected {
			session.Wait()
			c.connected.Store(false)
		}

		// 重连循环
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			// 尝试重连
			if err := c.connect(ctx, false); err == nil {
				backoff = time.Second
				break
			}
			// 等待后退避时间
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			// 指数退避
			if backoff < maxBackoff {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		}
	}
}
