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
	"encoding/base64"
	"fmt"

	"github.com/go-kratos/blades"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
)

// ImageConfig 持有图像生成的配置选项。
//
// # 配置字段说明
//
//   - BaseURL: API 基础 URL
//   - APIKey: OpenAI API 密钥
//   - Background: 背景类型（"transparent" 或 "opaque"）
//   - Size: 图像尺寸（如 "1024x1024", "1792x1024", "1024x1792"）
//   - Quality: 图像质量（"standard" 或 "hd"）
//   - ResponseFormat: 响应格式（"url" 或 "b64_json"）
//   - OutputFormat: 输出格式（"png", "jpeg", "webp"）
//   - Moderation: 审核级别（"low" 或 "auto"）
//   - Style: 风格（"vivid" 或 "natural"）
//   - User: 用户标识，用于内容审核
//   - N: 生成图像数量
//   - PartialImages: 部分图像数量（用于渐进式加载）
//   - OutputCompression: 输出压缩质量（0-100）
//   - ExtraFields: 额外字段
//   - RequestOptions: 额外请求选项
//
// # DALL-E 3 支持尺寸
//
//   - 1024x1024: 正方形（默认）
//   - 1792x1024: 横向
//   - 1024x1792: 纵向
//
// # 使用示例
//
//	config := openai.ImageConfig{
//	    APIKey:         "sk-...",
//	    Size:           "1024x1024",
//	    Quality:        "hd",
//	    Style:          "vivid",
//	    ResponseFormat: "b64_json",
//	}
type ImageConfig struct {
	BaseURL           string
	APIKey            string
	Background        string
	Size              string
	Quality           string
	ResponseFormat    string
	OutputFormat      string
	Moderation        string
	Style             string
	User              string
	N                 int64
	PartialImages     int64
	OutputCompression int64
	ExtraFields       map[string]any
	RequestOptions    []option.RequestOption
}

// imageModel 调用 OpenAI 的图像生成 API。
//
// # 结构说明
//
//   - model: 模型名称，如 "dall-e-3" 或 "dall-e-2"
//   - config: 图像生成配置
//   - client: OpenAI SDK 客户端
//
// # 支持的模型
//
//   - dall-e-3: 最新模型，支持更高质量的图像和更详细的提示
//   - dall-e-2: 旧版模型，速度更快但质量略低
//
// # 实现接口
//
// imageModel 实现了 blades.ModelProvider 接口：
//   - Name(): 返回模型名称
//   - Generate(): 生成图像
//   - NewStreaming(): 流式生成（包装为单次 yield）
type imageModel struct {
	model  string
	config ImageConfig
	client openai.Client
}

// NewImage 创建一个新的 imageModel 实例。
//
// # 参数说明
//
//   - model: 模型名称，如 "dall-e-3"
//   - config: 图像生成配置
//
// # 返回值
//
// blades.ModelProvider: 图像模型提供者
//
// # 使用示例
//
//	provider := openai.NewImage("dall-e-3", openai.ImageConfig{
//	    APIKey: "sk-...",
//	    Size:   "1024x1024",
//	})
//	resp, err := provider.Generate(ctx, &blades.ModelRequest{
//	    Messages: []*blades.Message{
//	        {Role: blades.RoleUser, Parts: []blades.Part{
//	            blades.TextPart{Text: "一只在太空的猫"},
//	        }},
//	    },
//	})
func NewImage(model string, config ImageConfig) blades.ModelProvider {
	opts := config.RequestOptions
	// 设置 base URL 和 API key
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}
	if config.APIKey != "" {
		opts = append(opts, option.WithAPIKey(config.APIKey))
	}
	return &imageModel{
		model:  model,
		config: config,
		client: openai.NewClient(opts...),
	}
}

// Name 返回 OpenAI 图像模型的名称。
//
// 这是 blades.ModelProvider 接口的必需方法。
func (m *imageModel) Name() string {
	return m.model
}

// Generate 使用配置的 OpenAI 模型生成图像。
//
// # 参数说明
//
//   - ctx: 上下文
//   - req: 模型请求，Messages 中包含提示文本
//
// # 返回值
//
//   - *blades.ModelResponse: 包含生成的图像数据
//   - error: 生成错误
//
// # 处理流程
//
// 1. 构建图像生成参数（buildGenerateParams）
// 2. 调用 OpenAI Images API
// 3. 转换响应为 Blades 格式（toImageResponse）
//
// # 响应内容
//
//   - DataPart: base64 编码的图像数据（如果 response_format=b64_json）
//   - FilePart: 图像 URL（如果 response_format=url）
//   - Metadata: 图像元数据（尺寸、质量、修订后的提示等）
func (m *imageModel) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	// 构建请求参数
	params, err := m.buildGenerateParams(req)
	if err != nil {
		return nil, err
	}
	// 调用 API
	res, err := m.client.Images.Generate(ctx, params)
	if err != nil {
		return nil, err
	}
	// 转换响应
	return toImageResponse(res)
}

// NewStreaming 为 API 兼容性包装 Generate 为单次 yield 的流。
//
// # 作用说明
//
// 图像生成是单次请求，不支持真正的流式输出。
// 此方法实现 blades.ModelProvider 接口，将结果包装为 Generator。
func (m *imageModel) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	return func(yield func(*blades.ModelResponse, error) bool) {
		message, err := m.Generate(ctx, req)
		if err != nil {
			yield(nil, err)
			return
		}
		yield(message, nil)
	}
}

