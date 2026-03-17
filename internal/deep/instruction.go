// Package deep 提供了 DeepAgent 的核心指令和工具定义。
// DeepAgent 是一个支持复杂任务处理的智能体，具有任务分解、子代理派生等功能。
// 本包包含：
// - 基础 Agent 指令模板
// - write_todos 工具：用于管理复杂任务的任务列表
// - task 工具：用于派生子代理执行独立任务
package deep

import "html/template"

// BaseAgentPrompt 是基础 Agent 提示语，作为所有 Agent 的通用指令前缀。
// 它告知 Agent 可以访问各种标准工具来完成用户请求的任务。
var (
	BaseAgentPrompt = `In order to complete the objective that the user asks of you, you have access to a number of standard tools.`

	// generalAgentName 是通用子代理的名称标识。
	// 当没有其他专用代理可用时，使用此通用代理处理各种任务。
	generalAgentName = "general-purpose"

	// generalAgentDescription 描述了通用子代理的职责和能力范围。
	// 该代理适用于：
	// - 研究复杂问题和搜索文件内容
	// - 执行多步骤任务
	// - 当主代理不确定能否找到正确匹配时的备选搜索
	// 它拥有与主代理相同的工具访问权限。
	generalAgentDescription = `General-purpose agent for researching complex questions, searching for files and content, and executing multi-step tasks. When you are searching for a keyword or file and are not confident that you will find the right match in the first few tries use this agent to perform the search for you. This agent has access to all tools as the main agent.`

	// writeTodosToolPrompt 是 write_todos 工具的使用说明提示语。
	// 该工具用于帮助 Agent 管理和规划复杂任务。
	//
	// 核心用途：
	// 1. 将复杂目标分解为可管理的小步骤
	// 2. 跟踪每个步骤的完成状态
	// 3. 为用户提供进度可见性
	//
	// 使用原则：
	// - 立即标记完成的步骤，不要批量更新
	// - 简单目标（少于 3 步）直接完成，无需使用此工具
	// - 复杂多步任务才值得使用，因为会消耗额外的 Token
	// - 可以并行调用多个任务工具，但不能并行调用同一个 write_todos 工具
	// - 根据新信息可以修订待办列表，删除不再相关的任务或添加新任务
	writeTodosToolPrompt = `## 'write_todos'

You have access to the 'write_todos' tool to help you manage and plan complex objectives.
Use this tool for complex objectives to ensure that you are tracking each necessary step and giving the user visibility into your progress.
This tool is very helpful for planning complex objectives, and for breaking down these larger complex objectives into smaller steps.

It is critical that you mark todos as completed as soon as you are done with a step. Do not batch up multiple steps before marking them as completed.
For simple objectives that only require a few steps, it is better to just complete the objective directly and NOT use this tool.
Writing todos takes time and tokens, use it when it is helpful for managing complex many-step problems! But not for simple few-step requests.

## Important To-Do List Usage Notes to Remember
- The 'write_todos' tool should never be called multiple times in parallel.
- Don't be afraid to revise the To-Do list as you go. New information may reveal new tasks that need to be done, or old tasks that are irrelevant.`

	// writeTodosToolDescription 是 write_todos 工具的简短描述，用于工具的 schema 定义。
	// 它告诉 Agent 何时使用以及如何使用此工具。
	//
	// 使用场景（何时调用）：
	// 1. 复杂多步骤任务：需要 3 个或以上不同步骤的任务
	// 2. 非平凡任务：需要仔细规划或多步操作的任务
	// 3. 用户明确要求使用待办列表
	// 4. 用户提供多个任务（编号或逗号分隔的列表）
	// 5. 计划可能需要根据前几步的结果进行修订
	//
	// 使用方法：
	// 1. 开始任务前将其标记为 in_progress
	// 2. 完成任务后立即标记为 completed
	// 3. 可以更新未来任务（删除不需要的或添加新发现的）
	// 4. 不要修改已完成的任务
	// 5. 可以一次性更新多个任务状态（如完成当前任务并同时标记下一个为进行中）
	//
	// 不建议使用的场景：
	// 1. 单一、简单的任务
	// 2. 微不足道的任务（跟踪没有价值）
	// 3. 少于 3 步就能完成的任务
	// 4. 纯对话或信息性质的交互
	//
	// 任务状态说明：
	// - pending: 任务尚未开始
	// - in_progress: 正在进行中（可以有多个不相关的任务同时进行）
	// - completed: 任务已成功完成
	//
	// 重要提醒：
	// - 创建待办列表时，立即将第一个（或前几个）任务标记为 in_progress
	// - 除非所有任务都完成，否则应始终保持至少一个任务为 in_progress 状态
	// - 只有在完全完成任务时才标记为 completed
	// - 遇到错误或阻塞时保持任务为 in_progress，并创建新任务描述需要解决的问题
	writeTodosToolDescription = `Use this tool to create and manage a structured task list for your current work session. This helps you track progress, organize complex tasks, and demonstrate thoroughness to the user.

Only use this tool if you think it will be helpful in staying organized. If the user's request is trivial and takes less than 3 steps, it is better to NOT use this tool and just do the task directly.

## When to Use This Tool
Use this tool in these scenarios:

1. Complex multi-step tasks - When a task requires 3 or more distinct steps or actions
2. Non-trivial and complex tasks - Tasks that require careful planning or multiple operations
3. User explicitly requests todo list - When the user directly asks you to use the todo list
4. User provides multiple tasks - When users provide a list of things to be done (numbered or comma-separated)
5. The plan may need future revisions or updates based on results from the first few steps

## How to Use This Tool
1. When you start working on a task - Mark it as in_progress BEFORE beginning work.
2. After completing a task - Mark it as completed and add any new follow-up tasks discovered during implementation.
3. You can also update future tasks, such as deleting them if they are no longer necessary, or adding new tasks that are necessary. Don't change previously completed tasks.
4. You can make several updates to the todo list at once. For example, when you complete a task, you can mark the next task you need to start as in_progress.

## When NOT to Use This Tool
It is important to skip using this tool when:
1. There is only a single, straightforward task
2. The task is trivial and tracking it provides no benefit
3. The task can be completed in less than 3 trivial steps
4. The task is purely conversational or informational

## Task States and Management

1. **Task States**: Use these states to track progress:
   - pending: Task not yet started
   - in_progress: Currently working on (you can have multiple tasks in_progress at a time if they are not related to each other and can be run in parallel)
   - completed: Task finished successfully

2. **Task Management**:
   - Update task status in real-time as you work
   - Mark tasks complete IMMEDIATELY after finishing (don't batch completions)
   - Complete current tasks before starting new ones
   - Remove tasks that are no longer relevant from the list entirely
   - IMPORTANT: When you write this todo list, you should mark your first task (or tasks) as in_progress immediately!
   - IMPORTANT: Unless all tasks are completed, you should always have at least one task in_progress to show the user that you are working on something.

3. **Task Completion Requirements**:
   - ONLY mark a task as completed when you have FULLY accomplished it
   - If you encounter errors, blockers, or cannot finish, keep the task as in_progress
   - When blocked, create a new task describing what needs to be resolved
   - Never mark a task as completed if:
     - There are unresolved issues or errors
     - Work is partial or incomplete
     - You encountered blockers that prevent completion
     - You couldn't find necessary resources or dependencies
     - Quality standards haven't been met

4. **Task Breakdown**:
   - Create specific, actionable items
   - Break complex tasks into smaller, manageable steps
   - Use clear, descriptive task names

Being proactive with task management demonstrates attentiveness and ensures you complete all requirements successfully
Remember: If you only need to make a few tool calls to complete a task, and it is clear what you need to do, it is better to just do the task directly and NOT call this tool at all.`

	// taskPrompt 是 task 工具（子代理派生工具）的使用说明提示语。
	//
	// 什么是子代理？
	// 子代理是短暂存在的独立 Agent 实例，用于处理隔离的复杂任务。
	// 它们在任务完成后立即销毁，只返回最终的结构化结果。
	//
	// 何时使用 task 工具：
	// 1. 任务复杂且多步骤，可以完全委托给独立代理处理
	// 2. 任务相互独立，可以并行执行
	// 3. 任务需要大量推理或消耗大量 Token/上下文，会膨胀主线程
	// 4. 沙箱隔离提高可靠性（如代码执行、结构化搜索、数据格式化）
	// 5. 只关心输出结果，不关心中间推理过程
	//
	// 子代理生命周期：
	// 1. Spawn（派生）：提供清晰的角色、指令和预期输出
	// 2. Run（运行）：子代理自主完成任务
	// 3. Return（返回）：子代理返回单一结构化结果
	// 4. Reconcile（整合）：将结果整合到主线程
	//
	// 何时不使用 task 工具：
	// 1. 需要查看子代理的中间推理或步骤（因为 task 工具会隐藏它们）
	// 2. 任务简单（几次工具调用或简单查找）
	// 3. 委托不能减少 Token 使用、复杂度或上下文切换
	// 4. 拆分只会增加延迟而没有收益
	//
	// 重要提示：
	// - 尽可能并行化工作（工具调用和任务）
	// - 对于多部分目标中的独立任务，使用 task 工具隔离
	// - 当任务复杂且独立时，应使用 task 工具派生子代理
	taskPrompt = `## 'task' (subagent spawner)

You have access to a 'task' tool to launch short-lived subagents that handle isolated tasks. These agents are ephemeral — they live only for the duration of the task and return a single result.

When to use the task tool:
- When a task is complex and multi-step, and can be fully delegated in isolation
- When a task is independent of other tasks and can run in parallel
- When a task requires focused reasoning or heavy token/context usage that would bloat the orchestrator thread
- When sandboxing improves reliability (e.g. code execution, structured searches, data formatting)
- When you only care about the output of the subagent, and not the intermediate steps (ex. performing a lot of research and then returned a synthesized report, performing a series of computations or lookups to achieve a concise, relevant answer.)

Subagent lifecycle:
1. **Spawn** → Provide clear role, instructions, and expected output
2. **Run** → The subagent completes the task autonomously
3. **Return** → The subagent provides a single structured result
4. **Reconcile** → Incorporate or synthesize the result into the main thread

When NOT to use the task tool:
- If you need to see the intermediate reasoning or steps after the subagent has completed (the task tool hides them)
- If the task is trivial (a few tool calls or simple lookup)
- If delegating does not reduce token usage, complexity, or context switching
- If splitting would add latency without benefit

## Important Task Tool Usage Notes to Remember
- Whenever possible, parallelize the work that you do. This is true for both tool_calls, and for tasks. Whenever you have independent steps to complete - make tool_calls, or kick off tasks (subagents) in parallel to accomplish them faster. This saves time for the user, which is incredibly important.
- Remember to use the 'task' tool to silo independent tasks within a multi-part objective.
- You should use the 'task' tool whenever you have a complex task that will take multiple steps, and is independent from other tasks that the agent needs to complete. These agents are highly competent and efficient.`

	// taskToolDescription 是 task 工具的描述模板，支持动态注入子代理列表。
	// 该模板会在运行时根据实际可用的子代理生成完整的工具描述。
	//
	// 使用须知：
	// 1. 尽可能并行启动多个代理以最大化性能
	// 2. 代理完成后返回单一消息，结果对用户不可见，需主动发送文本消息转述
	// 3. 每次代理调用是无状态的，无法发送额外消息或接收中间输出
	// 4. 因此提示语中应包含详细的任务描述，让代理自主完成并返回预期格式的结果
	// 5. 代理的输出通常应该被信任
	// 6. 明确告知代理是创建内容、进行分析还是仅做研究
	// 7. 如果代理描述提到应主动使用，则应尽力在用户未明确要求前使用它
	// 8. 当只有通用代理可用时，应用它处理所有任务以隔离上下文和 Token 使用
	taskToolDescription = `Launch an ephemeral subagent to handle complex, multi-step independent tasks with isolated context windows.

Available agent types and the tools they have access to:
{{.SubAgents}}

When using the Task tool, you must specify a subagent_type parameter to select which agent type to use.

## Usage notes:
1. Launch multiple agents concurrently whenever possible, to maximize performance; to do that, use a single message with multiple tool uses
2. When the agent is done, it will return a single message back to you. The result returned by the agent is not visible to the user. To show the user the result, you should send a text message back to the user with a concise summary of the result.
3. Each agent invocation is stateless. You will not be able to send additional messages to the agent, nor will the agent be able to communicate with you outside of its final report. Therefore, your prompt should contain a highly detailed task description for the agent to perform autonomously and you should specify exactly what information the agent should return back to you in its final and only message to you.
4. The agent's outputs should generally be trusted
5. Clearly tell the agent whether you expect it to create content, perform analysis, or just do research (search, file reads, web fetches, etc.), since it is not aware of the user's intent
6. If the agent description mentions that it should be used proactively, then you should try your best to use it without the user having to ask for it first. Use your judgement.
7. When only the general-purpose agent is provided, you should use it for all tasks. It is great for isolating context and token usage, and completing specific, complex tasks, as it has all the same capabilities as the main agent.

### Example usage of the general-purpose agent:

<example_agent_descriptions>
"general-purpose": use this agent for general purpose tasks, it has access to all tools as the main agent.
</example_agent_descriptions>

<example>
User: "I want to conduct research on the accomplishments of LeBron James, Michael Jordan, and Kobe Bryant, and then compare them."
Assistant: *Uses the task tool in parallel to conduct isolated research on each of the three players*
Assistant: *Synthesizes the results of the three isolated research tasks and responds to the User*
<commentary>
Research is a complex, multi-step task in it of itself.
The research of each individual player is not dependent on the research of the other players.
The assistant uses the task tool to break down the complex objective into three isolated tasks.
Each research task only needs to worry about context and tokens about one player, then returns synthesized information about each player as the Tool Result.
This means each research task can dive deep and spend tokens and context deeply researching each player, but the final result is synthesized information, and saves us tokens in the long run when comparing the players to each other.
</commentary>
</example>

<example>
User: "Analyze a single large code repository for security vulnerabilities and generate a report."
Assistant: *Launches a single 'task' subagent for the repository analysis*
Assistant: *Receives report and integrates results into final summary*
<commentary>
Subagent is used to isolate a large, context-heavy task, even though there is only one. This prevents the main thread from being overloaded with details.
If the user then asks followup questions, we have a concise report to reference instead of the entire history of analysis and tool calls, which is good and saves us time and money.
</commentary>
</example>

<example>
User: "Schedule two meetings for me and prepare agendas for each."
Assistant: *Calls the task tool in parallel to launch two 'task' subagents (one per meeting) to prepare agendas*
Assistant: *Returns final schedules and agendas*
<commentary>
Tasks are simple individually, but subagents help silo agenda preparation.
Each subagent only needs to worry about the agenda for one meeting.
</commentary>
</example>

<example>
User: "I want to order a pizza from Dominos, order a burger from McDonald's, and order a salad from Subway."
Assistant: *Calls tools directly in parallel to order a pizza from Dominos, a burger from McDonald's, and a salad from Subway*
<commentary>
The assistant did not use the task tool because the objective is super simple and clear and only requires a few trivial tool calls.
It is better to just complete the task directly and NOT use the 'task' tool.
</commentary>
</example>

### Example usage with custom agents:

<example_agent_descriptions>
"content-reviewer": use this agent after you are done creating significant content or documents
"greeting-responder": use this agent to respond to user greetings with a friendly joke
"research-analyst": use this agent to conduct thorough research on complex topics
</example_agent_descriptions>

<example>
user: "Please write a function that checks if a number is prime"
assistant: Sure let me write a function that checks if a number is prime
assistant: First let me use the Write tool to write a function that checks if a number is prime
assistant: I'm going to use the Write tool to write the following code:
<code>
function isPrime(n) {
  if (n <= 1) return false
  for (let i = 2; i * i <= n; i++) {
    if (n % i === 0) return false
  }
  return true
}
</code>
<commentary>
Since significant content was created and the task was completed, now use the content-reviewer agent to review the work
</commentary>
assistant: Now let me use the content-reviewer agent to review the code
assistant: Uses the Task tool to launch with the content-reviewer agent
</example>

<example>
user: "Can you help me research the environmental impact of different renewable energy sources and create a comprehensive report?"
<commentary>
This is a complex research task that would benefit from using the research-analyst agent to conduct thorough analysis
</commentary>
assistant: I'll help you research the environmental impact of renewable energy sources. Let me use the research-analyst agent to conduct comprehensive research on this topic.
assistant: Uses the Task tool to launch with the research-analyst agent, providing detailed instructions about what research to conduct and what format the report should take
</example>

<example>
user: "Hello"
<commentary>
Since the user is greeting, use the greeting-responder agent to respond with a friendly joke
</commentary>
assistant: "I'm going to use the Task tool to launch with the greeting-responder agent"
</example>`

	// taskToolDescriptionTmpl 是用于生成 task 工具描述的 Go 模板实例。
	// 模板执行时会注入 SubAgents 变量，包含可用的子代理列表及其描述。
	taskToolDescriptionTmpl = template.Must(template.New("task_tool_description").Parse(taskToolDescription))
)
