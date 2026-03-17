// Package counter 提供了用于估算大语言模型 Token 消耗量的计数器实现。
// 在没有特定模型 tokenizer 的情况下，使用基于字符数的启发式算法进行估算。
package counter

import "github.com/go-kratos/blades"

// charBasedCounter 是基于字符数的 Token 估算器。
//
// 什么是 Token 估算？
// 大语言模型（如 GPT、Claude 等）按 Token 数量计费和处理输入。
// 1 个 Token 约等于 4 个英文字符或 1.5 个汉字。
// 精确的 Token 计算需要调用模型的 tokenizer，但这会增加延迟和依赖。
//
// 为什么使用字符估算？
// - 快速：无需加载 tokenizer 或调用外部服务
// - 轻量：不增加额外依赖
// - 足够准确：对于容量规划和上下文管理场景，估算值已足够使用
//
// 工作原理：
// 将文本长度除以 4 来估算 Token 数，这是 OpenAI 模型的常用经验法则。
// 对于其他模型（如 Claude），这个比例可能略有不同，但作为默认估算仍然有效。
type charBasedCounter struct{}

// NewCharBasedCounter 创建并返回一个基于字符数的 Token 估算器。
//
// 返回值说明：
// 返回 blades.TokenCounter 接口，该接口定义了 Count 方法用于计算消息的 Token 数。
//
// 使用场景：
// - 作为默认 Token 计数器，当没有特定模型的 tokenizer 时
// - 快速估算上下文窗口的 Token 使用量
// - 决定是否需要进行上下文压缩或截断
//
// 示例：
//
//	counter := counter.NewCharBasedCounter()
//	tokens := counter.Count(messages...)
//	if tokens > maxTokens {
//	    // 需要压缩上下文
//	}
func NewCharBasedCounter() blades.TokenCounter {
	return &charBasedCounter{}
}

// Count 计算给定消息列表的估算 Token 数量。
//
// 参数说明：
// messages - 可变参数，接受任意数量的 *blades.Message 消息对象
//
// 计算逻辑：
// 1. 遍历每条消息的所有部分（Parts）
// 2. 对于文本部分（TextPart）：计算文本长度的 Token 估算值
// 3. 对于工具调用部分（ToolPart）：计算工具名称、请求和响应的总 Token 数
// 4. 每条消息额外增加 4 个 Token 的开销（用于角色标记、元数据等）
//
// 公式：
// TextPart: (len(text) + 3) / 4  // 加 3 后整除 4，相当于向上取整
// ToolPart: (len(name) + len(request) + len(response) + 3) / 4
// 每条消息：+4 Token  overhead
//
// 返回值：
// 返回估算的总 Token 数（int64 类型）
//
// 使用示例：
//
//	counter := counter.NewCharBasedCounter()
//	tokens := counter.Count(
//	    blades.UserMessage("Hello"),
//	    blades.AssistantMessage("Hi there!"),
//	)
//	fmt.Printf("Total tokens: %d\n", tokens)
func (c *charBasedCounter) Count(messages ...*blades.Message) int64 {
	var total int64
	for _, m := range messages {
		for _, p := range m.Parts {
			switch v := p.(type) {
			case blades.TextPart:
				// 文本部分：每 4 个字符约等于 1 个 Token
				// +3 后整除 4 实现向上取整效果
				total += int64(len(v.Text)+3) / 4
			case blades.ToolPart:
				// 工具调用部分：计算名称、请求和响应的总长度
				total += int64(len(v.Name)+len(v.Request)+len(v.Response)+3) / 4
			}
		}
		// 每条消息的固定开销：角色标记、元数据等约 4 个 Token
		total += 4
	}
	return total
}
