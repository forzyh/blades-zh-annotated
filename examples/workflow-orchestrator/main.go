// Blades 示例：编排器工作流（workflow-orchestrator）
//
// 本示例演示如何使用 Orchestrator（编排器）模式协调多个专业 Agent。
// Orchestrator 负责接收用户请求，分析需求，然后调用相应的专业 Agent
// 完成子任务，最后整合结果返回给用户。
//
// 适用场景：
// - 多技能助手（翻译、查询、分析等）
// - 任务分解和分配
// - 多服务协调
// - 复杂查询处理
//
// 核心概念：
// 1. Orchestrator：主控 Agent，负责任务协调
// 2. Worker Agents：专业 Agent，负责具体任务
// 3. Agent Tools：将 Agent 封装为工具供 Orchestrator 调用
// 4. Synthesizer：整合多个 Worker 的输出
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

// createTranslatorWorkers 创建翻译工作者 Agent 列表
// 每个 Agent 负责一种语言的翻译
// 返回 Agent 封装的工具列表
func createTranslatorWorkers(model blades.ModelProvider) []tools.Tool {
	// 西班牙语翻译 Agent
	spanishAgent, err := blades.NewAgent(
		"spanish_agent",
		blades.WithDescription("An English to Spanish translator"),
		blades.WithInstruction("You translate the user's message to Spanish"),
		blades.WithModel(model),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 法语翻译 Agent
	frenchAgent, err := blades.NewAgent(
		"french_agent",
		blades.WithDescription("An English to French translator"),
		blades.WithInstruction("You translate the user's message to French"),
		blades.WithModel(model),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 意大利语翻译 Agent
	italianAgent, err := blades.NewAgent(
		"italian_agent",
		blades.WithDescription("An English to Italian translator"),
		blades.WithInstruction("You translate the user's message to Italian"),
		blades.WithModel(model),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 将 Agent 封装为工具
	// blades.NewAgentTool 创建 Agent 工具，Orchestrator 可以调用它们
	return []tools.Tool{
		blades.NewAgentTool(spanishAgent),
		blades.NewAgentTool(frenchAgent),
		blades.NewAgentTool(italianAgent),
	}
}

func main() {
	// 步骤 1: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建翻译工作者工具
	translatorWorkers := createTranslatorWorkers(model)

	// 步骤 3: 创建 Orchestrator Agent（编排器）
	// Orchestrator 负责接收用户请求，调用合适的翻译工具
	orchestratorAgent, err := blades.NewAgent(
		"orchestrator_agent",
		blades.WithInstruction(`You are a translation agent. You use the tools given to you to translate.
If asked for multiple translations, you call the relevant tools in order.
You never translate on your own, you always use the provided tools.`),
		blades.WithModel(model),
		blades.WithTools(translatorWorkers...), // 绑定所有翻译工具
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建 Synthesizer Agent（整合器）
	// Synthesizer 负责整合多个翻译结果，生成最终响应
	synthesizerAgent, err := blades.NewAgent(
		"synthesizer_agent",
		blades.WithInstruction("You inspect translations, correct them if needed, and produce a final concatenated response."),
		blades.WithModel(model),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 5: 准备用户输入
	// 请求将句子翻译成三种语言
	ctx := context.Background()
	input := blades.UserMessage("Please translate the following sentence to Spanish, French, and Italian: 'Hello, how are you?'")

	// 步骤 6: 执行 Orchestrator
	orchestratorRunner := blades.NewRunner(orchestratorAgent)
	stream := orchestratorRunner.RunStream(ctx, input)
	var message *blades.Message

	// 遍历流式输出，获取最终结果
	for message, err = range stream {
		if err != nil {
			log.Fatal(err)
		}
	}
	// message.Text() 包含 Orchestrator 收集的翻译结果

	// 步骤 7: 执行 Synthesizer
	// 将 Orchestrator 的输出作为输入，生成最终响应
	synthesizerRunner := blades.NewRunner(synthesizerAgent)
	output, err := synthesizerRunner.Run(ctx, blades.UserMessage(message.Text()))
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 8: 输出最终结果
	log.Println("Final Output:", output.Text())

	// 预期流程：
	// 1. Orchestrator 接收请求
	// 2. 调用 spanish_agent 翻译为西班牙语
	// 3. 调用 french_agent 翻译为法语
	// 4. 调用 italian_agent 翻译为意大利语
	// 5. 整合所有翻译结果
	// 6. Synthesizer 检查并修正翻译
	// 7. 输出最终结果
	//
	// 输出示例：
	// Spanish: Hola, ¿cómo estás?
	// French: Bonjour, comment allez-vous ?
	// Italian: Ciao, come stai?
	//
	// 扩展提示：
	// 1. 动态工作者：
	//    根据需要动态创建/选择工作者
	//    workers := selectWorkers(request)
	//
	// 2. 并行执行：
	//    多个独立任务可以并行执行
	//    参见 workflow-parallel 示例
	//
	// 3. 错误处理：
	//    某个工作者失败时，可以：
	//    - 重试
	//    - 跳过
	//    - 使用备用工作者
	//
	// 4. 结果验证：
	//    添加验证 Agent 检查翻译质量
	//    validationAgent, _ := blades.NewAgent("Validator", ...)
	//
	// 5. 缓存优化：
	//    缓存常见翻译结果
	//    避免重复调用相同翻译
}
