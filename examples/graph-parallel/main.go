// Blades 示例：图并行执行（graph-parallel）
//
// 本示例演示如何在 Graph 工作流中实现并行执行。
// 并行执行可以同时运行多个独立的任务，提高整体执行效率。
//
// 适用场景：
// - 同时调用多个独立的 API 或微服务
// - 并行处理多个数据源
// - 同时执行多个独立的分析任务
// - 需要等待多个异步操作完成
//
// 核心概念：
// 1. 并行分支：从一个节点分出多条执行路径
// 2. 汇合节点（Join）：等待所有分支完成后继续执行
// 3. DAG（有向无环图）：Blades 的图工作流支持复杂的并行拓扑
//
// 使用方法：
// go run main.go
package main

import (
	"context"
	"log"
	"time"

	"github.com/go-kratos/blades/graph"
)

// logger 是一个辅助函数，创建用于日志记录的节点处理器
// 参数 name 是节点名称，用于标识日志来源
// 返回值 graph.Handler 是一个处理函数，模拟工作执行并记录日志
func logger(name string) graph.Handler {
	return func(ctx context.Context, state graph.State) (graph.State, error) {
		// 模拟 500ms 的工作时间，用于观察并行执行效果
		time.Sleep(time.Millisecond * 500)
		log.Println("execute node:", name)
		state[name] = "visited" // 在状态中标记该节点已访问
		return state, nil
	}
}

func main() {
	// 步骤 1: 创建图工作流
	g := graph.New()

	// 步骤 2: 定义节点
	// 构建一个并行执行的图结构：
	//           start
	//          /     \
	//     branch_a   branch_b
	//                 /    \
	//           branch_c  branch_d
	//                 \    /
	//                   join

	g.AddNode("start", logger("start"))       // 起始节点
	g.AddNode("branch_a", logger("branch_a")) // 分支 A
	g.AddNode("branch_b", logger("branch_b")) // 分支 B（会进一步分叉）
	g.AddNode("branch_c", logger("branch_c")) // 分支 C
	g.AddNode("branch_d", logger("branch_d")) // 分支 D
	g.AddNode("join", logger("join"))         // 汇合节点

	// 步骤 3: 定义边（执行顺序和并行关系）

	// start 节点分出两个分支：branch_a 和 branch_b
	// 这两个分支会并行执行
	g.AddEdge("start", "branch_a")
	g.AddEdge("start", "branch_b")

	// branch_b 进一步分出两个子分支：branch_c 和 branch_d
	// 注意：branch_b 完成后，branch_c 和 branch_d 会并行执行
	g.AddEdge("branch_b", "branch_c")
	g.AddEdge("branch_b", "branch_d")

	// 所有分支最终汇合到 join 节点
	// join 节点会等待所有上游分支完成后才执行
	g.AddEdge("branch_c", "join")
	g.AddEdge("branch_d", "join")
	g.AddEdge("branch_a", "join")

	// 步骤 4: 设置入口和终点
	g.SetEntryPoint("start") // 从 start 开始
	g.SetFinishPoint("join") // 在 join 结束

	// 步骤 5: 编译工作流
	executor, err := g.Compile()
	if err != nil {
		log.Fatal(err)
	}

	// 步骤 6: 执行工作流
	// Execute 会自动处理并行执行和汇合逻辑
	state, err := executor.Execute(context.Background(), graph.State{})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("task final state: %+v", state)

	// 预期执行顺序（可能有细微差异）：
	// 1. start 执行（0ms）
	// 2. branch_a 和 branch_b 并行启动（约 500ms 后完成）
	// 3. branch_b 完成后，branch_c 和 branch_d 并行启动（约 1000ms 后完成）
	// 4. 所有分支完成后，join 执行（约 1500ms 后完成）
	//
	// 如果是串行执行，总耗时会是 5 * 500ms = 2500ms
	// 并行执行后，总耗时约 1500ms，提升了效率
	//
	// 最终状态：map[branch_a:visited branch_b:visited branch_c:visited branch_d:visited join:visited start:visited]
}
