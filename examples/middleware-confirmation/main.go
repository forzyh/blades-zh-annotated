// Blades 示例：确认中间件（middleware-confirmation）
//
// 本示例演示如何使用确认中间件（Confirm Middleware）在 Agent 执行前
// 要求用户确认。这种模式适用于需要人工审核或批准的场景。
//
// 适用场景：
// - 敏感操作确认（删除数据、发送消息）
// - 成本控制（昂贵的 LLM 调用前确认）
// - 合规审核（需要人工审批的场景）
// - 交互式调试（逐步确认 Agent 行为）
//
// 核心概念：
// 1. Confirm Middleware：在执行前调用确认回调函数
// 2. 确认回调：接收用户消息预览，返回是否继续
// 3. ErrInterrupted：当确认被拒绝时返回的错误
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
//       运行时程序会暂停等待用户输入 y/N
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/middleware"
)

// confirmPrompt 是一个简单的交互式确认函数
// 它向用户显示请求预览并要求确认是否继续
// 参数：
//   - ctx: 上下文（这里未使用，但符合接口签名）
//   - message: 用户的原始输入消息
// 返回值：
//   - bool: true 表示确认继续，false 表示拒绝
//   - error: 读取输入时可能发生的错误
func confirmPrompt(ctx context.Context, message *blades.Message) (bool, error) {
	// 提取并修剪消息文本
	preview := strings.TrimSpace(message.Text())

	// 显示请求预览
	fmt.Println("Request preview:")
	fmt.Println(preview)

	// 提示用户确认
	fmt.Print("Proceed? [y/N]: ")

	// 从标准输入读取用户输入
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}

	// 规范化输入（转小写、去空白）
	line = strings.TrimSpace(strings.ToLower(line))

	// 接受 "y" 或 "yes" 作为确认
	return line == "y" || line == "yes", nil
}

func main() {
	// 步骤 1: 创建 OpenAI 模型提供者
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建 Agent 并绑定确认中间件
	// middleware.Confirm 接受一个确认回调函数
	// 在 Agent 执行前会调用该函数，只有返回 true 才会继续
	agent, err := blades.NewAgent(
		"ConfirmAgent", // Agent 名称
		blades.WithModel(model),
		blades.WithInstruction("Answer clearly and concisely."), // 系统指令
		blades.WithMiddleware(middleware.Confirm(confirmPrompt)), // 绑定确认中间件
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建用户输入消息
	// 这里是一个示例请求：总结敏捷宣言的核心思想
	input := blades.UserMessage("Summarize the key ideas of the Agile Manifesto in 3 bullet points.")

	// 步骤 4: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	output, err := runner.Run(context.Background(), input)

	// 步骤 5: 处理可能的错误
	if err != nil {
		// 检查是否是中断错误（用户拒绝确认）
		if errors.Is(err, blades.ErrInterrupted) {
			log.Println("Confirmation denied. Aborting.")
			return
		}
		// 其他错误，终止程序
		log.Fatal(err)
	}

	// 步骤 6: 输出结果
	log.Println(output.Text())

	// 运行流程说明：
	// 1. 用户运行程序
	// 2. 程序显示请求预览："Summarize the key ideas of the Agile Manifesto in 3 bullet points."
	// 3. 程序提示："Proceed? [y/N]:"
	// 4. 用户输入 y 或 N
	//    - 如果输入 y：Agent 执行并返回结果
	//    - 如果输入 N 或其他：程序输出 "Confirmation denied. Aborting." 并退出
	//
	// 扩展提示：
	// 1. 自定义确认逻辑：
	//    - 检查用户权限
	//    - 验证请求成本
	//    - 记录审计日志
	//
	// 2. 批量确认：
	//    可以累积多个请求后一次性确认
	//
	// 3. 超时确认：
	//    添加确认超时机制，超时后自动拒绝
}
