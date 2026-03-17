// Blades 示例：图检查点（graph-checkpoint）
//
// 本示例演示如何在 Graph 工作流中使用检查点（Checkpoint）机制。
// 检查点允许在长时间运行的工作流中保存中间状态，并在需要时恢复执行。
// 这对于需要人工审批、长时间等待或可能失败的工作流非常有用。
//
// 适用场景：
// - 需要人工审批的工作流（如请假审批、内容审核）
// - 长时间运行的任务，需要断点续传
// - 容易失败的任务，需要从中断点恢复
// - 多阶段工作流，需要在各阶段间持久化状态
//
// 核心概念：
// 1. Checkpoint（检查点）：工作流执行过程中的快照
// 2. Checkpointer（检查点存储）：负责保存和加载检查点的组件
// 3. Execute/Resume：执行工作流和从检查点恢复
//
// 使用方法：
// go run main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/go-kratos/blades/graph"
)

// ErrProcessApproval 是一个自定义错误，表示需要审批才能继续
// 这种模式允许工作流在需要人工干预时暂停
var ErrProcessApproval = errors.New("approval is required")

// checkpointStore 是一个简单的内存检查点存储器
// 生产环境中应该使用数据库或持久化存储
type checkpointStore struct {
	checkpoints map[string]*graph.Checkpoint // 使用 map 存储检查点，key 是检查点 ID
}

// NewCheckpointStore 创建一个新的检查点存储器
func NewCheckpointStore() *checkpointStore {
	return &checkpointStore{
		checkpoints: make(map[string]*graph.Checkpoint),
	}
}

// Save 保存检查点
// ctx 是上下文，checkpoint 是要保存的检查点
// Clone() 确保保存的是副本，避免后续修改影响已保存的数据
func (s *checkpointStore) Save(ctx context.Context, checkpoint *graph.Checkpoint) error {
	s.checkpoints[checkpoint.ID] = checkpoint.Clone()
	return nil
}

// Resume 恢复检查点
// checkpointID 是要恢复的检查点 ID
// 返回检查点的副本，确保外部修改不会影响存储的数据
func (s *checkpointStore) Resume(ctx context.Context, checkpointID string) (*graph.Checkpoint, error) {
	if cp, ok := s.checkpoints[checkpointID]; ok {
		return cp.Clone(), nil
	}
	return nil, fmt.Errorf("checkpoint %s not found", checkpointID)
}

func main() {
	// 步骤 1: 创建图工作流
	// graph.New 创建一个空的工作流图
	// WithMiddleware(graph.Retry(3)) 添加重试中间件，失败时最多重试 3 次
	g := graph.New(graph.WithMiddleware(graph.Retry(3)))

	// 步骤 2: 定义节点（工作流的处理步骤）
	// 每个节点是一个 Handler 函数，接收上下文和状态，返回新状态和错误

	// "start" 节点：工作流入口，初始化状态
	g.AddNode("start", func(ctx context.Context, state graph.State) (graph.State, error) {
		state["start"] = true // 在状态中标记 start 节点已执行
		return state, nil
	})

	// "process" 节点：处理逻辑，需要审批才能继续
	g.AddNode("process", func(ctx context.Context, state graph.State) (graph.State, error) {
		state["process"] = true // 标记 process 节点已执行
		// 从状态中读取审批结果
		approved, _ := state["approved"].(bool)
		if !approved {
			// 如果未审批，返回错误，工作流会暂停并保存检查点
			return nil, ErrProcessApproval
		}
		return state, nil
	})

	// "finish" 节点：工作流终点
	g.AddNode("finish", func(ctx context.Context, state graph.State) (graph.State, error) {
		state["finish"] = true // 标记 finish 节点已执行
		return state, nil
	})

	// 步骤 3: 定义边（节点间的执行顺序）
	// AddEdge 定义从一个节点到另一个节点的转换
	g.AddEdge("start", "process")    // start -> process
	g.AddEdge("process", "finish")   // process -> finish
	g.SetEntryPoint("start")         // 设置入口节点
	g.SetFinishPoint("finish")       // 设置结束节点

	// 步骤 4: 编译工作流
	// Compile 将图结构编译成可执行的 Executor
	// WithCheckpointer 绑定检查点存储器，启用持久化功能
	checkpointID := "checkpoint_1" // 检查点 ID，用于标识和恢复
	checkpointer := NewCheckpointStore()
	executor, err := g.Compile(graph.WithCheckpointer(checkpointer))
	if err != nil {
		log.Fatalf("compile error: %v", err)
	}

	// 步骤 5: 首次执行工作流
	// Execute 从入口节点开始执行
	// WithCheckpointID 指定检查点 ID，执行状态会被保存
	state, err := executor.Execute(context.Background(), graph.State{}, graph.WithCheckpointID(checkpointID))
	if err != nil {
		// 如果是审批错误，这是预期的行为，工作流会暂停
		if !errors.Is(err, ErrProcessApproval) {
			log.Fatalf("execute error: %v", err)
		}
		log.Printf("execution paused waiting for approval: %v", err)
	}
	log.Println("task paused, waiting for approval...", state)

	// 步骤 6: 模拟审批通过
	// 在实际应用中，这里可以是 HTTP 回调、消息队列等异步机制
	resumeState := graph.State{
		"approved": true, // 设置审批通过
	}

	// 步骤 7: 从检查点恢复执行
	// Resume 从上次保存的检查点继续执行
	// resumeState 包含审批结果，会被合并到之前的状态中
	finalState, err := executor.Resume(context.Background(), resumeState, graph.WithCheckpointID(checkpointID))
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("resumed from checkpoint %s, final state: %+v", checkpointID, finalState)

	// 预期输出：
	// 1. 首次执行：task paused, waiting for approval... map[start:true process:true]
	// 2. 恢复执行：resumed from checkpoint checkpoint_1, final state: map[approved:true finish:true process:true start:true]
}
