// Package handoff 提供了 Agent 交接（handoff）功能的实现。
// 本文件实现了交接指令的构建函数，用于生成包含可用 Agent 列表的指令文本。
package handoff

import (
	"bytes"
	"html/template"

	"github.com/go-kratos/blades"
)

// handoffInstructionTemplate 是交接指令的 Go 模板。
// 该模板用于生成 Agent 的指令，告知它有哪些可用的子代理可以交接。
//
// 模板结构说明：
// 1. 列出所有可用的子代理及其描述
// 2. 指示 Agent 根据描述判断是否自己最适合处理用户请求
// 3. 如果有更合适的代理，必须使用 handoff_to_agent 工具进行交接
// 4. 如果没有更合适的代理，直接响应用户
//
// 重要规则：
// - 交接时只输出函数调用，不包含任何解释或额外文本
// - 这确保交接动作干净利落，避免混淆
//
// 模板变量：
// - Targets: []blades.Agent 类型，包含所有可用子代理的列表
//
// 使用示例（模板执行后）：
//
//	You have access to the following agents:
//
//	Agent Name: research-agent
//	Agent Description: Specialized in deep research tasks.
//
//	Agent Name: coding-agent
//	Agent Description: Expert in code generation and analysis.
//
//	Your task:
//	- Determine whether you are the most appropriate agent to answer the user's question...
const handoffInstructionTemplate = `You have access to the following agents:
{{range .Targets}}
Agent Name: {{.Name}}
Agent Description: {{.Description}}
{{end}}
Your task:
- Determine whether you are the most appropriate agent to answer the user's question based on your own description.
- If another agent is clearly better suited to handle the user's request, you must transfer the query by calling the "handoff_to_agent" function.
- If no other agent is more suitable, respond to the user directly as a helpful assistant, providing clear, detailed, and accurate information.

Important rules:
- When transferring a query, output only the function call, and nothing else.
- Do not include explanations, reasoning, or any additional text outside of the function call.`

// handoffToAgentPromptTmpl 是从模板字符串编译后的模板实例。
// 使用 template.Must 确保模板在编译时解析，如果解析失败则立即 panic。
//
// 为什么使用 template.Must？
// - 模板字符串是硬编码的，已知是有效的
// - 在编译时失败比在运行时失败更好
// - 避免每次调用都进行模板解析
var handoffToAgentPromptTmpl = template.Must(template.New("handoff_to_agent_prompt").Parse(handoffInstructionTemplate))

// BuildInstruction 构建交接指令字符串。
//
// 参数说明：
// - targets: 可用的子代理列表，每个 Agent 需要提供 Name() 和 Description() 方法
//
// 处理流程：
// 1. 创建 bytes.Buffer 用于存储模板执行结果
// 2. 执行模板，传入 targets 数据
// 3. 返回生成的指令字符串
//
// 返回值：
// - string: 生成的交接指令文本
// - error: 模板执行过程中的错误（通常不会发生，因为模板是预编译的）
//
// 使用示例：
//
//	instruction, err := handoff.BuildInstruction([]blades.Agent{
//	    researchAgent,
//	    codingAgent,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// 将 instruction 作为系统指令传递给 Agent
//	agent := blades.NewAgent("router", blades.WithInstruction(instruction))
func BuildInstruction(targets []blades.Agent) (string, error) {
	var buf bytes.Buffer
	// 执行模板，生成完整的交接指令
	if err := handoffToAgentPromptTmpl.Execute(&buf, map[string]any{
		"Targets": targets,
	}); err != nil {
		return "", err
	}
	return buf.String(), nil
}
