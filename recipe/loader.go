package recipe

import (
	"fmt"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadFromFile 从 YAML 文件路径加载并解析 AgentSpec
// 这是最常用的加载方式，直接从文件系统读取配方配置文件
//
// 使用示例:
//
//	spec, err := recipe.LoadFromFile("recipes/code_reviewer.yaml")
//	if err != nil {
//	    return nil, err
//	}
//
// 文件内容会被解析为 AgentSpec 结构体，然后通过 Validate 函数验证有效性
func LoadFromFile(path string) (*AgentSpec, error) {
	// 读取文件内容为字节切片
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("recipe: failed to read file %q: %w", path, err)
	}
	// 调用 Parse 函数解析 YAML 内容并验证
	return Parse(data)
}

// LoadFromFS 从 fs.FS（文件系统接口）加载并解析 AgentSpec
// 此函数支持从嵌入的文件系统（如 embed.FS）加载配方，适用于将配置文件打包到二进制中的场景
//
// 使用示例（使用 embed）:
//
//	//go:embed recipes/*.yaml
//	var recipeFS embed.FS
//
//	spec, err := recipe.LoadFromFS(recipeFS, "recipes/code_reviewer.yaml")
func LoadFromFS(fsys fs.FS, path string) (*AgentSpec, error) {
	// 从 fs.FS 接口读取文件内容
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("recipe: failed to read %q from fs: %w", path, err)
	}
	// 调用 Parse 函数解析 YAML 内容并验证
	return Parse(data)
}

// Parse 解析原始 YAML 字节为 AgentSpec 并验证其有效性
// 此函数是加载配方的核心函数，LoadFromFile 和 LoadFromFS 最终都调用此函数
//
// 解析流程：
// 1. 使用 yaml.v3 库反序列化 YAML 字节到 AgentSpec 结构体
// 2. 调用 Validate 函数验证配方的有效性（必需字段、类型检查等）
// 3. 返回验证通过的 AgentSpec 或错误
func Parse(data []byte) (*AgentSpec, error) {
	var spec AgentSpec
	// 反序列化 YAML 字节到结构体
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("recipe: failed to parse YAML: %w", err)
	}
	// 验证配方的有效性
	if err := Validate(&spec); err != nil {
		return nil, err
	}
	return &spec, nil
}
