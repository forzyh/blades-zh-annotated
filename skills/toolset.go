// Package skills 提供技能（Skill）系统的核心功能。
//
// 技能是什么：
// 技能是 blades 项目中用于扩展 Agent 能力的模块化单元。每个技能包含：
// - 元数据（名称、描述、许可证等）
// - 指令（SKILL.md 中的详细说明）
// - 资源文件（references/、assets/、scripts/）
//
// 技能如何工作：
// 1. 从目录或 embed.FS 加载技能
// 2. 解析 SKILL.md 文件获取元数据和指令
// 3. 将技能转换为工具（Tool）供 Agent 使用
// 4. 通过工具集（Toolset）统一管理所有技能
package skills

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/go-kratos/blades/tools"
	"github.com/google/jsonschema-go/jsonschema"
)

const (
	// ToolListSkillsName 列出所有可用技能的工具名称
	ToolListSkillsName = "list_skills"
	// ToolLoadSkillName 加载指定技能指令的工具名称
	ToolLoadSkillName = "load_skill"
	// ToolLoadSkillResourceName 加载技能资源文件的工具名称
	ToolLoadSkillResourceName = "load_skill_resource"
	// ToolRunSkillScriptName 执行技能脚本的工具名称
	ToolRunSkillScriptName = "run_skill_script"

	// defaultScriptTimeoutSeconds 脚本执行的默认超时时间（秒）
	defaultScriptTimeoutSeconds = 300
	// maxScriptTimeoutSeconds 脚本执行的最大超时时间（秒）
	maxScriptTimeoutSeconds     = 1800
)

// coreSkillToolNames 核心技能工具的名称集合。
//
// 为什么需要：
// 核心工具是技能系统的基础工具，不受 allowed-tools 限制，
// 始终可用以保证技能系统的基本功能。
var coreSkillToolNames = map[string]struct{}{
	ToolListSkillsName:        {},
	ToolLoadSkillName:         {},
	ToolLoadSkillResourceName: {},
	ToolRunSkillScriptName:    {},
}

// isCoreToolName 检查工具名称是否为核心工具。
// 核心工具始终可用，不受 allowed-tools 过滤影响。
func isCoreToolName(name string) bool {
	_, ok := coreSkillToolNames[name]
	return ok
}

// skillEntry 是技能的内部表示，包含技能实例及其元数据和资源。
//
// 为什么需要：
// - 缓存已解析的元数据和资源，避免重复解析
// - 统一管理技能的三个组成部分（Skill、Frontmatter、Resources）
type skillEntry struct {
	skill       Skill       // 技能实例
	frontmatter Frontmatter // 技能元数据
	resources   Resources   // 技能资源
}

// Toolset 提供已加载技能的工具和指令。
//
// 是什么：
// Toolset 是技能系统的核心组件，它：
// 1. 管理所有已加载的技能
// 2. 提供 4 个核心工具（list_skills、load_skill、load_skill_resource、run_skill_script）
// 3. 生成技能系统指令供 Agent 使用
// 4. 支持 allowed-tools 过滤机制
//
// 为什么需要：
// - 将技能转换为 Agent 可调用的工具
// - 统一管理技能的访问权限
// - 提供一致的技能使用接口
//
// 怎么用：
// 1. 使用 NewToolset 创建工具集
// 2. 调用 ComposeTools 将技能工具与基础工具合并
// 3. 调用 Instruction 获取技能系统指令
type Toolset struct {
	// skills 所有已加载的技能列表
	skills []Skill
	// skillByName 按名称索引的技能，用于快速查找
	skillByName map[string]skillEntry
	// tools 4 个核心技能工具
	tools []tools.Tool
	// allowedToolPatterns 所有技能允许的工具模式集合
	allowedToolPatterns []string
	// instruction 技能系统的完整指令（包含 DefaultSystemInstruction 和技能列表）
	instruction string
}

