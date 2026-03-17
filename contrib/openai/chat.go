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
	"log"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"
)

// Config 持有 OpenAI 客户端的配置选项。
//
// # 配置字段说明
//
//   - BaseURL: API 基础 URL，可用于私有部署或代理（如 Azure OpenAI）
//   - APIKey: OpenAI API 密钥，也可通过环境变量设置
//   - Seed: 随机种子，用于复现结果
//   - MaxOutputTokens: 最大输出 token 数
//   - FrequencyPenalty: 频率惩罚（-2 到 2），降低高频词的出现
//   - PresencePenalty: 存在惩罚（-2 到 2），降低重复话题的出现
//   - Temperature: 温度参数（0-2），控制随机性
//   - TopP: 核采样累积概率（0-1）
//   - StopSequences: 停止序列列表
//   - ExtraFields: 额外字段，用于传递 SDK 未支持的参数
//   - RequestOptions: 额外的请求选项（如重试策略）
//   - ReasoningEffort: 推理努力程度，用于 o1 等推理模型（"low", "medium", "high"）
//
// # 使用示例
//
//	config := openai.Config{
//	    APIKey:          "sk-...",
//	    Temperature:     0.7,
//	    MaxOutputTokens: 2048,
//	    ReasoningEffort: "medium", // 用于 o1 模型
//	}
type Config struct {
	BaseURL          string
	APIKey           string
	Seed             int64
	MaxOutputTokens  int64
	FrequencyPenalty float64
	PresencePenalty  float64
	Temperature      float64
	TopP             float64
	StopSequences    []string
	ExtraFields      map[string]any
	RequestOptions   []option.RequestOption
	ReasoningEffort  shared.ReasoningEffort
}

// chatModel 实现了 blades.ModelProvider 接口，用于 OpenAI 兼容的聊天模型。
//
// # 结构说明
//
//   - model: 模型名称，如 "gpt-4o", "gpt-4-turbo", "o1-preview"
//   - config: 客户端配置
//   - client: OpenAI SDK 客户端实例
//
// # 支持的模型
//
//   - GPT-4 系列：gpt-4o, gpt-4-turbo, gpt-4
//   - GPT-3.5 系列：gpt-3.5-turbo
//   - O1 系列：o1-preview, o1-mini（推理模型）
//   - 兼容模型：通过 BaseURL 配置的第三方模型（如 Azure OpenAI）
//
// # 实现接口
//
// chatModel 实现了 blades.ModelProvider 接口：
//   - Name(): 返回模型名称
//   - Generate(): 非流式生成
//   - NewStreaming(): 流式生成
type chatModel struct {
	model  string
	config Config
	client openai.Client
}

// NewModel 构建一个 OpenAI 聊天模型提供者。
//
// # 参数说明
//
//   - model: 模型名称，如 "gpt-4o"
//   - config: 配置选项
//
// # 返回值
//
// blades.ModelProvider: 模型提供者接口实例
//
// # 环境变量
//
//   - OPENAI_API_KEY: API 密钥（如果 config.APIKey 未设置）
//   - OPENAI_BASE_URL: API 基础 URL（如果 config.BaseURL 未设置）
//
// # 使用示例
//
//	provider := openai.NewModel("gpt-4o", openai.Config{
//	    APIKey:      "sk-...",
//	    Temperature: 0.7,
//	})
//	resp, err := provider.Generate(ctx, request)
func NewModel(model string, config Config) blades.ModelProvider {
	opts := config.RequestOptions
	// 设置 base URL 和 API key
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}
	if config.APIKey != "" {
		opts = append(opts, option.WithAPIKey(config.APIKey))
	}
	return &chatModel{
		model:  model,
		config: config,
		client: openai.NewClient(opts...),
	}
}

// Name 返回模型名称。
//
// 这是 blades.ModelProvider 接口的必需方法。
func (m *chatModel) Name() string {
	return m.model
}

