package main

import (
	"context"
	"fmt"
	"log"
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

// UserServer 用户服务实现
type UserServer struct {
	proto.UserServiceServer
	port     int
	serverId string
	version  string
	users    map[string]*proto.GetUserResponse // 简单的内存存储
	consul   *api.Client
}

// NewUserServer 创建用户服务
func NewUserServer(port int, version string) *UserServer {
	// 创建Consul客户端
	config := api.DefaultConfig()
	config.Address = "localhost:8500"
	consul, err := api.NewClient(config)
	if err != nil {
		log.Printf("Failed to create consul client: %v", err)
	}

	return &UserServer{
		port:     port,
		serverId: fmt.Sprintf("user-server-%d", port),
		version:  version,
		users:    make(map[string]*proto.GetUserResponse),
		consul:   consul,
	}
}

// registerToConsul 注册服务到Consul
func (s *UserServer) registerToConsul() error {
	if s.consul == nil {
		log.Println("Consul client not available, skipping registration")
		return nil
	}

	registration := &api.AgentServiceRegistration{
		ID:      s.serverId,
		Name:    "user-service",
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
func (s *UserServer) deregisterFromConsul() error {
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

// GetUser 获取用户
func (s *UserServer) GetUser(ctx context.Context, req *proto.GetUserRequest) (*proto.GetUserResponse, error) {
	log.Printf("[%s] GetUser called: user_id=%s", s.serverId, req.UserId)

	// 模拟一些处理时间
	time.Sleep(50 * time.Millisecond)

	// 检查用户是否存在
	if user, exists := s.users[req.UserId]; exists {
		return user, nil
	}

	// 创建模拟用户数据
	user := &proto.GetUserResponse{
		UserId:    req.UserId,
		Name:      fmt.Sprintf("User_%s", req.UserId),
		Email:     fmt.Sprintf("user_%s@example.com", req.UserId),
		Age:       25 + int32(len(req.UserId)%50), // 模拟不同年龄
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	// 存储用户数据
	s.users[req.UserId] = user

	return user, nil
}

// CreateUser 创建用户
func (s *UserServer) CreateUser(ctx context.Context, req *proto.CreateUserRequest) (*proto.CreateUserResponse, error) {
	log.Printf("[%s] CreateUser called: name=%s, email=%s", s.serverId, req.Name, req.Email)

	// 模拟一些处理时间
	time.Sleep(100 * time.Millisecond)

	// 生成用户ID
	userId := fmt.Sprintf("user_%d", time.Now().Unix())

	// 创建用户
	user := &proto.GetUserResponse{
		UserId:    userId,
		Name:      req.Name,
		Email:     req.Email,
		Age:       req.Age,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	// 存储用户
	s.users[userId] = user

	return &proto.CreateUserResponse{
		UserId:  userId,
		Message: "User created successfully",
	}, nil
}

// startHealthServer 启动健康检查HTTP服务器
func (s *UserServer) startHealthServer() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := fmt.Sprintf(`{
			"status": "healthy",
			"service": "user-service",
			"version": "%s",
			"server_id": "%s",
			"timestamp": "%s",
			"user_count": %d
		}`, s.version, s.serverId, time.Now().Format(time.RFC3339), len(s.users))
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

	// 创建用户服务实例
	userServer := NewUserServer(port, version)

	// 注册服务（这里简化实现，通常使用生成的注册函数）
	// proto.RegisterUserServiceServer(grpcServer, userServer)

	// 注册健康检查服务
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("user-service", grpc_health_v1.HealthCheckResponse_SERVING)

	// 启用reflection（方便调试）
	reflection.Register(grpcServer)

	// 启动HTTP健康检查服务器
	go userServer.startHealthServer()

	// 注册到Consul
	if err := userServer.registerToConsul(); err != nil {
		log.Printf("Failed to register to Consul: %v", err)
	}

	// 设置优雅关闭
	defer func() {
		if err := userServer.deregisterFromConsul(); err != nil {
			log.Printf("Failed to deregister from Consul: %v", err)
		}
	}()

	log.Printf("[%s] UserService %s starting on port %d", userServer.serverId, version, port)

	// 启动gRPC服务器
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
