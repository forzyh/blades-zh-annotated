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
	"context"
	"errors"
	"io"
	"strings"

	"github.com/go-kratos/blades"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
)

var (
	// ErrAudioGenerationEmpty 当 Provider 返回空音频数据时返回此错误。
	ErrAudioGenerationEmpty = errors.New("openai/audio: provider returned no audio")

	// ErrAudioRequestNil 当请求为 nil 时返回此错误。
	ErrAudioRequestNil = errors.New("openai/audio: request is nil")

	// ErrAudioModelRequired 当未指定模型时返回此错误。
	ErrAudioModelRequired = errors.New("openai/audio: model is required")

	// ErrAudioVoiceRequired 当未指定语音时返回此错误。
	ErrAudioVoiceRequired = errors.New("openai/audio: voice is required")
)

// AudioConfig 持有音频模型的配置选项。
//
// # 配置字段说明
//
//   - BaseURL: API 基础 URL
//   - APIKey: OpenAI API 密钥
//   - Voice: 语音名称（"alloy", "echo", "fable", "onyx", "nova", "shimmer"）
//   - ResponseFormat: 响应格式（"mp3", "opus", "aac", "flac", "wav", "pcm"）
//   - StreamFormat: 流式格式（"sse" 用于 Server-Sent Events）
//   - Speed: 语速（0.25 到 4.0，默认 1.0）
//   - ExtraFields: 额外字段
//   - RequestOptions: 额外请求选项
//
// # 支持的语音
//
// OpenAI TTS 支持 6 种语音：
//   - alloy: 中性、平衡的语音
//   - echo: 男性、温暖的语音
//   - fable: 女性、英国口音
//   - onyx: 男性、低沉的语音
//   - nova: 女性、美国口音
//   - shimmer: 女性、温暖的语音
//
// # 支持的音频格式
//
//   - mp3: 最常用，兼容性好（默认）
//   - opus: 高质量，低码率
//   - aac: Apple 设备常用
//   - flac: 无损压缩
//   - wav: 未压缩，高质量
//   - pcm: 原始音频数据
//
// # 使用示例
//
//	config := openai.AudioConfig{
//	    APIKey:         "sk-...",
//	    Voice:          "alloy",
//	    ResponseFormat: "mp3",
//	    Speed:          1.0,
//	}
type AudioConfig struct {
	BaseURL        string
	APIKey         string
	Voice          string
	ResponseFormat string
	StreamFormat   string
	Speed          float64
	ExtraFields    map[string]any
	RequestOptions []option.RequestOption
}

// audioModel 实现了 blades.ModelProvider 接口，用于音频生成。
//
// # 结构说明
//
//   - model: 模型名称，如 "tts-1" 或 "tts-1-hd"
//   - config: 音频生成配置
//   - client: OpenAI SDK 客户端
//
// # 支持的模型
//
//   - tts-1: 标准模型，延迟低
//   - tts-1-hd: 高质量模型，音质更好
//
// # 实现接口
//
// audioModel 实现了 blades.ModelProvider 接口：
//   - Name(): 返回模型名称
//   - Generate(): 生成音频
//   - NewStreaming(): 流式生成（包装为单次 yield）
type audioModel struct {
	model  string
	config AudioConfig
	client openai.Client
}

// NewAudio 创建一个新的 audioModel 实例。
//
// # 参数说明
//
//   - model: 模型名称，如 "tts-1"
//   - config: 音频生成配置
//
// # 返回值
//
// blades.ModelProvider: 音频模型提供者
//
// # 使用示例
//
//	provider := openai.NewAudio("tts-1", openai.AudioConfig{
//	    APIKey: "sk-...",
//	    Voice:  "alloy",
//	})
//	resp, err := provider.Generate(ctx, &blades.ModelRequest{
//	    Messages: []*blades.Message{
//	        {Role: blades.RoleUser, Parts: []blades.Part{
//	            blades.TextPart{Text: "你好，世界"},
//	        }},
//	    },
//	})
func NewAudio(model string, config AudioConfig) blades.ModelProvider {
	opts := config.RequestOptions
	// 添加 base URL 和 API key
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}
	if config.APIKey != "" {
		opts = append(opts, option.WithAPIKey(config.APIKey))
	}
	return &audioModel{
		model:  model,
		config: config,
		client: openai.NewClient(opts...),
	}
}

// Name 返回音频模型的名称。
//
// 这是 blades.ModelProvider 接口的必需方法。
func (m *audioModel) Name() string {
	return m.model
}