// Generate 执行非流式聊天完成请求。
//
// # 参数说明
//
//   - ctx: 上下文，用于控制超时和取消
//   - req: 模型请求，包含消息历史、工具定义等
//
// # 返回值
//
//   - *blades.ModelResponse: 模型响应
//   - error: 请求错误
//
// # 处理流程
//
// 1. 转换请求参数（toChatCompletionParams）
// 2. 调用 OpenAI Chat Completions API
// 3. 转换响应（choiceToResponse）
func (m *chatModel) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	// 转换请求参数
	params, err := m.toChatCompletionParams(false, req)
	if err != nil {
		return nil, err
	}
	// 调用 API
	chatResponse, err := m.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, err
	}
	// 转换响应
	res, err := choiceToResponse(ctx, params, chatResponse)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// NewStreaming 流式聊天完成请求，将每个选择增量转换为 ModelResponse。
//
// # 参数说明
//
//   - ctx: 上下文
//   - req: 模型请求
//
// # 返回值
//
// blades.Generator，通过 yield 回调返回流式响应。
//
// # 流式处理流程
//
// 1. 构建请求参数（设置 Stream = true）
// 2. 创建流式请求（NewStreaming）
// 3. 遍历流式 chunk：
//   - 累积到 ChatCompletionAccumulator
//   - 提取增量响应并 yield
//
// 4. 流式结束后，返回最终完整响应
//
// # ChatCompletionAccumulator
//
// OpenAI SDK 提供的累积器，自动处理流式响应的累积和合并。
// 可以通过 acc.ChatCompletion 访问累积的完整响应。
func (m *chatModel) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	return func(yield func(*blades.ModelResponse, error) bool) {
		// 构建流式请求参数
		params, err := m.toChatCompletionParams(true, req)
		if err != nil {
			yield(nil, err)
			return
		}
		// 创建流式请求
		streaming := m.client.Chat.Completions.NewStreaming(ctx, params)
		defer streaming.Close()
		// 累积器，自动处理 chunk 合并
		acc := openai.ChatCompletionAccumulator{}
		// 遍历流式 chunk
		for streaming.Next() {
			chunk := streaming.Current()
			acc.AddChunk(chunk)
			// 转换增量响应
			message, err := chunkChoiceToResponse(ctx, chunk.Choices)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(message, nil) {
				return
			}
		}
		// 检查流式错误
		if err := streaming.Err(); err != nil {
			yield(nil, err)
			return
		}
		// 返回最终响应
		finalResponse, err := choiceToResponse(ctx, params, &acc.ChatCompletion)
		if err != nil {
			yield(nil, err)
			return
		}
		yield(finalResponse, nil)
	}
}

