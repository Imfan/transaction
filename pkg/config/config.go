package config

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"
)

// Duration 自定义Duration类型，支持JSON解析
type Duration time.Duration

// UnmarshalJSON 实现JSON解析
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	duration, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration format: %s", s)
	}

	*d = Duration(duration)
	return nil
}

// MarshalJSON 实现JSON序列化
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// ToDuration 转换为标准time.Duration
func (d Duration) ToDuration() time.Duration {
	return time.Duration(d)
}

// Config 连接池和服务发现的总配置
type Config struct {
	Pool      *PoolConfig         `json:"pool"`
	Discovery *DiscoveryConfig    `json:"discovery"`
	TLS       *TLSConfig          `json:"tls"`
	Metrics   *MetricsConfig      `json:"metrics"`
	Services  map[string][]string `json:"services,omitempty"` // 直接连接模式的服务地址
}

// PoolConfig 连接池配置
type PoolConfig struct {
	InitialSize       int      `json:"initial_size"`        // 初始连接数
	MaxSize           int      `json:"max_size"`            // 最大连接数
	IdleTimeout       Duration `json:"idle_timeout"`        // 空闲超时时间
	MaxConnAge        Duration `json:"max_conn_age"`        // 连接最大存活时间
	HealthCheckPeriod Duration `json:"health_check_period"` // 健康检查周期
	ConnectTimeout    Duration `json:"connect_timeout"`     // 连接超时
	KeepAlive         Duration `json:"keep_alive"`          // 保活时间
}

// DiscoveryConfig 服务发现配置
type DiscoveryConfig struct {
	ConsulAddr    string   `json:"consul_addr"`    // Consul地址
	ServiceName   string   `json:"service_name"`   // 服务名称
	RefreshPeriod Duration `json:"refresh_period"` // 刷新周期
	Tags          []string `json:"tags"`           // 服务标签过滤
	Datacenter    string   `json:"datacenter"`     // 数据中心
	Token         string   `json:"token"`          // Consul Token
}

// TLSConfig TLS配置
type TLSConfig struct {
	Enable     bool   `json:"enable"`      // 是否启用TLS
	CertFile   string `json:"cert_file"`   // 证书文件路径
	KeyFile    string `json:"key_file"`    // 私钥文件路径
	CAFile     string `json:"ca_file"`     // CA证书文件路径
	ServerName string `json:"server_name"` // 服务器名称
	SkipVerify bool   `json:"skip_verify"` // 跳过证书验证
}

// MetricsConfig 监控配置
type MetricsConfig struct {
	Enable bool   `json:"enable"` // 是否启用监控
	Port   int    `json:"port"`   // 监控端口
	Path   string `json:"path"`   // 监控路径
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Pool: &PoolConfig{
			InitialSize:       5,
			MaxSize:           50,
			IdleTimeout:       Duration(30 * time.Minute),
			MaxConnAge:        Duration(1 * time.Hour),
			HealthCheckPeriod: Duration(30 * time.Second),
			ConnectTimeout:    Duration(5 * time.Second),
			KeepAlive:         Duration(30 * time.Second),
		},
		Discovery: &DiscoveryConfig{
			ConsulAddr:    "localhost:8500",
			RefreshPeriod: Duration(10 * time.Second),
			Tags:          []string{},
			Datacenter:    "dc1",
		},
		TLS: &TLSConfig{
			Enable:     false,
			SkipVerify: false,
		},
		Metrics: &MetricsConfig{
			Enable: true,
			Port:   9090,
			Path:   "/metrics",
		},
	}
}

// GetTLSConfig 获取TLS配置
func (c *TLSConfig) GetTLSConfig() (*tls.Config, error) {
	if !c.Enable {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		ServerName:         c.ServerName,
		InsecureSkipVerify: c.SkipVerify,
	}

	if c.CertFile != "" && c.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Pool.InitialSize <= 0 {
		c.Pool.InitialSize = 1
	}
	if c.Pool.MaxSize <= 0 {
		c.Pool.MaxSize = 10
	}
	if c.Pool.InitialSize > c.Pool.MaxSize {
		c.Pool.InitialSize = c.Pool.MaxSize
	}
	if c.Discovery.ServiceName == "" {
		c.Discovery.ServiceName = "unknown"
	}
	return nil
}
