package recipe

import (
	"fmt"
	"slices"
)

// Validate 检查 AgentSpec 的一致性和必需字段
// 这是配方加载后的第一步验证，确保配置在构建前是有效的
//
// 验证内容包括：
// 1. 基本字段：version、name、instruction 等必需字段
// 2. 模型配置：无子 Agent 时必须有 model，有子 Agent 时根据模式检查
// 3. 执行模式：有子 Agent 时必须指定 execution，且值必须有效
// 4. 参数验证：参数名称、类型、默认值等
// 5. 上下文配置：策略类型、参数范围等
// 6. 中间件配置：名称唯一性等
// 7. 子 Agent 验证：递归验证每个子 Agent 的配置
// 8. 命名冲突：检查子 Agent 名称、工具名称是否重复
func Validate(spec *AgentSpec) error {
	if spec == nil {
		return fmt.Errorf("recipe: spec is required")
	}
	// version 是必需字段，用于标识配置格式版本
	if spec.Version == "" {
		return fmt.Errorf("recipe: version is required")
	}
	// name 是必需字段，用于识别 Agent
	if spec.Name == "" {
		return fmt.Errorf("recipe: name is required")
	}
	// instruction 是必需字段，但在顺序/并行/循环模式下例外
	// 因为这些流程 Agent 只是编排子 Agent，本身不调用 LLM
	if spec.Instruction == "" && spec.Execution != ExecutionSequential &&
		spec.Execution != ExecutionParallel && spec.Execution != ExecutionLoop {
		return fmt.Errorf("recipe: instruction is required")
	}
	// 无子 Agent 时，必须指定 model
	if len(spec.SubAgents) == 0 && spec.Model == "" {
		return fmt.Errorf("recipe: model is required when there are no sub_agents")
	}
	// 有子 Agent 时，必须指定 execution 模式
	if len(spec.SubAgents) > 0 && spec.Execution == "" {
		return fmt.Errorf("recipe: execution mode is required when sub_agents are defined")
	}
	// 检查 execution 模式是否有效
	if spec.Execution != "" && spec.Execution != ExecutionSequential &&
		spec.Execution != ExecutionParallel && spec.Execution != ExecutionTool &&
		spec.Execution != ExecutionLoop {
		return fmt.Errorf("recipe: invalid execution mode %q (must be sequential, parallel, tool, or loop)", spec.Execution)
	}
	// tool 模式需要 parent model 来做决策调用
	if spec.Execution == ExecutionTool && spec.Model == "" {
		return fmt.Errorf("recipe: model is required for tool execution mode")
	}
	// 顺序/并行模式使用 flow Agent，不支持 output_key 和 max_iterations
	if spec.Execution == ExecutionSequential || spec.Execution == ExecutionParallel {
		if spec.OutputKey != "" {
			return fmt.Errorf("recipe %q: output_key is not supported in %s mode", spec.Name, spec.Execution)
		}
		if spec.MaxIterations > 0 {
			return fmt.Errorf("recipe %q: max_iterations is not supported in %s mode", spec.Name, spec.Execution)
		}
	}
	// loop 模式：output_key 不受支持，因为 LoopAgent 本身不调用 LLM
	if spec.Execution == ExecutionLoop && spec.OutputKey != "" {
		return fmt.Errorf("recipe %q: output_key is not supported in loop mode", spec.Name)
	}
	// 验证参数列表
	if err := validateParameters(spec.Parameters); err != nil {
		return fmt.Errorf("recipe %q: %w", spec.Name, err)
	}
	// 验证上下文配置
	if err := validateContextSpec(spec.Context); err != nil {
		return fmt.Errorf("recipe %q: context: %w", spec.Name, err)
	}
	// 验证中间件列表
	if err := validateMiddlewares(fmt.Sprintf("recipe %q", spec.Name), spec.Middlewares); err != nil {
		return err
	}
	// 验证工具名称并返回工具名称集合（用于检查冲突）
	toolNames, err := validateToolNames(fmt.Sprintf("recipe %q", spec.Name), spec.Tools)
	if err != nil {
		return err
	}
	// 检查子 Agent 名称唯一性
	subNames := make(map[string]bool, len(spec.SubAgents))
	for i := range spec.SubAgents {
		sub := &spec.SubAgents[i]
		// 递归验证子 Agent 配置
		if err := validateSubAgent(sub, i); err != nil {
			return fmt.Errorf("recipe %q: %w", spec.Name, err)
		}
		// 检查名称重复
		if subNames[sub.Name] {
			return fmt.Errorf("recipe %q: duplicate sub_agent name %q", spec.Name, sub.Name)
		}
		subNames[sub.Name] = true
		// tool 模式下，子 Agent 名称不能与外部工具名称冲突
		if spec.Execution == ExecutionTool && toolNames[sub.Name] {
			return fmt.Errorf("recipe %q: sub_agent %q conflicts with an external tool of the same name", spec.Name, sub.Name)
		}
		// tool 模式下，子 Agent 不支持 output_key
		if spec.Execution == ExecutionTool && sub.OutputKey != "" {
			return fmt.Errorf("recipe %q: sub_agent %q: output_key is not supported in tool mode", spec.Name, sub.Name)
		}
		// 顺序/并行/循环模式下，如果父 Agent 没有 model，每个子 Agent 必须指定自己的 model
		if (spec.Execution == ExecutionSequential || spec.Execution == ExecutionParallel || spec.Execution == ExecutionLoop) &&
			spec.Model == "" && sub.Model == "" {
			return fmt.Errorf("recipe %q: sub_agent %q: model is required when parent has no model", spec.Name, sub.Name)
		}
	}
	return nil
}

