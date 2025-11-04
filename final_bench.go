package main

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

func main() {
	fmt.Println("🎯 SpeedMimi 最终负载均衡测试")
	fmt.Println("=============================")

	// 启动后端服务器
	fmt.Println("启动后端服务器...")
	// 这里假设后端服务器已经在运行

	// 启动代理服务器
	fmt.Println("启动代理服务器...")
	// 这里假设代理服务器已经在运行

	time.Sleep(2 * time.Second)

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 测试并发请求
	fmt.Println("测试并发负载均衡...")

	totalRequests := 50
	concurrency := 10

	var wg sync.WaitGroup
	results := make(chan string, totalRequests)

	// 启动并发请求
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			requestsPerWorker := totalRequests / concurrency
			if workerID < totalRequests%concurrency {
				requestsPerWorker++
			}

			for j := 0; j < requestsPerWorker; j++ {
				resp, err := client.Get("http://localhost:8080/")
				if err != nil {
					results <- fmt.Sprintf("ERROR: %v", err)
					continue
				}

				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()

				if err != nil {
					results <- "ERROR: Read body failed"
					continue
				}

				// 解析服务器信息
				server := "unknown"
				if len(body) > 0 {
					// 简单的字符串查找
					bodyStr := string(body)
					if contains(bodyStr, "Backend-1") {
						server = "Backend-1"
					} else if contains(bodyStr, "Backend-2") {
						server = "Backend-2"
					}
				}

				results <- fmt.Sprintf("SUCCESS: %s", server)
			}
		}(i)
	}

	// 收集结果
	go func() {
		wg.Wait()
		close(results)
	}()

	// 统计结果
	backend1Count := 0
	backend2Count := 0
	errorCount := 0
	processed := 0

	for result := range results {
		processed++
		fmt.Printf("\r处理请求: %d/%d", processed, totalRequests)

		if contains(result, "ERROR") {
			errorCount++
		} else if contains(result, "Backend-1") {
			backend1Count++
		} else if contains(result, "Backend-2") {
			backend2Count++
		}
	}

	fmt.Println("\n")
	fmt.Println("=== 最终测试结果 ===")
	fmt.Printf("总请求数: %d\n", totalRequests)
	fmt.Printf("成功请求: %d\n", backend1Count+backend2Count)
	fmt.Printf("错误请求: %d\n", errorCount)
	fmt.Printf("Backend-1: %d 次 (%.1f%%)\n", backend1Count, float64(backend1Count)/float64(backend1Count+backend2Count)*100)
	fmt.Printf("Backend-2: %d 次 (%.1f%%)\n", backend2Count, float64(backend2Count)/float64(backend1Count+backend2Count)*100)

	if backend1Count > 0 && backend2Count > 0 {
		fmt.Println("✅ 负载均衡正常工作！")
	} else {
		fmt.Println("❌ 负载均衡可能有问题")
	}

	fmt.Println("\n测试完成!")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
