// Package memory 提供了 Agent 记忆（Memory）系统的实现。
// 记忆系统允许 Agent 存储和检索信息，实现跨会话的知识持久化。
//
// 核心概念：
// 1. Memory: 一条记忆，包含内容（消息）和元数据
// 2. MemoryStore: 记忆存储接口，定义了存储和检索记忆的方法
// 3. InMemoryStore: 内存存储实现，用于开发和测试场景
//
// 使用场景：
// - 持久化用户偏好和历史信息
// - 跨会话保存关键事实和决策
// - 实现长期记忆功能，让 Agent"记住"用户
// - 支持语义搜索，快速检索相关记忆
package memory

import (
	"context"

	"github.com/go-kratos/blades"
)

// Memory 表示存储在记忆系统中的一条信息。
//
// 字段说明：
// - Content: 记忆的内容，是一条 blades.Message 消息
//   - 可以是用户的问题、Agent 的回答或工具调用结果
//   - 使用 Message 类型保持了与对话历史的一致性
// - Metadata: 记忆的元数据（可选），用于存储额外信息
//   - 例如：时间戳、来源、重要性评分等
//   - 使用 map[string]any 提供灵活的扩展能力
//
// 使用示例：
//
//	memory := &memory.Memory{
//	    Content: blades.UserMessage("用户喜欢 Python 胜过 Java"),
//	    Metadata: map[string]any{
//	        "timestamp": time.Now(),
//	        "category":  "preference",
//	    },
//	}
type Memory struct {
	Content  *blades.Message `json:"content"`
	Metadata map[string]any  `json:"metadata,omitempty"`
}

// MemoryStore 定义了记忆存储的接口。
// 任何想要作为记忆存储后端的实现都需要实现此接口。
//
// 接口方法说明：
// 1. AddMemory: 添加单条记忆
//    - 用于存储新的记忆条目
//    - 可以是用户信息、事实、偏好等
//
// 2. SaveSession: 保存整个会话历史为记忆
//    - 用于将会话的完整对话历史转为记忆
//    - 适合在会话结束时调用，持久化重要交互
//
// 3. SearchMemory: 根据查询搜索相关记忆
//    - 支持语义搜索或关键字搜索
//    - 返回与查询相关的记忆列表
//
// 实现建议：
// - 生产环境可以实现基于向量数据库的记忆存储（如 Pinecone、Milvus）
// - 测试环境可以使用 InMemoryStore 简单实现
// - 可以考虑添加记忆过期、重要性评分等功能
type MemoryStore interface {
	// AddMemory 添加一条记忆到存储中。
	// 参数：ctx - 上下文；m - 要添加的记忆
	AddMemory(context.Context, *Memory) error

	// SaveSession 保存会话历史为记忆。
	// 参数：ctx - 上下文；session - 要保存的会话
	SaveSession(context.Context, blades.Session) error

	// SearchMemory 搜索与查询相关的记忆。
	// 参数：ctx - 上下文；query - 搜索查询字符串
	// 返回：匹配的记忆列表和错误
	SearchMemory(context.Context, string) ([]*Memory, error)
}
