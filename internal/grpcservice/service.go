package grpcservice

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/quqi/speedmimi/internal/config"
	"github.com/quqi/speedmimi/internal/proxy"
	"github.com/quqi/speedmimi/pkg/types"
)

// Server 管理API服务器 (暂时用HTTP替代gRPC)
type Server struct {
	configMgr   *config.Manager
	proxyServer *proxy.Server
	server      *http.Server
}

// NewServer 创建管理API服务器
func NewServer(configMgr *config.Manager, proxyServer *proxy.Server) *Server {
	return &Server{
		configMgr:   configMgr,
		proxyServer: proxyServer,
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

	upstream := r.URL.Query().Get("upstream")
	if upstream == "" {
		http.Error(w, "upstream parameter required", http.StatusBadRequest)
		return
	}

	// 暂时返回空实现
	json.NewEncoder(w).Encode(map[string]interface{}{
		"backends": []*types.Backend{},
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

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": "Not implemented yet",
	})
}

// handleDisconnectBackend 断开后端连接
func (s *Server) handleDisconnectBackend(w http.ResponseWriter, r *http.Request) {
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

// handleServerStats 获取服务器统计
func (s *Server) handleServerStats(w http.ResponseWriter, r *http.Request) {
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

// handleReportPerformance 上报性能
func (s *Server) handleReportPerformance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Performance data reported successfully",
	})
}