package recipe

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// missingKeyPattern 用于匹配 Go template 的缺失键错误信息
// 当模板引用了 map 中不存在的键时，Go 会返回类似 "map has no entry for key "xxx"" 的错误
var missingKeyPattern = regexp.MustCompile(`map has no entry for key "([^"]+)"`)

// renderTemplate 使用给定的数据渲染 Go text/template 模板字符串
// 如果模板字符串为空，返回空字符串
//
// 模板语法参考：https://pkg.go.dev/text/template
// 支持的语法包括：
//   - {{.param}} - 引用参数
//   - {{if .cond}}...{{end}} - 条件判断
//   - {{range .items}}...{{end}} - 循环
//   - {{.field.subfield}} - 嵌套字段访问
func renderTemplate(tmplStr string, data map[string]any) (string, error) {
	return executeTemplate(tmplStr, data, true)
}

// renderTemplatePreservingUnknown 渲染模板并保留未知的 map 键作为原始 {{.key}} 占位符
// 这种模式用于子 Agent 的指令渲染，允许在构建时保留一些运行时才能解析的变量
//
// 工作原理：
// 1. 尝试渲染模板
// 2. 如果遇到缺失键错误，将该键的值设置为原始的 {{.key}} 字符串
// 3. 重新渲染，直到所有已知键都被解析或遇到无法处理的错误
// 4. 最多尝试 64 次，防止无限循环
//
// 使用场景：
// 子 Agent 的指令中可能包含 {{.syntax_report}} 这样的变量，它在构建时未知，
// 但在运行时可以从会话状态中获取。通过保留这些占位符，可以在运行时再解析。
func renderTemplatePreservingUnknown(tmplStr string, data map[string]any) (string, error) {
	if tmplStr == "" {
		return "", nil
	}
	// 复制数据 map，避免修改原始数据
	working := cloneMap(data)
	// 迭代渲染，每次处理一个缺失的键
	for i := 0; i < 64; i++ {
		// 尝试渲染模板
		out, err := executeTemplate(tmplStr, working, true)
		if err == nil {
			// 渲染成功，返回结果
			return out, nil
		}
		// 提取缺失的键名
		key, ok := extractMissingKey(err)
		if !ok {
			// 不是缺失键错误，返回原始错误
			return "", err
		}
		// 如果键已经存在，说明是其他错误，返回原始错误
		if _, exists := working[key]; exists {
			return "", err
		}
		// 将缺失的键设置为原始占位符格式，这样下次渲染时会保留
		working[key] = "{{." + key + "}}"
	}
	// 超过最大迭代次数，返回错误
	return "", fmt.Errorf("recipe: too many unresolved template keys")
}

// executeTemplate 执行模板渲染的内部函数
// strictMissingKey 控制是否对缺失键返回错误（true）还是忽略（false）
func executeTemplate(tmplStr string, data map[string]any, strictMissingKey bool) (string, error) {
	if tmplStr == "" {
		return "", nil
	}
	// 创建新模板
	t := template.New("recipe")
	// 如果启用严格模式，设置 missingkey=error 选项
	if strictMissingKey {
		t = t.Option("missingkey=error")
	}
	// 解析模板字符串
	t, err := t.Parse(tmplStr)
	if err != nil {
		return "", err
	}
	// 执行模板，写入缓冲区
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// resolveParams 合并用户提供的参数和参数默认值
// 返回合并后的 map，准备好用于模板渲染
//
// 合并规则：
// 1. 首先将所有参数的默认值放入合并 map
// 2. 然后用用户提供的值覆盖默认值
// 3. 用户可以只提供部分参数，未提供的参数使用默认值
func resolveParams(params []ParameterSpec, userParams map[string]any) map[string]any {
	// 创建合并后的 map，预分配空间
	merged := make(map[string]any, len(params))
	// 1. 添加默认值
	for _, p := range params {
		if p.Default != nil {
			merged[p.Name] = p.Default
		}
	}
	// 2. 用用户提供的值覆盖
	for k, v := range userParams {
		merged[k] = v
	}
	return merged
}

// hasTemplateActions 检查字符串是否包含 Go template 动作（如 {{.something}}）
// 这用于判断是否需要在构建时预渲染模板，还是推迟到运行时渲染
//
// 返回 true 表示字符串包含模板语法，需要进一步处理
// 返回 false 表示字符串是纯文本，可以直接使用
func hasTemplateActions(s string) bool {
	return strings.Contains(s, "{{")
}

// cloneMap 复制一个 map
// 用于创建数据的独立副本，避免修改原始数据
func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return make(map[string]any)
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// extractMissingKey 从 Go template 错误信息中提取缺失的键名
// 错误信息格式：template: recipe:1:5: executing "recipe" at <.key>: map has no entry for key "key"
func extractMissingKey(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	// 使用正则表达式匹配错误信息
	match := missingKeyPattern.FindStringSubmatch(err.Error())
	if len(match) != 2 {
		return "", false
	}
	return match[1], true
}
