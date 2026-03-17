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
	"encoding/json"
	"fmt"

	"github.com/go-kratos/blades/tools"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// toBladesTool 将 MCP 工具转换为 Blades 工具。
//
// # 作用说明
//
// 此函数是 MCP 与 Blades 框架集成的核心，负责将 MCP 服务器的工具定义
// 转换为 blades/tools 包定义的 Tool 接口，使 Blades 框架能够统一处理
// 各种来源的工具（MCP、自定义工具等）。
//
// # 参数说明
//
//   - mcpTool: MCP 服务器的工具定义，包含 Name、Description、InputSchema 等
//   - handler: 工具执行函数，由 client.handler() 提供，负责实际调用 MCP 工具
//
// # 返回值
//
//   - tools.Tool: Blades 框架的工具接口实现
//   - error: 转换错误，通常在 Schema 转换失败时返回
//
// # 转换过程
//
// 1. 转换输入 Schema：将 MCP 的 JSON Schema 转换为 Blades 的 jsonschema.Schema
// 2. 转换输出 Schema（如果有）：处理工具返回值的 Schema
// 3. 使用 tools.NewTool 创建工具实例，注册处理函数
//
// # 使用场景
//
// 此方法主要由 Provider 或 ToolsResolver 在加载 MCP 工具时调用，
// 不需要直接创建独立的 Adapter 实例。
func toBladesTool(mcpTool *mcp.Tool, handler tools.HandleFunc) (tools.Tool, error) {
	// 转换输入 Schema
	inputSchema, err := convertSchema(mcpTool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to convert input schema: %w", err)
	}
	// 转换输出 Schema（如果存在）
	var outputSchema *jsonschema.Schema
	if mcpTool.OutputSchema != nil {
		outputSchema, err = convertSchema(mcpTool.OutputSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to convert output schema: %w", err)
		}
	}
	// 创建 Blades 工具
	return tools.NewTool(
		mcpTool.Name,
		mcpTool.Description,
		handler,
		tools.WithInputSchema(inputSchema),
		tools.WithOutputSchema(outputSchema),
	), nil
}

// convertSchema 将 MCP Schema 转换为 Blades jsonschema.Schema。
//
// # 作用说明
//
// MCP 工具使用 JSON Schema 定义输入/输出格式，但 MCP SDK 使用 interface{}
// 表示 Schema，而 Blades 使用 jsonschema.Schema 类型。此函数负责类型转换。
//
// # 参数说明
//
//   - mcpSchema: MCP 的 Schema，类型为 interface{}（实际是 map[string]interface{}）
//
// # 返回值
//
//   - *jsonschema.Schema: Blades 的 Schema 指针
//   - error: 转换错误
//
// # 转换步骤
//
// 1. 如果 Schema 为 nil，返回空的 object Schema（MCP 允许无 Schema 的工具）
// 2. 将 interface{} 序列化为 JSON 字节
// 3. 反序列化为 jsonschema.Schema
//
// # 为什么使用序列化/反序列化方式
//
// 因为 MCP SDK 的 Schema 类型是 interface{}，而 jsonschema.Schema 是
// 具体结构体。直接类型转换不可行，通过 JSON 作为中间格式可以：
//   - 保留所有 Schema 字段
//   - 处理嵌套结构
//   - 避免手动字段映射
func convertSchema(mcpSchema any) (*jsonschema.Schema, error) {
	if mcpSchema == nil {
		// 如果没有 Schema，返回空的 object Schema
		// 这表示工具接受任意输入
		return &jsonschema.Schema{
			Type: "object",
		}, nil
	}

	// 将 interface{} 序列化为 JSON
	schemaBytes, err := json.Marshal(mcpSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	// 反序列化为 jsonschema.Schema
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	return &schema, nil
}

// formatToolResult 将 MCP 工具调用结果转换为 JSON 字符串。
//
// # 作用说明
//
// MCP 工具执行后返回 CallToolResult，包含 Content 或 StructuredContent。
// 此函数将结果格式化为 JSON 字符串，供 Blades 框架使用。
//
// # 参数说明
//
//   - result: MCP 工具调用结果
//
// # 返回值
//
//   - string: JSON 格式的结果
//   - error: 格式化错误，工具执行失败时也返回错误
//
// # 处理逻辑
//
// 1. 检查 IsError 标志：如果工具执行失败，提取错误信息并返回错误
// 2. 优先使用 StructuredContent：如果存在，序列化为 JSON
// 3. 回退到 Content：将 Content 列表序列化为 JSON
//
// # 错误处理
//
// 当 result.IsError 为 true 时：
//   - 尝试从 Content 中提取文本错误信息
//   - 如果无法提取，返回通用错误消息
func formatToolResult(result *mcp.CallToolResult) (string, error) {
	// 检查工具执行是否失败
	if result.IsError {
		// 尝试从 Content 中提取错误信息
		if len(result.Content) > 0 {
			var errorMsg string
			for _, content := range result.Content {
				if textContent, ok := content.(*mcp.TextContent); ok {
					errorMsg += textContent.Text
				}
			}
			if errorMsg != "" {
				return "", fmt.Errorf("tool execution failed: %s", errorMsg)
			}
		}
		return "", fmt.Errorf("tool execution failed")
	}
	// 优先使用结构化内容
	if result.StructuredContent != nil {
		outputBytes, err := json.Marshal(result.StructuredContent)
		if err != nil {
			return "", fmt.Errorf("failed to marshal structured content: %w", err)
		}
		return string(outputBytes), nil
	}
	// 回退到 Content 列表
	outputBytes, err := json.Marshal(result.Content)
	if err != nil {
		return "", fmt.Errorf("failed to marshal content: %w", err)
	}
	return string(outputBytes), nil
}
