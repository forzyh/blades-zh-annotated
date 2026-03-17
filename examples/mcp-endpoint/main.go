// Blades 示例：MCP HTTP 端点工具（mcp-endpoint）
//
// 本示例演示如何通过 HTTP 端点连接 MCP（Model Context Protocol）服务器，
// 让 Agent 能够使用外部工具和服务。MCP 是一个开放协议，允许 LLM 应用
// 安全地连接到各种数据源和工具。
//
// 适用场景：
// - 连接外部 API 和服务
// - 访问数据库和文件系统
// - 集成第三方工具（如时间、日历、天气等）
// - 构建可扩展的 Agent 工具生态系统
//
// 核心概念：
// 1. MCP Server：提供工具实现的服务端
// 2. MCP Client：Blades 中的工具解析器，连接到 MCP 服务器
// 3. Tools Resolver：动态解析和提供工具给 Agent
//
// 使用方法：
// 1. 启动 MCP 服务器（如 time server）
// 2. go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/mcp"
	"github.com/go-kratos/blades/contrib/openai"
)

func main() {
	// 步骤 1: 创建 MCP 工具解析器（HTTP 端点模式）
	// mcp.NewToolsResolver 创建一个能够连接 MCP 服务器的工具解析器
	// ClientConfig 配置 MCP 客户端连接参数：
	// - Name: 客户端名称，用于标识连接
	// - Transport: 传输方式，这里使用 TransportHTTP 通过 HTTP 连接
	// - Endpoint: MCP 服务器的 HTTP 端点 URL
	//
	// 这里连接的是一个 time MCP 服务器，它提供时间查询工具
	// 参考：https://github.com/modelcontextprotocol/servers/tree/main/src/time
	mcpResolver, err := mcp.NewToolsResolver(
		mcp.ClientConfig{
			Name:      "github",
			Transport: mcp.TransportHTTP,
			Endpoint:  "http://localhost:8000/mcp/time",
		},
	)
	if err != nil {
		log.Fatalf("Failed to create MCP tools resolver: %v", err)
	}
	defer mcpResolver.Close() // 确保程序退出时关闭连接

	// 步骤 2: 创建 OpenAI 模型提供者
	// openai.NewModel 创建一个实现 blades.ModelProvider 接口的模型实例
	// 需要从环境变量中获取模型名称和 API 密钥
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 3: 创建 Agent 并绑定 MCP 工具解析器
	// WithToolsResolver 告诉 Agent 可以从 MCP 服务器动态获取工具
	// Agent 会自动发现并调用可用的 MCP 工具
	agent, err := blades.NewAgent("time-assistant",
		blades.WithModel(model),
		blades.WithInstruction("You are a helpful assistant that can tell time in different timezones."),
		blades.WithToolsResolver(mcpResolver), // 绑定 MCP 工具解析器
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建输入消息
	// 询问当前时间，Agent 会自动调用 MCP 服务器的时间工具
	input := blades.UserMessage("What time is it right now?")

	fmt.Println("Asking agent: What time is it right now?")
	fmt.Println("--------------------------------------------------")

	// 步骤 5: 创建 Runner 并执行
	ctx := context.Background()
	runner := blades.NewRunner(agent)
	output, err := runner.Run(ctx, input)
	if err != nil {
		log.Fatalf("Agent run failed: %v", err)
	}

	// 步骤 6: 输出结果
	fmt.Printf("Agent: %s\n", output.Text())

	// 扩展提示：
	// 1. 其他 MCP 服务器示例：
	//    - 文件系统：mcp-servers/fs
	//    - PostgreSQL：mcp-servers/postgres
	//    - Slack：mcp-servers/slack
	//    - GitHub：mcp-servers/github
	//
	// 2. 使用 stdio 传输方式（见 mcp-stdio/main.go 示例）
	//
	// 3. 多个 MCP 服务器：
	//    可以创建多个 mcpResolver 并绑定到同一个 Agent
}
