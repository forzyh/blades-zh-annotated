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
	"encoding/json"
	"fmt"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"
	"google.golang.org/genai"
)

// convertMessageToGenAI 将 Blades 框架的消息格式转换为 Gemini API 格式。
//
// # 作用说明
//
// 此函数负责将 blades.ModelRequest 中的消息历史转换为 Gemini SDK 能理解的
// Content 格式。Gemini 的消息格式与 Blades 有所不同，需要进行适配。
//
// # 参数说明
//
//   - req: Blades 模型请求，包含 Instruction 和 Messages
//
// # 返回值
//
//   - *genai.Content: 系统指令内容（如果有）
//   - []*genai.Content: 对话历史内容列表
//   - error: 转换错误
//
// # 角色映射
//
// Blades 角色 -> Gemini 角色:
//   - RoleSystem -> 系统指令（单独返回）
//   - RoleUser -> genai.RoleUser
//   - RoleAssistant -> genai.RoleModel
//   - RoleTool -> genai.RoleUser（工具结果作为用户消息返回）
//
// # 工具结果处理
//
// 工具调用结果会被转换为 FunctionResponse 格式，包含：
//   - 函数名
//   - 函数调用 ID
//   - 响应内容（尝试解析为 JSON，失败则作为普通文本）
func convertMessageToGenAI(req *blades.ModelRequest) (*genai.Content, []*genai.Content, error) {
	var (
		system   *genai.Content
		contents []*genai.Content
	)
	// 处理系统指令
	if req.Instruction != nil {
		system = &genai.Content{Parts: convertMessagePartsToGenAI(req.Instruction.Parts)}
	}
	// 遍历消息历史
	for _, msg := range req.Messages {
		switch msg.Role {
		case blades.RoleSystem:
			// System 角色消息也作为系统指令处理
			system = &genai.Content{Parts: convertMessagePartsToGenAI(msg.Parts)}
		case blades.RoleUser:
			// 用户消息直接转换
			contents = append(contents, &genai.Content{Role: genai.RoleUser, Parts: convertMessagePartsToGenAI(msg.Parts)})
		case blades.RoleAssistant:
			// 助手消息映射为 Model 角色
			contents = append(contents, &genai.Content{Role: genai.RoleModel, Parts: convertMessagePartsToGenAI(msg.Parts)})
		case blades.RoleTool:
			// 工具调用结果需要特殊处理
			var parts []*genai.Part
			for _, part := range msg.Parts {
				switch v := any(part).(type) {
				case blades.ToolPart:
					// 尝试将工具响应解析为 JSON
					response := map[string]any{}
					if err := json.Unmarshal([]byte(v.Response), &response); err != nil {
						// 解析失败则作为普通文本存储
						response["output"] = v.Response
					}
					// 创建函数响应 Part
					parts = append(parts, genai.NewPartFromFunctionResponse(v.Name, response))
				}
			}
			// 工具结果作为用户消息发送回模型
			contents = append(contents, &genai.Content{Role: genai.RoleUser, Parts: parts})
		}
	}
	return system, contents, nil
}

// convertMessagePartsToGenAI 将 Blades 的 Part 列表转换为 Gemini 的 Part 列表。
//
// # 作用说明
//
// 处理消息中的各种内容类型（文本、图片、文件等），转换为 Gemini SDK 的 Part 格式。
//
// # 参数说明
//
//   - parts: Blades 框架的内容片段列表
//
// # 返回值
//
// Gemini SDK 的 Part 列表
//
// # 支持的类型转换
//
//   - TextPart -> genai.Part{Text: ...}
//   - DataPart -> genai.Part{InlineData: ...}（内联二进制数据）
//   - FilePart -> genai.Part{FileData: ...}（文件 URI 引用）
func convertMessagePartsToGenAI(parts []blades.Part) []*genai.Part {
	res := make([]*genai.Part, 0, len(parts))
	for _, part := range parts {
		switch v := part.(type) {
		case blades.TextPart:
			// 文本内容直接复制
			res = append(res, &genai.Part{Text: v.Text})
		case blades.DataPart:
			// 内联数据（如 base64 编码的图片）
			res = append(res, &genai.Part{
				InlineData: &genai.Blob{
					Data:        v.Bytes,
					DisplayName: v.Name,
					MIMEType:    string(v.MIMEType),
				},
			})
		case blades.FilePart:
			// 文件 URI 引用
			res = append(res, &genai.Part{
				FileData: &genai.FileData{
					FileURI:     v.URI,
					DisplayName: v.Name,
					MIMEType:    string(v.MIMEType),
				},
			})
		}
	}
	return res
}

