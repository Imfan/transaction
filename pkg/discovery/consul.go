package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"transaction/pkg/balancer"
	"transaction/pkg/config"
	"transaction/pkg/metrics"

	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

// ServiceDiscovery 服务发现接口
type ServiceDiscovery interface {
	// Start 启动服务发现
	Start(ctx context.Context) error
	// Stop 停止服务发现
	Stop() error
	// GetNodes 获取服务节点
	GetNodes() ([]*balancer.Node, error)
	// Subscribe 订阅节点变化
	Subscribe(callback func([]*balancer.Node))
}

// consulDiscovery Consul服务发现实现
type consulDiscovery struct {
	client    *api.Client
	config    *config.DiscoveryConfig
	logger    *zap.Logger
	nodes     []*balancer.Node
	mutex     sync.RWMutex
	callbacks []func([]*balancer.Node)
	stopCh    chan struct{}
	stopped   bool
	lastIndex uint64
}

// NewConsulDiscovery 创建Consul服务发现
func NewConsulDiscovery(cfg *config.DiscoveryConfig, logger *zap.Logger) (ServiceDiscovery, error) {
	clientConfig := api.DefaultConfig()
	clientConfig.Address = cfg.ConsulAddr
	clientConfig.Datacenter = cfg.Datacenter
	clientConfig.Token = cfg.Token

	client, err := api.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	return &consulDiscovery{
		client:    client,
		config:    cfg,
		logger:    logger,
		nodes:     make([]*balancer.Node, 0),
		callbacks: make([]func([]*balancer.Node), 0),
		stopCh:    make(chan struct{}),
	}, nil
}

func (c *consulDiscovery) Start(ctx context.Context) error {
	c.logger.Info("Starting consul service discovery",
		zap.String("service", c.config.ServiceName),
		zap.String("consul_addr", c.config.ConsulAddr))

	// 初始化获取服务节点
	if err := c.updateNodes(); err != nil {
		c.logger.Error("Failed to initialize nodes", zap.Error(err))
		metrics.RecordServiceDiscoveryError(c.config.ServiceName, "initialization")
		return err
	}

	// 启动定期刷新goroutine
	go c.watchNodes(ctx)

	return nil
}

func (c *consulDiscovery) Stop() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.stopped {
		return nil
	}

	close(c.stopCh)
	c.stopped = true
	c.logger.Info("Consul service discovery stopped")
	return nil
}

func (c *consulDiscovery) GetNodes() ([]*balancer.Node, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	nodes := make([]*balancer.Node, len(c.nodes))
	copy(nodes, c.nodes)
	return nodes, nil
}

func (c *consulDiscovery) Subscribe(callback func([]*balancer.Node)) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.callbacks = append(c.callbacks, callback)
}

func (c *consulDiscovery) watchNodes(ctx context.Context) {
	ticker := time.NewTicker(c.config.RefreshPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Context cancelled, stopping watch nodes")
			return
		case <-c.stopCh:
			c.logger.Info("Stop signal received, stopping watch nodes")
			return
		case <-ticker.C:
			if err := c.updateNodes(); err != nil {
				c.logger.Error("Failed to update nodes", zap.Error(err))
				metrics.RecordServiceDiscoveryError(c.config.ServiceName, "update")
			}
		}
	}
}

func (c *consulDiscovery) updateNodes() error {
	// 查询健康的服务实例
	queryOpts := &api.QueryOptions{
		WaitIndex: c.lastIndex,
		WaitTime:  c.config.RefreshPeriod,
	}

	services, meta, err := c.client.Health().Service(
		c.config.ServiceName,
		"",   // tag
		true, // passing only (健康检查通过的)
		queryOpts,
	)
	if err != nil {
		return fmt.Errorf("failed to query consul health service: %w", err)
	}

	c.lastIndex = meta.LastIndex

	// 转换为节点列表
	newNodes := c.convertToNodes(services)

	c.mutex.Lock()
	oldNodesCount := len(c.nodes)
	c.nodes = newNodes
	c.mutex.Unlock()

	// 更新指标
	metrics.UpdateServiceNodes(c.config.ServiceName, len(newNodes))
	if len(newNodes) != oldNodesCount {
		metrics.RecordServiceDiscoveryUpdate(c.config.ServiceName)
		c.logger.Info("Service nodes updated",
			zap.String("service", c.config.ServiceName),
			zap.Int("old_count", oldNodesCount),
			zap.Int("new_count", len(newNodes)))
	}

	// 通知订阅者
	c.notifyCallbacks(newNodes)

	return nil
}

