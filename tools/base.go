package tools

import (
	"context"

	"github.com/google/jsonschema-go/jsonschema"
)

// ============================================================================
// Tool 接口：工具的核心抽象
// ============================================================================

// Tool 定义了系统中所有工具必须实现的接口。
//
// 【是什么】
// Tool 是 blades 框架中"工具"的核心抽象。你可以把工具理解为 Agent 可以调用的"函数"或"能力"。
// 例如：搜索文件、执行命令、调用 API、读写数据库等，都可以封装成 Tool。
//
// 【为什么】
// 定义统一的接口有多个好处：
// 1. 统一调用方式：无论工具内部如何实现，Agent 都用相同的方式调用它们
// 2. 自描述能力：通过 Name、Description、InputSchema，工具可以告诉 Agent "我能做什么"和"需要什么参数"
// 3. 类型安全：InputSchema 和 OutputSchema 确保输入输出符合预期格式
// 4. 可扩展：新的工具只需实现这个接口即可被系统识别和使用
//
// 【怎么用】
// 通常不需要直接实现这个接口，而是使用 NewTool 或 NewFunc 来创建工具。
// 如果需要自定义工具类型（如 ExitTool），可以定义一个结构体并实现以下所有方法。
type Tool interface {
	// Name 返回工具的名称。
	// 名称是工具的唯一标识符，在调用工具时使用。
	// 命名建议：使用小写字母和下划线，简洁明了，如 "search_file", "run_command"。
	Name() string

	// Description 返回工具的描述。
	// 描述会传递给 LLM，帮助它理解工具的用途和何时使用。
	// 描述应该清晰、具体，说明工具的功能和使用场景。
	Description() string

	// InputSchema 返回工具输入参数的 JSON Schema。
	// JSON Schema 是一种描述 JSON 数据格式的标准，LLM 会根据它生成符合格式的参数。
	// 例如：{"type": "object", "properties": {"query": {"type": "string"}}}
	// 表示工具需要一个包含 "query" 字符串字段的对象作为输入。
	InputSchema() *jsonschema.Schema

	// OutputSchema 返回工具输出结果的 JSON Schema。
	// 用于描述工具返回值的格式，帮助调用者理解如何处理结果。
	// 如果工具不返回结构化数据，可以返回 nil。
	OutputSchema() *jsonschema.Schema

	// Handler 是工具的执行逻辑。
	// Handle 方法接收上下文和 JSON 字符串格式的输入，执行工具逻辑后返回 JSON 字符串格式的结果。
	// 这种设计使得工具可以处理任意复杂的输入输出，只需保证是有效的 JSON 即可。
	Handler
}

// ============================================================================
// 工具构造函数
// ============================================================================

// NewTool 创建一个基础的 Tool 实例。
//
// 【参数说明】
//   - name: 工具名称，用于标识和调用
//   - description: 工具描述，帮助 LLM 理解工具用途
//   - handler: 工具的执行逻辑，实现 Handler 接口
//   - opts: 可选配置项，用于设置中间件、输入输出 Schema 等
//
// 【返回值】
// 返回一个 Tool 接口实例，可以直接注册到系统中使用。
//
// 【使用示例】
//
//	tool := tools.NewTool("greet", "向用户打招呼", handler,
//	    tools.WithInputSchema(schema),
//	)
//
// 【底层实现】
// NewTool 创建一个 baseTool 结构体，应用所有可选配置后返回。
// baseTool 是 Tool 接口的默认实现，封装了工具的所有核心属性。
func NewTool(name string, description string, handler Handler, opts ...Option) Tool {
	t := &baseTool{
		name:        name,
		description: description,
		handler:     handler,
	}
	// 应用可选配置：中间件、Schema 等
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// NewFunc 创建一个基于泛型函数的 Tool 实例。
//
// 【是什么】
// NewFunc 是一个更高级的工具创建函数，它利用 Go 的泛型特性，
// 允许你用强类型的函数来定义工具，而不是直接处理 JSON 字符串。
//
// 【为什么】
// 1. 类型安全：通过泛型 I 和 O，编译期就能检查输入输出类型是否正确
// 2. 自动 Schema 生成：自动根据类型 I 和 O 生成 InputSchema 和 OutputSchema
// 3. 简化开发：你只需关注业务逻辑，不用手动处理 JSON 编解码
//
// 【参数说明】
//   - name: 工具名称
//   - description: 工具描述
//   - handler: 处理函数，接收类型 I 的输入，返回类型 O 的输出
//   - opts: 可选配置项
//
// 【泛型参数】
//   - I: 输入参数的类型，必须是可序列化为 JSON 的结构体
//   - O: 输出结果的类型，必须是可从 JSON 反序列化的结构体
//
// 【使用示例】
//
//	type Input struct {
//	    Query string `json:"query"`
//	}
//	type Output struct {
//	    Result string `json:"result"`
//	}
//
//	tool, err := tools.NewFunc[Input, Output]("search", "搜索内容",
//	    func(ctx context.Context, input Input) (Output, error) {
//	        return Output{Result: "found: " + input.Query}, nil
//	    })
//
// 【底层实现】
// 1. 使用 jsonschema.For 根据泛型类型 I 和 O 自动生成 JSON Schema
// 2. 使用 JSONAdapter 将强类型函数包装成 Handler 接口
// 3. 创建 baseTool 并应用配置
func NewFunc[I, O any](name string, description string, handler func(context.Context, I) (O, error), opts ...Option) (Tool, error) {
	// 根据输入类型 I 生成 JSON Schema
	// jsonschema.For 会分析结构体字段和 tag，生成对应的 Schema
	inputSchema, err := jsonschema.For[I](nil)
	if err != nil {
		return nil, err
	}
	// 根据输出类型 O 生成 JSON Schema
	outputSchema, err := jsonschema.For[O](nil)
	if err != nil {
		return nil, err
	}
	t := &baseTool{
		name:         name,
		description:  description,
		inputSchema:  inputSchema,
		outputSchema: outputSchema,
		// JSONAdapter 将强类型函数转换成 Handler 接口
		// 它会自动处理 JSON 的反序列化和序列化
		handler: JSONAdapter(handler),
	}
	// 应用可选配置
	for _, opt := range opts {
		opt(t)
	}
	return t, nil
}
