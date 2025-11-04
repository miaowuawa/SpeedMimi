package types

import (
	"context"
	"crypto/tls"
	"sync/atomic"
	"time"
)

// LoadBalancerType 负载均衡类型
type LoadBalancerType string

const (
	IPHash               LoadBalancerType = "ip_hash"
	LeastConnections     LoadBalancerType = "least_connections"
	LeastConnectionsWeight LoadBalancerType = "least_connections_weight"
	Weight               LoadBalancerType = "weight"
	PerformanceLCW       LoadBalancerType = "performance_least_connections_weight"
)

// ProtocolType 协议类型
type ProtocolType string

const (
	HTTP      ProtocolType = "http"
	HTTPS     ProtocolType = "https"
	WebSocket ProtocolType = "websocket"
	SSE       ProtocolType = "sse"
)

// Backend 后端服务器信息（高并发优化版）
type Backend struct {
	ID           string            `yaml:"id" json:"id"`
	Name         string            `yaml:"name" json:"name"`
	Host         string            `yaml:"host" json:"host"`
	Port         int               `yaml:"port" json:"port"`
	Weight       int               `yaml:"weight" json:"weight"`
	Scheme       string            `yaml:"scheme" json:"scheme"`
	Active       bool              `yaml:"active" json:"active"`
	Connections  int64             `yaml:"-" json:"connections"`  // 当前连接数（原子操作）
	MaxConn      int               `yaml:"max_conn" json:"max_conn"`
	HealthCheck  *HealthCheck      `yaml:"health_check" json:"health_check"`
	Performance  *PerformanceInfo  `yaml:"-" json:"performance"`
	LastReport   time.Time         `yaml:"-" json:"last_report"`
	active       int32             `yaml:"-" json:"-"`           // 活跃状态（原子操作）
}

// PerformanceInfo 性能信息
type PerformanceInfo struct {
	CPUUsage    float64 `json:"cpu_usage"`    // CPU使用率 0-100
	MemoryUsage float64 `json:"memory_usage"` // 内存使用率 0-100
	DiskUsage   float64 `json:"disk_usage"`   // 磁盘使用率 0-100
	LoadAvg1    float64 `json:"load_avg_1"`   // 1分钟负载平均值
	LoadAvg5    float64 `json:"load_avg_5"`   // 5分钟负载平均值
	LoadAvg15   float64 `json:"load_avg_15"`  // 15分钟负载平均值
	NetworkIn   float64 `json:"network_in"`   // 网络流入速度 KB/s
	NetworkOut  float64 `json:"network_out"`  // 网络流出速度 KB/s
	Timestamp   int64   `json:"timestamp"`    // 时间戳
}

// HealthCheck 健康检查配置
type HealthCheck struct {
	Path     string        `yaml:"path" json:"path"`
	Interval time.Duration `yaml:"interval" json:"interval"`
	Timeout  time.Duration `yaml:"timeout" json:"timeout"`
	Failures int           `yaml:"failures" json:"failures"`
}

// Config 配置文件结构
type Config struct {
	Server   ServerConfig           `yaml:"server" json:"server"`
	SSL      SSLConfig              `yaml:"ssl" json:"ssl"`
	Backends map[string][]*Backend  `yaml:"backends" json:"backends"` // key为upstream名称
	Routing  map[string]*RoutingRule `yaml:"routing" json:"routing"`   // key为路径前缀
	GRPC     GRPCConfig             `yaml:"grpc" json:"grpc"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host         string            `yaml:"host" json:"host"`
	Port         int               `yaml:"port" json:"port"`
	ReadTimeout  time.Duration     `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration     `yaml:"write_timeout" json:"write_timeout"`
	MaxConn      int               `yaml:"max_conn" json:"max_conn"`
	RealIPHeader string            `yaml:"real_ip_header" json:"real_ip_header"`
	TrustedProxies []string        `yaml:"trusted_proxies" json:"trusted_proxies"`
}

// SSLConfig SSL配置
type SSLConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	CertFile string `yaml:"cert_file" json:"cert_file"`
	KeyFile  string `yaml:"key_file" json:"key_file"`
}

