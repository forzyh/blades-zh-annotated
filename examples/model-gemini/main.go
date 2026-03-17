// Blades 示例：Gemini 模型（model-gemini）
//
// 本示例演示如何使用 Google 的 Gemini 模型。
// Gemini 是 Google 开发的多模态大语言模型系列，支持文本、图像、音频等多种输入。
//
// 适用场景：
// - 多模态任务（文本 + 图像理解）
// - 需要 Google AI 生态集成的应用
// - 代码生成和理解
// - 多语言支持
//
// 核心概念：
// 1. Gemini Model：通过 Google AI API 访问 Gemini 模型
// 2. 工具集成：使用 tools.NewFunc 创建函数工具
// 3. 会话状态：在会话中保存和共享数据
//
// 使用方法：
// go run main.go
// 注意：需要设置 GOOGLE_API_KEY 和 GEMINI_MODEL 环境变量
package main

import (
	"context"
	"log"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/gemini"
	"github.com/go-kratos/blades/tools"
	"google.golang.org/genai"
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
	session.SetState("location", req.Location)

	// 返回模拟的天气数据
	// 实际应用中应该调用真实的天气 API
	return WeatherRes{Forecast: "Sunny, 25°C"}, nil
}

func main() {
	// 步骤 1: 从环境变量读取配置
	apiKey := os.Getenv("GOOGLE_API_KEY")
	modelName := os.Getenv("GEMINI_MODEL")

	// 步骤 2: 创建天气工具
	// tools.NewFunc 根据 Go 函数创建一个工具
	weatherTool, err := tools.NewFunc(
		"get_weather",
		"Get the current weather for a given city",
		weatherHandle,
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 Gemini 模型
	// gemini.NewModel 创建一个实现 blades.ModelProvider 接口的模型实例
	// 参数：
	//   - ctx: 上下文，用于初始化客户端
	//   - modelName: 模型名称（如 gemini-1.5-pro, gemini-1.5-flash）
	//   - Config: 模型配置
	//     - ClientConfig: Google AI 客户端配置
	//       - APIKey: API 密钥
	//       - Backend: 后端类型（geminiAPI 表示使用 Google AI API）
	//     - MaxOutputTokens: 最大输出 token 数
	//     - Temperature: 采样温度（0.0-1.0）
	ctx := context.Background()
	model, err := gemini.NewModel(ctx, modelName, gemini.Config{
		ClientConfig: genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		},
		MaxOutputTokens: 1024,
		Temperature:     0.7,
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// 步骤 4: 创建 Agent
	agent, err := blades.NewAgent(
		"Weather Agent",
		blades.WithModel(model), // 使用 Gemini 模型
		blades.WithInstruction("You are a helpful assistant that provides weather information."),
		blades.WithTools(weatherTool), // 绑定天气工具
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 5: 创建用户输入
	input := blades.UserMessage("What is the weather in New York City?")

	// 步骤 6: 创建会话
	session := blades.NewSession()

	// 步骤 7: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	output, err := runner.Run(ctx, input, blades.WithSession(session))
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 8: 输出结果
	log.Println("state:", session.State())
	log.Println("output:", output.Text())

	// 预期输出：
	// state: map[location:New York City]
	// output: The current weather in New York City is: Sunny, 25°C
	//
	// 扩展提示：
	// 1. 常用 Gemini 模型：
	//    - gemini-1.5-pro: 高性能，适合复杂任务
	//    - gemini-1.5-flash: 快速、低成本
	//    - gemini-2.0-flash: 最新快速模型
	//
	// 2. Gemini 的特性：
	//    - 原生支持 1M+ tokens 上下文窗口
	//    - 多模态输入（文本、图像、音频、视频）
	//    - 原生工具调用支持
	//
	// 3. 后端选项：
	//    - genai.BackendGeminiAPI: 使用 Google AI API
	//    - genai.BackendVertexAI: 使用 Google Cloud Vertex AI
}
