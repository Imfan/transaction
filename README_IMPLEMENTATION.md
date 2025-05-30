# gRPC 连接池实现

这是一个基于 Go 语言实现的高性能 gRPC 连接池系统，集成了服务发现、负载均衡、监控和健康检查等功能。

## 功能特性

### 核心功能
- ✅ **连接池管理**: 支持连接预热、复用、自动清理
- ✅ **服务发现**: 集成 Consul，动态感知服务变化
- ✅ **负载均衡**: 轮询和加权轮询算法
- ✅ **健康检查**: 自动检测并剔除不健康节点
- ✅ **TLS 支持**: 可选的加密通信
- ✅ **监控指标**: Prometheus 集成，丰富的指标暴露
- ✅ **滚动发布**: 支持服务平滑升级，连接自动迁移

### 高级特性
- ✅ **多服务管理**: 统一管理多个后端服务的连接
- ✅ **并发安全**: 线程安全的连接池实现
- ✅ **熔断机制**: 快速失败保护
- ✅ **配置热加载**: 支持运行时配置调整
- ✅ **优雅关闭**: 平滑的服务停止流程

## 项目结构

```
transaction/
├── pkg/
│   ├── config/         # 配置管理
│   │   └── config.go
│   ├── pool/           # 连接池核心实现
│   │   ├── connection.go
│   │   ├── pool.go
│   │   └── manager.go
│   ├── discovery/      # 服务发现
│   │   └── consul.go
│   ├── balancer/       # 负载均衡
│   │   └── balancer.go
│   └── metrics/        # 监控指标
│       └── metrics.go
├── examples/           # 示例代码
│   └── main.go
├── cmd/               # 命令行工具
│   └── grpc-pool/
│       └── main.go
├── go.mod
├── go.sum
└── README.md
```

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 运行示例

```bash
# 启动示例应用
go run examples/main.go

# 或者使用 CLI 工具
go run cmd/grpc-pool/main.go start
```

### 3. 查看监控指标

访问以下端点：
- 监控指标: http://localhost:9090/metrics
- 健康检查: http://localhost:9090/health  
- 统计信息: http://localhost:9090/stats

## 配置说明

### 默认配置

```go
cfg := config.DefaultConfig()
```

### 自定义配置

```go
cfg := &config.Config{
    Pool: &config.PoolConfig{
        InitialSize:        5,              // 初始连接数
        MaxSize:           50,              // 最大连接数
        IdleTimeout:       30 * time.Minute, // 空闲超时
        MaxConnAge:        1 * time.Hour,   // 连接最大存活时间
        HealthCheckPeriod: 30 * time.Second, // 健康检查周期
        ConnectTimeout:    5 * time.Second,  // 连接超时
        KeepAlive:         30 * time.Second, // 保活时间
    },
    Discovery: &config.DiscoveryConfig{
        ConsulAddr:    "localhost:8500",     // Consul 地址
        ServiceName:   "your-service",       // 服务名称
        RefreshPeriod: 10 * time.Second,     // 刷新周期
        Tags:          []string{"v1.0"},     // 服务标签
        Datacenter:    "dc1",                // 数据中心
    },
    TLS: &config.TLSConfig{
        Enable:     false,                   // 启用 TLS
        CertFile:   "/path/to/cert.pem",    // 证书文件
        KeyFile:    "/path/to/key.pem",     // 私钥文件
        SkipVerify: false,                   // 跳过证书验证
    },
    Metrics: &config.MetricsConfig{
        Enable: true,                        // 启用监控
        Port:   9090,                       // 监控端口
        Path:   "/metrics",                 // 监控路径
    },
}
```

## 使用方法

### 基本用法

```go
package main

import (
    "context"
    "log"
    
    "transaction/pkg/config"
    "transaction/pkg/pool"
    "go.uber.org/zap"
    "google.golang.org/grpc"
)

func main() {
    // 创建配置
    cfg := config.DefaultConfig()
    
    // 创建 logger
    logger, _ := zap.NewProduction()
    
    // 创建连接池管理器
    manager := pool.NewManager(cfg, logger)
    
    // 启动管理器
    ctx := context.Background()
    if err := manager.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer manager.Stop()
    
    // 使用连接池执行 gRPC 调用
    err := manager.ExecuteWithPool(ctx, "user-service", func(conn *grpc.ClientConn) error {
        // 在这里创建客户端并执行调用
        // client := userpb.NewUserServiceClient(conn)
        // resp, err := client.GetUser(ctx, &userpb.GetUserRequest{Id: "123"})
        return nil
    })
    
    if err != nil {
        log.Printf("Call failed: %v", err)
    }
}
```

### 高级用法

```go
// 获取特定服务的连接池
pool, err := manager.GetPool("user-service")
if err != nil {
    log.Fatal(err)
}

// 手动获取连接
conn, err := pool.Get(ctx)
if err != nil {
    log.Fatal(err)
}
defer pool.Put(conn) // 记得归还连接

// 使用连接
grpcConn := conn.GetConn()
// ... 执行 gRPC 调用
```

