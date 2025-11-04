# SpeedMimi - 高性能反向代理服务器

SpeedMimi 是一个高性能的反向代理转发服务器，支持多种负载均衡算法和动态配置管理。

## 特性

### 负载均衡算法
- **IP Hash**: 基于客户端IP地址进行哈希选择
- **最少连接数 (Least Connections)**: 选择当前连接数最少的后端服务器
- **最少连接数+权重 (Least Connections + Weight)**: 综合考虑连接数和权重
- **权重 (Weight)**: 基于权重比例分配请求
- **性能+最少连接数+权重 (Performance + Least Connections + Weight)**: 综合考虑服务器性能、连接数和权重

### 协议特定路由
- 支持WebSocket、SSE等特殊协议的特定负载均衡策略
- HTTP/HTTPS请求可使用不同的负载均衡算法

### 配置管理
- YAML配置文件
- SSL证书配置和动态重新加载
- 真实IP头配置，支持可信代理
- 后端服务器权重和健康检查配置

### 管理API
- RESTful API用于动态配置管理
- 实时性能监控和统计
- 后端服务器动态添加/移除/更新
- 性能数据上报接口

## 快速开始

### 编译
```bash
go build -o bin/speedmimi cmd/server/main.go
```

### 运行
```bash
./bin/speedmimi -config configs/config.yaml
```

### Docker部署
```bash
# 构建镜像
make docker-build

# 运行容器
make docker-run
```

### 配置示例

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  max_conn: 10000
  real_ip_header: "X-Real-IP"
  trusted_proxies:
    - "127.0.0.1/32"
    - "10.0.0.0/8"

ssl:
  enabled: false
  cert_file: "certs/server.crt"
  key_file: "certs/server.key"

backends:
  default:
    - id: "backend1"
      name: "Backend Server 1"
      host: "127.0.0.1"
      port: 8081
      weight: 100
      scheme: "http"
      active: true
      max_conn: 1000
      health_check:
        path: "/health"
        interval: 30s
        timeout: 5s
        failures: 3

routing:
  default:
    path: "/"
    upstream: "default"
    load_balancer: "least_connections_weight"
    protocols:
      websocket: "ip_hash"
      sse: "ip_hash"

grpc:
  enabled: true
  host: "127.0.0.1"
  port: 9090
```

## API文档

### 配置管理

#### 获取当前配置
```http
GET /api/v1/config
```

#### 更新配置
```http
PUT /api/v1/config
Content-Type: application/json

{
  "config": {
    "server": {...},
    "backends": {...},
    "routing": {...}
  }
}
```

#### 重新加载SSL证书
```http
POST /api/v1/config/reload-ssl
```

### 后端管理

#### 获取后端列表
```http
GET /api/v1/backends?upstream=default
```

#### 添加后端
```http
POST /api/v1/backends/add
Content-Type: application/json

{
  "upstream": "default",
  "backend": {
    "id": "backend3",
    "host": "127.0.0.1",
    "port": 8083,
    "weight": 50
  }
}
```

#### 移除后端
```http
DELETE /api/v1/backends/remove?upstream=default&backend_id=backend1
```

#### 更新后端
```http
PUT /api/v1/backends/update
Content-Type: application/json

{
  "upstream": "default",
  "backend": {
    "id": "backend1",
    "weight": 200
  }
}
```

#### 断开后端连接
```http
POST /api/v1/backends/disconnect?upstream=default&backend_id=backend1
```

### 监控

#### 获取服务器性能统计
```http
GET /api/v1/stats/server
```

#### 获取后端性能统计
```http
GET /api/v1/stats/backend?upstream=default&backend_id=backend1
```

#### 上报性能数据
```http
POST /api/v1/report
Content-Type: application/json

{
  "upstream": "default",
  "backend_id": "backend1",
  "performance": {
    "cpu_usage": 45.2,
    "memory_usage": 67.8,
    "load_avg_1": 2.1
  }
}
```

## 架构特点

### 高性能设计
- 基于fasthttp框架，性能优于标准库
- 支持数万个并发连接
- 优化的内存使用和GC压力

### 安全性
- SSL/TLS证书支持
- 真实IP获取和可信代理验证
- 请求头清理和安全检查

### 可扩展性
- 插件式的负载均衡器设计
- 动态配置热更新
- 模块化的架构设计

## 部署建议

### 系统要求
- Go 1.19+
- Linux/macOS/Windows
- 至少1GB RAM
- 支持数千并发连接

### 生产部署
1. 配置SSL证书
2. 设置适当的超时时间
3. 配置健康检查
4. 监控性能指标
5. 设置日志轮转

### 监控建议
- 使用管理API定期收集性能数据
- 监控后端服务器健康状态
- 设置告警阈值
- 日志分析和异常检测

## 许可证

MIT License

## 贡献

欢迎提交Issue和Pull Request！