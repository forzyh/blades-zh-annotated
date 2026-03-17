// Blades 示例：技能使用（skills）
//
// 本示例演示如何使用 Blades 的 Skills（技能）功能。
// Skills 是预定义的工具集合，可以通过文件系统加载，
// 让 Agent 能够使用外部工具和能力。
//
// 适用场景：
// - 加载预定义工具库
// - 从文件/嵌入资源加载技能
// - 模块化工具管理
// - 第三方技能集成
//
// 核心概念：
// 1. Skill（技能）：封装的工具定义
// 2. embed.FS：Go 的嵌入文件系统
// 3. NewFromEmbed：从嵌入文件加载技能
//
// 使用方法：
// go run main.go
// 注意：需要嵌入 skills 目录到可执行文件中
//       需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"embed"
	"log"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	"github.com/go-kratos/blades/skills"
)

//go:embed skills
// 嵌入 skills 目录到可执行文件
// 这允许在不依赖外部文件的情况下使用技能
// embed 是 Go 1.16+ 的标准库功能
var skillFS embed.FS

func main() {
	// 步骤 1: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		APIKey:  os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 从嵌入文件加载技能
	// skills.NewFromEmbed 从 embed.FS 加载所有技能定义
	// 技能目录应包含 YAML/JSON 格式的技能描述文件
	// 每个技能定义包含：名称、描述、参数 Schema 等
	weatherSkills, err := skills.NewFromEmbed(skillFS)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 Agent 并绑定技能
	// WithSkills 将技能注册为 Agent 可用的工具
	// Agent 会根据需要自动调用合适的技能
	agent, err := blades.NewAgent(
		"SkillUserAgent",
		blades.WithModel(model),
		blades.WithInstruction("Use skills when they are relevant to the task."),
		blades.WithSkills(weatherSkills...), // 展开技能切片
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	output, err := runner.Run(
		context.Background(),
		blades.UserMessage("What's the weather in San Francisco, and what's the humidity?"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 5: 输出结果
	log.Println(output.Text())

	// 预期输出：
	// Agent 会调用天气技能获取天气信息，然后回答用户
	//
	// 扩展提示：
	// 1. 技能文件格式（示例）：
	//    skills/weather.yaml:
	//    ---
	//    name: get_weather
	//    description: Get the current weather for a city
	//    parameters:
	//      type: object
	//      properties:
	//        city:
	//          type: string
	//          description: The city name
	//    handler: weather_handler
	//
	// 2. 从文件加载技能：
	//    skills, err := skills.NewFromDir("./skills")
	//
	// 3. 手动创建技能：
	//    skill := &skills.Skill{
	//        Name:        "my_skill",
	//        Description: "Does something useful",
	//        Handler:     myHandler,
	//    }
	//
	// 4. 技能 vs 工具：
	//    - 技能：从文件/嵌入加载的预定义工具
	//    - 工具：通过 tools.NewFunc/Tool 动态创建
	//    - 两者可以混合使用
	//
	// 5. 技能目录结构：
	//    skills/
	//      weather.yaml   # 天气技能
	//      search.yaml    # 搜索技能
	//      calc.yaml      # 计算技能
}
