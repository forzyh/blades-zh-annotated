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
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"
)

// convertPartsToContent 将 Blades 框架的 Parts 转换为 Claude API 的 ContentBlockParamUnion。
//
// # 作用说明
//
// Blades 框架使用统一的 Part 接口表示消息内容（文本、图片、文件等）。
// 此函数负责将框架的 Part 转换为 Claude API 能理解的内容块格式。
//
// # 参数说明
//
//   - parts: Blades 框架的消息内容片段列表，可以包含 TextPart、DataPart、FilePart 等
//
// # 返回值
//
// Claude API 的内容块参数列表，可直接用于构建消息对象。
//
// # 转换规则
//
// 目前仅支持 TextPart 的转换，其他类型（如图片）的转换逻辑在消息构建时处理。
func convertPartsToContent(parts []blades.Part) []anthropic.ContentBlockParamUnion {
	var content []anthropic.ContentBlockParamUnion
	for _, part := range parts {
		switch p := part.(type) {
		case blades.TextPart:
			// 将文本片段转换为 Claude 的文本块
			content = append(content, anthropic.NewTextBlock(p.Text))
		}
	}
	return content
}

// convertBladesToolsToClaude 将 Blades 框架的工具定义转换为 Claude API 的工具参数。
//
// # 作用说明
//
// 使 Claude 模型能够调用外部工具（函数）。Blades 框架的工具定义需要转换为
// Claude API 的 ToolParam 格式，包括工具名称、描述和输入参数 Schema。
//
// # 参数说明
//
//   - tools: Blades 框架的工具列表，每个工具包含 Name、Description、InputSchema 等属性
//
// # 返回值
//
//   - Claude API 的工具参数列表，可用于 MessageNewParams.Tools 字段
//   - 错误信息，如果 Schema 转换失败
//
// # 转换过程
//
// 1. 将工具的 InputSchema（JSON Schema）序列化为 JSON
// 2. 反序列化为 Claude SDK 的 ToolInputSchemaParam 类型
// 3. 构建 ToolParam，包含名称、描述和输入 Schema
// 4. 包装为 ToolUnionParam 返回
func convertBladesToolsToClaude(tools []tools.Tool) ([]anthropic.ToolUnionParam, error) {
	var claudeTools []anthropic.ToolUnionParam
	for _, tool := range tools {
		var inputSchema anthropic.ToolInputSchemaParam
		// 将 JSON Schema 序列化为字节，再反序列化为 Claude 的 Schema 类型
		schemaBytes, err := json.Marshal(tool.InputSchema())
		if err != nil {
			return nil, fmt.Errorf("marshaling tool schema: %w", err)
		}
		if err := json.Unmarshal(schemaBytes, &inputSchema); err != nil {
			return nil, fmt.Errorf("unmarshaling tool schema: %w", err)
		}
		// 构建工具参数
		toolParam := anthropic.ToolParam{
			Name:        tool.Name(),
			InputSchema: inputSchema,
		}
		// 如果工具有描述，添加描述字段
		if tool.Description() != "" {
			toolParam.Description = anthropic.String(tool.Description())
		}
		// 包装为 Union 类型，这是 Claude SDK 的类型设计
		claudeTools = append(claudeTools, anthropic.ToolUnionParam{
			OfTool: &toolParam,
		})
	}
	return claudeTools, nil
}

// convertClaudeToBlades 将 Claude API 的响应消息转换为 Blades 框架的标准响应格式。
//
// # 作用说明
//
// 将 Claude SDK 的 Message 类型转换为 blades.ModelResponse，使上层应用
// 可以统一处理不同 LLM Provider 的响应。
//
// # 参数说明
//
//   - message: Claude API 返回的完整消息对象
//   - status: 响应状态（Completed 表示完成，Incomplete 表示流式传输中）
//
// # 返回值
//
//   - Blades 框架的模型响应，包含消息内容、工具调用等信息
//   - 错误信息，如果转换失败
//
// # 处理逻辑
//
// 1. 创建新的 Assistant 消息
// 2. 遍历 Claude 返回的内容块：
//   - TextBlock: 转换为 TextPart
//   - ToolUseBlock: 转换为 ToolPart，用于工具调用
//
// 3. 如果包含工具调用，将消息角色设置为 RoleTool
func convertClaudeToBlades(message *anthropic.Message, status blades.Status) (*blades.ModelResponse, error) {
	msg := blades.NewAssistantMessage(status)
	hasToolUse := false
	for _, block := range message.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			// 文本内容直接添加
			msg.Parts = append(msg.Parts, blades.TextPart{Text: b.Text})
		case anthropic.ToolUseBlock:
			// 标记有工具调用
			hasToolUse = true
			// 将工具输入参数序列化为 JSON 字符串
			input, err := json.Marshal(b.Input)
			if err != nil {
				return nil, err
			}
			// 创建工具调用片段：ID、函数名、参数 JSON
			msg.Parts = append(msg.Parts, blades.NewToolPart(b.ID, b.Name, string(input)))
		}
	}
	// 如果有工具调用，设置角色为 RoleTool，表示这是一个工具调用消息
	if hasToolUse {
		msg.Role = blades.RoleTool
	}
	return &blades.ModelResponse{
		Message: msg,
	}, nil
}

// convertStreamDeltaToBlades 将 Claude 流式响应的增量事件转换为 Blades 框架响应。
//
// # 作用说明
//
// 在流式传输过程中，Claude API 会发送多个增量事件（ContentBlockDeltaEvent）。
// 此函数将每个增量事件转换为 blades.ModelResponse，供调用方逐步消费。
//
// # 参数说明
//
//   - event: Claude 流式响应的事件，包含增量内容（delta）
//
// # 返回值
//
//   - Blades 框架的模型响应，状态为 Incomplete（表示流式传输中）
//   - 错误信息，如果转换失败
//
// # 支持的增量类型
//
// 目前仅处理 TextDelta（文本增量），其他类型（如工具调用增量）可扩展支持。
func convertStreamDeltaToBlades(event anthropic.ContentBlockDeltaEvent) (*blades.ModelResponse, error) {
	// 创建不完整的消息，表示流式传输尚未结束
	message := blades.NewAssistantMessage(blades.StatusIncomplete)
	switch delta := event.Delta.AsAny().(type) {
	case anthropic.TextDelta:
		// 将文本增量添加为 TextPart
		message.Parts = append(message.Parts, blades.TextPart{Text: delta.Text})
	}
	return &blades.ModelResponse{
		Message: message,
	}, nil
}
