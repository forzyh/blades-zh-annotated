package tools

import (
	"context"

	"github.com/google/jsonschema-go/jsonschema"
)

// ============================================================================
// Option：工具配置选项
// ============================================================================

// Option 定义了 baseTool 的配置选项函数类型。
//
// 【是什么】
// Option 是一个函数类型，它接收一个 *baseTool 指针，对其进行配置。
// 这种模式称为"函数式选项模式"（Functional Options Pattern）。
//
// 【为什么使用这个模式】
// 1. 可选参数：Go 不支持默认参数和可变命名参数，这个模式提供了优雅的替代方案
// 2. 类型安全：每个选项函数有明确的类型，编译期检查
// 3. 易扩展：添加新选项只需增加新函数，不破坏现有代码
// 4. 链式调用：可以传入多个选项，依次应用
//
// 【使用示例】
//
//	tool := NewTool("name", "desc", handler,
//	    WithMiddleware(mw1, mw2),
//	    WithInputSchema(schema),
//	    WithOutputSchema(outSchema))
//
// 【底层原理】
// Option 函数在内部被调用时，会修改 baseTool 的字段：
//
//	func WithMiddleware(mw ...Middleware) Option {
//	    return func(t *baseTool) {
//	        t.middlewares = mw  // 修改字段
//	    }
//	}
type Option func(*baseTool)

// WithMiddleware 设置工具的中间件列表。
//
// 【参数】
//   - mw: 可变数量的 Middleware，按顺序应用
//
// 【使用示例】
//
//	tool := NewTool("mytool", "描述", handler,
//	    WithMiddleware(LoggingMiddleware(), MetricsMiddleware()))
//
// 【注意事项】
//   - 多次调用 WithMiddleware 时，后面的会覆盖前面的
//   - 如果想追加中间件，需要合并列表后一次性传入
func WithMiddleware(mw ...Middleware) Option {
	return func(t *baseTool) {
		t.middlewares = mw
	}
}

// WithInputSchema 设置工具的输入 JSON Schema。
//
// 【为什么需要手动设置】
// 通常 InputSchema 由 NewFunc 根据泛型类型自动生成，但在以下情况需要手动设置：
//   - 使用 NewTool 创建工具时，没有类型信息
//   - 需要覆盖自动生成的 Schema
//   - 需要动态构建 Schema
//
// 【使用示例】
//
//	schema := &jsonschema.Schema{
//	    Type: "object",
//	    Properties: map[string]*jsonschema.Schema{
//	        "query": {Type: "string"},
//	    },
//	}
//	tool := NewTool("search", "搜索", handler,
//	    WithInputSchema(schema))
func WithInputSchema(schema *jsonschema.Schema) Option {
	return func(t *baseTool) {
		t.inputSchema = schema
	}
}

// WithOutputSchema 设置工具的输出 JSON Schema。
//
// 【使用场景】
//   - 描述工具返回值的结构，帮助调用者解析
//   - 用于文档生成和类型检查
//
// 【使用示例】
//
//	outSchema := &jsonschema.Schema{
//	    Type: "object",
//	    Properties: map[string]*jsonschema.Schema{
//	        "result": {Type: "string"},
//	        "count": {Type: "integer"},
//	    },
//	}
//	tool := NewTool("search", "搜索", handler,
//	    WithOutputSchema(outSchema))
func WithOutputSchema(schema *jsonschema.Schema) Option {
	return func(t *baseTool) {
		t.outputSchema = schema
	}
}

// ============================================================================
// baseTool：基础工具实现
// ============================================================================

