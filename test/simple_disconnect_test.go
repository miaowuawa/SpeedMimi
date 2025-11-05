package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

func main() {
	fmt.Println("🔧 简单断开连接测试")

	// 启动后端服务器
	fmt.Println("启动后端服务器...")
	backend1Cmd := exec.Command("go", "run", "backend_performance.go", "8081")
	backend1Cmd.Dir = "test"
	backend1Cmd.Stdout = nil
	backend1Cmd.Stderr = nil
	backend1Cmd.Start()
	defer backend1Cmd.Process.Kill()

	backend2Cmd := exec.Command("go", "run", "backend_performance.go", "8082")
	backend2Cmd.Dir = "test"
	backend2Cmd.Stdout = nil
	backend2Cmd.Stderr = nil
	backend2Cmd.Start()
	defer backend2Cmd.Process.Kill()

	time.Sleep(2 * time.Second)

	// 启动SpeedMimi代理服务器
	fmt.Println("启动SpeedMimi代理服务器...")
	proxyCmd := exec.Command("./bin/speedmimi", "-config", "configs/config.yaml")
	proxyCmd.Stdout = os.Stdout
	proxyCmd.Stderr = os.Stderr
	proxyCmd.Start()
	defer proxyCmd.Process.Kill()

	time.Sleep(3 * time.Second)

	// 发送断开连接请求
	fmt.Println("发送断开backend1连接请求...")
	client := &http.Client{Timeout: 5 * time.Second}

	disconnectData := map[string]string{
		"upstream_id": "default",
		"backend_id":  "backend1",
	}
	jsonData, _ := json.Marshal(disconnectData)

	resp, err := client.Post("http://localhost:9091/api/v1/backends/disconnect", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("响应: %s\n", string(body))

	// 等待异步处理
	fmt.Println("等待异步处理完成...")
	time.Sleep(3 * time.Second)

	fmt.Println("测试完成")
}

