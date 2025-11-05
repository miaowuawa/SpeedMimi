package loadbalancer

import (
	"hash/fnv"
	"math"
	"math/rand"
	"net"
	"sort"
	"time"

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

	// 过滤出未达到连接限制的后端
	var availableBackends []*types.Backend
	for _, backend := range backends {
		if !backend.IsConnectionLimitReached() {
			availableBackends = append(availableBackends, backend)
		}
	}

	if len(availableBackends) == 0 {
		return nil // 所有后端都达到连接限制
	}

	// 获取客户端IP
	clientIP := b.getClientIP(req)
	if clientIP == "" {
		// 如果无法获取IP，使用随机选择
		return b.selectRandom(availableBackends)
	}

	// 使用IP的hash值选择后端
	hash := b.hashIP(clientIP)
	index := int(hash) % len(availableBackends)

	return availableBackends[index]
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
		if !backend.IsActive() || backend.ShouldDisconnect() || backend.IsConnectionLimitReached() {
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

	minConn := int64(math.MaxInt64)
	var selected *types.Backend

	for _, backend := range availableBackends {
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

	for _, backend := range backends {
		if !backend.IsActive() || backend.ShouldDisconnect() || backend.IsConnectionLimitReached() {
			continue
		}

		weight := backend.Weight
		if weight <= 0 {
			weight = 1
		}

		connections := backend.GetConnections()
		score := float64(connections) / float64(weight)
		candidates = append(candidates, backendScore{backend, score})
	}

	if len(candidates) == 0 {
		return nil // 所有后端都达到连接限制
	}

	// 选择得分最低的
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score < candidates[j].score
	})

	// 如果多个后端有相同的得分，随机选择一个
	minScore := candidates[0].score
	var sameScoreCandidates []backendScore

	for _, candidate := range candidates {
		if candidate.score == minScore {
			sameScoreCandidates = append(sameScoreCandidates, candidate)
		} else {
			break // 因为已经排序，后面的一定大于等于minScore
		}
	}

	// 在得分相同的后端中随机选择一个
	if len(sameScoreCandidates) == 1 {
		return sameScoreCandidates[0].backend
	}

	// 使用随机数生成器
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	index := r.Intn(len(sameScoreCandidates))
	return sameScoreCandidates[index].backend
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

	// 过滤出未达到连接限制的后端
	var availableBackends []*types.Backend
	totalWeight := 0
	for _, backend := range backends {
		if backend.IsActive() && !backend.ShouldDisconnect() && !backend.IsConnectionLimitReached() {
			availableBackends = append(availableBackends, backend)
			totalWeight += backend.Weight
		}
	}

	if len(availableBackends) == 0 {
		return nil // 所有后端都达到连接限制
	}

	if totalWeight == 0 {
		return nil
	}

	// 使用简单的轮询权重算法
	// 这里可以优化为更高效的实现
	r := 0 // 可以使用随机数或计数器
	currentWeight := 0

	for _, backend := range availableBackends {
		currentWeight += backend.Weight
		if r < currentWeight {
			return backend
		}
	}

	return availableBackends[0]
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
		if !backend.IsActive() || backend.ShouldDisconnect() || backend.IsConnectionLimitReached() {
			continue
		}

		// 计算综合得分
		score := b.calculateScore(backend)
		candidates = append(candidates, backendScore{backend, score})
	}

	if len(candidates) == 0 {
		return nil // 所有后端都达到连接限制
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

// 高性能负载均衡器工厂（无锁设计）
type Factory struct {
	balancers map[types.LoadBalancerType]types.LoadBalancer
}

func NewFactory() *Factory {
	f := &Factory{
		balancers: make(map[types.LoadBalancerType]types.LoadBalancer),
	}

	// 预分配负载均衡器实例，避免运行时分配
	f.balancers[types.IPHash] = &IPHashBalancer{}
	f.balancers[types.LeastConnections] = &LeastConnectionsBalancer{}
	f.balancers[types.LeastConnectionsWeight] = &LeastConnectionsWeightBalancer{}
	f.balancers[types.Weight] = &WeightBalancer{}
	f.balancers[types.PerformanceLCW] = &PerformanceLCWBalancer{}

	return f
}

func (f *Factory) GetBalancer(lbType types.LoadBalancerType) types.LoadBalancer {
	if balancer, exists := f.balancers[lbType]; exists {
		return balancer
	}
	return f.balancers[types.LeastConnectionsWeight] // 默认使用最少连接数+权重
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