// validateMiddlewares 验证中间件列表
// 检查每个中间件是否有名称，且名称不重复
func validateMiddlewares(scope string, specs []MiddlewareSpec) error {
	seen := make(map[string]bool, len(specs))
	for i, mw := range specs {
		if mw.Name == "" {
			return fmt.Errorf("%s: middleware[%d]: name is required", scope, i)
		}
		if seen[mw.Name] {
			return fmt.Errorf("%s: duplicate middleware name %q", scope, mw.Name)
		}
		seen[mw.Name] = true
	}
	return nil
}

// validateContextSpec 验证上下文配置规范
// 检查策略类型是否有效，参数是否在合理范围内
func validateContextSpec(spec *ContextSpec) error {
	if spec == nil {
		return nil
	}
	switch spec.Strategy {
	case ContextStrategySummarize:
		// summarize 策略：model 是可选的，构建时会回退到 Agent 的模型
	case ContextStrategyWindow:
		// window 策略：期望至少配置 max_tokens 或 max_messages 之一
	case "":
		return fmt.Errorf("strategy is required")
	default:
		return fmt.Errorf("unknown strategy %q (must be %q or %q)", spec.Strategy, ContextStrategySummarize, ContextStrategyWindow)
	}
	// max_tokens 不能为负数
	if spec.MaxTokens < 0 {
		return fmt.Errorf("max_tokens must be >= 0")
	}
	return nil
}

