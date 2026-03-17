// Blades 示例：工具 Recipe（recipe-tool）
//
// 本示例演示如何使用 Recipe 定义支持工具调用的 Agent。
// Tool 模式允许 Agent 使用预定义的函数工具（Function Tools）
// 和子 Agent 工具（Agent Tools），实现复杂任务的处理。
//
// 适用场景：
// - 需要调用外部 API 的 Agent
// - 多 Agent 协作系统
// - 工具库复用于多个 Agent
// - 配置化 Agent 工具管理
//
// 核心概念：
// 1. Tool Recipe：支持工具调用的 Agent 配置
// 2. Function Tool：Go 函数封装成的工具
// 3. Agent Tool：将整个 Agent 作为工具
// 4. ToolRegistry：工具注册表，管理可用工具
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_API_KEY 环境变量
//       确保当前目录存在 agent.yaml 文件
package main

import (
	"context"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/recipe"
	"github.com/go-kratos/blades/tools"
)

// ==================== 工具 1: 提取邮箱（基于正则） ====================

// ExtractEmailsReq 是提取邮箱工具的请求结构
type ExtractEmailsReq struct {
	Text string `json:"text" jsonschema:"The text to extract email addresses from"`
}

// ExtractEmailsRes 是提取邮箱工具的响应结构
type ExtractEmailsRes struct {
	Matches []string `json:"matches" jsonschema:"The extracted email addresses"`
}

// 编译邮箱正则表达式
// 这个正则可以匹配大多数常见邮箱格式
var emailPattern = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

// extractEmails 是提取邮箱工具的处理函数
// 它从文本中提取所有邮箱地址
func extractEmails(_ context.Context, req ExtractEmailsReq) (ExtractEmailsRes, error) {
	// 使用正则查找所有匹配的邮箱
	matches := emailPattern.FindAllString(req.Text, -1)
	if matches == nil {
		matches = []string{} // 返回空切片而不是 nil
	}
	return ExtractEmailsRes{Matches: matches}, nil
}

// ==================== 工具 2: 查询天气（简单查找） ====================

// GetWeatherReq 是天气查询工具的请求结构
type GetWeatherReq struct {
	City string `json:"city" jsonschema:"The city name to get weather for"`
}

// GetWeatherRes 是天气查询工具的响应结构
type GetWeatherRes struct {
	City     string `json:"city" jsonschema:"The city name"`
	Forecast string `json:"forecast" jsonschema:"The weather forecast"`
}

// getWeather 是天气查询工具的处理函数
// 这里使用模拟数据，实际应用中应该调用天气 API
func getWeather(_ context.Context, req GetWeatherReq) (GetWeatherRes, error) {
	// 模拟天气数据
	forecasts := map[string]string{
		"new york":  "Partly cloudy, 18°C",
		"london":    "Rainy, 12°C",
		"tokyo":     "Sunny, 22°C",
		"beijing":   "Hazy, 15°C",
		"singapore": "Thunderstorms, 30°C",
	}
	// 查找天气，如果城市不存在返回提示
	forecast, ok := forecasts[strings.ToLower(req.City)]
	if !ok {
		forecast = fmt.Sprintf("No data available for %s", req.City)
	}
	return GetWeatherRes{City: req.City, Forecast: forecast}, nil
}

func main() {
	// 步骤 1: 使用 tools.NewFunc 创建函数工具
	// 参数：
	//   - 工具名称：extract-emails
	//   - 工具描述：告诉 LLM 这个工具的用途
	//   - 处理函数：extractEmails
	emailTool, err := tools.NewFunc("extract-emails", "Extract email addresses from text using regex", extractEmails)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 2: 创建第二个工具（天气查询）
	weatherTool, err := tools.NewFunc("get-weather", "Get the current weather for a city", getWeather)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建工具注册表并注册工具
	// ToolRegistry 允许在 YAML 中通过名称引用已注册的工具
	toolRegistry := recipe.NewToolRegistry()
	toolRegistry.Register("extract-emails", emailTool)
	toolRegistry.Register("get-weather", weatherTool)

	// 步骤 4: 创建模型注册表并注册模型
	modelRegistry := recipe.NewModelRegistry()
	modelRegistry.Register("gpt-4o", openai.NewModel("gpt-4o", openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	}))

	// 步骤 5: 从 YAML 文件加载 Recipe
	// agent.yaml 定义了工具模式的 Agent
	// 示例结构：
	// ---
	// mode: tool
	// name: ResearchAssistant
	// model: gpt-4o
	// instruction: |
	//   You are a research assistant. Use the available tools to gather information.
	// tools:
	//   - extract-emails
	//   - get-weather
	//   - name: sub-agent
	//     agent:
	//       name: Summarizer
	//       instruction: Summarize the given text.
	spec, err := recipe.LoadFromFile("agent.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 6: 根据 Recipe 构建 Agent
	// Build 方法会：
	// 1. 从 ToolRegistry 解析 tools 字段
	// 2. 创建子 Agent 工具（如果定义）
	// 3. 配置工具调用逻辑
	agent, err := recipe.Build(spec,
		recipe.WithModelRegistry(modelRegistry),
		recipe.WithToolRegistry(toolRegistry),
		recipe.WithParams(map[string]any{"topic": "climate change impact on agriculture"}),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 7: 创建 Runner 并使用流式输出
	runner := blades.NewRunner(agent)
	stream := runner.RunStream(context.Background(), blades.UserMessage(
		"Research the topic and provide a summary with verified facts and data analysis.",
	))

	// 步骤 8: 遍历流式响应
	for message, err := range stream {
		if err != nil {
			log.Fatal(err)
		}
		// 只显示有文本内容的消息
		if text := message.Text(); text != "" {
			log.Printf("[%s] %s\n", message.Author, text)
		}
	}

	// 预期输出：
	// Agent 会使用注册的工具来收集信息，然后提供研究摘要
	//
	// 扩展提示：
	// 1. agent.yaml 完整示例（工具模式）：
	//    ---
	//    mode: tool
	//    name: DataAnalyst
	//    model: gpt-4o
	//    instruction: |
	//      You are a data analyst. Use tools to gather insights.
	//    tools:
	//      - extract-emails
	//      - get-weather
	//      - name: data-processor
	//        agent:
	//          name: Processor
	//          model: gpt-4o
	//          instruction: Process and analyze the given data.
	//
	// 2. 工具调用流程：
	//    - LLM 决定调用哪个工具
	//    - Blades 执行工具函数
	//    - 工具返回结果给 LLM
	//    - LLM 根据结果生成最终回答
	//
	// 3. 工具最佳实践：
	//    - 提供清晰的工具描述
	//    - 使用准确的参数类型和 Schema
	//    - 处理边界情况和错误
	//    - 记录工具调用日志
}
