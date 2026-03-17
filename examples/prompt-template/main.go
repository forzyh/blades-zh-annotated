// Blades 示例：提示词模板（prompt-template）
//
// 本示例演示如何使用 Go 模板引擎动态生成提示词。
// 模板允许在运行时根据参数定制提示词内容，
// 是实现灵活、可复用提示词的关键技术。
//
// 适用场景：
// - 动态内容生成（根据用户/场景定制）
// - 提示词复用（同一模板不同参数）
// - A/B 测试不同提示词版本
// - 多语言/多风格支持
//
// 核心概念：
// 1. Go Template：Go 标准库的模板引擎
// 2. 模板参数：运行时传递给模板的数据
// 3. 提示词构建：将模板渲染为最终字符串
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
)

// buildPrompt 使用 Go 模板构建提示词
// 参数 params 是模板渲染时使用的数据 map
// 返回值是渲染后的提示词字符串和可能的错误
func buildPrompt(params map[string]any) (string, error) {
	// 定义模板字符串
	// {{.audience}} 是占位符，会被 params 中的 "audience" 值替换
	tmpl := "Respond concisely and accurately for a {{.audience}} audience."

	// 使用 strings.Builder 高效构建字符串
	var buf strings.Builder

	// 步骤 1: 解析模板
	// template.New 创建一个具名模板（名称用于错误信息）
	// Parse 方法解析模板字符串
	// 如果模板语法有误，Parse 会返回错误
	t, err := template.New("message").Parse(tmpl)
	if err != nil {
		return "", err
	}

	// 步骤 2: 执行模板渲染
	// Execute 方法将 params 中的数据填充到模板中
	// &buf 是输出目标（实现 io.Writer 接口）
	// params 是包含模板变量的 map
	if err := t.Execute(&buf, params); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func main() {
	// 步骤 1: 创建模型提供者
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建 Agent
	// 注意：Agent 的指令也可以使用模板
	// blades.WithInstruction("Please summarize {{.topic}} in three key points.")
	agent, err := blades.NewAgent(
		"Template Agent",
		blades.WithModel(model),
		blades.WithInstruction("Please summarize {{.topic}} in three key points."),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 定义模板参数
	// params 是一个 map，包含模板中使用的变量
	params := map[string]any{
		"topic":    "The Future of Artificial Intelligence", // 主题
		"audience": "General reader",                         // 目标受众
	}

	// 步骤 4: 使用模板构建提示词
	// buildPrompt 将 audience 参数渲染到提示词中
	// 渲染后的提示词："Respond concisely and accurately for a General reader audience."
	prompt, err := buildPrompt(params)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 5: 创建用户消息
	input := blades.UserMessage(prompt)

	// 步骤 6: 创建会话
	// Session 可以存储模板参数，用于渲染 Agent 指令中的 {{.topic}}
	session := blades.NewSession(params)

	// 步骤 7: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	// WithSession 绑定会话，使 Agent 可以访问模板参数
	output, err := runner.Run(context.Background(), input, blades.WithSession(session))
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 8: 输出结果
	log.Println(output.Text())

	// 预期输出：
	// Agent 会返回关于"人工智能未来"的三个要点的总结
	//
	// 扩展提示：
	// 1. 复杂模板语法：
	//    - 条件：{{if .showDetails}}详细版本{{else}}简要版本{{end}}
	//    - 循环：{{range .items}}- {{.}}{{end}}
	//    - 嵌套：{{.user.name}} 访问嵌套字段
	//    - 函数：{{toUpperCase .text}} 使用自定义函数
	//
	// 2. 模板函数：
	//    funcMap := template.FuncMap{
	//        "upper": strings.ToUpper,
	//    }
	//    t := template.New("msg").Funcs(funcMap)
	//
	// 3. 模板复用：
	//    可以将模板定义到文件中，使用 template.ParseFiles 加载
	//    便于管理和版本控制提示词模板
	//
	// 4. 提示词模板最佳实践：
	//    - 保持模板简单，避免过度复杂
	//    - 使用有意义的变量名
	//    - 为可选参数提供默认值
	//    - 测试边界情况（空值、特殊字符）
}
