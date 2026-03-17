// Blades 示例：基础 Recipe（recipe-basic）
//
// 本示例演示如何使用 Blades 的 Recipe（配方）系统声明式地定义 Agent。
// Recipe 是一种 YAML 配置方式，允许在不修改代码的情况下配置 Agent 的行为、
// 工具和中间件，适合配置驱动的应用场景。
//
// 适用场景：
// - 配置驱动的 Agent 应用
// - 多环境部署（开发/测试/生产）
// - 非开发者配置 Agent（产品/运营人员）
// - 版本化管理 Agent 配置
//
// 核心概念：
// 1. Recipe（配方）：YAML 格式的 Agent 配置
// 2. ModelRegistry（模型注册表）：注册和管理可用的模型
// 3. MiddlewareRegistry（中间件注册表）：注册可用的中间件
// 4. Build：根据 Recipe 构建 Agent
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

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/middleware"
	"github.com/go-kratos/blades/recipe"
)

func main() {
	// 步骤 1: 创建模型注册表并注册模型
	// ModelRegistry 允许在 YAML 中通过名称引用已注册的模型
	registry := recipe.NewModelRegistry()
	registry.Register("gpt-4o", openai.NewModel("gpt-4o", openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	}))
	// YAML 中可以使用 model: gpt-4o 来引用这个模型

	// 步骤 2: 创建中间件注册表并注册中间件
	// MiddlewareRegistry 允许在 YAML 中通过名称引用已注册的中间件
	// 这里注册了一个 "retry" 中间件，支持动态配置重试次数
	mwRegistry := recipe.NewMiddlewareRegistry()
	mwRegistry.Register("retry", func(opts map[string]any) (blades.Middleware, error) {
		// 从配置中读取重试次数，默认为 3
		attempts := 3
		if v, ok := opts["attempts"].(int); ok && v > 0 {
			attempts = v
		}
		return middleware.Retry(attempts), nil
	})
	// YAML 中可以使用 middlewares: [retry] 来引用这个中间件

	// 步骤 3: 从 YAML 文件加载 Recipe
	// agent.yaml 示例结构：
	// ---
	// name: CodeReviewer
	// description: A code review assistant
	// model: gpt-4o
	// instruction: |
	//   You are a code reviewer. Review the code and provide feedback.
	// context:
	//   type: window
	//   max_messages: 10
	// middlewares:
	//   - name: retry
	//     options:
	//       attempts: 3
	spec, err := recipe.LoadFromFile("agent.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 根据 Recipe 构建 Agent
	// Build 方法会：
	// 1. 从 ModelRegistry 解析 model 字段
	// 2. 从 MiddlewareRegistry 解析 middlewares 字段
	// 3. 应用 context 配置（如 window/summary）
	// 4. 渲染 instruction 模板（使用 WithParams 传递的参数）
	agent, err := recipe.Build(spec,
		recipe.WithModelRegistry(registry),    // 绑定模型注册表
		recipe.WithMiddlewareRegistry(mwRegistry), // 绑定中间件注册表
		recipe.WithParams(map[string]any{"language": "go"}), // 模板参数
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 5: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	output, err := runner.Run(context.Background(), blades.UserMessage(`
		Review this code:
		func add(a, b int) int {
			return a - b
		}
	`))
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 6: 输出结果
	log.Println(output.Text())

	// 预期输出：
	// Agent 会指出代码中的 bug（应该是 a + b 而不是 a - b）
	//
	// 扩展提示：
	// 1. agent.yaml 完整示例：
	//    ---
	//    name: CodeReviewer
	//    description: A code review assistant
	//    model: gpt-4o
	//    instruction: |
	//      You are a {{.language}} code reviewer.
	//    context:
	//      type: window
	//      max_messages: 10
	//    middlewares:
	//      - name: retry
	//        options:
	//          attempts: 3
	//
	// 2. Recipe 模式：
	//    - basic: 基础 Agent，单一模型
	//    - sequential: 顺序执行多个 Agent
	//    - tool: 支持工具调用的 Agent
	//
	// 3. 热重载配置：
	//    可以定期重新加载 YAML 文件，实现配置热更新
	//    无需重启服务即可修改 Agent 行为
}
