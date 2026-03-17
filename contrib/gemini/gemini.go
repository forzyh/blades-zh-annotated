// Package gemini 提供了 Google Gemini API 的客户端实现。
// 本包封装了官方的 google.golang.org/genai SDK，为 blades 框架提供统一的模型接口。
//
// # 核心功能
//
//   - 支持文本生成（Generate）和流式响应（NewStreaming）
//   - 支持工具调用（Function Calling）
//   - 支持多模态输入（文本、图片、文件）
//   - 支持思维链（ThinkingConfig）
//
// # 使用示例
//
//	// 创建 Gemini 模型实例
//	provider, err := gemini.NewModel(ctx, "gemini-2.0-flash", gemini.Config{
//	    ClientConfig: genai.ClientConfig{
//	        APIKey: "your-api-key",
//	    },
//	    Temperature: 0.7,
//	})
//
//	// 调用模型生成响应
//	resp, err := provider.Generate(ctx, &blades.ModelRequest{
//	    Messages: []*blades.Message{
//	        {Role: blades.RoleUser, Parts: []blades.Part{blades.TextPart{Text: "你好"}}},
//	    },
//	})
package gemini

import (
	"context"
	"fmt"

	"github.com/go-kratos/blades"
	"google.golang.org/genai"
)

// Config 持有 Gemini 模型的配置。
//
// # 配置字段说明
//
//   - ClientConfig: Gemini 客户端基础配置（包含 APIKey、Endpoint 等）
//   - Seed: 随机种子，用于复现结果
//   - MaxOutputTokens: 最大输出 token 数
//   - Temperature: 温度参数，控制生成随机性
//   - TopP: 核采样累积概率阈值
//   - TopK: Top-K 采样参数
//   - PresencePenalty: 存在惩罚，降低重复话题的出现概率
//   - FrequencyPenalty: 频率惩罚，降低高频词的出现概率
//   - StopSequences: 停止序列列表
//   - ThinkingConfig: 思维链配置，启用模型的"思考"能力
//
// # 使用示例
//
//	config := gemini.Config{
//	    ClientConfig: genai.ClientConfig{
//	        APIKey: "your-api-key",
//	    },
//	    Temperature:     0.7,
//	    MaxOutputTokens: 2048,
//	}
type Config struct {
	genai.ClientConfig
	Seed             int32
	MaxOutputTokens  int32
	Temperature      float32
	TopP             float32
	TopK             float32
	PresencePenalty  float32
	FrequencyPenalty float32
	StopSequences    []string
	ThinkingConfig   *genai.ThinkingConfig
}

// Gemini 提供了统一的 Gemini API 访问接口。
//
// # 结构说明
//
//   - model: 模型名称，如 "gemini-2.0-flash" 或 "gemini-1.5-pro"
//   - config: 客户端配置
//   - client: Gemini SDK 客户端实例
//
// # 实现接口
//
// Gemini 实现了 blades.ModelProvider 接口，提供以下方法：
//   - Name(): 返回模型名称
//   - Generate(): 非流式生成
//   - NewStreaming(): 流式生成
type Gemini struct {
	model  string
	config Config
	client *genai.Client
}

// NewModel 创建一个新的 Gemini 模型提供者。
//
// # 参数说明
//
//   - ctx: 上下文，用于 SDK 初始化
//   - model: 模型名称
//   - config: 配置选项
//
// # 返回值
//
//   - blades.ModelProvider: 模型提供者接口实例
//   - error: 初始化错误
//
// # 初始化逻辑
//
// 1. 使用 genai.NewClient 创建 SDK 客户端
// 2. 返回 Gemini 实例
//
// # 使用示例
//
//	provider, err := gemini.NewModel(ctx, "gemini-2.0-flash", gemini.Config{
//	    ClientConfig: genai.ClientConfig{
//	        APIKey: "your-api-key",
//	    },
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewModel(ctx context.Context, model string, config Config) (blades.ModelProvider, error) {
	client, err := genai.NewClient(ctx, &config.ClientConfig)
	if err != nil {
		return nil, err
	}
	return &Gemini{
		model:  model,
		config: config,
		client: client,
	}, nil
}

// Name 返回 Gemini 模型的名称。
//
// 这是 blades.ModelProvider 接口的必需方法。
func (m *Gemini) Name() string {
	return m.model
}

// Generate 使用 Gemini API 生成内容。
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
// 1. 转换消息格式（convertMessageToGenAI）
// 2. 构建生成配置（toGenerateConfig）
// 3. 调用 Gemini API 生成内容
// 4. 转换响应格式（convertGenAIToBlades）
func (m *Gemini) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	// 转换消息为 Gemini 格式
	system, contents, err := convertMessageToGenAI(req)
	if err != nil {
		return nil, err
	}
	// 构建生成配置
	config, err := m.toGenerateConfig(req)
	if err != nil {
		return nil, err
	}
	config.SystemInstruction = system
	// 调用 Gemini API
	resp, err := m.client.Models.GenerateContent(ctx, m.model, contents, config)
	if err != nil {
		return nil, err
	}
	// 转换为 Blades 格式
	return convertGenAIToBlades(resp, blades.StatusCompleted)
}

