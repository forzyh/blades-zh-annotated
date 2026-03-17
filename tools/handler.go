package tools

import (
	"context"
	"encoding/json"
)

// ============================================================================
// Handler：工具处理器接口
// ============================================================================

// Handler 定义了工具执行逻辑的接口。
//
// 【是什么】
// Handler 是实际执行工具业务逻辑的组件。当 LLM 决定调用某个工具时，
// Agent 运行时会调用该工具的 Handler.Handle 方法来执行具体操作。
//
// 【为什么】
// 1. 职责分离：将工具的定义（Tool 接口）与执行（Handler 接口）分离
// 2. 统一调用：无论工具内部如何实现，都通过统一的 Handle 方法调用
// 3. 灵活实现：可以用函数、结构体、外部服务等多种方式实现 Handler
//
// 【输入输出】
//   - 输入：context.Context（上下文）+ string（JSON 格式的参数）
//   - 输出：string（JSON 格式的结果）+ error（错误信息）
//
// 【为什么用 JSON 字符串】
// 1. 通用性：JSON 可以表示任意复杂的数据结构
// 2. LLM 友好：LLM 擅长生成和解析 JSON 格式
// 3. 解耦：Handler 不需要知道参数如何传递，只需处理 JSON
//
// 【使用示例】
//
//	type MyHandler struct{}
//
//	func (h *MyHandler) Handle(ctx context.Context, input string) (string, error) {
//	    var params struct {
//	        Query string `json:"query"`
//	    }
//	    json.Unmarshal([]byte(input), &params)
//	    result := "处理：" + params.Query
//	    return json.Marshal(map[string]string{"result": result})
//	}
type Handler interface {
	// Handle 执行工具逻辑。
	//
	// 【参数】
	//   - ctx: 上下文，包含调用信息和 ToolContext（如果有）
	//   - input: JSON 字符串，包含工具调用参数
	//
	// 【返回值】
	//   - string: JSON 字符串，包含工具执行结果
	//   - error: 如果执行失败，返回错误
	//
	// 【注意事项】
	//   - input 保证是有效的 JSON 格式（由 Agent 运行时保证）
	//   - 返回值应该是有意义的 JSON，便于调用者解析
	//   - 错误会直接返回给 Agent，可能导致调用失败
	Handle(context.Context, string) (string, error)
}

// HandleFunc 是一个适配器类型，将普通函数转换为 Handler 接口。
//
// 【是什么】
// HandleFunc 是一个函数类型，它本身实现了 Handler 接口。
// 这种设计类似于 net/http 包中的 http.HandlerFunc 模式。
//
// 【为什么】
// 1. 简化开发：对于简单工具，不需要定义结构体，直接写函数即可
// 2. 代码简洁：减少样板代码，逻辑集中在一个函数中
// 3. 灵活转换：可以轻松在函数和接口之间转换
//
// 【使用示例】
//
//	handler := tools.HandleFunc(func(ctx context.Context, input string) (string, error) {
//	    // 直接处理逻辑
//	    return `{"result": "done"}`, nil
//	})
type HandleFunc func(context.Context, string) (string, error)

// Handle 实现了 Handler 接口。
//
// 【实现说明】
// 这个方法只是简单地调用 HandleFunc 本身。
// 这使得 HandleFunc 可以直接作为 Handler 使用。
//
// 【底层原理】
// 这是 Go 中常见的"适配器模式"：
// 1. HandleFunc 是一个函数类型
// 2. 为它定义一个 Handle 方法，使其实现 Handler 接口
// 3. 这样任何符合签名的函数都可以当作 Handler 使用
func (f HandleFunc) Handle(ctx context.Context, input string) (string, error) {
	return f(ctx, input)
}

// ============================================================================
// JSONAdapter：类型安全的适配器
// ============================================================================

// JSONAdapter 将一个处理强类型输入输出的函数适配为处理 JSON 字符串的 HandleFunc。
//
// 【是什么】
// JSONAdapter 是一个泛型函数，它接收一个处理类型 I 和 O 的函数，
// 返回一个处理 JSON 字符串的 HandleFunc。
//
// 【为什么】
// 1. 类型安全：开发者可以用强类型编写逻辑，避免手动处理 JSON 的错误
// 2. 自动转换：适配器自动处理 JSON 的序列化和反序列化
// 3. 关注业务：开发者只需关注业务逻辑，不用关心数据格式转换
//
// 【泛型参数】
//   - I: 输入参数的类型，必须是可反序列化的结构体或基本类型
//   - O: 输出结果的类型，必须是可序列化的结构体或基本类型
//
// 【使用示例】
//
//	// 定义输入输出类型
//	type Input struct {
//	    Name string `json:"name"`
//	}
//	type Output struct {
//	    Greeting string `json:"greeting"`
//	}
//
//	// 编写强类型的处理函数
//	handle := func(ctx context.Context, input Input) (Output, error) {
//	    return Output{Greeting: "Hello, " + input.Name}, nil
//	}
//
//	// 使用 JSONAdapter 适配为 Handler
//	handler := tools.JSONAdapter(handle)
//
// 【底层实现】
// 1. 返回一个 HandleFunc（闭包）
// 2. 闭包内将输入 JSON 字符串反序列化为类型 I
// 3. 调用原始处理函数，得到类型 O 的结果
// 4. 将结果序列化为 JSON 字符串返回
// 5. 任何一步出错都返回 error
//
// 【工作流程图】
//
//	JSON 字符串 -> 反序列化 -> I 类型 -> handle 函数 -> O 类型 -> 序列化 -> JSON 字符串
func JSONAdapter[I, O any](handle func(context.Context, I) (O, error)) HandleFunc {
	return func(ctx context.Context, input string) (string, error) {
		var req I
		// 将 JSON 字符串反序列化为输入类型 I
		// 如果输入格式不匹配类型 I，这里会返回错误
		if err := json.Unmarshal([]byte(input), &req); err != nil {
			return "", err
		}
		// 调用原始处理函数，处理类型 I 的输入
		res, err := handle(ctx, req)
		if err != nil {
			// 处理函数出错，直接返回
			return "", err
		}
		// 将结果序列化为 JSON 字符串
		b, err := json.Marshal(res)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}
