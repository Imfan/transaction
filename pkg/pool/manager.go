package pool

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"transaction/pkg/config"
	"transaction/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Manager 连接池管理器
type Manager struct {
	pools   map[string]Pool
	config  *config.Config
	logger  *zap.Logger
	mutex   sync.RWMutex
	stopCh  chan struct{}
	httpSrv *http.Server
}

// NewManager 创建连接池管理器
func NewManager(cfg *config.Config, logger *zap.Logger) *Manager {
	return &Manager{
		pools:  make(map[string]Pool),
		config: cfg,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// Start 启动管理器
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting connection pool manager")

	// 启动监控服务器
	if m.config.Metrics.Enable {
		go m.startMetricsServer()
	}

	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.logger.Info("Stopping connection pool manager")

	close(m.stopCh)

	// 关闭所有连接池
	for serviceName, pool := range m.pools {
		if err := pool.Close(); err != nil {
			m.logger.Error("Failed to close pool",
				zap.String("service", serviceName),
				zap.Error(err))
		}
	}

	// 关闭监控服务器
	if m.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.httpSrv.Shutdown(ctx)
	}

	m.logger.Info("Connection pool manager stopped")
	return nil
}

// GetPool 获取或创建指定服务的连接池
func (m *Manager) GetPool(serviceName string) (Pool, error) {
	m.mutex.RLock()
	if pool, exists := m.pools[serviceName]; exists {
		m.mutex.RUnlock()
		return pool, nil
	}
	m.mutex.RUnlock()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 再次检查是否已存在
	if pool, exists := m.pools[serviceName]; exists {
		return pool, nil
	}

	// 复制配置并设置服务名
	cfg := *m.config
	cfg.Discovery.ServiceName = serviceName

	// 创建连接池
	pool, err := NewPool(serviceName, &cfg, m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool for service %s: %w", serviceName, err)
	}

	// 启动连接池
	ctx := context.Background()
	if err := pool.(*grpcPool).Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start pool for service %s: %w", serviceName, err)
	}

	m.pools[serviceName] = pool
	m.logger.Info("Created new connection pool", zap.String("service", serviceName))

	return pool, nil
}

// CreatePool 创建指定服务的连接池
func (m *Manager) CreatePool(serviceName string) (Pool, error) {
	return m.createPool(serviceName)
}

func (m *Manager) createPool(serviceName string) (Pool, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 再次检查是否已存在
	if pool, exists := m.pools[serviceName]; exists {
		return pool, nil
	}

	// 复制配置并设置服务名
	cfg := *m.config
	cfg.Discovery.ServiceName = serviceName

	// 创建连接池
	pool, err := NewPool(serviceName, &cfg, m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool for service %s: %w", serviceName, err)
	}

	// 启动连接池
	ctx := context.Background()
	if err := pool.(*grpcPool).Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start pool for service %s: %w", serviceName, err)
	}

	m.pools[serviceName] = pool
	m.logger.Info("Created new connection pool", zap.String("service", serviceName))

	return pool, nil
}

// RemovePool 移除指定服务的连接池
func (m *Manager) RemovePool(serviceName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	pool, exists := m.pools[serviceName]
	if !exists {
		return nil
	}

	if err := pool.Close(); err != nil {
		return fmt.Errorf("failed to close pool for service %s: %w", serviceName, err)
	}

	delete(m.pools, serviceName)
	m.logger.Info("Removed connection pool", zap.String("service", serviceName))

	return nil
}

// ExecuteWithPool 使用连接池执行gRPC调用
func (m *Manager) ExecuteWithPool(ctx context.Context, serviceName string, fn func(*grpc.ClientConn) error) error {
	pool, err := m.GetPool(serviceName)
	if err != nil {
		return fmt.Errorf("failed to get pool: %w", err)
	}

	conn, err := pool.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer pool.Put(conn)

	start := time.Now()
	err = fn(conn.GetConn())
	duration := time.Since(start).Seconds()

	// 记录请求指标
	status := "success"
	if err != nil {
		status = "error"
	}
	metrics.RecordRequest(serviceName, conn.GetTarget(), "unknown", status, duration)

	return err
}

// GetStats 获取所有连接池的统计信息
func (m *Manager) GetStats() map[string]PoolStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := make(map[string]PoolStats)
	for serviceName, pool := range m.pools {
		stats[serviceName] = pool.Stats()
	}

	return stats
}

// GetHealthyPools 获取健康的连接池列表
func (m *Manager) GetHealthyPools() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var healthyPools []string
	for serviceName, pool := range m.pools {
		stats := pool.Stats()
		if stats.TotalConnections > 0 {
			healthyPools = append(healthyPools, serviceName)
		}
	}

	return healthyPools
}

// startMetricsServer 启动监控服务器
func (m *Manager) startMetricsServer() {
	mux := http.NewServeMux()

	// Prometheus metrics endpoint
	mux.Handle(m.config.Metrics.Path, promhttp.Handler())

	// Health check endpoint
	mux.HandleFunc("/health", m.healthCheckHandler)

	// Stats endpoint
	mux.HandleFunc("/stats", m.statsHandler)

	m.httpSrv = &http.Server{
		Addr:    fmt.Sprintf(":%d", m.config.Metrics.Port),
		Handler: mux,
	}

	m.logger.Info("Starting metrics server",
		zap.Int("port", m.config.Metrics.Port),
		zap.String("path", m.config.Metrics.Path))

	if err := m.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		m.logger.Error("Metrics server failed", zap.Error(err))
	}
}

// healthCheckHandler 健康检查处理器
func (m *Manager) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	healthyPools := m.GetHealthyPools()

	if len(healthyPools) == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unhealthy","pools":[]}`))
		return
	}

	w.WriteHeader(http.StatusOK)
	response := fmt.Sprintf(`{"status":"healthy","pools":["%s"]}`,
		fmt.Sprintf(`","%s`, healthyPools))
	w.Write([]byte(response))
}

// statsHandler 统计信息处理器
func (m *Manager) statsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	stats := m.GetStats()

	response := `{"pools":{`
	first := true
	for serviceName, stat := range stats {
		if !first {
			response += ","
		}
		response += fmt.Sprintf(`"%s":{"total":%d,"active":%d,"idle":%d}`,
			serviceName, stat.TotalConnections, stat.ActiveConnections, stat.IdleConnections)
		first = false
	}
	response += `}}`

	w.Write([]byte(response))
}

// CircuitBreaker 熔断器（简单实现）
type CircuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration
	failureCount int
	lastFailTime time.Time
	state        string // "closed", "open", "half-open"
	mutex        sync.RWMutex
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        "closed",
	}
}

// Call 执行调用，带熔断保护
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	// 检查熔断器状态
	if cb.state == "open" {
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.state = "half-open"
			cb.failureCount = 0
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}

	err := fn()

	if err != nil {
		cb.failureCount++
		cb.lastFailTime = time.Now()

		if cb.failureCount >= cb.maxFailures {
			cb.state = "open"
		}

		return err
	}

	// 成功调用，重置计数器
	if cb.state == "half-open" {
		cb.state = "closed"
	}
	cb.failureCount = 0

	return nil
}

// GetState 获取熔断器状态
func (cb *CircuitBreaker) GetState() string {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}
