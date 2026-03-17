package blades

import (
	"context"

	"github.com/go-kratos/kit/container/maps"
	"github.com/go-kratos/kit/container/slices"
	"github.com/google/uuid"
)

// Session 保存流（Flow）的状态和唯一会话 ID。
// Session 是 Blades 框架中用于管理对话历史和状态的核心接口。
// 通过 Session，Agent 可以：
// 1. 存储和检索任意状态数据（State）
// 2. 追踪对话历史（History）
// 3. 追加新消息（Append）
//
// 实现 Session 接口可以自定义存储后端，如 Redis、数据库等。
// 默认提供 sessionInMemory 内存实现。
type Session interface {
	// ID 返回会话的唯一标识符。
	ID() string
	// State 返回当前会话状态（键值对映射）。
	State() State
	// SetState 设置状态中的键值对。
	SetState(key string, value any)
	// History 返回会话历史消息列表。
	History() []*Message
	// Append 追加一条消息到会话历史。
	Append(context.Context, *Message) error
}

// NewSession 创建一个新的 Session 实例。
// 会话 ID 通过 uuid 自动生成，确保全局唯一性。
// 可以通过参数传入初始状态映射。
//
// 参数：
//   - states: 可选的初始状态映射，可以有多个
//
// 返回：
//   - Session: 新的会话实例
//
// 使用示例：
//
//	// 创建空会话
//	session := blades.NewSession()
//
//	// 创建带初始状态的会话
//	session := blades.NewSession(
//	    map[string]any{"username": "Alice", "role": "admin"},
//	)
func NewSession(states ...map[string]any) Session {
	session := &sessionInMemory{id: uuid.NewString()}
	for _, state := range states {
		for k, v := range state {
			session.SetState(k, v)
		}
	}
	return session
}

// ctxSessionKey 是用于在上下文中存储 Session 的私有键类型。
// 使用私有类型防止外部包冲突。
type ctxSessionKey struct{}

// NewSessionContext 返回一个携带 Session 的子上下文。
// 用于将 Session 注入到执行上下文中，供 Agent 和中间件使用。
//
// 参数：
//   - ctx: 父上下文
//   - session: 要注入的 Session
//
// 返回：
//   - context.Context: 携带 Session 的子上下文
func NewSessionContext(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, ctxSessionKey{}, session)
}

// FromSessionContext 从上下文中检索 Session。
// 如果上下文中不存在 Session，返回 (nil, false)。
//
// 返回：
//   - Session: 会话（如果存在）
//   - bool: 是否成功获取
func FromSessionContext(ctx context.Context) (Session, bool) {
	session, ok := ctx.Value(ctxSessionKey{}).(Session)
	return session, ok
}

// sessionInMemory 是 Session 接口的内存实现。
// 使用内存存储状态和历史消息，适用于临时会话或测试场景。
// 生产环境中建议使用持久化实现（如 Redis、数据库）。
//
// 字段说明：
// - id: 会话唯一标识符
// - state: 状态映射，使用线程安全的 maps.Map
// - history: 消息历史，使用线程安全的 slices.Slice
type sessionInMemory struct {
	id      string
	state   maps.Map[string, any]
	history slices.Slice[*Message]
}

// ID 返回会话 ID。
func (s *sessionInMemory) ID() string {
	return s.id
}

// State 返回当前状态映射的拷贝。
func (s *sessionInMemory) State() State {
	return s.state.ToMap()
}

// History 返回历史消息列表的拷贝。
func (s *sessionInMemory) History() []*Message {
	return s.history.ToSlice()
}

// SetState 设置状态键值对。
// 此操作是线程安全的。
func (s *sessionInMemory) SetState(key string, value any) {
	s.state.Store(key, value)
}

// Append 追加消息到历史记录。
// 此操作是线程安全的。
//
// 参数：
//   - ctx: 上下文（未使用，保留用于接口兼容）
//   - message: 要追加的消息
func (s *sessionInMemory) Append(ctx context.Context, message *Message) error {
	s.history.Append(message)
	return nil
}
