#!/bin/bash

# SpeedMimi 系统调优脚本
# 用于支持千万级并发连接

set -e

echo "🔧 SpeedMimi 系统调优脚本"
echo "=========================="

# 检查是否为root用户
if [ "$EUID" -ne 0 ]; then
    echo "❌ 请使用root用户运行此脚本"
    exit 1
fi

echo "⚡ 调整内核参数以支持高并发..."

# 网络连接相关参数
cat << EOF > /etc/sysctl.d/99-speedmimi.conf
# SpeedMimi 高并发优化配置

# 网络连接优化
net.core.somaxconn = 65536
net.core.netdev_max_backlog = 500000
net.ipv4.tcp_max_syn_backlog = 1024000

# TCP连接优化
net.ipv4.ip_local_port_range = 1024 65535
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_tw_recycle = 0
net.ipv4.tcp_fin_timeout = 15
net.ipv4.tcp_keepalive_time = 300
net.ipv4.tcp_keepalive_probes = 3
net.ipv4.tcp_keepalive_intvl = 15

# 缓冲区优化
net.core.rmem_default = 262144
net.core.wmem_default = 262144
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216

# 文件描述符优化
fs.file-max = 10000000

# 进程和线程优化
kernel.pid_max = 1000000
kernel.threads-max = 1000000

# 内存优化
vm.max_map_count = 262144
vm.swappiness = 10

# 网络安全优化（减少检查开销）
net.ipv4.tcp_sack = 0
net.ipv4.tcp_dsack = 0
net.ipv4.tcp_fack = 0
net.ipv4.tcp_timestamps = 0
EOF

# 应用内核参数
sysctl -p /etc/sysctl.d/99-speedmimi.conf

# 设置文件描述符限制
cat << EOF > /etc/security/limits.d/speedmimi.conf
* soft nofile 10000000
* hard nofile 10000000
* soft nproc 1000000
* hard nproc 1000000
root soft nofile 10000000
root hard nofile 10000000
root soft nproc 1000000
root hard nproc 1000000
EOF

# 设置CPU性能模式（如果支持）
if command -v cpupower >/dev/null 2>&1; then
    cpupower frequency-set -g performance
    echo "✅ CPU已设置为性能模式"
fi

# 禁用透明大页（可能影响性能）
echo never > /sys/kernel/mm/transparent_hugepage/enabled 2>/dev/null || true
echo never > /sys/kernel/mm/transparent_hugepage/defrag 2>/dev/null || true

# 优化网络接口队列
for iface in $(ls /sys/class/net/ | grep -v lo); do
    # 设置多队列
    ethtool -L $iface combined 16 2>/dev/null || true
    # 启用RPS/RFS
    echo ffff > /sys/class/net/$iface/queues/rx-0/rps_cpus 2>/dev/null || true
    echo 4096 > /sys/class/net/$iface/queues/rx-0/rps_flow_cnt 2>/dev/null || true
done

echo ""
echo "✅ 系统调优完成！"
echo ""
echo "调优内容总结:"
echo "• 网络连接参数优化"
echo "• TCP连接优化"
echo "• 缓冲区大小调整"
echo "• 文件描述符限制提升"
echo "• 进程和线程限制调整"
echo "• 内存参数优化"
echo "• CPU性能模式设置"
echo "• 网络接口队列优化"
echo ""
echo "⚠️  注意：这些更改在重启后仍然有效"
echo "🔄 重启系统以应用所有更改: sudo reboot"
