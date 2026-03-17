// Package stream 提供了基于 Go 1.23+ iter.Seq2 的流处理工具。
//
// 核心概念：
// iter.Seq2 是 Go 1.23 引入的迭代器类型，表示一个产生值序列的函数。
// 类型签名：iter.Seq2[T, error] = func(yield func(T, error) bool)
//
// 流处理特点：
// - 惰性求值：值在需要时才生成，节省内存
// - 组合式：可以链式组合多个操作（Filter、Map 等）
// - 并发安全：支持并发合并多个流
//
// 本包提供的工具：
// - Just: 将值列表转为流
// - Error: 创建只产生错误的流
// - Filter: 过滤流中的元素
// - Observe: 观察流中的元素（用于副作用）
// - Map: 转换流中的元素
// - Merge: 合并多个流为一个
//
// 使用场景：
// - Agent 流式响应的处理
// - 异步任务结果的处理
// - 并发任务的合并与协调
package stream

import (
	"iter"
	"sync"
)

// Just 将提供的值转换为 iter.Seq2 流。
//
// 参数说明：
// - values: 可变参数，任意数量的值
//
// 返回值：
// iter.Seq2[T, error] - 按顺序产生提供的值的流
//
// 处理流程：
// 1. 遍历所有提供的值
// 2. 对每个值调用 yield(v, nil) 产生值和 nil 错误
// 3. 如果 yield 返回 false（消费者停止），则提前返回
//
// 使用示例：
//
//	// 创建一个包含 1, 2, 3 的流
//	stream := stream.Just(1, 2, 3)
//	for v, err := range stream {
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    fmt.Println(v) // 输出：1, 2, 3
//	}
//
// 典型用途：
// - 将现有数据集合转为流进行处理
// - 作为流的起点，然后进行 Filter、Map 等操作
func Just[T any](values ...T) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for _, v := range values {
			if !yield(v, nil) {
				return
			}
		}
	}
}

// Error 创建一个只产生指定错误的流。
//
// 参数说明：
// - err: 要产生的错误
//
// 返回值：
// iter.Seq2[T, error] - 只产生一个错误值的流
//
// 处理流程：
// 1. 调用 yield 一次，传递零值和错误
// 2. 流结束
//
// 使用示例：
//
//	// 创建一个错误流
//	errStream := stream.Error[any](errors.New("something went wrong"))
//	for v, err := range errStream {
//	    if err != nil {
//	        log.Fatal(err) // 输出：something went wrong
//	    }
//	}
//
// 典型用途：
// - 在流处理链中表示错误状态
// - 作为错误处理路径的返回值
// - 与 Merge 配合，实现错误传播
func Error[T any](err error) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		// 产生零值和错误
		yield(*new(T), err)
	}
}

// Filter 返回一个新流，只包含满足谓词函数的值。
//
// 参数说明：
// - stream: 输入流
// - predicate: 谓词函数，返回 true 表示保留该值
//
// 返回值：
// iter.Seq2[T, error] - 过滤后的流
//
// 处理流程：
// 1. 遍历输入流
// 2. 如果遇到错误，直接传递错误并继续
// 3. 如果值满足谓词条件，产生该值
// 4. 如果不满足，跳过该值继续下一个
//
// 使用示例：
//
//	// 过滤出偶数
//	numbers := stream.Just(1, 2, 3, 4, 5, 6)
//	evens := stream.Filter(numbers, func(n int) bool {
//	    return n%2 == 0
//	})
//	// 输出：2, 4, 6
//	for v, _ := range evens {
//	    fmt.Println(v)
//	}
//
// 典型用途：
// - 过滤掉不需要的值
// - 根据条件筛选数据
// - 实现流中的数据筛选逻辑
func Filter[T any](stream iter.Seq2[T, error], predicate func(T) bool) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		stream(func(v T, err error) bool {
			// 错误直接传递，不过滤
			if err != nil {
				return yield(*new(T), err)
			}
			// 只产生满足条件的值
			if predicate(v) {
				return yield(v, nil)
			}
			return true // 继续处理下一个值
		})
	}
}

