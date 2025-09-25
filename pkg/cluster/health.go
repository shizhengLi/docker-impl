package cluster

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type HealthChecker struct {
	nodeManager *NodeManager
	healthData  map[string]*NodeHealth
	mu          sync.RWMutex
	stopChan    chan struct{}
	interval    time.Duration
}

type HealthCheckConfig struct {
	Interval       time.Duration `json:"interval"`
	Timeout        time.Duration `json:"timeout"`
	MaxRetries     int           `json:"max_retries"`
	Checks         []string      `json:"checks"`
}

func NewHealthChecker(nodeManager *NodeManager) *HealthChecker {
	hc := &HealthChecker{
		nodeManager: nodeManager,
		healthData:  make(map[string]*NodeHealth),
		stopChan:    make(chan struct{}),
		interval:    10 * time.Second,
	}

	return hc
}

func (hc *HealthChecker) Start() {
	logrus.Info("Starting health checker")

	go hc.run()

	logrus.Info("Health checker started")
}

func (hc *HealthChecker) Stop() {
	logrus.Info("Stopping health checker")

	close(hc.stopChan)

	logrus.Info("Health checker stopped")
}

func (hc *HealthChecker) run() {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.checkAllNodes()
		case <-hc.stopChan:
			return
		}
	}
}

func (hc *HealthChecker) checkAllNodes() {
	nodes, err := hc.nodeManager.ListNodes()
	if err != nil {
		logrus.Errorf("Failed to list nodes for health check: %v", err)
		return
	}

	var wg sync.WaitGroup

	for _, node := range nodes {
		wg.Add(1)
		go func(node *Node) {
			defer wg.Done()
			hc.checkNodeHealth(node)
		}(node)
	}

	wg.Wait()
}

func (hc *HealthChecker) checkNodeHealth(node *Node) {
	start := time.Now()

	health := &NodeHealth{
		ID:        node.ID,
		CheckTime: start.Format(time.RFC3339),
		Checks:    []HealthCheck{},
	}

	// Perform health checks
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. API connectivity check
	apiCheck := hc.checkAPIConnectivity(ctx, node)
	health.Checks = append(health.Checks, apiCheck)

	// 2. Resource availability check
	resourceCheck := hc.checkResourceAvailability(node)
	health.Checks = append(health.Checks, resourceCheck)

	// 3. Disk space check
	diskCheck := hc.checkDiskSpace(node)
	health.Checks = append(health.Checks, diskCheck)

	// 4. Network connectivity check
	networkCheck := hc.checkNetworkConnectivity(node)
	health.Checks = append(health.Checks, networkCheck)

	// Calculate overall health status
	health.Status = hc.calculateOverallHealth(health.Checks)
	health.ResponseTime = time.Since(start).Milliseconds()

	// Store health data
	hc.mu.Lock()
	hc.healthData[node.ID] = health
	hc.mu.Unlock()

	// Update node status based on health
	if health.Status == StatusDown {
		logrus.Warnf("Node %s is unhealthy, updating status", node.ID)
		if err := hc.nodeManager.UpdateNodeStatus(node.ID, StatusDown); err != nil {
			logrus.Errorf("Failed to update node status: %v", err)
		}
	} else if node.Status == StatusDown && health.Status == StatusReady {
		logrus.Infof("Node %s recovered, updating status", node.ID)
		if err := hc.nodeManager.UpdateNodeStatus(node.ID, StatusReady); err != nil {
			logrus.Errorf("Failed to update node status: %v", err)
		}
	}

	logrus.Debugf("Health check completed for node %s: %s (%dms)",
		node.ID, health.Status, health.ResponseTime)
}