func (c *consulDiscovery) convertToNodes(services []*api.ServiceEntry) []*balancer.Node {
	nodes := make([]*balancer.Node, 0, len(services))

	for _, service := range services {
		// 检查标签过滤
		if !c.matchTags(service.Service.Tags) {
			continue
		}

		address := service.Service.Address
		if address == "" {
			address = service.Node.Address
		}

		// 获取权重（从服务元数据中）
		weight := 1
		if weightStr, exists := service.Service.Meta["weight"]; exists {
			if w := parseWeight(weightStr); w > 0 {
				weight = w
			}
		}

		node := &balancer.Node{
			Address: fmt.Sprintf("%s:%d", address, service.Service.Port),
			Weight:  weight,
			Healthy: c.isHealthy(service),
		}

		nodes = append(nodes, node)
	}

	return nodes
}

func (c *consulDiscovery) matchTags(serviceTags []string) bool {
	if len(c.config.Tags) == 0 {
		return true // 没有配置标签过滤
	}

	serviceTagSet := make(map[string]bool)
	for _, tag := range serviceTags {
		serviceTagSet[tag] = true
	}

	// 检查是否包含所有必需的标签
	for _, requiredTag := range c.config.Tags {
		if !serviceTagSet[requiredTag] {
			return false
		}
	}

	return true
}

func (c *consulDiscovery) isHealthy(service *api.ServiceEntry) bool {
	// 检查节点和服务的健康检查状态
	for _, check := range service.Checks {
		if check.Status != api.HealthPassing {
			return false
		}
	}
	return true
}

func (c *consulDiscovery) notifyCallbacks(nodes []*balancer.Node) {
	c.mutex.RLock()
	callbacks := make([]func([]*balancer.Node), len(c.callbacks))
	copy(callbacks, c.callbacks)
	c.mutex.RUnlock()

	// 异步通知所有回调
	for _, callback := range callbacks {
		go func(cb func([]*balancer.Node)) {
			defer func() {
				if r := recover(); r != nil {
					c.logger.Error("Callback panic recovered", zap.Any("panic", r))
				}
			}()
			cb(nodes)
		}(callback)
	}
}

// parseWeight 解析权重字符串
func parseWeight(weightStr string) int {
	// 简单的权重解析，可以根据需要扩展
	switch weightStr {
	case "1", "low":
		return 1
	case "2", "medium":
		return 2
	case "3", "high":
		return 3
	default:
		return 1
	}
}

// HealthChecker 健康检查器
type HealthChecker struct {
	discovery ServiceDiscovery
	balancer  balancer.Balancer
	config    *config.PoolConfig
	logger    *zap.Logger
	stopCh    chan struct{}
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(discovery ServiceDiscovery, bal balancer.Balancer, cfg *config.PoolConfig, logger *zap.Logger) *HealthChecker {
	return &HealthChecker{
		discovery: discovery,
		balancer:  bal,
		config:    cfg,
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
}

// Start 启动健康检查
func (h *HealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(h.config.HealthCheckPeriod)
	defer ticker.Stop()

	h.logger.Info("Health checker started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.checkHealth()
		}
	}
}

// Stop 停止健康检查
func (h *HealthChecker) Stop() {
	close(h.stopCh)
	h.logger.Info("Health checker stopped")
}

func (h *HealthChecker) checkHealth() {
	nodes := h.balancer.GetNodes()
	unhealthyCount := 0

	for _, node := range nodes {
		if !node.Healthy {
			unhealthyCount++
			continue
		}

		// 这里可以添加自定义的健康检查逻辑
		// 例如：ping检查、gRPC健康检查等
		if h.performHealthCheck(node) {
			metrics.RecordHealthCheck("", node.Address, "success")
		} else {
			h.balancer.SetUnhealthy(node.Address)
			metrics.RecordHealthCheck("", node.Address, "failure")
			unhealthyCount++
		}
	}

	metrics.UpdateUnhealthyNodes("", unhealthyCount)
}

func (h *HealthChecker) performHealthCheck(node *balancer.Node) bool {
	// 简单的健康检查实现
	// 在实际应用中，这里应该实现真正的健康检查逻辑
	// 例如：TCP连接检查、gRPC Health Check等
	return true
}
