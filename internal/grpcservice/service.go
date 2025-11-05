package grpcservice

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/quqi/speedmimi/internal/config"
	"github.com/quqi/speedmimi/internal/monitor"
	"github.com/quqi/speedmimi/internal/proxy"
	"github.com/quqi/speedmimi/pkg/types"
)

// Server 管理API服务器 (暂时用HTTP替代gRPC)
type Server struct {
	configMgr   *config.Manager
	proxyServer *proxy.Server
	monitor     *monitor.PerformanceMonitor
	server      *http.Server
}

// NewServer 创建管理API服务器
func NewServer(configMgr *config.Manager, proxyServer *proxy.Server, perfMonitor *monitor.PerformanceMonitor) *Server {
	return &Server{
		configMgr:   configMgr,
		proxyServer: proxyServer,
		monitor:     perfMonitor,
	}
}

// Start 启动管理API服务器
func (s *Server) Start(host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)

	mux := http.NewServeMux()
	s.setupRoutes(mux)

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	fmt.Printf("Management API server listening on %s\n", addr)
	return s.server.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// setupRoutes 设置路由
func (s *Server) setupRoutes(mux *http.ServeMux) {
	// 配置管理
	mux.HandleFunc("/api/v1/config", s.handleConfig)
	mux.HandleFunc("/api/v1/config/reload-ssl", s.handleReloadSSL)

	// 后端管理
	mux.HandleFunc("/api/v1/backends", s.handleBackends)
	mux.HandleFunc("/api/v1/backends/add", s.handleAddBackend)
	mux.HandleFunc("/api/v1/backends/remove", s.handleRemoveBackend)
	mux.HandleFunc("/api/v1/backends/update", s.handleUpdateBackend)
	mux.HandleFunc("/api/v1/backends/disconnect", s.handleDisconnectBackend)

	// 监控
	mux.HandleFunc("/api/v1/stats/server", s.handleServerStats)
	mux.HandleFunc("/api/v1/stats/backend", s.handleBackendStats)
	mux.HandleFunc("/api/v1/report", s.handleReportPerformance)
}

// handleConfig 配置管理
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		s.getConfig(w, r)
	case http.MethodPut:
		s.updateConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	config := s.configMgr.GetConfig()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"config": config,
	})
}

func (s *Server) updateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Config *types.Config `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.configMgr.UpdateConfig(req.Config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Configuration updated successfully",
	})
}

// handleReloadSSL 重新加载SSL
func (s *Server) handleReloadSSL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.configMgr.ReloadSSL(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "SSL certificates reloaded successfully",
	})
}

// handleBackends 获取后端列表
func (s *Server) handleBackends(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	upstreamID := r.URL.Query().Get("upstream")
	if upstreamID == "" {
		http.Error(w, "upstream parameter required", http.StatusBadRequest)
		return
	}

	// 获取upstream中的backend列表
	upstream := s.proxyServer.GetUpstreamManager().GetUpstream(upstreamID)
	if upstream == nil {
		http.Error(w, "upstream not found", http.StatusNotFound)
		return
	}

	backends := upstream.GetBackends()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"backends": backends,
	})
}

// handleAddBackend 添加后端
func (s *Server) handleAddBackend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": "Not implemented yet",
	})
}

// handleRemoveBackend 移除后端
func (s *Server) handleRemoveBackend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": "Not implemented yet",
	})
}

// handleUpdateBackend 更新后端
func (s *Server) handleUpdateBackend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var req struct {
		UpstreamID string `json:"upstream_id"`
		BackendID  string `json:"backend_id"`
		MaxConn    int    `json:"max_conn"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.UpstreamID == "" || req.BackendID == "" {
		http.Error(w, "upstream_id and backend_id are required", http.StatusBadRequest)
		return
	}

	// 获取upstream
	upstream := s.proxyServer.GetUpstreamManager().GetUpstream(req.UpstreamID)
	if upstream == nil {
		http.Error(w, "upstream not found", http.StatusNotFound)
		return
	}

	// 查找并更新后端
	backends := upstream.GetBackends()
	found := false
	for _, backend := range backends {
		if backend.ID == req.BackendID {
			backend.MaxConn = req.MaxConn
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "backend not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Backend updated successfully",
	})
}

