// Blades 示例：结构化输出（structured）
//
// 本示例演示如何使用 Blades 的结构化输出功能。
// 通过定义输出 Schema，可以强制 LLM 返回特定格式的 JSON 数据，
// 便于程序解析和后续处理。
//
// 适用场景：
// - 数据提取和结构化
// - API 响应格式化
// - 数据库记录生成
// - 需要机器可读的输出
//
// 核心概念：
// 1. OutputSchema：使用 JSON Schema 定义输出格式
// 2. jsonschema.For：从 Go 类型生成 JSON Schema
// 3. 结构化解析：LLM 输出自动匹配 Schema
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
	"github.com/google/jsonschema-go/jsonschema"
)

// ActorsFilms 定义输出数据结构
// 这个结构会被转换成 JSON Schema，指导 LLM 生成格式化输出
type ActorsFilms struct {
	// Actor 字段：演员姓名
	// jsonschema 标签描述字段的含义
	Actor  string   `json:"actor" jsonschema:"name of the actor"`
	// Movies 字段：电影列表
	// jsonschema 标签描述字段的含义
	Movies []string `json:"movies" jsonschema:"list of movies"`
}

func main() {
	// 步骤 1: 从 Go 类型生成 JSON Schema
	// jsonschema.For 自动分析结构体并生成对应的 JSON Schema
	// 这个 Schema 会发送给 LLM，指导它生成格式化输出
	schema, err := jsonschema.For[ActorsFilms](nil)
	if err != nil {
		panic(err)
	}

	// 步骤 2: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 3: 创建 Agent 并设置输出 Schema
	// WithOutputSchema 告诉 Agent 必须返回符合 Schema 的 JSON 输出
	// LLM 会被约束生成符合 Schema 结构的数据
	agent, err := blades.NewAgent(
		"filmography", // Agent 名称
		blades.WithModel(model),
		blades.WithOutputSchema(schema), // 设置输出 Schema
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 4: 创建用户输入
	// 请求生成 Tom Hanks 的电影作品列表
	input := blades.UserMessage("Generate the filmography of 5 movies for Tom Hanks")

	// 步骤 5: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	actorsFilms, err := runner.Run(context.Background(), input)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 6: 输出结果
	// actorsFilms 包含符合 Schema 的结构化数据
	log.Println(actorsFilms)

	// 预期输出示例：
	// {
	//   "actor": "Tom Hanks",
	//   "movies": [
	//     "Forrest Gump",
	//     "Cast Away",
	//     "Saving Private Ryan",
	//     "The Green Mile",
	//     "Apollo 13"
	//   ]
	// }
	//
	// 扩展提示：
	// 1. 复杂 Schema：
	//    type Movie struct {
	//        Title string `json:"title"`
	//        Year  int    `json:"year"`
	//        Genre string `json:"genre"`
	//    }
	//    type Filmography struct {
	//        Actor  string  `json:"actor"`
	//        Movies []Movie `json:"movies"`
	//    }
	//
	// 2. 嵌套结构：
	//    type Person struct {
	//        Name    string   `json:"name"`
	//        Address Address  `json:"address"`
	//    }
	//    type Address struct {
	//        Street string `json:"street"`
	//        City   string `json:"city"`
	//    }
	//
	// 3. 可选字段：
	//    使用指针类型表示可选字段
	//    type Person struct {
	//        Name  string  `json:"name"`
	//        Email *string `json:"email"` // 可选
	//    }
	//
	// 4. 字段验证：
	//    可以在结构体标签中添加验证规则
	//    或使用专门的验证库（如 go-playground/validator）
	//
	// 5. 解析 JSON 响应：
	//    var result ActorsFilms
	//    json.Unmarshal([]byte(actorsFilms.Text()), &result)
}
