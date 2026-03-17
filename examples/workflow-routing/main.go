// Blades 示例：路由工作流（workflow-routing）
//
// 本示例演示如何实现自定义路由工作流。
// 与 workflow-router 使用 flow.NewRoutingAgent 不同，
// 这里展示了如何手动实现路由逻辑，提供更大的灵活性。
//
// 适用场景：
// - 需要自定义路由逻辑
// - 复杂路由规则
// - 多层路由决策
// - 特殊路由需求
//
// 核心概念：
// 1. RoutingWorkflow：自定义路由工作流结构
// 2. 路由 Agent：决定目标 Agent
// 3. Agent Tool：将 Agent 封装为工具
// 4. 手动实现：直接控制路由流程
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/tools"
)

// RoutingWorkflow 是一个自定义路由工作流结构
// 它实现了 blades.Agent 接口，可以作为 Agent 使用
type RoutingWorkflow struct {
	blades.Agent          // 内嵌基础 Agent（用于路由决策）
	routes    map[string]string                        // 路由名称 -> 指令映射
	agents    map[string]blades.Agent                  // 路由名称 -> Agent 映射
}

// NewRoutingWorkflow 创建一个新的 RoutingWorkflow
// 参数 routes 定义了可用的路由和对应的 Agent 指令
func NewRoutingWorkflow(routes map[string]string) (*RoutingWorkflow, error) {
	// 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 创建路由 Agent（决策者）
	// 这个 Agent 负责分析用户请求，选择合适的路由
	router, err := blades.NewAgent(
		"triage_agent",
		blades.WithModel(model),
		blades.WithInstruction("You determine which agent to use based on the user's homework question"),
	)
	if err != nil {
		return nil, err
	}

	// 为每个路由创建对应的 Agent
	agents := make(map[string]blades.Agent, len(routes))
	for name, instructions := range routes {
		agent, err := blades.NewAgent(
			name,
			blades.WithModel(model),
			blades.WithInstruction(instructions),
		)
		if err != nil {
			return nil, err
		}
		agents[name] = agent
	}

	// 返回 RoutingWorkflow 实例
	return &RoutingWorkflow{
		Agent:  router, // 路由决策 Agent
		routes: routes, // 路由配置
		agents: agents, // 目标 Agents
	}, nil
}

// Run 实现 blades.Agent 接口
// 这是 RoutingWorkflow 的核心方法，执行路由逻辑
func (r *RoutingWorkflow) Run(ctx context.Context, invocation *blades.Invocation) blades.Generator[*blades.Message, error] {
	return func(yield func(*blades.Message, error) bool) {
		// 步骤 1: 确定路由
		agent, err := r.selectRoute(ctx, invocation)
		if err != nil {
			yield(nil, err)
			return
		}

		// 步骤 2: 将请求转发给选中的 Agent
		// 直接调用选中 Agent 的 Run 方法
		stream := agent.Run(ctx, invocation)
		for msg, err := range stream {
			if !yield(msg, err) {
				break
			}
		}
	}
}

// selectRoute 确定最佳路由
// 它分析用户请求，选择最合适的 Agent
func (r *RoutingWorkflow) selectRoute(ctx context.Context, invocation *blades.Invocation) (blades.Agent, error) {
	// 构建路由选择提示词
	var buf strings.Builder

	// 系统说明
	buf.WriteString("You are a routing agent.\n")
	buf.WriteString("Choose the single best route key for handling the user's request.\n")

	// 用户消息
	buf.WriteString("User message: " + invocation.Message.Text() + "\n")

	// 可用路由列表
	buf.WriteString("Available route keys (choose exactly one):\n")

	// 将路由配置编码为 JSON
	routes, err := json.Marshal(r.routes)
	if err != nil {
		return nil, err
	}
	buf.WriteString(string(routes))
	buf.WriteString("\nOnly return the name of the routing key.")

	// 调用路由 Agent 进行选择
	for res, err := range r.Agent.Run(ctx, &blades.Invocation{Message: blades.UserMessage(buf.String())}) {
		if err != nil {
			return nil, err
		}

		// 提取选择的路由名称
		choice := strings.TrimSpace(res.Text())

		// 查找对应的 Agent
		if a, ok := r.agents[choice]; ok {
			return a, nil // 找到匹配的 Agent
		}
	}

	// 没有找到合适的路由
	return nil, fmt.Errorf("no route selected")
}

func main() {
	// 定义路由配置
	// key 是路由名称，value 是对应 Agent 的指令
	routes := map[string]string{
		"math_agent": "You provide help with math problems. Explain your reasoning at each step and include examples.",
		"geo_agent":  "You provide assistance with geographical queries. Explain geographic concepts, locations, and spatial relationships clearly.",
	}

	// 创建 RoutingWorkflow
	routing, err := NewRoutingWorkflow(routes)
	if err != nil {
		log.Fatal(err)
	}

	// 准备用户输入
	// 这个问题应该被路由到 geo_agent
	input := blades.UserMessage("What is the capital of France?")

	// 创建 Runner 并执行
	// RoutingWorkflow 实现了 blades.Agent 接口，可以直接使用
	runner := blades.NewRunner(routing)
	res, err := runner.Run(context.Background(), input)
	if err != nil {
		log.Fatal(err)
	}

	// 输出结果
	log.Println(res.Text())

	// 预期流程：
	// 1. RoutingWorkflow 接收请求："What is the capital of France?"
	// 2. 构建路由选择提示词，包含路由配置
	// 3. 调用 triage_agent 选择路由
	// 4. triage_agent 返回 "geo_agent"（地理问题）
	// 5. 转发请求到 geo_agent
	// 6. geo_agent 回答问题
	// 7. 输出："The capital of France is Paris."
	//
	// 扩展提示：
	// 1. 更复杂的路由逻辑：
	//    - 基于用户身份路由
	//    - 基于内容敏感度路由
	//    - 基于负载路由
	//
	// 2. 多级路由：
	//    第一级：学科（数学/历史/科学）
	//    第二级：难度（初级/中级/高级）
	//
	// 3. 路由缓存：
	//    缓存常见问题的路由决策
	//    routeCache := make(map[string]string)
	//
	// 4. 路由日志：
	//    记录路由决策用于分析和优化
	//    log.Printf("Routed '%s' to '%s'", input, choice)
	//
	// 5. 回退处理：
	//    如果选中的 Agent 失败，可以：
	//    - 重试
	//    - 切换到备用 Agent
	//    - 返回默认响应
}
