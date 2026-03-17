// Package memory 提供了 Agent 记忆（Memory）系统的实现。
// 本文件实现了 InMemoryStore，这是 MemoryStore 接口的内存存储版本。
//
// InMemoryStore 特点：
// - 所有数据存储在内存中，程序重启后数据丢失
// - 适合开发、测试和演示场景
// - 无需外部依赖，开箱即用
// - 使用读写锁（RWMutex）保证并发安全
package memory

import (
	"context"
	"strings"
	"sync"

	"github.com/go-kratos/blades"
)

// InMemoryStore 是 MemoryStore 接口的内存实现。
//
// 数据结构说明：
// - m: 读写锁，保护并发访问
//   - 读操作（SearchMemory）使用读锁，允许多个并发读取
//   - 写操作（AddMemory、SaveSession）使用写锁，互斥访问
// - memories: 记忆切片，按添加顺序存储所有记忆
//
// 并发安全：
// InMemoryStore 使用 sync.RWMutex 实现并发安全：
// - 多个 goroutine 可以同时读取记忆
// - 写入时会独占锁，确保数据一致性
//
// 使用场景：
// - 开发和测试环境
// - 原型演示
// - 不需要持久化的临时会话
// - 作为实现其他存储后端的参考示例
//
// 示例：
//
//	store := memory.NewInMemoryStore()
//	agent := blades.NewAgent("my-agent",
//	    blades.WithTools(memory.NewMemoryTool(store)),
//	)
type InMemoryStore struct {
	m        sync.RWMutex
	memories []*Memory
}

// NewInMemoryStore 创建并返回一个新的内存存储实例。
//
// 返回值：
// - *InMemoryStore: 初始化的内存存储指针
//
// 使用示例：
//
//	store := memory.NewInMemoryStore()
//	// 添加记忆
//	err := store.AddMemory(ctx, &memory.Memory{
//	    Content: blades.UserMessage("用户喜欢咖啡"),
//	})
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{}
}

// AddMemory 添加一条新记忆到内存存储中。
//
// 参数说明：
// - ctx: 上下文（此实现中未使用，但为了符合接口定义而保留）
// - m: 要添加的记忆对象
//
// 处理流程：
// 1. 获取写锁（独占锁）
// 2. 将记忆追加到 memories 切片末尾
// 3. 释放锁
//
// 并发安全：
// 使用写锁确保同一时间只有一个 goroutine 能修改 memories 切片，
// 避免并发追加导致的数据竞争或切片损坏。
//
// 返回值：
// - error: 始终返回 nil（此实现不会失败）
//
// 时间复杂度：O(1)
func (s *InMemoryStore) AddMemory(ctx context.Context, m *Memory) error {
	s.m.Lock()
	defer s.m.Unlock()
	s.memories = append(s.memories, m)
	return nil
}

// SaveSession 将会话的历史记录保存为记忆。
//
// 参数说明：
// - ctx: 上下文（此实现中未使用）
// - session: 会话对象，包含历史对话记录
//
// 处理流程：
// 1. 检查 session 是否为 nil，如果是则直接返回（无操作）
// 2. 获取写锁
// 3. 获取会话的历史记录
// 4. 遍历每条消息，创建记忆并添加到存储
// 5. 释放锁
//
// 使用场景：
// - 会话结束时保存完整对话历史
// - 将重要会话转为可搜索的记忆
// - 实现会话归档功能
//
// 注意：
// 此实现简单地将每条消息转为一条记忆，没有进行过滤或聚合。
// 生产环境可能需要：
// - 只保存重要的消息（如用户确认的事实）
// - 合并相关消息为一条记忆
// - 添加时间戳、类别等元数据
//
// 返回值：
// - error: 始终返回 nil（此实现不会失败）
func (s *InMemoryStore) SaveSession(ctx context.Context, session blades.Session) error {
	if session == nil {
		return nil
	}
	s.m.Lock()
	defer s.m.Unlock()
	// 获取会话的历史对话记录
	history := session.History()
	// 将每条消息转为记忆存储
	for _, msg := range history {
		s.memories = append(s.memories, &Memory{Content: msg})
	}
	return nil
}

// SearchMemory 根据查询字符串搜索相关的记忆。
//
// 参数说明：
// - ctx: 上下文（此实现中未使用）
// - query: 搜索查询字符串
//
// 搜索算法：
// 1. 将查询字符串按空格分割为多个单词
// 2. 转换为小写，实现大小写不敏感搜索
// 3. 遍历所有记忆，检查消息文本是否包含任一单词
// 4. 返回匹配的记忆列表
//
// 特点：
// - 大小写不敏感（case-insensitive）
// - 子字符串匹配（substring match）
// - OR 逻辑：只要包含任一单词即匹配
// - 简单但有效，适合小规模记忆库
//
// 性能考虑：
// - 时间复杂度：O(n*m)，n 为记忆数量，m 为查询单词数
// - 适合记忆数量较少（< 10000）的场景
// - 大规模场景建议使用向量数据库或搜索引擎
//
// 返回值：
// - []*Memory: 匹配的记忆列表
// - error: 始终返回 nil（此实现不会失败）
//
// 使用示例：
//
//	memories, err := store.SearchMemory(ctx, "用户喜欢什么")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, mem := range memories {
//	    fmt.Println(mem.Content.Text())
//	}
func (s *InMemoryStore) SearchMemory(ctx context.Context, query string) ([]*Memory, error) {
	s.m.RLock()
	defer s.m.RUnlock()
	// 简单的大小写不敏感子字符串匹配
	// 将查询分割为单词，增加匹配灵活性
	words := strings.Fields(strings.ToLower(query))
	var result []*Memory
	for _, m := range s.memories {
		// 检查记忆文本是否包含任一查询单词
		for _, word := range words {
			if strings.Contains(strings.ToLower(m.Content.Text()), word) {
				result = append(result, m)
				break // 匹配一个单词即可，避免重复添加
			}
		}
	}
	return result, nil
}
