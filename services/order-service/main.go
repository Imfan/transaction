package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"transaction/proto"

	"github.com/hashicorp/consul/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// OrderServer 订单服务实现
type OrderServer struct {
	proto.OrderServiceServer
	port     int
	serverId string
	version  string
	orders   map[string]*proto.GetOrderResponse // 简单的内存存储
	consul   *api.Client
}

// NewOrderServer 创建订单服务
func NewOrderServer(port int, version string) *OrderServer {
	// 创建Consul客户端
	config := api.DefaultConfig()
	config.Address = "localhost:8500"
	consul, err := api.NewClient(config)
	if err != nil {
		log.Printf("Failed to create consul client: %v", err)
	}

	return &OrderServer{
		port:     port,
		serverId: fmt.Sprintf("order-server-%d", port),
		version:  version,
		orders:   make(map[string]*proto.GetOrderResponse),
		consul:   consul,
	}
}

// registerToConsul 注册服务到Consul
func (s *OrderServer) registerToConsul() error {
	if s.consul == nil {
		log.Println("Consul client not available, skipping registration")
		return nil
	}

	registration := &api.AgentServiceRegistration{
		ID:      s.serverId,
		Name:    "order-service",
		Tags:    []string{"production", s.version, "grpc"},
		Port:    s.port,
		Address: "192.168.2.132",
		Check: &api.AgentServiceCheck{
			TCP:                            fmt.Sprintf("192.168.2.132:%d", s.port),
			Interval:                       "10s",
			Timeout:                        "5s",
			DeregisterCriticalServiceAfter: "30s",
		},
	}

	err := s.consul.Agent().ServiceRegister(registration)
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	log.Printf("[%s] Service registered to Consul successfully", s.serverId)
	return nil
}

// deregisterFromConsul 从Consul注销服务
func (s *OrderServer) deregisterFromConsul() error {
	if s.consul == nil {
		return nil
	}

	err := s.consul.Agent().ServiceDeregister(s.serverId)
	if err != nil {
		return fmt.Errorf("failed to deregister service: %w", err)
	}

	log.Printf("[%s] Service deregistered from Consul", s.serverId)
	return nil
}

