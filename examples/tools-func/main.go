// Blades 示例：函数工具（tools-func）
//
// 本示例演示如何使用 tools.NewFunc 创建类型安全的函数工具。
// 函数工具允许 LLM 调用 Go 函数来执行实际操作，
// 如查询数据库、调用 API、计算数据等。
//
// 适用场景：
// - 集成外部 API 和服务
// - 执行代码计算和操作
// - 访问数据库和文件系统
// - 扩展 Agent 能力边界
//
// 核心概念：
// 1. NewFunc：从 Go 函数创建工具
// 2. 类型安全：使用 Go 类型定义请求/响应结构
// 3. Session 状态：在工具中访问和修改会话状态
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"log"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/tools"
)

// WeatherReq 表示天气查询请求结构
// 这个结构体会被自动转换为 JSON Schema
// 用于告诉 LLM 如何调用这个工具
type WeatherReq struct {
	// Location 字段：查询位置
	// json 标签定义 JSON 字段名
	// jsonschema 标签描述字段的用途，帮助 LLM 理解
	Location string `json:"location" jsonschema:"Get the current weather for a given city"`
}

// WeatherRes 表示天气查询响应结构
type WeatherRes struct {
	// Forecast 字段：天气预报
	Forecast string `json:"forecast" jsonschema:"The weather forecast"`
}

// weatherHandle 是天气查询工具的处理函数
// 这是一个类型安全的 Go 函数，LLM 通过 JSON 调用它
// 参数：
//   - ctx: 上下文，可以获取会话等信息
//   - req: 天气查询请求参数（已自动解析）
// 返回值：
//   - WeatherRes: 天气预报告
//   - error: 可能的错误
func weatherHandle(ctx context.Context, req WeatherReq) (WeatherRes, error) {
	// 记录日志
	log.Println("Fetching weather for:", req.Location)

	// 从上下文中获取会话
	// blades.FromSessionContext 返回 (Session, ok)
	// 会话允许在工具调用间保持状态
	session, ok := blades.FromSessionContext(ctx)
	if !ok {
		// 如果没有找到会话，返回错误
		return WeatherRes{}, blades.ErrNoSessionContext
	}

	// 在会话中保存位置信息
	// 这可以用于后续对话或其他工具
	session.SetState("location", req.Location)

	// 返回模拟的天气数据
	// 实际应用中应该调用真实的天气 API（如 OpenWeatherMap）
	return WeatherRes{Forecast: "Sunny, 25°C"}, nil
}

func main() {
	// 步骤 1: 创建天气工具
	// tools.NewFunc 根据 Go 函数创建一个工具
	// 参数：
	//   - 工具名称：get_weather（LLM 使用这个名字调用工具）
	//   - 工具描述：告诉 LLM 这个工具的用途
	//   - 处理函数：weatherHandle（实际执行的 Go 函数）
	weatherTool, err := tools.NewFunc(
		"get_weather",
		"Get the current weather for a given city",
		weatherHandle,
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 2: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 3: 创建 Agent 并绑定工具
	// WithTools 将工具注册为 Agent 可用的能力
	// Agent 会根据需要自动决定调用哪个工具
	agent, err := blades.NewAgent(
		"Weather Agent",
		blades.WithModel(model),
		blades.WithInstruction("You are a helpful assistant that provides weather information."),
		blades.WithTools(weatherTool), // 绑定天气工具
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建用户输入
	// 询问纽约市的天气
	input := blades.UserMessage("What is the weather in New York City?")

	// 步骤 5: 创建会话
	// Session 用于在工具调用间保持状态
	ctx := context.Background()
	session := blades.NewSession()

	// 步骤 6: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	output, err := runner.Run(ctx, input, blades.WithSession(session))
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 7: 输出结果
	// 显示会话状态（包含工具保存的位置信息）
	log.Println("state:", session.State())
	// 显示 Agent 的回答
	log.Println("output:", output.Text())

	// 预期输出：
	// state: map[location:New York City]
	// output: The current weather in New York City is: Sunny, 25°C
	//
	// 扩展提示：
	// 1. 多参数工具：
	//    type SearchReq struct {
	//        Query   string `json:"query"`
	//        Limit   int    `json:"limit"`
	//        Filters []string `json:"filters"`
	//    }
	//
	// 2. 错误处理：
	//    func handle(ctx context.Context, req Req) (Res, error) {
	//        if err := validate(req); err != nil {
	//            return Res{}, err // 错误会返回给 LLM
	//        }
	//        ...
	//    }
	//
	// 3. 调用外部 API：
	//    func getWeather(ctx context.Context, req WeatherReq) (WeatherRes, error) {
	//        resp, err := http.Get("https://api.weather.com/..." + req.Location)
	//        ...
	//    }
	//
	// 4. 工具组合：
	//    可以创建多个工具，让 LLM 选择使用
	//    agent, _ := blades.NewAgent("...",
	//        blades.WithTools(tool1, tool2, tool3),
	//    )
	//
	// 5. 工具中间件：
	//    可以为工具添加日志、限流、缓存等中间件
	//    参见 tools-middleware 示例
}
