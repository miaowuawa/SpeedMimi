#!/bin/bash

# SpeedMimi 自动化部署脚本
# 支持千万级并发优化部署

set -e

echo "🚀 SpeedMimi 高并发部署脚本"
echo "============================"

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 默认配置
PRODUCTION=false
TUNE_SYSTEM=false
BUILD_OPTIMIZED=false
ENABLE_PROFILING=false

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --production)
            PRODUCTION=true
            shift
            ;;
        --tune-system)
            TUNE_SYSTEM=true
            shift
            ;;
        --optimized)
            BUILD_OPTIMIZED=true
            shift
            ;;
        --profile)
            ENABLE_PROFILING=true
            shift
            ;;
        --help)
            echo "SpeedMimi 部署脚本"
            echo ""
            echo "用法: $0 [选项]"
            echo ""
            echo "选项:"
            echo "  --production    生产环境模式"
            echo "  --tune-system   执行系统调优（需要root权限）"
            echo "  --optimized     构建优化版本"
            echo "  --profile       启用性能分析"
            echo "  --help          显示此帮助信息"
            exit 0
            ;;
        *)
            echo -e "${RED}未知选项: $1${NC}"
            echo "使用 --help 查看帮助"
            exit 1
            ;;
    esac
done

# 检查环境
echo -e "${YELLOW}检查环境...${NC}"

# 检查Go
if ! command -v go >/dev/null 2>&1; then
    echo -e "${RED}错误: 未找到Go${NC}"
    exit 1
fi

# 检查root权限（如果需要调优系统）
if [ "$TUNE_SYSTEM" = true ] && [ "$EUID" -ne 0 ]; then
    echo -e "${RED}错误: 系统调优需要root权限${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 环境检查通过${NC}"

# 1. 代码检查和格式化
echo -e "${YELLOW}代码质量检查...${NC}"
go fmt ./...
go vet ./...
go mod tidy
echo -e "${GREEN}✓ 代码检查完成${NC}"

# 2. 构建
echo -e "${YELLOW}构建 SpeedMimi...${NC}"
if [ "$BUILD_OPTIMIZED" = true ] || [ "$PRODUCTION" = true ]; then
    echo "使用优化构建..."
    make build-prod
else
    make build
fi
echo -e "${GREEN}✓ 构建完成${NC}"

# 3. 系统调优
if [ "$TUNE_SYSTEM" = true ]; then
    echo -e "${YELLOW}执行系统调优...${NC}"
    if [ -f "tune_system.sh" ]; then
        ./tune_system.sh
        echo -e "${GREEN}✓ 系统调优完成${NC}"
    else
        echo -e "${RED}警告: 未找到系统调优脚本${NC}"
    fi
fi

# 4. 性能分析设置
if [ "$ENABLE_PROFILING" = true ]; then
    echo -e "${YELLOW}启用性能分析...${NC}"
    # 这里可以设置环境变量或配置文件来启用pprof
    export SPEEDMIMI_PROFILE=true
    echo -e "${GREEN}✓ 性能分析已启用${NC}"
fi

# 5. 创建必要的目录
echo -e "${YELLOW}创建运行目录...${NC}"
mkdir -p logs
mkdir -p pprof
echo -e "${GREEN}✓ 目录创建完成${NC}"

# 6. 配置检查
echo -e "${YELLOW}检查配置文件...${NC}"
if [ ! -f "configs/config.yaml" ]; then
    echo -e "${RED}错误: 未找到配置文件 configs/config.yaml${NC}"
    exit 1
fi

# 验证配置文件语法
if command -v yamllint >/dev/null 2>&1; then
    yamllint configs/config.yaml || echo -e "${YELLOW}警告: 配置文件格式可能有问题${NC}"
fi

echo -e "${GREEN}✓ 配置检查完成${NC}"

# 7. 部署信息
echo ""
echo -e "${BLUE}=== 部署信息 ===${NC}"
echo "构建模式: $(if [ "$BUILD_OPTIMIZED" = true ]; then echo "优化版"; else echo "标准版"; fi)"
echo "系统调优: $(if [ "$TUNE_SYSTEM" = true ]; then echo "已执行"; else echo "未执行"; fi)"
echo "性能分析: $(if [ "$ENABLE_PROFILING" = true ]; then echo "已启用"; else echo "未启用"; fi)"
echo "生产模式: $(if [ "$PRODUCTION" = true ]; then echo "是"; else echo "否"; fi)"

echo ""
echo -e "${GREEN}✅ SpeedMimi 部署完成！${NC}"
echo ""
echo "启动命令:"
echo "  ./bin/speedmimi -config configs/config.yaml"
echo ""
echo "测试命令:"
echo "  make test-million  # 千万级并发测试"
echo "  curl http://localhost:8080/  # 简单测试"
echo ""
echo "监控命令:"
if [ "$ENABLE_PROFILING" = true ]; then
    echo "  go tool pprof http://localhost:6060/debug/pprof/profile  # CPU分析"
    echo "  go tool pprof http://localhost:6060/debug/pprof/heap     # 内存分析"
fi
echo ""
echo -e "${YELLOW}注意: 建议在生产环境中运行时使用进程管理器如systemd或supervisor${NC}"

