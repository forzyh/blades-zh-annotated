// Package anthropic 提供了 Anthropic Claude API 的客户端实现。
// 本包封装了官方的 anthropic-sdk-go，为 blades 框架提供统一的模型接口。
//
// # 核心功能
//
//   - 支持文本生成（Generate）和流式响应（NewStreaming）
//   - 支持工具调用（Function Calling）
//   - 支持 Prompt 缓存（CacheControl）
//   - 支持思维链（ThinkingConfig）
//
// # 使用示例
//
//	// 创建 Claude 模型实例
//	provider := anthropic.NewModel("claude-sonnet-4-20250514", anthropic.Config{
//	    APIKey: "your-api-key",
//	    Temperature: 0.7,
//	    MaxOutputTokens: 1024,
//	})
//
//	// 调用模型生成响应
//	resp, err := provider.Generate(ctx, &blades.ModelRequest{
//	    Messages: []*blades.Message{
//	        {Role: blades.RoleUser, Parts: []blades.Part{blades.TextPart{Text: "你好"}}},
//	    },
//	})
package anthropic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/go-kratos/blades"
)

// Config 持有 Claude 客户端的配置选项。
//
// # 配置字段说明
//
//   - BaseURL: API 基础 URL，可用于私有部署或代理
//   - APIKey: Anthropic API 密钥，也可通过环境变量设置
//   - MaxOutputTokens: 最大输出 token 数，控制生成长度
//   - Seed: 随机种子，用于复现结果
//   - TopK: 采样时考虑的 top-k token 数
//   - TopP: 核采样（nucleus sampling）的累积概率阈值
//   - Temperature: 温度参数，控制随机性（值越高越随机）
//   - StopSequences: 停止序列列表，遇到这些序列时停止生成
//   - RequestOptions: 额外的请求选项，如重试策略等
//   - Thinking: 思维链配置，启用模型的"思考"能力
//   - CacheControl: 是否启用 Prompt 缓存，可降低成本和延迟
//
// # 使用示例
//
//	config := anthropic.Config{
//	    APIKey:          "sk-ant-...",
//	    Temperature:     0.7,
//	    MaxOutputTokens: 2048,
//	    CacheControl:    true, // 启用缓存
//	}
type Config struct {
	BaseURL         string
	APIKey          string
	MaxOutputTokens int64
	Seed            int64
	TopK            int64
	TopP            float64
	Temperature     float64
	StopSequences   []string
	RequestOptions  []option.RequestOption
	Thinking        *anthropic.ThinkingConfigParamUnion
	// CacheControl 启用 Prompt 缓存。当设置为 true 时，会在每个请求的
	// 最后一个内容块、最后的 system 块和最后的工具上添加临时的 cache_control 断点。
	// 默认禁用。
	CacheControl bool
}

// Claude 提供了统一的 Claude API 访问接口。
//
// # 结构说明
//
//   - model: 模型名称，如 "claude-sonnet-4-20250514"
//   - config: 客户端配置
//   - client: Claude SDK 客户端实例
//
// # 实现接口
//
// Claude 实现了 blades.ModelProvider 接口，提供以下方法：
//   - Name(): 返回模型名称
//   - Generate(): 非流式生成
//   - NewStreaming(): 流式生成
type Claude struct {
	model  string
	config Config
	client anthropic.Client
}

// NewModel 创建一个新的 Claude 模型提供者。
//
// # 参数说明
//
//   - model: 模型名称，如 "claude-sonnet-4-20250514" 或 "claude-opus-4-20250514"
//   - config: 配置选项，包括 API 密钥、温度等参数
//
// # 返回值
//
// blades.ModelProvider 接口实例，可用于调用模型。
//
// # 初始化逻辑
//
// 1. 如果配置了 BaseURL，添加到请求选项
// 2. 如果配置了 APIKey，添加到请求选项
// 3. 使用选项创建 anthropic.Client
// 4. 返回 Claude 实例
//
// # 使用示例
//
//	provider := anthropic.NewModel("claude-sonnet-4-20250514", anthropic.Config{
//	    APIKey:      "sk-ant-...",
//	    Temperature: 0.7,
//	})
//	resp, err := provider.Generate(ctx, request)
func NewModel(model string, config Config) blades.ModelProvider {
	// 应用 BaseURL 和 APIKey 配置
	opts := config.RequestOptions
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}
	if config.APIKey != "" {
		opts = append(opts, option.WithAPIKey(config.APIKey))
	}
	return &Claude{
		model:  model,
		config: config,
		client: anthropic.NewClient(opts...),
	}
}

