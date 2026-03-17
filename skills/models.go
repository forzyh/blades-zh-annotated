package skills

import (
	"html"
	"strings"
)

// DefaultSystemInstruction 是启用技能时注入的系统级指令。
//
// 是什么：
// 这是一段预定义的提示词，告知 Agent 可以使用技能来增强能力。
//
// 为什么需要：
// - 让 Agent 知道技能系统的存在
// - 说明技能目录结构（SKILL.md、references/、assets/、scripts/）
// - 指导 Agent 如何正确使用技能工具
//
// 怎么用：
// 此指令会在 Agent 初始化时添加到系统提示中，与技能列表一起构成完整的技能说明。
// Agent 会根据此指令的指引，在需要时使用 list_skills、load_skill 等工具。
const DefaultSystemInstruction = `You can use specialized 'skills' to help you with complex tasks. You MUST use the skill tools to interact with these skills.

Skills are folders of instructions and resources that extend your capabilities for specialized tasks. Each skill folder contains:
- SKILL.md (required): The main instruction file with skill metadata and detailed markdown instructions.
- references/ (optional): Additional documentation or examples for skill usage.
- assets/ (optional): Templates, docs, or other resources used by the skill.
- scripts/ (optional): Script files that can be inspected with load_skill_resource and executed with run_skill_script.

This is very important:
1. If a skill seems relevant, use load_skill with name="<SKILL_NAME>" to read full instructions first.
2. Once loaded, follow skill instructions exactly before replying.
3. Use load_skill_resource for files inside references/, assets/, and scripts/.
4. Use run_skill_script for scripts under scripts/ when execution is required.`

// FormatSkillsAsXML 将技能列表格式化为 XML 块。
//
// 是什么：
// 此函数将所有可用技能的信息转换为 XML 格式，供 Agent 在系统提示中查看。
//
// 为什么用 XML：
// - 结构清晰，易于解析
// - 与 Markdown 内容区分明显
// - 便于 Agent 识别和提取技能信息
//
// 参数说明：
// - skills: Skill 接口切片，包含所有已加载的技能
//
// 返回值：
// - 格式化的 XML 字符串，包含所有技能的名称和描述
// - 空技能列表返回空的 XML 块
//
// 怎么用：
// 通常在 Toolset 初始化时调用，将结果添加到系统指令中。
// Agent 看到 XML 后可以知道有哪些技能可用，然后决定是否加载。
func FormatSkillsAsXML(skills []Skill) string {
	if len(skills) == 0 {
		return "<available_skills>\n</available_skills>"
	}
	lines := []string{"<available_skills>"}
	for _, skill := range skills {
		if skill == nil {
			// 跳过 nil 技能，避免 panic
			continue
		}
		// 使用 html.EscapeString 转义特殊字符，防止 XML 注入
		lines = append(lines,
			"<skill>",
			"<name>",
			html.EscapeString(skill.Name()),
			"</name>",
			"<description>",
			html.EscapeString(skill.Description()),
			"</description>",
			"</skill>",
		)
	}
	lines = append(lines, "</available_skills>")
	return strings.Join(lines, "\n")
}
