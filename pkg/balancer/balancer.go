package balancer

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Algorithm 负载均衡算法类型
type Algorithm string

const (
	RoundRobin Algorithm = "round_robin"
	Weighted   Algorithm = "weighted"
)

// Node 表示一个服务节点
type Node struct {
	Address string `json:"address"`
	Weight  int    `json:"weight"`
	Healthy bool   `json:"healthy"`
}

// Balancer 负载均衡器接口
type Balancer interface {
	// Select 选择一个节点
	Select() (*Node, error)
	// Update 更新节点列表
	Update(nodes []*Node)
	// SetUnhealthy 设置节点为不健康状态
	SetUnhealthy(address string)
	// SetHealthy 设置节点为健康状态
	SetHealthy(address string)
	// GetNodes 获取所有节点
	GetNodes() []*Node
	// GetHealthyNodes 获取健康节点
	GetHealthyNodes() []*Node
	// Algorithm 获取算法类型
	Algorithm() Algorithm
}

// roundRobinBalancer 轮询负载均衡器
type roundRobinBalancer struct {
	nodes   []*Node
	counter int64
	mutex   sync.RWMutex
}

// NewRoundRobinBalancer 创建轮询负载均衡器
func NewRoundRobinBalancer() Balancer {
	return &roundRobinBalancer{
		nodes:   make([]*Node, 0),
		counter: 0,
	}
}

func (r *roundRobinBalancer) Select() (*Node, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	healthyNodes := r.getHealthyNodesUnsafe()
	if len(healthyNodes) == 0 {
		return nil, fmt.Errorf("no healthy nodes available")
	}

	// 原子操作确保线程安全
	index := atomic.AddInt64(&r.counter, 1) - 1
	return healthyNodes[index%int64(len(healthyNodes))], nil
}

func (r *roundRobinBalancer) Update(nodes []*Node) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.nodes = make([]*Node, len(nodes))
	copy(r.nodes, nodes)
}

func (r *roundRobinBalancer) SetUnhealthy(address string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, node := range r.nodes {
		if node.Address == address {
			node.Healthy = false
			break
		}
	}
}

func (r *roundRobinBalancer) SetHealthy(address string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, node := range r.nodes {
		if node.Address == address {
			node.Healthy = true
			break
		}
	}
}

func (r *roundRobinBalancer) GetNodes() []*Node {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	nodes := make([]*Node, len(r.nodes))
	copy(nodes, r.nodes)
	return nodes
}

func (r *roundRobinBalancer) GetHealthyNodes() []*Node {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.getHealthyNodesUnsafe()
}

func (r *roundRobinBalancer) getHealthyNodesUnsafe() []*Node {
	var healthyNodes []*Node
	for _, node := range r.nodes {
		if node.Healthy {
			healthyNodes = append(healthyNodes, node)
		}
	}
	return healthyNodes
}

func (r *roundRobinBalancer) Algorithm() Algorithm {
	return RoundRobin
}

// weightedBalancer 加权轮询负载均衡器
type weightedBalancer struct {
	nodes          []*Node
	currentWeights []int
	mutex          sync.RWMutex
}

// NewWeightedBalancer 创建加权轮询负载均衡器
func NewWeightedBalancer() Balancer {
	return &weightedBalancer{
		nodes:          make([]*Node, 0),
		currentWeights: make([]int, 0),
	}
}

func (w *weightedBalancer) Select() (*Node, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	healthyNodes := w.getHealthyNodesUnsafe()
	if len(healthyNodes) == 0 {
		return nil, fmt.Errorf("no healthy nodes available")
	}

	// 加权轮询算法
	return w.selectByWeight(healthyNodes), nil
}

func (w *weightedBalancer) selectByWeight(nodes []*Node) *Node {
	if len(nodes) == 1 {
		return nodes[0]
	}

	totalWeight := 0
	maxCurrentWeight := -1
	selectedNode := nodes[0]

	for i, node := range nodes {
		totalWeight += node.Weight
		if i >= len(w.currentWeights) {
			w.currentWeights = append(w.currentWeights, 0)
		}

		w.currentWeights[i] += node.Weight

		if w.currentWeights[i] > maxCurrentWeight {
			maxCurrentWeight = w.currentWeights[i]
			selectedNode = node
		}
	}

	// 找到选中节点的索引并减去总权重
	for i, node := range nodes {
		if node.Address == selectedNode.Address {
			w.currentWeights[i] -= totalWeight
			break
		}
	}

	return selectedNode
}

func (w *weightedBalancer) Update(nodes []*Node) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.nodes = make([]*Node, len(nodes))
	copy(w.nodes, nodes)

	// 重置权重
	w.currentWeights = make([]int, len(nodes))
}

func (w *weightedBalancer) SetUnhealthy(address string) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	for _, node := range w.nodes {
		if node.Address == address {
			node.Healthy = false
			break
		}
	}
}

func (w *weightedBalancer) SetHealthy(address string) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	for _, node := range w.nodes {
		if node.Address == address {
			node.Healthy = true
			break
		}
	}
}

func (w *weightedBalancer) GetNodes() []*Node {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	nodes := make([]*Node, len(w.nodes))
	copy(nodes, w.nodes)
	return nodes
}

func (w *weightedBalancer) GetHealthyNodes() []*Node {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return w.getHealthyNodesUnsafe()
}

func (w *weightedBalancer) getHealthyNodesUnsafe() []*Node {
	var healthyNodes []*Node
	for _, node := range w.nodes {
		if node.Healthy {
			healthyNodes = append(healthyNodes, node)
		}
	}
	return healthyNodes
}

func (w *weightedBalancer) Algorithm() Algorithm {
	return Weighted
}

// NewBalancer 根据算法类型创建负载均衡器
func NewBalancer(algorithm Algorithm) Balancer {
	switch algorithm {
	case RoundRobin:
		return NewRoundRobinBalancer()
	case Weighted:
		return NewWeightedBalancer()
	default:
		return NewRoundRobinBalancer()
	}
}