// NewToolset 创建一个新的技能工具集。
//
// 参数说明：
// - skills: Skill 接口切片，包含所有要加载的技能
//
// 处理流程：
// 1. 遍历每个技能，解析并验证元数据
// 2. 检查技能名称唯一性
// 3. 收集所有 allowed-tools 模式
// 4. 生成技能系统指令
// 5. 创建 4 个核心工具
//
// 返回值：
// - 成功：返回 Toolset 实例和 nil 错误
// - 失败：返回 nil 和具体错误（元数据验证失败、名称重复等）
//
// 怎么用：
// skills, _ := NewFromDir("./skills")
// toolset, err := NewToolset(skills)
func NewToolset(skills []Skill) (*Toolset, error) {
	ts := &Toolset{
		skills:      make([]Skill, 0, len(skills)),
		skillByName: make(map[string]skillEntry, len(skills)),
	}
	for _, skill := range skills {
		if skill == nil {
			// 跳过 nil 技能
			continue
		}
		// 解析技能元数据
		frontmatter, err := resolveFrontmatter(skill)
		if err != nil {
			return nil, err
		}
		// 验证元数据合法性
		if err := frontmatter.Validate(); err != nil {
			return nil, err
		}
		// 检查名称唯一性
		if _, exists := ts.skillByName[skill.Name()]; exists {
			return nil, fmt.Errorf("skills: duplicate skill name %q", skill.Name())
		}
		// 存储技能条目
		ts.skillByName[skill.Name()] = skillEntry{
			skill:       skill,
			frontmatter: frontmatter,
			resources:   resolveResources(skill),
		}
		ts.skills = append(ts.skills, skill)
	}
	// 收集所有 allowed-tools 模式
	allowedToolPatternSet := make(map[string]struct{})
	for _, entry := range ts.skillByName {
		for _, pattern := range splitAllowedToolPatterns(entry.frontmatter.AllowedTools) {
			// 验证模式语法
			if _, err := path.Match(pattern, "tool-name"); err != nil {
				return nil, fmt.Errorf("skills: invalid allowed-tools pattern %q in skill %q: %w", pattern, entry.skill.Name(), err)
			}
			// 去重
			if _, exists := allowedToolPatternSet[pattern]; exists {
				continue
			}
			allowedToolPatternSet[pattern] = struct{}{}
			ts.allowedToolPatterns = append(ts.allowedToolPatterns, pattern)
		}
	}
	sort.Strings(ts.allowedToolPatterns)
	// 生成技能系统指令
	ts.instruction = strings.Join([]string{
		DefaultSystemInstruction,
		FormatSkillsAsXML(ts.skills),
	}, "\n\n")
	// 创建 4 个核心工具
	ts.tools = []tools.Tool{
		&listSkillsTool{toolset: ts},
		&loadSkillTool{toolset: ts},
		&loadSkillResourceTool{toolset: ts},
		&runSkillScriptTool{toolset: ts},
	}
	return ts, nil
}

// resolveFrontmatter 解析技能的完整元数据。
//
// 处理逻辑：
// 1. 从 Skill 接口获取基本信息（Name、Description）
// 2. 如果技能实现了 FrontmatterProvider 接口，获取扩展元数据
// 3. 标准化 Metadata 字段
func resolveFrontmatter(skill Skill) (Frontmatter, error) {
	f := Frontmatter{
		Name:        skill.Name(),
		Description: skill.Description(),
	}
	// 检查是否实现了 FrontmatterProvider 接口
	provider, ok := skill.(FrontmatterProvider)
	if !ok {
		return f, nil
	}
	frontmatter := provider.Frontmatter()
	f.License = frontmatter.License
	f.Compatibility = frontmatter.Compatibility
	f.AllowedTools = frontmatter.AllowedTools
	// 标准化 Metadata
	if len(frontmatter.Metadata) > 0 {
		metadata, err := normalizeMetadataMap(frontmatter.Metadata)
		if err != nil {
			return Frontmatter{}, err
		}
		f.Metadata = metadata
	}
	return f, nil
}

// resolveResources 解析技能的资源文件。
//
// 处理逻辑：
// 1. 检查技能是否实现了 ResourcesProvider 接口
// 2. 如果实现，返回资源集合
// 3. 否则返回空的 Resources
func resolveResources(skill Skill) Resources {
	provider, ok := skill.(ResourcesProvider)
	if !ok {
		return Resources{}
	}
	return provider.Resources()
}

// Tools 返回技能工具集的 4 个核心工具。
//
// 返回值：
// 工具列表包含：
// 1. list_skills - 列出所有可用技能
// 2. load_skill - 加载指定技能的指令
// 3. load_skill_resource - 加载技能的资源文件
// 4. run_skill_script - 执行技能的脚本
//
// 注意：
// 返回的是拷贝，修改不会影响内部状态。
func (t *Toolset) Tools() []tools.Tool {
	out := make([]tools.Tool, 0, len(t.tools))
	out = append(out, t.tools...)
	return out
}

