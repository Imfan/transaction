# gRPC Connection Pool

这是一个用 Go 语言实现的 gRPC 连接池。它提供了以下特性：

- 支持最小和最大连接数配置
- 自动管理连接的生命周期
- 支持连接空闲超时
- 自动清理无效连接
- 线程安全

## 安装

```bash
go get github.com/yourusername/grpc-pool
```

## 使用方法

```go
import (
    "grpc-pool/pool"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

// 创建连接池配置
config := pool.Config{
    MinConns:    2,                // 最小连接数
    MaxConns:    10,               // 最大连接数
    IdleTimeout: 1 * time.Minute,  // 空闲超时时间
    Address:     "localhost:50051", // gRPC 服务器地址
    Options: []grpc.DialOption{
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    },
}

// 创建连接池
pool, err := pool.NewPool(config)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()

// 获取连接
conn, err := pool.Get(ctx)
if err != nil {
    // 处理错误
}

// 使用连接
// ...

// 将连接放回池中
pool.Put(conn)
```

## 特性

1. **连接管理**
   - 自动创建和销毁连接
   - 维护最小连接数
   - 限制最大连接数

2. **健康检查**
   - 自动检测连接状态
   - 移除无效连接
   - 自动重建连接

3. **性能优化**
   - 连接复用
   - 自动清理空闲连接
   - 线程安全

4. **可配置选项**
   - 最小连接数
   - 最大连接数
   - 空闲超时时间
   - gRPC 连接选项

## 注意事项

1. 使用完连接后，必须调用 `Put` 方法将连接放回池中
2. 程序退出时，应该调用 `Close` 方法关闭连接池
3. 连接池会自动处理连接的健康检查和清理工作