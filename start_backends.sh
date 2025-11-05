#!/bin/bash

# 启动模拟后端服务器
echo "启动模拟后端服务器..."

# 启动第一个后端服务器 (端口8081)
echo "启动后端服务器1 (端口8081)..."
go run test/mock_backend.go 8081 > backend1.log 2>&1 &
BACKEND1_PID=$!
echo "后端1 PID: $BACKEND1_PID"

# 启动第二个后端服务器 (端口8082)
echo "启动后端服务器2 (端口8082)..."
go run test/mock_backend.go 8082 > backend2.log 2>&1 &
BACKEND2_PID=$!
echo "后端2 PID: $BACKEND2_PID"

# 等待服务器启动
sleep 2

# 检查服务器是否启动成功
if curl -s http://localhost:8081/health > /dev/null 2>&1; then
    echo "✅ 后端服务器1启动成功"
else
    echo "❌ 后端服务器1启动失败"
    kill $BACKEND1_PID 2>/dev/null || true
    kill $BACKEND2_PID 2>/dev/null || true
    exit 1
fi

if curl -s http://localhost:8082/health > /dev/null 2>&1; then
    echo "✅ 后端服务器2启动成功"
else
    echo "❌ 后端服务器2启动失败"
    kill $BACKEND1_PID 2>/dev/null || true
    kill $BACKEND2_PID 2>/dev/null || true
    exit 1
fi

echo "所有后端服务器启动完成!"
echo "后端1: http://localhost:8081"
echo "后端2: http://localhost:8082"
echo ""
echo "按Ctrl+C停止服务器"

# 等待中断信号
trap "echo '停止后端服务器...'; kill $BACKEND1_PID 2>/dev/null || true; kill $BACKEND2_PID 2>/dev/null || true; exit 0" INT TERM

wait

