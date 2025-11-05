package monitor

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quqi/speedmimi/pkg/types"
)

// PerformanceMonitor 性能监控器（异步采样，避免阻塞主路径）
type PerformanceMonitor struct {
	// 采样配置
	sampleInterval time.Duration
	reportInterval time.Duration

	// 统计数据（原子操作）
	totalRequests     int64
	activeConnections int64
	totalBytesSent    int64
	totalBytesRecv    int64

	// 性能指标缓存（使用原子操作）
	lastCPUUsage    int64 // 使用int64存储float64的值（放大100倍）
	lastMemoryUsage int64
	lastLoadAvg     int64

	// 采样控制
	samplingEnabled bool
	reportEnabled   bool

	// 异步通道
	sampleChan chan *SampleData
	reportChan chan *types.PerformanceInfo

	// 上下文控制
	ctx    context.Context
	cancel context.CancelFunc

	// 同步保护
	mu sync.RWMutex
}

// SampleData 采样数据
type SampleData struct {
	Timestamp       time.Time
	ActiveRequests  int64
	TotalRequests   int64
	BytesSent       int64
	BytesRecv       int64
	ActiveGoroutines int
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor() *PerformanceMonitor {
	ctx, cancel := context.WithCancel(context.Background())

	pm := &PerformanceMonitor{
		sampleInterval: 100 * time.Millisecond, // 每100ms采样一次
		reportInterval: 5 * time.Second,       // 每5秒上报一次

		samplingEnabled: true,
		reportEnabled:   true,

		sampleChan: make(chan *SampleData, 1000),    // 缓冲1000个采样数据
		reportChan: make(chan *types.PerformanceInfo, 100),

		ctx:    ctx,
		cancel: cancel,
	}

	// 启动异步goroutine
	go pm.samplingLoop()
	go pm.reportingLoop()

	return pm
}

// RecordRequest 记录请求（轻量级，不阻塞）
func (pm *PerformanceMonitor) RecordRequest(bytesSent, bytesRecv int64) {
	if !pm.samplingEnabled {
		return
	}

	atomic.AddInt64(&pm.totalRequests, 1)
	atomic.AddInt64(&pm.totalBytesSent, bytesSent)
	atomic.AddInt64(&pm.totalBytesRecv, bytesRecv)
}

// StartConnection 连接开始
func (pm *PerformanceMonitor) StartConnection() {
	atomic.AddInt64(&pm.activeConnections, 1)
}

// EndConnection 连接结束
func (pm *PerformanceMonitor) EndConnection() {
	atomic.AddInt64(&pm.activeConnections, -1)
}

// GetStats 获取当前统计（非阻塞）
func (pm *PerformanceMonitor) GetStats() *types.PerformanceInfo {
	return &types.PerformanceInfo{
		CPUUsage:    float64(atomic.LoadInt64(&pm.lastCPUUsage)) / 100.0,
		MemoryUsage: float64(atomic.LoadInt64(&pm.lastMemoryUsage)) / 100.0,
		DiskUsage:   0,
		LoadAvg1:    float64(atomic.LoadInt64(&pm.lastLoadAvg)) / 100.0,
		LoadAvg5:    float64(atomic.LoadInt64(&pm.lastLoadAvg)) / 100.0,
		LoadAvg15:   float64(atomic.LoadInt64(&pm.lastLoadAvg)) / 100.0,
		NetworkIn:   0,
		NetworkOut:  0,
		Timestamp:   time.Now().Unix(),
	}
}

// SetReportCallback 设置上报回调（异步）
func (pm *PerformanceMonitor) SetReportCallback(callback func(*types.PerformanceInfo)) {
	go func() {
		for {
			select {
			case <-pm.ctx.Done():
				return
			case perf := <-pm.reportChan:
				if callback != nil {
					// 在单独的goroutine中执行回调，避免阻塞
					go callback(perf)
				}
			}
		}
	}()
}

// samplingLoop 采样循环（异步）
func (pm *PerformanceMonitor) samplingLoop() {
	ticker := time.NewTicker(pm.sampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			if !pm.samplingEnabled {
				continue
			}

			// 异步采样系统指标
			go pm.collectSystemMetrics()

			// 发送采样数据到通道（非阻塞）
			select {
			case pm.sampleChan <- &SampleData{
				Timestamp:       time.Now(),
				ActiveRequests:  atomic.LoadInt64(&pm.activeConnections),
				TotalRequests:   atomic.LoadInt64(&pm.totalRequests),
				BytesSent:       atomic.LoadInt64(&pm.totalBytesSent),
				BytesRecv:       atomic.LoadInt64(&pm.totalBytesRecv),
				ActiveGoroutines: runtime.NumGoroutine(),
			}:
			default:
				// 通道满，丢弃采样数据，确保不阻塞
			}
		}
	}
}

