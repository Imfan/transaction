# gRPC连接池生产环境使用指南

## 🚀 快速开始

### 1. 环境准备

#### 必需组件
- **Go 1.21+**：确保Go环境已安装
- **Consul**：用于服务发现（可选：支持其他服务发现机制）
- **gRPC服务**：您的实际gRPC服务实例

#### 可选组件
- **Prometheus**：用于指标收集
- **Grafana**：用于指标可视化

### 2. 配置文件

#### 生产配置 (`production_config.json`)
```json
{
  "pool": {
    "initial_size": 10,          // 初始连接数
    "max_size": 100,             // 最大连接数
    "idle_timeout": "30m",       // 空闲超时
    "max_conn_age": "2h",        // 连接最大生存时间
    "health_check_period": "30s", // 健康检查间隔
    "connect_timeout": "10s",    // 连接超时
    "keep_alive": "60s"          // 保活时间
  },
  "discovery": {
    "consul_addr": "localhost:8500",     // Consul地址
    "service_name": "user-service",      // 服务名称
    "refresh_period": "15s",             // 刷新周期
    "tags": ["production", "v1.0"],      // 服务标签
    "datacenter": "dc1",                 // 数据中心
    "token": ""                          // Consul Token
  },
  "tls": {
    "enable": false,             // 是否启用TLS
    "cert_file": "",            // 证书文件路径
    "key_file": "",             // 私钥文件路径
    "ca_file": "",              // CA证书路径
    "server_name": "",          // 服务器名称
    "skip_verify": false        // 跳过证书验证
  },
  "metrics": {
    "enable": true,             // 启用监控
    "port": 9090,               // 监控端口
    "path": "/metrics"          // 监控路径
  }
}
```

### 3. 启动方式

#### 方式一：使用启动脚本（推荐）
```bash
# Windows
start_production.bat

# 选择选项：
# 1. 启动完整生产应用（包含示例）
# 2. 仅启动CLI管理器
# 3. 查看连接统计
# 4. 检查健康状态
```

#### 方式二：直接运行
```bash
# 启动生产应用
go run production_example.go

# 或启动CLI管理器
go build -o grpc-pool.exe cmd/grpc-pool/main.go
./grpc-pool.exe -config production_config.json start
```

## 📋 使用方法

### 基本集成

#### 1. 在您的应用中集成连接池
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
    // 创建logger
    logger, _ := zap.NewProduction()
    
    // 加载配置
    cfg := config.DefaultConfig()
    cfg.Discovery.ServiceName = "your-service"
    cfg.Discovery.ConsulAddr = "your-consul:8500"
    
    // 创建连接池管理器
    manager := pool.NewManager(cfg, logger)
    
    // 启动管理器
    ctx := context.Background()
    if err := manager.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer manager.Stop()
    
    // 使用连接池
    err := manager.ExecuteWithPool(ctx, "user-service", func(conn *grpc.ClientConn) error {
        // 在这里使用gRPC连接
        // client := yourpb.NewYourServiceClient(conn)
        // resp, err := client.YourMethod(ctx, &yourpb.YourRequest{})
        return nil
    })
    
    if err != nil {
        log.Printf("调用失败: %v", err)
    }
}
```

#### 2. 高级用法：手动管理连接
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
defer pool.Put(conn) // 重要：归还连接

// 使用连接
grpcConn := conn.GetConn()
client := yourpb.NewYourServiceClient(grpcConn)
resp, err := client.YourMethod(ctx, &yourpb.YourRequest{})
```

### CLI工具使用

#### 1. 基本命令
```bash
# 启动连接池管理器
./grpc-pool.exe start

# 使用自定义配置启动
./grpc-pool.exe -config production_config.json start

# 查看所有服务统计
./grpc-pool.exe stats

# 查看特定服务统计
./grpc-pool.exe -service user-service stats

# 检查健康状态
./grpc-pool.exe health

# 检查特定服务健康状态
./grpc-pool.exe -service user-service health
```

#### 2. 配置参数
```bash
# 自定义Consul地址
./grpc-pool.exe -consul-addr=consul.example.com:8500 start

# 自定义监控端口
./grpc-pool.exe -metrics-port=9091 start

# 组合使用
./grpc-pool.exe -consul-addr=consul.example.com:8500 -metrics-port=9091 start
```

## 🔧 配置详解

### 连接池配置优化

#### 高并发场景
```json
{
  "pool": {
    "initial_size": 20,
    "max_size": 200,
    "idle_timeout": "10m",
    "max_conn_age": "1h",
    "health_check_period": "15s",
    "connect_timeout": "5s",
    "keep_alive": "30s"
  }
}
```

