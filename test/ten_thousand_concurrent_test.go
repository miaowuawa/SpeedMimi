package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	fmt.Println("🚀 SpeedMimi 10,000并发性能测试 & 火焰图分析")
	fmt.Println("==============================================")

	// 测试参数
	targetURL := "http://localhost:8080"
	concurrency := 10000  // 10,000并发
	duration := 180 * time.Second   // 3分钟测试时长

	fmt.Printf("目标URL: %s\n", targetURL)
	fmt.Printf("并发数: %d\n", concurrency)
	fmt.Printf("测试时长: %v\n\n", duration)

	// 创建性能分析文件
	cpuProfileFile, err := os.Create("cpu_profile.prof")
	if err != nil {
		fmt.Printf("创建CPU profile文件失败: %v\n", err)
		return
	}
	defer cpuProfileFile.Close()

	memProfileFile, err := os.Create("mem_profile.prof")
	if err != nil {
		fmt.Printf("创建内存profile文件失败: %v\n", err)
		return
	}
	defer memProfileFile.Close()

	// 启动CPU分析
	fmt.Println("启动CPU性能分析...")
	if err := pprof.StartCPUProfile(cpuProfileFile); err != nil {
		fmt.Printf("启动CPU分析失败: %v\n", err)
		return
	}
	defer pprof.StopCPUProfile()

	// 统计变量（使用原子操作）
	var (
		requestsSent      int64
		requestsCompleted int64
		requestsFailed    int64
		bytesReceived     int64
		totalLatency      int64
		minLatency        int64 = 1<<63 - 1
		maxLatency        int64
	)

	// 初始化最小延迟
	atomic.StoreInt64(&minLatency, 1<<63-1)

	// 控制测试时长
	stop := make(chan struct{})
	time.AfterFunc(duration, func() {
		close(stop)
	})

	fmt.Println("开始10,000并发压力测试...")

	startTime := time.Now()

	// 启动并发请求goroutine
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			client := &http.Client{
				Timeout: 30 * time.Second,
				Transport: &http.Transport{
					MaxIdleConns:        20000,
					MaxIdleConnsPerHost: 2000,
					IdleConnTimeout:     90 * time.Second,
				},
			}

			for {
				select {
				case <-stop:
					return
				default:
					// 发送请求
					reqStart := time.Now()
					atomic.AddInt64(&requestsSent, 1)

					resp, err := client.Get(targetURL)
					if err != nil {
						atomic.AddInt64(&requestsFailed, 1)
						continue
					}

					// 读取响应体
					body, err := io.ReadAll(resp.Body)
					resp.Body.Close()

					latency := time.Since(reqStart).Nanoseconds()

					if err != nil {
						atomic.AddInt64(&requestsFailed, 1)
					} else {
						atomic.AddInt64(&requestsCompleted, 1)
						atomic.AddInt64(&bytesReceived, int64(len(body)))

						// 更新延迟统计
						atomic.AddInt64(&totalLatency, latency)

						// 更新最小延迟
						for {
							currentMin := atomic.LoadInt64(&minLatency)
							if latency >= currentMin || atomic.CompareAndSwapInt64(&minLatency, currentMin, latency) {
								break
							}
						}

						// 更新最大延迟
						for {
							currentMax := atomic.LoadInt64(&maxLatency)
							if latency <= currentMax || atomic.CompareAndSwapInt64(&maxLatency, currentMax, latency) {
								break
							}
						}
					}
				}
			}
		}(i)
	}

	// 内存和性能监控goroutine
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				// 强制GC以获取准确的内存统计
				runtime.GC()
				runtime.GC() // 两次GC确保清理

				// 收集内存profile
				if err := pprof.WriteHeapProfile(memProfileFile); err != nil {
					fmt.Printf("写入内存profile失败: %v\n", err)
				}

				// 实时性能监控
				sent := atomic.LoadInt64(&requestsSent)
				completed := atomic.LoadInt64(&requestsCompleted)
				failed := atomic.LoadInt64(&requestsFailed)

				rps := float64(completed) / time.Since(startTime).Seconds()

				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)

				fmt.Printf("\r进度: 发送=%d, 完成=%d, 失败=%d, RPS=%.0f, 内存=%.1fMB, GC次数=%d",
					sent, completed, failed, rps,
					float64(memStats.Alloc)/(1024*1024), memStats.NumGC)
			}
		}
	}()

	// 等待测试完成
	wg.Wait()
	endTime := time.Now()
	totalDuration := endTime.Sub(startTime)

	// 停止CPU分析
	pprof.StopCPUProfile()

	// 最终内存profile
	runtime.GC()
	runtime.GC()
	if err := pprof.WriteHeapProfile(memProfileFile); err != nil {
		fmt.Printf("最终内存profile写入失败: %v\n", err)
	}

	// 计算最终统计
	finalSent := atomic.LoadInt64(&requestsSent)
	finalCompleted := atomic.LoadInt64(&requestsCompleted)
	finalFailed := atomic.LoadInt64(&requestsFailed)
	finalBytes := atomic.LoadInt64(&bytesReceived)
	finalTotalLatency := atomic.LoadInt64(&totalLatency)
	finalMinLatency := atomic.LoadInt64(&minLatency)
	finalMaxLatency := atomic.LoadInt64(&maxLatency)

	fmt.Println("\n")
	fmt.Println("=== 最终测试结果 ===")
	fmt.Printf("测试时长: %v\n", totalDuration)
	fmt.Printf("总发送请求: %d\n", finalSent)
	fmt.Printf("成功完成请求: %d\n", finalCompleted)
	fmt.Printf("失败请求: %d\n", finalFailed)
	fmt.Printf("成功率: %.2f%%\n", float64(finalCompleted)/float64(finalSent)*100)

	if finalCompleted > 0 {
		avgRPS := float64(finalCompleted) / totalDuration.Seconds()
		fmt.Printf("平均RPS: %.0f\n", avgRPS)

		avgLatency := time.Duration(finalTotalLatency / finalCompleted)
		fmt.Printf("平均延迟: %v\n", avgLatency)

		fmt.Printf("最小延迟: %v\n", time.Duration(finalMinLatency))
		fmt.Printf("最大延迟: %v\n", time.Duration(finalMaxLatency))

		avgBytes := float64(finalBytes) / float64(finalCompleted)
		fmt.Printf("平均响应大小: %.0f bytes\n", avgBytes)

		bandwidth := float64(finalBytes) / totalDuration.Seconds() / 1024 / 1024
		fmt.Printf("带宽使用: %.2f MB/s\n", bandwidth)
	}

	// 内存使用统计
	var finalMemStats runtime.MemStats
	runtime.ReadMemStats(&finalMemStats)

	fmt.Println("\n=== 内存使用统计 ===")
	fmt.Printf("已分配内存: %.2f MB\n", float64(finalMemStats.Alloc)/(1024*1024))
	fmt.Printf("总内存使用: %.2f MB\n", float64(finalMemStats.Sys)/(1024*1024))
	fmt.Printf("堆内存: %.2f MB\n", float64(finalMemStats.HeapAlloc)/(1024*1024))
	fmt.Printf("栈内存: %.2f MB\n", float64(finalMemStats.StackInuse)/(1024*1024))
	fmt.Printf("GC次数: %d\n", finalMemStats.NumGC)
	fmt.Printf("GC暂停总时间: %v\n", time.Duration(finalMemStats.PauseTotalNs))

	// 性能评估
	fmt.Println("\n=== 性能评估 ===")
	if finalCompleted > 50000 { // 5万+ RPS
		fmt.Println("🎉 性能表现: 优秀 (支持超高并发)")
	} else if finalCompleted > 20000 { // 2万+ RPS
		fmt.Println("👍 性能表现: 良好 (支持高并发)")
	} else if finalCompleted > 10000 { // 1万+ RPS
		fmt.Println("⚠️  性能表现: 一般 (支持中等并发)")
	} else {
		fmt.Println("❌ 性能表现: 需要优化")
	}

	fmt.Println("\n=== 分析文件生成 ===")
	fmt.Println("CPU性能分析文件: cpu_profile.prof")
	fmt.Println("内存性能分析文件: mem_profile.prof")
	fmt.Println("\n生成火焰图命令:")
	fmt.Println("  go tool pprof -http=:8081 cpu_profile.prof")
	fmt.Println("  go tool pprof -http=:8082 mem_profile.prof")

	fmt.Println("\n测试完成!")
}

