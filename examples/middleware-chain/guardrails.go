// Blades 示例：中间件链 - 防护栏（guardrails.go）
//
// 本文件演示如何实现一个自定义中间件 Guardrails，用于在请求处理前后添加安全防护。
// 防护栏中间件可以在请求发送给 LLM 之前进行检查，也可以在响应返回给用户之前进行过滤。
//
// 适用场景：
// - 内容安全过滤（屏蔽敏感话题）
// - 输入验证（检查恶意输入）
// - 输出审核（过滤不当内容）
// - 合规性检查（确保符合法规要求）
//
// 核心概念：
// 1. Middleware（中间件）：包装 Handler 的装饰器，可以在调用前后添加逻辑
// 2. Handle 方法：中间件的核心接口，接收 Invocation 并返回 Generator
// 3. stream.Observe：观察流式响应的辅助函数
//
// 注意：这是中间件链示例的一部分，与 logging.go 和 main.go 一起使用
package main

import (
	"context"
	"log"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/stream"
)

// Guardrails 是一个中间件结构体，用于添加安全防护功能
// 它实现了 blades.Handler 接口，可以包装其他 Handler
type Guardrails struct {
	next blades.Handler // 下一个处理器（被包装的 Handler）
}

// NewGuardrails 创建一个新的 Guardrails 中间件
// 这是中间件工厂函数，符合 blades.Middleware 类型签名
// 参数 next 是被包装的下一个 Handler
// 返回值 blades.Handler 是包装后的 Handler
func NewGuardrails(next blades.Handler) blades.Handler {
	return &Guardrails{next}
}

// Handle 是 Guardrails 中间件的核心方法
// 它在请求处理的前后添加防护栏逻辑
// 参数：
//   - ctx: 上下文，包含请求的生命周期信息
//   - invocation: 调用信息，包含用户消息、会话状态等
// 返回值：
//   - blades.Generator[*blades.Message, error]: 流式响应生成器
func (m *Guardrails) Handle(ctx context.Context, invocation *blades.Invocation) blades.Generator[*blades.Message, error] {
	// 预处理：在请求发送给 LLM 之前添加防护栏检查
	// 这里可以添加：
	// - 输入内容检查（如敏感词过滤）
	// - 用户权限验证
	// - 请求频率限制
	log.Println("Applying guardrails to the prompt (streaming)")

	// 使用 stream.Observe 观察下一个 Handler 的流式响应
	// stream.Observe 包装一个 Generator，允许在每次 yield 时执行回调
	return stream.Observe(m.next.Handle(ctx, invocation), func(msg *blades.Message, err error) error {
		// 后处理：在响应流式返回给用户之前进行检查
		// 这里可以添加：
		// - 输出内容审核
		// - 敏感信息过滤
		// - 响应格式校验

		if err != nil {
			// 记录错误日志
			log.Println("Error during streaming:", err)
			return err // 返回错误，中断流式传输
		}

		// 记录响应内容（生产环境中可以用于审核日志）
		log.Println("Streaming with guardrails applied:", msg.Text())

		// 如果返回 nil，表示继续处理
		// 如果返回 error，表示中断处理
		return nil
	})
}

// 扩展提示：
// 1. 实现内容过滤：
//    func (m *Guardrails) filterContent(text string) bool {
//        // 检查敏感词
//        sensitiveWords := []string{"xxx", "yyy"}
//        for _, word := range sensitiveWords {
//            if strings.Contains(text, word) {
//                return false
//            }
//        }
//        return true
//    }
//
// 2. 实现输入验证：
//    func (m *Guardrails) validateInput(invocation *blades.Invocation) error {
//        // 检查输入长度
//        if len(invocation.Message.Text()) > 10000 {
//            return errors.New("input too long")
//        }
//        // 检查恶意模式
//        // ...
//        return nil
//    }
//
// 3. 结合多个防护规则：
//    可以在 Handle 方法中依次调用多个检查函数
//    只有所有检查通过才继续处理