// toChatCompletionParams 将 Blades 请求转换为 OpenAI 聊天完成参数。
//
// # 参数说明
//
//   - isStreaming: 是否为流式请求
//   - req: Blades 模型请求
//
// # 返回值
//
//   - openai.ChatCompletionNewParams: OpenAI API 参数
//   - error: 转换错误
//
// # 转换内容
//
// 1. 基础参数：Model, Messages, Tools
// 2. 生成参数：Temperature, TopP, MaxTokens, FrequencyPenalty, PresencePenalty
// 3. 高级参数：Seed, StopSequences, ResponseFormat, ReasoningEffort
// 4. 消息历史：按角色转换（System, User, Assistant, Tool）
func (m *chatModel) toChatCompletionParams(isStreaming bool, req *blades.ModelRequest) (openai.ChatCompletionNewParams, error) {
	// 转换工具定义
	tools, err := toTools(req.Tools)
	if err != nil {
		return openai.ChatCompletionNewParams{}, err
	}
	params := openai.ChatCompletionNewParams{
		Tools:           tools,
		Model:           m.model,
		ReasoningEffort: m.config.ReasoningEffort,
		Messages:        make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages)),
	}
	// 设置生成参数（只有值 > 0 时才设置）
	if m.config.Seed > 0 {
		params.Seed = param.NewOpt(m.config.Seed)
	}
	if m.config.MaxOutputTokens > 0 {
		params.MaxCompletionTokens = param.NewOpt(m.config.MaxOutputTokens)
	}
	if m.config.FrequencyPenalty > 0 {
		params.FrequencyPenalty = param.NewOpt(m.config.FrequencyPenalty)
	}
	if m.config.PresencePenalty > 0 {
		params.PresencePenalty = param.NewOpt(m.config.PresencePenalty)
	}
	if m.config.Temperature > 0 {
		params.Temperature = param.NewOpt(m.config.Temperature)
	}
	if m.config.TopP > 0 {
		params.TopP = param.NewOpt(m.config.TopP)
	}
	if len(m.config.StopSequences) > 0 {
		params.Stop = openai.ChatCompletionNewParamsStopUnion{OfStringArray: m.config.StopSequences}
	}
	// 设置额外字段
	if len(m.config.ExtraFields) > 0 {
		params.SetExtraFields(m.config.ExtraFields)
	}
	// 设置响应格式（Structured Outputs）
	if req.OutputSchema != nil {
		schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
			Name:   "structured_outputs",
			Schema: req.OutputSchema,
			Strict: openai.Bool(true),
		}
		if req.OutputSchema.Title != "" {
			schemaParam.Name = req.OutputSchema.Title
		}
		if req.OutputSchema.Description != "" {
			schemaParam.Description = openai.String(req.OutputSchema.Description)
		}
		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{JSONSchema: schemaParam},
		}
	}
	// 流式选项
	if isStreaming {
		params.StreamOptions = openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: openai.Bool(true),
		}
	}
	// 转换消息历史
	if req.Instruction != nil {
		params.Messages = append(params.Messages, openai.SystemMessage(toTextParts(req.Instruction)))
	}
	for _, msg := range req.Messages {
		switch msg.Role {
		case blades.RoleUser:
			// 用户消息支持多模态内容
			params.Messages = append(params.Messages, openai.UserMessage(toContentParts(msg)))
		case blades.RoleAssistant:
			// 助手消息仅支持文本
			params.Messages = append(params.Messages, openai.AssistantMessage(msg.Text()))
		case blades.RoleSystem:
			// 系统消息
			params.Messages = append(params.Messages, openai.SystemMessage(toTextParts(msg)))
		case blades.RoleTool:
			// 工具调用消息
			params.Messages = append(params.Messages, toToolCallMessage(msg))
			// 添加工具响应
			for _, part := range msg.Parts {
				switch v := any(part).(type) {
				case blades.ToolPart:
					params.Messages = append(params.Messages, openai.ToolMessage(v.Response, v.ID))
				}
			}
		}
	}
	return params, nil
}

// toToolCallMessage 将 Blades 工具消息转换为 OpenAI 工具调用消息。
//
// # 作用说明
//
// 将助手发出的工具调用请求转换为 OpenAI API 的格式。
// OpenAI 要求工具调用以 ToolCalls 列表的形式发送。
//
// # 参数说明
//
//   - msg: Blades 消息，包含 ToolPart 列表
//
// # 返回值
//
// openai.ChatCompletionMessageParamUnion: OpenAI 消息参数
func toToolCallMessage(msg *blades.Message) openai.ChatCompletionMessageParamUnion {
	toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		switch v := any(part).(type) {
		case blades.ToolPart:
			toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
					ID: v.ID,
					Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name:      v.Name,
						Arguments: v.Request,
					},
				},
			})
		}
	}
	return openai.ChatCompletionMessageParamUnion{
		OfAssistant: &openai.ChatCompletionAssistantMessageParam{
			ToolCalls: toolCalls,
		},
	}
}

// toTools 将 Blades 工具列表转换为 OpenAI 工具参数。
//
// # 参数说明
//
//   - tools: Blades 工具列表
//
// # 返回值
//
//   - []openai.ChatCompletionToolUnionParam: OpenAI 工具参数
//   - error: 转换错误
//
// # OpenAI 工具格式
//
// OpenAI 的工具通过 Function Definition 定义，包含：
//   - Name: 函数名称
//   - Description: 函数描述
//   - Parameters: 输入参数的 JSON Schema
func toTools(tools []tools.Tool) ([]openai.ChatCompletionToolUnionParam, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	params := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		fn := openai.FunctionDefinitionParam{
			Name: tool.Name(),
		}
		if tool.Description() != "" {
			fn.Description = openai.String(tool.Description())
		}
		// 转换 InputSchema 为 JSON 并反序列化为 OpenAI 格式
		if tool.InputSchema() != nil {
			b, err := json.Marshal(tool.InputSchema())
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(b, &fn.Parameters); err != nil {
				return nil, err
			}
		}
		unionParam := openai.ChatCompletionToolUnionParam{
			OfFunction: &openai.ChatCompletionFunctionToolParam{
				Function: fn,
			},
		}
		params = append(params, unionParam)
	}
	return params, nil
}