// ComposeTools 合并基础工具与技能工具，并应用 allowed-tools 过滤。
//
// 参数说明：
// - base: 基础工具列表（如文件读写、网络请求等）
//
// 处理逻辑：
// 1. 合并基础工具和技能工具
// 2. 如果设置了 allowed-tools，过滤不在允许列表中的工具
// 3. 核心工具始终保留
//
// 过滤规则：
// - 核心工具（list_skills 等）始终保留
// - 工具名称匹配 allowed-tools 模式的保留
// - allowed-tools 为空时不过滤
//
// 怎么用：
// tools := toolset.ComposeTools(baseTools)
// agent := NewAgent(tools)
func (t *Toolset) ComposeTools(base []tools.Tool) []tools.Tool {
	out := make([]tools.Tool, 0, len(base)+len(t.tools))
	out = append(out, base...)
	out = append(out, t.tools...)
	if len(t.allowedToolPatterns) == 0 {
		return out
	}
	filtered := make([]tools.Tool, 0, len(out))
	for _, tool := range out {
		name := tool.Name()
		// 核心工具或匹配模式的工具保留
		if isCoreToolName(name) || matchesAllowedPattern(name, t.allowedToolPatterns) {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// matchesAllowedPattern 检查工具名称是否匹配任何允许的模式。
// 支持 glob 模式匹配（如 "read_*"、"*-file" 等）。
func matchesAllowedPattern(toolName string, patterns []string) bool {
	for _, pattern := range patterns {
		match, err := path.Match(pattern, toolName)
		if err != nil {
			continue
		}
		if match {
			return true
		}
	}
	return false
}

// Instruction 返回技能系统的完整指令。
//
// 返回值：
// 包含两部分的组合指令：
// 1. DefaultSystemInstruction - 技能系统使用说明
// 2. FormatSkillsAsXML - 可用技能列表
//
// 怎么用：
// 在 Agent 初始化时将此指令添加到系统提示中。
func (t *Toolset) Instruction() string {
	return t.instruction
}

// skillNotFound 生成技能未找到的错误响应。
func skillNotFound(name string) string {
	return mustJSON(map[string]any{
		"error":      fmt.Sprintf("Skill %q not found.", name),
		"error_code": "SKILL_NOT_FOUND",
	})
}

// invalidArgs 生成无效参数的错误响应。
func invalidArgs(msg string) string {
	return mustJSON(map[string]any{
		"error":      msg,
		"error_code": "INVALID_ARGUMENTS",
	})
}

// mustJSON 将值序列化为 JSON 字符串。
// 失败时返回通用错误响应，避免 panic。
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"error":"failed to marshal response","error_code":"INTERNAL_ERROR"}`
	}
	return string(b)
}

// toFrontmatterMap 将 Frontmatter 转换为 map 格式。
// 用于 JSON 响应，只包含非空字段。
func toFrontmatterMap(f Frontmatter) map[string]any {
	out := map[string]any{
		"name":        f.Name,
		"description": f.Description,
	}
	if f.License != "" {
		out["license"] = f.License
	}
	if f.Compatibility != "" {
		out["compatibility"] = f.Compatibility
	}
	if f.AllowedTools != "" {
		out["allowed-tools"] = f.AllowedTools
	}
	if len(f.Metadata) > 0 {
		out["metadata"] = f.Metadata
	}
	return out
}

// listSkillsTool 列出所有可用技能的工具实现。
type listSkillsTool struct {
	toolset *Toolset
}

func (t *listSkillsTool) Name() string { return ToolListSkillsName }

func (t *listSkillsTool) Description() string {
	return "Lists all available skills with their names and descriptions."
}

func (t *listSkillsTool) InputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:       "object",
		Properties: map[string]*jsonschema.Schema{},
	}
}

func (t *listSkillsTool) OutputSchema() *jsonschema.Schema { return nil }

// Handle 处理列出技能的请求。
// 返回 XML 格式的技能列表。
func (t *listSkillsTool) Handle(ctx context.Context, input string) (string, error) {
	return FormatSkillsAsXML(t.toolset.skills), nil
}

// loadSkillTool 加载指定技能指令的工具实现。
type loadSkillTool struct {
	toolset *Toolset
}

func (t *loadSkillTool) Name() string { return ToolLoadSkillName }

func (t *loadSkillTool) Description() string {
	return "Loads the SKILL.md instructions for a given skill."
}

func (t *loadSkillTool) InputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:     "object",
		Required: []string{"name"},
		Properties: map[string]*jsonschema.Schema{
			"name": {
				Type:        "string",
				Description: "The name of the skill to load.",
			},
		},
	}
}

func (t *loadSkillTool) OutputSchema() *jsonschema.Schema { return nil }

// Handle 处理加载技能的请求。
// 返回技能的指令、元数据等信息。
func (t *loadSkillTool) Handle(ctx context.Context, input string) (string, error) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return invalidArgs(fmt.Sprintf("Invalid tool arguments: %v", err)), nil
	}
	if req.Name == "" {
		return mustJSON(map[string]any{
			"error":      "Skill name is required.",
			"error_code": "MISSING_SKILL_NAME",
		}), nil
	}
	skill, ok := t.toolset.skillByName[req.Name]
	if !ok {
		return skillNotFound(req.Name), nil
	}
	return mustJSON(map[string]any{
		"skill_name":   skill.skill.Name(),
		"instructions": skill.skill.Instruction(),
		"frontmatter":  toFrontmatterMap(skill.frontmatter),
	}), nil
}

// loadSkillResourceTool 加载技能资源文件的工具实现。
type loadSkillResourceTool struct {
	toolset *Toolset
}

func (t *loadSkillResourceTool) Name() string { return ToolLoadSkillResourceName }

func (t *loadSkillResourceTool) Description() string {
	return "Loads a resource file from references/, assets/, or scripts/ in a skill."
}

func (t *loadSkillResourceTool) InputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:     "object",
		Required: []string{"skill_name", "path"},
		Properties: map[string]*jsonschema.Schema{
			"skill_name": {
				Type:        "string",
				Description: "The name of the skill.",
			},
			"path": {
				Type:        "string",
				Description: "Resource path under references/, assets/, or scripts/.",
			},
		},
	}
}

func (t *loadSkillResourceTool) OutputSchema() *jsonschema.Schema { return nil }

// Handle 处理加载技能资源的请求。
// 资源内容以 base64 编码返回。
func (t *loadSkillResourceTool) Handle(ctx context.Context, input string) (string, error) {
	var req struct {
		SkillName string `json:"skill_name"`
		Path      string `json:"path"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return invalidArgs(fmt.Sprintf("Invalid tool arguments: %v", err)), nil
	}
	if req.SkillName == "" {
		return mustJSON(map[string]any{
			"error":      "Skill name is required.",
			"error_code": "MISSING_SKILL_NAME",
		}), nil
	}
	if req.Path == "" {
		return mustJSON(map[string]any{
			"error":      "Resource path is required.",
			"error_code": "MISSING_RESOURCE_PATH",
		}), nil
	}
	skill, ok := t.toolset.skillByName[req.SkillName]
	if !ok {
		return skillNotFound(req.SkillName), nil
	}
	// 解析资源路径，确定资源类型
	resourceType, resourceName, err := normalizeResourcePath(req.Path)
	if err != nil {
		return mustJSON(map[string]any{
			"error":      "Path must start with 'references/', 'assets/', or 'scripts/' and remain within that directory.",
			"error_code": "INVALID_RESOURCE_PATH",
		}), nil
	}
	var (
		raw   []byte
		found bool
	)
	resources := skill.resources
	switch resourceType {
	case "references":
		content, ok := resources.GetReference(resourceName)
		found = ok
		if ok {
			raw = []byte(content)
		}
	case "assets":
		raw, found = resources.GetAsset(resourceName)
	case "scripts":
		content, ok := resources.GetScript(resourceName)
		found = ok
		if ok {
			raw = []byte(content)
		}
	default:
		return invalidArgs("Invalid resource type"), nil
	}
	if !found {
		return mustJSON(map[string]any{
			"error":      fmt.Sprintf("Resource %q not found in skill %q.", req.Path, req.SkillName),
			"error_code": "RESOURCE_NOT_FOUND",
		}), nil
	}
	return mustJSON(map[string]any{
		"skill_name":     req.SkillName,
		"path":           req.Path,
		"encoding":       "base64",
		"content_base64": base64.StdEncoding.EncodeToString(raw),
	}), nil
}

