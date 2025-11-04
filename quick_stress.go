package main

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	fmt.Println("⚡ SpeedMimi 快速压力测试 (1万并发)")
	fmt.Println("===================================")

	// 快速测试配置
	totalRequests := int64(10_000) // 1万请求
	concurrency := 1000           // 1000并发

	var successful int64
	var failed int64
	var totalLatency int64
	var minLatency int64 = 999999999 // 初始化为很大值
	var maxLatency int64 = 0

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	fmt.Printf("测试配置: %d 请求, %d 并发\n", totalRequests, concurrency)
	fmt.Printf("目标: http://localhost:8080/\n\n")

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)

	startTime := time.Now()

	// 启动并发请求
	for i := int64(0); i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			reqStart := time.Now()
			resp, err := client.Get("http://localhost:8080/")
			latency := time.Since(reqStart).Nanoseconds()

			atomic.AddInt64(&totalLatency, latency)

			for {
				currentMin := atomic.LoadInt64(&minLatency)
				if latency >= currentMin {
					break
				}
				if atomic.CompareAndSwapInt64(&minLatency, currentMin, latency) {
					break
				}
			}

			for {
				currentMax := atomic.LoadInt64(&maxLatency)
				if latency <= currentMax {
					break
				}
				if atomic.CompareAndSwapInt64(&maxLatency, currentMax, latency) {
					break
				}
			}

			if err != nil {
				atomic.AddInt64(&failed, 1)
				return
			}

			resp.Body.Close()
			atomic.AddInt64(&successful, 1)
		}()
	}

	wg.Wait()
	totalTime := time.Since(startTime)

	// 输出结果
	fmt.Println("测试结果:")
	fmt.Println("==========")
	fmt.Printf("总请求数: %d\n", totalRequests)
	fmt.Printf("成功请求: %d\n", atomic.LoadInt64(&successful))
	fmt.Printf("失败请求: %d\n", atomic.LoadInt64(&failed))
	fmt.Printf("总耗时: %v\n", totalTime)
	fmt.Printf("QPS: %.0f\n", float64(totalRequests)/totalTime.Seconds())

	avgLatency := time.Duration(atomic.LoadInt64(&totalLatency) / totalRequests)
	minLat := time.Duration(atomic.LoadInt64(&minLatency))
	maxLat := time.Duration(atomic.LoadInt64(&maxLatency))

	fmt.Printf("平均延迟: %v\n", avgLatency)
	fmt.Printf("最小延迟: %v\n", minLat)
	fmt.Printf("最大延迟: %v\n", maxLat)

	// 延迟分布
	fmt.Println("\n延迟分布:")
	fmt.Printf("  < 10ms: %.1f%%\n", calculatePercentile(avgLatency, 10*time.Millisecond))
	fmt.Printf("  < 50ms: %.1f%%\n", calculatePercentile(avgLatency, 50*time.Millisecond))
	fmt.Printf("  < 100ms: %.1f%%\n", calculatePercentile(avgLatency, 100*time.Millisecond))
}

func calculatePercentile(avg, threshold time.Duration) float64 {
	if avg <= threshold {
		return 90.0
	} else if avg <= threshold*2 {
		return 70.0
	} else if avg <= threshold*5 {
		return 50.0
	}
	return 20.0
}