// Name 返回 Claude 模型的名称。
//
// 这是 blades.ModelProvider 接口的必需方法。
func (m *Claude) Name() string {
	return m.model
}

// Generate 使用 Claude API 生成内容。
//
// # 参数说明
//
//   - ctx: 上下文，用于控制超时和取消
//   - req: 模型请求，包含消息历史、工具定义、系统指令等
//
// # 返回值
//
//   - blades.ModelResponse: 模型响应，包含生成的消息内容
//   - error: 错误信息，如果请求失败
//
// # 处理流程
//
// 1. 将 Blades 请求转换为 Claude API 参数（toClaudeParams）
// 2. 调用 Claude API 发送消息请求
// 3. 将 Claude 响应转换为 Blades 格式（convertClaudeToBlades）
//
// # 错误处理
//
// - 请求转换错误：参数构建失败时返回
// - API 调用错误：网络问题、认证失败等
func (m *Claude) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	// 转换请求参数
	params, err := m.toClaudeParams(req)
	if err != nil {
		return nil, fmt.Errorf("converting request: %w", err)
	}
	// 调用 Claude API
	message, err := m.client.Messages.New(ctx, *params)
	if err != nil {
		return nil, fmt.Errorf("generating content: %w", err)
	}
	// 转换为 Blades 格式
	return convertClaudeToBlades(message, blades.StatusCompleted)
}

// NewStreaming 执行流式请求并返回助手响应的流。
//
// # 参数说明
//
//   - ctx: 上下文，用于控制超时和取消
//   - req: 模型请求
//
// # 返回值
//
// blades.Generator，这是一个生成器函数，通过 yield 回调返回流式响应。
//
// # 流式处理流程
//
// 1. 构建请求参数
// 2. 创建流式请求（NewStreaming）
// 3. 遍历流式事件：
//   - 累积事件到 message 对象
//   - 提取增量内容（ContentBlockDeltaEvent）并 yield
//
// 4. 流式结束后，返回完整的最终响应
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
func (m *Claude) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	return func(yield func(*blades.ModelResponse, error) bool) {
		// 构建请求参数
		params, err := m.toClaudeParams(req)
		if err != nil {
			yield(nil, err)
			return
		}
		// 创建流式请求
		streaming := m.client.Messages.NewStreaming(ctx, *params)
		defer streaming.Close()
		// 累积完整消息
		message := &anthropic.Message{}
		// 遍历流式事件
		for streaming.Next() {
			event := streaming.Current()
			// 累积事件到消息对象
			if err := message.Accumulate(event); err != nil {
				yield(nil, err)
				return
			}
			// 如果是增量事件，yield 给用户
			if ev, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
				response, err := convertStreamDeltaToBlades(ev)
				if err != nil {
					yield(nil, err)
					return
				}
				// 如果用户取消（yield 返回 false），提前退出
				if !yield(response, nil) {
					return
				}
			}
		}
		// 检查流式错误
		if err := streaming.Err(); err != nil {
			yield(nil, err)
			return
		}
		// 返回最终完整响应
		finalResponse, err := convertClaudeToBlades(message, blades.StatusCompleted)
		if err != nil {
			yield(nil, err)
			return
		}
		yield(finalResponse, nil)
	}
}

