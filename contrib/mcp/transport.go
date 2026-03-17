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

import "net/http"

// headerPair 表示一个 HTTP 头键值对。
//
// # 设计说明
//
// 使用结构体而不是 map 来存储 header，是为了保持插入顺序。
// 某些场景下（如认证），header 的顺序可能有意义。
type headerPair struct {
	key   string
	value string
}

// headerRoundTripper 是一个 HTTP RoundTripper，用于在请求中添加自定义头。
//
// # 作用说明
//
// 实现了 http.RoundTripper 接口，包装基础 Transport，
// 在每次请求前添加配置的 headers。
//
// # 结构说明
//
//   - base: 基础 Transport，负责实际的 HTTP 通信
//   - headers: 要添加的头键值对列表
//
// # 使用场景
//
// 主要用于 MCP HTTP 传输，添加认证头等自定义头：
//
//	transport := &http.Transport{}
//	headers := map[string]string{
//	    "Authorization": "Bearer token",
//	    "X-Custom-Header": "value",
//	}
//	rt := newHeaderRoundTripper(headers, transport)
//	client := &http.Client{Transport: rt}
type headerRoundTripper struct {
	base    http.RoundTripper
	headers []headerPair
}

// newHeaderRoundTripper 创建一个新的 headerRoundTripper。
//
// # 参数说明
//
//   - headers: 要添加的 HTTP 头 map
//   - base: 基础 Transport，为 nil 时使用 http.DefaultTransport
//
// # 返回值
//
// http.RoundTripper: 包装后的 RoundTripper
//
// # 优化逻辑
//
//   - 如果 headers 为空，直接返回 base Transport，避免不必要的包装
//   - 跳过空 key 的 header（无效的 HTTP 头）
//
// # 使用示例
//
//	headers := map[string]string{
//	    "Authorization": "Bearer token123",
//	    "X-Client-ID":   "my-client",
//	}
//	rt := newHeaderRoundTripper(headers, nil) // 使用默认 Transport
//	client := &http.Client{Transport: rt}
func newHeaderRoundTripper(headers map[string]string, base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	// 转换为切片，保持顺序
	pairs := make([]headerPair, 0, len(headers))
	for k, v := range headers {
		if k == "" {
			// 跳过空 key
			continue
		}
		pairs = append(pairs, headerPair{key: k, value: v})
	}
	// 如果没有有效的 headers，直接返回基础 Transport
	if len(pairs) == 0 {
		return base
	}
	return &headerRoundTripper{
		base:    base,
		headers: pairs,
	}
}

// RoundTrip 实现 http.RoundTripper 接口。
//
// # 作用说明
//
// 在转发请求前，添加所有配置的 headers。
//
// # 参数说明
//
//   - req: HTTP 请求
//
// # 返回值
//
//   - *http.Response: HTTP 响应
//   - error: 请求错误
//
// # 执行流程
//
// 1. 遍历所有 header pairs
// 2. 使用 req.Header.Set() 设置每个 header（会覆盖已有值）
// 3. 调用 base.RoundTrip() 转发请求
func (rt *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for _, kv := range rt.headers {
		req.Header.Set(kv.key, kv.value)
	}
	return rt.base.RoundTrip(req)
}
