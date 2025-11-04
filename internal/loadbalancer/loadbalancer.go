package loadbalancer

import (
	"hash/fnv"
	"math"
	"net"
	"sort"
	"sync"

	"github.com/quqi/speedmimi/pkg/types"
)

// IPHashBalancer IP Hash负载均衡器
type IPHashBalancer struct{}

func (b *IPHashBalancer) Name() string {
	return "ip_hash"
}

func (b *IPHashBalancer) SelectBackend(backends []*types.Backend, req interface{}) *types.Backend {
	if len(backends) == 0 {
		return nil
	}

	// 获取客户端IP
	clientIP := b.getClientIP(req)
	if clientIP == "" {
		// 如果无法获取IP，使用随机选择
		return b.selectRandom(backends)
	}

	// 使用IP的hash值选择后端
	hash := b.hashIP(clientIP)
	index := int(hash) % len(backends)

	return backends[index]
}

func (b *IPHashBalancer) getClientIP(req interface{}) string {
	// 这里需要根据实际的请求类型来获取IP
	// 暂时返回空字符串，具体实现会在代理层处理
	return ""
}

func (b *IPHashBalancer) hashIP(ip string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(ip))
	return h.Sum32()
}

func (b *IPHashBalancer) selectRandom(backends []*types.Backend) *types.Backend {
	minConn := int64(math.MaxInt64)
	var selected *types.Backend

	for _, backend := range backends {
		if !backend.IsActive() {
			continue
		}
		if backend.GetConnections() < minConn {
			minConn = backend.GetConnections()
			selected = backend
		}
	}

	return selected
}

// LeastConnectionsBalancer 最少连接数负载均衡器
type LeastConnectionsBalancer struct{}

func (b *LeastConnectionsBalancer) Name() string {
	return "least_connections"
}

func (b *LeastConnectionsBalancer) SelectBackend(backends []*types.Backend, req interface{}) *types.Backend {
	if len(backends) == 0 {
		return nil
	}

	minConn := int64(math.MaxInt64)
	var selected *types.Backend

	for _, backend := range backends {
		if !backend.IsActive() {
			continue
		}
		if backend.GetConnections() < minConn {
			minConn = backend.GetConnections()
			selected = backend
		}
	}

	return selected
}

// LeastConnectionsWeightBalancer 最少连接数+权重负载均衡器
type LeastConnectionsWeightBalancer struct{}

func (b *LeastConnectionsWeightBalancer) Name() string {
	return "least_connections_weight"
}

func (b *LeastConnectionsWeightBalancer) SelectBackend(backends []*types.Backend, req interface{}) *types.Backend {
	if len(backends) == 0 {
		return nil
	}

	// 计算每个后端的得分 (连接数/权重)
	type backendScore struct {
		backend *types.Backend
		score   float64
	}

	var candidates []backendScore
	totalWeight := 0

	for _, backend := range backends {
		if !backend.IsActive() {
			continue
		}

		weight := backend.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight

		connections := backend.GetConnections()
		score := float64(connections) / float64(weight)
		candidates = append(candidates, backendScore{backend, score})
	}

	if len(candidates) == 0 {
		return nil
	}

	// 选择得分最低的
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score < candidates[j].score
	})

	return candidates[0].backend
}

// WeightBalancer 权重负载均衡器
type WeightBalancer struct{}

func (b *WeightBalancer) Name() string {
	return "weight"
}

func (b *WeightBalancer) SelectBackend(backends []*types.Backend, req interface{}) *types.Backend {
	if len(backends) == 0 {
		return nil
	}

	totalWeight := 0
	for _, backend := range backends {
		if backend.IsActive() {
			totalWeight += backend.Weight
		}
	}

	if totalWeight == 0 {
		return nil
	}

	// 使用简单的轮询权重算法
	// 这里可以优化为更高效的实现
	r := 0 // 可以使用随机数或计数器
	currentWeight := 0

	for _, backend := range backends {
		if !backend.IsActive() {
			continue
		}

		currentWeight += backend.Weight
		if r < currentWeight {
			return backend
		}
	}

	return backends[0]
}

// PerformanceLCWBalancer 性能+最少连接数+权重负载均衡器
type PerformanceLCWBalancer struct{}

func (b *PerformanceLCWBalancer) Name() string {
	return "performance_least_connections_weight"
}

func (b *PerformanceLCWBalancer) SelectBackend(backends []*types.Backend, req interface{}) *types.Backend {
	if len(backends) == 0 {
		return nil
	}

	type backendScore struct {
		backend *types.Backend
		score   float64
	}

	var candidates []backendScore

	for _, backend := range backends {
		if !backend.IsActive() {
			continue
		}

		// 计算综合得分
		score := b.calculateScore(backend)
		candidates = append(candidates, backendScore{backend, score})
	}

	if len(candidates) == 0 {
		return nil
	}

	// 选择得分最低的（得分越低越好）
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score < candidates[j].score
	})

	return candidates[0].backend
}

func (b *PerformanceLCWBalancer) calculateScore(backend *types.Backend) float64 {
	connections := backend.GetConnections()
	weight := float64(backend.Weight)
	if weight <= 0 {
		weight = 1
	}

	utilization := backend.CalculateUtilization()

	// 综合得分 = (连接数/权重) + 占用率权重
	connectionScore := float64(connections) / weight
	performanceScore := utilization * 100 // 占用率转换为0-100分

	// 连接数权重70%，性能权重30%
	return connectionScore*0.7 + performanceScore*0.3
}

// Factory 负载均衡器工厂
type Factory struct {
	balancers map[types.LoadBalancerType]types.LoadBalancer
	mu        sync.RWMutex
}

func NewFactory() *Factory {
	f := &Factory{
		balancers: make(map[types.LoadBalancerType]types.LoadBalancer),
	}

	// 注册所有负载均衡器
	f.Register(types.IPHash, &IPHashBalancer{})
	f.Register(types.LeastConnections, &LeastConnectionsBalancer{})
	f.Register(types.LeastConnectionsWeight, &LeastConnectionsWeightBalancer{})
	f.Register(types.Weight, &WeightBalancer{})
	f.Register(types.PerformanceLCW, &PerformanceLCWBalancer{})

	return f
}

func (f *Factory) Register(lbType types.LoadBalancerType, balancer types.LoadBalancer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.balancers[lbType] = balancer
}

func (f *Factory) GetBalancer(lbType types.LoadBalancerType) types.LoadBalancer {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.balancers[lbType]
}

// GetClientIP 获取客户端真实IP
func GetClientIP(req interface{}, realIPHeader string, trustedProxies []string) string {
	// 这里需要根据fasthttp的RequestCtx来实现
	// 暂时提供一个基础实现
	return ""
}

// ParseCIDR 解析CIDR
func ParseCIDR(cidr string) *net.IPNet {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil
	}
	return ipNet
}

// IsTrustedProxy 检查是否为可信代理
func IsTrustedProxy(ip string, trustedProxies []string) bool {
	clientIP := net.ParseIP(ip)
	if clientIP == nil {
		return false
	}

	for _, cidr := range trustedProxies {
		if ipNet := ParseCIDR(cidr); ipNet != nil {
			if ipNet.Contains(clientIP) {
				return true
			}
		}
	}
	return false
}
