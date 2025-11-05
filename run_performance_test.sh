#!/bin/bash

# SpeedMimi 10,000并发性能测试 & 火焰图分析脚本
# ==============================================

set -e

echo "🚀 SpeedMimi 10,000并发性能测试 & 火焰图分析"
echo "============================================"

# 配置参数
SERVER_BINARY="./bin/speedmimi"
CONFIG_FILE="configs/config.yaml"
TEST_PROGRAM="./ten_thousand_concurrent_bench.go"
RESULTS_DIR="performance_results_$(date +%Y%m%d_%H%M%S)"
PPROF_PORT=6060

# 创建结果目录
mkdir -p "$RESULTS_DIR"
echo "结果将保存到目录: $RESULTS_DIR"

# 函数：检查依赖
check_dependencies() {
    echo "检查依赖..."

    if ! command -v go &> /dev/null; then
        echo "❌ Go 未安装"
        exit 1
    fi

    if ! command -v curl &> /dev/null; then
        echo "❌ curl 未安装"
        exit 1
    fi

    echo "✅ 依赖检查通过"
}

# 函数：构建服务器
build_server() {
    echo "构建服务器..."

    if [ ! -f "$SERVER_BINARY" ]; then
        make build
    fi

    if [ ! -f "$SERVER_BINARY" ]; then
        echo "❌ 服务器构建失败"
        exit 1
    fi

    echo "✅ 服务器构建完成"
}

# 函数：启动服务器
start_server() {
    echo "启动服务器..."

    # 启动服务器（后台运行）
    $SERVER_BINARY -config $CONFIG_FILE > "$RESULTS_DIR/server.log" 2>&1 &
    SERVER_PID=$!

    echo "服务器PID: $SERVER_PID"

    # 等待服务器启动
    echo "等待服务器启动..."
    for i in {1..30}; do
        if curl -s http://localhost:8080 > /dev/null 2>&1; then
            echo "✅ 服务器启动成功"
            return 0
        fi
        sleep 1
    done

    echo "❌ 服务器启动超时"
    kill $SERVER_PID 2>/dev/null || true
    exit 1
}

# 函数：检查pprof服务
check_pprof() {
    echo "检查pprof服务..."

    for i in {1..10}; do
        if curl -s http://localhost:$PPROF_PORT/debug/pprof/ > /dev/null 2>&1; then
            echo "✅ pprof服务可用"
            return 0
        fi
        sleep 1
    done

    echo "❌ pprof服务不可用"
    return 1
}

# 函数：运行性能测试
run_performance_test() {
    echo "运行10,000并发性能测试..."

    # 运行测试程序
    go run $TEST_PROGRAM > "$RESULTS_DIR/test_output.log" 2>&1

    if [ $? -ne 0 ]; then
        echo "❌ 性能测试失败"
        return 1
    fi

    echo "✅ 性能测试完成"

    # 移动profile文件到结果目录
    if [ -f "cpu_profile.prof" ]; then
        mv cpu_profile.prof "$RESULTS_DIR/"
    fi

    if [ -f "mem_profile.prof" ]; then
        mv mem_profile.prof "$RESULTS_DIR/"
    fi
}

# 函数：收集pprof数据
collect_pprof_data() {
    echo "收集pprof性能数据..."

    # 下载CPU profile
    if curl -s "http://localhost:$PPROF_PORT/debug/pprof/profile?seconds=60" -o "$RESULTS_DIR/cpu_profile_server.prof"; then
        echo "✅ CPU profile收集完成"
    else
        echo "❌ CPU profile收集失败"
    fi

    # 下载内存profile
    if curl -s "http://localhost:$PPROF_PORT/debug/pprof/heap" -o "$RESULTS_DIR/mem_profile_server.prof"; then
        echo "✅ 内存profile收集完成"
    else
        echo "❌ 内存profile收集失败"
    fi

    # 下载goroutine profile
    if curl -s "http://localhost:$PPROF_PORT/debug/pprof/goroutine" -o "$RESULTS_DIR/goroutine_profile.prof"; then
        echo "✅ Goroutine profile收集完成"
    else
        echo "❌ Goroutine profile收集失败"
    fi

    # 下载block profile
    if curl -s "http://localhost:$PPROF_PORT/debug/pprof/block" -o "$RESULTS_DIR/block_profile.prof"; then
        echo "✅ Block profile收集完成"
    else
        echo "❌ Block profile收集失败"
    fi
}

