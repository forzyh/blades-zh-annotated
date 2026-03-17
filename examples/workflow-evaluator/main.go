// Blades 示例：评估者循环（workflow-evaluator）
//
// 本示例演示如何实现评估者循环（Evaluator Loop）模式。
// 这是一种自修正的工作流：生成器创建内容，评估者检查质量，
// 如不满足要求则提供反馈进行改进，直到达到标准。
//
// 适用场景：
// - 内容质量保障
// - 代码生成和审查
// - 写作和编辑流程
// - 需要迭代优化的任务
//
// 核心概念：
// 1. Generator（生成器）：创建初始内容
// 2. Evaluator（评估者）：评估内容质量
// 3. Feedback Loop：根据反馈迭代改进
// 4. Pass/Fail：评估通过/失败判断
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/evaluator"
)

// buildPrompt 构建评估提示词
// 将主题、内容和反馈组合成评估提示词
func buildPrompt(topic, content, feedback string) *blades.Message {
	return blades.UserMessage(fmt.Sprintf(
		"topic: %s\n**content**\n%s\n**feedback**\n%s",
		topic,
		content,
		feedback,
	))
}

func main() {
	// 步骤 1: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建故事大纲生成器
	// Generator 负责根据主题创建初始内容
	generator, err := blades.NewAgent(
		"story_outline_generator",
		blades.WithModel(model),
		blades.WithInstruction(`You generate a very short story outline based on the user's input.
		If there is any feedback provided, use it to improve the outline.`),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建评估器
	// Evaluator 负责检查生成内容的质量
	// evaluator.New 创建一个专门的评估 Agent
	evaluator, err := evaluator.New("story_evaluator",
		blades.WithModel(model),
		blades.WithInstruction(`You evaluate a story outline and decide if it's good enough.
		If it's not good enough, you provide feedback on what needs to be improved.
		You can give it a pass if the story outline is good enough - do not go for perfection`),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 定义主题
	topic := "Generate a story outline about a brave knight who saves a village from a dragon."
	input := blades.UserMessage(topic)

	// 步骤 5: 创建 Runner
	runner := blades.NewRunner(generator)

	// 步骤 6: 执行评估者循环
	// 最多迭代 3 次，或直到评估通过
	var output *blades.Message
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		// 运行生成器
		output, err = runner.Run(ctx, input)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(output.Text())

		// 运行评估器
		evaluation, err := evaluator.Run(ctx, output)
		if err != nil {
			log.Fatal(err)
		}

		// 检查评估结果
		if evaluation.Pass {
			// 评估通过，退出循环
			break
		}

		// 评估未通过，根据反馈构建新的输入
		if evaluation.Feedback != nil {
			input = buildPrompt(
				topic,
				output.Text(),        // 当前内容
				strings.Join(evaluation.Feedback.Suggestions, "\n"), // 改进建议
			)
		}
	}

	// 步骤 7: 输出最终结果
	log.Println("Final Output:", output.Text())

	// 预期流程：
	// 迭代 1:
	//   生成器：创建初始故事大纲
	//   评估者：反馈"需要更多冲突细节"
	// 迭代 2:
	//   生成器：根据反馈改进大纲
	//   评估者：反馈"通过"
	// 结束，输出最终版本
	//
	// 扩展提示：
	// 1. 多轮迭代：
	//    增加最大迭代次数允许更多改进机会
	//    for i := 0; i < 5; i++ { ... }
	//
	// 2. 评估标准：
	//    可以在指令中定义具体评估标准
	//    "Pass if: complete plot, clear characters, defined conflict"
	//
	// 3. 反馈格式：
	//    可以结构化反馈便于处理
	//    type Feedback struct {
	//        Strengths []string
	//        Weaknesses []string
	//        Suggestions []string
	//    }
	//
	// 4. 多评估者：
	//    使用多个评估者从不同角度评估
	//    - 结构评估者
	//    - 角色评估者
	//    - 创意评估者
	//
	// 5. 评分系统：
	//    使用数值评分而非 Pass/Fail
	//    if evaluation.Score >= 8.0 { break }
	//
	// 6. 早期退出：
	//    检测到严重问题时提前终止
	//    if hasCriticalIssue(output) { break }
}
