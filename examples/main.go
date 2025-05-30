package main

import (
	"context"
	"log"
	"time"

	"grpc-pool/pool"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// 创建连接池配置
	config := pool.Config{
		MinConns:    2,
		MaxConns:    10,
		IdleTimeout: 1 * time.Minute,
		Address:     "localhost:50051",
		Options: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
	}
	log.Println("创建连接池")
	// 创建连接池
	pool, err := pool.NewPool(config)
	if err != nil {
		log.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()
	log.Println("使用连接池")
	// 使用连接池
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		// 获取连接
		conn, err := pool.Get(ctx)
		if err != nil {
			log.Printf("Failed to get connection: %v", err)
			continue
		}

		// 使用连接
		log.Printf("Using connection %d", i)

		// 将连接放回池中
		pool.Put(conn)
	}
	log.Println("关闭连接池")
	// 等待一段时间以观察连接池的行为
	time.Sleep(2 * time.Second)
}
