package cluster

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type Node struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Address      string            `json:"address"`
	Port         int               `json:"port"`
	Role         NodeRole          `json:"role"`
	Status       NodeStatus        `json:"status"`
	Capabilities map[string]bool  `json:"capabilities"`
	Labels       map[string]string `json:"labels"`
	Resources    Resources         `json:"resources"`
	LastSeen     string            `json:"last_seen"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
	Version      string            `json:"version"`
	Manager      *ClusterManager   `json:"-"`
}

type NodeRole string

const (
	RoleManager  NodeRole = "manager"
	RoleWorker   NodeRole = "worker"
	RoleAgent    NodeRole = "agent"
)

type NodeStatus string

const (
	StatusReady    NodeStatus = "ready"
	StatusActive   NodeStatus = "active"
	StatusDraining NodeStatus = "draining"
	StatusDown     NodeStatus = "down"
	StatusUnknown  NodeStatus = "unknown"
)

type Resources struct {
	CPU        int64   `json:"cpu"`         // CPU cores in millicores
	Memory     int64   `json:"memory"`      // Memory in bytes
	Disk       int64   `json:"disk"`        // Disk space in bytes
	GPU        int     `json:"gpu"`         // Number of GPUs
	Network    Network `json:"network"`     // Network resources
}

type Network struct {
	Interfaces []string `json:"interfaces"` // Network interfaces
	Bandwidth  int64     `json:"bandwidth"`  // Network bandwidth in bps
}

type NodeHealth struct {
	ID          string    `json:"id"`
	Status      NodeStatus `json:"status"`
	CheckTime   string    `json:"check_time"`
	ResponseTime int64     `json:"response_time_ms"`
	Error       string    `json:"error,omitempty"`
	Checks      []HealthCheck `json:"checks"`
}

type HealthCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Duration int64  `json:"duration_ms"`
	Message  string `json:"message,omitempty"`
}

type NodeManager struct {
	nodes       map[string]*Node
	mu          sync.RWMutex
	manager     *ClusterManager
	healthCheck *HealthChecker
}

func NewNodeManager(manager *ClusterManager) *NodeManager {
	nm := &NodeManager{
		nodes:   make(map[string]*Node),
		manager: manager,
	}

	nm.healthCheck = NewHealthChecker(nm)
	nm.healthCheck.Start()

	return nm
}

func (nm *NodeManager) RegisterNode(node *Node) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	logrus.Infof("Registering node: %s (%s)", node.ID, node.Address)

	// Check if node already exists
	if existingNode, exists := nm.nodes[node.ID]; exists {
		// Update existing node
		node.CreatedAt = existingNode.CreatedAt
		node.UpdatedAt = time.Now().Format(time.RFC3339)
	} else {
		// New node
		node.CreatedAt = time.Now().Format(time.RFC3339)
		node.UpdatedAt = time.Now().Format(time.RFC3339)
	}

	// Set node manager reference
	node.Manager = nm.manager

	// Validate node
	if err := nm.validateNode(node); err != nil {
		return fmt.Errorf("node validation failed: %v", err)
	}

	// Add to nodes map
	nm.nodes[node.ID] = node

	logrus.Infof("Node registered successfully: %s", node.ID)
	return nil
}

func (nm *NodeManager) UnregisterNode(nodeID string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	logrus.Infof("Unregistering node: %s", nodeID)

	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	// Check if node is manager
	if node.Role == RoleManager {
		managers := nm.GetManagerNodes()
		if len(managers) <= 1 {
			return fmt.Errorf("cannot remove last manager node")
		}
	}

	// Remove from nodes map
	delete(nm.nodes, nodeID)

	logrus.Infof("Node unregistered successfully: %s", nodeID)
	return nil
}

func (nm *NodeManager) GetNode(nodeID string) (*Node, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	return node, nil
}

func (nm *NodeManager) ListNodes() ([]*Node, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	nodes := make([]*Node, 0, len(nm.nodes))
	for _, node := range nm.nodes {
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (nm *NodeManager) UpdateNodeStatus(nodeID string, status NodeStatus) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	node.Status = status
	node.UpdatedAt = time.Now().Format(time.RFC3339)
	node.LastSeen = time.Now().Format(time.RFC3339)

	logrus.Infof("Updated node %s status to %s", nodeID, status)
	return nil
}

func (nm *NodeManager) GetManagerNodes() []*Node {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	var managers []*Node
	for _, node := range nm.nodes {
		if node.Role == RoleManager {
			managers = append(managers, node)
		}
	}

	return managers
}

func (nm *NodeManager) GetWorkerNodes() []*Node {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	var workers []*Node
	for _, node := range nm.nodes {
		if node.Role == RoleWorker {
			workers = append(workers, node)
		}
	}

	return workers
}

func (nm *NodeManager) GetReadyNodes() []*Node {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	var readyNodes []*Node
	for _, node := range nm.nodes {
		if node.Status == StatusReady || node.Status == StatusActive {
			readyNodes = append(readyNodes, node)
		}
	}

	return readyNodes
}

func (nm *NodeManager) GetNodesByRole(role NodeRole) []*Node {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	var roleNodes []*Node
	for _, node := range nm.nodes {
		if node.Role == role {
			roleNodes = append(roleNodes, node)
		}
	}

	return roleNodes
}

func (nm *NodeManager) SelectNodeForTask(task *Task) (*Node, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	// Filter ready nodes
	var candidateNodes []*Node
	for _, node := range nm.nodes {
		if node.Status == StatusReady || node.Status == StatusActive {
			if nm.nodeHasCapacity(node, task) {
				candidateNodes = append(candidateNodes, node)
			}
		}
	}

	if len(candidateNodes) == 0 {
		return nil, fmt.Errorf("no available nodes with sufficient capacity")
	}

	// Simple scheduling: select node with most available resources
	selectedNode := nm.selectNodeByResources(candidateNodes, task)

	logrus.Infof("Selected node %s for task %s", selectedNode.ID, task.ID)
	return selectedNode, nil
}

func (nm *NodeManager) nodeHasCapacity(node *Node, task *Task) bool {
	// Check if node has sufficient resources for the task
	return node.Resources.CPU >= task.Resources.CPU &&
		node.Resources.Memory >= task.Resources.Memory &&
		node.Resources.Disk >= task.Resources.Disk
}

func (nm *NodeManager) selectNodeByResources(nodes []*Node, task *Task) *Node {
	// Simple selection based on available CPU and memory
	var bestNode *Node
	bestScore := -1.0

	for _, node := range nodes {
		// Calculate score based on available resources
		cpuScore := float64(node.Resources.CPU-task.Resources.CPU) / float64(node.Resources.CPU)
		memoryScore := float64(node.Resources.Memory-task.Resources.Memory) / float64(node.Resources.Memory)
		totalScore := (cpuScore + memoryScore) / 2.0

		if totalScore > bestScore {
			bestScore = totalScore
			bestNode = node
		}
	}

	return bestNode
}

func (nm *NodeManager) GetNodeHealth(nodeID string) (*NodeHealth, error) {
	return nm.healthCheck.GetNodeHealth(nodeID)
}

func (nm *NodeManager) GetAllNodesHealth() map[string]*NodeHealth {
	return nm.healthCheck.GetAllNodesHealth()
}

func (nm *NodeManager) validateNode(node *Node) error {
	// Validate required fields
	if node.ID == "" {
		return fmt.Errorf("node ID is required")
	}

	if node.Name == "" {
		return fmt.Errorf("node name is required")
	}

	if node.Address == "" {
		return fmt.Errorf("node address is required")
	}

	if node.Port <= 0 || node.Port > 65535 {
		return fmt.Errorf("invalid node port: %d", node.Port)
	}

	// Validate role
	switch node.Role {
	case RoleManager, RoleWorker, RoleAgent:
		// Valid role
	default:
		return fmt.Errorf("invalid node role: %s", node.Role)
	}

	// Validate resources
	if node.Resources.CPU <= 0 {
		return fmt.Errorf("node CPU must be positive")
	}

	if node.Resources.Memory <= 0 {
		return fmt.Errorf("node memory must be positive")
	}

	if node.Resources.Disk <= 0 {
		return fmt.Errorf("node disk must be positive")
	}

	return nil
}

func (nm *NodeManager) GetClusterStats() map[string]interface{} {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	stats := map[string]interface{}{
		"total_nodes":      len(nm.nodes),
		"manager_nodes":    len(nm.GetManagerNodes()),
		"worker_nodes":     len(nm.GetWorkerNodes()),
		"ready_nodes":      len(nm.GetReadyNodes()),
	}

	// Calculate total resources
	var totalCPU, totalMemory, totalDisk int64
	for _, node := range nm.nodes {
		totalCPU += node.Resources.CPU
		totalMemory += node.Resources.Memory
		totalDisk += node.Resources.Disk
	}

	stats["total_resources"] = map[string]interface{}{
		"cpu":    totalCPU,
		"memory": totalMemory,
		"disk":   totalDisk,
	}

	// Health stats
	healthStats := nm.healthCheck.GetStats()
	stats["health"] = healthStats

	return stats
}

func (nm *NodeManager) DrainNode(nodeID string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	if node.Role == RoleManager {
		return fmt.Errorf("cannot drain manager node")
	}

	// Set node to draining status
	node.Status = StatusDraining
	node.UpdatedAt = time.Now().Format(time.RFC3339)

	logrus.Infof("Node %s set to draining mode", nodeID)
	return nil
}

func (nm *NodeManager) ActivateNode(nodeID string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	// Set node to active status
	node.Status = StatusActive
	node.UpdatedAt = time.Now().Format(time.RFC3339)
	node.LastSeen = time.Now().Format(time.RFC3339)

	logrus.Infof("Node %s activated", nodeID)
	return nil
}

func (nm *NodeManager) UpdateNodeResources(nodeID string, resources Resources) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	node, exists := nm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	node.Resources = resources
	node.UpdatedAt = time.Now().Format(time.RFC3339)

	logrus.Infof("Updated resources for node %s", nodeID)
	return nil
}

func (nm *NodeManager) Shutdown() {
	if nm.healthCheck != nil {
		nm.healthCheck.Stop()
	}
	logrus.Info("Node manager shutdown")
}