// buildAudioParams 构建 OpenAI 音频生成参数。
//
// # 参数说明
//
//   - req: Blades 模型请求
//
// # 返回值
//
// openai.AudioSpeechNewParams: OpenAI 音频生成参数
//
// # 参数说明
//
//   - Input: 要转换的文本（从消息历史提取）
//   - Model: TTS 模型名称
//   - Voice: 语音名称（必填）
//   - Instructions: 可选的发音指导（从 Instruction 提取）
//   - ResponseFormat: 输出音频格式
//   - StreamFormat: 流式格式
//   - Speed: 语速
func (m *audioModel) buildAudioParams(req *blades.ModelRequest) openai.AudioSpeechNewParams {
	params := openai.AudioSpeechNewParams{
		Input: promptFromMessages(req.Messages),
		Model: m.model,
		Voice: openai.AudioSpeechNewParamsVoice(m.config.Voice),
	}
	// 可选的发音指导
	if req.Instruction != nil {
		params.Instructions = param.NewOpt(req.Instruction.Text())
	}
	if m.config.ResponseFormat != "" {
		params.ResponseFormat = openai.AudioSpeechNewParamsResponseFormat(m.config.ResponseFormat)
	}
	if m.config.StreamFormat != "" {
		params.StreamFormat = openai.AudioSpeechNewParamsStreamFormat(m.config.StreamFormat)
	}
	if m.config.Speed > 0 {
		params.Speed = param.NewOpt(m.config.Speed)
	}
	if len(m.config.ExtraFields) > 0 {
		params.SetExtraFields(m.config.ExtraFields)
	}
	return params
}

// Generate 使用配置的 OpenAI 模型从文本生成音频。
//
// # 参数说明
//
//   - ctx: 上下文
//   - req: 模型请求，Messages 中包含要转换的文本
//
// # 返回值
//
//   - *blades.ModelResponse: 包含生成的音频数据
//   - error: 生成错误
//
// # 前置检查
//
//   - 请求不能为 nil
//   - 模型必须指定
//   - 语音必须指定
//
// # 处理流程
//
// 1. 验证请求和配置
// 2. 构建音频生成参数
// 3. 调用 OpenAI Audio API
// 4. 读取响应体为字节数据
// 5. 转换为 Blades 格式
//
// # 响应内容
//
//   - DataPart: 音频二进制数据
//   - Metadata: content_type, response_format
func (m *audioModel) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	// 验证请求
	if req == nil {
		return nil, ErrAudioRequestNil
	}
	if m.model == "" {
		return nil, ErrAudioModelRequired
	}
	if m.config.Voice == "" {
		return nil, ErrAudioVoiceRequired
	}
	// 构建参数
	params := m.buildAudioParams(req)
	// 调用 API
	resp, err := m.client.Audio.Speech.New(ctx, params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// 读取音频数据
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// 检查空响应
	if len(data) == 0 {
		return nil, ErrAudioGenerationEmpty
	}
	// 构建响应
	name := "audio." + strings.ToLower(string(params.ResponseFormat))
	mimeType := audioMimeType(params.ResponseFormat)
	message := blades.NewAssistantMessage(blades.StatusCompleted)
	message.Parts = append(message.Parts, blades.DataPart{
		Name:     name,
		Bytes:    data,
		MIMEType: mimeType,
	})
	// 保存元数据
	message.Metadata["content_type"] = resp.Header.Get("Content-Type")
	message.Metadata["response_format"] = params.ResponseFormat
	return &blades.ModelResponse{Message: message}, nil
}

// NewStreaming 为 API 兼容性包装 Generate 为单次 yield 的流。
//
// # 作用说明
//
// 音频生成是单次请求，不支持真正的流式输出。
// 此方法实现 blades.ModelProvider 接口。
func (m *audioModel) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	return func(yield func(*blades.ModelResponse, error) bool) {
		r, err := m.Generate(ctx, req)
		if err != nil {
			yield(nil, err)
			return
		}
		yield(r, nil)
	}
}

// audioMimeType 根据 OpenAI 音频格式返回 MIME 类型。
//
// # 参数说明
//
//   - format: OpenAI 音频格式
//
// # 返回值
//
// blades.MIMEType: 对应的 MIME 类型
//
// # 映射关系
//
//   - mp3 -> MIMEAudioMP3
//   - wav -> MIMEAudioWAV
//   - opus -> MIMEAudioOpus
//   - aac -> MIMEAudioAAC
//   - flac -> MIMEAudioFLAC
//   - pcm -> MIMEAudioPCM
//   - 其他（默认） -> MIMEAudioMP3
func audioMimeType(format openai.AudioSpeechNewParamsResponseFormat) blades.MIMEType {
	switch strings.ToLower(string(format)) {
	case "mp3":
		return blades.MIMEAudioMP3
	case "wav":
		return blades.MIMEAudioWAV
	case "opus":
		return blades.MIMEAudioOpus
	case "aac":
		return blades.MIMEAudioAAC
	case "flac":
		return blades.MIMEAudioFLAC
	case "pcm":
		return blades.MIMEAudioPCM
	}
	// 默认返回 MP3
	return blades.MIMEAudioMP3
}