// runSkillScriptTool 执行技能脚本的工具实现。
type runSkillScriptTool struct {
	toolset *Toolset
}

func (t *runSkillScriptTool) Name() string { return ToolRunSkillScriptName }

func (t *runSkillScriptTool) Description() string {
	return "Executes a script from scripts/ in a skill."
}

func (t *runSkillScriptTool) InputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:     "object",
		Required: []string{"skill_name", "script_path"},
		Properties: map[string]*jsonschema.Schema{
			"skill_name": {
				Type:        "string",
				Description: "The name of the skill.",
			},
			"script_path": {
				Type:        "string",
				Description: "Script path under scripts/.",
			},
			"args": {
				Type:        "array",
				Description: "Optional script args.",
				Items:       &jsonschema.Schema{Type: "string"},
			},
			"env": {
				Type:        "object",
				Description: "Optional environment variables.",
			},
			"timeout_seconds": {
				Type:        "integer",
				Description: fmt.Sprintf("Optional timeout in seconds. Default: %d, max: %d.", defaultScriptTimeoutSeconds, maxScriptTimeoutSeconds),
			},
		},
	}
}

func (t *runSkillScriptTool) OutputSchema() *jsonschema.Schema { return nil }

// Handle 处理执行脚本的请求。
//
// 处理流程：
// 1. 解析并验证请求参数
// 2. 查找技能和脚本
// 3. 创建临时工作目录
// 4. 将技能资源写入工作目录
// 5. 执行脚本并捕获输出
// 6. 清理临时目录
//
// 安全措施：
// - 脚本路径必须相对，防止目录遍历攻击
// - 环境变量名称验证，防止注入
// - 超时控制，防止无限执行
// - 临时目录隔离，防止污染主机
func (t *runSkillScriptTool) Handle(ctx context.Context, input string) (string, error) {
	var req struct {
		SkillName      string            `json:"skill_name"`
		ScriptPath     string            `json:"script_path"`
		Args           []string          `json:"args"`
		Env            map[string]string `json:"env"`
		TimeoutSeconds int               `json:"timeout_seconds"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return invalidArgs(fmt.Sprintf("Invalid tool arguments: %v", err)), nil
	}
	if req.SkillName == "" {
		return mustJSON(map[string]any{
			"error":      "Skill name is required.",
			"error_code": "MISSING_SKILL_NAME",
		}), nil
	}
	if req.ScriptPath == "" {
		return mustJSON(map[string]any{
			"error":      "Script path is required.",
			"error_code": "MISSING_SCRIPT_PATH",
		}), nil
	}
	skill, ok := t.toolset.skillByName[req.SkillName]
	if !ok {
		return skillNotFound(req.SkillName), nil
	}
	// 解析并验证脚本路径
	scriptName, fullScriptPath, err := normalizeScriptPath(req.ScriptPath)
	if err != nil {
		return mustJSON(map[string]any{
			"error":      err.Error(),
			"error_code": "INVALID_SCRIPT_PATH",
		}), nil
	}
	resources := skill.resources
	if _, found := resources.GetScript(scriptName); !found {
		return mustJSON(map[string]any{
			"error":      fmt.Sprintf("Script %q not found in skill %q.", fullScriptPath, req.SkillName),
			"error_code": "SCRIPT_NOT_FOUND",
		}), nil
	}

	// 处理超时参数
	timeoutSeconds := req.TimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = defaultScriptTimeoutSeconds
	}
	if timeoutSeconds < 0 || timeoutSeconds > maxScriptTimeoutSeconds {
		return mustJSON(map[string]any{
			"error":      fmt.Sprintf("timeout_seconds must be between 1 and %d.", maxScriptTimeoutSeconds),
			"error_code": "INVALID_TIMEOUT",
		}), nil
	}
	// 验证环境变量
	for key, value := range req.Env {
		if key == "" ||
			strings.Contains(key, "=") ||
			strings.ContainsRune(key, 0) ||
			strings.ContainsRune(value, 0) {
			return mustJSON(map[string]any{
				"error":      "Environment variable names must be non-empty, must not contain '=', and keys/values must not contain NUL.",
				"error_code": "INVALID_ENV",
			}), nil
		}
	}

	// 创建临时工作目录
	tmpRoot, err := os.MkdirTemp("", "blades-skill-*")
	if err != nil {
		return mustJSON(map[string]any{
			"error":      fmt.Sprintf("Failed to prepare skill workspace: %v", err),
			"error_code": "WORKSPACE_ERROR",
		}), nil
	}
	defer os.RemoveAll(tmpRoot)

	// 将技能资源写入工作目录
	if err := materializeSkillWorkspace(tmpRoot, resources); err != nil {
		return mustJSON(map[string]any{
			"error":      fmt.Sprintf("Failed to materialize skill workspace: %v", err),
			"error_code": "WORKSPACE_ERROR",
		}), nil
	}

	return executeSkillScript(ctx, tmpRoot, req.SkillName, fullScriptPath, req.Args, req.Env, timeoutSeconds), nil
}

// normalizeResourcePath 解析资源路径，返回资源类型和相对路径。
//
// 支持的路径前缀：
// - references/ - 参考文档
// - assets/ - 资源文件
// - scripts/ - 脚本文件
//
// 安全措施：
// - 路径必须是相对的
// - 防止目录遍历（..）
// - 支持 Windows 路径分隔符转换
func normalizeResourcePath(resourcePath string) (resourceType string, resourceName string, err error) {
	resourcePath = strings.TrimSpace(resourcePath)
	resourcePath = strings.ReplaceAll(resourcePath, "\\", "/")
	switch {
	case strings.HasPrefix(resourcePath, "references/"):
		resourceType = "references"
		resourcePath = strings.TrimPrefix(resourcePath, "references/")
	case strings.HasPrefix(resourcePath, "assets/"):
		resourceType = "assets"
		resourcePath = strings.TrimPrefix(resourcePath, "assets/")
	case strings.HasPrefix(resourcePath, "scripts/"):
		resourceType = "scripts"
		resourcePath = strings.TrimPrefix(resourcePath, "scripts/")
	default:
		return "", "", fmt.Errorf("resource path must start with references/, assets/, or scripts/")
	}
	resourceName, err = normalizeSkillRelativePath(resourcePath)
	if err != nil {
		return "", "", fmt.Errorf("resource path must be a relative path within %s/", resourceType)
	}
	return resourceType, resourceName, nil
}

// splitAllowedToolPatterns 解析 allowed-tools 字符串为模式列表。
// 支持逗号或空格分隔的模式。
func splitAllowedToolPatterns(raw string) []string {
	items := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

// normalizeScriptPath 解析脚本路径，返回脚本名称和完整路径。
// 自动去除 scripts/ 前缀并进行路径规范化。
func normalizeScriptPath(scriptPath string) (scriptName string, fullScriptPath string, err error) {
	scriptPath = strings.TrimSpace(scriptPath)
	scriptPath = strings.ReplaceAll(scriptPath, "\\", "/")
	scriptPath = strings.TrimPrefix(scriptPath, "scripts/")
	clean, err := normalizeSkillRelativePath(scriptPath)
	if err != nil {
		return "", "", fmt.Errorf("script path must be a relative path under scripts/")
	}
	return clean, path.Join("scripts", clean), nil
}

// materializeSkillWorkspace 将技能资源写入临时工作目录。
//
// 处理流程：
// 1. 遍历 references/、assets/、scripts/ 的所有文件
// 2. 创建对应的子目录
// 3. 写入文件内容
// 4. 脚本文件设置可执行权限（0755）
//
// 为什么需要：
// - 为脚本执行提供隔离的工作空间
// - 确保脚本可以访问同技能的资源文件
// - 执行完成后自动清理，不留痕迹
func materializeSkillWorkspace(root string, resources Resources) error {
	for rel, content := range resources.References {
		if err := writeWorkspaceFile(root, "references", rel, []byte(content), 0o644); err != nil {
			return err
		}
	}
	for rel, content := range resources.Assets {
		if err := writeWorkspaceFile(root, "assets", rel, content, 0o644); err != nil {
			return err
		}
	}
	for rel, content := range resources.Scripts {
		if err := writeWorkspaceFile(root, "scripts", rel, []byte(content), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// writeWorkspaceFile 向工作目录写入文件。
//
// 安全措施：
// - 验证路径的相对性
// - 防止目录遍历攻击
// - 自动创建必要的父目录
func writeWorkspaceFile(root string, dir string, rel string, content []byte, mode fs.FileMode) error {
	clean, err := normalizeSkillRelativePath(rel)
	if err != nil {
		return fmt.Errorf("invalid file path %q", rel)
	}

	baseDir := filepath.Join(root, filepath.FromSlash(dir))
	targetPath := filepath.Join(baseDir, filepath.FromSlash(clean))
	relToBase, err := filepath.Rel(baseDir, targetPath)
	if err != nil {
		return err
	}
	// 检查是否尝试跳出目录
	if relToBase == ".." || strings.HasPrefix(relToBase, ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid file path %q", rel)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(targetPath, content, mode)
}

// normalizeSkillRelativePath 规范化相对路径。
// 返回清理后的路径或错误。
func normalizeSkillRelativePath(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	rel = strings.ReplaceAll(rel, "\\", "/")
	clean := path.Clean(rel)
	if isInvalidSkillRelativePath(clean) {
		return "", fmt.Errorf("invalid relative path")
	}
	return clean, nil
}

// isInvalidSkillRelativePath 检查路径是否为无效的相对路径。
// 无效情况：
// - 空路径
// - 当前目录（.）或父目录（..）
// - 以 ../ 开头
// - 绝对路径
// - Windows 卷标前缀（如 C:）
func isInvalidSkillRelativePath(clean string) bool {
	return clean == "" ||
		clean == "." ||
		clean == ".." ||
		strings.HasPrefix(clean, "../") ||
		path.IsAbs(clean) ||
		hasWindowsVolumePrefix(clean)
}

// hasWindowsVolumePrefix 检查路径是否有 Windows 卷标前缀（如 C:）。
func hasWindowsVolumePrefix(p string) bool {
	if len(p) < 2 {
		return false
	}
	return ((p[0] >= 'a' && p[0] <= 'z') || (p[0] >= 'A' && p[0] <= 'Z')) && p[1] == ':'
}

// mergeCommandEnv 合并基础环境变量和覆盖环境变量。
//
// 处理逻辑：
// 1. 复制基础环境变量
// 2. 移除被覆盖的变量
// 3. 添加覆盖的变量（按字母排序）
func mergeCommandEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		out := make([]string, len(base))
		copy(out, base)
		return out
	}
	out := make([]string, 0, len(base)+len(overrides))
	for _, item := range base {
		key := item
		if i := strings.IndexByte(item, '='); i >= 0 {
			key = item[:i]
		}
		if _, overridden := overrides[key]; overridden {
			continue
		}
		out = append(out, item)
	}
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out = append(out, key+"="+overrides[key])
	}
	return out
}

// executeSkillScript 执行技能脚本。
//
// 参数说明：
// - ctx: 上下文，用于取消和超时控制
// - tmpRoot: 临时工作目录
// - skillName: 技能名称
// - scriptPath: 脚本相对路径
// - args: 脚本参数
// - env: 环境变量
// - timeoutSeconds: 超时时间（秒）
//
// 执行流程：
// 1. 创建带超时的上下文
// 2. 根据脚本扩展名确定解释器（.py -> python3, .sh/.bash -> bash）
// 3. 执行命令并捕获输出
// 4. 处理超时和错误
// 5. 返回执行结果（stdout、stderr、exit_code、status）
func executeSkillScript(
	ctx context.Context,
	tmpRoot string,
	skillName string,
	scriptPath string,
	args []string,
	env map[string]string,
	timeoutSeconds int,
) string {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	commandName := scriptPath
	commandArgs := append([]string{}, args...)
	switch strings.ToLower(path.Ext(scriptPath)) {
	case ".py":
		commandName = "python3"
		commandArgs = append([]string{scriptPath}, commandArgs...)
	case ".sh", ".bash":
		commandName = "bash"
		commandArgs = append([]string{scriptPath}, commandArgs...)
	}

	cmd := exec.CommandContext(timeoutCtx, commandName, commandArgs...)
	cmd.Dir = tmpRoot
	cmd.Env = mergeCommandEnv(os.Environ(), env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	status := "success"
	if err != nil {
		switch {
		case errors.Is(timeoutCtx.Err(), context.DeadlineExceeded):
			// 超时错误
			exitCode = -1
			status = "timeout"
		default:
			status = "error"
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				return mustJSON(map[string]any{
					"error":      fmt.Sprintf("Failed to execute script %q: %v", scriptPath, err),
					"error_code": "EXECUTION_ERROR",
				})
			}
		}
	}

	return mustJSON(map[string]any{
		"skill_name":  skillName,
		"script_path": scriptPath,
		"args":        args,
		"stdout":      stdout.String(),
		"stderr":      stderr.String(),
		"exit_code":   exitCode,
		"status":      status,
	})
}
