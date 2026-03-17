// Blades 示例：音频生成（model-audio）
//
// 本示例演示如何使用 Blades 框架生成音频（文本转语音，TTS）。
// 通过 openai.NewAudio 创建音频模型，可以将文本转换为语音输出。
//
// 适用场景：
// - 语音助手和聊天机器人
// - 有声读物生成
// - 通知和提醒的语音播报
// - 辅助功能（为视障用户提供语音）
//
// 核心概念：
// 1. Audio Model：专门用于音频生成的模型（如 gpt-4o-mini-tts）
// 2. DataPart：包含音频二进制数据的部分
// 3. FilePart：包含文件 URI 的部分（远程 URL 或本地路径）
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_API_KEY 环境变量
//       生成的音频文件会保存到当前目录的 generated/ 文件夹
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
	// 步骤 1: 创建音频模型
	// openai.NewAudio 创建一个专门用于音频生成的模型
	// 参数：
	//   - 模型名称：如 "gpt-4o-mini-tts"
	//   - AudioConfig：音频配置
	//     - Voice: 语音音色（如 alloy, echo, fable, onyx, nova, shimmer）
	//     - ResponseFormat: 输出格式（如 mp3, opus, aac, flac）
	model := openai.NewAudio(
		"gpt-4o-mini-tts",
		openai.AudioConfig{Voice: "alloy", ResponseFormat: "mp3"},
	)

	// 步骤 2: 创建 Agent
	// 音频 Agent 不需要额外的指令，因为任务是纯文本转语音
	agent, err := blades.NewAgent(
		"Audio Agent",
		blades.WithModel(model),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	output, err := runner.Run(
		context.Background(),
		blades.UserMessage("Welcome to the Blades audio demo!"), // 要转换的文本
	)
	if err != nil {
		log.Fatalf("generate audio: %v", err)
	}

	// 步骤 4: 创建输出目录
	outputDir := "generated"
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	// 步骤 5: 处理输出结果
	// output.Parts 包含一个或多个内容部分
	// 遍历所有部分，根据类型处理音频数据
	for n, part := range output.Parts {
		switch audio := part.(type) {
		case blades.DataPart:
			// DataPart：包含原始音频字节数据
			// 将字节写入文件，保存到 generated/ 目录
			path := filepath.Join(outputDir, fmt.Sprintf("speech-%d.mp3", n))
			if err := os.WriteFile(path, audio.Bytes, 0o644); err != nil {
				log.Fatalf("write file %s: %v", path, err)
			}
			log.Printf("saved %s", path)

		case blades.FilePart:
			// FilePart：包含音频文件的 URI（通常是远程 URL）
			// 可以直接使用或下载该 URL
			log.Printf("streamed audio url: %s", audio.URI)
		}
	}

	// 扩展提示：
	// 1. 其他语音音色：
	//    - alloy: 中性、清晰
	//    - echo: 深沉、男性化
	//    - fable: 温暖、叙事感
	//    - onyx: 低沉、权威感
	//    - nova: 活泼、女性化
	//    - shimmer: 柔和、女性化
	//
	// 2. 其他输出格式：
	//    - mp3: 最通用，适合网络传输
	//    - opus: 高质量、低码率
	//    - aac: Apple 设备友好
	//    - flac: 无损格式
	//
	// 3. 流式音频：
	//    可以使用 RunStream 实现流式音频生成
	//    适合长文本或实时语音场景
}
