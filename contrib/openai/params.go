// Package openai 提供了 OpenAI API 的客户端实现。
//
// # 核心功能
//
//   - 聊天模型（chat.go）：支持文本生成、工具调用、多模态输入
//   - 图像模型（image.go）：支持图像生成（DALL-E 3）
//   - 音频模型（audio.go）：支持语音生成（TTS）
//
// # 使用示例
//
//	// 创建聊天模型
//	provider := openai.NewModel("gpt-4o", openai.Config{
//	    APIKey:      "sk-...",
//	    Temperature: 0.7,
//	})
//
//	// 创建图像模型
//	imageProvider := openai.NewImage("dall-e-3", openai.ImageConfig{
//	    APIKey: "sk-...",
//	    Size:   "1024x1024",
//	})
//
//	// 创建音频模型
//	audioProvider := openai.NewAudio("tts-1", openai.AudioConfig{
//	    APIKey: "sk-...",
//	    Voice:  "alloy",
//	})
package openai

import (
	"strings"

	"github.com/go-kratos/blades"
)

// promptFromMessages 从消息列表中提取文本提示。
//
// # 作用说明
//
// 将多条消息的文本内容连接为一个完整的提示字符串。
// 主要用于图像生成和音频生成，这些场景通常只需要简单的文本提示。
//
// # 参数说明
//
//   - messages: Blades 消息列表
//
// # 返回值
//
// string: 连接后的文本提示，每条消息占一行
//
// # 使用场景
//
//   - 图像生成：将消息历史转换为 DALL-E 的 prompt
//   - 音频生成：将消息历史转换为 TTS 的输入文本
//
// # 示例
//
//	messages := []*blades.Message{
//	    {Parts: []blades.Part{blades.TextPart{Text: "你好"}}},
//	    {Parts: []blades.Part{blades.TextPart{Text: "世界"}}},
//	}
//	prompt := promptFromMessages(messages)
//	// 返回："你好\n世界"
func promptFromMessages(messages []*blades.Message) string {
	var sections []string
	for _, msg := range messages {
		// 提取每条消息的文本内容
		sections = append(sections, msg.Text())
	}
	// 用换行符连接所有消息
	return strings.Join(sections, "\n")
}
