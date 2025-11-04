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
	"github.com/quqi/speedmimi/pkg/types"
)

// Server 反向代理服务器
type Server struct {
	config         *config.Manager
	lbFactory      *loadbalancer.Factory
	upstreamMgr    *UpstreamManager
	server         *fasthttp.Server
	tlsConfig      *tls.Config
	mu             sync.RWMutex
}

// UpstreamManager 上游管理器
type UpstreamManager struct {
	upstreams map[string]*Upstream
	mu        sync.RWMutex
}

type Upstream struct {
	name     string
	backends []*types.Backend
	lbType   types.LoadBalancerType
	balancer types.LoadBalancer
	mu       sync.RWMutex
}

// NewServer 创建代理服务器
func NewServer(cfgMgr *config.Manager) (*Server, error) {
	lbFactory := loadbalancer.NewFactory()
	upstreamMgr := NewUpstreamManager()

	server := &Server{
		config:      cfgMgr,
		lbFactory:   lbFactory,
		upstreamMgr: upstreamMgr,
	}

	// 初始化上游
	if err := server.initUpstreams(); err != nil {
		return nil, fmt.Errorf("failed to init upstreams: %w", err)
	}

	// 创建fasthttp服务器
	fasthttpServer := &fasthttp.Server{
		Handler:                       server.handleRequest,
		ReadTimeout:                   cfgMgr.GetConfig().Server.ReadTimeout,
		WriteTimeout:                  cfgMgr.GetConfig().Server.WriteTimeout,
		MaxConnsPerIP:                 0, // 不限制
		MaxRequestsPerConn:            0, // 不限制
		MaxKeepaliveDuration:          60 * time.Second,
		TCPKeepalive:                  true,
		TCPKeepalivePeriod:            60 * time.Second,
		ReduceMemoryUsage:             false, // 性能优先
		GetOnly:                       false,
		DisablePreParseMultipartForm: true,
		LogAllErrors:                  false,
		DisableHeaderNamesNormalizing: true,
		NoDefaultServerHeader:         true,
		NoDefaultDate:                 false,
		NoDefaultContentType:          true,
		KeepHijackedConns:             false,
		CloseOnShutdown:               true,
		StreamRequestBody:             true,
		MaxRequestBodySize:            4 * 1024 * 1024, // 4MB
		SleepWhenConcurrencyLimitsExceeded: 0,
		Concurrency:                   cfgMgr.GetConfig().Server.MaxConn,
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
	return s.server.Shutdown()
}

// handleRequest 处理请求
func (s *Server) handleRequest(ctx *fasthttp.RequestCtx) {
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
		ctx.Error("Service Unavailable", fasthttp.StatusServiceUnavailable)
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

	// 创建代理客户端
	client := &fasthttp.Client{
		ReadTimeout:              30 * time.Second,
		WriteTimeout:             30 * time.Second,
		MaxConnDuration:          60 * time.Second,
		MaxConnWaitTimeout:       30 * time.Second,
		MaxIdleConnDuration:      60 * time.Second,
		NoDefaultUserAgentHeader: true,
		Dial: func(addr string) (net.Conn, error) {
			return fasthttp.DialDualStackTimeout(addr, 5*time.Second)
		},
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

// UpstreamManager methods

func NewUpstreamManager() *UpstreamManager {
	return &UpstreamManager{
		upstreams: make(map[string]*Upstream),
	}
}

func (um *UpstreamManager) CreateUpstream(name string, backends []*types.Backend) (*Upstream, error) {
	um.mu.Lock()
	defer um.mu.Unlock()

	upstream := &Upstream{
		name:     name,
		backends: backends,
	}

	um.upstreams[name] = upstream
	return upstream, nil
}

func (um *UpstreamManager) GetUpstream(name string) *Upstream {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.upstreams[name]
}

func (um *UpstreamManager) RemoveUpstream(name string) {
	um.mu.Lock()
	defer um.mu.Unlock()
	delete(um.upstreams, name)
}

// Upstream methods

func (u *Upstream) SetLoadBalancer(lbType types.LoadBalancerType, factory *loadbalancer.Factory) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.lbType = lbType
	u.balancer = factory.GetBalancer(lbType)
}

func (u *Upstream) GetBackends() []*types.Backend {
	u.mu.RLock()
	defer u.mu.RUnlock()

	backends := make([]*types.Backend, 0, len(u.backends))
	for _, backend := range u.backends {
		if backend.IsActive() {
			backends = append(backends, backend)
		}
	}
	return backends
}

func (u *Upstream) AddBackend(backend *types.Backend) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.backends = append(u.backends, backend)
}

func (u *Upstream) RemoveBackend(backendID string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	for i, backend := range u.backends {
		if backend.ID == backendID {
			u.backends = append(u.backends[:i], u.backends[i+1:]...)
			break
		}
	}
}
