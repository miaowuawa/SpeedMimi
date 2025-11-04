# SpeedMimi 高并发优化 Dockerfile

# 构建阶段
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git ca-certificates

# 复制go mod文件
COPY go.mod go.sum ./

# 下载依赖（使用国内代理加速）
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download

# 复制源代码
COPY . .

# 生产环境优化构建
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a \
    -installsuffix cgo \
    -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty)" \
    -gcflags="all=-l -B" \
    -o speedmimi \
    cmd/server/main.go

# 运行阶段 - 使用优化的基础镜像
FROM alpine:latest

# 安装运行时依赖和系统调优工具
RUN apk --no-cache add \
    ca-certificates \
    libcap \
    && rm -rf /var/cache/apk/*

# 创建非root用户
RUN addgroup -S -g 1001 speedmimi && \
    adduser -S -u 1001 -G speedmimi speedmimi

# 创建工作目录
WORKDIR /app

# 创建必要的目录
RUN mkdir -p configs certs logs pprof && \
    chown -R speedmimi:speedmimi /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/speedmimi .

# 复制配置文件
COPY --from=builder /app/configs ./configs

# 设置适当的文件权限
RUN chmod +x ./speedmimi

# 切换到非root用户
USER speedmimi

# 暴露端口
EXPOSE 8080 9091

# 设置环境变量优化
ENV GOMAXPROCS=0 \
    GOGC=100 \
    GODEBUG=gctrace=0

# 容器优化
RUN ulimit -n 1000000 && \
    ulimit -u 100000

# 健康检查 - 使用更轻量的检查
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD /app/speedmimi --version || exit 1

# 启动命令 - 使用exec格式避免shell层
CMD ["./speedmimi", "-config", "configs/config.yaml"]