// toTextParts 将消息转换为纯文本内容（用于 System/Assistant 消息）。
//
// # 作用说明
//
// System 和 Assistant 消息通常只包含文本内容。
// 此函数提取消息中的 TextPart，忽略其他类型。
func toTextParts(message *blades.Message) []openai.ChatCompletionContentPartTextParam {
	parts := make([]openai.ChatCompletionContentPartTextParam, 0, len(message.Parts))
	for _, part := range message.Parts {
		switch v := part.(type) {
		case blades.TextPart:
			parts = append(parts, openai.ChatCompletionContentPartTextParam{Text: v.Text})
		}
	}
	return parts
}

// toContentParts 将消息转换为 OpenAI 内容片段（用于 User 消息）。
//
// # 作用说明
//
// User 消息支持多模态输入（文本、图片、音频、文件）。
// 此函数根据 MIME 类型将 Blades Part 转换为对应的 OpenAI 内容类型。
//
// # 支持的类型
//
//   - TextPart -> TextContentPart
//   - FilePart:
//     - image/* -> ImageContentPart
//     - audio/* -> InputAudioContentPart
//   - DataPart:
//     - image/* -> ImageContentPart（base64 编码）
//     - audio/* -> InputAudioContentPart（base64 编码）
//     - 其他 -> FileContentPart
func toContentParts(message *blades.Message) []openai.ChatCompletionContentPartUnionParam {
	parts := make([]openai.ChatCompletionContentPartUnionParam, 0, len(message.Parts))
	for _, part := range message.Parts {
		switch v := part.(type) {
		case blades.TextPart:
			// 文本内容
			parts = append(parts, openai.TextContentPart(v.Text))
		case blades.FilePart:
			// 文件引用，根据 MIME 类型处理
			switch v.MIMEType.Type() {
			case "image":
				// 图片 URI
				parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL: v.URI,
				}))
			case "audio":
				// 音频 URI
				parts = append(parts, openai.InputAudioContentPart(openai.ChatCompletionContentPartInputAudioInputAudioParam{
					Data:   v.URI,
					Format: v.MIMEType.Format(),
				}))
			default:
				log.Println("failed to process file part with MIME type:", v.MIMEType)
			}
		case blades.DataPart:
			// 内联数据，根据 MIME 类型处理
			switch v.MIMEType.Type() {
			case "image":
				// 图片 base64
				mimeType := string(v.MIMEType)
				base64Data := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(v.Bytes)
				parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL: base64Data,
				}))
			case "audio":
				// 音频 base64
				parts = append(parts, openai.InputAudioContentPart(openai.ChatCompletionContentPartInputAudioInputAudioParam{
					Data:   "data:;base64," + base64.StdEncoding.EncodeToString(v.Bytes),
					Format: v.MIMEType.Format(),
				}))
			default:
				// 其他文件类型
				fileParam := openai.ChatCompletionContentPartFileFileParam{
					FileData: param.NewOpt(base64.StdEncoding.EncodeToString(v.Bytes)),
					Filename: param.NewOpt(v.Name),
				}
				parts = append(parts, openai.FileContentPart(fileParam))
			}
		}
	}
	return parts
}

// choiceToToolCalls 将非流式响应的选择转换为工具调用响应。
//
// # 参数说明
//
//   - ctx: 上下文
//   - tools: 工具定义列表（未使用）
//   - choices: OpenAI 响应选择
//
// # 返回值
//
//   - *blades.ModelResponse: Blades 响应
//   - error: 转换错误
func choiceToToolCalls(ctx context.Context, tools []*tools.Tool, choices []openai.ChatCompletionChoice) (*blades.ModelResponse, error) {
	msg := &blades.Message{
		Role:   blades.RoleTool,
		Status: blades.StatusCompleted,
	}
	for _, choice := range choices {
		if choice.Message.Content != "" {
			msg.Parts = append(msg.Parts, blades.TextPart{Text: choice.Message.Content})
		}
		if len(choice.Message.ToolCalls) > 0 {
			for _, call := range choice.Message.ToolCalls {
				msg.Role = blades.RoleTool
				msg.Parts = append(msg.Parts, blades.NewToolPart(call.ID, call.Function.Name, call.Function.Arguments))
			}
		}
	}
	return &blades.ModelResponse{
		Message: msg,
	}, nil
}