// toGenerateConfig 将 Blades 请求转换为 Gemini 生成配置。
//
// # 作用说明
//
// 提取请求中的配置参数和工具定义，构建 Gemini SDK 的 GenerateContentConfig。
//
// # 参数说明
//
//   - req: Blades 模型请求
//
// # 返回值
//
//   - *genai.GenerateContentConfig: Gemini 生成配置
//   - error: 转换错误
//
// # 配置项说明
//
//   - Temperature: 控制随机性（0-1 之间）
//   - TopP: 核采样参数（0-1 之间）
//   - TopK: Top-K 采样参数
//   - MaxOutputTokens: 最大输出长度
//   - StopSequences: 停止序列
//   - PresencePenalty: 存在惩罚（-2 到 2 之间）
//   - FrequencyPenalty: 频率惩罚（-2 到 2 之间）
//   - Seed: 随机种子
//   - ThinkingConfig: 思维链配置
//   - Tools: 工具定义列表
func (m *Gemini) toGenerateConfig(req *blades.ModelRequest) (*genai.GenerateContentConfig, error) {
	var config genai.GenerateContentConfig
	// 只有当配置值大于 0 时才设置，使用 API 默认值
	if m.config.Temperature > 0 {
		config.Temperature = &m.config.Temperature
	}
	if m.config.TopP > 0 {
		config.TopP = &m.config.TopP
	}
	if m.config.TopK > 0 {
		config.TopK = &m.config.TopK
	}
	if m.config.MaxOutputTokens > 0 {
		config.MaxOutputTokens = m.config.MaxOutputTokens
	}
	if len(m.config.StopSequences) > 0 {
		config.StopSequences = m.config.StopSequences
	}
	if m.config.PresencePenalty > 0 {
		config.PresencePenalty = &m.config.PresencePenalty
	}
	if m.config.FrequencyPenalty > 0 {
		config.FrequencyPenalty = &m.config.FrequencyPenalty
	}
	if m.config.Seed > 0 {
		config.Seed = &m.config.Seed
	}
	if m.config.ThinkingConfig != nil {
		config.ThinkingConfig = m.config.ThinkingConfig
	}
	// 转换工具定义
	if len(req.Tools) > 0 {
		tools, err := convertBladesToolsToGenAI(req.Tools)
		if err != nil {
			return nil, fmt.Errorf("converting tools: %w", err)
		}
		config.Tools = tools
	}
	return &config, nil
}

// NewStreaming 执行流式请求并返回助手响应的流。
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
// 1. 构建请求参数和配置
// 2. 创建流式生成器（GenerateContentStream）
// 3. 遍历流式 chunk：
//   - 转换每个 chunk 为 Blades 格式
//   - yield 增量响应
//   - 累积完整响应
//
// 4. 流式结束后，返回最终完整响应
//
// # 累积响应逻辑
//
// 由于流式响应是增量的，需要累积所有 chunk 才能得到完整响应。
// 累积逻辑处理：
//   - 第一个 chunk 作为初始响应
//   - 后续 chunk 的 Parts 追加到初始响应
//   - FinishReason 以最后一个 chunk 为准
//
// # 使用示例
//
//	streaming := provider.NewStreaming(ctx, request)
//	for resp, err := range streaming(func(yield) { ... }) {
//	    if err != nil {
//	        // 处理错误
//	    }
//	    // 处理增量响应
//	    fmt.Print(resp.Message.Text())
//	}
func (m *Gemini) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	return func(yield func(*blades.ModelResponse, error) bool) {
		// 转换消息为 Gemini 格式
		system, contents, err := convertMessageToGenAI(req)
		if err != nil {
			yield(nil, err)
			return
		}
		// 构建生成配置
		config, err := m.toGenerateConfig(req)
		if err != nil {
			yield(nil, err)
			return
		}
		config.SystemInstruction = system
		// 创建流式生成器
		streaming := m.client.Models.GenerateContentStream(ctx, m.model, contents, config)
		var accumulatedResponse *genai.GenerateContentResponse
		// 遍历流式 chunk
		for chunk, err := range streaming {
			if err != nil {
				yield(nil, err)
				return
			}
			// 转换为 Blades 格式并 yield
			response, err := convertGenAIToBlades(chunk, blades.StatusIncomplete)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(response, nil) {
				return
			}
			// 累积响应
			if accumulatedResponse == nil {
				accumulatedResponse = chunk
			} else {
				if len(chunk.Candidates) > 0 && len(accumulatedResponse.Candidates) > 0 {
					candidate := accumulatedResponse.Candidates[0]
					chunkCandidate := chunk.Candidates[0]
					// 追加 chunk 的内容到累积响应
					if chunkCandidate.Content != nil {
						if candidate.Content == nil {
							candidate.Content = &genai.Content{Parts: []*genai.Part{}}
						}
						candidate.Content.Parts = append(candidate.Content.Parts, chunkCandidate.Content.Parts...)
					}
					// 更新结束原因
					if chunkCandidate.FinishReason != "" {
						candidate.FinishReason = chunkCandidate.FinishReason
					}
				}
			}
		}
		// 流式结束，返回最终响应
		if accumulatedResponse != nil {
			finalResponse, err := convertGenAIToBlades(accumulatedResponse, blades.StatusCompleted)
			if err != nil {
				yield(nil, err)
				return
			}
			yield(finalResponse, nil)
		}
	}
}
