# gRPC服务使用指南

## 🚀 快速启动

### 1. 启动所有服务

```bash
# Windows
start_services.bat

# 或者手动启动每个服务
cd services/user-service && go run main.go 8001 v1.0
cd services/order-service && go run main.go 8003 v1.0  
cd services/payment-service && go run main.go 8005 v1.0
```

### 2. 运行生产示例

等待服务启动后（约5秒），运行：

```bash
go run production_example.go
```

## 📋 服务架构

### 服务列表

| 服务名称 | 端口 | 健康检查端口 | 功能描述 |
|---------|-----|-------------|----------|
| **用户服务** | 8001, 8002 | 9001, 9002 | 用户管理、注册、查询 |
| **订单服务** | 8003, 8004 | 9003, 9004 | 订单创建、查询、管理 |
| **支付服务** | 8005, 8006 | 9005, 9006 | 支付处理、查询、退款 |

### 负载均衡

每个服务运行**2个实例**，连接池会自动：
- 🔄 轮询分发请求
- 🩺 健康检查监控
- ⚡ 自动故障转移
- 📊 连接复用优化

## 🔧 API说明

### 用户服务 (UserService)

#### GetUser - 获取用户信息
```go
req := &proto.GetUserRequest{
    UserId: "user_123",
}
resp, err := userClient.GetUser(ctx, req)
```

#### CreateUser - 创建用户
```go
req := &proto.CreateUserRequest{
    Name:  "John Doe",
    Email: "john.doe@example.com", 
    Age:   30,
}
resp, err := userClient.CreateUser(ctx, req)
```

### 订单服务 (OrderService)

#### GetOrder - 获取订单信息
```go
req := &proto.GetOrderRequest{
    OrderId: "order_456",
}
resp, err := orderClient.GetOrder(ctx, req)
```

#### CreateOrder - 创建订单
```go
req := &proto.CreateOrderRequest{
    UserId: "user_123",
    Items: []proto.OrderItem{
        {
            ProductId:   "laptop_001",
            ProductName: "Gaming Laptop",
            Quantity:    1,
            Price:       1299.99,
        },
    },
}
resp, err := orderClient.CreateOrder(ctx, req)
```

#### ListOrders - 获取订单列表
```go
req := &proto.ListOrdersRequest{
    UserId: "user_123",
    Limit:  10,
}
resp, err := orderClient.ListOrders(ctx, req)
```

### 支付服务 (PaymentService)

#### ProcessPayment - 处理支付
```go
req := &proto.ProcessPaymentRequest{
    OrderId:       "order_456",
    UserId:        "user_123",
    Amount:        99.99,
    PaymentMethod: "credit_card",
}
resp, err := paymentClient.ProcessPayment(ctx, req)
```

#### GetPayment - 获取支付信息
```go
req := &proto.GetPaymentRequest{
    PaymentId: "pay_123",
}
resp, err := paymentClient.GetPayment(ctx, req)
```

#### RefundPayment - 处理退款
```go
req := &proto.RefundPaymentRequest{
    PaymentId: "pay_123",
    Amount:    50.00,
    Reason:    "Customer request",
}
resp, err := paymentClient.RefundPayment(ctx, req)
```

## 💡 使用连接池的代码示例

### 基本用法

```go
// 通过连接池调用用户服务
err := manager.ExecuteWithPool(ctx, "user-service", func(conn *grpc.ClientConn) error {
    client := proto.NewUserServiceClient(conn)
    resp, err := client.GetUser(ctx, &proto.GetUserRequest{
        UserId: "user_123",
    })
    if err != nil {
        return err
    }
    
    log.Printf("User: %s (%s)", resp.Name, resp.Email)
    return nil
})
```

### 完整业务流程

```go
func CompleteBusinessFlow(ctx context.Context, manager *pool.Manager) error {
    // 1. 创建用户
    var userId string
    err := manager.ExecuteWithPool(ctx, "user-service", func(conn *grpc.ClientConn) error {
        client := proto.NewUserServiceClient(conn)
        resp, err := client.CreateUser(ctx, &proto.CreateUserRequest{
            Name: "Alice", Email: "alice@example.com", Age: 28,
        })
        if err != nil {
            return err
        }
        userId = resp.UserId
        return nil
    })
    if err != nil {
        return err
    }

    // 2. 创建订单
    var orderId string  
    err = manager.ExecuteWithPool(ctx, "order-service", func(conn *grpc.ClientConn) error {
        client := proto.NewOrderServiceClient(conn)
        resp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
            UserId: userId,
            Items: []proto.OrderItem{{
                ProductId: "laptop_001", ProductName: "Laptop", 
                Quantity: 1, Price: 1299.99,
            }},
        })
        if err != nil {
            return err
        }
        orderId = resp.OrderId
        return nil
    })
    if err != nil {
        return err
    }

    // 3. 处理支付
    return manager.ExecuteWithPool(ctx, "payment-service", func(conn *grpc.ClientConn) error {
        client := proto.NewPaymentServiceClient(conn)
        _, err := client.ProcessPayment(ctx, &proto.ProcessPaymentRequest{
            OrderId: orderId, UserId: userId, Amount: 1299.99, PaymentMethod: "credit_card",
        })
        return err
    })
}
```

## 🩺 健康检查

### HTTP健康检查端点

每个服务都提供HTTP健康检查：

```bash
# 用户服务
curl http://localhost:9001/health
curl http://localhost:9002/health

# 订单服务  
curl http://localhost:9003/health
curl http://localhost:9004/health

# 支付服务
curl http://localhost:9005/health
curl http://localhost:9006/health
```

### 响应示例

```json
{
  "status": "healthy",
  "service": "user-service", 
  "version": "v1.0",
  "server_id": "user-server-8001",
  "timestamp": "2024-01-20T10:30:00Z",
  "user_count": 5
}
```

## 📊 监控指标

连接池提供详细的监控指标：

```bash
# Prometheus指标
curl http://localhost:9090/metrics

# 连接池统计
curl http://localhost:9090/stats

# 健康状态
curl http://localhost:9090/health
```

## 🔍 故障模拟

### 1. 停止某个服务实例

关闭其中一个服务窗口，观察连接池自动故障转移。

### 2. 重启服务

重新启动服务，观察连接池自动恢复。

### 3. 模拟高负载

运行多个客户端实例，观察负载分布：

```bash
# 终端1
go run production_example.go

# 终端2  
go run production_example.go

# 终端3
go run production_example.go
```

## ⚙️ 配置调优

### 高并发场景

修改 `production_config.json`：

```json
{
  "pool": {
    "initial_size": 10,
    "max_size": 100,
    "idle_timeout": "10m",
    "health_check_period": "15s"
  }
}
```

### 低延迟场景

```json
{
  "pool": {
    "initial_size": 15,
    "max_size": 30,
    "idle_timeout": "5m", 
    "health_check_period": "10s",
    "connect_timeout": "2s"
  }
}
```

## 🚨 常见问题

### Q: 服务启动失败
**A:** 检查端口是否被占用，或尝试修改端口号。

### Q: 连接失败
**A:** 确保所有服务已启动，等待5-10秒后重试。

### Q: 性能问题
**A:** 调整连接池大小和超时配置。

### Q: 健康检查失败
**A:** 检查防火墙设置和网络连接。

## 📞 技术支持

- 查看日志输出了解详细错误信息
- 使用健康检查端点验证服务状态
- 监控连接池指标了解性能状况

**恭喜！您现在已经拥有了一个完整的微服务架构演示系统！** 🎉 