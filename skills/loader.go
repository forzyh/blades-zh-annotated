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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// skillNamePattern 定义技能名称的合法格式。
// 技能名称必须是小写的 kebab-case 格式，例如：
// - 合法："my-skill"、"skill123"、"a"
// - 非法："MySkill"、"my_skill"、"my skill"
var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Frontmatter 描述 SKILL.md 文件头部的 YAML 元数据。
//
// 是什么：
// Frontmatter 是技能配置文件 SKILL.md 开头的 YAML 部分，包含技能的基本信息。
//
// 为什么需要：
// - 让 Agent 快速了解技能的用途和能力
// - 提供技能的元数据用于过滤和管理
// - 定义技能允许使用的工具模式
//
// 怎么用：
// 在 SKILL.md 文件开头使用 YAML 格式定义：
// ---
// name: my-skill
// description: 这是一个示例技能
// license: MIT
// allowed-tools: read_file, write_file
// metadata:
//   version: "1.0"
//   author: "developer"
// ---
type Frontmatter struct {
	// Name 技能的唯一标识符，必须符合 kebab-case 格式
	Name string
	// Description 技能的简短描述，用于向 Agent 展示技能用途
	Description string
	// License 技能的许可证信息（可选）
	License string
	// Compatibility 技能的兼容性说明，如适用的 Agent 版本（可选）
	Compatibility string
	// AllowedTools 技能允许使用的工具模式，支持通配符（可选）
	// 例如："read_*"、"file-*"等
	AllowedTools string
	// Metadata 技能的自定义元数据，支持嵌套的 map 和数组
	Metadata map[string]any
}

// Validate 验证技能元数据是否合法。
//
// 验证规则：
// 1. 名称长度不超过 64 字符，且必须符合 kebab-case 格式
// 2. 描述不能为空，且长度不超过 1024 字符
// 3. 兼容性说明长度不超过 500 字符
//
// 为什么验证：
// - 确保技能名称的唯一性和可识别性
// - 防止过长的描述影响 Agent 的理解效率
// - 提前发现配置错误，避免运行时问题
func (f Frontmatter) Validate() error {
	if len(f.Name) > 64 {
		return fmt.Errorf("skills: name must be at most 64 characters")
	}
	if !skillNamePattern.MatchString(f.Name) {
		return fmt.Errorf("skills: name must be lowercase kebab-case")
	}
	if f.Description == "" {
		return fmt.Errorf("skills: description must not be empty")
	}
	if len(f.Description) > 1024 {
		return fmt.Errorf("skills: description must be at most 1024 characters")
	}
	if len(f.Compatibility) > 500 {
		return fmt.Errorf("skills: compatibility must be at most 500 characters")
	}
	return nil
}

// Resources 保存技能的所有资源文件。
//
// 是什么：
// Resources 是技能目录下三个子目录中所有文件的集合：
// - references/: 参考文档和示例
// - assets/: 模板、文档等二进制或文本资源
// - scripts/: 可执行的脚本文件
//
// 为什么这样设计：
// - 分类管理不同类型的资源
// - 支持 Agent 按需加载资源，减少内存占用
// - 通过 map 结构实现 O(1) 时间复杂度的快速查找
type Resources struct {
	// References 存储 references/ 目录下的文本文件内容
	References map[string]string
	// Assets 存储 assets/ 目录下的二进制文件内容（如图片、PDF 等）
	Assets map[string][]byte
	// Scripts 存储 scripts/ 目录下的脚本文件内容
	Scripts map[string]string
}

// GetReference 获取 references/ 目录下的参考文件内容。
// 返回文件内容和是否存在标志。
func (r Resources) GetReference(path string) (string, bool) {
	v, ok := r.References[path]
	return v, ok
}

// GetAsset 获取 assets/ 目录下的资源文件内容。
// 返回二进制数据和是否存在标志。
func (r Resources) GetAsset(path string) ([]byte, bool) {
	v, ok := r.Assets[path]
	return v, ok
}

// GetScript 获取 scripts/ 目录下的脚本文件内容。
// 返回脚本内容和是否存在标志。
func (r Resources) GetScript(path string) (string, bool) {
	v, ok := r.Scripts[path]
	return v, ok
}

