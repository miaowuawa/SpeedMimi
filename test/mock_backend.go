package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	port := 8081 // 默认端口
	if len(os.Args) > 1 {
		if p, err := strconv.Atoi(os.Args[1]); err == nil {
			port = p
		}
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 模拟一些处理时间
		time.Sleep(1 * time.Millisecond)

		// 返回一个小的JSON响应
		response := `{"status":"ok","message":"Hello from SpeedMimi backend","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	fmt.Printf("Mock backend server starting on port %d...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

