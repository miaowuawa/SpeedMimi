# SpeedMimi Makefile

.PHONY: build run clean test fmt vet mod-tidy

# 构建二进制文件
build:
	go build -o bin/speedmimi cmd/server/main.go

# 运行服务器
run: build
	./bin/speedmimi -config configs/config.yaml

# 开发模式运行（带热重载）
dev:
	@echo "Starting SpeedMimi in development mode..."
	./bin/speedmimi -config configs/config.yaml

# 清理构建文件
clean:
	rm -rf bin/
	go clean

# 运行测试
test:
	go test ./...

# 格式化代码
fmt:
	go fmt ./...

# 代码检查
vet:
	go vet ./...

# 依赖整理
mod-tidy:
	go mod tidy

# 安装依赖
deps:
	go mod download

# 创建必要的目录
init:
	mkdir -p bin certs configs

# 构建Docker镜像
docker-build:
	docker build -t speedmimi:latest .

# 运行Docker容器
docker-run:
	docker run -p 8080:8080 -p 9091:9091 -v $(PWD)/configs:/app/configs speedmimi:latest

# 运行千万级并发测试
test-million:
	@echo "Running SpeedMimi million concurrent test..."
	go run test/million_concurrent_test.go

# 系统调优（需要root权限）
tune-system:
	@echo "Tuning system for high concurrency (requires root)..."
	sudo ./tune_system.sh

# 生产环境构建（优化版本）
build-prod:
	@echo "Building SpeedMimi for production..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-s -w" -o bin/speedmimi cmd/server/main.go
	@echo "Production binary built with optimizations"

# 性能分析
profile:
	@echo "Running performance profiling..."
	go test -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof ./...
	@echo "Profiles saved: cpu.prof, mem.prof"

# 帮助信息
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  run          - Build and run the server"
	@echo "  dev          - Run in development mode"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run Go unit tests"
	@echo "  test-million - Run million concurrent test"
	@echo "  tune-system  - Tune system for high concurrency (root)"
	@echo "  build-prod   - Build optimized production binary"
	@echo "  profile      - Run performance profiling"
	@echo "  fmt          - Format code"
	@echo "  vet          - Run go vet"
	@echo "  mod-tidy     - Tidy go modules"
	@echo "  deps         - Download dependencies"
	@echo "  init         - Create necessary directories"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo "  help         - Show this help message"
