// Blades 示例：图条件分支（graph-conditional）
//
// 本示例演示如何在 Graph 工作流中使用条件边（Conditional Edge）。
// 条件边允许根据运行时状态动态决定执行路径，实现分支逻辑。
//
// 适用场景：
// - 根据输入类型路由到不同处理逻辑
// - 实现 if-else 条件判断的工作流
// - 基于业务规则动态选择执行路径
// - 构建决策树或状态机
//
// 核心概念：
// 1. Conditional Edge（条件边）：带有判断条件的边，只有条件为真时才会执行
// 2. WithEdgeCondition：设置边的条件函数
// 3. 分支合并：多个分支可以汇聚到同一个后续节点
//
// 使用方法：
// go run main.go
package main

import (
	"context"
	"log"

	"github.com/go-kratos/blades/graph"
)

// logger 是一个辅助函数，创建用于日志记录的节点处理器
// 参数 name 是节点名称，用于标识日志来源
// 返回值 graph.Handler 是一个处理函数，记录节点执行日志并传递状态
func logger(name string) graph.Handler {
	return func(ctx context.Context, state graph.State) (graph.State, error) {
		log.Println("execute node:", name)
		return state, nil
	}
}

func main() {
	// 步骤 1: 创建图工作流
	// graph.New 创建一个空的工作流图
	g := graph.New()

	// 步骤 2: 定义节点
	// 使用 logger 辅助函数创建简单的日志记录节点
	g.AddNode("start", logger("start"))       // 起始节点
	g.AddNode("decision", logger("decision")) // 决策节点，判断执行路径
	g.AddNode("positive", logger("positive")) // 正数分支节点
	g.AddNode("negative", logger("negative")) // 负数分支节点
	g.AddNode("finish", logger("finish"))     // 结束节点

	// 步骤 3: 定义无条件的边
	// 这些边没有条件限制，总是会执行
	g.AddEdge("start", "decision")   // start -> decision
	g.AddEdge("positive", "finish")  // positive -> finish
	g.AddEdge("negative", "finish")  // negative -> finish

	// 步骤 4: 定义条件边
	// 从 decision 节点出发，根据状态中的值决定走哪条分支

	// 条件边 1: decision -> positive（当 n > 0 时）
	// WithEdgeCondition 接受一个函数，返回 true 表示执行这条边
	g.AddEdge("decision", "positive", graph.WithEdgeCondition(func(_ context.Context, state graph.State) bool {
		n, _ := state["n"].(int) // 从状态中获取 n 的值
		return n > 0             // 如果 n 是正数，走 positive 分支
	}))

	// 条件边 2: decision -> negative（当 n < 0 时）
	g.AddEdge("decision", "negative", graph.WithEdgeCondition(func(_ context.Context, state graph.State) bool {
		n, _ := state["n"].(int) // 从状态中获取 n 的值
		return n < 0             // 如果 n 是负数，走 negative 分支
	}))

	// 注意：如果 n == 0，两条条件边都不会执行，工作流会在此停止
	// 实际应用中可以添加一个默认分支处理 n == 0 的情况

	// 步骤 5: 设置入口和终点
	g.SetEntryPoint("start")   // 从 start 节点开始执行
	g.SetFinishPoint("finish") // 在 finish 节点结束

	// 步骤 6: 编译工作流
	// Compile 将图结构编译成可执行的 Executor
	executor, err := g.Compile()
	if err != nil {
		log.Fatalf("compile error: %v", err)
	}

	// 步骤 7: 执行工作流
	// 传入初始状态，n=100 是一个正数，应该走 positive 分支
	state, err := executor.Execute(context.Background(), graph.State{"n": 100})
	if err != nil {
		log.Fatalf("execution error: %v", err)
	}

	log.Printf("task final state: %+v", state)

	// 预期输出：
	// execute node: start
	// execute node: decision
	// execute node: positive
	// execute node: finish
	// task final state: map[n:100]
	//
	// 如果传入 n=-50，输出会是：
	// execute node: start
	// execute node: decision
	// execute node: negative
	// execute node: finish
}