#### 低延迟场景
```json
{
  "pool": {
    "initial_size": 15,
    "max_size": 50,
    "idle_timeout": "5m",
    "max_conn_age": "30m",
    "health_check_period": "10s",
    "connect_timeout": "2s",
    "keep_alive": "15s"
  }
}
```

#### 资源受限场景
```json
{
  "pool": {
    "initial_size": 3,
    "max_size": 10,
    "idle_timeout": "1h",
    "max_conn_age": "4h",
    "health_check_period": "60s",
    "connect_timeout": "10s",
    "keep_alive": "120s"
  }
}
```

### TLS配置

#### 启用TLS
```json
{
  "tls": {
    "enable": true,
    "cert_file": "/path/to/client.crt",
    "key_file": "/path/to/client.key",
    "ca_file": "/path/to/ca.crt",
    "server_name": "your-service.example.com",
    "skip_verify": false
  }
}
```

## 📊 监控与运维

### 监控端点

#### Prometheus指标
```bash
# 访问指标
curl http://localhost:9090/metrics

# 主要指标：
# grpc_pool_size - 连接池大小
# grpc_pool_active_connections - 活跃连接数
# grpc_pool_idle_connections - 空闲连接数
# grpc_requests_total - 请求总数
# grpc_request_duration_seconds - 请求持续时间
```

#### 健康检查
```bash
# 检查整体健康状态
curl http://localhost:9090/health

# 响应示例：
# {"status":"healthy","pools":["user-service","order-service"]}
```

#### 统计信息
```bash
# 获取详细统计
curl http://localhost:9090/stats

# 响应示例：
# {"pools":{"user-service":{"total":10,"active":3,"idle":7}}}
```

### 日志监控

#### 关键日志模式
```bash
# 连接池错误
grep "Failed to get connection" /var/log/app.log

# 服务发现问题
grep "service discovery" /var/log/app.log

# 健康检查失败
grep "health check failed" /var/log/app.log
```

### 告警配置

#### Prometheus告警规则
```yaml
groups:
- name: grpc_pool
  rules:
  - alert: PoolSizeHigh
    expr: grpc_pool_size > 80
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "连接池大小过高"
      
  - alert: HighErrorRate
    expr: rate(grpc_requests_total{status="error"}[5m]) > 0.1
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "gRPC请求错误率过高"
```

## 🔍 故障排查

### 常见问题

#### 1. 连接池满
```bash
# 症状：获取连接超时
# 检查：
curl http://localhost:9090/stats

# 解决方案：
# - 增加 max_size 配置
# - 检查连接是否正确归还
# - 优化业务逻辑，减少连接持有时间
```

#### 2. 服务发现失败
```bash
# 症状：找不到健康节点
# 检查：
curl http://localhost:8500/v1/health/service/user-service

# 解决方案：
# - 验证Consul连接
# - 检查服务注册状态
# - 确认标签过滤配置
```

#### 3. 健康检查失败
```bash
# 症状：节点被标记为不健康
# 检查：连接池日志和目标服务状态
# 解决方案：
# - 检查网络连接
# - 验证服务端口
# - 调整健康检查超时时间
```

### 性能调优

#### 1. 连接数优化
```go
// 计算公式：
// initial_size = 预期并发请求数 * 0.5
// max_size = 预期并发请求数 * 2.0

// 例：预期100并发
initial_size: 50
max_size: 200
```

#### 2. 超时配置
```go
// 网络稳定环境
connect_timeout: "5s"
keep_alive: "60s"

// 网络不稳定环境
connect_timeout: "10s" 
keep_alive: "30s"
```

## 🚀 部署建议

### Docker部署
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o grpc-pool cmd/grpc-pool/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/grpc-pool .
COPY production_config.json .
CMD ["./grpc-pool", "-config", "production_config.json", "start"]
```

### Kubernetes部署
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grpc-pool
spec:
  replicas: 3
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
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "200m"
        livenessProbe:
          httpGet:
            path: /health
            port: 9090
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 9090
          initialDelaySeconds: 5
          periodSeconds: 5
```

## 📞 技术支持

### 获取帮助
```bash
# 查看帮助信息
./grpc-pool.exe -help

# 查看版本信息
./grpc-pool.exe -version
```

### 社区支持
- GitHub Issues: 提交bug报告和功能请求
- 文档: 查看完整技术文档
- 示例: 参考examples目录中的示例代码

这个生产指南为您提供了完整的gRPC连接池部署和使用方案！ 