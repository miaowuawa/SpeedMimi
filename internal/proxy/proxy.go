package proxy

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/quqi/speedmimi/internal/config"
	"github.com/quqi/speedmimi/internal/loadbalancer"
	"github.com/quqi/speedmimi/internal/monitor"
	"github.com/quqi/speedmimi/pkg/types"
)

// Server 反向代理服务器
type Server struct {
	config         *config.Manager
	lbFactory      *loadbalancer.Factory
	upstreamMgr    *UpstreamManager
	monitor        *monitor.PerformanceMonitor
	server         *fasthttp.Server
	tlsConfig      *tls.Config
	mu             sync.RWMutex
}

// 高性能上游管理器（预分配和无锁优化）
type UpstreamManager struct {
	upstreams []*Upstream
	names     map[string]int // name -> index映射
}

type Upstream struct {
	name     string
	backends []*types.Backend
	lbType   types.LoadBalancerType
	balancer types.LoadBalancer
}

// NewServer 创建代理服务器
func NewServer(cfgMgr *config.Manager) (*Server, error) {
	lbFactory := loadbalancer.NewFactory()
	upstreamMgr := NewUpstreamManager()
	perfMonitor := monitor.NewPerformanceMonitor()

	server := &Server{
		config:      cfgMgr,
		lbFactory:   lbFactory,
		upstreamMgr: upstreamMgr,
		monitor:     perfMonitor,
	}

	// 初始化上游
	if err := server.initUpstreams(); err != nil {
		return nil, fmt.Errorf("failed to init upstreams: %w", err)
	}

	// 创建高性能fasthttp服务器配置（支持千万级并发）
	fasthttpServer := &fasthttp.Server{
		Handler:                       server.handleRequest,
		ReadTimeout:                   cfgMgr.GetConfig().Server.ReadTimeout,
		WriteTimeout:                  cfgMgr.GetConfig().Server.WriteTimeout,
		MaxConnsPerIP:                 0, // 不限制单IP连接数
		MaxRequestsPerConn:            0, // 不限制单连接请求数
		MaxKeepaliveDuration:          300 * time.Second, // 增加keepalive时间
		TCPKeepalive:                  true,
		TCPKeepalivePeriod:            30 * time.Second, // 减少keepalive周期
		ReduceMemoryUsage:             false, // 性能优先
		GetOnly:                       false,
		DisablePreParseMultipartForm: true,
		LogAllErrors:                  false,
		DisableHeaderNamesNormalizing: true,
		NoDefaultServerHeader:         true,
		NoDefaultDate:                 true,  // 禁用默认日期头以提高性能
		NoDefaultContentType:          true,
		KeepHijackedConns:             false,
		CloseOnShutdown:               true,
		StreamRequestBody:             true,
		MaxRequestBodySize:            4 * 1024 * 1024, // 4MB

		// 高并发优化配置
		SleepWhenConcurrencyLimitsExceeded: 0,
		Concurrency:                        10000000, // 支持1000万个并发连接

		// 内存池优化
		ReadBufferSize:  4096,  // 4KB读取缓冲区
		WriteBufferSize: 4096,  // 4KB写入缓冲区

		// 连接优化
		MaxIdleWorkerDuration: 60 * time.Second,

		// 错误处理优化
		ErrorHandler: func(ctx *fasthttp.RequestCtx, err error) {
			// 静默处理错误，避免日志输出影响性能
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		},
	}

	server.server = fasthttpServer

	// 监听配置变化
	go server.watchConfig()

	return server, nil
}

// Start 启动服务器
func (s *Server) Start() error {
	cfg := s.config.GetConfig()
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	if cfg.SSL.Enabled {
		if err := s.initTLS(); err != nil {
			return fmt.Errorf("failed to init TLS: %w", err)
		}
		return s.server.ListenAndServeTLS(addr, cfg.SSL.CertFile, cfg.SSL.KeyFile)
	}

	return s.server.ListenAndServe(addr)
}

// Stop 停止服务器
func (s *Server) Stop() error {
	if s.monitor != nil {
		s.monitor.Stop()
	}
	return s.server.Shutdown()
}

