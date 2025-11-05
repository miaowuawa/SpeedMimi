package main

import (
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof" // 导入pprof包
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/quqi/speedmimi/internal/config"
	"github.com/quqi/speedmimi/internal/grpcservice"
	"github.com/quqi/speedmimi/internal/proxy"
)

var (
	configPath = flag.String("config", "configs/config.yaml", "Path to configuration file")
)

func main() {
	flag.Parse()

	// 初始化配置管理器
	configMgr, err := config.NewManager(*configPath)
	if err != nil {
		log.Fatalf("Failed to initialize config manager: %v", err)
	}

	cfg := configMgr.GetConfig()

	// 初始化反向代理服务器
	proxyServer, err := proxy.NewServer(configMgr)
	if err != nil {
		log.Fatalf("Failed to initialize proxy server: %v", err)
	}

	// 启动反向代理服务器
	go func() {
		log.Printf("Starting proxy server on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := proxyServer.Start(); err != nil {
			log.Fatalf("Failed to start proxy server: %v", err)
		}
	}()

	// 启动pprof性能分析服务器
	go func() {
		log.Printf("Starting pprof server on 0.0.0.0:6060")
		log.Printf("Access pprof at: http://localhost:6060/debug/pprof/")
		if err := http.ListenAndServe("0.0.0.0:6060", nil); err != nil {
			log.Printf("Failed to start pprof server: %v", err)
		}
	}()

	// 启动系统性能监控
	go startSystemMonitoring()

	// 初始化并启动管理API服务器
	if cfg.GRPC.Enabled {
		monitor := proxyServer.GetMonitor()
		grpcServer := grpcservice.NewServer(configMgr, proxyServer, monitor)
		go func() {
			log.Printf("Starting management API server on %s:%d", cfg.GRPC.Host, cfg.GRPC.Port)
			if err := grpcServer.Start(cfg.GRPC.Host, cfg.GRPC.Port); err != nil {
				log.Fatalf("Failed to start management API server: %v", err)
			}
		}()
	}

	// 等待中断信号
	waitForShutdown(proxyServer)
}

// startSystemMonitoring 启动系统性能监控
func startSystemMonitoring() {
	log.Println("Starting system performance monitoring...")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var lastNumGC uint32
	var lastPauseTotalNs uint64

	for {
		select {
		case <-ticker.C:
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			// 计算GC统计
			gcCount := memStats.NumGC - lastNumGC
			gcPause := memStats.PauseTotalNs - lastPauseTotalNs

			log.Printf("📊 System Metrics - Goroutines: %d, Memory: %.1fMB, Heap: %.1fMB, Stack: %.1fMB, GC: %d (%.2fms)",
				runtime.NumGoroutine(),
				float64(memStats.Sys)/(1024*1024),
				float64(memStats.HeapAlloc)/(1024*1024),
				float64(memStats.StackInuse)/(1024*1024),
				gcCount,
				float64(gcPause)/1000000)

			lastNumGC = memStats.NumGC
			lastPauseTotalNs = memStats.PauseTotalNs
		}
	}
}

func waitForShutdown(proxyServer *proxy.Server) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	log.Println("Shutting down server...")

	// 优雅关闭
	if err := proxyServer.Stop(); err != nil {
		log.Printf("Error stopping proxy server: %v", err)
	}

	log.Println("Server stopped")
}
