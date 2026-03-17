// Blades 示例：文档翻译（translation）
//
// 本示例演示如何使用 Blades 构建多 Agent 翻译工作流。
// 通过两个 Agent 串联（翻译 -> 润色），实现高质量的文档翻译。
//
// 适用场景：
// - 技术文档翻译
// - 网站内容本地化
// - 多语言内容生成
// - 批量翻译任务
//
// 核心概念：
// 1. Sequential Agents：顺序执行的多个 Agent
// 2. Agent 链：前一个 Agent 的输出作为后一个的输入
// 3. 文件 I/O：读取源文件并写入翻译结果
//
// 使用方法：
// go run main.go -input=README_zh.md -output=README.md
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
)

var (
	input  string // 输入文件路径
	output string // 输出文件路径
)

func init() {
	// 定义命令行参数
	flag.StringVar(&input, "input", "../../README_zh.md", "input file path")
	flag.StringVar(&output, "output", "../../README.md", "output file path")
}

func main() {
	flag.Parse() // 解析命令行参数

	// 步骤 1: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建翻译 Agent
	// 负责将中文 Markdown 文档翻译成英文
	tr, err := blades.NewAgent(
		"Document translator",
		blades.WithModel(model),
		blades.WithInstruction("Translate the Chinese text within the given Markdown content to fluent, publication-quality English, perfectly preserving all Markdown syntax and structure, and outputting only the raw translated Markdown content."),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建润色 Agent
	// 负责优化翻译结果，提升流畅度和准确性
	refine, err := blades.NewAgent(
		"Refine Agent",
		blades.WithModel(model),
		blades.WithInstruction("Polish the following translated Markdown text by refining its sentence structure and correcting grammatical errors to improve fluency and readability, while ensuring the original meaning and all Markdown \n  syntax remain unchanged"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 读取源文件
	content, err := os.ReadFile(input)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 5: 准备初始输入
	inputMsg := blades.UserMessage(string(content))
	var outputMsg *blades.Message

	// 步骤 6: 顺序执行两个 Agent
	// 1. 翻译 Agent 执行中文到英文的翻译
	// 2. 润色 Agent 优化翻译质量
	for _, agent := range []blades.Agent{tr, refine} {
		runner := blades.NewRunner(agent)
		outputMsg, err = runner.Run(context.Background(), inputMsg)
		if err != nil {
			log.Fatal(err)
		}
		// 将当前输出作为下一个 Agent 的输入
		inputMsg = outputMsg
	}

	// 步骤 7: 写入翻译结果到文件
	// outputMsg.Text() 包含最终的翻译内容
	if err := os.WriteFile(output.Text(), []byte(outputMsg.Text()), 0644); err != nil {
		log.Fatal(err)
	}

	// 预期流程：
	// 1. 读取 README_zh.md（中文）
	// 2. 翻译 Agent 生成英文初稿
	// 3. 润色 Agent 优化翻译质量
	// 4. 写入 README.md（英文）
	//
	// 扩展提示：
	// 1. 多语言支持：
	//    可以创建多个翻译 Agent 支持多种语言
	//    trEn, _ := blades.NewAgent("English Translator", ...)
	//    trJa, _ := blades.NewAgent("Japanese Translator", ...)
	//
	// 2. 质量检查：
	//    添加第三个 Agent 进行质量检查
	//    qaAgent, _ := blades.NewAgent("QA Agent",
	//        blades.WithInstruction("Check translation accuracy..."),
	//    )
	//
	// 3. 术语表：
	//    在指令中提供术语表确保一致性
	//    "Use these terms: API=API, Server=服务器..."
	//
	// 4. 分批处理：
	//    对于大文档，可以分段翻译后合并
	//
	// 5. 并发翻译：
	//    对于独立章节，可以并发翻译提高效率
	//
	// 6. 上下文保持：
	//    使用 Session 在多个翻译请求间保持上下文
	//    确保术语和风格的一致性
}
