package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	// 初始化并启动gRPC服务器
	if cfg.GRPC.Enabled {
		grpcServer := grpcservice.NewServer(configMgr, proxyServer)
		go func() {
			log.Printf("Starting gRPC server on %s:%d", cfg.GRPC.Host, cfg.GRPC.Port)
			if err := grpcServer.Start(cfg.GRPC.Host, cfg.GRPC.Port); err != nil {
				log.Fatalf("Failed to start gRPC server: %v", err)
			}
		}()
	}

	// 等待中断信号
	waitForShutdown(proxyServer)
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