// Observe 返回一个新流，对每个值调用观察者函数。
//
// 参数说明：
// - stream: 输入流
// - observer: 观察者函数，对每个值执行，可以返回错误停止观察
//
// 返回值：
// iter.Seq2[T, error] - 观察后的流（与原流相同，但有副作用）
//
// 处理流程：
// 1. 遍历输入流
// 2. 对每个值和错误调用观察者函数
// 3. 如果观察者返回错误，传递该错误
// 4. 否则继续传递原流的值和错误
//
// 与 Map 的区别：
// - Observe 用于副作用（如日志、监控），不改变值
// - Map 用于转换值，产生新类型的值
//
// 使用示例：
//
//	// 记录流中的每个值
//	numbers := stream.Just(1, 2, 3, 4, 5)
//	observed := stream.Observe(numbers, func(n int, err error) error {
//	    if err != nil {
//	        log.Printf("Error: %v", err)
//	        return err
//	    }
//	    log.Printf("Processing: %d", n)
//	    return nil
//	})
//
// 典型用途：
// - 日志记录
// - 指标收集
// - 调试和监控
// - 副作用操作（如发送通知）
func Observe[T any](stream iter.Seq2[T, error], observer func(T, error) error) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		stream(func(v T, err error) bool {
			// 先调用观察者函数
			if err := observer(v, err); err != nil {
				return yield(v, err)
			}
			// 然后传递原值
			return yield(v, err)
		})
	}
}

// Map 返回一个新流，将 mapper 函数应用于每个值。
//
// 参数说明：
// - stream: 输入流
// - mapper: 转换函数，将 T 类型转换为 R 类型
//
// 返回值：
// iter.Seq2[R, error] - 转换后的流
//
// 处理流程：
// 1. 遍历输入流
// 2. 如果遇到错误，传递错误并转换类型
// 3. 否则调用 mapper 转换值
// 4. 如果 mapper 返回错误，传递错误
// 5. 否则产生转换后的值
//
// 使用示例：
//
//	// 将整数转换为字符串
//	numbers := stream.Just(1, 2, 3)
//	strings := stream.Map(numbers, func(n int) (string, error) {
//	    return fmt.Sprintf("number-%d", n), nil
//	})
//	// 输出："number-1", "number-2", "number-3"
//	for v, _ := range strings {
//	    fmt.Println(v)
//	}
//
// 典型用途：
// - 类型转换
// - 数据格式变换
// - 提取字段
// - 计算派生值
func Map[T, R any](stream iter.Seq2[T, error], mapper func(T) (R, error)) iter.Seq2[R, error] {
	return func(yield func(R, error) bool) {
		stream(func(v T, err error) bool {
			// 错误直接传递，但要转换为正确的类型
			if err != nil {
				return yield(*new(R), err)
			}
			// 调用 mapper 转换值
			mapped, err := mapper(v)
			if err != nil {
				return yield(*new(R), err)
			}
			return yield(mapped, nil)
		})
	}
}

// Merge 合并多个输入流为一个输出流。
//
// 参数说明：
// - streams: 可变参数，任意数量的输入流
//
// 返回值：
// iter.Seq2[T, error] - 合并后的流
//
// 处理流程：
// 1. 为每个输入流启动一个 goroutine
// 2. 每个 goroutine 遍历其流并调用 yield
// 3. 使用互斥锁保护 yield 的并发调用
// 4. 使用 WaitGroup 等待所有 goroutine 完成
//
// 并发模型：
// - 每个输入流在独立的 goroutine 中处理
// - 使用 mutex 确保 yield 的线程安全
// - 所有流并发执行，提高吞吐量
//
// 使用示例：
//
//	// 合并三个流
//	stream1 := stream.Just(1, 2, 3)
//	stream2 := stream.Just(4, 5, 6)
//	stream3 := stream.Just(7, 8, 9)
//	merged := stream.Merge(stream1, stream2, stream3)
//	// 输出：1-9（顺序不确定，取决于 goroutine 调度）
//	for v, _ := range merged {
//	    fmt.Println(v)
//	}
//
// 典型用途：
// - 合并多个并发任务的结果
// - 并行处理多个数据源
// - 实现扇入（fan-in）模式
//
// 注意：
// - 输出顺序不确定，取决于 goroutine 调度
// - 如果需要有序输出，需要额外的排序逻辑
func Merge[T any](streams ...iter.Seq2[T, error]) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var (
			mu sync.Mutex
			wg sync.WaitGroup
		)
		// 等待所有流完成
		wg.Add(len(streams))
		for _, stream := range streams {
			// 每个流在独立 goroutine 中处理
			go func(next iter.Seq2[T, error]) {
				defer wg.Done()
				next(func(v T, err error) bool {
					// 使用互斥锁保护并发调用
					mu.Lock()
					defer mu.Unlock()
					return yield(v, err)
				})
			}(stream)
		}
		wg.Wait()
	}
}
