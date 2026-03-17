// Blades 示例：LLM 输出评估（evaluation）
//
// 本示例演示如何使用 blades/evaluator 包来评估 LLM 的输出质量。
// evaluator 是一个专门用于评估 LLM 响应是否满足要求的组件，
// 它可以自动判断回答是否正确、相关，并提供改进建议。
//
// 适用场景：
// - 自动化测试 LLM 模型的输出质量
// - 构建自修正的 Agent 系统（评估 - 反馈循环）
// - 内容审核和质量控制
// - A/B 测试不同提示词或模型的效果
//
// 核心概念：
// 1. evaluator.Evaluator：专门的评估器，输出包含 Pass/Fail、分数和反馈
// 2. 评估提示词：指导 LLM 如何评估响应的系统指令
// 3. Go 模板：使用 text/template 动态生成评估提示词
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/evaluator"
)

// buildPrompt 使用 Go 模板构建评估提示词
// 参数 params 包含待评估的输入（Input）和输出（Output）
// 返回值是填充后的评估提示词字符串
func buildPrompt(params map[string]any) (string, error) {
	// 定义评估提示词模板
	// 模板使用 Go 的 text/template 语法，{{.Input}}和{{.Output}}会被动态替换
	tmpl := `You are an expert evaluator. Your task is to assess the relevancy of the LLM's response to the given input prompt.
Please follow these guidelines:
1. Understand the Input Prompt: Carefully read and comprehend the input prompt to grasp what is being asked.
2. Analyze the LLM's Response: Evaluate the response provided by the LLM in relation to the input prompt.
3. Determine Relevancy: Decide if the response directly addresses the input prompt. A relevant response should be on-topic and provide information or answers that align with the prompt's intent.
4. Scoring Criteria:
   - Pass: If the response is relevant and adequately addresses the prompt.
   - Fail: If the response is off-topic, irrelevant, or does not answer the prompt.
5. Provide Feedback: Offer constructive feedback on why the response was deemed relevant or irrelevant.
Use the above guidelines to evaluate the LLM's response.
Below are the inputs:
{
  "User prompt": {{ .Input }},
  "Agent response": {{ .Output }},
}`

	// 使用 strings.Builder 高效构建模板输出
	var buf strings.Builder

	// 步骤 1: 解析模板
	// template.New 创建一个具名的模板
	// Parse 方法解析模板字符串，返回错误如果模板语法有问题
	t, err := template.New("message").Parse(tmpl)
	if err != nil {
		return "", err
	}

	// 步骤 2: 执行模板渲染
	// Execute 方法将 params 中的数据填充到模板中
	// &buf 是输出目标，params 包含 Input 和 Output 两个字段
	if err := t.Execute(&buf, params); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func main() {
	// 步骤 1: 创建 OpenAI 模型提供者
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建评估器
	// evaluator.New 创建一个专门用于评估的 Agent
	// - 第一个参数是评估器名称
	// - WithModel 指定用于评估的 LLM 模型
	r, err := evaluator.New(
		"Evaluation Agent",
		blades.WithModel(model),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 准备测试数据
	// qa 是一个 map，包含问题和答案
	// 这里故意放入了一个错误答案（5 公里=60 公里/小时）用于演示评估功能
	qa := map[string]string{
		"What is the capital of France?":  "Paris.",        // 正确答案
		"Convert 5 kilometers to meters.": "60 km/h.",      // 错误答案（单位都不对）
	}

	// 步骤 4: 遍历每个问答对进行评估
	for q, a := range qa {
		// 使用模板构建评估提示词
		// params 包含 Input（用户问题）和 Output（Agent 回答）
		prompt, err := buildPrompt(map[string]any{
			"Input":  q, // 用户原始问题
			"Output": a, // 待评估的回答
		})
		if err != nil {
			log.Fatal(err)
		}

		// 创建用户消息，包含评估提示词
		input := blades.UserMessage(prompt)

		// 步骤 5: 运行评估器
		// r.Run 调用底层 LLM 进行评估，返回评估结果
		// 评估结果包含 Pass（是否通过）、Score（分数）、Feedback（反馈建议）
		output, err := r.Run(context.Background(), input)
		if err != nil {
			log.Fatal(err)
		}

		// 步骤 6: 输出评估结果
		// output.Pass: 布尔值，表示回答是否通过评估
		// output.Score: 数值分数，表示回答质量
		// output.Feedback: 详细的反馈意见和改进建议
		log.Printf("Pass: %t Score: %f Feedback: %+v", output.Pass, output.Score, output.Feedback)
	}

	// 预期输出：
	// 第一个问题（法国首都）应该 Pass=true
	// 第二个问题（公里转米）应该 Pass=false，因为答案完全错误
}
