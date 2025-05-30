package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"transaction/pkg/config"
	"transaction/pkg/pool"
	"transaction/proto"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	// 创建生产级logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting gRPC Connection Pool in production mode")

	// 加载配置
	cfg, err := loadConfig("production_config.json")
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
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
	defer manager.Stop()

	logger.Info("gRPC Connection Pool started successfully")
	logger.Info("Monitoring endpoints:",
		zap.String("metrics", "http://localhost:9090/metrics"),
		zap.String("health", "http://localhost:9090/health"),
		zap.String("stats", "http://localhost:9090/stats"))

	// 示例：在生产环境中使用连接池
	go func() {
		// 等待服务发现初始化
		time.Sleep(5 * time.Second)

		// 调用用户服务示例
		if err := callUserService(ctx, manager, logger); err != nil {
			logger.Error("Failed to call user service", zap.Error(err))
		}

		// 调用订单服务示例
		if err := callOrderService(ctx, manager, logger); err != nil {
			logger.Error("Failed to call order service", zap.Error(err))
		}

		// 调用支付服务示例
		if err := callPaymentService(ctx, manager, logger); err != nil {
			logger.Error("Failed to call payment service", zap.Error(err))
		}

		// 业务流程示例：完整的电商交易流程
		if err := businessWorkflow(ctx, manager, logger); err != nil {
			logger.Error("Failed to execute business workflow", zap.Error(err))
		}
	}()

	// 定期打印统计信息
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				printPoolStats(manager, logger)
			}
		}
	}()

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	logger.Info("Received shutdown signal, gracefully shutting down...")

	// 优雅关闭
	cancel()
	logger.Info("gRPC Connection Pool stopped")
}

// loadConfig 从文件加载配置
func loadConfig(filename string) (*config.Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg config.Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// callUserService 调用用户服务
func callUserService(ctx context.Context, manager *pool.Manager, logger *zap.Logger) error {
	return manager.ExecuteWithPool(ctx, "user-service", func(conn *grpc.ClientConn) error {
		// 首先进行健康检查
		healthClient := grpc_health_v1.NewHealthClient(conn)
		healthResp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{
			Service: "user-service",
		})
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}

		logger.Info("User service health check",
			zap.String("status", healthResp.Status.String()))

		// 调用用户服务的业务方法
		userClient := proto.NewUserServiceClient(conn)

		// 获取用户信息
		getUserResp, err := userClient.GetUser(ctx, &proto.GetUserRequest{
			UserId: "user_123",
		})
		if err != nil {
			return fmt.Errorf("get user failed: %w", err)
		}

		logger.Info("Get user successful",
			zap.String("user_id", getUserResp.UserId),
			zap.String("name", getUserResp.Name),
			zap.String("email", getUserResp.Email))

		// 创建新用户
		createUserResp, err := userClient.CreateUser(ctx, &proto.CreateUserRequest{
			Name:  "John Doe",
			Email: "john.doe@example.com",
			Age:   30,
		})
		if err != nil {
			return fmt.Errorf("create user failed: %w", err)
		}

		logger.Info("Create user successful",
			zap.String("user_id", createUserResp.UserId),
			zap.String("message", createUserResp.Message))

		return nil
	})
}

// callOrderService 调用订单服务
func callOrderService(ctx context.Context, manager *pool.Manager, logger *zap.Logger) error {
	return manager.ExecuteWithPool(ctx, "order-service", func(conn *grpc.ClientConn) error {
		// 健康检查
		healthClient := grpc_health_v1.NewHealthClient(conn)
		healthResp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{
			Service: "order-service",
		})
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}

		logger.Info("Order service health check",
			zap.String("status", healthResp.Status.String()))

		// 调用订单服务的业务方法
		orderClient := proto.NewOrderServiceClient(conn)

		// 获取订单信息
		getOrderResp, err := orderClient.GetOrder(ctx, &proto.GetOrderRequest{
			OrderId: "order_456",
		})
		if err != nil {
			return fmt.Errorf("get order failed: %w", err)
		}

		logger.Info("Get order successful",
			zap.String("order_id", getOrderResp.OrderId),
			zap.String("user_id", getOrderResp.UserId),
			zap.Float64("total_amount", getOrderResp.TotalAmount),
			zap.String("status", getOrderResp.Status))

		// 获取用户订单列表
		listOrdersResp, err := orderClient.ListOrders(ctx, &proto.ListOrdersRequest{
			UserId: "user_123",
			Limit:  5,
		})
		if err != nil {
			return fmt.Errorf("list orders failed: %w", err)
		}

		logger.Info("List orders successful",
			zap.Int("order_count", len(listOrdersResp.Orders)))

		return nil
	})
}

