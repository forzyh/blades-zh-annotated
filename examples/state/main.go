// Blades 示例：状态管理（state）
//
// 本示例演示如何在多 Agent 序列执行中使用 State（状态）进行数据传递。
// 通过将每个 Agent 的输出保存到 Session State，
// 后续 Agent 可以通过模板引用之前的输出。
//
// 适用场景：
// - 多阶段内容生成（草稿 -> 审核 -> 修改）
// - 代码生成和审查流程
// - 翻译和润色工作流
// - 任何需要数据在 Agent 间传递的场景
//
// 核心概念：
// 1. Session State：跨 Agent 共享的键值存储
// 2. OutputKey：Agent 输出在 State 中的键名
// 3. 模板引用：使用 {{.OutputKey}} 引用之前的输出
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
)

func main() {
	// 步骤 1: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建 CodeWriterAgent（代码生成 Agent）
	// 负责根据用户需求编写 Python 代码
	codeWriterAgent, err := blades.NewAgent(
		"CodeWriterAgent",
		blades.WithModel(model),
		blades.WithInstruction(`You are a Python Code Generator.
Based *only* on the user's request, write Python code that fulfills the requirement.
Output *only* the complete Python code block, enclosed in triple backticks ("python ... ").
Do not add any other text before or after the code block.`),
		blades.WithDescription("Writes initial Python code based on a specification."),
		// 注意：这里没有设置 WithOutputKey，所以输出直接作为消息返回
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 CodeReviewerAgent（代码审查 Agent）
	// 负责审查 CodeWriterAgent 生成的代码
	codeReviewerAgent, err := blades.NewAgent(
		"CodeReviewerAgent",
		blades.WithModel(model),
		blades.WithInstruction(`You are an expert Python Code Reviewer.
    Your task is to provide constructive feedback on the provided code.

    **Code to Review:**
	{{.CodeWriterAgent}}  // 使用模板引用 CodeWriterAgent 的输出

**Review Criteria:**
1.  **Correctness:** Does the code work as intended? Are there logic errors?
2.  **Readability:** Is the code clear and easy to understand? Follows PEP 8 style guidelines?
3.  **Efficiency:** Is the code reasonably efficient? Any obvious performance bottlenecks?
4.  **Edge Cases:** Does the code handle potential edge cases or invalid inputs gracefully?
5.  **Best Practices:** Does the code follow common Python best practices?

**Output:**
Provide your feedback as a concise, bulleted list. Focus on the most important points for improvement.
If the code is excellent and requires no changes, simply state: "No major issues found."
Output *only* the review comments or the "No major issues" statement.`),
		blades.WithDescription("Reviews code and provides feedback."),
		// 注意：这里也没有设置 WithOutputKey
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建 CodeRefactorerAgent（代码重构 Agent）
	// 负责根据审查意见修改代码
	codeRefactorerAgent, err := blades.NewAgent(
		"CodeRefactorerAgent",
		blades.WithModel(model),
		blades.WithInstruction(`You are a Python Code Refactoring AI.
Your goal is to improve the given Python code based on the provided review comments.

  **Original Code:**
  {{.CodeWriterAgent}}  // 引用原始代码

  **Review Comments:**
  {{.CodeReviewerAgent}}  // 引用审查意见

**Task:**
Carefully apply the suggestions from the review comments to refactor the original code.
If the review comments state "No major issues found," return the original code unchanged.
Ensure the final code is complete, functional, and includes necessary imports and docstrings.

**Output:**
Output *only* the final, refactored Python code block, enclosed in triple backticks ("python ... ").
Do not add any other text before or after the code block.`),
		blades.WithDescription("Refactors code based on review comments."),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 5: 准备初始输入
	// 用户请求：编写一个 Python 函数，从列表中提取偶数
	input := blades.UserMessage("Write a Python function that takes a list of integers and returns a new list containing only the even integers from the original list.")

	// 步骤 6: 创建会话
	// Session 用于在 Agent 间传递状态
	session := blades.NewSession()
	ctx := context.Background()

	// 步骤 7: 顺序执行三个 Agent
	// 每个 Agent 的输出保存到 Session State，供后续 Agent 引用
	var output *blades.Message
	for _, agent := range []blades.Agent{codeWriterAgent, codeReviewerAgent, codeRefactorerAgent} {
		runner := blades.NewRunner(agent)
		// 第一次迭代：input 是用户请求
		// 后续迭代：input 为 nil，Agent 从 State 获取所需数据
		output, err = runner.Run(ctx, input, blades.WithSession(session))
		if err != nil {
			log.Fatal(err)
		}

		// 清除 input，后续 Agent 不接收新的用户输入
		input = nil

		// 将当前 Agent 的输出保存到 Session State
		// 键名是 Agent 的名称，值是输出的文本
		// 这样后续 Agent 可以通过 {{.AgentName}} 引用
		session.SetState(agent.Name(), output.Text())

		// 记录日志
		log.Println(agent.Name(), output.Text())
	}

	// 预期流程：
	// 1. CodeWriterAgent 生成初始代码：
	//    def get_even_numbers(numbers):
	//        return [n for n in numbers if n % 2 == 0]
	//
	// 2. CodeReviewerAgent 审查代码（通过 {{.CodeWriterAgent}} 获取）：
	//    - Code is correct and follows PEP 8
	//    - Consider adding a docstring
	//    - Consider adding type hints
	//
	// 3. CodeRefactorerAgent 重构代码（通过 {{.CodeWriterAgent}} 和 {{.CodeReviewerAgent}} 获取）：
	//    def get_even_numbers(numbers: list[int]) -> list[int]:
	//        """Return a list of even numbers from the input list."""
	//        return [n for n in numbers if n % 2 == 0]
	//
	// 扩展提示：
	// 1. 使用 WithOutputKey 简化状态管理：
	//    agent, _ := blades.NewAgent("...",
	//        blades.WithOutputKey("draft"),  // 输出保存到 state["draft"]
	//    )
	//
	// 2. 使用 flow 包简化流程：
	//    sequential := flow.NewSequentialAgent(...)
	//    参见 workflow-sequential 示例
	//
	// 3. 条件执行：
	//    根据前一个 Agent 的输出决定是否执行后续 Agent
	//
	// 4. 并行执行：
	//    多个独立 Agent 可以并行执行
	//    参见 workflow-parallel 示例
}
