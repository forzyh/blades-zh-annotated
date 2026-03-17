// Blades 示例：DeepAgent 工作流（workflow-deepagent）
//
// 本示例演示如何使用 DeepAgent，一个强大的多 Agent 协作框架。
// DeepAgent 可以自动分解复杂任务，协调多个专业 Agent 完成工作，
// 适合处理需要多步骤推理和专业知识的复杂任务。
//
// 适用场景：
// - 复杂研究和分析任务
// - 多步骤问题求解
// - 需要专家协作的场景
// - 自主任务执行
//
// 核心概念：
// 1. DeepAgent：自主分解和分配任务的主控 Agent
// 2. Sub-Agents：专业领域的子 Agent
// 3. Task Decomposition：自动将大任务分解为小任务
// 4. Tool Integration：集成自定义工具
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL、OPENAI_BASE_URL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/flow"
	"github.com/go-kratos/blades/tools"
)

// ==================== 自定义工具定义 ====================

// SearchRequest 表示搜索请求
type SearchRequest struct {
	Query string `json:"query" jsonschema:"The search query to execute"`
}

// SearchResponse 表示搜索结果
type SearchResponse struct {
	Results []string `json:"results" jsonschema:"List of search results"`
}

// AnalysisRequest 表示数据分析请求
type AnalysisRequest struct {
	Data   string `json:"data" jsonschema:"Data to analyze"`
	Format string `json:"format" jsonschema:"Output format (json, markdown, text)"`
}

// AnalysisResponse 表示分析结果
type AnalysisResponse struct {
	Summary     string `json:"summary" jsonschema:"Analysis summary"`
	Insights    string `json:"insights" jsonschema:"Key insights"`
	ResultCount int    `json:"result_count" jsonschema:"Number of data points analyzed"`
}

// ==================== 工具处理函数 ====================

// searchHandler 模拟搜索工具
// 实际应用中可以集成 Google Search、搜索引擎 API 等
func searchHandler(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	log.Printf("Searching for: %s", req.Query)
	// 模拟搜索结果
	results := []string{
		fmt.Sprintf("Result 1 for '%s': NBA Championship records", req.Query),
		fmt.Sprintf("Result 2 for '%s': Career statistics and achievements", req.Query),
		fmt.Sprintf("Result 3 for '%s': Individual awards and honors", req.Query),
		fmt.Sprintf("Result 4 for '%s': Team performance and impact", req.Query),
	}
	return SearchResponse{Results: results}, nil
}

// analyzeDataHandler 模拟数据分析工具
// 实际应用中可以集成数据分析库、统计工具等
func analyzeDataHandler(ctx context.Context, req AnalysisRequest) (AnalysisResponse, error) {
	log.Printf("Analyzing data in format: %s", req.Format)
	return AnalysisResponse{
		Summary:     fmt.Sprintf("Analysis of %d characters of data", len(req.Data)),
		Insights:    "Key patterns identified: championship wins, MVP awards, scoring records",
		ResultCount: 3,
	}, nil
}

func main() {
	// 步骤 1: 创建 OpenAI 模型提供者
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		APIKey:  os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建自定义工具
	// 这些工具可以被 DeepAgent 和子 Agent 使用
	searchTool, err := tools.NewFunc(
		"search",
		"Search for information on a given topic",
		searchHandler,
	)
	if err != nil {
		log.Fatal(err)
	}

	analyzeTool, err := tools.NewFunc(
		"analyze_data",
		"Analyze data and generate insights in specified format",
		analyzeDataHandler,
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建专业子 Agent

	// ResearchAgent：研究专家，负责深入调研
	researchAgent, err := blades.NewAgent(
		"ResearchAgent",
		blades.WithDescription("Specialized agent for conducting in-depth research on specific topics"),
		blades.WithInstruction("You are a research specialist. Conduct thorough research using available tools and provide comprehensive, well-structured reports."),
		blades.WithModel(model),
		blades.WithTools(searchTool), // 研究 Agent 可以使用搜索工具
	)
	if err != nil {
		log.Fatal(err)
	}

	// DataAnalystAgent：数据分析师，负责数据分析
	dataAnalystAgent, err := blades.NewAgent(
		"DataAnalystAgent",
		blades.WithDescription("Specialized agent for data analysis and generating insights"),
		blades.WithInstruction("You are a data analyst. Analyze data thoroughly and provide actionable insights with clear visualizations when possible."),
		blades.WithModel(model),
		blades.WithTools(analyzeTool), // 分析 Agent 可以使用分析工具
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 配置 DeepAgent
	config := flow.DeepConfig{
		Name:          "ResearchCoordinator", // DeepAgent 名称
		Model:         model,                 // 使用的模型
		Description:   "An intelligent coordinator that decomposes complex research and analysis tasks into manageable subtasks",
		Instruction:   "You are an expert research coordinator. You excel at breaking down complex multi-step tasks into smaller, focused subtasks. Use the write_todos tool to plan complex tasks, and delegate specialized work to appropriate sub-agents using the task tool.",
		Tools:         []tools.Tool{searchTool, analyzeTool}, // 可用工具
		SubAgents:     []blades.Agent{researchAgent, dataAnalystAgent}, // 可用子 Agent
		MaxIterations: 20, // 最大迭代次数，防止无限循环
	}

	// 步骤 5: 创建 DeepAgent
	agent, err := flow.NewDeepAgent(config)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 6: 创建 Runner
	runner := blades.NewRunner(agent)

	// 步骤 7: 执行复杂任务
	// 这个任务需要：
	// 1. 分解为多个子任务
	// 2. 研究三位 NBA 球员的成就
	// 3. 比较他们的数据
	input := blades.UserMessage("I want to conduct research on the accomplishments of LeBron James, Michael Jordan, and Kobe Bryant, then compare them. Use the write_todos tool to plan this task and the task tool to delegate research to specialized agents.")

	output, err := runner.Run(context.Background(), input)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 8: 输出结果
	log.Println(output.Text())

	// 预期流程：
	// 1. DeepAgent 分析任务，创建待办清单：
	//    - 研究 LeBron James 的成就
	//    - 研究 Michael Jordan 的成就
	//    - 研究 Kobe Bryant 的成就
	//    - 比较三位球员的数据
	//
	// 2. DeepAgent 分配任务给子 Agent：
	//    - ResearchAgent 研究每位球员
	//    - DataAnalystAgent 分析比较数据
	//
	// 3. DeepAgent 整合结果，生成最终报告
	//
	// 扩展提示：
	// 1. 添加更多专业 Agent：
	//    - WriterAgent：负责撰写报告
	//    - FactCheckerAgent：负责事实核查
	//    - EditorAgent：负责编辑润色
	//
	// 2. 自定义工具：
	//    - 数据库查询工具
	//    - API 集成工具
	//    - 文件处理工具
	//
	// 3. 任务优先级：
	//    可以在指令中定义任务优先级规则
	//
	// 4. 结果验证：
	//    添加验证 Agent 检查结果质量
}