// validateParameters 验证参数列表
// 检查每个参数的必需字段、类型有效性、默认值类型匹配等
func validateParameters(params []ParameterSpec) error {
	seen := make(map[string]bool, len(params))
	for _, p := range params {
		// name 是必需字段
		if p.Name == "" {
			return fmt.Errorf("parameter name is required")
		}
		// 检查名称重复
		if seen[p.Name] {
			return fmt.Errorf("duplicate parameter %q", p.Name)
		}
		seen[p.Name] = true
		// type 是必需字段
		if p.Type == "" {
			return fmt.Errorf("parameter %q: type is required", p.Name)
		}
		// 检查类型是否有效
		if p.Type != ParameterString && p.Type != ParameterNumber &&
			p.Type != ParameterBoolean && p.Type != ParameterSelect {
			return fmt.Errorf("parameter %q: invalid type %q", p.Name, p.Type)
		}
		// 检查 required 字段是否有效
		if p.Required != "" && p.Required != ParameterRequired && p.Required != ParameterOptional {
			return fmt.Errorf("parameter %q: invalid required value %q", p.Name, p.Required)
		}
		// select 类型必须有 options
		if p.Type == ParameterSelect && len(p.Options) == 0 {
			return fmt.Errorf("parameter %q: select type requires options", p.Name)
		}
		// 验证默认值类型是否与声明的类型匹配
		if p.Default != nil {
			if err := validateParamType("default value", p, p.Default); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateSubAgent 验证子 Agent 配置
// index 用于在错误信息中标识子 Agent 的位置
func validateSubAgent(sub *SubAgentSpec, index int) error {
	// name 是必需字段
	if sub.Name == "" {
		return fmt.Errorf("sub_agent[%d]: name is required", index)
	}
	// instruction 是必需字段
	if sub.Instruction == "" {
		return fmt.Errorf("sub_agent %q: instruction is required", sub.Name)
	}
	// 递归验证子 Agent 的参数
	if err := validateParameters(sub.Parameters); err != nil {
		return fmt.Errorf("sub_agent %q: %w", sub.Name, err)
	}
	// 验证子 Agent 的工具名称
	if _, err := validateToolNames(fmt.Sprintf("sub_agent %q", sub.Name), sub.Tools); err != nil {
		return err
	}
	// 验证子 Agent 的上下文配置
	if err := validateContextSpec(sub.Context); err != nil {
		return fmt.Errorf("sub_agent %q: context: %w", sub.Name, err)
	}
	// 验证子 Agent 的中间件列表
	if err := validateMiddlewares(fmt.Sprintf("sub_agent %q", sub.Name), sub.Middlewares); err != nil {
		return err
	}
	return nil
}

// validateToolNames 验证工具名称列表
// 检查名称非空且不重复，返回工具名称集合用于后续冲突检查
func validateToolNames(scope string, toolNames []string) (map[string]bool, error) {
	seen := make(map[string]bool, len(toolNames))
	for _, t := range toolNames {
		if t == "" {
			return nil, fmt.Errorf("%s: tool name must be non-empty", scope)
		}
		if seen[t] {
			return nil, fmt.Errorf("%s: duplicate tool name %q", scope, t)
		}
		seen[t] = true
	}
	return seen, nil
}

// ValidateParams 检查提供的参数值是否满足配方规范
// 用于在构建时验证运行时参数是否正确
func ValidateParams(spec *AgentSpec, params map[string]any) error {
	if spec == nil {
		return fmt.Errorf("recipe: spec is required")
	}
	return validateParamValues(fmt.Sprintf("recipe %q", spec.Name), spec.Parameters, params)
}

// validateParamValues 验证参数值是否满足参数规范
// 检查必需参数是否提供、参数类型是否匹配、select 类型值是否在选项中等
func validateParamValues(scope string, paramSpecs []ParameterSpec, params map[string]any) error {
	for _, p := range paramSpecs {
		val, ok := params[p.Name]
		// 如果参数未提供但有默认值，跳过验证（使用默认值）
		if !ok && p.Default != nil {
			continue
		}
		// 如果参数是必需的但未提供，返回错误
		if !ok && p.Required == ParameterRequired {
			return fmt.Errorf("%s: required parameter %q is missing", scope, p.Name)
		}
		// 如果参数未提供且不是必需的，跳过
		if !ok {
			continue
		}
		// 验证参数值类型是否正确
		if err := validateParamType(scope, p, val); err != nil {
			return err
		}
	}
	return nil
}

// validateParamType 验证参数值的类型是否与声明的类型匹配
func validateParamType(scope string, p ParameterSpec, val any) error {
	switch p.Type {
	case ParameterString:
		// 字符串类型：检查是否是 string
		if _, ok := val.(string); !ok {
			return fmt.Errorf("%s: parameter %q must be a string", scope, p.Name)
		}
	case ParameterNumber:
		// 数字类型：检查是否是任何整数或浮点数类型
		switch val.(type) {
		case int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64,
			float32, float64:
		default:
			return fmt.Errorf("%s: parameter %q must be a number", scope, p.Name)
		}
	case ParameterBoolean:
		// 布尔类型：检查是否是 bool
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("%s: parameter %q must be a boolean", scope, p.Name)
		}
	case ParameterSelect:
		// 选择类型：检查是否是字符串且在选项列表中
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("%s: parameter %q must be a string for select type", scope, p.Name)
		}
		if !slices.Contains(p.Options, s) {
			return fmt.Errorf("%s: parameter %q value %q is not in options %v", scope, p.Name, s, p.Options)
		}
	}
	return nil
}
