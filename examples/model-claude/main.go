// Blades 示例：Claude 模型（model-claude）
//
// 本示例演示如何使用 Anthropic 的 Claude 模型。
// Claude 是 Anthropic 公司开发的一系列大语言模型，以安全性和有用性强著称。
//
// 适用场景：
// - 需要更长上下文窗口的场景（Claude 支持 100K+ tokens）
// - 需要更高安全性的应用
// - 复杂推理和代码生成
// - 多轮对话和长文档分析
//
// 核心概念：
// 1. Anthropic Model：通过 Anthropic API 访问 Claude 模型
// 2. 工具集成：使用 tools 包创建函数工具
// 3. Session 状态：在会话中保存和共享状态
//
// 使用方法：
// go run main.go
// 注意：需要设置 ANTHROPIC_BASE_URL、ANTHROPIC_AUTH_TOKEN 和 ANTHROPIC_DEFAULT_HAIKU_MODEL 环境变量
package main

import (
	"context"
	"log"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/anthropic"
	"github.com/go-kratos/blades/tools"
)

// WeatherReq 表示天气查询请求结构
// 使用 json 标签定义 JSON 字段名
// 使用 jsonschema 标签定义字段的 JSON Schema 描述
type WeatherReq struct {
	Location string `json:"location" jsonschema:"Get the current weather for a given city"`
}

// WeatherRes 表示天气查询响应结构
type WeatherRes struct {
	Forecast string `json:"forecast" jsonschema:"The weather forecast"`
}

// weatherHandle 是天气查询工具的处理函数
// 参数：
//   - ctx: 上下文，可以通过 blades.FromSessionContext 获取会话
//   - req: 天气查询请求参数
// 返回值：
//   - WeatherRes: 天气预报告
//   - error: 可能的错误
func weatherHandle(ctx context.Context, req WeatherReq) (WeatherRes, error) {
	// 记录日志
	log.Println("Fetching weather for:", req.Location)

	// 从上下文中获取会话
	// blades.FromSessionContext 返回 (Session, ok)
	// 会话允许在多次工具调用间保持状态
	session, ok := blades.FromSessionContext(ctx)
	if !ok {
		// 如果没有找到会话，返回错误
		return WeatherRes{}, blades.ErrNoSessionContext
	}

	// 在会话中保存位置信息
	// 这可以用于后续的对话或工具调用
	session.SetState("location", req.Location)

	// 返回模拟的天气数据
	// 实际应用中应该调用真实的天气 API
	return WeatherRes{Forecast: "Sunny, 25°C"}, nil
}

func main() {
	// 步骤 1: 从环境变量读取配置
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	authToken := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	modelName := os.Getenv("ANTHROPIC_DEFAULT_HAIKU_MODEL")

	// 步骤 2: 创建天气工具
	// tools.NewFunc 根据 Go 函数创建一个工具
	// 参数：
	//   - 工具名称：get_weather
	//   - 工具描述：告诉 LLM 这个工具的用途
	//   - 处理函数：weatherHandle
	weatherTool, err := tools.NewFunc(
		"get_weather",
		"Get the current weather for a given city",
		weatherHandle,
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 Claude 模型
	// anthropic.NewModel 创建一个实现 blades.ModelProvider 接口的模型实例
	// Config 配置：
	//   - BaseURL: Anthropic API 的基础 URL（可以是代理）
	//   - APIKey: 认证令牌
	//   - MaxOutputTokens: 最大输出 token 数
	//   - Temperature: 采样温度（0.0-1.0，越高越随机）
	model := anthropic.NewModel(modelName, anthropic.Config{
		BaseURL:         baseURL,
		APIKey:          authToken,
		MaxOutputTokens: 1024,
		Temperature:     0.7,
	})

	// 步骤 4: 创建 Agent
	// Agent 是 blades 的核心组件，负责与 LLM 交互
	agent, err := blades.NewAgent(
		"Weather Agent",
		blades.WithModel(model), // 使用 Claude 模型
		blades.WithInstruction("You are a helpful assistant that provides weather information."),
		blades.WithTools(weatherTool), // 绑定天气工具
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 5: 创建用户输入
	// 询问纽约市的天气
	input := blades.UserMessage("What is the weather in New York City?")

	// 步骤 6: 创建会话
	// Session 用于在多次调用间保持状态
	ctx := context.Background()
	session := blades.NewSession()

	// 步骤 7: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	output, err := runner.Run(ctx, input, blades.WithSession(session))
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 8: 输出结果
	// 显示会话状态（包含保存的位置信息）
	log.Println("state:", session.State())
	// 显示 Agent 的回答
	log.Println("output:", output.Text())

	// 预期输出：
	// state: map[location:New York City]
	// output: The current weather in New York City is: Sunny, 25°C
	//
	// 扩展提示：
	// 1. 常用 Claude 模型：
	//    - claude-3-5-sonnet: 平衡性能和速度
	//    - claude-3-5-haiku: 快速、轻量
	//    - claude-3-opus: 最强性能，适合复杂任务
	//
	// 2. 工具调用最佳实践：
	//    - 提供清晰的工具描述
	//    - 使用准确的参数类型
	//    - 在指令中说明何时使用工具
	//
	// 3. 会话管理：
	//    - 使用 session.SetState 保存状态
	//    - 使用 session.GetState 读取状态
	//    - 会话可以在多次 Run 调用间保持
}
