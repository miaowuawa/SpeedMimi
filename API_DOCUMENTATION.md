# SpeedMimi 反向代理服务器 API 文档

## 概述

SpeedMimi 是一个高性能的反向代理服务器，提供 RESTful API 用于配置管理、后端服务管理和监控功能。所有 API 都使用 JSON 格式进行请求和响应。

**基础 URL**: `http://localhost:9091` (默认配置，可通过配置文件修改)

**API 版本**: v1

**认证**: 目前无认证机制，请在生产环境中添加适当的安全措施

## API 端点概览

| 分类 | 端点 | 方法 | 描述 |
|------|------|------|------|
| 配置管理 | `/api/v1/config` | GET, PUT | 获取和更新服务器配置 |
| 配置管理 | `/api/v1/config/reload-ssl` | POST | 重新加载 SSL 证书 |
| 后端管理 | `/api/v1/backends` | GET | 获取后端服务列表 |
| 后端管理 | `/api/v1/backends/add` | POST | 添加后端服务 (未实现) |
| 后端管理 | `/api/v1/backends/remove` | DELETE | 移除后端服务 (未实现) |
| 后端管理 | `/api/v1/backends/update` | PUT | 更新后端服务配置 |
| 后端管理 | `/api/v1/backends/disconnect` | POST | 异步断开后端连接 |
| 监控 | `/api/v1/stats/server` | GET | 获取服务器性能统计 |
| 监控 | `/api/v1/stats/backend` | GET | 获取后端性能统计 (模拟数据) |
| 监控 | `/api/v1/report` | POST | 上报后端性能数据 |

## 数据模型

### Backend (后端服务)

```json
{
  "id": "backend1",
  "name": "Backend Server 1",
  "host": "127.0.0.1",
  "port": 8081,
  "weight": 100,
  "scheme": "http",
  "active": true,
  "connections": 42,
  "max_conn": 1000,
  "health_check": {
    "path": "/health",
    "interval": "30s",
    "timeout": "5s",
    "failures": 3
  },
  "performance": {
    "cpu_usage": 25.5,
    "memory_usage": 60.2,
    "disk_usage": 15.1,
    "load_avg_1": 1.2,
    "load_avg_5": 1.1,
    "load_avg_15": 1.0,
    "network_in": 1024.5,
    "network_out": 2048.3,
    "timestamp": 1638360000000
  },
  "last_report": "2023-12-01T12:00:00Z"
}
```

