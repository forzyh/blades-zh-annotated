// Blades 示例：OpenTelemetry 中间件（middleware-otel）
//
// 本示例演示如何使用 OpenTelemetry 中间件对 Agent 执行进行链路追踪。
// OpenTelemetry 是一个开源的可观测性框架，支持追踪、指标和日志的统一处理。
//
// 适用场景：
// - 分布式链路追踪（查看请求在系统中的完整路径）
// - 性能分析和瓶颈定位
// - 生产环境监控和告警
// - 多服务架构中的请求追踪
//
// 核心概念：
// 1. TracerProvider：创建和管理 Trace 的组件
// 2. Span：追踪的基本单位，表示一个操作的开始和结束
// 3. Exporter：将追踪数据发送到后端（如 Jaeger、Zipkin、stdout）
// 4. Resource：描述被追踪服务的元数据
//
// 使用方法：
// go run main.go
// 注意：需要设置 OPENAI_MODEL 和 OPENAI_API_KEY 环境变量
//       追踪数据会输出到标准输出（stdout）
package main

import (
	"context"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/openai"
	middleware "github.com/go-kratos/blades/contrib/otel"
)

func main() {
	// 步骤 1: 创建 stdout Exporter
	// stdouttrace.New 创建一个将追踪数据输出到标准出口的 Exporter
	// 生产环境中可以使用其他 Exporter（如 OTLP、Jaeger、Zipkin）
	exporter, err := stdouttrace.New()
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 2: 创建 Resource
	// Resource 描述被追踪服务的元数据
	// semconv.ServiceNameKey 是 OpenTelemetry 标准约定的服务名属性
	resource, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("otel-demo"), // 设置服务名称
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 3: 创建 TracerProvider
	// TracerProvider 负责创建 Tracer，是 OpenTelemetry 追踪的核心组件
	// WithBatcher: 使用批量导出器，定期批量发送追踪数据
	//   - WithBatchTimeout(1*time.Millisecond): 设置批量超时为 1 毫秒（示例用，生产应更大）
	// WithResource: 将 Resource 与 TracerProvider 关联
	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(1*time.Millisecond)),
			sdktrace.WithResource(resource),
		),
	)

	// 步骤 4: 创建 OpenAI 模型提供者
	model := openai.NewModel(os.Getenv("OPENAI_MODEL"), openai.Config{
		APIKey: os.Getenv("OPENAI_API_KEY"),
	})

	// 步骤 5: 创建 Agent 并绑定 OpenTelemetry 中间件
	// middleware.Tracing() 创建一个追踪中间件，自动记录：
	// - Agent 执行开始和结束
	// - 输入消息和输出消息
	// - 执行耗时
	// - 错误信息（如果发生）
	agent, err := blades.NewAgent(
		"OpenTelemetry Agent",
		blades.WithMiddleware(
			middleware.Tracing(), // 绑定 OpenTelemetry 追踪中间件
		),
		blades.WithModel(model),
	)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 6: 创建用户输入消息
	input := blades.UserMessage("Write a diary about spring, within 100 words")

	// 步骤 7: 创建 Runner 并执行
	runner := blades.NewRunner(agent)
	msg, err := runner.Run(context.Background(), input)
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 8: 输出结果
	log.Println(msg.Text())

	// 步骤 9: 关闭 Exporter，刷新剩余的追踪数据
	// Shutdown 会导出所有未发送的 Span 并释放资源
	if err := exporter.Shutdown(context.Background()); err != nil {
		log.Fatal(err)
	}

	// 预期输出（追踪数据会先于日志输出）：
	// {
	//   "Name": "Agent/OpenTelemetry Agent",
	//   "SpanContext": {...},
	//   "Parent": ...,
	//   "Attributes": [
	//     {"Key": "input", "Value": "..."},
	//     {"Key": "output", "Value": "..."}
	//   ],
	//   ...
	// }
	// [日志] 关于春天的日记内容...
	//
	// 扩展提示：
	// 1. 使用 OTLP Exporter 发送到后端：
	//    import "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	//    exporter, _ := otlptrace.New(ctx, otlptrace.WithInsecure())
	//
	// 2. 集成 Jaeger:
	//    import "go.opentelemetry.io/otel/exporters/jaeger"
	//    exporter, _ := jaeger.New(jaeger.WithCollectorEndpoint("http://localhost:14268/api/traces"))
	//
	// 3. 自定义 Span 属性：
	//    可以在中间件中添加自定义属性（如用户 ID、请求 ID 等）
}