// reportingLoop 上报循环（异步）
func (pm *PerformanceMonitor) reportingLoop() {
	ticker := time.NewTicker(pm.reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-ticker.C:
			if !pm.reportEnabled {
				continue
			}

			// 异步生成性能报告
			go pm.generateReport()
		}
	}
}

// collectSystemMetrics 收集系统指标（异步，避免阻塞）
func (pm *PerformanceMonitor) collectSystemMetrics() {
	// 这里应该收集CPU、内存等系统指标
	// 为避免复杂性，这里使用模拟数据
	// 实际实现中应该使用gopsutil等库

	// 模拟CPU使用率（基于goroutine数量估算）
	goroutines := runtime.NumGoroutine()
	cpuUsage := float64(goroutines) * 0.01 // 简单估算
	if cpuUsage > 100 {
		cpuUsage = 100
	}

	// 模拟内存使用率
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memUsage := float64(memStats.Alloc) / float64(memStats.Sys) * 100

	// 模拟负载
	loadAvg := float64(runtime.NumGoroutine()) / 100.0

	// 原子更新缓存
	atomic.StoreInt64(&pm.lastCPUUsage, int64(cpuUsage*100))
	atomic.StoreInt64(&pm.lastMemoryUsage, int64(memUsage*100))
	atomic.StoreInt64(&pm.lastLoadAvg, int64(loadAvg*100))
}

// generateReport 生成性能报告（异步）
func (pm *PerformanceMonitor) generateReport() {
	perf := &types.PerformanceInfo{
		CPUUsage:    float64(atomic.LoadInt64(&pm.lastCPUUsage)) / 100.0,
		MemoryUsage: float64(atomic.LoadInt64(&pm.lastMemoryUsage)) / 100.0,
		DiskUsage:   0, // 暂时不支持
		LoadAvg1:    float64(atomic.LoadInt64(&pm.lastLoadAvg)) / 100.0,
		LoadAvg5:    float64(atomic.LoadInt64(&pm.lastLoadAvg)) / 100.0,
		LoadAvg15:   float64(atomic.LoadInt64(&pm.lastLoadAvg)) / 100.0,
		NetworkIn:   0, // 暂时不支持
		NetworkOut:  0, // 暂时不支持
		Timestamp:   time.Now().Unix(),
	}

	// 发送到上报通道（非阻塞）
	select {
	case pm.reportChan <- perf:
	default:
		// 通道满，丢弃报告，确保不阻塞
	}
}

// Stop 停止监控
func (pm *PerformanceMonitor) Stop() {
	pm.cancel()
	close(pm.sampleChan)
	close(pm.reportChan)
}

// EnableSampling 启用采样
func (pm *PerformanceMonitor) EnableSampling(enabled bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.samplingEnabled = enabled
}

// EnableReporting 启用上报
func (pm *PerformanceMonitor) EnableReporting(enabled bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.reportEnabled = enabled
}

// GetSampleChannel 获取采样数据通道（用于调试）
func (pm *PerformanceMonitor) GetSampleChannel() <-chan *SampleData {
	return pm.sampleChan
}
