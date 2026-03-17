// Blades 示例：图重试机制（graph-retry）
//
// 本示例演示如何在 Graph 工作流中使用重试中间件。
// 重试机制可以自动处理临时性故障，提高工作流的可靠性。
//
// 适用场景：
// - 调用可能失败的外部 API 或服务
// - 处理临时性网络故障
// - 应对资源竞争或限流
// - 处理偶发性错误的任务
//
// 核心概念：
// 1. Retry Middleware（重试中间件）：自动捕获错误并重试失败的操作
// 2. 瞬态故障（Transient Failure）：暂时性的错误，重试后可能成功
// 3. 重试次数：可以配置最大重试次数，避免无限重试
//
// 使用方法：
// go run main.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-kratos/blades/graph"
)

// flakyProcessor 创建一个模拟不稳定处理的节点处理器
// 参数 maxFailures 指定在前多少次尝试时会失败
// 返回值 graph.Handler 是一个带故障模拟的处理函数
//
// 这个处理器模拟了现实中的瞬态故障场景：
// - 前 N 次调用会失败（模拟网络抖动、服务暂时不可用等）
// - 第 N+1 次调用成功（模拟服务恢复）
func flakyProcessor(maxFailures int) graph.Handler {
	attempts := 0 // 记录尝试次数，使用闭包保持状态
	return func(ctx context.Context, state graph.State) (graph.State, error) {
		attempts++
		log.Printf("[process] attempt %d", attempts)

		// 模拟瞬态故障：前 maxFailures 次尝试会失败
		if attempts <= maxFailures {
			return nil, fmt.Errorf("transient failure %d/%d", attempts, maxFailures)
		}

		// 故障恢复后，在状态中记录处理结果
		state["attempts"] = attempts
		state["processed_at"] = time.Now().Format(time.RFC3339Nano)
		return state, nil
	}
}

func main() {
	// 步骤 1: 创建图工作流，配置重试中间件
	// graph.New 创建一个空的工作流图
	// WithMiddleware(graph.Retry(3)) 添加重试中间件，最多重试 3 次
	// 这意味着失败的操作会自动重试，最多执行 4 次（1 次初始 + 3 次重试）
	g := graph.New(graph.WithMiddleware(graph.Retry(3)))

	// 步骤 2: 定义节点

	// "start" 节点：准备工作项
	g.AddNode("start", func(ctx context.Context, state graph.State) (graph.State, error) {
		log.Println("[start] preparing work item")
		state["payload"] = "retry-demo" // 在状态中存储示例数据
		return state, nil
	})

	// "process" 节点：使用 flakyProcessor 模拟可能失败的处理
	// flakyProcessor(2) 表示前 2 次尝试会失败，第 3 次成功
	// 重试中间件会自动捕获错误并重试，直到成功或达到最大重试次数
	g.AddNode("process", flakyProcessor(2))

	// "finish" 节点：处理完成后的清理工作
	g.AddNode("finish", func(ctx context.Context, state graph.State) (graph.State, error) {
		// 从状态中获取处理结果并记录日志
		attempts, _ := state["attempts"]
		processedAt, _ := state["processed_at"]
		log.Printf("[finish] workflow complete. attempts=%v processed_at=%v", attempts, processedAt)
		return state, nil
	})

	// 步骤 3: 定义边（执行顺序）
	g.AddEdge("start", "process")  // start -> process
	g.AddEdge("process", "finish") // process -> finish

	// 步骤 4: 设置入口和终点
	g.SetEntryPoint("start")   // 从 start 开始
	g.SetFinishPoint("finish") // 在 finish 结束

	// 步骤 5: 编译工作流
	executor, err := g.Compile()
	if err != nil {
		log.Fatalf("compile error: %v", err)
	}

	// 步骤 6: 执行工作流
	// 尽管 process 节点前 2 次会失败，但重试中间件会自动处理
	// 最终应该成功完成，attempts 应该为 3
	state, err := executor.Execute(context.Background(), graph.State{})
	if err != nil {
		log.Fatalf("execution error: %v", err)
	}

	log.Printf("task final state: %+v", state)

	// 预期输出：
	// [start] preparing work item
	// [process] attempt 1
	// [process] attempt 2
	// [process] attempt 3
	// [finish] workflow complete. attempts=3 processed_at=2024-...
	// task final state: map[attempts:3 payload:retry-demo processed_at:...]
	//
	// 注意：如果没有重试中间件，工作流会在第 1 次尝试失败时就终止
	// 有了重试中间件，工作流能够自动恢复并最终成功
}
