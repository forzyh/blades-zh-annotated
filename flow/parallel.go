package flow

import (
	"context"

	"github.com/go-kratos/blades"
	"golang.org/x/sync/errgroup"
)

// ParallelConfig 是并行代理（ParallelAgent）的配置结构体
// 用于定义并行执行模式所需的参数
//
// 字段说明：
// - Name: 代理的名称，用于标识和日志记录
// - Description: 代理的描述信息，说明该代理的用途和功能
// - SubAgents: 子代理列表，这些子代理将被并行执行
//
// 使用场景：当需要同时执行多个独立任务时使用并行配置
// 例如：同时收集多个数据源的信息、并行处理多个独立请求等
type ParallelConfig struct {
	Name        string
	Description string
	SubAgents   []blades.Agent
}

// ParallelAgent 是并行代理结构体
// 它负责将多个子代理并发执行，提高任务处理效率
//
// 核心特性：
// 1. 所有子代理同时启动，互不等待
// 2. 任意子代理产生的输出会立即被传递出去
// 3. 支持错误传播，单个子代理失败会影响整体流程
//
// 数据结构：内部持有 ParallelConfig 配置，通过配置驱动行为
type ParallelAgent struct {
	config ParallelConfig
}

// NewParallelAgent 是并行代理的构造函数
//
// 参数：config - ParallelConfig 配置结构体，包含代理名称、描述和子代理列表
// 返回：blades.Agent 接口，实现标准化的代理运行协议
//
// 使用示例：
// parallelAgent := flow.NewParallelAgent(flow.ParallelConfig{
//     Name: "数据收集器",
//     Description: "并行从多个数据源收集信息",
//     SubAgents: []blades.Agent{agent1, agent2, agent3},
// })
func NewParallelAgent(config ParallelConfig) blades.Agent {
	return &ParallelAgent{config: config}
}

// Name 返回代理的名称
// 实现 blades.Agent 接口的方法
func (p *ParallelAgent) Name() string {
	return p.config.Name
}

// Description 返回代理的描述信息
// 实现 blades.Agent 接口的方法
func (p *ParallelAgent) Description() string {
	return p.config.Description
}

// Run 是并行代理的核心执行方法
// 它并发运行所有子代理，并将它们的输出流式传递给调用者
//
// 参数：
// - ctx: 上下文，用于控制生命周期和取消信号
// - invocation: 调用参数，包含输入消息和执行上下文
//
// 返回：Generator[*blades.Message, error] - 一个生成器函数
// 生成器会逐步产出每个子代理产生的 Message 或 error
//
// 执行流程详解：
// 1. 创建一个带缓冲的 channel 用于收集所有子代理的结果
// 2. 使用 errgroup 管理所有子代理的并发执行
// 3. 每个子代理在独立的 goroutine 中运行
// 4. 子代理的输出通过 channel 传递回主循环
// 5. 主循环将结果逐个 yield 给调用者
//
// 并发控制机制：
// - 使用 errgroup.WithContext 创建可取消的上下文组
// - 当任意子代理返回错误时，通过 cancel() 取消其他所有子代理
// - 使用 defer 确保 cancel 函数一定会被调用，防止资源泄漏
//
// 错误处理策略：
// - 子代理产生的错误会被封装在 result 结构中传递
// - 错误会导致 channel 关闭和其他子代理被取消
// - 调用者可以通过检查 error 判断执行状态
//
// 使用示例：
// for message, err := range parallelAgent.Run(ctx, invocation) {
//     if err != nil {
//         log.Printf("执行错误：%v", err)
//         continue
//     }
//     fmt.Println("收到消息:", message.Text())
// }
func (p *ParallelAgent) Run(ctx context.Context, invocation *blades.Invocation) blades.Generator[*blades.Message, error] {
	// 返回一个生成器函数，这是 Go 中实现协程迭代器的模式
	// yield 函数由调用者提供，用于接收每个输出结果
	return func(yield func(*blades.Message, error) bool) {
		// result 是内部结构体，用于在 channel 中传递子代理的执行结果
		// 包含消息和错误两个字段，二者互斥（有消息则无错误，反之亦然）
		type result struct {
			message *blades.Message
			err     error
		}
		// 创建带缓冲的 channel 用于收集结果
		// 缓冲大小为子代理数量 * 8，减少 goroutine 阻塞等待
		// 乘以 8 是因为每个子代理可能产生多个消息，需要足够缓冲
		ch := make(chan result, len(p.config.SubAgents)*8)
		// 创建可取消的上下文，用于在需要时终止所有子代理
		ctx, cancel := context.WithCancel(ctx)
		defer cancel() // 确保函数退出时释放上下文资源
		// 创建 errgroup，它封装了 goroutine 的创建和错误等待逻辑
		eg, ctx := errgroup.WithContext(ctx)
		// 遍历所有子代理，为每个子代理启动一个独立的 goroutine
		for _, agent := range p.config.SubAgents {
			eg.Go(func() error {
				// 在每个 goroutine 中运行子代理
				// 注意：invocation.Clone() 确保每个子代理有独立的调用副本
				// 避免多个 goroutine 共享同一状态导致的数据竞争
				for message, err := range agent.Run(ctx, invocation.Clone()) {
					if err != nil {
						// 子代理执行出错时，将错误发送到 channel
						// 然后返回错误，errgroup 会捕获并处理
						ch <- result{message: nil, err: err}
						return err
					}
					// 正常输出时，将消息发送到 channel
					ch <- result{message: message, err: nil}
				}
				return nil
			})
		}
		// 启动一个后台 goroutine 等待所有子代理完成
		// 完成后关闭 channel，通知主循环没有更多数据
		go func() {
			eg.Wait()    // 等待所有 goroutine 完成
			close(ch)    // 关闭 channel，使主循环的 range 结束
		}()
		// 主循环：从 channel 读取结果并 yield 给调用者
		for res := range ch {
			// 调用 yield 将结果传递给外部迭代器
			// 如果 yield 返回 false，说明调用者希望提前终止
			if !yield(res.message, res.err) {
				cancel() // 调用者终止时，取消所有子代理
				break
			}
		}
	}
}
