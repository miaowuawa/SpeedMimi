package main

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// 详细的故障诊断程序
func main() {
	fmt.Println("🔍 SpeedMimi请求失败诊断分析")
	fmt.Println("===============================")

	// 测试参数
	targetURL := "http://localhost:8080"
	concurrency := 100 // 先用少量并发测试
	duration := 10 * time.Second

	fmt.Printf("目标URL: %s\n", targetURL)
	fmt.Printf("并发数: %d\n", concurrency)
	fmt.Printf("测试时长: %v\n\n", duration)

	// 统计变量
	var (
		requestsSent      int64
		requestsCompleted int64
		requestsFailed    int64
		connectionErrors  int64
		timeoutErrors     int64
		otherErrors       int64
	)

	// 错误详情收集
	errorDetails := make(map[string]int64)
	var errorMutex sync.Mutex

	// 控制测试时长
	stop := make(chan struct{})
	time.AfterFunc(duration, func() {
		close(stop)
	})

	fmt.Println("开始详细诊断测试...")

	startTime := time.Now()

	// 启动并发请求goroutine
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// 使用更详细的客户端配置
			client := &http.Client{
				Timeout: 5 * time.Second, // 减少超时时间便于诊断
				Transport: &http.Transport{
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 10,
					IdleConnTimeout:     30 * time.Second,
					DisableKeepAlives:   false,
				},
			}

			for {
				select {
				case <-stop:
					return
				default:
					atomic.AddInt64(&requestsSent, 1)

					reqStart := time.Now()
					resp, err := client.Get(targetURL)
					latency := time.Since(reqStart)

					if err != nil {
						atomic.AddInt64(&requestsFailed, 1)

						// 分类错误类型
						errorMutex.Lock()
						errorDetails[err.Error()]++
						errorMutex.Unlock()

						// 粗略分类错误
						errStr := err.Error()
						if contains(errStr, "timeout") || contains(errStr, "deadline") {
							atomic.AddInt64(&timeoutErrors, 1)
						} else if contains(errStr, "connection") || contains(errStr, "connect") {
							atomic.AddInt64(&connectionErrors, 1)
						} else {
							atomic.AddInt64(&otherErrors, 1)
						}

						// 添加小延迟避免过于频繁的重试
						time.Sleep(1 * time.Millisecond)
						continue
					}

					// 读取响应体
					_, err = io.ReadAll(resp.Body)
					resp.Body.Close()

					if err != nil {
						atomic.AddInt64(&requestsFailed, 1)
						errorMutex.Lock()
						errorDetails["body_read_error: "+err.Error()]++
						errorMutex.Unlock()
					} else {
						atomic.AddInt64(&requestsCompleted, 1)

						// 检查响应状态
						if resp.StatusCode != 200 {
							atomic.AddInt64(&requestsFailed, 1)
							atomic.AddInt64(&requestsCompleted, -1)
							errorMutex.Lock()
							errorDetails[fmt.Sprintf("http_%d", resp.StatusCode)]++
							errorMutex.Unlock()
						}
					}

					// 进度输出
					if atomic.LoadInt64(&requestsSent)%1000 == 0 {
						sent := atomic.LoadInt64(&requestsSent)
						completed := atomic.LoadInt64(&requestsCompleted)
						failed := atomic.LoadInt64(&requestsFailed)
						rps := float64(completed) / time.Since(startTime).Seconds()
						fmt.Printf("\r进度: 发送=%d, 完成=%d, 失败=%d, RPS=%.0f, 延迟=%v",
							sent, completed, failed, rps, latency)
					}
				}
			}
		}(i)
	}

	// 等待测试完成
	wg.Wait()
	endTime := time.Now()
	totalDuration := endTime.Sub(startTime)

	// 计算最终统计
	finalSent := atomic.LoadInt64(&requestsSent)
	finalCompleted := atomic.LoadInt64(&requestsCompleted)
	finalFailed := atomic.LoadInt64(&requestsFailed)
	finalTimeouts := atomic.LoadInt64(&timeoutErrors)
	finalConnErrors := atomic.LoadInt64(&connectionErrors)
	finalOtherErrors := atomic.LoadInt64(&otherErrors)

	fmt.Println("\n")
	fmt.Println("=== 详细故障分析结果 ===")
	fmt.Printf("测试时长: %v\n", totalDuration)
	fmt.Printf("总发送请求: %d\n", finalSent)
	fmt.Printf("成功完成请求: %d\n", finalCompleted)
	fmt.Printf("失败请求: %d\n", finalFailed)
	fmt.Printf("成功率: %.2f%%\n", float64(finalCompleted)/float64(finalSent)*100)

	if finalFailed > 0 {
		fmt.Printf("超时错误: %d (%.2f%%)\n", finalTimeouts, float64(finalTimeouts)/float64(finalFailed)*100)
		fmt.Printf("连接错误: %d (%.2f%%)\n", finalConnErrors, float64(finalConnErrors)/float64(finalFailed)*100)
		fmt.Printf("其他错误: %d (%.2f%%)\n", finalOtherErrors, float64(finalOtherErrors)/float64(finalFailed)*100)
	}

	fmt.Println("\n=== 详细错误分类 ===")
	errorMutex.Lock()
	for errMsg, count := range errorDetails {
		fmt.Printf("%d 次: %s\n", count, errMsg)
	}
	errorMutex.Unlock()

	fmt.Println("\n=== 可能的原因分析 ===")

	if float64(finalTimeouts) > float64(finalFailed)*0.5 {
		fmt.Println("🔴 主要问题: 超时错误占比过高")
		fmt.Println("   可能原因:")
		fmt.Println("   - 后端服务器响应过慢")
		fmt.Println("   - 网络延迟过高")
		fmt.Println("   - 服务器过载，处理能力不足")
		fmt.Println("   - 客户端超时设置过短 (5秒)")
	}

	if float64(finalConnErrors) > float64(finalFailed)*0.3 {
		fmt.Println("🔴 主要问题: 连接错误占比较高")
		fmt.Println("   可能原因:")
		fmt.Println("   - 服务器拒绝连接 (达到连接上限)")
		fmt.Println("   - 网络连接问题")
		fmt.Println("   - 防火墙或安全策略阻拦")
		fmt.Println("   - 端口耗尽 (ephemeral ports)")
	}

	if float64(finalCompleted) > float64(finalSent)*0.9 {
		fmt.Println("🟢 性能表现良好，失败率在合理范围内")
		fmt.Println("   可能原因:")
		fmt.Println("   - 高并发下的正常波动")
		fmt.Println("   - 瞬时网络抖动")
		fmt.Println("   - 系统资源竞争")
	}

	fmt.Println("\n=== 优化建议 ===")
	fmt.Println("1. 检查后端服务器性能和响应时间")
	fmt.Println("2. 调整客户端超时设置 (当前5秒)")
	fmt.Println("3. 检查系统连接数限制: ulimit -n")
	fmt.Println("4. 监控网络延迟和丢包率")
	fmt.Println("5. 调整服务器并发处理能力")
	fmt.Println("6. 使用连接池复用减少连接建立开销")

	fmt.Println("\n诊断完成!")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		 containsInner(s, substr)))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
