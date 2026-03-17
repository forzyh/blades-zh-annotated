// Package recipe 提供基于 YAML 的声明式配方系统，用于定义和构建 blades.Agent 工作流
//
// 配方（Recipe）是一种 YAML 格式的配置文件，可以描述单个 Agent 或由多个子 Agent 组成的复杂工作流
// 通过配方，用户可以无需编写代码即可配置 Agent 的行为、工具、上下文管理等
//
// 配方的主要功能：
//   - 定义 Agent 的基本配置（名称、描述、模型、指令）
//   - 配置参数化模板，支持动态指令生成
//   - 定义多 Agent 工作流（顺序、并行、循环、工具模式）
//   - 配置上下文管理策略（摘要压缩、滑动窗口）
//   - 配置中间件（日志、追踪等）
//   - 配置外部工具和子 Agent
//
// 基本使用流程:
//
//	// 1. 注册模型提供者
//	registry := recipe.NewModelRegistry()
//	registry.Register("gpt-4o", myModelProvider)
//
//	// 2. 从 YAML 文件加载配方
//	spec, err := recipe.LoadFromFile("agent.yaml")
//
//	// 3. 构建 Agent（提供必要的注册表和参数）
//	agent, err := recipe.Build(spec,
//	    recipe.WithModelRegistry(registry),
//	    recipe.WithParams(map[string]any{"language": "go"}),
//	)
//
//	// 4. 使用 Runner 执行 Agent
//	runner := blades.NewRunner(agent)
//	output, err := runner.Run(ctx, blades.UserMessage("Review this code"))
package recipe