func (hc *HealthChecker) checkAPIConnectivity(ctx context.Context, node *Node) HealthCheck {
	start := time.Now()

	check := HealthCheck{
		Name: "api_connectivity",
	}

	// Simulate API connectivity check
	// In real implementation, this would make HTTP request to node's API
	url := fmt.Sprintf("http://%s:%d/health", node.Address, node.Port)

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		check.Status = "failed"
		check.Duration = time.Since(start).Milliseconds()
		check.Message = fmt.Sprintf("Failed to create request: %v", err)
		return check
	}

	resp, err := client.Do(req)
	if err != nil {
		check.Status = "failed"
		check.Duration = time.Since(start).Milliseconds()
		check.Message = fmt.Sprintf("Request failed: %v", err)
		return check
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		check.Status = "passed"
		check.Message = "API is responsive"
	} else {
		check.Status = "failed"
		check.Message = fmt.Sprintf("API returned status %d", resp.StatusCode)
	}

	check.Duration = time.Since(start).Milliseconds()
	return check
}

func (hc *HealthChecker) checkResourceAvailability(node *Node) HealthCheck {
	start := time.Now()

	check := HealthCheck{
		Name: "resource_availability",
	}

	// Check if node has sufficient resources
	// This is simplified - in real implementation would get actual usage
	cpuUsage := float64(50) // Simulated 50% CPU usage
	memoryUsage := float64(60) // Simulated 60% memory usage
	diskUsage := float64(30) // Simulated 30% disk usage

	if cpuUsage < 90 && memoryUsage < 90 && diskUsage < 90 {
		check.Status = "passed"
		check.Message = fmt.Sprintf("Resources available (CPU: %.1f%%, Memory: %.1f%%, Disk: %.1f%%)",
			cpuUsage, memoryUsage, diskUsage)
	} else {
		check.Status = "failed"
		check.Message = fmt.Sprintf("Resource constraints exceeded (CPU: %.1f%%, Memory: %.1f%%, Disk: %.1f%%)",
			cpuUsage, memoryUsage, diskUsage)
	}

	check.Duration = time.Since(start).Milliseconds()
	return check
}

func (hc *HealthChecker) checkDiskSpace(node *Node) HealthCheck {
	start := time.Now()

	check := HealthCheck{
		Name: "disk_space",
	}

	// Simulate disk space check
	// In real implementation, would check actual disk usage
	diskUsage := float64(30) // 30% disk usage

	if diskUsage < 85 {
		check.Status = "passed"
		check.Message = fmt.Sprintf("Disk space sufficient (%.1f%% used)", diskUsage)
	} else if diskUsage < 95 {
		check.Status = "warning"
		check.Message = fmt.Sprintf("Disk space low (%.1f%% used)", diskUsage)
	} else {
		check.Status = "failed"
		check.Message = fmt.Sprintf("Disk space critical (%.1f%% used)", diskUsage)
	}

	check.Duration = time.Since(start).Milliseconds()
	return check
}

func (hc *HealthChecker) checkNetworkConnectivity(node *Node) HealthCheck {
	start := time.Now()

	check := HealthCheck{
		Name: "network_connectivity",
	}

	// Simulate network connectivity check
	// In real implementation, would check actual network connectivity
	networkLatency := 10 * time.Millisecond // Simulated network latency

	if networkLatency < 100*time.Millisecond {
		check.Status = "passed"
		check.Message = fmt.Sprintf("Network latency: %v", networkLatency)
	} else if networkLatency < 500*time.Millisecond {
		check.Status = "warning"
		check.Message = fmt.Sprintf("Network latency high: %v", networkLatency)
	} else {
		check.Status = "failed"
		check.Message = fmt.Sprintf("Network connectivity issues: %v", networkLatency)
	}

	check.Duration = time.Since(start).Milliseconds()
	return check
}

func (hc *HealthChecker) calculateOverallHealth(checks []HealthCheck) NodeStatus {
	failedCount := 0
	warningCount := 0

	for _, check := range checks {
		switch check.Status {
		case "failed":
			failedCount++
		case "warning":
			warningCount++
		}
	}

	if failedCount > 0 {
		return StatusDown
	} else if warningCount > 0 {
		return StatusUnknown
	} else {
		return StatusReady
	}
}