// ListReferences 列出所有参考文件的路径（按字母顺序排序）。
func (r Resources) ListReferences() []string {
	return listKeys(r.References)
}

// ListAssets 列出所有资源文件的路径（按字母顺序排序）。
func (r Resources) ListAssets() []string {
	return listKeys(r.Assets)
}

// ListScripts 列出所有脚本文件的路径（按字母顺序排序）。
func (r Resources) ListScripts() []string {
	return listKeys(r.Scripts)
}

// listKeys 提取 map 的所有键并排序。
//
// 泛型函数说明：
// - 支持任何类型的 map 值（string、[]byte 等）
// - 返回排序后的键列表，确保输出的一致性
// - 空 map 返回 nil 而非空切片，便于 JSON 序列化时省略空字段
func listKeys[T any](m map[string]T) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Skill 是技能的最小接口定义。
//
// 是什么：
// Skill 接口定义了技能必须具备的三个基本能力：
// 1. Name() - 获取技能名称
// 2. Description() - 获取技能描述
// 3. Instruction() - 获取技能的详细指令
//
// 为什么用接口：
// - 解耦技能的加载方式和实现细节
// - 支持多种技能来源（文件系统、embed.FS、内存等）
// - 便于测试和扩展
//
// 怎么用：
// 实现此接口即可让技能被 Toolset 识别和使用。
// 通常使用 staticSkill 结构体作为默认实现。
type Skill interface {
	// Name 返回技能的唯一名称
	Name() string
	// Description 返回技能的简短描述
	Description() string
	// Instruction 返回技能的详细指令（SKILL.md 的正文部分）
	Instruction() string
}

// FrontmatterProvider 是提供技能完整元数据的接口。
//
// 为什么需要：
// Skill 接口只包含基本信息，FrontmatterProvider 扩展接口提供：
// - License（许可证）
// - Compatibility（兼容性）
// - AllowedTools（允许的工具）
// - Metadata（自定义元数据）
//
// 怎么用：
// 当需要访问完整元数据时，使用类型断言检查技能是否实现此接口。
type FrontmatterProvider interface {
	Frontmatter() Frontmatter
}

// ResourcesProvider 是提供技能资源数据的接口。
//
// 为什么需要：
// 将资源访问抽象为接口，支持：
// - 延迟加载资源（按需读取文件）
// - 模拟资源（测试时使用内存数据）
// - 远程资源（从网络或数据库加载）
//
// 怎么用：
// 通过类型断言检查技能是否实现此接口，然后调用 Resources() 获取资源。
type ResourcesProvider interface {
	Resources() Resources
}

// staticSkill 是默认的内存技能实现。
//
// 是什么：
// staticSkill 是 Skill 接口的标准实现，将所有数据存储在内存中。
//
// 为什么需要：
// - 作为 loadFS 等加载函数的返回类型
// - 提供简单、高效的技能存储方式
// - 所有方法都有 nil 检查，避免空指针异常
type staticSkill struct {
	// frontmatter 技能的元数据
	frontmatter Frontmatter
	// instruction 技能的详细指令（SKILL.md 正文）
	instruction string
	// resources 技能的资源文件集合
	resources   Resources
}

// Name 返回技能名称。
// nil 检查确保调用安全。
func (s *staticSkill) Name() string {
	if s == nil {
		return ""
	}
	return s.frontmatter.Name
}

// Description 返回技能描述。
// nil 检查确保调用安全。
func (s *staticSkill) Description() string {
	if s == nil {
		return ""
	}
	return s.frontmatter.Description
}

// Instruction 返回技能指令。
// nil 检查确保调用安全。
func (s *staticSkill) Instruction() string {
	if s == nil {
		return ""
	}
	return s.instruction
}

// Frontmatter 返回技能完整元数据。
// 实现 FrontmatterProvider 接口。
// nil 检查确保调用安全。
func (s *staticSkill) Frontmatter() Frontmatter {
	if s == nil {
		return Frontmatter{}
	}
	return s.frontmatter
}

// Resources 返回技能资源集合。
// 实现 ResourcesProvider 接口。
// nil 检查确保调用安全。
func (s *staticSkill) Resources() Resources {
	if s == nil {
		return Resources{}
	}
	return s.resources
}

