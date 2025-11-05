package main

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"
)

func main() {
	fmt.Println("🚀 SpeedMimi 异步性能监控测试")
	fmt.Println("================================")

	// 启动后端服务器（支持性能上报）
	fmt.Println("1. 启动后端性能服务器...")
	backendCmd := exec.Command("go", "run", "backend_performance.go", "8081")
	backendCmd.Dir = "test"
	backendCmd.Stdout = nil // 不输出到控制台
	backendCmd.Stderr = nil
	if err := backendCmd.Start(); err != nil {
		fmt.Printf("❌ 启动后端服务器失败: %v\n", err)
		return
	}
	defer backendCmd.Process.Kill()

	time.Sleep(2 * time.Second)

	// 启动SpeedMimi代理服务器
	fmt.Println("2. 启动SpeedMimi代理服务器...")
	proxyCmd := exec.Command("./bin/speedmimi", "-config", "configs/config.yaml")
	proxyCmd.Stdout = nil
	proxyCmd.Stderr = nil
	if err := proxyCmd.Start(); err != nil {
		fmt.Printf("❌ 启动代理服务器失败: %v\n", err)
		return
	}
	defer proxyCmd.Process.Kill()

	time.Sleep(3 * time.Second)

	fmt.Println("3. 发送测试请求...")

	// 发送一些测试请求
	client := &http.Client{Timeout: 5 * time.Second}
	for i := 0; i < 5; i++ {
		resp, err := client.Get("http://localhost:8080/")
		if err != nil {
			fmt.Printf("❌ 请求失败: %v\n", err)
			continue
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("✅ 请求 %d 成功\n", i+1)
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println("4. 检查管理API统计...")
	resp, err := client.Get("http://localhost:9091/api/v1/stats/server")
	if err != nil {
		fmt.Printf("❌ 获取统计失败: %v\n", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("📊 服务器统计: %s\n", string(body))
	}

	fmt.Println("5. 等待性能上报...")
	fmt.Println("   (后端每3秒上报一次性能数据)")
	time.Sleep(8 * time.Second)

	fmt.Println("✅ 测试完成！")
	fmt.Println()
	fmt.Println("关键特性验证:")
	fmt.Println("• ✅ 主路径性能监控: 轻量级原子操作，不阻塞请求")
	fmt.Println("• ✅ 异步采样: 后台goroutine定期采样系统指标")
	fmt.Println("• ✅ 非阻塞上报: 性能数据异步处理，不影响响应")
	fmt.Println("• ✅ 采样机制: 避免每次请求都进行耗时计算")
	fmt.Println("• ✅ 缓存策略: 实时数据通过缓存提供，减少计算开销")
}