// baseTool 是 Tool 接口的默认实现。
//
// 【是什么】
// baseTool 是一个结构体，它实现了 Tool 接口的所有方法。
// 使用 NewTool 或 NewFunc 创建的工具实际上都是 *baseTool 类型。
//
// 【字段说明】
//   - name: 工具名称，用于标识和调用
//   - description: 工具描述，帮助 LLM 理解工具用途
//   - inputSchema: 输入参数的 JSON Schema，定义工具需要什么参数
//   - outputSchema: 输出结果的 JSON Schema，描述工具返回什么
//   - handler: 实际执行逻辑，实现 Handler 接口
//   - middlewares: 中间件列表，在 handler 执行前后添加额外逻辑
//
// 【为什么不导出】
// baseTool 是小写开头，表示它是包内私有类型。
// 这样设计的好处：
//   1. 强制使用构造函数：用户必须通过 NewTool/NewFunc 创建
//   2. 隐藏实现细节：未来可以更改内部结构而不影响外部
//   3. 简化 API：用户只关心 Tool 接口，不关心具体实现
//
// 【生命周期】
// 1. 通过 NewTool 或 NewFunc 创建
// 2. 应用 Option 配置（中间件、Schema 等）
// 3. 注册到 Agent 或工具 registry
// 4. 被 LLM 调用时执行 Handle 方法
type baseTool struct {
	name         string             // 工具名称
	description  string             // 工具描述
	inputSchema  *jsonschema.Schema // 输入参数 Schema
	outputSchema *jsonschema.Schema // 输出结果 Schema
	handler      Handler            // 执行逻辑
	middlewares  []Middleware       // 中间件列表
}

// Name 返回工具的名称。
//
// 【用途】
//   - 标识工具：每个工具有唯一的名称
//   - 调用工具：LLM 通过名称指定要调用的工具
//   - 日志记录：记录哪个工具被调用
func (t *baseTool) Name() string {
	return t.name
}

// Description 返回工具的描述。
//
// 【用途】
//   - LLM 理解：帮助 LLM 知道工具的用途
//   - 文档生成：可以自动生成工具文档
//   - 调试：帮助开发者理解工具作用
//
// 【编写建议】
//   - 清晰简洁：一句话说明工具做什么
//   - 包含场景：说明何时使用此工具
//   - 避免歧义：描述应该明确无二义
func (t *baseTool) Description() string {
	return t.description
}

// InputSchema 返回工具的输入 JSON Schema。
//
// 【用途】
//   - LLM 参数生成：LLM 根据 Schema 生成符合格式的参数
//   - 参数验证：可以验证 LLM 生成的参数是否合法
//   - 文档：描述工具需要什么输入
//
// 【返回值】
// 返回 nil 表示工具不需要输入参数，或输入是自由格式的 JSON。
func (t *baseTool) InputSchema() *jsonschema.Schema {
	return t.inputSchema
}

// OutputSchema 返回工具的输出 JSON Schema。
//
// 【用途】
//   - 结果解析：帮助调用者理解如何解析返回值
//   - 文档：描述工具返回什么数据
//
// 【返回值】
// 返回 nil 表示工具不返回结构化数据，或输出是自由格式的 JSON。
func (t *baseTool) OutputSchema() *jsonschema.Schema {
	return t.outputSchema
}

// Handle 执行工具的核心逻辑。
//
// 【是什么】
// Handle 是 Tool 接口要求实现的方法，当 LLM 调用工具时执行。
//
// 【执行流程】
// 1. 检查是否配置了中间件
// 2. 如果有中间件，使用 ChainMiddlewares 包装 handler
// 3. 调用（可能是包装后的）handler 执行实际逻辑
// 4. 返回结果和错误
//
// 【参数】
//   - ctx: 上下文，包含调用信息和可能的 ToolContext
//   - input: JSON 字符串，包含工具调用参数
//
// 【返回值】
//   - string: JSON 字符串，工具执行结果
//   - error: 执行错误
//
// 【中间件处理】
// 如果配置了中间件，handler 会被包装：
//
//	原始：handler.Handle(ctx, input)
//	包装后：mw1(mw2(handler)).Handle(ctx, input)
//
// 这样中间件可以在 handler 执行前后添加逻辑。
//
// 【使用示例】
//
//	// LLM 调用工具时，Agent 运行时执行：
//	result, err := tool.Handle(ctx, `{"query": "hello"}`)
//
// 【注意事项】
//   - 中间件只在 Handle 第一次被调用时包装一次
//   - 包装后的 handler 会缓存，后续调用直接使用
func (t *baseTool) Handle(ctx context.Context, input string) (string, error) {
	handler := t.handler
	// 如果配置了中间件，包装 handler
	if len(t.middlewares) > 0 {
		// ChainMiddlewares 返回一个 Middleware，应用后返回新的 Handler
		// 中间件按顺序包装：middlewares[0] 是最外层
		handler = ChainMiddlewares(t.middlewares...)(t.handler)
	}
	return handler.Handle(ctx, input)
}
