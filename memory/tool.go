// Package memory 提供了 Agent 记忆（Memory）系统的实现。
// 本文件实现了 MemoryTool，这是 Agent 用于搜索记忆的工具。
package memory

import (
	"context"

	"github.com/go-kratos/blades/tools"
)

// Request 是记忆工具的请求结构体。
// 它封装了工具调用时传递的参数。
//
// 字段说明：
// - Query: 搜索查询字符串
//   - Agent 使用自然语言描述想要查找的信息
//   - 例如："用户喜欢什么编程语言"、"上次讨论的项目名称"
//
// 使用示例（Agent 调用）：
//
//	{
//	  "query": "用户的编程偏好"
//	}
type Request struct {
	Query string `json:"query" jsonschema:"The query to search the memory."`
}

// Response 是记忆工具的响应结构体。
// 它封装了工具执行后返回的结果。
//
// 字段说明：
// - Memories: 匹配的记忆列表
//   - 包含所有与查询相关的记忆
//   - 每条记忆包含原始消息内容和元数据
//
// 使用示例（工具返回）：
//
//	{
//	  "memories": [
//	    {
//	      "content": {"role": "user", "text": "我喜欢 Python"},
//	      "metadata": {"timestamp": "2024-01-01T00:00:00Z"}
//	    }
//	  ]
//	}
type Response struct {
	Memories []*Memory `json:"memories" jsonschema:"The memories found for the query."`
}

// NewMemoryTool 创建并返回一个新的记忆工具实例。
//
// 参数说明：
// - store: MemoryStore 实现，用于实际的存储和搜索操作
//   - 可以是 InMemoryStore（内存存储）
//   - 也可以是其他持久化存储实现（如向量数据库）
//
// 工具功能：
// - 允许 Agent 搜索和检索存储的记忆
// - 支持自然语言查询
// - 返回相关的历史信息、事实或用户偏好
//
// 返回值：
// - tools.Tool: 记忆工具实例
// - error: 创建过程中的错误
//
// 使用示例：
//
//	store := memory.NewInMemoryStore()
//	memoryTool, err := memory.NewMemoryTool(store)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	agent := blades.NewAgent("my-agent", blades.WithTools(memoryTool))
func NewMemoryTool(store MemoryStore) (tools.Tool, error) {
	return tools.NewFunc[Request, Response](
		"Memory",
		"You have memory. You can use it to answer questions. You can look up the memory.",
		func(ctx context.Context, req Request) (Response, error) {
			// 调用存储后端的搜索方法
			memories, err := store.SearchMemory(ctx, req.Query)
			if err != nil {
				return Response{}, err
			}
			return Response{Memories: memories}, nil
		},
	)
}
