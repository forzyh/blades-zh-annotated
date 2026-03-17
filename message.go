package blades

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Role 表示消息在对话中的作者类型。
// Role 用于区分消息来源，帮助模型理解上下文结构。
// 例如，模型会根据 RoleUser 和 RoleAssistant 判断对话轮次。
type Role string

const (
	// RoleUser 表示终端用户发送的消息。
	RoleUser Role = "user"
	// RoleSystem 表示系统级指令，通常用于设定模型行为。
	// 系统消息通常放在对话开头，提供全局指导。
	RoleSystem Role = "system"
	// RoleAssistant 表示模型（助手）生成的消息。
	RoleAssistant Role = "assistant"
	// RoleTool 表示工具调用产生的消息。
	// 当模型调用工具后，工具的执行结果以 RoleTool 返回。
	RoleTool Role = "tool"
)

// Status 表示消息的生成状态。
// Status 用于跟踪消息的生命周期，特别是在流式生成过程中。
type Status string

const (
	// StatusInProgress 表示消息正在生成中。
	// 用于流式模式，表示生成尚未完成。
	StatusInProgress Status = "in_progress"
	// StatusIncomplete 表示消息部分生成。
	// 可能由于错误或被中断导致未完成。
	StatusIncomplete Status = "incomplete"
	// StatusCompleted 表示消息已完全生成。
	// 这是正常完成状态，表示消息内容完整可用。
	StatusCompleted Status = "completed"
)

// TextPart 是纯文本内容部分。
// 消息可以由多个 Part 组成，TextPart 用于表示文本内容。
type TextPart struct {
	Text string `json:"text"`
}

// FilePart 是通过 URI 引用的文件。
// 用于表示外部文件引用，模型可以读取文件内容。
// 与 DataPart 不同，FilePart 不直接包含文件字节，而是通过 URI 间接引用。
type FilePart struct {
	Name     string   `json:"name"`     // 文件名
	URI      string   `json:"uri"`      // 文件 URI 地址
	MIMEType MIMEType `json:"mimeType"` // MIME 类型，如 "image/png"
}

// DataPart 是直接包含字节内容的文件。
// 与 FilePart 不同，DataPart 将文件内容直接嵌入消息中。
// 适用于小文件或需要自包含的场景。
type DataPart struct {
	Name     string   `json:"name"`     // 文件名
	Bytes    []byte   `json:"bytes"`    // 文件字节内容
	MIMEType MIMEType `json:"mimeType"` // MIME 类型
}

// ToolPart 是工具调用部分，包含请求、响应和完成状态。
// 当模型决定调用工具时，会生成一个 ToolPart。
// 执行流程：
// 1. 模型生成 ToolPart（仅包含 ID、Name、Request）
// 2. 系统执行工具
// 3. 填充 Response 和 Completed 字段
type ToolPart struct {
	ID        string `json:"id"`        // 工具调用的唯一标识符
	Name      string `json:"name"`      // 工具名称
	Request   string `json:"arguments"` // 工具调用参数（JSON 格式）
	Response  string `json:"result,omitempty"` // 工具执行结果
	Completed bool   `json:"completed,omitempty"` // 是否已完成执行
}

// NewToolPart 创建一个未完成的工具调用部分。
// 用于初始化 ToolPart，通常由模型生成调用时使用。
//
// 参数：
//   - id: 工具调用唯一标识符
//   - name: 工具名称
//   - request: 工具调用参数
func NewToolPart(id, name, request string) ToolPart {
	return ToolPart{
		ID:      id,
		Name:    name,
		Request: request,
	}
}

// Part 是消息的组成部分接口。
// 消息是多模态的，可以包含文本、文件、数据、工具调用等多种部分。
// 使用标记方法 isPart() 实现密封接口，确保只有预定义类型可以实现。
type Part interface {
	isPart()
}

// 实现 Part 接口的标记方法。
// 这些方法没有实际逻辑，仅用于类型识别。
func (TextPart) isPart() {}
func (FilePart) isPart() {}
func (DataPart) isPart() {}
func (ToolPart) isPart() {}

// TokenUsage 跟踪消息的 token 消耗。
// 用于成本估算和上下文窗口管理。
type TokenUsage struct {
	InputTokens  int64 `json:"inputTokens"`  // 输入 token 数
	OutputTokens int64 `json:"outputTokens"` // 输出 token 数
	TotalTokens  int64 `json:"totalTokens"`  // 总 token 数
}