// GetMonitor 获取性能监控器
func (s *Server) GetMonitor() *monitor.PerformanceMonitor {
	return s.monitor
}

// DisconnectBackend 异步断开后端连接（标记机制）
func (s *Server) DisconnectBackend(upstreamID, backendID string) error {
	upstream := s.upstreamMgr.GetUpstream(upstreamID)
	if upstream == nil {
		return fmt.Errorf("upstream %s not found", upstreamID)
	}

	backends := upstream.GetBackends()
	for _, backend := range backends {
		if backend.ID == backendID {
			// 标记后端为断开状态
			backend.MarkForDisconnect()
			fmt.Printf("[DISCONNECT] Backend %s/%s marked for disconnection\n", upstreamID, backendID)
			return nil
		}
	}

	return fmt.Errorf("backend %s not found in upstream %s", backendID, upstreamID)
}

// GetUpstreamManager 获取上游管理器（用于调试）
func (s *Server) GetUpstreamManager() *UpstreamManager {
	return s.upstreamMgr
}

// handleRequest 处理请求
func (s *Server) handleRequest(ctx *fasthttp.RequestCtx) {
	// 轻量级性能监控记录（非阻塞）
	s.monitor.StartConnection()

	// 使用defer确保连接结束被记录
	defer func() {
		// 记录请求完成（异步，非阻塞）
		if s.monitor != nil {
			bytesSent := int64(len(ctx.Response.Body()))
			bytesRecv := int64(len(ctx.Request.Body()))
			s.monitor.RecordRequest(bytesSent, bytesRecv)
			s.monitor.EndConnection()
		}
	}()

	// 获取路由规则
	rule := s.findRoutingRule(string(ctx.Path()))
	if rule == nil {
		ctx.Error("Not Found", fasthttp.StatusNotFound)
		return
	}

	// 获取上游
	upstream := s.upstreamMgr.GetUpstream(rule.Upstream)
	if upstream == nil {
		ctx.Error("Service Unavailable", fasthttp.StatusServiceUnavailable)
		return
	}

	// 获取后端列表
	backends := upstream.GetBackends()
	if len(backends) == 0 {
		ctx.Error("Service Unavailable", fasthttp.StatusServiceUnavailable)
		return
	}

	// 确定负载均衡类型
	lbType := s.determineLBType(rule, ctx)
	balancer := s.lbFactory.GetBalancer(lbType)
	if balancer == nil {
		balancer = s.lbFactory.GetBalancer(types.LeastConnectionsWeight)
	}

	// 选择后端
	backend := balancer.SelectBackend(backends, ctx)
	if backend == nil {
		ctx.Error("Service Unavailable (All backends at connection limit)", fasthttp.StatusServiceUnavailable)
		return
	}

	// 代理请求
	s.proxyRequest(ctx, backend)
}

// proxyRequest 代理请求到后端
func (s *Server) proxyRequest(ctx *fasthttp.RequestCtx, backend *types.Backend) {
	// 增加连接数
	backend.IncConnections()
	defer backend.DecConnections()

	// 构建后端URL
	_ = fmt.Sprintf("%s://%s:%d", backend.Scheme, backend.Host, backend.Port)

	// 设置请求头
	s.setProxyHeaders(ctx, backend)

	// 创建高性能代理客户端（支持千万级并发）
	client := &fasthttp.Client{
		// 基础超时设置
		ReadTimeout:              30 * time.Second,
		WriteTimeout:             30 * time.Second,
		MaxConnDuration:          300 * time.Second, // 增加连接持续时间
		MaxConnWaitTimeout:       10 * time.Second,  // 减少等待超时
		MaxIdleConnDuration:      120 * time.Second, // 增加空闲连接时间

		// 高并发优化
		MaxConnsPerHost:     100000, // 每个主机最大连接数
		ReadBufferSize:      8192,   // 8KB读取缓冲区
		WriteBufferSize:     8192,   // 8KB写入缓冲区

		// 连接优化
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
		NoDefaultUserAgentHeader:      true,

		// 自定义拨号函数（高性能）
		Dial: func(addr string) (net.Conn, error) {
			return fasthttp.DialDualStackTimeout(addr, 3*time.Second)
		},

		// 连接重试策略
		RetryIf: func(req *fasthttp.Request) bool {
			// 只对GET请求重试，避免副作用
			return string(req.Header.Method()) == "GET"
		},
		MaxIdemponentCallAttempts: 2, // 最多重试2次
	}

	// 执行代理
	req := &ctx.Request
	resp := &ctx.Response

	if err := client.Do(req, resp); err != nil {
		ctx.Error("Bad Gateway", fasthttp.StatusBadGateway)
		return
	}
}