// choiceToResponse 将非流式响应转换为 Blades ModelResponse。
//
// # 参数说明
//
//   - ctx: 上下文
//   - params: 请求参数（未使用）
//   - cc: OpenAI 聊天完成响应
//
// # 返回值
//
//   - *blades.ModelResponse: Blades 响应
//   - error: 转换错误
//
// # 处理内容
//
// 1. Token 用量：InputTokens, OutputTokens, TotalTokens
// 2. 文本内容：Content 字段
// 3. 音频内容：Audio.Data（base64 解码）
// 4. 工具调用：ToolCalls 列表
// 5. 结束原因：FinishReason
func choiceToResponse(ctx context.Context, params openai.ChatCompletionNewParams, cc *openai.ChatCompletion) (*blades.ModelResponse, error) {
	message := blades.NewAssistantMessage(blades.StatusCompleted)
	message.TokenUsage = blades.TokenUsage{
		InputTokens:  cc.Usage.PromptTokens,
		OutputTokens: cc.Usage.CompletionTokens,
		TotalTokens:  cc.Usage.TotalTokens,
	}
	for _, choice := range cc.Choices {
		if choice.Message.Content != "" {
			message.Parts = append(message.Parts, blades.TextPart{Text: choice.Message.Content})
		}
		if choice.Message.Audio.Data != "" {
			// 解码 base64 音频数据
			bytes, err := base64.StdEncoding.DecodeString(choice.Message.Audio.Data)
			if err != nil {
				return nil, err
			}
			message.Parts = append(message.Parts, blades.DataPart{Bytes: bytes})
		}
		if choice.Message.Refusal != "" {
			// TODO: 将拒绝代码映射到特定错误类型
		}
		if choice.FinishReason != "" {
			message.FinishReason = choice.FinishReason
		}
		// 处理工具调用
		for _, call := range choice.Message.ToolCalls {
			message.Role = blades.RoleTool
			message.Parts = append(message.Parts, blades.NewToolPart(call.ID, call.Function.Name, call.Function.Arguments))
		}
	}
	return &blades.ModelResponse{Message: message}, nil
}

// chunkChoiceToResponse 将流式 chunk 转换为 Blades ModelResponse。
//
// # 参数说明
//
//   - ctx: 上下文
//   - choices: OpenAI chunk 选择
//
// # 返回值
//
//   - *blades.ModelResponse: Blades 响应
//   - error: 转换错误
//
// # 与 choiceToResponse 的区别
//
// chunkChoiceToResponse 处理增量响应：
//   - 使用 StatusIncomplete 状态
//   - 处理 Delta 字段而非完整字段
//   - 不包含 Token 用量（chunk 中无此信息）
func chunkChoiceToResponse(ctx context.Context, choices []openai.ChatCompletionChunkChoice) (*blades.ModelResponse, error) {
	message := blades.NewAssistantMessage(blades.StatusIncomplete)
	for _, choice := range choices {
		if choice.Delta.Content != "" {
			message.Parts = append(message.Parts, blades.TextPart{Text: choice.Delta.Content})
		}
		if choice.Delta.Refusal != "" {
			// TODO: 将拒绝代码映射到特定错误类型
		}
		if choice.FinishReason != "" {
			message.FinishReason = choice.FinishReason
		}
		// 处理流式工具调用
		for _, call := range choice.Delta.ToolCalls {
			message.Role = blades.RoleTool
			message.Parts = append(message.Parts, blades.NewToolPart(call.ID, call.Function.Name, call.Function.Arguments))
		}
	}
	return &blades.ModelResponse{Message: message}, nil
}