// callPaymentService 调用支付服务
func callPaymentService(ctx context.Context, manager *pool.Manager, logger *zap.Logger) error {
	return manager.ExecuteWithPool(ctx, "payment-service", func(conn *grpc.ClientConn) error {
		// 健康检查
		healthClient := grpc_health_v1.NewHealthClient(conn)
		healthResp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{
			Service: "payment-service",
		})
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}

		logger.Info("Payment service health check",
			zap.String("status", healthResp.Status.String()))

		// 调用支付服务的业务方法
		paymentClient := proto.NewPaymentServiceClient(conn)

		// 处理支付
		processPaymentResp, err := paymentClient.ProcessPayment(ctx, &proto.ProcessPaymentRequest{
			OrderId:       "order_456",
			UserId:        "user_123",
			Amount:        99.99,
			PaymentMethod: "credit_card",
		})
		if err != nil {
			return fmt.Errorf("process payment failed: %w", err)
		}

		logger.Info("Process payment response",
			zap.String("payment_id", processPaymentResp.PaymentId),
			zap.String("status", processPaymentResp.Status),
			zap.String("message", processPaymentResp.Message),
			zap.String("transaction_id", processPaymentResp.TransactionId))

		// 如果支付成功，获取支付信息
		if processPaymentResp.Status == "success" {
			getPaymentResp, err := paymentClient.GetPayment(ctx, &proto.GetPaymentRequest{
				PaymentId: processPaymentResp.PaymentId,
			})
			if err != nil {
				return fmt.Errorf("get payment failed: %w", err)
			}

			logger.Info("Get payment successful",
				zap.String("payment_id", getPaymentResp.PaymentId),
				zap.Float64("amount", getPaymentResp.Amount),
				zap.String("status", getPaymentResp.Status))
		}

		return nil
	})
}

// businessWorkflow 完整的业务流程示例
func businessWorkflow(ctx context.Context, manager *pool.Manager, logger *zap.Logger) error {
	logger.Info("=== Starting Business Workflow ===")

	// 1. 创建用户
	var userId string
	err := manager.ExecuteWithPool(ctx, "user-service", func(conn *grpc.ClientConn) error {
		client := proto.NewUserServiceClient(conn)
		resp, err := client.CreateUser(ctx, &proto.CreateUserRequest{
			Name:  "Alice Smith",
			Email: "alice.smith@example.com",
			Age:   28,
		})
		if err != nil {
			return err
		}
		userId = resp.UserId
		logger.Info("Step 1: User created", zap.String("user_id", userId))
		return nil
	})
	if err != nil {
		return fmt.Errorf("step 1 failed: %w", err)
	}

	// 2. 创建订单
	var orderId string
	err = manager.ExecuteWithPool(ctx, "order-service", func(conn *grpc.ClientConn) error {
		client := proto.NewOrderServiceClient(conn)
		resp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
			UserId: userId,
			Items: []proto.OrderItem{
				{
					ProductId:   "laptop_001",
					ProductName: "Gaming Laptop",
					Quantity:    1,
					Price:       1299.99,
				},
			},
		})
		if err != nil {
			return err
		}
		orderId = resp.OrderId
		logger.Info("Step 2: Order created", zap.String("order_id", orderId))
		return nil
	})
	if err != nil {
		return fmt.Errorf("step 2 failed: %w", err)
	}

	// 3. 处理支付
	err = manager.ExecuteWithPool(ctx, "payment-service", func(conn *grpc.ClientConn) error {
		client := proto.NewPaymentServiceClient(conn)
		resp, err := client.ProcessPayment(ctx, &proto.ProcessPaymentRequest{
			OrderId:       orderId,
			UserId:        userId,
			Amount:        1299.99,
			PaymentMethod: "credit_card",
		})
		if err != nil {
			return err
		}
		logger.Info("Step 3: Payment processed",
			zap.String("payment_id", resp.PaymentId),
			zap.String("status", resp.Status))
		return nil
	})
	if err != nil {
		return fmt.Errorf("step 3 failed: %w", err)
	}

	logger.Info("=== Business Workflow Completed Successfully ===")
	return nil
}

// printPoolStats 打印连接池统计信息
func printPoolStats(manager *pool.Manager, logger *zap.Logger) {
	stats := manager.GetStats()

	logger.Info("=== Connection Pool Statistics ===")
	for serviceName, stat := range stats {
		logger.Info("Service pool stats",
			zap.String("service", serviceName),
			zap.Int("total_connections", stat.TotalConnections),
			zap.Int("active_connections", stat.ActiveConnections),
			zap.Int("idle_connections", stat.IdleConnections))
	}

	healthyPools := manager.GetHealthyPools()
	logger.Info("Healthy services",
		zap.Strings("services", healthyPools),
		zap.Int("count", len(healthyPools)))
}
