package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/quqi/speedmimi/pkg/types"
)

func main() {
	// 测试连接限制功能
	backend := &types.Backend{
		ID:       "test-backend",
		Name:     "Test Backend",
		MaxConn:  2, // 设置最大连接数为2
		Connections: 0,
	}

	// 初始化为活跃状态
	backend.SetActive(true)

	fmt.Printf("Backend MaxConn: %d\n", backend.MaxConn)

	// 测试连接数增加
	fmt.Println("Testing connection increments...")
	backend.IncConnections()
	fmt.Printf("Connections after IncConnections(): %d\n", backend.GetConnections())
	fmt.Printf("IsConnectionLimitReached(): %v\n", backend.IsConnectionLimitReached())

	backend.IncConnections()
	fmt.Printf("Connections after IncConnections(): %d\n", backend.GetConnections())
	fmt.Printf("IsConnectionLimitReached(): %v\n", backend.IsConnectionLimitReached())

	// 尝试超过限制
	backend.IncConnections()
	fmt.Printf("Connections after IncConnections() (exceeded): %d\n", backend.GetConnections())
	fmt.Printf("IsConnectionLimitReached(): %v\n", backend.IsConnectionLimitReached())

	// 测试连接数减少
	backend.DecConnections()
	fmt.Printf("Connections after DecConnections(): %d\n", backend.GetConnections())
	fmt.Printf("IsConnectionLimitReached(): %v\n", backend.IsConnectionLimitReached())

	// 测试无限制情况
	backend.MaxConn = 0 // 0表示无限制
	fmt.Printf("\nTesting unlimited connections (MaxConn=0)...\n")
	fmt.Printf("IsConnectionLimitReached(): %v\n", backend.IsConnectionLimitReached())

	backend.MaxConn = -1 // 负数也表示无限制
	fmt.Printf("Testing unlimited connections (MaxConn=-1)...\n")
	fmt.Printf("IsConnectionLimitReached(): %v\n", backend.IsConnectionLimitReached())

	fmt.Println("\nConnection limit functionality test completed!")

	// 测试API调用
	testAPI()
}

func testAPI() {
	fmt.Println("\nTesting API for updating MaxConn...")

	// 启动一个简单的HTTP服务器来模拟API调用
	go func() {
		http.HandleFunc("/api/v1/backends/update", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "message": "Backend updated successfully"}`))
		})
		log.Println("Test API server started on :9090")
		log.Fatal(http.ListenAndServe(":9090", nil))
	}()

	time.Sleep(100 * time.Millisecond) // 等待服务器启动

	// 发送API请求
	resp, err := http.Post("http://localhost:9090/api/v1/backends/update", "application/json",
		strings.NewReader(`{"upstream_id": "default", "backend_id": "backend1", "max_conn": 500}`))
	if err != nil {
		fmt.Printf("API call failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("API call successful, status: %d\n", resp.StatusCode)
}