// buildGenerateParams 构建 OpenAI 图像生成参数。
//
// # 参数说明
//
//   - req: Blades 模型请求
//
// # 返回值
//
//   - openai.ImageGenerateParams: OpenAI 图像生成参数
//   - error: 参数构建错误
//
// # 提示提取
//
// 使用 promptFromMessages() 从消息历史中提取文本提示。
// DALL-E 只需要简单的文本提示，不需要复杂的消息结构。
func (m *imageModel) buildGenerateParams(req *blades.ModelRequest) (openai.ImageGenerateParams, error) {
	params := openai.ImageGenerateParams{
		Prompt: promptFromMessages(req.Messages),
		Model:  openai.ImageModel(m.model),
	}
	// 设置可选参数
	if m.config.Background != "" {
		params.Background = openai.ImageGenerateParamsBackground(m.config.Background)
	}
	if m.config.Size != "" {
		params.Size = openai.ImageGenerateParamsSize(m.config.Size)
	}
	if m.config.Quality != "" {
		params.Quality = openai.ImageGenerateParamsQuality(m.config.Quality)
	}
	if m.config.ResponseFormat != "" {
		params.ResponseFormat = openai.ImageGenerateParamsResponseFormat(m.config.ResponseFormat)
	}
	if m.config.OutputFormat != "" {
		params.OutputFormat = openai.ImageGenerateParamsOutputFormat(m.config.OutputFormat)
	}
	if m.config.Moderation != "" {
		params.Moderation = openai.ImageGenerateParamsModeration(m.config.Moderation)
	}
	if m.config.Style != "" {
		params.Style = openai.ImageGenerateParamsStyle(m.config.Style)
	}
	if m.config.User != "" {
		params.User = param.NewOpt(m.config.User)
	}
	if m.config.N > 0 {
		params.N = param.NewOpt(m.config.N)
	}
	if m.config.PartialImages > 0 {
		params.PartialImages = param.NewOpt(m.config.PartialImages)
	}
	if m.config.OutputCompression > 0 {
		params.OutputCompression = param.NewOpt(m.config.OutputCompression)
	}
	if len(m.config.ExtraFields) > 0 {
		params.SetExtraFields(m.config.ExtraFields)
	}
	return params, nil
}

// toImageResponse 将 OpenAI 图像响应转换为 Blades 响应。
//
// # 参数说明
//
//   - res: OpenAI 图像生成响应
//
// # 返回值
//
//   - *blades.ModelResponse: Blades 响应
//   - error: 转换错误
//
// # 处理内容
//
// 1. 提取元数据：size, quality, background, output_format, created
// 2. 遍历生成的图像：
//   - B64JSON: base64 解码为 DataPart
//   - URL: 作为 FilePart
//   - RevisedPrompt: 保存到 Metadata（DALL-E 3 会优化提示）
//
// # 图像命名
//
// 生成的图像按顺序命名：image-1, image-2, ...
func toImageResponse(res *openai.ImagesResponse) (*blades.ModelResponse, error) {
	message := blades.NewAssistantMessage(blades.StatusCompleted)
	// 保存元数据
	message.Metadata["size"] = res.Size
	message.Metadata["quality"] = res.Quality
	message.Metadata["background"] = res.Background
	message.Metadata["output_format"] = res.OutputFormat
	message.Metadata["created"] = res.Created
	// 确定 MIME 类型
	mimeType := imageMimeType(res.OutputFormat)
	// 处理每张生成的图像
	for i, img := range res.Data {
		name := fmt.Sprintf("image-%d", i+1)
		if img.B64JSON != "" {
			// base64 编码的图像数据
			data, err := base64.StdEncoding.DecodeString(img.B64JSON)
			if err != nil {
				return nil, fmt.Errorf("openai/image: decode response: %w", err)
			}
			message.Parts = append(message.Parts, blades.DataPart{
				Name:     name,
				Bytes:    data,
				MIMEType: mimeType,
			})
		}
		if img.URL != "" {
			// 图像 URL
			message.Parts = append(message.Parts, blades.FilePart{
				Name:     name,
				URI:      img.URL,
				MIMEType: mimeType,
			})
		}
		// 保存 DALL-E 3 优化后的提示
		if img.RevisedPrompt != "" {
			key := fmt.Sprintf("%s_revised_prompt_%d", name, i+1)
			message.Metadata[key] = img.RevisedPrompt
		}
	}
	return &blades.ModelResponse{Message: message}, nil
}

// imageMimeType 根据 OpenAI 输出格式返回 MIME 类型。
//
// # 参数说明
//
//   - format: OpenAI 输出格式
//
// # 返回值
//
// blades.MIMEType: 对应的 MIME 类型
//
// # 映射关系
//
//   - jpeg -> MIMEImageJPEG
//   - webp -> MIMEImageWEBP
//   - 其他（默认 png） -> MIMEImagePNG
func imageMimeType(format openai.ImagesResponseOutputFormat) blades.MIMEType {
	switch format {
	case openai.ImagesResponseOutputFormatJPEG:
		return blades.MIMEImageJPEG
	case openai.ImagesResponseOutputFormatWebP:
		return blades.MIMEImageWEBP
	default:
		return blades.MIMEImagePNG
	}
}
