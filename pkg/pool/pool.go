package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"transaction/pkg/balancer"
	"transaction/pkg/config"
	"transaction/pkg/discovery"
	"transaction/pkg/metrics"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Pool gRPC连接池接口
type Pool interface {
	// Get 获取连接
	Get(ctx context.Context) (*Connection, error)
	// Put 归还连接
	Put(conn *Connection)
	// GetTarget 获取目标地址
	GetTarget() string
	// Close 关闭连接池
	Close() error
	// Stats 获取连接池统计信息
	Stats() PoolStats
}

// PoolStats 连接池统计信息
type PoolStats struct {
	TotalConnections  int `json:"total_connections"`
	ActiveConnections int `json:"active_connections"`
	IdleConnections   int `json:"idle_connections"`
}

// grpcPool gRPC连接池实现
type grpcPool struct {
	serviceName   string
	config        *config.Config
	balancer      balancer.Balancer
	discovery     discovery.ServiceDiscovery
	healthChecker *discovery.HealthChecker
	logger        *zap.Logger

	// 连接管理
	connections map[string][]*Connection // target -> connections
	connChan    chan *Connection        // 连接通道
	mutex       sync.RWMutex

	// 控制通道
	stopCh chan struct{}
	closed bool

	// 统计信息
	stats PoolStats
}

// NewPool 创建新的连接池
func NewPool(serviceName string, cfg *config.Config, logger *zap.Logger) (Pool, error) {
	// 创建负载均衡器
	bal := balancer.NewBalancer(balancer.RoundRobin)

	// 创建服务发现
	disc, err := discovery.NewConsulDiscovery(cfg.Discovery, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create service discovery: %w", err)
	}

	// 创建健康检查器
	healthChecker := discovery.NewHealthChecker(disc, bal, cfg.Pool, logger)

	pool := &grpcPool{
		serviceName:   serviceName,
		config:        cfg,
		balancer:      bal,
		discovery:     disc,
		healthChecker: healthChecker,
		logger:        logger,
		connections:   make(map[string][]*Connection),
		connChan:      make(chan *Connection, cfg.Pool.MaxSize),
		stopCh:        make(chan struct{}),
	}

	// 订阅服务发现变化
	disc.Subscribe(pool.onNodesChanged)

	return pool, nil
}

// Start 启动连接池
func (p *grpcPool) Start(ctx context.Context) error {
	p.logger.Info("Starting gRPC connection pool", zap.String("service", p.serviceName))

	// 启动服务发现
	if err := p.discovery.Start(ctx); err != nil {
		return fmt.Errorf("failed to start service discovery: %w", err)
	}

	// 启动健康检查
	go p.healthChecker.Start(ctx)

	// 启动连接清理
	go p.startConnectionCleaner(ctx)

	p.logger.Info("gRPC connection pool started successfully")
	return nil
}

func (p *grpcPool) Get(ctx context.Context) (*Connection, error) {
	// 尝试从通道获取空闲连接
	select {
	case conn := <-p.connChan:
		if conn.IsHealthy() {
			conn.MarkAsUsed()
			return conn, nil
		}
		// 连接不健康，关闭并创建新连接
		conn.Close()
		metrics.RecordConnectionDestroyed(p.serviceName, conn.GetTarget())
	default:
		// 没有空闲连接
	}

	// 创建新连接
	return p.createConnection(ctx)
}

func (p *grpcPool) Put(conn *Connection) {
	if conn == nil {
		return
	}

	conn.MarkAsIdle()

	// 检查连接是否健康
	if !conn.IsHealthy() {
		conn.Close()
		metrics.RecordConnectionDestroyed(p.serviceName, conn.GetTarget())
		return
	}

	// 尝试放回连接池
	select {
	case p.connChan <- conn:
		// 成功放回连接池
	default:
		// 连接池已满，关闭连接
		conn.Close()
		metrics.RecordConnectionDestroyed(p.serviceName, conn.GetTarget())
	}
}

func (p *grpcPool) GetTarget() string {
	return p.serviceName
}

func (p *grpcPool) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return nil
	}

	p.logger.Info("Closing gRPC connection pool", zap.String("service", p.serviceName))

	close(p.stopCh)
	p.closed = true

	// 关闭所有连接
	close(p.connChan)
	for conn := range p.connChan {
		conn.Close()
	}

	// 关闭目标连接
	for target, conns := range p.connections {
		for _, conn := range conns {
			conn.Close()
		}
		delete(p.connections, target)
	}

	// 停止服务发现和健康检查
	p.discovery.Stop()
	p.healthChecker.Stop()

	p.logger.Info("gRPC connection pool closed")
	return nil
}

func (p *grpcPool) Stats() PoolStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	stats := PoolStats{}
	
	for _, conns := range p.connections {
		for _, conn := range conns {
			stats.TotalConnections++
			if conn.GetState() == StateActive {
				stats.ActiveConnections++
			} else if conn.GetState() == StateIdle {
				stats.IdleConnections++
			}
		}
	}

	return stats
}

