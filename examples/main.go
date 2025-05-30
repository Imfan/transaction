package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"transaction/pkg/config"
	"transaction/pkg/pool"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	// 创建logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// 创建配置
	cfg := config.DefaultConfig()
	
	// 可以通过环境变量或配置文件覆盖默认配置
	if consulAddr := os.Getenv("CONSUL_ADDR"); consulAddr != "" {
		cfg.Discovery.ConsulAddr = consulAddr
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		logger.Fatal("Invalid configuration", zap.Error(err))
	}

	// 创建连接池管理器
	manager := pool.NewManager(cfg, logger)

	// 启动管理器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		logger.Fatal("Failed to start pool manager", zap.Error(err))
	}

	// 示例：使用连接池调用服务
	go demonstratePoolUsage(ctx, manager, logger)

	// 示例：监控连接池状态
	go monitorPoolStats(ctx, manager, logger)

	// 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("gRPC Connection Pool Example started")
	logger.Info("Metrics available at http://localhost:9090/metrics")
	logger.Info("Health check available at http://localhost:9090/health")
	logger.Info("Stats available at http://localhost:9090/stats")

	<-sigCh
	logger.Info("Received shutdown signal")

	// 优雅关闭
	cancel()
	if err := manager.Stop(); err != nil {
		logger.Error("Failed to stop pool manager", zap.Error(err))
	}

	logger.Info("Application shutdown complete")
}

// demonstratePoolUsage 演示连接池的使用
func demonstratePoolUsage(ctx context.Context, manager *pool.Manager, logger *zap.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	serviceNames := []string{"user-service", "order-service", "payment-service"}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, serviceName := range serviceNames {
				// 使用连接池执行健康检查
				err := manager.ExecuteWithPool(ctx, serviceName, func(conn *grpc.ClientConn) error {
					return performHealthCheck(ctx, conn)
				})

				if err != nil {
					logger.Warn("Health check failed",
						zap.String("service", serviceName),
						zap.Error(err))
				} else {
					logger.Info("Health check successful",
						zap.String("service", serviceName))
				}
			}
		}
	}
}

// performHealthCheck 执行gRPC健康检查
func performHealthCheck(ctx context.Context, conn *grpc.ClientConn) error {
	client := grpc_health_v1.NewHealthClient(conn)
	
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("service is not serving, status: %v", resp.Status)
	}

	return nil
}

// monitorPoolStats 监控连接池统计信息
func monitorPoolStats(ctx context.Context, manager *pool.Manager, logger *zap.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := manager.GetStats()
			
			if len(stats) == 0 {
				logger.Info("No active connection pools")
				continue
			}

			logger.Info("=== Connection Pool Statistics ===")
			for serviceName, stat := range stats {
				logger.Info("Pool stats",
					zap.String("service", serviceName),
					zap.Int("total_connections", stat.TotalConnections),
					zap.Int("active_connections", stat.ActiveConnections),
					zap.Int("idle_connections", stat.IdleConnections))
			}

			healthyPools := manager.GetHealthyPools()
			logger.Info("Healthy pools", zap.Strings("services", healthyPools))
		}
	}
}

// 示例：自定义服务调用
func callUserService(ctx context.Context, manager *pool.Manager) error {
	return manager.ExecuteWithPool(ctx, "user-service", func(conn *grpc.ClientConn) error {
		// 这里可以创建具体的服务客户端并调用方法
		// 例如：
		// client := userpb.NewUserServiceClient(conn)
		// resp, err := client.GetUser(ctx, &userpb.GetUserRequest{Id: "123"})
		// return err
		
		// 示例中只执行健康检查
		return performHealthCheck(ctx, conn)
	})
}

// 示例：批量调用
func batchCalls(ctx context.Context, manager *pool.Manager) error {
	services := []string{"user-service", "order-service", "payment-service"}
	
	for _, service := range services {
		if err := manager.ExecuteWithPool(ctx, service, func(conn *grpc.ClientConn) error {
			// 执行具体的业务逻辑
			return performHealthCheck(ctx, conn)
		}); err != nil {
			return fmt.Errorf("call to %s failed: %w", service, err)
		}
	}
	
	return nil
} 