// setProxyHeaders 设置代理请求头
func (s *Server) setProxyHeaders(ctx *fasthttp.RequestCtx, backend *types.Backend) {
	cfg := s.config.GetConfig()

	// 添加或更新X-Forwarded-For
	clientIP := s.getClientIP(ctx)
	if existing := ctx.Request.Header.Peek("X-Forwarded-For"); len(existing) > 0 {
		ctx.Request.Header.Set("X-Forwarded-For", string(existing)+", "+clientIP)
	} else {
		ctx.Request.Header.Set("X-Forwarded-For", clientIP)
	}

	// 设置X-Real-IP
	if cfg.Server.RealIPHeader != "" {
		ctx.Request.Header.Set(cfg.Server.RealIPHeader, clientIP)
	}

	// 添加其他代理头
	ctx.Request.Header.Set("X-Forwarded-Proto", s.getProto(ctx))
	ctx.Request.Header.Set("X-Forwarded-Host", string(ctx.Host()))
}

// getClientIP 获取客户端真实IP
func (s *Server) getClientIP(ctx *fasthttp.RequestCtx) string {
	cfg := s.config.GetConfig()

	// 首先尝试从指定头获取
	if cfg.Server.RealIPHeader != "" {
		if ip := string(ctx.Request.Header.Peek(cfg.Server.RealIPHeader)); ip != "" {
			return ip
		}
	}

	// 尝试从X-Forwarded-For获取
	if xff := string(ctx.Request.Header.Peek("X-Forwarded-For")); xff != "" {
		// 取第一个IP
		if idx := strings.Index(xff, ","); idx > 0 {
			ip := strings.TrimSpace(xff[:idx])
			// 验证是否为可信代理
			if loadbalancer.IsTrustedProxy(ip, cfg.Server.TrustedProxies) {
				return ip
			}
		}
	}

	// 从连接获取
	return ctx.RemoteIP().String()
}

// getProto 获取协议
func (s *Server) getProto(ctx *fasthttp.RequestCtx) string {
	if ctx.IsTLS() {
		return "https"
	}
	return "http"
}

// findRoutingRule 查找路由规则
func (s *Server) findRoutingRule(path string) *types.RoutingRule {
	cfg := s.config.GetConfig()

	// 简单的路径匹配，可以优化为更高效的实现
	for _, rule := range cfg.Routing {
		if strings.HasPrefix(path, rule.Path) {
			return rule
		}
	}

	// 返回默认规则
	if defaultRule, exists := cfg.Routing["default"]; exists {
		return defaultRule
	}

	return nil
}

// determineLBType 确定负载均衡类型
func (s *Server) determineLBType(rule *types.RoutingRule, ctx *fasthttp.RequestCtx) types.LoadBalancerType {
	// 检查协议特定配置
	protocol := s.detectProtocol(ctx)
	if lbType, exists := rule.Protocols[protocol]; exists {
		return lbType
	}

	// 返回默认负载均衡类型
	return rule.LoadBalancer
}

// detectProtocol 检测协议类型
func (s *Server) detectProtocol(ctx *fasthttp.RequestCtx) types.ProtocolType {
	// 检查是否为WebSocket
	if string(ctx.Request.Header.Peek("Upgrade")) == "websocket" {
		return types.WebSocket
	}

	// 检查是否为SSE
	if string(ctx.Request.Header.Peek("Accept")) == "text/event-stream" {
		return types.SSE
	}

	// 检查是否为HTTPS
	if ctx.IsTLS() {
		return types.HTTPS
	}

	return types.HTTP
}

