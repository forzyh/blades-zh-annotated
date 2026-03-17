// Blades 示例：图像理解（image-to-text）
//
// 本示例演示如何使用 Blades 框架处理多模态输入（文本 + 图像）。
// 通过组合 TextPart 和 FilePart，可以向 LLM 发送包含图像的请求，
// 让模型理解和分析图像内容。
//
// 适用场景：
// - 图像内容分析和描述
// - OCR（光学字符识别）
// - 图表、截图理解
// - 视觉问答（Visual QA）
//
// 核心概念：
// 1. TextPart：文本消息部分
// 2. FilePart：文件/图像部分，支持 URI 或本地文件
// 3. 多模态消息：组合多种内容类型的消息
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
//       使用的模型需要支持视觉输入（如 GPT-4V、GPT-4o 等）
package main

import (
	"context"
	"log"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
)

func main() {
	// 步骤 1: 创建 OpenAI 模型提供者
	// 使用支持视觉的模型（如 gpt-4-vision-preview、gpt-4o 等）
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建 Agent（智能体）
	// Agent 是 blades 的核心组件，负责与 LLM 交互
	agent, err := blades.NewAgent(
		"Basic Agent", // Agent 名称
		blades.WithModel(model), // 使用 OpenAI 模型
		blades.WithInstruction("You are a helpful assistant that provides detailed and accurate information."),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建多模态输入消息
	// blades.UserMessage 可以接受多种类型的内容部分（Parts）
	// 这里组合了 TextPart（文本）和 FilePart（图像）
	input := blades.UserMessage(
		// TextPart：向 LLM 发送的文本指令
		// 告诉模型需要分析图像并描述内容
		blades.TextPart{
			Text: "Can you describe the image in logo.svg?",
		},
		// FilePart：图像内容
		// MIMEType：指定图像的 MIME 类型（如 image/png、image/jpeg）
		// URI：图像的 URL 或本地文件路径
		//     支持 http(s):// 开头的远程 URL
		//     也支持本地文件路径（如 ./image.png）
		blades.FilePart{
			MIMEType: "image/png",
			URI:      "https://go-kratos.dev/images/architecture.png",
		},
	)

	// 步骤 4: 创建 Runner（执行器）
	// Runner 负责执行 Agent，可以配置额外选项
	runner := blades.NewRunner(agent)

	// 步骤 5: 运行 Agent
	// Run 方法发送请求到 LLM 并等待完整响应
	// 返回的 output.Message 包含模型的回复
	output, err := runner.Run(context.Background(), input)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 6: 输出结果
	// output.Text() 提取模型的文本回复
	log.Println(output.Text())

	// 扩展提示：
	// 1. 分析本地图片：
	//    blades.FilePart{MIMEType: "image/jpeg", URI: "./photo.jpg"}
	//
	// 2. 多图像输入（比较两张图片）：
	//    blades.UserMessage(
	//        blades.TextPart{Text: "Compare these two images"},
	//        blades.FilePart{MIMEType: "image/png", URI: "http://.../img1.png"},
	//        blades.FilePart{MIMEType: "image/png", URI: "http://.../img2.png"},
	//    )
	//
	// 3. 图像 + 上下文：
	//    blades.UserMessage(
	//        blades.TextPart{Text: "Based on this chart, what's the trend?"},
	//        blades.FilePart{MIMEType: "image/png", URI: "http://.../chart.png"},
	//    )
}
