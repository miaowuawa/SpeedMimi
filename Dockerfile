# SpeedMimi Dockerfile

# 构建阶段
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制go mod文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o speedmimi cmd/server/main.go

# 运行阶段
FROM alpine:latest

# 安装ca-certificates用于HTTPS
RUN apk --no-cache add ca-certificates

# 创建非root用户
RUN addgroup -S speedmimi && adduser -S speedmimi -G speedmimi

# 创建工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/speedmimi .

# 复制配置文件
COPY --from=builder /app/configs ./configs

# 创建证书目录
RUN mkdir -p certs && chown -R speedmimi:speedmimi /app

# 切换到非root用户
USER speedmimi

# 暴露端口
EXPOSE 8080 9091

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:9091/api/v1/stats/server || exit 1

# 启动命令
CMD ["./speedmimi", "-config", "configs/config.yaml"]