// Message 表示对话中的单条消息。
// 消息是 Blades 框架中的基本通信单元，包含：
// - 元数据（ID、Role、Author、Status 等）
// - 内容（Parts，可以是文本、文件、工具调用等）
// - 使用统计（TokenUsage）
// - 扩展字段（Actions、Metadata）
type Message struct {
	ID           string         `json:"id"`                      // 消息唯一标识符
	Role         Role           `json:"role"`                    // 消息角色（user/system/assistant/tool）
	Parts        []Part         `json:"parts"`                   // 消息内容部分
	Author       string         `json:"author"`                  // 作者名称（如代理名称）
	InvocationID string         `json:"invocationId,omitempty"`  // 所属调用 ID
	Status       Status         `json:"status"`                  // 消息状态
	FinishReason string         `json:"finishReason,omitempty"`  // 结束原因（如 "stop", "length"）
	TokenUsage   TokenUsage     `json:"tokenUsage,omitempty"`    // Token 使用情况
	Actions      map[string]any `json:"actions,omitempty"`       // 动态动作映射
	Metadata     map[string]any `json:"metadata,omitempty"`      // 元数据
}

// Text 返回消息的第一个文本部分内容。
// 如果消息包含多个 TextPart，会连接所有文本部分。
// 如果消息没有文本内容，返回空字符串。
//
// 使用示例：
//
//	msg := UserMessage("你好", "世界")
//	fmt.Println(msg.Text()) // 输出：你好\n世界
func (m *Message) Text() string {
	var buf strings.Builder
	for _, part := range m.Parts {
		switch v := any(part).(type) {
		case TextPart:
			buf.WriteString(v.Text)
			buf.WriteByte('\n')
		}
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// File 返回消息的第一个文件部分。
// 如果消息没有文件内容，返回 nil。
// 用于快速获取消息中 attached 的文件引用。
func (m *Message) File() *FilePart {
	for _, part := range m.Parts {
		if file, ok := part.(FilePart); ok {
			return &file
		}
	}
	return nil
}

// Data 返回消息的第一个数据部分。
// 如果消息没有嵌入的文件字节内容，返回 nil。
// 用于获取直接嵌入消息的文件数据。
func (m *Message) Data() *DataPart {
	for _, part := range m.Parts {
		if data, ok := part.(DataPart); ok {
			return &data
		}
	}
	return nil
}

// Clone 创建消息的浅拷贝。
// 浅拷贝意味着 Parts 切片本身不会被复制，只是引用传递。
// 如果需要深拷贝，需要手动复制 Parts 和其他引用类型字段。
//
// 返回：
//   - *Message: 新的消息实例
//   - 如果原消息为 nil，返回 nil
func (m *Message) Clone() *Message {
	if m == nil {
		return nil
	}
	return &(*m)
}

// String 返回消息的字符串表示，用于调试。
// 格式化为可读的形式，显示每个部分的内容。
// 实现 fmt.Stringer 接口。
func (m *Message) String() string {
	var buf strings.Builder
	for _, part := range m.Parts {
		switch v := part.(type) {
		case TextPart:
			buf.WriteString("[Text: " + v.Text + "]")
		case FilePart:
			buf.WriteString("[File: " + v.Name + " (" + string(v.MIMEType) + ")]")
		case DataPart:
			buf.WriteString("[Data: " + v.Name + " (" + string(v.MIMEType) + "), " + fmt.Sprintf("%d bytes", len(v.Bytes)) + "]")
		case ToolPart:
			buf.WriteString("[Tool: " + v.Name + " (Request: " + v.Request + ", Response: " + v.Response + ")]")
		}
	}
	return buf.String()
}

// UserMessage 创建一个用户消息。
// 这是常用的构造函数，自动设置 Role=RoleUser 和 Author="user"。
//
// 参数：
//   - parts: 可变参数，可以是 string、TextPart、FilePart、DataPart 等
//
// 返回：
//   - *Message: 新的用户消息
//
// 使用示例：
//
//	// 纯文本消息
//	msg := UserMessage("你好，请帮我分析这个文件")
//
//	// 混合消息（文本 + 文件）
//	msg := UserMessage("请分析这个文件", FilePart{URI: "file:///path/to/file.pdf"})
func UserMessage(parts ...any) *Message {
	return &Message{ID: NewMessageID(), Role: RoleUser, Author: "user", Parts: NewMessageParts(parts...)}
}

// SystemMessage 创建一个系统消息。
// 系统消息用于提供系统级指令，通常放在对话开头。
//
// 参数：
//   - parts: 可变参数，内容部分
//
// 返回：
//   - *Message: 新的系统消息
//
// 使用示例：
//
//	msg := SystemMessage("你是一个专业的代码助手，擅长 Go 语言编程。")
func SystemMessage(parts ...any) *Message {
	return &Message{ID: NewMessageID(), Role: RoleSystem, Parts: NewMessageParts(parts...)}
}

// AssistantMessage 创建一个助手消息。
// 助手消息通常由模型生成，表示 AI 的响应。
//
// 参数：
//   - parts: 可变参数，内容部分
//
// 返回：
//   - *Message: 新的助手消息
func AssistantMessage(parts ...any) *Message {
	return &Message{ID: NewMessageID(), Role: RoleAssistant, Parts: NewMessageParts(parts...)}
}

// NewAssistantMessage 创建一个新的助手消息，带有指定的状态。
// 与 AssistantMessage 不同，此函数允许显式设置 Status。
//
// 参数：
//   - status: 消息状态（如 StatusInProgress、StatusCompleted）
//
// 返回：
//   - *Message: 新的助手消息，带有空的 Actions 和 Metadata 映射
func NewAssistantMessage(status Status) *Message {
	return &Message{
		ID:       NewMessageID(),
		Role:     RoleAssistant,
		Status:   status,
		Actions:  make(map[string]any),
		Metadata: make(map[string]any),
	}
}

// NewMessageID 生成一个新的随机消息标识符。
// 使用 UUID v4 确保全局唯一性。
func NewMessageID() string {
	return uuid.NewString()
}

// NewMessageParts 将异构的内容输入转换为模型 Parts 切片。
// 支持的输入类型：string、TextPart、FilePart、DataPart、ToolPart。
// 其他类型会被忽略。
//
// 参数：
//   - inputs: 可变参数，可以是多种类型
//
// 返回：
//   - []Part: Parts 切片
//
// 使用示例：
//
//	parts := NewMessageParts("你好", TextPart{Text: "世界"}, FilePart{URI: "file:///test.png"})
func NewMessageParts(inputs ...any) []Part {
	parts := make([]Part, 0, len(inputs))
	for _, input := range inputs {
		switch v := any(input).(type) {
		case string:
			parts = append(parts, TextPart{v})
		case TextPart:
			parts = append(parts, v)
		case FilePart:
			parts = append(parts, v)
		case DataPart:
			parts = append(parts, v)
		case ToolPart:
			parts = append(parts, v)
		}
	}
	return parts
}

// MergeParts 合并两个消息的 Parts，修改并返回 base。
// 如果 base 为 nil，返回 extra。如果 extra 为 nil，返回 base。
// 这是一个可变操作，会修改 base 消息。
//
// 参数：
//   - base: 基础消息，会被修改
//   - extra: 额外消息，其 Parts 会被追加到 base
//
// 返回：
//   - *Message: 合并后的消息（即 base）
func MergeParts(base, extra *Message) *Message {
	if base == nil {
		return extra
	}
	if extra == nil {
		return base
	}
	base.Parts = append(base.Parts, extra.Parts...)
	return base
}

// MergeActions 合并两个动作映射，extra 的值会覆盖 base 中的同键值。
// 用于合并工具执行过程中的 Actions。
//
// 参数：
//   - base: 基础动作映射
//   - extra: 额外动作映射，优先级更高
//
// 返回：
//   - map[string]any: 合并后的动作映射
func MergeActions(base, extra map[string]any) map[string]any {
	actions := make(map[string]any, len(base)+len(extra))
	for k, v := range base {
		actions[k] = v
	}
	for k, v := range extra {
		actions[k] = v
	}
	return actions
}

// AppendMessages 将 extra 消息追加到 base 消息列表。
// 如果 extra 中的消息 ID 与 base 中的消息 ID 重复，则 base 中的消息会被移除。
// 这确保消息列表中不会有重复 ID 的消息。
//
// 参数：
//   - base: 基础消息列表
//   - extra: 要追加的消息（可变参数）
//
// 返回：
//   - []*Message: 合并后的消息列表
func AppendMessages(base []*Message, extra ...*Message) []*Message {
	var (
		// 记录 extra 中出现的 ID
		sets     = make(map[string]struct{}, len(extra))
		// 过滤后的基础消息
		filtered = make([]*Message, 0, len(base))
	)
	// 收集 extra 中的 ID
	for _, m := range extra {
		if m.ID == "" {
			continue
		}
		sets[m.ID] = struct{}{}
	}
	// 过滤 base，移除 ID 重复的消息
	for _, m := range base {
		if _, exists := sets[m.ID]; !exists {
			filtered = append(filtered, m)
		}
	}
	// 追加 extra 消息
	return append(filtered, extra...)
}
