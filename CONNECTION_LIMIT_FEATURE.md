# 连接数限制功能

## 功能概述

为反向代理服务器添加了连接数限制功能，支持限制每个后端服务器的最大并发连接数，当超过限制时自动将请求分发到其他后端，如果所有后端都达到限制则返回503错误。同时提供了通过API动态调整连接限制的功能。

## 功能特性

### 1. 连接数限制
- 为每个后端服务器配置最大连接数 (`max_conn`)
- 当 `max_conn` <= 0 时，表示无连接限制
- 使用原子操作确保连接计数的高并发安全性

### 2. 智能负载均衡
- 所有负载均衡算法都支持连接限制检查
- 当某个后端达到连接限制时，自动跳过该后端选择其他可用后端
- 支持的负载均衡算法：
  - IP Hash
  - 最少连接数
  - 最少连接数+权重
  - 权重
  - 性能+最少连接数+权重

### 3. 503错误处理
- 当所有后端都达到连接限制时，返回HTTP 503 Service Unavailable错误
- 错误信息包含"Service Unavailable (All backends at connection limit)"说明

### 4. API动态配置
- 通过HTTP API实时调整后端的连接限制
- 支持热更新，无需重启服务

## 配置方式

### 配置文件

在 `configs/config.yaml` 中为每个后端配置 `max_conn`：

```yaml
backends:
  default:
    - id: "backend1"
      name: "Backend Server 1"
      host: "127.0.0.1"
      port: 8081
      weight: 100
      scheme: "http"
      active: true
      max_conn: 1000  # 最大连接数
      health_check:
        path: "/health"
        interval: 30s
        timeout: 5s
        failures: 3
```

### API接口

#### 更新后端连接限制

**接口**: `PUT /api/v1/backends/update`

**请求体**:
```json
{
  "upstream_id": "default",
  "backend_id": "backend1",
  "max_conn": 500
}
```

**响应**:
```json
{
  "success": true,
  "message": "Backend updated successfully"
}
```

## 代码实现

### Backend结构扩展

```go
// Backend 后端服务器信息
type Backend struct {
    // ... 其他字段 ...
    MaxConn int `yaml:"max_conn" json:"max_conn"` // 最大连接数
    // ... 其他字段 ...
}

// IsConnectionLimitReached 检查是否达到连接数限制
func (b *Backend) IsConnectionLimitReached() bool {
    if b.MaxConn <= 0 {
        return false // 无限制
    }
    return b.GetConnections() >= int64(b.MaxConn)
}
```

### 负载均衡器更新

所有负载均衡器都添加了连接限制检查：

```go
// 过滤出未达到连接限制的后端
var availableBackends []*types.Backend
for _, backend := range backends {
    if backend.IsActive() && !backend.ShouldDisconnect() && !backend.IsConnectionLimitReached() {
        availableBackends = append(availableBackends, backend)
    }
}

if len(availableBackends) == 0 {
    return nil // 所有后端都达到连接限制
}
```

### API服务扩展

在 `internal/grpcservice/service.go` 中实现了 `handleUpdateBackend` 方法，支持更新 `max_conn` 参数。

## 使用示例

### 1. 启动服务器
```bash
go run cmd/server/main.go -config configs/config.yaml
```

### 2. 动态调整连接限制
```bash
curl -X PUT http://localhost:9091/api/v1/backends/update \
  -H "Content-Type: application/json" \
  -d '{
    "upstream_id": "default",
    "backend_id": "backend1",
    "max_conn": 200
  }'
```

### 3. 查看后端状态
```bash
curl http://localhost:9091/api/v1/backends?upstream=default
```

## 测试验证

运行测试验证连接限制功能：

```go
backend := &types.Backend{MaxConn: 2}
backend.IncConnections() // 连接数: 1, 未达到限制
backend.IncConnections() // 连接数: 2, 达到限制
backend.IsConnectionLimitReached() // 返回: true
```

## 性能影响

- 连接限制检查使用高效的原子操作，无锁竞争
- 负载均衡器只在选择后端时进行一次过滤，不会影响请求处理性能
- API更新是原子的，立即生效

## 注意事项

1. 连接限制是软限制，实际连接数可能在短时间内略微超过限制
2. 建议根据后端服务器的实际承载能力设置合理的 `max_conn` 值
3. 可以通过API动态调整连接限制，无需重启服务
4. 当所有后端都达到连接限制时，会返回503错误，确保服务稳定性
