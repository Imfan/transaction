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

// PaymentServer 支付服务实现
type PaymentServer struct {
	proto.PaymentServiceServer
	port     int
	serverId string
	version  string
	payments map[string]*proto.GetPaymentResponse // 简单的内存存储
	consul   *api.Client
}

// NewPaymentServer 创建支付服务
func NewPaymentServer(port int, version string) *PaymentServer {
	// 创建Consul客户端
	config := api.DefaultConfig()
	config.Address = "localhost:8500"
	consul, err := api.NewClient(config)
	if err != nil {
		log.Printf("Failed to create consul client: %v", err)
	}

	return &PaymentServer{
		port:     port,
		serverId: fmt.Sprintf("payment-server-%d", port),
		version:  version,
		payments: make(map[string]*proto.GetPaymentResponse),
		consul:   consul,
	}
}

// registerToConsul 注册服务到Consul
func (s *PaymentServer) registerToConsul() error {
	if s.consul == nil {
		log.Println("Consul client not available, skipping registration")
		return nil
	}

	registration := &api.AgentServiceRegistration{
		ID:      s.serverId,
		Name:    "payment-service",
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
func (s *PaymentServer) deregisterFromConsul() error {
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

// ProcessPayment 处理支付
func (s *PaymentServer) ProcessPayment(ctx context.Context, req *proto.ProcessPaymentRequest) (*proto.ProcessPaymentResponse, error) {
	log.Printf("[%s] ProcessPayment called: order_id=%s, amount=%.2f, method=%s",
		s.serverId, req.OrderId, req.Amount, req.PaymentMethod)

	// 模拟支付处理时间
	time.Sleep(200 * time.Millisecond)

	// 生成支付ID和交易ID
	paymentId := fmt.Sprintf("pay_%d", time.Now().Unix())
	transactionId := fmt.Sprintf("txn_%d_%d", time.Now().Unix(), rand.Intn(10000))

	// 模拟支付成功/失败（90%成功率）
	success := rand.Float32() < 0.9
	status := "success"
	message := "Payment processed successfully"

	if !success {
		status = "failed"
		message = "Payment failed: insufficient funds"
	}

	// 如果支付成功，存储支付记录
	if success {
		payment := &proto.GetPaymentResponse{
			PaymentId: paymentId,
			OrderId:   req.OrderId,
			UserId:    req.UserId,
			Amount:    req.Amount,
			Status:    status,
			CreatedAt: time.Now().Format(time.RFC3339),
		}
		s.payments[paymentId] = payment
	}

	return &proto.ProcessPaymentResponse{
		PaymentId:     paymentId,
		Status:        status,
		Message:       message,
		TransactionId: transactionId,
	}, nil
}

// GetPayment 获取支付信息
func (s *PaymentServer) GetPayment(ctx context.Context, req *proto.GetPaymentRequest) (*proto.GetPaymentResponse, error) {
	log.Printf("[%s] GetPayment called: payment_id=%s", s.serverId, req.PaymentId)

	// 模拟一些处理时间
	time.Sleep(50 * time.Millisecond)

	// 检查支付是否存在
	if payment, exists := s.payments[req.PaymentId]; exists {
		return payment, nil
	}

	// 创建模拟支付数据
	payment := &proto.GetPaymentResponse{
		PaymentId: req.PaymentId,
		OrderId:   fmt.Sprintf("order_%d", rand.Intn(10000)),
		UserId:    fmt.Sprintf("user_%d", rand.Intn(1000)),
		Amount:    float64(rand.Intn(1000) + 50),
		Status:    []string{"success", "pending", "failed"}[rand.Intn(3)],
		CreatedAt: time.Now().Add(-time.Duration(rand.Intn(72)) * time.Hour).Format(time.RFC3339),
	}

	// 存储支付数据
	s.payments[req.PaymentId] = payment

	return payment, nil
}

// RefundPayment 退款
func (s *PaymentServer) RefundPayment(ctx context.Context, req *proto.RefundPaymentRequest) (*proto.RefundPaymentResponse, error) {
	log.Printf("[%s] RefundPayment called: payment_id=%s, amount=%.2f, reason=%s",
		s.serverId, req.PaymentId, req.Amount, req.Reason)

	// 模拟退款处理时间
	time.Sleep(150 * time.Millisecond)

	// 检查原支付是否存在
	payment, exists := s.payments[req.PaymentId]
	if !exists {
		return &proto.RefundPaymentResponse{
			RefundId: "",
			Status:   "failed",
			Message:  "Original payment not found",
		}, nil
	}

	// 检查退款金额是否有效
	if req.Amount > payment.Amount {
		return &proto.RefundPaymentResponse{
			RefundId: "",
			Status:   "failed",
			Message:  "Refund amount exceeds original payment amount",
		}, nil
	}

	// 生成退款ID
	refundId := fmt.Sprintf("refund_%d", time.Now().Unix())

	// 模拟退款成功/失败（95%成功率）
	success := rand.Float32() < 0.95
	status := "success"
	message := "Refund processed successfully"

	if !success {
		status = "failed"
		message = "Refund failed: processing error"
	}

	return &proto.RefundPaymentResponse{
		RefundId: refundId,
		Status:   status,
		Message:  message,
	}, nil
}

// startHealthServer 启动健康检查HTTP服务器
func (s *PaymentServer) startHealthServer() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := fmt.Sprintf(`{
			"status": "healthy",
			"service": "payment-service",
			"version": "%s",
			"server_id": "%s",
			"timestamp": "%s",
			"payment_count": %d
		}`, s.version, s.serverId, time.Now().Format(time.RFC3339), len(s.payments))
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

	// 创建支付服务实例
	paymentServer := NewPaymentServer(port, version)

	// 注册健康检查服务
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("payment-service", grpc_health_v1.HealthCheckResponse_SERVING)

	// 启用reflection（方便调试）
	reflection.Register(grpcServer)

	// 启动HTTP健康检查服务器
	go paymentServer.startHealthServer()

	// 注册到Consul
	if err := paymentServer.registerToConsul(); err != nil {
		log.Printf("Failed to register to Consul: %v", err)
	}

	// 设置优雅关闭
	defer func() {
		if err := paymentServer.deregisterFromConsul(); err != nil {
			log.Printf("Failed to deregister from Consul: %v", err)
		}
	}()

	log.Printf("[%s] PaymentService %s starting on port %d", paymentServer.serverId, version, port)

	// 启动gRPC服务器
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