// convertBladesToolsToGenAI 将 Blades 工具列表转换为 Gemini 工具列表。
//
// # 作用说明
//
// 批量转换工具定义，使 Gemini 模型能够调用外部工具。
//
// # 参数说明
//
//   - tools: Blades 框架的工具列表
//
// # 返回值
//
//   - []*genai.Tool: Gemini 工具列表
//   - error: 转换错误
func convertBladesToolsToGenAI(tools []tools.Tool) ([]*genai.Tool, error) {
	genaiTools := make([]*genai.Tool, 0, len(tools))
	for _, tool := range tools {
		genaiTool, err := convertBladesToolToGenAI(tool)
		if err != nil {
			return nil, fmt.Errorf("converting tool %s: %w", tool.Name(), err)
		}
		if genaiTool != nil {
			genaiTools = append(genaiTools, genaiTool)
		}
	}
	return genaiTools, nil
}

// convertBladesToolToGenAI 将单个 Blades 工具转换为 Gemini 工具。
//
// # 作用说明
//
// 将 Blades 工具的定义（名称、描述、输入/输出 Schema）转换为
// Gemini API 的 FunctionDeclaration 格式。
//
// # 参数说明
//
//   - tool: Blades 框架的工具
//
// # 返回值
//
// Gemini 工具指针，包含函数声明
//
// # Gemini 工具格式
//
// Gemini 的工具通过 FunctionDeclarations 定义，包含：
//   - Name: 函数名称
//   - Description: 函数描述
//   - ParametersJsonSchema: 输入参数的 JSON Schema
//   - ResponseJsonSchema: 响应的 JSON Schema（可选）
func convertBladesToolToGenAI(tool tools.Tool) (*genai.Tool, error) {
	return &genai.Tool{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			&genai.FunctionDeclaration{
				Name:                 tool.Name(),
				Description:          tool.Description(),
				ParametersJsonSchema: tool.InputSchema(),
				ResponseJsonSchema:   tool.OutputSchema(),
			},
		},
	}, nil
}

// convertGenAIToBlades 将 Gemini 响应转换为 Blades 框架响应。
//
// # 作用说明
//
// 将 Gemini SDK 的 GenerateContentResponse 转换为 blades.ModelResponse，
// 使上层应用可以统一处理不同 LLM Provider 的响应。
//
// # 参数说明
//
//   - resp: Gemini API 响应
//   - status: 响应状态（Completed 或 Incomplete）
//
// # 返回值
//
//   - *blades.ModelResponse: Blades 框架响应
//   - error: 转换错误
//
// # 处理逻辑
//
// 1. 创建 Assistant 消息
// 2. 遍历所有候选响应（Candidates）
// 3. 对每个候选的内容 Part 进行转换
// 4. 如果包含工具调用，设置消息角色为 RoleTool
func convertGenAIToBlades(resp *genai.GenerateContentResponse, status blades.Status) (*blades.ModelResponse, error) {
	message := blades.NewAssistantMessage(status)
	hasToolCall := false
	for _, candidate := range resp.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			bladesPart, err := convertGenAIPartToBlades(part)
			if err != nil {
				return nil, err
			}
			message.Parts = append(message.Parts, bladesPart)
			// 检查是否是工具调用
			if _, ok := bladesPart.(blades.ToolPart); ok {
				hasToolCall = true
			}
		}
	}
	// 如果有工具调用，设置角色为 RoleTool
	if hasToolCall {
		message.Role = blades.RoleTool
	}
	return &blades.ModelResponse{Message: message}, nil
}

// convertGenAIPartToBlades 将 Gemini Part 转换为 Blades Part。
//
// # 作用说明
//
// 处理 Gemini 响应中的各种内容类型，转换为 Blades 框架的统一格式。
//
// # 参数说明
//
//   - part: Gemini SDK 的 Part 对象
//
// # 返回值
//
// Blades 框架的 Part 接口实现
//
// # 支持的类型转换
//
//   - FunctionCall -> ToolPart（工具调用）
//   - FileData -> FilePart（文件引用）
//   - InlineData -> DataPart（内联数据）
//   - Text -> TextPart（文本）
func convertGenAIPartToBlades(part *genai.Part) (blades.Part, error) {
	// 函数调用（工具调用）
	if part.FunctionCall != nil {
		request := "{}"
		if len(part.FunctionCall.Args) > 0 {
			// 将参数序列化为 JSON
			args, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return nil, fmt.Errorf("marshal function call args: %w", err)
			}
			request = string(args)
		}
		return blades.NewToolPart(part.FunctionCall.ID, part.FunctionCall.Name, request), nil
	}
	// 文件数据
	if part.FileData != nil {
		return blades.FilePart{
			URI:      part.FileData.FileURI,
			Name:     part.FileData.DisplayName,
			MIMEType: blades.MIMEType(part.FileData.MIMEType),
		}, nil
	}
	// 内联数据
	if part.InlineData != nil {
		return blades.DataPart{
			Bytes:    part.InlineData.Data,
			Name:     part.InlineData.DisplayName,
			MIMEType: blades.MIMEType(part.InlineData.MIMEType),
		}, nil
	}
	// 默认返回文本
	return blades.TextPart{Text: part.Text}, nil
}
