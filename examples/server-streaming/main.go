// Blades 示例：HTTP 流式服务（server-streaming）
//
// 本示例演示如何将 Blades Agent 封装为 HTTP 流式服务。
// 流式输出允许客户端实时接收 Agent 的生成内容，
// 提供更好的用户体验，尤其是长文本生成场景。
//
// 适用场景：
// - 聊天机器人实时回复
// - 长文本生成的进度展示
// - 降低首字节延迟（TTFB）
// - 实时翻译/转写服务
//
// 核心概念：
// 1. Server-Sent Events (SSE)：服务器推送技术
// 2. Stream API：Blades 的流式执行接口
// 3. HTTP Flusher：立即刷新响应缓冲区
//
// 使用方法：
// go run main.go
// 然后访问：http://localhost:8000/streaming?input=你的问题
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
package main

import (
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
	agent, err := blades.NewAgent(
		"Server Agent",
		blades.WithModel(model),
		blades.WithInstruction("You are a helpful assistant that provides detailed and accurate information."),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 HTTP 路由
	mux := http.NewServeMux()

	// 步骤 4: 注册 /streaming 处理函数
	mux.HandleFunc("/streaming", func(w http.ResponseWriter, r *http.Request) {
		// 解析表单参数
		r.ParseForm()

		// 创建 Runner
		runner := blades.NewRunner(agent)

		// 从查询参数获取用户输入
		input := blades.UserMessage(r.FormValue("input"))

		// 使用 RunStream 获取流式生成器
		// RunStream 返回一个 Generator，可以遍历每次输出
		for output, err := range runner.RunStream(r.Context(), input) {
			if err != nil {
				// 发生错误时返回 500 状态码
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// 设置 SSE 响应头
			// text/event-stream 告诉浏览器这是 SSE 流
			w.Header().Set("Content-Type", "text/event-stream")

			// 将响应编码为 JSON 并写入
			if err := json.NewEncoder(w).Encode(output); err != nil {
				// 写入失败（通常意味着客户端断开）时返回
				return
			}

			// 刷新响应缓冲区，立即发送数据给客户端
			// 这对于流式输出至关重要
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	})

	// 步骤 5: 启动 HTTP 服务器
	log.Println("Streaming server starting on http://localhost:8000")
	http.ListenAndServe(":8000", mux)
}

// API 使用示例：
// curl "http://localhost:8000/streaming?input=Write%20a%20story%20about%20a%20cat"
//
// 响应流示例（每行是一个 JSON 对象）：
// {"role":"assistant","content":"Once","status":"streaming"}
// {"role":"assistant","content":" upon","status":"streaming"}
// {"role":"assistant","content":" a time","status":"streaming"}
// ...
// {"role":"assistant","content":"...the end.","status":"completed"}
//
// 前端集成示例（JavaScript）：
// const eventSource = new EventSource(
//   'http://localhost:8000/streaming?input=Hello'
// );
// eventSource.onmessage = (event) => {
//   const data = JSON.parse(event.data);
//   console.log(data.content);
// };
//
// 扩展提示：
// 1. CORS 支持（跨域请求）：
//    w.Header().Set("Access-Control-Allow-Origin", "*")
//
// 2. 心跳保持连接：
//    定期发送注释行防止连接超时
//    fmt.Fprintf(w, ": heartbeat\n\n")
//    f.Flush()
//
// 3. 流式错误处理：
//    错误时发送特殊标记
//    {"error": "message"}
//
// 4. 认证和限流：
//    与 server-generate 类似，添加中间件保护
//
// 5. 上下文超时：
//    设置合理的超时时间
//    ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
//    defer cancel()
//
// 6. 性能优化：
//    - 使用 bufio.Writer 缓冲写入
//    - 批量发送多个 token
//    - 启用 HTTP/2 提高并发
