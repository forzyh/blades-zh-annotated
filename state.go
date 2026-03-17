package blades

import (
	"maps"
)

// State 保存任意键值对，表示会话或代理的状态。
// State 是一个类型别名，底层是 map[string]any。
// 它可以存储任何类型的数据，如字符串、数字、结构体等。
//
// 使用场景：
// 1. 在 Agent 之间传递数据（通过 Session.State()）
// 2. 存储用户偏好、配置信息
// 3. 跟踪任务进度和中间结果
//
// 使用示例：
//
//	state := blades.State{
//	    "username": "Alice",
//	    "preferences": map[string]string{"theme": "dark"},
//	    "step": 3,
//	}
type State map[string]any

// Clone 创建 State 的深拷贝。
// 使用 maps.Clone 确保返回独立的副本，修改副本不会影响原 State。
// 如果 State 为 nil，会创建一个空 State。
//
// 返回：
//   - State: 新的 State 实例
func (s State) Clone() State {
	if s == nil {
		s = State{}
	}
	return State(maps.Clone(map[string]any(s)))
}
