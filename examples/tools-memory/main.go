// Blades 示例：记忆工具（tools-memory）
//
// 本示例演示如何使用 Blades 的记忆工具（Memory Tool）。
// 记忆工具允许 Agent 存储和检索信息，实现长期记忆能力，
// 使 Agent 能够记住用户偏好、历史对话等重要信息。
//
// 适用场景：
// - 个性化助手（记住用户偏好）
// - 长期对话机器人
// - 知识库问答系统
// - 需要上下文记忆的任务
//
// 核心概念：
// 1. MemoryTool：用于存储和检索记忆的专用工具
// 2. InMemoryStore：内存中的记忆存储器
// 3. 记忆检索：根据相关性自动检索记忆
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"log"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/memory"
)

func main() {
	ctx := context.Background()

	// 步骤 1: 创建内存记忆存储器
	// InMemoryStore 是一个简单的内存实现
	// 生产环境中可以使用数据库实现持久化存储
	memoryStore := memory.NewInMemoryStore()

	// 步骤 2: 创建记忆工具
	// MemoryTool 允许 Agent 存储和检索记忆
	// LLM 可以自主决定何时存储或查询记忆
	memoryTool, err := memory.NewMemoryTool(memoryStore)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 预置一些记忆
	// AddMemory 向存储器添加记忆条目
	// Memory 结构包含 Content（消息内容）和 Metadata（元数据）
	memoryStore.AddMemory(ctx, &memory.Memory{
		Content: blades.AssistantMessage("My favorite project is the Blades Agent kit."),
	})
	memoryStore.AddMemory(ctx, &memory.Memory{
		Content: blades.AssistantMessage("My favorite programming language is Go."),
	})

	// 步骤 4: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 5: 创建 Agent 并绑定记忆工具
	agent, err := blades.NewAgent(
		"MemoryRecallAgent",
		blades.WithModel(model),
		blades.WithInstruction("Answer the user's question. Use the 'Memory' tool if the answer might be in past conversations."),
		blades.WithTools(memoryTool), // 绑定记忆工具
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 6: 创建用户输入
	// 询问用户最喜欢的项目
	// Agent 应该会使用记忆工具检索相关信息
	input := blades.UserMessage("What is my favorite project?")

	// 步骤 7: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	output, err := runner.Run(ctx, input)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 8: 输出结果
	log.Println(output.Text())

	// 预期输出：
	// Your favorite project is the Blades Agent kit.
	//
	// 扩展提示：
	// 1. 记忆结构：
	//    type Memory struct {
	//        ID        string    // 记忆 ID
	//        Content   *Message  // 记忆内容
	//        Metadata  map[string]any // 元数据
	//        CreatedAt time.Time // 创建时间
	//    }
	//
	// 2. 持久化存储：
	//    实现 memory.Store 接口使用数据库存储
	//    - PostgreSQL store
	//    - Redis store
	//    - Vector database（用于语义搜索）
	//
	// 3. 记忆检索策略：
	//    - 关键词匹配
	//    - 语义相似度（使用向量嵌入）
	//    - 时间范围筛选
	//    - 重要性排序
	//
	// 4. 记忆管理：
	//    - 定期清理过期记忆
	//    - 合并相似记忆
	//    - 记忆重要性衰减
	//
	// 5. 使用场景：
	//    - 用户偏好："我喜欢吃辣"
	//    - 个人信息："我在北京工作"
	//    - 对话历史："上次我们讨论了..."
	//    - 任务进度："我正在学习..."
}
