// Blades 示例：HTTP 生成服务（server-generate）
//
// 本示例演示如何将 Blades Agent 封装为 HTTP 服务。
// 通过 HTTP API，可以让其他应用或用户通过网络调用 Agent 能力。
//
// 适用场景：
// - 构建 Agent API 服务
// - 集成到现有 Web 应用
// - 提供 SaaS 化的 AI 能力
// - 微服务架构中的 AI 服务
//
// 核心概念：
// 1. HTTP Handler：处理 HTTP 请求的标准 Go 接口
// 2. JSON 编码：将 Agent 响应编码为 JSON 格式
// 3. 同步响应：等待 Agent 完整响应后返回
//
// 使用方法：
// go run main.go
// 然后访问：http://localhost:8000/generate?input=你的问题
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
)

func main() {
	// 步骤 1: 创建 OpenAI 模型
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 2: 创建 Agent
	// 这个 Agent 充当历史导师，提供详细准确的历史信息
	agent, err := blades.NewAgent(
		"History Tutor",
		blades.WithModel(model),
		blades.WithInstruction("You are a knowledgeable history tutor. Provide detailed and accurate information on historical events."),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 HTTP 路由
	mux := http.NewServeMux()

	// 步骤 4: 注册 /generate 处理函数
	mux.HandleFunc("/generate", func(w http.ResponseWriter, r *http.Request) {
		// 解析表单参数
		r.ParseForm()

		// 创建 Runner
		runner := blades.NewRunner(agent)

		// 从查询参数获取用户输入
		input := blades.UserMessage(r.FormValue("input"))

		// 运行 Agent
		// 使用 r.Context() 绑定 HTTP 请求上下文
		// 如果客户端断开连接，Agent 执行会自动取消
		output, err := runner.Run(r.Context(), input)
		if err != nil {
			// 发生错误时返回 500 状态码
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// 设置响应头
		w.Header().Set("Content-Type", "application/json")

		// 将 Agent 响应编码为 JSON 并返回
		json.NewEncoder(w).Encode(output)
	})

	// 步骤 5: 启动 HTTP 服务器
	// 服务器监听 8000 端口
	log.Println("Server starting on http://localhost:8000")
	http.ListenAndServe(":8000", mux)
}

// API 使用示例：
// curl "http://localhost:8000/generate?input=Tell%20me%20about%20World%20War%20II"
//
// 响应示例：
// {
//   "role": "assistant",
//   "content": "World War II was a global conflict from 1939 to 1945...",
//   ...
// }
//
// 扩展提示：
// 1. 添加认证：
//    func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
//        return func(w http.ResponseWriter, r *http.Request) {
//            token := r.Header.Get("Authorization")
//            if !validateToken(token) {
//                http.Error(w, "Unauthorized", http.StatusUnauthorized)
//                return
//            }
//            next(w, r)
//        }
//    }
//
// 2. 限流保护：
//    使用 golang.org/x/time/rate 限制请求频率
//
// 3. 请求日志：
//    记录每个请求的输入、输出、耗时
//
// 4. 错误处理：
//    区分不同类型的错误（超时、API 错误、业务错误）
//    返回合适的 HTTP 状态码
//
// 5. 流式响应（Server-Sent Events）：
//    使用 SSE 实现流式输出
//    参见 server-streaming/main.go 示例
