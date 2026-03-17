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
