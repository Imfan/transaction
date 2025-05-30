package pool

import (
	"context"
	"errors"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

var (
	ErrPoolClosed = errors.New("pool is closed")
	ErrConnClosed = errors.New("connection is closed")
)

// Conn 包装了 gRPC 连接
type Conn struct {
	*grpc.ClientConn
	pool     *Pool
	lastUsed time.Time
}

// Pool 表示 gRPC 连接池
type Pool struct {
	mu          sync.Mutex
	conns       []*Conn
	minConns    int
	maxConns    int
	idleTimeout time.Duration
	address     string
	opts        []grpc.DialOption
	closed      bool
}

// Config 是连接池的配置选项
type Config struct {
	MinConns    int
	MaxConns    int
	IdleTimeout time.Duration
	Address     string
	Options     []grpc.DialOption
}

// NewPool 创建一个新的 gRPC 连接池
func NewPool(config Config) (*Pool, error) {
	if config.MinConns < 0 {
		config.MinConns = 0
	}
	if config.MaxConns <= 0 {
		config.MaxConns = 10
	}
	if config.MinConns > config.MaxConns {
		config.MinConns = config.MaxConns
	}
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = 1 * time.Minute
	}

	pool := &Pool{
		minConns:    config.MinConns,
		maxConns:    config.MaxConns,
		idleTimeout: config.IdleTimeout,
		address:     config.Address,
		opts:        config.Options,
		conns:       make([]*Conn, 0, config.MaxConns),
	}

	// 初始化最小连接数
	for i := 0; i < config.MinConns; i++ {
		conn, err := pool.createConn()
		if err != nil {
			pool.Close()
			return nil, err
		}
		pool.conns = append(pool.conns, conn)
	}

	// 启动清理过期连接的 goroutine
	go pool.cleanupLoop()

	return pool, nil
}

// Get 从连接池获取一个连接
func (p *Pool) Get(ctx context.Context) (*Conn, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}

	// 尝试获取空闲连接
	for i, conn := range p.conns {
		if conn.GetState() == connectivity.Ready {
			// 移除连接并返回
			p.conns = append(p.conns[:i], p.conns[i+1:]...)
			conn.lastUsed = time.Now()
			p.mu.Unlock()
			return conn, nil
		}
	}

	// 如果没有可用连接且未达到最大连接数，创建新连接
	if len(p.conns) < p.maxConns {
		p.mu.Unlock()
		return p.createConn()
	}

	// 等待连接可用
	p.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
		return p.Get(ctx)
	}
}

// Put 将连接放回连接池
func (p *Pool) Put(conn *Conn) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		conn.Close()
		return
	}

	// 检查连接状态
	if conn.GetState() != connectivity.Ready {
		conn.Close()
		return
	}

	// 如果连接池已满，关闭连接
	if len(p.conns) >= p.maxConns {
		conn.Close()
		return
	}

	conn.lastUsed = time.Now()
	p.conns = append(p.conns, conn)
}

// Close 关闭连接池
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.closed = true
	for _, conn := range p.conns {
		conn.Close()
	}
	p.conns = nil
}

// createConn 创建新的 gRPC 连接
func (p *Pool) createConn() (*Conn, error) {
	conn, err := grpc.Dial(p.address, p.opts...)
	if err != nil {
		return nil, err
	}

	return &Conn{
		ClientConn: conn,
		pool:       p,
		lastUsed:   time.Now(),
	}, nil
}

// cleanupLoop 定期清理过期连接
func (p *Pool) cleanupLoop() {
	ticker := time.NewTicker(p.idleTimeout / 2)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}

		now := time.Now()
		validConns := make([]*Conn, 0, len(p.conns))
		for _, conn := range p.conns {
			// 检查连接是否过期
			if now.Sub(conn.lastUsed) > p.idleTimeout {
				conn.Close()
				continue
			}
			// 检查连接状态
			if conn.GetState() != connectivity.Ready {
				conn.Close()
				continue
			}
			validConns = append(validConns, conn)
		}

		// 确保最小连接数
		for len(validConns) < p.minConns {
			conn, err := p.createConn()
			if err != nil {
				break
			}
			validConns = append(validConns, conn)
		}

		p.conns = validConns
		p.mu.Unlock()
	}
}
