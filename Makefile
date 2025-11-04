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

# 帮助信息
help:
	@echo "Available targets:"
	@echo "  build      - Build the binary"
	@echo "  run        - Build and run the server"
	@echo "  dev        - Run in development mode"
	@echo "  clean      - Clean build artifacts"
	@echo "  test       - Run tests"
	@echo "  fmt        - Format code"
	@echo "  vet        - Run go vet"
	@echo "  mod-tidy   - Tidy go modules"
	@echo "  deps       - Download dependencies"
	@echo "  init       - Create necessary directories"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo "  help       - Show this help message"
