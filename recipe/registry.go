package recipe

import (
	"fmt"
	"sync"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/tools"
)

// ModelResolver 模型解析器接口
// 用于将 YAML 配置中的模型名称（如 "gpt-4o"）解析为实际的 blades.ModelProvider 实例
// 这种抽象使得配方系统不依赖于具体的模型实现，支持多种模型提供者
type ModelResolver interface {
	// Resolve 根据名称解析模型提供者
	// 如果名称未注册，返回错误
	Resolve(name string) (blades.ModelProvider, error)
}

// ToolResolver 工具解析器接口
// 用于将 YAML 配置中的工具名称（如 "search"、"file_read"）解析为实际的 tools.Tool 实例
// 通过注册表模式，配方可以引用外部工具而无需知道具体实现
type ToolResolver interface {
	// Resolve 根据名称解析工具
	// 如果名称未注册，返回错误
	Resolve(name string) (tools.Tool, error)
}

// MiddlewareFactory 中间件工厂函数类型
// 用于从 YAML 配置中的选项构建 blades.Middleware 实例
// 工厂函数接收 options 映射，返回配置好的中间件
//
// 示例：注册一个日志中间件工厂
//
//	registry.Register("logging", func(options map[string]any) (blades.Middleware, error) {
//	    level, _ := options["level"].(string)
//	    return logging.NewMiddleware(level), nil
//	})
type MiddlewareFactory func(options map[string]any) (blades.Middleware, error)

// MiddlewareResolver 中间件解析器接口
// 用于将 YAML 配置中的中间件名称和选项解析为实际的 blades.Middleware 实例
// 每个中间件声明可以有自己的选项，这些选项会传递给工厂函数
type MiddlewareResolver interface {
	// Resolve 根据名称和选项解析中间件
	// name 是中间件名称，必须在注册表中存在
	// options 是配置选项，传递给工厂函数
	// 如果名称未注册或工厂返回错误，则返回错误
	Resolve(name string, options map[string]any) (blades.Middleware, error)
}

// MiddlewareRegistry 基于工厂的中间件注册表
// 这是一个简单的内存注册表，使用 map 存储中间件工厂
// 支持并发安全的注册和解析操作
type MiddlewareRegistry struct {
	mu        sync.RWMutex
	factories map[string]MiddlewareFactory
}

// NewMiddlewareRegistry 创建一个新的空中间件注册表
// 在使用前需要先注册中间件工厂
func NewMiddlewareRegistry() *MiddlewareRegistry {
	return &MiddlewareRegistry{
		factories: make(map[string]MiddlewareFactory),
	}
}

// Register 注册一个中间件工厂
// name 是中间件名称，在 YAML 配置中引用此名称
// factory 是工厂函数，用于从选项构建中间件实例
//
// 使用示例:
//
//	registry.Register("tracing", func(options map[string]any) (blades.Middleware, error) {
//	    return tracing.NewMiddleware(), nil
//	})
func (r *MiddlewareRegistry) Register(name string, factory MiddlewareFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Resolve 解析中间件名称为实际的 Middleware 实例
// 此方法查找注册的工厂并调用它，传递 options 参数
// 如果名称未注册或工厂返回错误，则返回错误
func (r *MiddlewareRegistry) Resolve(name string, options map[string]any) (blades.Middleware, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.factories[name]
	if !ok {
		return nil, fmt.Errorf("recipe: middleware %q not found in registry", name)
	}
	return f(options)
}

// ModelRegistry 简单的内存模型注册表
// 实现 ModelResolver 接口，存储模型名称到 ModelProvider 的映射
type ModelRegistry struct {
	mu        sync.RWMutex
	providers map[string]blades.ModelProvider
}

// NewModelRegistry 创建一个新的空模型注册表
// 在使用前需要先注册模型提供者
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		providers: make(map[string]blades.ModelProvider),
	}
}

// Register 注册一个模型提供者
// name 是模型名称，在 YAML 配置中引用此名称
// provider 是实际的 ModelProvider 实例
//
// 使用示例:
//
//	registry.Register("gpt-4o", openai.NewModelProvider())
//	registry.Register("claude-3-5", anthropic.NewModelProvider())
func (r *ModelRegistry) Register(name string, provider blades.ModelProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
}

// Resolve 解析模型名称为实际的 ModelProvider 实例
// 如果名称未注册，返回错误
func (r *ModelRegistry) Resolve(name string) (blades.ModelProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("recipe: model %q not found in registry", name)
	}
	return p, nil
}

// ToolRegistry 简单的内存工具注册表
// 实现 ToolResolver 接口，存储工具名称到 Tool 的映射
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]tools.Tool
}

// NewToolRegistry 创建一个新的空工具注册表
// 在使用前需要先注册工具
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]tools.Tool),
	}
}

// Register 注册一个工具
// name 是工具名称，在 YAML 配置中引用此名称
// tool 是实际的 Tool 实例
//
// 使用示例:
//
//	registry.Register("search", search.NewTool())
//	registry.Register("file_read", filesystem.NewReadTool())
func (r *ToolRegistry) Register(name string, tool tools.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = tool
}

// Resolve 解析工具名称为实际的 Tool 实例
// 如果名称未注册，返回错误
func (r *ToolRegistry) Resolve(name string) (tools.Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("recipe: tool %q not found in registry", name)
	}
	return t, nil
}