## CLI 工具使用

### 构建 CLI 工具

```bash
go build -o grpc-pool cmd/grpc-pool/main.go
```

### 命令说明

```bash
# 启动连接池管理器
./grpc-pool start

# 使用自定义 Consul 地址启动
./grpc-pool -consul-addr=consul.example.com:8500 start

# 查看所有服务的统计信息
./grpc-pool stats

# 查看特定服务的统计信息
./grpc-pool -service=user-service stats

# 检查健康状态
./grpc-pool health

# 检查特定服务的健康状态
./grpc-pool -service=user-service health
```

## 监控指标

系统提供了丰富的 Prometheus 指标：

### 连接池指标
- `grpc_pool_size`: 当前连接池大小
- `grpc_pool_active_connections`: 活跃连接数
- `grpc_pool_idle_connections`: 空闲连接数
- `grpc_connections_created_total`: 创建的连接总数
- `grpc_connections_destroyed_total`: 销毁的连接总数

### 请求指标
- `grpc_requests_total`: gRPC 请求总数
- `grpc_request_duration_seconds`: 请求持续时间

### 服务发现指标
- `grpc_service_discovery_nodes`: 发现的节点数量
- `grpc_service_discovery_updates_total`: 服务发现更新次数
- `grpc_service_discovery_errors_total`: 服务发现错误数

### 健康检查指标
- `grpc_health_checks_total`: 健康检查总数
- `grpc_unhealthy_nodes`: 不健康节点数量

## 部署指南

### Docker 部署

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o grpc-pool cmd/grpc-pool/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/grpc-pool .
CMD ["./grpc-pool", "start"]
```

### Kubernetes 部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grpc-pool
spec:
  replicas: 1
  selector:
    matchLabels:
      app: grpc-pool
  template:
    metadata:
      labels:
        app: grpc-pool
    spec:
      containers:
      - name: grpc-pool
        image: grpc-pool:latest
        ports:
        - containerPort: 9090
        env:
        - name: CONSUL_ADDR
          value: "consul:8500"
        resources:
          requests:
            memory: "64Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "100m"
---
apiVersion: v1
kind: Service
metadata:
  name: grpc-pool
spec:
  selector:
    app: grpc-pool
  ports:
  - port: 9090
    targetPort: 9090
```

## 性能调优

### 连接池配置

```go
// 高并发场景
cfg.Pool.InitialSize = 10
cfg.Pool.MaxSize = 100
cfg.Pool.IdleTimeout = 5 * time.Minute

// 低延迟场景
cfg.Pool.ConnectTimeout = 1 * time.Second
cfg.Pool.KeepAlive = 10 * time.Second
cfg.Pool.HealthCheckPeriod = 10 * time.Second
```

### 服务发现优化

```go
// 频繁变化的环境
cfg.Discovery.RefreshPeriod = 5 * time.Second

// 稳定的环境
cfg.Discovery.RefreshPeriod = 30 * time.Second
```

## 故障排查

### 常见问题

1. **连接池满**
   - 检查 `grpc_pool_size` 指标
   - 增加 `MaxSize` 配置
   - 检查连接是否正确归还

2. **服务发现失败**
   - 检查 Consul 连接
   - 验证服务名称和标签
   - 查看 `grpc_service_discovery_errors_total` 指标

3. **健康检查失败**
   - 检查目标服务的健康状态
   - 验证网络连接
   - 查看 `grpc_health_checks_total` 指标

### 日志分析

```bash
# 查看连接池状态
curl http://localhost:9090/stats

# 查看健康状态
curl http://localhost:9090/health

# 查看 Prometheus 指标
curl http://localhost:9090/metrics
```

## 最佳实践

1. **连接池大小设置**
   - 初始大小设为预期并发数的 50%
   - 最大大小设为预期并发数的 2-3 倍

2. **超时配置**
   - 连接超时不要超过 10 秒
   - 空闲超时建议 5-30 分钟

3. **监控告警**
   - 设置连接池使用率告警（> 80%）
   - 监控连接创建失败率
   - 关注服务发现错误

4. **服务发现**
   - 使用合适的刷新周期
   - 配置正确的服务标签
   - 确保 Consul 高可用

## 扩展开发

### 添加新的负载均衡算法

```go
// 实现 Balancer 接口
type CustomBalancer struct {
    // your implementation
}

func (c *CustomBalancer) Select() (*Node, error) {
    // your algorithm
}
```

### 添加新的服务发现机制

```go
// 实现 ServiceDiscovery 接口
type EtcdDiscovery struct {
    // your implementation
}

func (e *EtcdDiscovery) Start(ctx context.Context) error {
    // your implementation
}
```

### 自定义监控指标

```go
import "github.com/prometheus/client_golang/prometheus"

var customMetric = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "custom_metric_total",
        Help: "Custom metric description",
    },
    []string{"label1", "label2"},
)
```

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！ 