// normalizeMetadataMap 将任意类型的值标准化为 map[string]any。
//
// 为什么需要标准化：
// YAML 解析后的值可能包含各种类型（包括自定义类型），
// 此函数确保所有值都能被 JSON 序列化，便于网络传输和存储。
func normalizeMetadataMap(value any) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}
	normalized, err := normalizeMetadataValue(value)
	if err != nil {
		return nil, err
	}
	items, ok := normalized.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("skills: metadata must be a map")
	}
	return items, nil
}

// normalizeMetadataValue 递归标准化任意值为 JSON 兼容类型。
//
// 支持的类型：
// - 基础类型：nil、string、bool、所有整数和浮点数类型
// - 复合类型：map[string]any、[]any
// - 反射类型：通过 reflect 处理自定义类型
//
// 标准化规则：
// 1. 基础类型直接返回
// 2. map 和 slice 递归处理每个元素
// 3. 其他类型通过 reflect 尝试转换
// 4. 无法转换的类型返回错误
func normalizeMetadataValue(value any) (any, error) {
	switch v := value.(type) {
	case nil, string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		// 基础类型直接返回
		return v, nil
	case map[string]any:
		// 递归处理 map 的每个值
		out := make(map[string]any, len(v))
		for key, item := range v {
			normalized, err := normalizeMetadataValue(item)
			if err != nil {
				return nil, err
			}
			out[key] = normalized
		}
		return out, nil
	case []any:
		// 递归处理数组的每个元素
		out := make([]any, len(v))
		for i, item := range v {
			normalized, err := normalizeMetadataValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = normalized
		}
		return out, nil
	}

	// 使用 reflect 处理其他类型
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return nil, nil
	}

	switch rv.Kind() {
	case reflect.Interface, reflect.Pointer:
		// 解引用指针和接口
		if rv.IsNil() {
			return nil, nil
		}
		return normalizeMetadataValue(rv.Elem().Interface())
	case reflect.Map:
		// 处理 reflect.Map 类型，要求键为 string
		if rv.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("skills: metadata map keys must be strings")
		}
		out := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			normalized, err := normalizeMetadataValue(iter.Value().Interface())
			if err != nil {
				return nil, err
			}
			out[iter.Key().String()] = normalized
		}
		return out, nil
	case reflect.Slice, reflect.Array:
		// 处理 reflect.Slice 和 reflect.Array 类型
		out := make([]any, rv.Len())
		for i := range rv.Len() {
			normalized, err := normalizeMetadataValue(rv.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			out[i] = normalized
		}
		return out, nil
	default:
		// 不支持的类型
		return nil, fmt.Errorf("skills: metadata value of type %T is not JSON-compatible", value)
	}
}

// NewFromDir 从本地目录加载所有技能。
//
// 是什么：
// 此函数扫描指定目录及其子目录，发现所有包含 SKILL.md 文件的技能目录，
// 并加载每个技能的元数据、指令和资源文件。
//
// 参数说明：
// - dir: 技能根目录的绝对路径或相对路径
//
// 返回说明：
// - skills: 所有成功加载的技能列表
// - err: 加载过程中的错误（如目录不存在、SKILL.md 格式错误等）
//
// 目录结构要求：
// skills/
// ├── my-skill/
// │   ├── SKILL.md       # 必需：技能元数据和指令
// │   ├── references/    # 可选：参考文档
// │   ├── assets/        # 可选：资源文件
// │   └── scripts/       # 可选：可执行脚本
// └── another-skill/
//     └── SKILL.md
//
// 怎么用：
// skills, err := NewFromDir("./skills")
// if err != nil {
//     log.Fatal(err)
// }
// toolset, err := NewToolset(skills)
func NewFromDir(dir string) ([]Skill, error) {
	return loadAllFS(os.DirFS(dir))
}