func (hc *HealthChecker) GetNodeHealth(nodeID string) (*NodeHealth, error) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	health, exists := hc.healthData[nodeID]
	if !exists {
		return nil, fmt.Errorf("health data not found for node: %s", nodeID)
	}

	return health, nil
}

func (hc *HealthChecker) GetAllNodesHealth() map[string]*NodeHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result := make(map[string]*NodeHealth)
	for nodeID, health := range hc.healthData {
		result[nodeID] = health
	}

	return result
}

func (hc *HealthChecker) GetStats() map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	stats := map[string]interface{}{
		"total_nodes": len(hc.healthData),
		"check_interval": hc.interval.String(),
	}

	statusCounts := make(map[NodeStatus]int)
	for _, health := range hc.healthData {
		statusCounts[health.Status]++
	}

	statusMap := make(map[string]int)
	for status, count := range statusCounts {
		statusMap[string(status)] = count
	}

	stats["nodes_by_status"] = statusMap

	// Calculate average response time
	var totalResponseTime int64
	for _, health := range hc.healthData {
		totalResponseTime += health.ResponseTime
	}

	if len(hc.healthData) > 0 {
		avgResponseTime := totalResponseTime / int64(len(hc.healthData))
		stats["average_response_time_ms"] = avgResponseTime
	}

	return stats
}

func (hc *HealthChecker) ForceCheck(nodeID string) error {
	nodes, err := hc.nodeManager.ListNodes()
	if err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}

	for _, node := range nodes {
		if node.ID == nodeID {
			hc.checkNodeHealth(node)
			logrus.Infof("Forced health check completed for node %s", nodeID)
			return nil
		}
	}

	return fmt.Errorf("node not found: %s", nodeID)
}

type DiscoveryService struct {
	manager      *ClusterManager
	config       DiscoveryConfig
	peers        map[string]*Peer
	mu           sync.RWMutex
	broadcastCh  chan *DiscoveryMessage
	stopChan     chan struct{}
}

type Peer struct {
	ID        string    `json:"id"`
	Address   string    `json:"address"`
	LastSeen  time.Time `json:"last_seen"`
	Status    string    `json:"status"`
	Version   string    `json:"version"`
}