func (p *grpcPool) createConnection(ctx context.Context) (*Connection, error) {
	// 选择目标节点
	node, err := p.balancer.Select()
	if err != nil {
		return nil, fmt.Errorf("failed to select node: %w", err)
	}

	// 记录负载均衡请求
	metrics.RecordLoadBalancerRequest(p.serviceName, node.Address, string(p.balancer.Algorithm()))

	// 创建gRPC连接
	grpcConn, err := p.createGRPCConnection(ctx, node.Address)
	if err != nil {
		metrics.RecordConnectionError(p.serviceName, node.Address, "create")
		return nil, fmt.Errorf("failed to create grpc connection to %s: %w", node.Address, err)
	}

	conn := NewConnection(grpcConn, node.Address)
	conn.MarkAsUsed()

	// 记录连接创建
	metrics.RecordConnectionCreated(p.serviceName, node.Address)

	// 添加到连接映射
	p.mutex.Lock()
	p.connections[node.Address] = append(p.connections[node.Address], conn)
	p.mutex.Unlock()

	p.logger.Debug("Created new gRPC connection",
		zap.String("service", p.serviceName),
		zap.String("target", node.Address))

	return conn, nil
}

func (p *grpcPool) createGRPCConnection(ctx context.Context, target string) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption

	// 设置TLS
	if p.config.TLS.Enable {
		tlsConfig, err := p.config.TLS.GetTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS config: %w", err)
		}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// 设置keepalive
	kacp := keepalive.ClientParameters{
		Time:                p.config.Pool.KeepAlive,
		Timeout:             p.config.Pool.ConnectTimeout,
		PermitWithoutStream: true,
	}
	opts = append(opts, grpc.WithKeepaliveParams(kacp))

	// 设置连接超时
	ctx, cancel := context.WithTimeout(ctx, p.config.Pool.ConnectTimeout)
	defer cancel()

	return grpc.DialContext(ctx, target, opts...)
}

func (p *grpcPool) onNodesChanged(nodes []*balancer.Node) {
	p.logger.Info("Service nodes changed",
		zap.String("service", p.serviceName),
		zap.Int("node_count", len(nodes)))

	// 更新负载均衡器
	p.balancer.Update(nodes)

	// 清理不再存在的节点的连接
	p.cleanupObsoleteConnections(nodes)

	// 预热新节点的连接
	p.preheatConnections(nodes)
}

func (p *grpcPool) cleanupObsoleteConnections(nodes []*balancer.Node) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 构建当前节点地址集合
	currentNodes := make(map[string]bool)
	for _, node := range nodes {
		currentNodes[node.Address] = true
	}

	// 清理不再存在的节点连接
	for target, conns := range p.connections {
		if !currentNodes[target] {
			p.logger.Info("Removing obsolete connections",
				zap.String("service", p.serviceName),
				zap.String("target", target),
				zap.Int("connection_count", len(conns)))

			for _, conn := range conns {
				conn.Close()
				metrics.RecordConnectionDestroyed(p.serviceName, target)
			}
			delete(p.connections, target)
		}
	}
}

func (p *grpcPool) preheatConnections(nodes []*balancer.Node) {
	for _, node := range nodes {
		p.mutex.RLock()
		existingConns := len(p.connections[node.Address])
		p.mutex.RUnlock()

		// 如果连接数少于初始大小，创建新连接
		if existingConns < p.config.Pool.InitialSize {
			needed := p.config.Pool.InitialSize - existingConns
			for i := 0; i < needed; i++ {
				go func(target string) {
					ctx, cancel := context.WithTimeout(context.Background(), p.config.Pool.ConnectTimeout)
					defer cancel()

					grpcConn, err := p.createGRPCConnection(ctx, target)
					if err != nil {
						p.logger.Warn("Failed to preheat connection",
							zap.String("target", target),
							zap.Error(err))
						return
					}

					conn := NewConnection(grpcConn, target)
					
					p.mutex.Lock()
					p.connections[target] = append(p.connections[target], conn)
					p.mutex.Unlock()

					// 放入连接池
					select {
					case p.connChan <- conn:
						metrics.RecordConnectionCreated(p.serviceName, target)
					default:
						// 连接池已满
						conn.Close()
					}
				}(node.Address)
			}
		}
	}
}

func (p *grpcPool) startConnectionCleaner(ctx context.Context) {
	ticker := time.NewTicker(p.config.Pool.HealthCheckPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.cleanupConnections()
		}
	}
}

func (p *grpcPool) cleanupConnections() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for target, conns := range p.connections {
		var activeConns []*Connection
		
		for _, conn := range conns {
			// 检查连接是否需要清理
			shouldCleanup := false
			
			// 检查连接年龄
			if conn.GetAge() > p.config.Pool.MaxConnAge {
				shouldCleanup = true
			}
			
			// 检查空闲时间
			if conn.IsIdle() && conn.GetIdleTime() > p.config.Pool.IdleTimeout {
				shouldCleanup = true
			}
			
			// 检查连接健康状态
			if !conn.IsHealthy() {
				shouldCleanup = true
			}

			if shouldCleanup {
				conn.Close()
				metrics.RecordConnectionDestroyed(p.serviceName, target)
				p.logger.Debug("Cleaned up connection",
					zap.String("service", p.serviceName),
					zap.String("target", target),
					zap.Duration("age", conn.GetAge()),
					zap.Duration("idle_time", conn.GetIdleTime()))
			} else {
				activeConns = append(activeConns, conn)
			}
		}
		
		p.connections[target] = activeConns
	}

	// 更新指标
	p.updateMetrics()
}

func (p *grpcPool) updateMetrics() {
	stats := p.Stats()
	
	for target := range p.connections {
		metrics.UpdatePoolSize(p.serviceName, target, stats.TotalConnections)
		metrics.UpdateActiveConnections(p.serviceName, target, stats.ActiveConnections)
		metrics.UpdateIdleConnections(p.serviceName, target, stats.IdleConnections)
	}
} 