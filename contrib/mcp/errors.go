// Package mcp 提供了 MCP（Model Context Protocol）客户端实现。
//
// # MCP 是什么
//
// MCP（Model Context Protocol）是一个开放协议，用于标准化 AI 模型与外部工具/资源的交互方式。
// 它定义了一套通用的 JSON-RPC 2.0 接口，使 AI 模型能够发现、调用各种工具和服务。
package mcp

import "errors"

var (
	// ErrNotConnected 表示客户端未连接到服务器。
	// 在调用需要连接的方法（如 CallTool、ListTools）前，必须先调用 Connect()。
	ErrNotConnected = errors.New("mcp: not connected")

	// ErrToolNotFound 表示请求的工具不存在。
	// 当指定的工具名称在服务器上找不到时返回此错误。
	ErrToolNotFound = errors.New("mcp: tool not found")

	// ErrInvalidResponse 表示服务器返回了无效的响应。
	// 可能是响应格式错误、缺少必要字段等。
	ErrInvalidResponse = errors.New("mcp: invalid response")

	// ErrTransportFailed 表示传输层失败。
	// 可能是网络连接断开、子进程崩溃等原因。
	ErrTransportFailed = errors.New("mcp: transport failed")
)