type DiscoveryMessage struct {
	Type      string      `json:"type"`
	From      string      `json:"from"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

func NewDiscoveryService(manager *ClusterManager, config DiscoveryConfig) *DiscoveryService {
	return &DiscoveryService{
		manager:     manager,
		config:      config,
		peers:       make(map[string]*Peer),
		broadcastCh: make(chan *DiscoveryMessage, 100),
		stopChan:    make(chan struct{}),
	}
}

func (ds *DiscoveryService) Initialize() error {
	logrus.Infof("Initializing discovery service with mode: %s", ds.config.Mode)

	switch ds.config.Mode {
	case "static":
		return ds.initializeStaticDiscovery()
	case "dns":
		return ds.initializeDNSDiscovery()
	case "token":
		return ds.initializeTokenDiscovery()
	default:
		return fmt.Errorf("unsupported discovery mode: %s", ds.config.Mode)
	}
}

func (ds *DiscoveryService) initializeStaticDiscovery() error {
	logrus.Info("Initializing static discovery")

	// For static discovery, we just use the configured endpoints
	for _, endpoint := range ds.config.Endpoints {
		peer := &Peer{
			ID:       generatePeerID(endpoint),
			Address:  endpoint,
			LastSeen: time.Now(),
			Status:   "active",
		}
		ds.peers[peer.ID] = peer
	}

	return nil
}

func (ds *DiscoveryService) initializeDNSDiscovery() error {
	logrus.Info("Initializing DNS discovery (not implemented)")
	// DNS discovery would resolve DNS names to get peer addresses
	return nil
}

func (ds *DiscoveryService) initializeTokenDiscovery() error {
	logrus.Info("Initializing token discovery (not implemented)")
	// Token discovery would use join tokens to discover peers
	return nil
}

func (ds *DiscoveryService) Start() error {
	logrus.Info("Starting discovery service")

	go ds.broadcastLoop()
	go ds.peerHealthCheck()

	return nil
}

func (ds *DiscoveryService) Stop() error {
	logrus.Info("Stopping discovery service")

	close(ds.stopChan)

	return nil
}

func (ds *DiscoveryService) broadcastLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-ds.broadcastCh:
			ds.broadcastMessage(msg)
		case <-ticker.C:
			ds.heartbeat()
		case <-ds.stopChan:
			return
		}
	}
}

func (ds *DiscoveryService) broadcastMessage(msg *DiscoveryMessage) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	logrus.Debugf("Broadcasting discovery message: %s", msg.Type)

	// In real implementation, this would send messages to all peers
	// For simulation, we just log the message
}

func (ds *DiscoveryService) heartbeat() {
	msg := &DiscoveryMessage{
		Type:      "heartbeat",
		From:      ds.manager.ID,
		Timestamp: time.Now(),
		Payload: map[string]interface{}{
			"status": "alive",
			"version": ds.manager.Version,
		},
	}

	ds.broadcastCh <- msg
}

func (ds *DiscoveryService) peerHealthCheck() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ds.checkPeerHealth()
		case <-ds.stopChan:
			return
		}
	}
}

func (ds *DiscoveryService) checkPeerHealth() {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	now := time.Now()
	for id, peer := range ds.peers {
		if now.Sub(peer.LastSeen) > 120*time.Second {
			peer.Status = "inactive"
			logrus.Warnf("Peer %s is inactive", id)
		}
	}
}

func (ds *DiscoveryService) AddPeer(address string) error {
	peer := &Peer{
		ID:       generatePeerID(address),
		Address:  address,
		LastSeen: time.Now(),
		Status:   "active",
	}

	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.peers[peer.ID] = peer
	logrus.Infof("Added peer: %s", address)

	return nil
}

func (ds *DiscoveryService) RemovePeer(peerID string) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if _, exists := ds.peers[peerID]; exists {
		delete(ds.peers, peerID)
		logrus.Infof("Removed peer: %s", peerID)
		return nil
	}

	return fmt.Errorf("peer not found: %s", peerID)
}

func (ds *DiscoveryService) ListPeers() []*Peer {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	peers := make([]*Peer, 0, len(ds.peers))
	for _, peer := range ds.peers {
		peers = append(peers, peer)
	}

	return peers
}

func generatePeerID(address string) string {
	return fmt.Sprintf("peer-%x", address)[:12]
}

// Simple scheduler placeholder
type Scheduler struct {
	manager *ClusterManager
	stopChan chan struct{}
}

func NewScheduler(manager *ClusterManager) *Scheduler {
	return &Scheduler{
		manager:  manager,
		stopChan: make(chan struct{}),
	}
}

func (s *Scheduler) Start() error {
	logrus.Info("Starting scheduler")

	// Start scheduling loop
	go s.scheduleLoop()

	return nil
}

func (s *Scheduler) Stop() error {
	logrus.Info("Stopping scheduler")
	close(s.stopChan)
	return nil
}

func (s *Scheduler) scheduleLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.scheduleTasks()
		case <-s.stopChan:
			return
		}
	}
}

func (s *Scheduler) scheduleTasks() {
	// Get pending tasks
	tasks, err := s.manager.TaskManager.GetTasksByStatus(TaskPending)
	if err != nil {
		logrus.Errorf("Failed to get pending tasks: %v", err)
		return
	}

	// Schedule each task
	for _, task := range tasks {
		// Find suitable node
		node, err := s.manager.NodeManager.SelectNodeForTask(task)
		if err != nil {
			logrus.Errorf("Failed to find node for task %s: %v", task.ID, err)
			continue
		}

		// Assign task to node
		task.NodeID = node.ID
		if err := s.manager.TaskManager.UpdateTask(task.ID, task); err != nil {
			logrus.Errorf("Failed to assign task %s to node %s: %v", task.ID, node.ID, err)
			continue
		}

		logrus.Infof("Scheduled task %s on node %s", task.ID, node.ID)
	}
}