// handleDisconnectBackend 异步断开后端连接（标记机制）
func (s *Server) handleDisconnectBackend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 在主线程中读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// 立即返回响应，不等待处理完成
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Backend disconnect request accepted",
	})

	// 异步处理断开连接请求，避免阻塞响应
	go func(data []byte) {
		var req struct {
			UpstreamID string `json:"upstream_id"`
			BackendID  string `json:"backend_id"`
		}

		if err := json.Unmarshal(data, &req); err != nil {
			fmt.Printf("[DISCONNECT ERROR] Failed to parse request: %v\n", err)
			return
		}

		if req.UpstreamID == "" || req.BackendID == "" {
			fmt.Printf("[DISCONNECT ERROR] Missing upstream_id or backend_id\n")
			return
		}

		// 异步标记后端为断开状态
		s.disconnectBackendAsync(req.UpstreamID, req.BackendID)
	}(body)
}

// disconnectBackendAsync 异步断开后端连接
func (s *Server) disconnectBackendAsync(upstreamID, backendID string) {
	fmt.Printf("[DISCONNECT] Processing disconnect request for backend %s/%s\n", upstreamID, backendID)

	// 通过proxyServer断开后端连接
	if s.proxyServer != nil {
		if err := s.proxyServer.DisconnectBackend(upstreamID, backendID); err != nil {
			fmt.Printf("[DISCONNECT ERROR] Failed to disconnect backend %s/%s: %v\n", upstreamID, backendID, err)
			return
		}
		fmt.Printf("[DISCONNECT] Backend %s/%s successfully marked for disconnection\n", upstreamID, backendID)

		// 验证断开状态
		if err := s.verifyBackendStatus(upstreamID); err != nil {
			fmt.Printf("[DISCONNECT WARNING] Status verification failed: %v\n", err)
		}
	} else {
		fmt.Printf("[DISCONNECT ERROR] Proxy server not available\n")
	}
}

// verifyBackendStatus 验证后端状态（用于调试）
func (s *Server) verifyBackendStatus(upstreamID string) error {
	upstream := s.proxyServer.GetUpstreamManager().GetUpstream(upstreamID)
	if upstream == nil {
		return fmt.Errorf("upstream %s not found", upstreamID)
	}

	backends := upstream.GetBackends()
	fmt.Printf("[STATUS] Upstream %s has %d backends:\n", upstreamID, len(backends))

	activeCount := 0
	disconnectCount := 0
	for _, backend := range backends {
		status := "ACTIVE"
		if !backend.IsActive() {
			status = "INACTIVE"
		}
		if backend.ShouldDisconnect() {
			status += "(DISCONNECTING)"
			disconnectCount++
		} else {
			activeCount++
		}
		fmt.Printf("  - %s: %s (connections: %d)\n", backend.ID, status, backend.GetConnections())
	}

	fmt.Printf("[STATUS] Active backends: %d, Disconnecting: %d\n", activeCount, disconnectCount)
	return nil
}

// handleServerStats 获取服务器统计（非阻塞）
func (s *Server) handleServerStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从异步monitor获取最新的性能数据（非阻塞）
	var stats *types.PerformanceInfo
	if s.monitor != nil {
		stats = s.monitor.GetStats()
	} else {
		// fallback
		stats = &types.PerformanceInfo{
			CPUUsage:    0,
			MemoryUsage: 0,
			DiskUsage:   0,
			LoadAvg1:    0,
			LoadAvg5:    0,
			LoadAvg15:   0,
			NetworkIn:   0,
			NetworkOut:  0,
			Timestamp:   0,
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats": stats,
	})
}

// handleBackendStats 获取后端统计
func (s *Server) handleBackendStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 返回模拟数据
	stats := &types.PerformanceInfo{
		CPUUsage:    0,
		MemoryUsage: 0,
		DiskUsage:   0,
		LoadAvg1:    0,
		LoadAvg5:    0,
		LoadAvg15:   0,
		NetworkIn:   0,
		NetworkOut:  0,
		Timestamp:   0,
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats": stats,
	})
}

// handleReportPerformance 上报性能（异步处理）
func (s *Server) handleReportPerformance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 在主线程中读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// 立即返回响应，不等待处理完成
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Performance data accepted",
	})

	// 异步处理性能上报，避免阻塞响应
	go func(data []byte) {
		var req struct {
			Upstream    string                `json:"upstream"`
			BackendID   string                `json:"backend_id"`
			Performance *types.PerformanceInfo `json:"performance"`
		}

		if err := json.Unmarshal(data, &req); err != nil {
			return
		}

		// 更新后端性能信息（异步）
		if req.Upstream != "" && req.BackendID != "" && req.Performance != nil {
			// 这里可以更新upstream中的后端性能信息
			// 为了演示，我们暂时只记录
			fmt.Printf("[PERF REPORT] %s/%s: CPU=%.1f%%, MEM=%.1f%%\n",
				req.Upstream, req.BackendID, req.Performance.CPUUsage, req.Performance.MemoryUsage)
		}
	}(body)
}