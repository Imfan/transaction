package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"transaction/pkg/config"
	"transaction/pkg/pool"

	"go.uber.org/zap"
)

const usage = `gRPC Connection Pool Tool

Usage:
  grpc-pool [options] <command>

Commands:
  start     Start the connection pool manager
  stats     Show connection pool statistics
  health    Check pool health status

Options:
  -consul-addr    Consul address (default: localhost:8500)
  -metrics-port   Metrics server port (default: 9090)
  -config         Configuration file path
  -service        Service name for stats/health commands
  -help           Show this help message

Examples:
  # Start the pool manager
  grpc-pool start

  # Start with custom Consul address
  grpc-pool -consul-addr=consul.example.com:8500 start

  # Check stats for a specific service
  grpc-pool -service=user-service stats

  # Check health of all services
  grpc-pool health
`

func main() {
	var (
		consulAddr  = flag.String("consul-addr", "localhost:8500", "Consul address")
		metricsPort = flag.Int("metrics-port", 9090, "Metrics server port")
		configFile  = flag.String("config", "", "Configuration file path")
		serviceName = flag.String("service", "", "Service name")
		showHelp    = flag.Bool("help", false, "Show help message")
	)

	flag.Usage = func() {
		fmt.Print(usage)
	}

	flag.Parse()

	if *showHelp {
		flag.Usage()
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("Error: No command specified")
		flag.Usage()
		os.Exit(1)
	}

	command := args[0]

	// 创建logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// 创建配置
	cfg := createConfig(*consulAddr, *metricsPort, *configFile)

	switch command {
	case "start":
		startManager(cfg, logger)
	case "stats":
		showStats(cfg, logger, *serviceName)
	case "health":
		checkHealth(cfg, logger, *serviceName)
	default:
		fmt.Printf("Error: Unknown command '%s'\n", command)
		flag.Usage()
		os.Exit(1)
	}
}

func createConfig(consulAddr string, metricsPort int, configFile string) *config.Config {
	var cfg *config.Config

	if configFile != "" {
		// 从文件加载配置
		cfg = loadConfigFromFile(configFile)
	} else {
		// 使用默认配置
		cfg = config.DefaultConfig()
	}

	// 覆盖命令行参数
	if consulAddr != "" {
		cfg.Discovery.ConsulAddr = consulAddr
	}
	if metricsPort != 0 {
		cfg.Metrics.Port = metricsPort
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	return cfg
}

func loadConfigFromFile(filepath string) *config.Config {
	file, err := os.Open(filepath)
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	defer file.Close()

	var cfg config.Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		log.Fatalf("Failed to decode config file: %v", err)
	}

	return &cfg
}

func startManager(cfg *config.Config, logger *zap.Logger) {
	logger.Info("Starting gRPC Connection Pool Manager")

	manager := pool.NewManager(cfg, logger)

	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		logger.Fatal("Failed to start pool manager", zap.Error(err))
	}

	logger.Info("Pool manager started successfully")
	logger.Info("Metrics server", zap.Int("port", cfg.Metrics.Port))

	// 保持运行
	select {}
}

func showStats(cfg *config.Config, logger *zap.Logger, serviceName string) {
	manager := pool.NewManager(cfg, logger)

	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		logger.Fatal("Failed to start pool manager", zap.Error(err))
	}
	defer manager.Stop()

	// 等待一下让服务发现初始化
	time.Sleep(2 * time.Second)

	stats := manager.GetStats()

	if serviceName != "" {
		// 显示特定服务的统计信息
		if stat, exists := stats[serviceName]; exists {
			printServiceStats(serviceName, stat)
		} else {
			fmt.Printf("Service '%s' not found\n", serviceName)
			os.Exit(1)
		}
	} else {
		// 显示所有服务的统计信息
		if len(stats) == 0 {
			fmt.Println("No active connection pools found")
			return
		}

		fmt.Println("=== Connection Pool Statistics ===")
		for service, stat := range stats {
			printServiceStats(service, stat)
		}
	}
}

func printServiceStats(serviceName string, stats pool.PoolStats) {
	fmt.Printf("Service: %s\n", serviceName)
	fmt.Printf("  Total Connections:  %d\n", stats.TotalConnections)
	fmt.Printf("  Active Connections: %d\n", stats.ActiveConnections)
	fmt.Printf("  Idle Connections:   %d\n", stats.IdleConnections)
	fmt.Println()
}

func checkHealth(cfg *config.Config, logger *zap.Logger, serviceName string) {
	manager := pool.NewManager(cfg, logger)

	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		logger.Fatal("Failed to start pool manager", zap.Error(err))
	}
	defer manager.Stop()

	// 等待一下让服务发现初始化
	time.Sleep(2 * time.Second)

	healthyPools := manager.GetHealthyPools()

	if serviceName != "" {
		// 检查特定服务的健康状态
		isHealthy := false
		for _, healthy := range healthyPools {
			if healthy == serviceName {
				isHealthy = true
				break
			}
		}

		if isHealthy {
			fmt.Printf("Service '%s' is healthy\n", serviceName)
		} else {
			fmt.Printf("Service '%s' is unhealthy or not found\n", serviceName)
			os.Exit(1)
		}
	} else {
		// 显示所有服务的健康状态
		if len(healthyPools) == 0 {
			fmt.Println("No healthy connection pools found")
			os.Exit(1)
		}

		fmt.Println("=== Healthy Services ===")
		for _, service := range healthyPools {
			fmt.Printf("✓ %s\n", service)
		}
	}
}
