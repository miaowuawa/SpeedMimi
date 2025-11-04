#!/bin/bash

# SpeedMimi 调试测试

echo "🔍 SpeedMimi 调试测试"
echo "======================"

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 步骤1: 构建程序
echo -e "${YELLOW}1. 构建程序${NC}"
if make build > /dev/null 2>&1; then
    echo -e "${GREEN}✓ 构建成功${NC}"
else
    echo -e "${RED}✗ 构建失败${NC}"
    exit 1
fi

# 步骤2: 启动后端服务器
echo -e "${YELLOW}2. 启动后端服务器${NC}"

cd test
echo "启动 Backend-1 (8081)..."
go run backend_server.go 8081 "Backend-1" > backend1.log 2>&1 &
BACKEND1_PID=$!
sleep 1

if kill -0 $BACKEND1_PID 2>/dev/null; then
    echo -e "${GREEN}✓ Backend-1 进程启动 (PID: $BACKEND1_PID)${NC}"
else
    echo -e "${RED}✗ Backend-1 启动失败${NC}"
    cat backend1.log
    exit 1
fi

echo "启动 Backend-2 (8082)..."
go run backend_server.go 8082 "Backend-2" > backend2.log 2>&1 &
BACKEND2_PID=$!
sleep 1

if kill -0 $BACKEND2_PID 2>/dev/null; then
    echo -e "${GREEN}✓ Backend-2 进程启动 (PID: $BACKEND2_PID)${NC}"
else
    echo -e "${RED}✗ Backend-2 启动失败${NC}"
    cat backend2.log
    exit 1
fi

cd ..

# 步骤3: 测试后端服务器
echo -e "${YELLOW}3. 测试后端服务器${NC}"

echo "测试 Backend-1 健康检查:"
response1=$(curl -s -w "HTTPSTATUS:%{http_code};" http://localhost:8081/health)
http_code1=$(echo $response1 | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')
body1=$(echo $response1 | sed -e 's/HTTPSTATUS:.*//g')

if [ "$http_code1" = "200" ]; then
    echo -e "${GREEN}✓ Backend-1 健康检查通过${NC}"
    echo "  响应: $body1"
else
    echo -e "${RED}✗ Backend-1 健康检查失败 (HTTP $http_code1)${NC}"
    echo "  响应: $body1"
fi

echo "测试 Backend-1 主页面:"
response1_main=$(curl -s -w "HTTPSTATUS:%{http_code};" http://localhost:8081/)
http_code1_main=$(echo $response1_main | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')
body1_main=$(echo $response1_main | sed -e 's/HTTPSTATUS:.*//g')

if [ "$http_code1_main" = "200" ]; then
    echo -e "${GREEN}✓ Backend-1 主页面正常${NC}"
    echo "  响应长度: ${#body1_main} 字符"
    echo "  响应预览: ${body1_main:0:100}..."
else
    echo -e "${RED}✗ Backend-1 主页面失败 (HTTP $http_code1_main)${NC}"
    echo "  响应: $body1_main"
fi

# 步骤4: 启动代理服务器
echo -e "${YELLOW}4. 启动代理服务器${NC}"

echo "启动 SpeedMimi..."
./bin/speedmimi -config configs/config.yaml > proxy.log 2>&1 &
PROXY_PID=$!
sleep 2

if kill -0 $PROXY_PID 2>/dev/null; then
    echo -e "${GREEN}✓ 代理服务器启动 (PID: $PROXY_PID)${NC}"
else
    echo -e "${RED}✗ 代理服务器启动失败${NC}"
    echo "代理日志:"
    cat proxy.log
    # 清理后端进程
    kill $BACKEND1_PID $BACKEND2_PID 2>/dev/null || true
    exit 1
fi

# 步骤5: 测试代理转发
echo -e "${YELLOW}5. 测试代理转发${NC}"

echo "测试代理转发 (5个请求):"
for i in {1..5}; do
    echo "请求 $i:"

    # 使用详细的curl测试
    response=$(curl -s -w "HTTPSTATUS:%{http_code};TIME:%{time_total};" http://localhost:8080/ 2>&1)
    http_code=$(echo $response | tr -d '\n' | sed -e 's/.*HTTPSTATUS://' | sed -e 's/;TIME:.*//')
    time_taken=$(echo $response | tr -d '\n' | sed -e 's/.*;TIME://')
    body=$(echo $response | sed -e 's/HTTPSTATUS:.*//g')

    if [ "$http_code" = "200" ]; then
        echo -e "  ${GREEN}✓ HTTP 200, 耗时: ${time_taken}s${NC}"

        # 解析JSON响应
        if echo "$body" | grep -q '"server":'; then
            server=$(echo "$body" | grep -o '"server":"[^"]*"' | cut -d'"' -f4)
            port=$(echo "$body" | grep -o '"port":"[^"]*"' | cut -d'"' -f4)
            echo "  路由到: $server (端口: $port)"
        else
            echo "  响应内容: ${body:0:50}..."
        fi
    else
        echo -e "  ${RED}✗ HTTP $http_code, 耗时: ${time_taken}s${NC}"
        echo "  响应内容: ${body:0:100}..."
    fi

    echo ""
done

# 步骤6: 测试管理API
echo -e "${YELLOW}6. 测试管理API${NC}"

echo "测试配置API:"
config_response=$(curl -s -w "HTTPSTATUS:%{http_code};" http://localhost:9091/api/v1/config)
config_code=$(echo $config_response | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')

if [ "$config_code" = "200" ]; then
    echo -e "${GREEN}✓ 配置API正常${NC}"
else
    echo -e "${RED}✗ 配置API失败 (HTTP $config_code)${NC}"
fi

# 步骤7: 清理
echo -e "${YELLOW}7. 清理进程${NC}"
kill $PROXY_PID $BACKEND1_PID $BACKEND2_PID 2>/dev/null || true
wait $PROXY_PID $BACKEND1_PID $BACKEND2_PID 2>/dev/null || true
echo -e "${GREEN}✓ 清理完成${NC}"

# 步骤8: 分析结果
echo ""
echo -e "${BLUE}=== 测试结果分析 ===${NC}"

if [ "$http_code1" = "200" ] && [ "$http_code1_main" = "200" ]; then
    echo -e "${GREEN}✓ 后端服务器工作正常${NC}"
else
    echo -e "${RED}✗ 后端服务器存在问题${NC}"
fi

if [ "$config_code" = "200" ]; then
    echo -e "${GREEN}✓ 管理API工作正常${NC}"
else
    echo -e "${RED}✗ 管理API存在问题${NC}"
fi

echo ""
echo "日志文件位置:"
echo "  Backend-1: test/backend1.log"
echo "  Backend-2: test/backend2.log"
echo "  Proxy: proxy.log"

echo -e "${GREEN}调试测试完成!${NC}"