// initUpstreams 初始化上游
func (s *Server) initUpstreams() error {
	cfg := s.config.GetConfig()

	for name, backends := range cfg.Backends {
		// 确保backend的原子字段与配置字段同步
		for _, backend := range backends {
			if backend.Active {
				backend.SetActive(true) // 同步原子字段
			} else {
				backend.SetActive(false)
			}
		}

		upstream, err := s.upstreamMgr.CreateUpstream(name, backends)
		if err != nil {
			return fmt.Errorf("failed to create upstream %s: %w", name, err)
		}

		// 设置默认负载均衡器
		upstream.SetLoadBalancer(types.LeastConnectionsWeight, s.lbFactory)
	}

	return nil
}

// initTLS 初始化TLS
func (s *Server) initTLS() error {
	cfg := s.config.GetConfig()

	cert, err := tls.LoadX509KeyPair(cfg.SSL.CertFile, cfg.SSL.KeyFile)
	if err != nil {
		return fmt.Errorf("failed to load TLS cert: %w", err)
	}

	s.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   cfg.Server.Host,
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}

	return nil
}

// watchConfig 监听配置变化
func (s *Server) watchConfig() {
	watcher := s.config.WatchConfig()
	defer s.config.StopWatching(watcher)

	for {
		select {
		case newConfig := <-watcher:
			s.mu.Lock()
			s.updateConfig(newConfig)
			s.mu.Unlock()
		}
	}
}

// updateConfig 更新配置
func (s *Server) updateConfig(config *types.Config) {
	// 更新服务器配置
	s.server.ReadTimeout = config.Server.ReadTimeout
	s.server.WriteTimeout = config.Server.WriteTimeout
	s.server.Concurrency = config.Server.MaxConn

	// 更新上游配置
	s.initUpstreams()
}

// 高性能UpstreamManager方法（无锁设计）
func NewUpstreamManager() *UpstreamManager {
	return &UpstreamManager{
		upstreams: make([]*Upstream, 0, 16), // 预分配容量
		names:     make(map[string]int),
	}
}

func (um *UpstreamManager) CreateUpstream(name string, backends []*types.Backend) (*Upstream, error) {
	// 检查是否已存在
	if _, exists := um.names[name]; exists {
		return nil, fmt.Errorf("upstream %s already exists", name)
	}

	upstream := &Upstream{
		name:     name,
		backends: backends,
	}

	// 添加到切片
	um.upstreams = append(um.upstreams, upstream)
	um.names[name] = len(um.upstreams) - 1

	return upstream, nil
}

func (um *UpstreamManager) GetUpstream(name string) *Upstream {
	if index, exists := um.names[name]; exists && index < len(um.upstreams) {
		return um.upstreams[index]
	}
	return nil
}

// 注意：RemoveUpstream在高并发环境下不安全，需要外部同步
func (um *UpstreamManager) RemoveUpstream(name string) {
	if index, exists := um.names[name]; exists && index < len(um.upstreams) {
		// 从映射中删除
		delete(um.names, name)
		// 注意：这里不删除切片元素以避免索引变化
		// 在生产环境中可能需要更复杂的处理
	}
}

// 高性能Upstream方法（简化锁使用）
func (u *Upstream) SetLoadBalancer(lbType types.LoadBalancerType, factory *loadbalancer.Factory) {
	u.lbType = lbType
	u.balancer = factory.GetBalancer(lbType)
}

func (u *Upstream) GetBackends() []*types.Backend {
	// 创建活跃后端列表，避免锁竞争
	backends := make([]*types.Backend, 0, len(u.backends))
	for _, backend := range u.backends {
		// 检查活跃状态（同时检查原子字段和配置字段）
		if backend.IsActive() && backend.Active {
			backends = append(backends, backend)
		}
	}
	return backends
}

func (u *Upstream) AddBackend(backend *types.Backend) {
	u.backends = append(u.backends, backend)
}

func (u *Upstream) RemoveBackend(backendID string) {
	for i, backend := range u.backends {
		if backend.ID == backendID {
			u.backends = append(u.backends[:i], u.backends[i+1:]...)
			break
		}
	}
}