// NewFromEmbed 从 embed.FS 加载所有技能。
//
// 是什么：
// 此函数支持从 Go 的 embed.FS（嵌入文件系统）加载技能，
// 适用于将技能文件编译到二进制中的场景。
//
// 为什么需要：
// - 发布时不需要额外的技能文件
// - 确保技能文件的完整性和版本一致性
// - 减少部署复杂度
//
// 参数说明：
// - fsys: embed.FS 实例，通常通过 //go:embed 指令生成
//
// 怎么用：
// //go:embed skills/*
// var skillsFS embed.FS
//
// skills, err := NewFromEmbed(skillsFS)
func NewFromEmbed(fsys fs.FS) ([]Skill, error) {
	return loadAllFS(fsys)
}

// ReadSkillFrontmatter 从本地技能目录读取并验证元数据。
//
// 是什么：
// 此函数读取单个技能目录的 SKILL.md 文件，解析 YAML frontmatter 并验证其合法性。
//
// 参数说明：
// - dir: 技能目录路径（必须包含 SKILL.md）
//
// 返回说明：
// - frontmatter: 解析后的技能元数据
// - err: 读取或验证过程中的错误
//
// 验证内容：
// 1. SKILL.md 文件存在
// 2. YAML frontmatter 格式正确
// 3. name 和 description 字段合法
// 4. 目录名与技能名一致
//
// 怎么用：
// fm, err := ReadSkillFrontmatter("./skills/my-skill")
// if err != nil {
//     log.Fatal(err)
// }
// fmt.Printf("Skill: %s, Description: %s\n", fm.Name, fm.Description)
func ReadSkillFrontmatter(dir string) (Frontmatter, error) {
	fsys := os.DirFS(dir)
	frontmatter, _, err := parseSkillMarkdown(fsys, ".")
	if err != nil {
		return Frontmatter{}, err
	}
	if err := validateSkillRootName(".", dirBaseName(dir), frontmatter.Name); err != nil {
		return Frontmatter{}, err
	}
	return frontmatter, nil
}

// loadAllFS 从文件系统加载所有技能。
//
// 处理流程：
// 1. 检测所有包含 SKILL.md 的目录（技能根目录）
// 2. 过滤嵌套在资源目录中的伪技能目录
// 3. 按顺序加载每个技能
// 4. 验证技能名称唯一性
// 5. 返回技能列表
//
// 为什么需要独立函数：
// - NewFromDir 和 NewFromEmbed 都使用此函数
// - 统一处理技能加载逻辑
func loadAllFS(fsys fs.FS) ([]Skill, error) {
	// 检测所有技能根目录
	roots, err := detectSkillRoots(fsys)
	if err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("skills: SKILL.md not found")
	}
	sort.Strings(roots)
	out := make([]Skill, 0, len(roots))
	nameToRoot := make(map[string]string, len(roots))
	for _, root := range roots {
		// 加载单个技能
		skill, err := loadFS(fsys, root)
		if err != nil {
			return nil, fmt.Errorf("skills: load %q: %w", root, err)
		}
		// 验证目录名与技能名一致
		if err := validateSkillRootName(root, "", skill.Name()); err != nil {
			return nil, fmt.Errorf("skills: load %q: %w", root, err)
		}
		// 检查名称唯一性
		if prevRoot, exists := nameToRoot[skill.Name()]; exists {
			return nil, fmt.Errorf("skills: duplicate skill name %q in %q and %q", skill.Name(), prevRoot, root)
		}
		nameToRoot[skill.Name()] = root
		out = append(out, skill)
	}
	return out, nil
}