// RoutingRule 路由规则
type RoutingRule struct {
	Path         string           `yaml:"path" json:"path"`
	Upstream     string           `yaml:"upstream" json:"upstream"`
	LoadBalancer LoadBalancerType `yaml:"load_balancer" json:"load_balancer"`
	Protocols    map[ProtocolType]LoadBalancerType `yaml:"protocols" json:"protocols"` // 协议特定负载均衡
}

// GRPCConfig gRPC配置
type GRPCConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Host    string `yaml:"host" json:"host"`
	Port    int    `yaml:"port" json:"port"`
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	SelectBackend(backends []*Backend, req interface{}) *Backend
	Name() string
}

// ProxyRequest 代理请求接口
type ProxyRequest interface {
	GetHeader(key string) []byte
	GetMethod() string
	GetURI() *URI
	GetBody() []byte
	SetHeader(key, value string)
	SetBody(body []byte)
}

// URI URI接口
type URI interface {
	String() string
	Path() string
	QueryString() []byte
}

// GRPC Services

// ConfigService 配置服务
type ConfigService interface {
	UpdateConfig(ctx context.Context, config *Config) error
	GetConfig(ctx context.Context) (*Config, error)
	ReloadSSL(ctx context.Context) error
}

// BackendService 后端服务
type BackendService interface {
	GetBackends(ctx context.Context, upstream string) ([]*Backend, error)
	AddBackend(ctx context.Context, upstream string, backend *Backend) error
	RemoveBackend(ctx context.Context, upstream string, backendID string) error
	UpdateBackend(ctx context.Context, upstream string, backend *Backend) error
	DisconnectBackend(ctx context.Context, upstream string, backendID string) error
}

// MonitorService 监控服务
type MonitorService interface {
	GetServerStats(ctx context.Context) (*PerformanceInfo, error)
	GetBackendStats(ctx context.Context, upstream, backendID string) (*PerformanceInfo, error)
	ReportPerformance(ctx context.Context, upstream, backendID string, perf *PerformanceInfo) error
}

// 高性能Backend方法（使用原子操作，避免锁竞争）
func (b *Backend) GetConnections() int64 {
	return atomic.LoadInt64(&b.Connections)
}

func (b *Backend) IncConnections() {
	atomic.AddInt64(&b.Connections, 1)
}

func (b *Backend) DecConnections() {
	// 使用CAS操作确保不会减到负数
	for {
		current := atomic.LoadInt64(&b.Connections)
		if current <= 0 {
			return
		}
		if atomic.CompareAndSwapInt64(&b.Connections, current, current-1) {
			return
		}
		// CAS失败，重试
	}
}

func (b *Backend) SetConnections(conns int64) {
	atomic.StoreInt64(&b.Connections, conns)
}

func (b *Backend) IsActive() bool {
	return atomic.LoadInt32(&b.active) == 1
}

func (b *Backend) SetActive(active bool) {
	var val int32
	if active {
		val = 1
	}
	atomic.StoreInt32(&b.active, val)
	// 同步更新Active字段用于序列化
	b.Active = active
}

// 高并发优化：性能信息直接访问，无锁
func (b *Backend) UpdatePerformance(perf *PerformanceInfo) {
	b.Performance = perf
	b.LastReport = time.Now()
}

func (b *Backend) GetPerformance() *PerformanceInfo {
	return b.Performance
}

// CalculateUtilization 计算节点占用率 (0-1)
func (b *Backend) CalculateUtilization() float64 {
	perf := b.GetPerformance()
	if perf == nil {
		return 0
	}

	// 综合考虑CPU、内存、负载的占用率
	cpuWeight := 0.4
	memWeight := 0.4
	loadWeight := 0.2

	utilization := (perf.CPUUsage/100)*cpuWeight +
		(perf.MemoryUsage/100)*memWeight +
		(perf.LoadAvg1/100)*loadWeight // 假设load avg最大值为100

	if utilization > 1 {
		utilization = 1
	}

	return utilization
}

// TLSConfig TLS配置
type TLSConfig struct {
	Certificates []tls.Certificate
}
