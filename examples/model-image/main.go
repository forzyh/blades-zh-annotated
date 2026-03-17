// Blades 示例：图像生成（model-image）
//
// 本示例演示如何使用 Blades 框架生成图像（文生图）。
// 通过 openai.NewImage 创建图像模型，可以根据文本描述生成图像。
//
// 适用场景：
// - 创意设计和艺术创作
// - 营销材料生成
// - 游戏和影视概念设计
// - 社交媒体内容创作
//
// 核心概念：
// 1. Image Model：专门用于图像生成的模型（如 dall-e-3, gpt-image-1）
// 2. DataPart：包含图像二进制数据的部分
// 3. FilePart：包含图像 URI 的部分（远程 URL）
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_API_KEY 环境变量
//       生成的图像文件会保存到当前目录的 generated/ 文件夹
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
)

func main() {
	// 步骤 1: 创建图像模型
	// openai.NewImage 创建一个专门用于图像生成的模型
	// 参数：
	//   - 模型名称：如 "dall-e-3", "gpt-image-1"
	//   - ImageConfig：图像配置
	//     - Size: 图像尺寸（如 1024x1024, 1024x1536, 1536x1024）
	//     - OutputFormat: 输出格式（如 png, jpeg, webp）
	model := openai.NewImage(
		"gpt-image-1",
		openai.ImageConfig{Size: "1024x1024", OutputFormat: "png"},
	)

	// 步骤 2: 创建 Agent
	// 图像生成 Agent 不需要额外的指令
	// 提示词中已经包含了图像描述
	agent, err := blades.NewAgent(
		"Image Agent",
		blades.WithModel(model),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	output, err := runner.Run(
		context.Background(),
		// 图像生成提示词
		// 描述要生成的图像内容
		// 提示词越详细，生成的图像越符合预期
		blades.UserMessage("A watercolor illustration of a mountain cabin at sunrise"),
	)
	if err != nil {
		log.Fatalf("generate image: %v", err)
	}

	// 步骤 4: 创建输出目录
	outputDir := "generated"
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	// 步骤 5: 处理输出结果
	// output.Parts 包含一个或多个图像部分
	// 遍历所有部分，根据类型处理图像数据
	saved := 0
	for _, part := range output.Parts {
		switch img := part.(type) {
		case blades.DataPart:
			// DataPart：包含原始图像字节数据
			// 将字节写入文件，保存到 generated/ 目录
			saved++
			// 使用 MIMEType.Format() 获取文件扩展名
			name := fmt.Sprintf("image-%d.%s", saved, img.MIMEType.Format())
			path := filepath.Join(outputDir, name)
			if err := os.WriteFile(path, img.Bytes, 0o644); err != nil {
				log.Fatalf("write file %s: %v", path, err)
			}
			log.Printf("saved %s", path)

		case blades.FilePart:
			// FilePart：包含图像的 URL
			// 这通常用于 dall-e-3 等模型，返回远程图像链接
			log.Printf("image url: %s", img.URI)
		}
	}

	// 扩展提示：
	// 1. 提示词编写技巧：
	//    - 详细描述场景：主体、背景、光线、颜色
	//    - 指定艺术风格：水彩、油画、素描、3D 渲染
	//    - 添加情感氛围：温馨、神秘、欢快
	//    - 示例："A cozy cabin in snowy mountains at sunrise, watercolor style, warm colors"
	//
	// 2. 常用尺寸选项：
	//    - 1024x1024: 正方形，适合头像、图标
	//    - 1024x1536: 纵向，适合人像
	//    - 1536x1024: 横向，适合风景
	//
	// 3. 其他输出格式：
	//    - png: 无损压缩，适合编辑
	//    - jpeg: 有损压缩，文件更小
	//    - webp: 现代格式，更好的压缩率
	//
	// 4. 图像编辑：
	//    可以使用 blades.FilePart 上传参考图像
	//    结合文本提示进行图像编辑或变体生成
}
