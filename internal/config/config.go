package config

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/quqi/speedmimi/pkg/types"
)

// Manager 配置管理器
type Manager struct {
	config     *types.Config
	configPath string
	mu         sync.RWMutex
	watchers   []chan *types.Config
}

// NewManager 创建配置管理器
func NewManager(configPath string) (*Manager, error) {
	m := &Manager{
		configPath: configPath,
		watchers:   make([]chan *types.Config, 0),
	}

	// 加载初始配置
	if err := m.loadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return m, nil
}

// GetConfig 获取当前配置
func (m *Manager) GetConfig() *types.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *types.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证配置
	if err := m.validateConfig(config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// 保存到文件
	if err := m.saveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// 更新内存配置
	m.config = config

	// 通知观察者
	m.notifyWatchers(config)

	return nil
}

// ReloadSSL 重新加载SSL证书
func (m *Manager) ReloadSSL() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查SSL配置
	if !m.config.SSL.Enabled {
		return fmt.Errorf("SSL is not enabled")
	}

	// 验证证书文件是否存在
	if _, err := os.Stat(m.config.SSL.CertFile); os.IsNotExist(err) {
		return fmt.Errorf("SSL cert file not found: %s", m.config.SSL.CertFile)
	}
	if _, err := os.Stat(m.config.SSL.KeyFile); os.IsNotExist(err) {
		return fmt.Errorf("SSL key file not found: %s", m.config.SSL.KeyFile)
	}

	// 这里可以添加证书重新加载的逻辑
	// 由于fasthttp的证书重新加载需要在服务器层面处理，这里只验证文件存在

	return nil
}

// WatchConfig 监听配置变化
func (m *Manager) WatchConfig() <-chan *types.Config {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *types.Config, 1)
	m.watchers = append(m.watchers, ch)

	return ch
}

// StopWatching 停止监听
func (m *Manager) StopWatching(ch <-chan *types.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, watcher := range m.watchers {
		if watcher == ch {
			m.watchers = append(m.watchers[:i], m.watchers[i+1:]...)
			close(watcher)
			break
		}
	}
}

// loadConfig 从文件加载配置
func (m *Manager) loadConfig() error {
	viper.SetConfigFile(m.configPath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	config := &types.Config{}
	if err := viper.Unmarshal(config); err != nil {
		return err
	}

	// 设置默认值
	m.setDefaults(config)

	// 验证配置
	if err := m.validateConfig(config); err != nil {
		return err
	}

	m.config = config
	return nil
}

// saveConfig 保存配置到文件
func (m *Manager) saveConfig(config *types.Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// setDefaults 设置默认值
func (m *Manager) setDefaults(config *types.Config) {
	if config.Server.ReadTimeout == 0 {
		config.Server.ReadTimeout = 30 * time.Second
	}
	if config.Server.WriteTimeout == 0 {
		config.Server.WriteTimeout = 30 * time.Second
	}
	if config.Server.MaxConn == 0 {
		config.Server.MaxConn = 10000
	}
	if config.Server.RealIPHeader == "" {
		config.Server.RealIPHeader = "X-Real-IP"
	}

	// 设置后端默认值
	for upstream, backends := range config.Backends {
		for _, backend := range backends {
			if backend.ID == "" {
				backend.ID = fmt.Sprintf("%s-%s-%d", upstream, backend.Host, backend.Port)
			}
			if backend.Weight == 0 {
				backend.Weight = 100
			}
			if backend.Scheme == "" {
				backend.Scheme = "http"
			}
			if backend.MaxConn == 0 {
				backend.MaxConn = 1000
			}
			if backend.HealthCheck != nil {
				if backend.HealthCheck.Interval == 0 {
					backend.HealthCheck.Interval = 30 * time.Second
				}
				if backend.HealthCheck.Timeout == 0 {
					backend.HealthCheck.Timeout = 5 * time.Second
				}
				if backend.HealthCheck.Failures == 0 {
					backend.HealthCheck.Failures = 3
				}
			}
		}
	}

	// 设置路由默认值
	for name, rule := range config.Routing {
		if rule.Path == "" {
			rule.Path = "/"
		}
		if rule.LoadBalancer == "" {
			rule.LoadBalancer = types.LeastConnectionsWeight
		}
		if rule.Protocols == nil {
			rule.Protocols = make(map[types.ProtocolType]types.LoadBalancerType)
		}
		config.Routing[name] = rule
	}
}

// validateConfig 验证配置
func (m *Manager) validateConfig(config *types.Config) error {
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	if config.SSL.Enabled {
		if config.SSL.CertFile == "" {
			return fmt.Errorf("SSL cert file is required when SSL is enabled")
		}
		if config.SSL.KeyFile == "" {
			return fmt.Errorf("SSL key file is required when SSL is enabled")
		}
	}

	// 验证后端配置
	for upstream, backends := range config.Backends {
		if len(backends) == 0 {
			return fmt.Errorf("upstream %s has no backends", upstream)
		}

		for _, backend := range backends {
			if backend.Host == "" {
				return fmt.Errorf("backend host is required for upstream %s", upstream)
			}
			if backend.Port <= 0 || backend.Port > 65535 {
				return fmt.Errorf("invalid backend port %d for upstream %s", backend.Port, upstream)
			}
		}
	}

	// 验证路由配置
	for name, rule := range config.Routing {
		if rule.Upstream == "" {
			return fmt.Errorf("upstream is required for routing rule %s", name)
		}
		if _, exists := config.Backends[rule.Upstream]; !exists {
			return fmt.Errorf("upstream %s not found for routing rule %s", rule.Upstream, name)
		}
	}

	return nil
}

// notifyWatchers 通知观察者
func (m *Manager) notifyWatchers(config *types.Config) {
	for _, watcher := range m.watchers {
		select {
		case watcher <- config:
		default:
			// 如果通道已满，跳过
		}
	}
}
