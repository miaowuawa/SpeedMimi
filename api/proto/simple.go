package proto

import (
	"time"

	"github.com/quqi/speedmimi/pkg/types"
)

// 简化版本的proto消息，直接使用types包中的类型

// ConfigService 请求响应
type UpdateConfigRequest struct {
	Config *types.Config `json:"config"`
}

type UpdateConfigResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type GetConfigResponse struct {
	Config *types.Config `json:"config"`
}

type ReloadSSLResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// BackendService 请求响应
type GetBackendsRequest struct {
	Upstream string `json:"upstream"`
}

type GetBackendsResponse struct {
	Backends []*types.Backend `json:"backends"`
}

type AddBackendRequest struct {
	Upstream string         `json:"upstream"`
	Backend  *types.Backend `json:"backend"`
}

type AddBackendResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type RemoveBackendRequest struct {
	Upstream   string `json:"upstream"`
	BackendID string `json:"backend_id"`
}

type RemoveBackendResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type UpdateBackendRequest struct {
	Upstream string         `json:"upstream"`
	Backend  *types.Backend `json:"backend"`
}

type UpdateBackendResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type DisconnectBackendRequest struct {
	Upstream   string `json:"upstream"`
	BackendID string `json:"backend_id"`
}

type DisconnectBackendResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// MonitorService 请求响应
type GetServerStatsResponse struct {
	Stats *types.PerformanceInfo `json:"stats"`
}

type GetBackendStatsRequest struct {
	Upstream   string `json:"upstream"`
	BackendID string `json:"backend_id"`
}

type GetBackendStatsResponse struct {
	Stats *types.PerformanceInfo `json:"stats"`
}

type ReportPerformanceRequest struct {
	Upstream     string                `json:"upstream"`
	BackendID   string                `json:"backend_id"`
	Performance *types.PerformanceInfo `json:"performance"`
}

type ReportPerformanceResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// 简化时间解析函数
func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 30 * time.Second // 默认值
	}
	return d
}