# 函数：生成火焰图
generate_flamegraphs() {
    echo "生成火焰图..."

    # 使用go tool pprof生成交互式火焰图
    if [ -f "$RESULTS_DIR/cpu_profile.prof" ]; then
        echo "生成CPU火焰图..."
        # 生成SVG格式的火焰图
        go tool pprof -svg "$RESULTS_DIR/cpu_profile.prof" > "$RESULTS_DIR/cpu_flamegraph.svg" 2>/dev/null
        if [ $? -eq 0 ]; then
            echo "✅ CPU火焰图生成完成: $RESULTS_DIR/cpu_flamegraph.svg"
        else
            echo "⚠️  CPU火焰图生成失败，使用文本模式"
            go tool pprof -text "$RESULTS_DIR/cpu_profile.prof" > "$RESULTS_DIR/cpu_profile.txt"
        fi
    fi

    if [ -f "$RESULTS_DIR/cpu_profile_server.prof" ]; then
        echo "生成服务器CPU火焰图..."
        go tool pprof -svg "$RESULTS_DIR/cpu_profile_server.prof" > "$RESULTS_DIR/cpu_flamegraph_server.svg" 2>/dev/null
        if [ $? -eq 0 ]; then
            echo "✅ 服务器CPU火焰图生成完成: $RESULTS_DIR/cpu_flamegraph_server.svg"
        else
            echo "⚠️  服务器CPU火焰图生成失败，使用文本模式"
            go tool pprof -text "$RESULTS_DIR/cpu_profile_server.prof" > "$RESULTS_DIR/cpu_profile_server.txt"
        fi
    fi

    # 生成内存火焰图
    if [ -f "$RESULTS_DIR/mem_profile.prof" ]; then
        echo "生成内存火焰图..."
        go tool pprof -svg "$RESULTS_DIR/mem_profile.prof" > "$RESULTS_DIR/mem_flamegraph.svg" 2>/dev/null
        if [ $? -eq 0 ]; then
            echo "✅ 内存火焰图生成完成: $RESULTS_DIR/mem_flamegraph.svg"
        else
            echo "⚠️  内存火焰图生成失败，使用文本模式"
            go tool pprof -text "$RESULTS_DIR/mem_profile.prof" > "$RESULTS_DIR/mem_profile.txt"
        fi
    fi

    # 生成goroutine分析
    if [ -f "$RESULTS_DIR/goroutine_profile.prof" ]; then
        echo "分析goroutine..."
        go tool pprof -text "$RESULTS_DIR/goroutine_profile.prof" > "$RESULTS_DIR/goroutine_analysis.txt"
        echo "✅ Goroutine分析完成: $RESULTS_DIR/goroutine_analysis.txt"
    fi
}

# 函数：分析结果
analyze_results() {
    echo "分析测试结果..."

    # 复制配置文件用于分析
    cp $CONFIG_FILE "$RESULTS_DIR/"

    # 创建分析报告
    cat > "$RESULTS_DIR/analysis_report.md" << EOF
# SpeedMimi 10,000并发性能测试报告

## 测试概况
- 测试时间: $(date)
- 并发数: 10,000
- 测试时长: 180秒
- 服务器配置: $CONFIG_FILE

## 测试结果
$(cat "$RESULTS_DIR/test_output.log" | grep -E "(平均RPS|平均延迟|内存使用|成功率|性能表现)" || echo "测试结果解析失败")

## 系统资源使用
$(tail -10 "$RESULTS_DIR/server.log" | grep "📊 System Metrics" || echo "无系统监控数据")

## 生成的文件
- CPU Profile (客户端): cpu_profile.prof
- 内存Profile (客户端): mem_profile.prof
- CPU Profile (服务器): cpu_profile_server.prof
- 内存Profile (服务器): mem_profile_server.prof
- Goroutine Profile: goroutine_profile.prof
- Block Profile: block_profile.prof
- CPU火焰图: cpu_flamegraph.svg
- 服务器CPU火焰图: cpu_flamegraph_server.svg

## 查看火焰图
\`\`\`bash
# 使用go tool pprof查看
go tool pprof -http=:8081 cpu_profile.prof
go tool pprof -http=:8082 mem_profile.prof

# 或直接打开SVG文件
open cpu_flamegraph.svg
open cpu_flamegraph_server.svg
\`\`\`

## 性能优化建议
1. 查看火焰图确定热点函数
2. 分析内存分配模式
3. 检查GC频率和暂停时间
4. 优化锁竞争和goroutine调度
EOF

    echo "✅ 分析报告生成: $RESULTS_DIR/analysis_report.md"
}

# 函数：清理
cleanup() {
    echo "清理测试环境..."

    # 停止服务器
    if [ ! -z "$SERVER_PID" ]; then
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
        echo "✅ 服务器已停止"
    fi

    # 清理临时文件
    rm -f cpu_profile.prof mem_profile.prof
}

# 主函数
main() {
    trap cleanup EXIT

    check_dependencies
    build_server
    start_server

    if check_pprof; then
        # 启动pprof数据收集（后台）
        collect_pprof_data &
        PPROF_PID=$!
    fi

    run_performance_test
    generate_flamegraphs
    analyze_results

    echo ""
    echo "🎉 性能测试完成!"
    echo "📁 结果目录: $RESULTS_DIR"
    echo ""
    echo "📊 查看分析报告:"
    echo "   cat $RESULTS_DIR/analysis_report.md"
    echo ""
    echo "🔥 查看火焰图:"
    echo "   open $RESULTS_DIR/cpu_flamegraph.svg 2>/dev/null || echo '火焰图生成失败，请检查go-torch安装'"
    echo "   open $RESULTS_DIR/cpu_flamegraph_server.svg 2>/dev/null || echo '服务器火焰图生成失败'"
    echo ""
    echo "🕵️  深入分析:"
    echo "   go tool pprof -http=:8081 $RESULTS_DIR/cpu_profile.prof 2>/dev/null || echo 'CPU profile不可用'"
    echo "   go tool pprof -http=:8082 $RESULTS_DIR/mem_profile.prof 2>/dev/null || echo '内存profile不可用'"
}

# 运行主函数
main "$@"