// toClaudeParams 将 Blades ModelRequest 转换为 Claude MessageNewParams。
//
// # 作用说明
//
// 这是请求转换的核心函数，负责将框架的通用请求格式转换为
// Claude API 特定的参数格式。
//
// # 参数说明
//
//   - req: Blades 模型请求
//
// # 返回值
//
//   - *anthropic.MessageNewParams: Claude API 请求参数
//   - error: 转换错误
//
// # 转换内容
//
// 1. 基础参数：MaxTokens, Temperature, TopK, TopP, StopSequences
// 2. 系统指令：从 Instruction 或 System Message 提取
// 3. 消息历史：遍历 Messages，按角色转换
// 4. 工具定义：转换 Tools 为 Claude 格式
// 5. 缓存控制：如果启用，应用临时缓存标记
func (m *Claude) toClaudeParams(req *blades.ModelRequest) (*anthropic.MessageNewParams, error) {
	params := &anthropic.MessageNewParams{
		Model: anthropic.Model(m.model),
	}
	// 设置生成参数
	if m.config.MaxOutputTokens > 0 {
		params.MaxTokens = m.config.MaxOutputTokens
	}
	if m.config.Temperature > 0 {
		params.Temperature = anthropic.Float(m.config.Temperature)
	}
	if m.config.TopK > 0 {
		params.TopK = anthropic.Int(m.config.TopK)
	}
	if m.config.TopP > 0 {
		params.TopP = anthropic.Float(m.config.TopP)
	}
	if len(m.config.StopSequences) > 0 {
		params.StopSequences = m.config.StopSequences
	}
	if m.config.Thinking != nil {
		params.Thinking = *m.config.Thinking
	}
	// 设置系统指令
	if req.Instruction != nil {
		params.System = []anthropic.TextBlockParam{{Text: req.Instruction.Text()}}
	}
	// 转换消息历史
	for _, msg := range req.Messages {
		switch msg.Role {
		case blades.RoleSystem:
			// System 角色消息也设置为系统指令
			params.System = []anthropic.TextBlockParam{{Text: msg.Text()}}
		case blades.RoleUser:
			// 用户消息
			params.Messages = append(params.Messages, anthropic.NewUserMessage(convertPartsToContent(msg.Parts)...))
		case blades.RoleAssistant:
			// 助手消息
			params.Messages = append(params.Messages, anthropic.NewAssistantMessage(convertPartsToContent(msg.Parts)...))
		case blades.RoleTool:
			// 工具调用消息，需要分离工具结果和助手内容
			var (
				toolResults      []anthropic.ContentBlockParamUnion
				assistantContent []anthropic.ContentBlockParamUnion
			)
			for _, part := range msg.Parts {
				switch v := any(part).(type) {
				case blades.TextPart:
					assistantContent = append(assistantContent, anthropic.NewTextBlock(v.Text))
				case blades.ToolPart:
					// 工具结果作为用户消息发送回模型
					toolResults = append(toolResults, anthropic.NewToolResultBlock(v.ID, v.Response, false))
					// 工具调用本身作为助手消息
					assistantContent = append(assistantContent, anthropic.NewToolUseBlock(v.ID, decodeToolRequest(v.Request), v.Name))
				}
			}
			if len(assistantContent) > 0 {
				params.Messages = append(params.Messages, anthropic.NewAssistantMessage(assistantContent...))
			}
			if len(toolResults) > 0 {
				params.Messages = append(params.Messages, anthropic.NewUserMessage(toolResults...))
			}
		}
	}
	// 转换工具定义
	if len(req.Tools) > 0 {
		tools, err := convertBladesToolsToClaude(req.Tools)
		if err != nil {
			return params, fmt.Errorf("converting tools: %w", err)
		}
		params.Tools = tools
	}
	// 应用缓存控制
	if m.config.CacheControl {
		applyEphemeralCache(params)
	}
	return params, nil
}

// applyEphemeralCache 为请求添加临时缓存控制标记。
//
// # 作用说明
//
// Claude 支持 Prompt 缓存功能，可以在连续请求中缓存系统指令、工具定义和消息历史。
// 此函数在 cacheable 内容的最后一个块上添加 ephemeral cache_control 标记。
//
// # 缓存位置
//
// 1. System 指令的最后一个块
// 2. Tools 列表的最后一个工具
// 3. Messages 列表的最后一个消息的最后一个内容块
//
// # 工作原理
//
// 通过设置 CacheControl 字段为 NewCacheControlEphemeralParam()，
// 告诉 Claude API 这些内容应该被缓存，但可以随时失效。
func applyEphemeralCache(params *anthropic.MessageNewParams) {
	// 在系统指令的最后一个块添加缓存标记
	if len(params.System) > 0 {
		params.System[len(params.System)-1].CacheControl = anthropic.NewCacheControlEphemeralParam()
	}
	// 在最后一个工具添加缓存标记
	if len(params.Tools) > 0 {
		if cc := params.Tools[len(params.Tools)-1].GetCacheControl(); cc != nil {
			*cc = anthropic.NewCacheControlEphemeralParam()
		}
	}
	// 在最后一个消息的最后一个内容块添加缓存标记
	if len(params.Messages) > 0 {
		last := &params.Messages[len(params.Messages)-1]
		if len(last.Content) > 0 {
			if cc := last.Content[len(last.Content)-1].GetCacheControl(); cc != nil {
				*cc = anthropic.NewCacheControlEphemeralParam()
			}
		}
	}
}

// decodeToolRequest 解码工具请求 JSON 字符串。
//
// # 作用说明
//
// 尝试将工具请求字符串解析为 JSON 对象。如果解析失败，返回原始字符串。
// 这用于在工具调用消息中正确格式化请求参数。
func decodeToolRequest(request string) any {
	var decoded any
	if err := json.Unmarshal([]byte(request), &decoded); err == nil {
		return decoded
	}
	return request
}
