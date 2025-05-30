package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// 连接池指标
	PoolSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "grpc_pool_size",
		Help: "当前连接池中的连接数",
	}, []string{"service", "target"})

	PoolActiveConnections = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "grpc_pool_active_connections",
		Help: "当前活跃的连接数",
	}, []string{"service", "target"})

	PoolIdleConnections = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "grpc_pool_idle_connections",
		Help: "当前空闲的连接数",
	}, []string{"service", "target"})

	// 连接操作指标
	ConnectionsCreated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_connections_created_total",
		Help: "创建的连接总数",
	}, []string{"service", "target"})

	ConnectionsDestroyed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_connections_destroyed_total",
		Help: "销毁的连接总数",
	}, []string{"service", "target"})

	ConnectionErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_connection_errors_total",
		Help: "连接错误总数",
	}, []string{"service", "target", "error_type"})

	// 请求指标
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_requests_total",
		Help: "gRPC请求总数",
	}, []string{"service", "target", "method", "status"})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_request_duration_seconds",
		Help:    "gRPC请求持续时间",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "target", "method"})

	// 负载均衡指标
	LoadBalancerRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_load_balancer_requests_total",
		Help: "负载均衡处理的请求总数",
	}, []string{"service", "target", "algorithm"})

	// 服务发现指标
	ServiceDiscoveryUpdates = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_service_discovery_updates_total",
		Help: "服务发现更新次数",
	}, []string{"service"})

	ServiceDiscoveryNodes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "grpc_service_discovery_nodes",
		Help: "服务发现的节点数量",
	}, []string{"service"})

	ServiceDiscoveryErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_service_discovery_errors_total",
		Help: "服务发现错误总数",
	}, []string{"service", "error_type"})

	// 健康检查指标
	HealthChecksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_health_checks_total",
		Help: "健康检查总数",
	}, []string{"service", "target", "status"})

	UnhealthyNodes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "grpc_unhealthy_nodes",
		Help: "不健康的节点数量",
	}, []string{"service"})
)

// RecordConnectionCreated 记录连接创建
func RecordConnectionCreated(service, target string) {
	ConnectionsCreated.WithLabelValues(service, target).Inc()
}

// RecordConnectionDestroyed 记录连接销毁
func RecordConnectionDestroyed(service, target string) {
	ConnectionsDestroyed.WithLabelValues(service, target).Inc()
}

// RecordConnectionError 记录连接错误
func RecordConnectionError(service, target, errorType string) {
	ConnectionErrors.WithLabelValues(service, target, errorType).Inc()
}

// UpdatePoolSize 更新连接池大小
func UpdatePoolSize(service, target string, size int) {
	PoolSize.WithLabelValues(service, target).Set(float64(size))
}

// UpdateActiveConnections 更新活跃连接数
func UpdateActiveConnections(service, target string, count int) {
	PoolActiveConnections.WithLabelValues(service, target).Set(float64(count))
}

// UpdateIdleConnections 更新空闲连接数
func UpdateIdleConnections(service, target string, count int) {
	PoolIdleConnections.WithLabelValues(service, target).Set(float64(count))
}

// RecordRequest 记录请求
func RecordRequest(service, target, method, status string, duration float64) {
	RequestsTotal.WithLabelValues(service, target, method, status).Inc()
	RequestDuration.WithLabelValues(service, target, method).Observe(duration)
}

// RecordLoadBalancerRequest 记录负载均衡请求
func RecordLoadBalancerRequest(service, target, algorithm string) {
	LoadBalancerRequests.WithLabelValues(service, target, algorithm).Inc()
}

// RecordServiceDiscoveryUpdate 记录服务发现更新
func RecordServiceDiscoveryUpdate(service string) {
	ServiceDiscoveryUpdates.WithLabelValues(service).Inc()
}

// UpdateServiceNodes 更新服务节点数
func UpdateServiceNodes(service string, count int) {
	ServiceDiscoveryNodes.WithLabelValues(service).Set(float64(count))
}

// RecordServiceDiscoveryError 记录服务发现错误
func RecordServiceDiscoveryError(service, errorType string) {
	ServiceDiscoveryErrors.WithLabelValues(service, errorType).Inc()
}

// RecordHealthCheck 记录健康检查
func RecordHealthCheck(service, target, status string) {
	HealthChecksTotal.WithLabelValues(service, target, status).Inc()
}

// UpdateUnhealthyNodes 更新不健康节点数
func UpdateUnhealthyNodes(service string, count int) {
	UnhealthyNodes.WithLabelValues(service).Set(float64(count))
} 