### Config (服务器配置)

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080,
    "read_timeout": "30s",
    "write_timeout": "30s",
    "max_conn": 10000000,
    "real_ip_header": "X-Real-IP",
    "trusted_proxies": ["127.0.0.1/32", "10.0.0.0/8"]
  },
  "ssl": {
    "enabled": false,
    "cert_file": "certs/server.crt",
    "key_file": "certs/server.key"
  },
  "backends": {
    "default": [
      {
        "id": "backend1",
        "name": "Backend Server 1",
        "host": "127.0.0.1",
        "port": 8081,
        "weight": 100,
        "scheme": "http",
        "active": true,
        "max_conn": 1000,
        "health_check": {
          "path": "/health",
          "interval": "30s",
          "timeout": "5s",
          "failures": 3
        }
      }
    ]
  },
  "routing": {
    "default": {
      "path": "/",
      "upstream": "default",
      "load_balancer": "least_connections_weight",
      "protocols": {
        "websocket": "ip_hash",
        "sse": "ip_hash",
        "http": "least_connections_weight",
        "https": "least_connections_weight"
      }
    }
  },
  "grpc": {
    "enabled": true,
    "host": "127.0.0.1",
    "port": 9091
  }
}
```

### PerformanceInfo (性能信息)

```json
{
  "cpu_usage": 25.5,
  "memory_usage": 60.2,
  "disk_usage": 15.1,
  "load_avg_1": 1.2,
  "load_avg_5": 1.1,
  "load_avg_15": 1.0,
  "network_in": 1024.5,
  "network_out": 2048.3,
  "timestamp": 1638360000000
}
```

## API 详情

### 配置管理

#### 获取服务器配置

**接口**: `GET /api/v1/config`

**描述**: 获取当前服务器的完整配置信息

**响应示例**:
```json
{
  "config": {
    "server": {...},
    "ssl": {...},
    "backends": {...},
    "routing": {...},
    "grpc": {...}
  }
}
```

**状态码**:
- `200`: 成功
- `500`: 服务器内部错误

#### 更新服务器配置

**接口**: `PUT /api/v1/config`

**描述**: 更新服务器配置，会触发配置重载

**请求体**:
```json
{
  "config": {
    "server": {...},
    "ssl": {...},
    "backends": {...},
    "routing": {...},
    "grpc": {...}
  }
}
```

**响应示例**:
```json
{
  "success": true,
  "message": "Configuration updated successfully"
}
```

**状态码**:
- `200`: 成功
- `400`: 请求体格式错误
- `500`: 配置更新失败

#### 重新加载 SSL 证书

**接口**: `POST /api/v1/config/reload-ssl`

**描述**: 重新加载 SSL 证书文件，无需重启服务

**响应示例**:
```json
{
  "success": true,
  "message": "SSL certificates reloaded successfully"
}
```

**状态码**:
- `200`: 成功
- `500`: SSL 重新加载失败

### 后端管理

#### 获取后端服务列表

**接口**: `GET /api/v1/backends?upstream={upstream_id}`

**描述**: 获取指定上游服务的所有后端列表

**查询参数**:
- `upstream` (必需): 上游服务 ID

**请求示例**:
```
GET /api/v1/backends?upstream=default
```

**响应示例**:
```json
{
  "backends": [
    {
      "id": "backend1",
      "name": "Backend Server 1",
      "host": "127.0.0.1",
      "port": 8081,
      "weight": 100,
      "scheme": "http",
      "active": true,
      "connections": 42,
      "max_conn": 1000,
      "health_check": {...},
      "performance": {...},
      "last_report": "2023-12-01T12:00:00Z"
    }
  ]
}
```

**状态码**:
- `200`: 成功
- `400`: 缺少 upstream 参数
- `404`: 上游服务不存在

#### 添加后端服务

**接口**: `POST /api/v1/backends/add`

**描述**: 添加新的后端服务到指定上游

**状态**: 未实现

**响应示例**:
```json
{
  "success": false,
  "message": "Not implemented yet"
}
```

**状态码**:
- `200`: 响应成功 (功能未实现)

#### 移除后端服务

**接口**: `DELETE /api/v1/backends/remove`

**描述**: 从上游服务中移除指定的后端

**状态**: 未实现

**响应示例**:
```json
{
  "success": false,
  "message": "Not implemented yet"
}
```

**状态码**:
- `200`: 响应成功 (功能未实现)

#### 更新后端服务配置

**接口**: `PUT /api/v1/backends/update`

**描述**: 更新指定后端的配置参数，目前支持更新 `max_conn` (最大连接数)

**请求体**:
```json
{
  "upstream_id": "default",
  "backend_id": "backend1",
  "max_conn": 500
}
```

**请求参数**:
- `upstream_id` (必需): 上游服务 ID
- `backend_id` (必需): 后端服务 ID
- `max_conn` (必需): 新的最大连接数限制

**响应示例**:
```json
{
  "success": true,
  "message": "Backend updated successfully"
}
```

**状态码**:
- `200`: 成功
- `400`: 请求参数错误或请求体格式错误
- `404`: 上游服务或后端服务不存在

#### 异步断开后端连接

**接口**: `POST /api/v1/backends/disconnect`

**描述**: 异步标记指定后端为断开状态，负载均衡器将不再向该后端转发新请求

**请求体**:
```json
{
  "upstream_id": "default",
  "backend_id": "backend1"
}
```

**请求参数**:
- `upstream_id` (必需): 上游服务 ID
- `backend_id` (必需): 后端服务 ID

**响应示例**:
```json
{
  "success": true,
  "message": "Backend disconnect request accepted"
}
```

**注意**: 此操作是异步的，立即返回成功响应，实际断开操作在后台进行

**状态码**:
- `200`: 请求已接受
- `400`: 请求参数错误或请求体格式错误

### 监控

#### 获取服务器性能统计

**接口**: `GET /api/v1/stats/server`

**描述**: 获取服务器的实时性能统计信息

**响应示例**:
```json
{
  "stats": {
    "cpu_usage": 25.5,
    "memory_usage": 60.2,
    "disk_usage": 15.1,
    "load_avg_1": 1.2,
    "load_avg_5": 1.1,
    "load_avg_15": 1.0,
    "network_in": 1024.5,
    "network_out": 2048.3,
    "timestamp": 1638360000000
  }
}
```

**状态码**:
- `200`: 成功
- `500`: 获取统计信息失败

#### 获取后端性能统计

**接口**: `GET /api/v1/stats/backend`

**描述**: 获取后端服务的性能统计信息

**注意**: 当前返回模拟数据，实际实现需要从后端收集真实性能数据

**响应示例**:
```json
{
  "stats": {
    "cpu_usage": 0,
    "memory_usage": 0,
    "disk_usage": 0,
    "load_avg_1": 0,
    "load_avg_5": 0,
    "load_avg_15": 0,
    "network_in": 0,
    "network_out": 0,
    "timestamp": 0
  }
}
```

**状态码**:
- `200`: 成功

#### 上报后端性能数据

**接口**: `POST /api/v1/report`

**描述**: 后端服务上报性能数据给代理服务器

**请求体**:
```json
{
  "upstream": "default",
  "backend_id": "backend1",
  "performance": {
    "cpu_usage": 25.5,
    "memory_usage": 60.2,
    "disk_usage": 15.1,
    "load_avg_1": 1.2,
    "load_avg_5": 1.1,
    "load_avg_15": 1.0,
    "network_in": 1024.5,
    "network_out": 2048.3,
    "timestamp": 1638360000000
  }
}
```

**请求参数**:
- `upstream` (必需): 上游服务 ID
- `backend_id` (必需): 后端服务 ID
- `performance` (必需): 性能信息对象

**响应示例**:
```json
{
  "success": true,
  "message": "Performance data accepted"
}
```

**注意**: 此操作是异步的，立即返回成功响应，性能数据在后台处理

**状态码**:
- `200`: 数据已接受
- `400`: 请求体格式错误

## 使用示例

### cURL 示例

#### 获取服务器配置
```bash
curl -X GET http://localhost:9091/api/v1/config
```

#### 更新后端连接限制
```bash
curl -X PUT http://localhost:9091/api/v1/backends/update \
  -H "Content-Type: application/json" \
  -d '{
    "upstream_id": "default",
    "backend_id": "backend1",
    "max_conn": 500
  }'
