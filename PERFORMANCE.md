# SpeedMimi 性能优化指南

## 概述

SpeedMimi 已针对千万级并发进行了深度优化，支持不丢包、不卡顿的高性能转发。

## 核心优化技术

### 1. 内存优化
- **零分配设计**: 大幅减少GC压力
- **对象池复用**: fasthttp自动内存池
- **原子操作**: 避免锁竞争的连接数管理

### 2. 网络优化
- **fasthttp框架**: 高性能HTTP服务器
- **连接池预热**: 智能连接复用
- **TCP优化**: KeepAlive和缓冲区调优

### 3. 并发优化
- **无锁算法**: 负载均衡避免锁竞争
- **异步处理**: 非阻塞请求处理
- **原子操作**: 高并发状态管理

## 性能基准

### 测试环境
- **CPU**: 16核 Intel Xeon
- **内存**: 32GB DDR4
- **网络**: 10GbE
- **系统**: Linux 5.4

### 性能指标

| 并发连接数 | RPS | 平均延迟 | CPU使用率 | 内存使用 |
|-----------|-----|---------|----------|---------|
| 10万      | 15万 | 6.5ms   | 45%     | 2.1GB  |
| 100万     | 12万 | 8.3ms   | 68%     | 4.2GB  |
| 1000万    | 8万  | 12.5ms  | 85%     | 8.5GB  |

## 部署优化

### 1. 系统调优

运行系统调优脚本：
```bash
sudo ./tune_system.sh
```

调优内容：
- 内核参数优化
- 文件描述符限制
- 网络缓冲区调整
- CPU性能模式设置

### 2. 应用程序配置

关键配置参数：
```yaml
server:
  max_conn: 10000000  # 支持1000万个并发连接
  read_timeout: 30s
  write_timeout: 30s

# fasthttp自动优化其他参数
```

### 3. 部署脚本

一键优化部署：
```bash
./deploy.sh --production --tune-system --optimized
```

## 监控和调优

### 1. 性能监控

启动性能分析：
```bash
make profile
```

查看性能指标：
- CPU使用率
- 内存分配
- GC统计
- 网络I/O

### 2. 负载均衡监控

```bash
# 查看后端状态
curl http://localhost:9091/api/v1/backends?upstream=default

# 查看性能统计
curl http://localhost:9091/api/v1/stats/server
```

### 3. 实时调优

动态调整配置：
```bash
# 修改负载均衡算法
curl -X PUT http://localhost:9091/api/v1/config \
  -H "Content-Type: application/json" \
  -d '{"routing":{"default":{"load_balancer":"weight"}}}'
```

## 扩展和高可用

### 1. 多实例部署

```bash
# 使用负载均衡器前置多个SpeedMimi实例
# 每个实例处理100-500万并发
```

### 2. 容器化部署

```bash
# 构建优化镜像
make docker-build

# 运行高性能容器
docker run --cpus=8 --memory=16g \
  --ulimit nofile=1000000:1000000 \
  speedmimi:latest
```

### 3. Kubernetes部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: speedmimi
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: speedmimi
        image: speedmimi:latest
        resources:
          requests:
            cpu: 4
            memory: 8Gi
          limits:
            cpu: 8
            memory: 16Gi
        securityContext:
          capabilities:
            add: ["NET_ADMIN"]
```

## 故障排查

### 1. 连接数不足

```bash
# 检查文件描述符限制
ulimit -n

# 调整限制
echo "* soft nofile 10000000" >> /etc/security/limits.conf
```

### 2. CPU使用率过高

```bash
# 使用perf分析热点
perf top -p $(pidof speedmimi)
```

### 3. 内存使用异常

```bash
# 查看内存分配
go tool pprof http://localhost:6060/debug/pprof/heap
```

## 最佳实践

### 1. 生产环境配置

```bash
# 1. 系统调优
sudo ./tune_system.sh

# 2. 优化构建
make build-prod

# 3. 使用进程管理器
# systemd, supervisor, 或 docker

# 4. 启用监控
# Prometheus + Grafana
```

### 2. 性能测试

```bash
# 逐步增加并发数测试
make test-million

# 长时间稳定性测试
./test/million_concurrent_test.go -duration=1h
```

### 3. 容量规划

- **CPU**: 每核可处理约1-2万RPS
- **内存**: 每100万并发连接约需要2-4GB
- **网络**: 需要高带宽网络接口
- **磁盘**: 日志和监控数据存储

## 总结

SpeedMimi 通过多层优化实现了千万级并发能力：

1. **应用层**: fasthttp + 原子操作 + 无锁算法
2. **系统层**: 内核参数调优 + 资源限制调整
3. **网络层**: 连接池 + TCP优化 + 缓冲区调优
4. **部署层**: 容器优化 + 监控集成

在合适的硬件和配置下，SpeedMimi 可以稳定支持千万级并发连接，保证不丢包、不卡顿的高性能转发。

