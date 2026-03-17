// Blades 示例：路由工作流（workflow-router）
//
// 本示例演示如何使用 Routing Agent 实现智能路由。
// Routing Agent 分析用户请求，然后路由到最合适的专业 Agent
// 进行处理，适合多技能助手的场景。
//
// 适用场景：
// - 多领域问答系统
// - 客服工单分发
// - 专家系统选择
// - 多语言路由
//
// 核心概念：
// 1. Routing Agent：分析请求并选择目标 Agent
// 2. Sub-Agents：处理具体领域的专业 Agent
// 3. 动态路由：根据请求内容动态选择
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
	"github.com/go-kratos/blades/flow"
)

func main() {
	// 步骤 1: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建专业子 Agent

	// 数学辅导 Agent
	// 负责回答数学相关问题
	mathTutorAgent, err := blades.NewAgent(
		"MathTutor",
		blades.WithDescription("An agent that helps with math questions"),
		blades.WithInstruction("You are a helpful math tutor. Answer questions related to mathematics."),
		blades.WithModel(model),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 历史辅导 Agent
	// 负责回答历史相关问题
	historyTutorAgent, err := blades.NewAgent(
		"HistoryTutor",
		blades.WithDescription("An agent that helps with history questions"),
		blades.WithInstruction("You are a helpful history tutor. Answer questions related to history."),
		blades.WithModel(model),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 Routing Agent
	// Routing Agent 负责分析用户问题，路由到合适的辅导 Agent
	// flow.NewRoutingAgent 创建一个路由配置
	agent, err := flow.NewRoutingAgent(flow.RoutingConfig{
		Name:        "TriageAgent", // 路由 Agent 名称
		Description: "You determine which agent to use based on the user's homework question", // 描述
		Model:       model, // 使用的模型
		SubAgents: []blades.Agent{
			mathTutorAgent,    // 可路由到数学辅导
			historyTutorAgent, // 可路由到历史辅导
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建用户输入
	// 这个问题会被路由到 HistoryTutor
	input := blades.UserMessage("What is the capital of France?")

	// 步骤 5: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	output, err := runner.Run(context.Background(), input)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 6: 输出结果
	log.Println(output.Text())

	// 预期流程：
	// 1. Routing Agent 分析问题："What is the capital of France?"
	// 2. 判断这是历史/地理问题
	// 3. 路由到 HistoryTutor
	// 4. HistoryTutor 回答问题
	// 5. 输出："The capital of France is Paris."
	//
	// 如果问题是 "What is 2 + 2?"：
	// 1. Routing Agent 分析问题
	// 2. 判断这是数学问题
	// 3. 路由到 MathTutor
	// 4. MathTutor 回答问题
	//
	// 扩展提示：
	// 1. 更多专业 Agent：
	//    - ScienceTutor：科学问题
	//    - LanguageTutor：语言学习
	//    - CodingTutor：编程帮助
	//
	// 2. 自定义路由逻辑：
	//    可以实现自己的路由决策
	//    type CustomRouter struct { ... }
	//    func (r *CustomRouter) Route(req Request) Agent { ... }
	//
	// 3. 路由日志：
	//    记录路由决策便于调试和优化
	//    log.Printf("Routed %s to %s", request, agentName)
	//
	// 4. 回退机制：
	//    如果没有合适的 Agent，可以：
	//    - 返回默认 Agent
	//    - 返回错误
	//    - 转人工处理
	//
	// 5. 多级路由：
	//    第一级：学科（数学/历史/科学）
	//    第二级：难度（初级/中级/高级）
	//
	// 6. 路由置信度：
	//    可以要求 Routing Agent 返回置信度
	//    低置信度时转人工或请求澄清
}