// GetOrder 获取订单
func (s *OrderServer) GetOrder(ctx context.Context, req *proto.GetOrderRequest) (*proto.GetOrderResponse, error) {
	log.Printf("[%s] GetOrder called: order_id=%s", s.serverId, req.OrderId)

	// 模拟一些处理时间
	time.Sleep(80 * time.Millisecond)

	// 检查订单是否存在
	if order, exists := s.orders[req.OrderId]; exists {
		return order, nil
	}

	// 创建模拟订单数据
	items := []proto.OrderItem{
		{
			ProductId:   "prod_001",
			ProductName: "iPhone 15",
			Quantity:    1,
			Price:       999.99,
		},
		{
			ProductId:   "prod_002",
			ProductName: "AirPods Pro",
			Quantity:    1,
			Price:       249.99,
		},
	}

	order := &proto.GetOrderResponse{
		OrderId:     req.OrderId,
		UserId:      fmt.Sprintf("user_%d", rand.Intn(1000)),
		Items:       items,
		TotalAmount: 1249.98,
		Status:      "confirmed",
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	// 存储订单数据
	s.orders[req.OrderId] = order

	return order, nil
}

// CreateOrder 创建订单
func (s *OrderServer) CreateOrder(ctx context.Context, req *proto.CreateOrderRequest) (*proto.CreateOrderResponse, error) {
	log.Printf("[%s] CreateOrder called: user_id=%s, items_count=%d", s.serverId, req.UserId, len(req.Items))

	// 模拟一些处理时间
	time.Sleep(120 * time.Millisecond)

	// 生成订单ID
	orderId := fmt.Sprintf("order_%d", time.Now().Unix())

	// 计算总金额
	var totalAmount float64
	for _, item := range req.Items {
		totalAmount += item.Price * float64(item.Quantity)
	}

	// 创建订单
	order := &proto.GetOrderResponse{
		OrderId:     orderId,
		UserId:      req.UserId,
		Items:       req.Items,
		TotalAmount: totalAmount,
		Status:      "pending",
		CreatedAt:   time.Now().Format(time.RFC3339),
	}

	// 存储订单
	s.orders[orderId] = order

	return &proto.CreateOrderResponse{
		OrderId: orderId,
		Message: "Order created successfully",
	}, nil
}

// ListOrders 获取订单列表
func (s *OrderServer) ListOrders(ctx context.Context, req *proto.ListOrdersRequest) (*proto.ListOrdersResponse, error) {
	log.Printf("[%s] ListOrders called: user_id=%s, limit=%d", s.serverId, req.UserId, req.Limit)

	// 模拟一些处理时间
	time.Sleep(60 * time.Millisecond)

	var userOrders []proto.GetOrderResponse
	count := 0

	// 查找用户的订单
	for _, order := range s.orders {
		if order.UserId == req.UserId {
			userOrders = append(userOrders, *order)
			count++
			if req.Limit > 0 && int32(count) >= req.Limit {
				break
			}
		}
	}

	// 如果没有找到订单，创建一些模拟数据
	if len(userOrders) == 0 {
		for i := 0; i < 3; i++ {
			orderId := fmt.Sprintf("order_%s_%d", req.UserId, i+1)
			order := proto.GetOrderResponse{
				OrderId: orderId,
				UserId:  req.UserId,
				Items: []proto.OrderItem{
					{
						ProductId:   fmt.Sprintf("prod_%03d", i+1),
						ProductName: fmt.Sprintf("Product %d", i+1),
						Quantity:    int32(rand.Intn(5) + 1),
						Price:       float64(rand.Intn(100) + 10),
					},
				},
				TotalAmount: float64(rand.Intn(500) + 50),
				Status:      []string{"pending", "confirmed", "shipped", "delivered"}[rand.Intn(4)],
				CreatedAt:   time.Now().Add(-time.Duration(i*24) * time.Hour).Format(time.RFC3339),
			}
			userOrders = append(userOrders, order)
			s.orders[orderId] = &order
		}
	}

	return &proto.ListOrdersResponse{
		Orders: userOrders,
	}, nil
}

// startHealthServer 启动健康检查HTTP服务器
func (s *OrderServer) startHealthServer() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := fmt.Sprintf(`{
			"status": "healthy",
			"service": "order-service",
			"version": "%s",
			"server_id": "%s",
			"timestamp": "%s",
			"order_count": %d
		}`, s.version, s.serverId, time.Now().Format(time.RFC3339), len(s.orders))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	})

	healthPort := s.port + 1000 // HTTP端口 = gRPC端口 + 1000
	log.Printf("[%s] Health check server starting on port %d", s.serverId, healthPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", healthPort), nil))
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <port> [version]")
	}

	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("Invalid port: %v", err)
	}

	version := "v1.0"
	if len(os.Args) > 2 {
		version = os.Args[2]
	}

	// 创建gRPC监听器
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", port, err)
	}

	// 创建gRPC服务器
	grpcServer := grpc.NewServer()

	// 创建订单服务实例
	orderServer := NewOrderServer(port, version)

	// 注册健康检查服务
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("order-service", grpc_health_v1.HealthCheckResponse_SERVING)

	// 启用reflection（方便调试）
	reflection.Register(grpcServer)

	// 启动HTTP健康检查服务器
	go orderServer.startHealthServer()

	// 注册到Consul
	if err := orderServer.registerToConsul(); err != nil {
		log.Printf("Failed to register to Consul: %v", err)
	}

	// 设置优雅关闭
	defer func() {
		if err := orderServer.deregisterFromConsul(); err != nil {
			log.Printf("Failed to deregister from Consul: %v", err)
		}
	}()

	log.Printf("[%s] OrderService %s starting on port %d", orderServer.serverId, version, port)

	// 启动gRPC服务器
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
