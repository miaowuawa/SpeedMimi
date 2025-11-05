package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/quqi/speedmimi/pkg/types"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run backend_performance.go <port>")
	}

	port := os.Args[1]

	// 创建HTTP客户端用于上报性能
	client := &http.Client{Timeout: 5 * time.Second}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 模拟处理时间
		time.Sleep(5 * time.Millisecond)

		response := map[string]interface{}{
			"server":    fmt.Sprintf("Backend-%s", port),
			"port":      port,
			"timestamp": time.Now().Format(time.RFC3339),
			"path":      r.URL.Path,
			"method":    r.Method,
			"user_agent": r.Header.Get("User-Agent"),
			"remote_addr": r.RemoteAddr,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Server", fmt.Sprintf("Backend-%s", port))
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "healthy",
			"server": fmt.Sprintf("Backend-%s", port),
			"port":   port,
		})
	})

	// 异步性能上报goroutine
	go func() {
		ticker := time.NewTicker(3 * time.Second) // 每3秒上报一次
		defer ticker.Stop()

		upstream := "default"
		backendID := fmt.Sprintf("backend-%s", port)

		for range ticker.C {
			// 异步收集性能指标（模拟）
			perf := &types.PerformanceInfo{
				CPUUsage:    float64(runtime.NumGoroutine()) * 0.5, // 模拟CPU使用率
				MemoryUsage: float64(runtime.NumGoroutine()) * 2.0, // 模拟内存使用率
				DiskUsage:   15.5,  // 模拟磁盘使用率
				LoadAvg1:    float64(runtime.NumGoroutine()) / 10.0,
				LoadAvg5:    float64(runtime.NumGoroutine()) / 10.0,
				LoadAvg15:   float64(runtime.NumGoroutine()) / 10.0,
				NetworkIn:   1250.5,  // KB/s
				NetworkOut:  890.3,   // KB/s
				Timestamp:   time.Now().Unix(),
			}

			// 上报到管理API（异步，不阻塞）
			go reportPerformance(client, upstream, backendID, perf)
		}
	}()

	addr := ":" + port
	fmt.Printf("后端性能服务器启动在端口 %s (支持自动性能上报)\n", port)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// reportPerformance 上报性能数据（异步）
func reportPerformance(client *http.Client, upstream, backendID string, perf *types.PerformanceInfo) {
	reportData := map[string]interface{}{
		"upstream":  upstream,
		"backend_id": backendID,
		"performance": perf,
	}

	jsonData, err := json.Marshal(reportData)
	if err != nil {
		fmt.Printf("序列化性能数据失败: %v\n", err)
		return
	}

	// 发送到管理API
	resp, err := client.Post("http://localhost:9091/api/v1/report", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		// 不输出错误，避免影响性能
		return
	}
	defer resp.Body.Close()

	// 读取响应（可选，用于调试）
	io.ReadAll(resp.Body)
}

