// Blades 示例：MCP stdio 工具（mcp-stdio）
//
// 本示例演示如何通过 stdio（标准输入/输出）连接 MCP 服务器。
// 与 HTTP 端点模式不同，stdio 模式通过子进程启动 MCP 服务器，
// 适用于本地工具和命令行程序。
//
// 适用场景：
// - 运行本地 CLI 工具作为 MCP 服务器
// - 连接不需要网络访问的本地服务
// - 使用 npm/npx 安装的 MCP 服务器
// - 需要更高安全性的场景（进程隔离）
//
// 核心概念：
// 1. stdio 传输：通过子进程的 stdin/stdout 与 MCP 服务器通信
// 2. Command + Args：指定启动 MCP 服务器的命令和参数
// 3. 进程管理：MCP 客户端负责启动和停止子进程
//
// 使用方法：
// go run main.go
// 注意：需要安装 npx（Node.js 包管理器）
//       需要设置 OPENAI_MODEL、OPENAI_API_KEY 环境变量
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
	// 步骤 1: 创建 MCP 工具解析器（stdio 模式）
	// mcp.NewToolsResolver 创建一个能够连接 MCP 服务器的工具解析器
	// ClientConfig 配置 MCP 客户端连接参数：
	// - Name: 客户端名称，用于标识连接
	// - Transport: 传输方式，这里使用 TransportStdio 通过标准输入/输出连接
	// - Command: 启动 MCP 服务器的命令，这里是 npx（Node.js 包执行器）
	// - Args: 命令参数，-y 表示自动确认，@modelcontextprotocol/server-time 是服务器包名
	//
	// 这会启动一个子进程运行 time MCP 服务器
	// 参考：https://github.com/modelcontextprotocol/servers/tree/main/src/time
	mcpResolver, err := mcp.NewToolsResolver(
		mcp.ClientConfig{
			Name:      "time",
			Transport: mcp.TransportStdio,
			Command:   "npx",
			Args:      []string{"-y", "@modelcontextprotocol/server-time"},
		},
	)
	if err != nil {
		log.Fatalf("Failed to create MCP tools resolver: %v", err)
	}
	defer mcpResolver.Close() // 确保程序退出时关闭连接并终止子进程

	// 步骤 2: 创建 OpenAI 模型提供者
	// openai.NewModel 创建一个实现 blades.ModelProvider 接口的模型实例
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 3: 创建 Agent 并绑定 MCP 工具解析器
	// WithToolsResolver 告诉 Agent 可以从 MCP 服务器动态获取工具
	agent, err := blades.NewAgent("time-assistant",
		blades.WithModel(model),
		blades.WithInstruction("You are a helpful assistant that can tell time in different timezones."),
		blades.WithToolsResolver(mcpResolver), // 绑定 MCP 工具解析器
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 第一次询问 - 当前时间
	input := blades.UserMessage("What time is it right now?")

	fmt.Println("Asking agent: What time is it right now?")
	fmt.Println("--------------------------------------------------")

	ctx := context.Background()
	runner := blades.NewRunner(agent)
	output, err := runner.Run(ctx, input)
	if err != nil {
		log.Fatalf("Agent run failed: %v", err)
	}

	fmt.Printf("Agent: %s\n", output.Text())

	// 步骤 5: 第二次询问 - 特定时区的时间
	// 这演示了多轮对话中 MCP 工具的持续可用性
	input2 := blades.UserMessage("What time is it in Tokyo right now?")

	fmt.Println("\n--------------------------------------------------")
	fmt.Println("Asking agent: What time is it in Tokyo right now?")
	fmt.Println("--------------------------------------------------")

	// 创建新的 Runner（也可以复用同一个）
	runner2 := blades.NewRunner(agent)
	output2, err := runner2.Run(ctx, input2)
	if err != nil {
		log.Fatalf("Agent run failed: %v", err)
	}
	fmt.Printf("Agent: %s\n", output2.Text())

	// 扩展提示：
	// 1. 其他可用的 MCP 服务器（通过 npx 安装）：
	//    - @modelcontextprotocol/server-filesystem：文件系统访问
	//    - @modelcontextprotocol/server-postgres：PostgreSQL 数据库
	//    - @modelcontextprotocol/server-slack：Slack 集成
	//
	// 2. stdio vs HTTP 传输方式对比：
	//    | 特性 | stdio | HTTP |
	//    |------|-------|------|
	//    | 部署 | 本地安装 | 远程服务 |
	//    | 安全 | 进程隔离 | 网络安全 |
	//    | 性能 | 无网络开销 | 有网络延迟 |
	//    | 适用 | 本地工具 | 云服务 |
	//
	// 3. 自定义 MCP 服务器：
	//    可以编写自己的 MCP 服务器并通过 stdio 或 HTTP 提供
}
