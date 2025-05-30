package pool

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// ConnectionState 连接状态
type ConnectionState int

const (
	StateIdle ConnectionState = iota
	StateActive
	StateClosed
)

// Connection gRPC连接封装
type Connection struct {
	conn        *grpc.ClientConn
	target      string
	state       ConnectionState
	createdAt   time.Time
	lastUsedAt  time.Time
	activeCount int32
	mutex       sync.RWMutex
}

// NewConnection 创建新连接
func NewConnection(conn *grpc.ClientConn, target string) *Connection {
	now := time.Now()
	return &Connection{
		conn:       conn,
		target:     target,
		state:      StateIdle,
		createdAt:  now,
		lastUsedAt: now,
	}
}

// GetConn 获取gRPC连接
func (c *Connection) GetConn() *grpc.ClientConn {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.conn
}

// GetTarget 获取目标地址
func (c *Connection) GetTarget() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.target
}

// GetState 获取连接状态
func (c *Connection) GetState() ConnectionState {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.state
}

// SetState 设置连接状态
func (c *Connection) SetState(state ConnectionState) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.state = state
}

// IsIdle 检查连接是否空闲
func (c *Connection) IsIdle() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.state == StateIdle && c.activeCount == 0
}

// IsHealthy 检查连接是否健康
func (c *Connection) IsHealthy() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.state == StateClosed {
		return false
	}

	state := c.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
}

// MarkAsUsed 标记连接为已使用
func (c *Connection) MarkAsUsed() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.lastUsedAt = time.Now()
	c.activeCount++
	if c.state == StateIdle {
		c.state = StateActive
	}
}

// MarkAsIdle 标记连接为空闲
func (c *Connection) MarkAsIdle() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.activeCount > 0 {
		c.activeCount--
	}
	if c.activeCount == 0 {
		c.state = StateIdle
	}
}

// GetAge 获取连接年龄
func (c *Connection) GetAge() time.Duration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return time.Since(c.createdAt)
}

// GetIdleTime 获取空闲时间
func (c *Connection) GetIdleTime() time.Duration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if c.state != StateIdle {
		return 0
	}
	return time.Since(c.lastUsedAt)
}

// GetActiveCount 获取活跃计数
func (c *Connection) GetActiveCount() int32 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.activeCount
}

// Close 关闭连接
func (c *Connection) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.state == StateClosed {
		return nil
	}

	c.state = StateClosed
	return c.conn.Close()
}

// WaitForStateChange 等待连接状态变化
func (c *Connection) WaitForStateChange(ctx context.Context, timeout time.Duration) bool {
	c.mutex.RLock()
	conn := c.conn
	c.mutex.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	currentState := conn.GetState()
	return conn.WaitForStateChange(ctx, currentState)
}