// detectSkillRoots 检测所有技能根目录。
//
// 是什么：
// 通过遍历文件系统，查找所有包含 SKILL.md 或 skill.md 的目录。
//
// 处理流程：
// 1. 使用 fs.WalkDir 遍历整个文件系统
// 2. 收集包含 SKILL.md 的目录
// 3. 过滤嵌套在资源目录（references/、assets/、scripts/）中的目录
// 4. 返回排序后的根目录列表
//
// 为什么过滤嵌套目录：
// - 防止技能内部的 SKILL.md 被误认为是独立技能
// - 确保只有顶层技能目录被加载
func detectSkillRoots(fsys fs.FS) ([]string, error) {
	candidates := make(map[string]struct{})
	err := fs.WalkDir(fsys, ".", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// 检查文件名是否为 SKILL.md（大小写不敏感）
		switch d.Name() {
		case "SKILL.md", "skill.md":
			candidates[path.Dir(filePath)] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	roots := make([]string, 0, len(candidates))
	for root := range candidates {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	// 过滤嵌套在资源目录中的目录
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		if isNestedInSkillResourceDir(root, out) {
			continue
		}
		out = append(out, root)
	}
	return out, nil
}

// isNestedInSkillResourceDir 检查目录是否嵌套在已有技能资源目录中。
//
// 是什么：
// 此函数判断一个候选目录是否位于已知技能根目录的 references/、assets/ 或 scripts/ 子目录中。
//
// 为什么需要：
// - 避免将技能内部的 SKILL.md 误识别为独立技能
// - 例如：my-skill/references/doc/SKILL.md 不应被视为独立技能
//
// 检测逻辑：
// 遍历已知技能根目录，检查候选目录是否为其资源子目录的后代。
func isNestedInSkillResourceDir(candidate string, skillRoots []string) bool {
	for _, root := range skillRoots {
		for _, subdir := range []string{"references", "assets", "scripts"} {
			resourceRoot := path.Clean(path.Join(root, subdir))
			if candidate == resourceRoot || strings.HasPrefix(candidate, resourceRoot+"/") {
				return true
			}
		}
	}
	return false
}

// validateSkillRootName 验证技能根目录名与技能名是否一致。
//
// 是什么：
// 此函数确保技能目录的名称与 SKILL.md 中声明的 name 字段一致。
//
// 为什么需要：
// - 保持命名一致性，便于文件组织
// - 防止技能名与目录名不匹配导致的混淆
// - 约定优于配置，减少配置错误
//
// 验证规则：
// - 目录名必须与技能名完全匹配
// - 根目录（.）的特殊处理
func validateSkillRootName(root string, dotRootName string, skillName string) error {
	expectedName, ok := expectedSkillDirName(root, dotRootName)
	if !ok {
		return nil
	}
	if expectedName != skillName {
		return fmt.Errorf("skills: skill name %q does not match directory name %q", skillName, expectedName)
	}
	return nil
}

// expectedSkillDirName 根据根目录路径推导期望的技能名称。
//
// 返回说明：
// - 第一个返回值：期望的技能名称（目录名）
// - 第二个返回值：是否需要验证（根目录为 "." 时不需要）
func expectedSkillDirName(root string, dotRootName string) (string, bool) {
	if root == "." {
		// 根目录为 "." 时，不强制要求目录名与技能名一致
		if dotRootName == "" || dotRootName == "." {
			return "", false
		}
		return dotRootName, true
	}
	// 非根目录时，期望技能名为目录的 basename
	return path.Base(root), true
}

// dirBaseName 获取目录的基准名称（basename）。
//
// 处理逻辑：
// 1. 清理路径
// 2. 尝试转换为绝对路径
// 3. 返回路径的最后一部分
//
// 返回值：
// - 目录名（如 "./skills/my-skill" 返回 "my-skill"）
// - 根目录或无效路径返回空字符串
func dirBaseName(dir string) string {
	clean := filepath.Clean(dir)
	if abs, err := filepath.Abs(clean); err == nil {
		clean = abs
	}
	base := filepath.Base(clean)
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	return base
}

// loadFS 从文件系统的指定根目录加载单个技能。
//
// 处理流程：
// 1. 解析 SKILL.md 获取元数据和指令正文
// 2. 加载 references/ 目录的所有文本文件
// 3. 加载 assets/ 目录的所有二进制文件
// 4. 加载 scripts/ 目录的所有脚本文件
// 5. 创建 staticSkill 实例并返回
//
// 参数说明：
// - fsys: 文件系统接口
// - root: 技能根目录路径（相对于 fsys）
//
// 返回说明：
// - skill: 加载的技能实例
// - err: 加载过程中的错误
func loadFS(fsys fs.FS, root string) (Skill, error) {
	// 解析 SKILL.md
	frontmatter, body, err := parseSkillMarkdown(fsys, root)
	if err != nil {
		return nil, err
	}
	// 加载 references/ 目录
	references, err := loadDirFiles(fsys, path.Join(root, "references"))
	if err != nil {
		return nil, err
	}
	// 加载 assets/ 目录
	assets, err := loadDirBinaryFiles(fsys, path.Join(root, "assets"))
	if err != nil {
		return nil, err
	}
	// 加载 scripts/ 目录
	scripts, err := loadDirFiles(fsys, path.Join(root, "scripts"))
	if err != nil {
		return nil, err
	}
	// 创建 staticSkill 实例
	return &staticSkill{
		frontmatter: frontmatter,
		instruction: body,
		resources: Resources{
			References: references,
			Assets:     assets,
			Scripts:    scripts,
		},
	}, nil
}

// parseSkillMarkdown 解析技能目录的 SKILL.md 文件。
//
// 处理流程：
// 1. 读取 SKILL.md 文件内容
// 2. 分离 YAML frontmatter 和正文
// 3. 解析 YAML frontmatter
// 4. 验证 frontmatter 合法性
// 5. 返回 frontmatter 和正文
//
// SKILL.md 格式示例：
// ---
// name: my-skill
// description: 这是一个示例技能
// ---
//
// # 技能指令
//
// 这里是技能的详细说明...
func parseSkillMarkdown(fsys fs.FS, root string) (Frontmatter, string, error) {
	skillMD, err := readSkillMarkdown(fsys, root)
	if err != nil {
		return Frontmatter{}, "", err
	}
	frontmatterContent, body, err := splitFrontmatterBlock(skillMD)
	if err != nil {
		return Frontmatter{}, "", err
	}
	frontmatter, err := parseFrontmatter(frontmatterContent)
	if err != nil {
		return Frontmatter{}, "", err
	}
	if err := frontmatter.Validate(); err != nil {
		return Frontmatter{}, "", err
	}
	return frontmatter, strings.TrimSpace(body), nil
}

// splitFrontmatterBlock 分离 SKILL.md 的 YAML frontmatter 和正文。
//
// SKILL.md 格式：
// ---              <- frontmatter 开始标记
// name: my-skill   <- YAML 内容
// description: ... <- YAML 内容
// ---              <- frontmatter 结束标记
// # 正文开始        <- 正文内容
//
// 处理流程：
// 1. 检查第一行是否为 "---" 开始标记
// 2. 逐行查找结束标记 "---"
// 3. 返回 frontmatter 内容和正文内容
//
// 错误情况：
// - 第一行不是 "---"
// - 没有找到结束标记
func splitFrontmatterBlock(skillMD string) (string, string, error) {
	firstLine, rest, hasMore := cutLine(skillMD)
	if !isFrontmatterDelimiterLine(firstLine) {
		return "", "", fmt.Errorf("skills: SKILL.md must start with YAML frontmatter")
	}
	if !hasMore {
		return "", "", fmt.Errorf("skills: SKILL.md frontmatter not properly closed with ---")
	}

	search := rest
	frontmatterLen := 0
	for {
		line, remaining, hasNext := cutLine(search)
		if isFrontmatterDelimiterLine(line) {
			return rest[:frontmatterLen], remaining, nil
		}
		if !hasNext {
			break
		}
		frontmatterLen += len(line) + 1
		search = remaining
	}
	return "", "", fmt.Errorf("skills: SKILL.md frontmatter not properly closed with ---")
}

// cutLine 切割一行内容，返回当前行、剩余内容和是否有更多行。
// 用于逐行解析文件内容。
func cutLine(content string) (line string, rest string, hasMore bool) {
	idx := strings.IndexByte(content, '\n')
	if idx < 0 {
		return content, "", false
	}
	return content[:idx], content[idx+1:], true
}

// isFrontmatterDelimiterLine 检查一行是否为 frontmatter 分隔线（"---"）。
// 自动处理行尾的回车符（\r）。
func isFrontmatterDelimiterLine(line string) bool {
	return trimTrailingCarriageReturn(line) == "---"
}

// trimTrailingCarriageReturn 移除行尾的回车符（\r）。
// 用于处理 Windows 换行符（\r\n）。
func trimTrailingCarriageReturn(line string) string {
	if strings.HasSuffix(line, "\r") {
		return line[:len(line)-1]
	}
	return line
}

// readSkillMarkdown 读取技能目录的 SKILL.md 文件内容。
//
// 查找顺序：
// 1. 先查找 "SKILL.md"（大写）
// 2. 再查找 "skill.md"（小写）
//
// 这种设计支持不同操作系统的命名习惯。
func readSkillMarkdown(fsys fs.FS, root string) (string, error) {
	for _, name := range []string{"SKILL.md", "skill.md"} {
		b, err := fs.ReadFile(fsys, path.Join(root, name))
		if err == nil {
			return string(b), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("skills: SKILL.md not found")
}

// parseFrontmatter 解析 YAML frontmatter 内容为 Frontmatter 结构体。
//
// 处理流程：
// 1. 使用 yaml.v3 解析 YAML 为 map[string]any
// 2. 提取并验证必需字段（name、description）
// 3. 提取可选字段（license、compatibility、allowed-tools、metadata）
// 4. 标准化 metadata 字段
//
// 支持的字段：
// - name (必需): 技能名称
// - description (必需): 技能描述
// - license (可选): 许可证
// - compatibility (可选): 兼容性说明
// - allowed-tools | allowed_tools (可选): 允许的工具模式
// - metadata (可选): 自定义元数据
func parseFrontmatter(content string) (Frontmatter, error) {
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
		return Frontmatter{}, fmt.Errorf("skills: invalid YAML in frontmatter: %w", err)
	}
	if raw == nil {
		return Frontmatter{}, fmt.Errorf("skills: frontmatter must be a mapping")
	}
	f := Frontmatter{}
	// 解析必需字段：name
	name, ok := raw["name"].(string)
	if !ok {
		return Frontmatter{}, fmt.Errorf("skills: name must be a string")
	}
	f.Name = name
	// 解析必需字段：description
	description, ok := raw["description"].(string)
	if !ok {
		return Frontmatter{}, fmt.Errorf("skills: description must be a string")
	}
	f.Description = description
	// 解析可选字段：license
	if v, ok := raw["license"]; ok {
		s, ok := v.(string)
		if !ok {
			return Frontmatter{}, fmt.Errorf("skills: license must be a string")
		}
		f.License = s
	}
	// 解析可选字段：compatibility
	if v, ok := raw["compatibility"]; ok {
		s, ok := v.(string)
		if !ok {
			return Frontmatter{}, fmt.Errorf("skills: compatibility must be a string")
		}
		f.Compatibility = s
	}
	// 解析可选字段：allowed-tools（支持两种命名风格）
	switch {
	case raw["allowed-tools"] != nil:
		s, ok := raw["allowed-tools"].(string)
		if !ok {
			return Frontmatter{}, fmt.Errorf("skills: allowed-tools must be a string")
		}
		f.AllowedTools = s
	case raw["allowed_tools"] != nil:
		s, ok := raw["allowed_tools"].(string)
		if !ok {
			return Frontmatter{}, fmt.Errorf("skills: allowed_tools must be a string")
		}
		f.AllowedTools = s
	}
	// 解析可选字段：metadata
	if v, ok := raw["metadata"]; ok {
		items, err := normalizeMetadataMap(v)
		if err != nil {
			return Frontmatter{}, err
		}
		f.Metadata = items
	}
	return f, nil
}

// loadDirFiles 加载目录下所有文本文件。
//
// 处理流程：
// 1. 使用 fs.WalkDir 遍历目录
// 2. 跳过子目录
// 3. 读取每个文件内容
// 4. 存储为相对路径 -> 内容的 map
//
// 返回说明：
// - 目录不存在时返回空 map，不报错
// - 文件内容为字符串
func loadDirFiles(fsys fs.FS, dir string) (map[string]string, error) {
	files := make(map[string]string)
	err := fs.WalkDir(fsys, dir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fs.SkipDir
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(filePath, dir+"/")
		files[rel] = string(b)
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return files, nil
}

// loadDirBinaryFiles 加载目录下所有二进制文件。
//
// 与 loadDirFiles 的区别：
// - 返回 []byte 而非 string
// - 适用于图片、PDF 等二进制资源
//
// 返回说明：
// - 目录不存在时返回空 map，不报错
func loadDirBinaryFiles(fsys fs.FS, dir string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	err := fs.WalkDir(fsys, dir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fs.SkipDir
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(filePath, dir+"/")
		files[rel] = b
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return files, nil
}