```

#### 断开后端连接
```bash
curl -X POST http://localhost:9091/api/v1/backends/disconnect \
  -H "Content-Type: application/json" \
  -d '{
    "upstream_id": "default",
    "backend_id": "backend1"
  }'
```

#### 获取服务器统计
```bash
curl -X GET http://localhost:9091/api/v1/stats/server
```

### JavaScript/Node.js 示例

```javascript
const API_BASE = 'http://localhost:9091/api/v1';

// 获取后端列表
async function getBackends(upstreamId) {
  const response = await fetch(`${API_BASE}/backends?upstream=${upstreamId}`);
  const data = await response.json();
  return data.backends;
}

// 更新连接限制
async function updateMaxConn(upstreamId, backendId, maxConn) {
  const response = await fetch(`${API_BASE}/backends/update`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      upstream_id: upstreamId,
      backend_id: backendId,
      max_conn: maxConn
    })
  });
  const result = await response.json();
  return result;
}

// 断开后端
async function disconnectBackend(upstreamId, backendId) {
  const response = await fetch(`${API_BASE}/backends/disconnect`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      upstream_id: upstreamId,
      backend_id: backendId
    })
  });
  const result = await response.json();
  return result;
}
```

## 错误处理

所有 API 错误响应都遵循以下格式：

```json
{
  "error": "错误描述信息"
}
```

或者对于某些操作：

```json
{
  "success": false,
  "message": "错误描述信息"
}
```

## 性能考虑

- API 使用异步处理，避免阻塞主线程
- 监控数据获取是非阻塞的
- 配置更新会触发内部重载，可能影响性能

## 安全注意事项

⚠️ **重要**: 当前 API 没有任何认证或授权机制。在生产环境中，请务必添加：

1. API 密钥认证
2. HTTPS/TLS 加密
3. 访问控制列表 (ACL)
4. 请求频率限制
5. 输入验证和清理

## 版本历史

- **v1.0.0**: 初始版本，支持基础的配置管理和后端管理功能
- **v1.1.0**: 添加连接数限制功能，支持动态调整后端最大连接数
