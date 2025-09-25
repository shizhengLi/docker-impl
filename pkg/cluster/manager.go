package cluster

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type ClusterManager struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Config      *ClusterConfig    `json:"config"`
	NodeManager *NodeManager      `json:"-"`
	TaskManager *TaskManager      `json:"-"`
	Scheduler   *Scheduler        `json:"-"`
	APIServer   *APIServer        `json:"-"`
	Discovery   *DiscoveryService `json:"-"`
	mu          sync.RWMutex
	started     bool
	shutdown    chan struct{}
}

type ClusterConfig struct {
	AdvertiseAddr    string            `json:"advertise_addr"`
	AdvertisePort    int               `json:"advertise_port"`
	DataDir          string            `json:"data_dir"`
	JoinToken        string            `json:"join_token"`
	HeartbeatInterval time.Duration   `json:"heartbeat_interval"`
	ElectionTimeout  time.Duration   `json:"election_timeout"`
	TaskTimeout      time.Duration   `json:"task_timeout"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	Discovery        DiscoveryConfig   `json:"discovery"`
	Security         SecurityConfig    `json:"security"`
}

type DiscoveryConfig struct {
	Mode     string            `json:"mode"`
	Endpoints []string          `json:"endpoints"`
	Options  map[string]string `json:"options"`
}

type SecurityConfig struct {
	AutoTLS     bool   `json:"auto_tls"`
	TLSCertFile string `json:"tls_cert_file"`
	TLSKeyFile  string `json:"tls_key_file"`
	Token       string `json:"token"`
}

type ClusterStatus struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Status       string            `json:"status"`
	Nodes        int               `json:"nodes"`
	Managers     int               `json:"managers"`
	Workers      int               `json:"workers"`
	ActiveTasks  int               `json:"active_tasks"`
	CompletedTasks int             `json:"completed_tasks"`
	Uptime       string            `json:"uptime"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
}

var (
	clusterManager *ClusterManager
	managerOnce    sync.Once
)

func GetClusterManager() *ClusterManager {
	managerOnce.Do(func() {
		config := &ClusterConfig{
			AdvertiseAddr:       "0.0.0.0",
			AdvertisePort:       2377,
			DataDir:            "/var/lib/mydocker/cluster",
			HeartbeatInterval:   5 * time.Second,
			ElectionTimeout:    10 * time.Second,
			TaskTimeout:        30 * time.Second,
			HealthCheckInterval: 10 * time.Second,
			Discovery: DiscoveryConfig{
				Mode:     "static",
				Endpoints: []string{},
			},
			Security: SecurityConfig{
				AutoTLS: false,
				Token:   "",
			},
		}
		clusterManager = NewClusterManager(config)
	})
	return clusterManager
}

func NewClusterManager(config *ClusterConfig) *ClusterManager {
	cm := &ClusterManager{
		ID:       generateClusterID(),
		Name:     "mydocker-cluster",
		Version:  "1.0.0",
		Config:   config,
		shutdown: make(chan struct{}),
	}

	// Initialize components
	cm.NodeManager = NewNodeManager(cm)
	cm.TaskManager = NewTaskManager(cm)
	cm.Scheduler = NewScheduler(cm)
	cm.APIServer = NewAPIServer(cm)
	cm.Discovery = NewDiscoveryService(cm, config.Discovery)

	return cm
}

func (cm *ClusterManager) Initialize() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	logrus.Info("Initializing cluster manager")

	if cm.started {
		return fmt.Errorf("cluster manager is already initialized")
	}

	// Initialize discovery service
	if err := cm.Discovery.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize discovery service: %v", err)
	}

	// Start API server
	if err := cm.APIServer.Start(); err != nil {
		return fmt.Errorf("failed to start API server: %v", err)
	}

	// Start scheduler
	if err := cm.Scheduler.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %v", err)
	}

	// Register this node
	if err := cm.registerLocalNode(); err != nil {
		return fmt.Errorf("failed to register local node: %v", err)
	}

	cm.started = true
	logrus.Info("Cluster manager initialized successfully")

	return nil
}

func (cm *ClusterManager) Shutdown() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.started {
		return fmt.Errorf("cluster manager is not initialized")
	}

	logrus.Info("Shutting down cluster manager")

	close(cm.shutdown)

	// Shutdown components
	if cm.Scheduler != nil {
		cm.Scheduler.Stop()
	}

	if cm.APIServer != nil {
		cm.APIServer.Stop()
	}

	if cm.Discovery != nil {
		cm.Discovery.Stop()
	}

	if cm.NodeManager != nil {
		cm.NodeManager.Shutdown()
	}

	cm.started = false
	logrus.Info("Cluster manager shutdown successfully")

	return nil
}

func (cm *ClusterManager) JoinCluster(joinAddr, joinToken string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	logrus.Infof("Joining cluster at %s", joinAddr)

	if cm.started {
		return fmt.Errorf("cluster manager is already initialized")
	}

	// Validate join token
	if joinToken == "" {
		return fmt.Errorf("join token is required")
	}

	// Set join token in config
	cm.Config.JoinToken = joinToken

	// Initialize discovery with join address
	cm.Config.Discovery.Endpoints = []string{joinAddr}

	// Initialize cluster
	if err := cm.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize cluster: %v", err)
	}

	logrus.Infof("Successfully joined cluster at %s", joinAddr)
	return nil
}

func (cm *ClusterManager) LeaveCluster(force bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.started {
		return fmt.Errorf("cluster manager is not initialized")
	}

	logrus.Info("Leaving cluster")

	if !force {
		// Check if this is the last manager node
		managers := cm.NodeManager.GetManagerNodes()
		if len(managers) <= 1 {
			return fmt.Errorf("cannot leave cluster: this is the last manager node")
		}

		// Check if there are running tasks
		tasks, err := cm.TaskManager.ListTasks()
		if err != nil {
			return fmt.Errorf("failed to list tasks: %v", err)
		}

		runningTasks := 0
		for _, task := range tasks {
			if task.Status == TaskRunning {
				runningTasks++
			}
		}

		if runningTasks > 0 {
			return fmt.Errorf("cannot leave cluster: %d tasks are still running", runningTasks)
		}
	}

	// Shutdown cluster manager
	if err := cm.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown cluster manager: %v", err)
	}

	logrus.Info("Successfully left cluster")
	return nil
}

func (cm *ClusterManager) GetStatus() *ClusterStatus {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if !cm.started {
		return &ClusterStatus{
			ID:     cm.ID,
			Name:   cm.Name,
			Status: "stopped",
		}
	}

	nodes, _ := cm.NodeManager.ListNodes()
	managers := cm.NodeManager.GetManagerNodes()
	workers := cm.NodeManager.GetWorkerNodes()

	tasks, _ := cm.TaskManager.ListTasks()
	activeTasks := 0
	completedTasks := 0
	for _, task := range tasks {
		if task.Status == TaskRunning {
			activeTasks++
		} else if task.Status == TaskCompleted {
			completedTasks++
		}
	}

	return &ClusterStatus{
		ID:            cm.ID,
		Name:          cm.Name,
		Status:        "running",
		Nodes:         len(nodes),
		Managers:      len(managers),
		Workers:       len(workers),
		ActiveTasks:   activeTasks,
		CompletedTasks: completedTasks,
		CreatedAt:     "now", // Would be stored during initialization
		UpdatedAt:     time.Now().Format(time.RFC3339),
	}
}

func (cm *ClusterManager) GetClusterInfo() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	info := map[string]interface{}{
		"id":      cm.ID,
		"name":    cm.Name,
		"version": cm.Version,
		"config":  cm.Config,
	}

	if cm.started {
		info["status"] = "running"
		info["node_stats"] = cm.NodeManager.GetClusterStats()
		info["task_stats"] = cm.TaskManager.GetStats()
	} else {
		info["status"] = "stopped"
	}

	return info
}

func (cm *ClusterManager) ScaleWorkers(targetCount int) error {
	cm.mu.RLock()
	if !cm.started {
		cm.mu.RUnlock()
		return fmt.Errorf("cluster manager is not initialized")
	}
	cm.mu.RUnlock()

	logrus.Infof("Scaling workers to %d", targetCount)

	currentWorkers := cm.NodeManager.GetWorkerNodes()
	currentCount := len(currentWorkers)

	if targetCount > currentCount {
		// Scale up
		for i := currentCount; i < targetCount; i++ {
			if err := cm.addWorkerNode(); err != nil {
				logrus.Errorf("Failed to add worker node %d: %v", i, err)
				continue
			}
		}
	} else if targetCount < currentCount {
		// Scale down
		for i := currentCount; i > targetCount; i-- {
			if len(currentWorkers) <= 1 {
				logrus.Warn("Cannot scale below 1 worker")
				break
			}

			// Remove last worker
			worker := currentWorkers[len(currentWorkers)-1]
			if err := cm.removeWorkerNode(worker.ID); err != nil {
				logrus.Errorf("Failed to remove worker node %s: %v", worker.ID, err)
				continue
			}
			currentWorkers = currentWorkers[:len(currentWorkers)-1]
		}
	}

	logrus.Infof("Successfully scaled workers to %d", targetCount)
	return nil
}

func (cm *ClusterManager) addWorkerNode() error {
	// This would integrate with cloud provider or infrastructure
	// For demonstration, we'll simulate adding a worker
	node := &Node{
		ID:      generateNodeID(),
		Name:    fmt.Sprintf("worker-%d", time.Now().Unix()),
		Address: "127.0.0.1", // Would be actual IP
		Port:    2376,
		Role:    RoleWorker,
		Status:  StatusReady,
		Resources: Resources{
			CPU:    2000, // 2 cores
			Memory: 4 * 1024 * 1024 * 1024, // 4GB
			Disk:   50 * 1024 * 1024 * 1024, // 50GB
		},
	}

	return cm.NodeManager.RegisterNode(node)
}

func (cm *ClusterManager) removeWorkerNode(nodeID string) error {
	return cm.NodeManager.UnregisterNode(nodeID)
}

func (cm *ClusterManager) registerLocalNode() error {
	// Get local system resources
	resources := cm.getLocalResources()

	node := &Node{
		ID:      getLocalNodeID(),
		Name:    getLocalHostname(),
		Address: cm.Config.AdvertiseAddr,
		Port:    cm.Config.AdvertisePort,
		Role:    RoleManager,
		Status:  StatusActive,
		Resources: resources,
		Capabilities: map[string]bool{
			"manager": true,
			"worker":  true,
		},
		Version: cm.Version,
	}

	return cm.NodeManager.RegisterNode(node)
}

func (cm *ClusterManager) getLocalResources() Resources {
	// In real implementation, this would get actual system resources
	return Resources{
		CPU:    4000, // 4 cores
		Memory: 8 * 1024 * 1024 * 1024, // 8GB
		Disk:   100 * 1024 * 1024 * 1024, // 100GB
		GPU:    0,
		Network: Network{
			Interfaces: []string{"eth0", "lo"},
			Bandwidth:  1000000000, // 1Gbps
		},
	}
}

func (cm *ClusterManager) GetJoinToken() (string, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if !cm.started {
		return "", fmt.Errorf("cluster manager is not initialized")
	}

	if cm.Config.JoinToken == "" {
		cm.Config.JoinToken = generateJoinToken()
	}

	return cm.Config.JoinToken, nil
}

func (cm *ClusterManager) RotateJoinToken() (string, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.started {
		return "", fmt.Errorf("cluster manager is not initialized")
	}

	cm.Config.JoinToken = generateJoinToken()
	logrus.Info("Join token rotated")

	return cm.Config.JoinToken, nil
}

func (cm *ClusterManager) HandleNodeFailure(nodeID string) error {
	logrus.Warnf("Handling node failure: %s", nodeID)

	// Get failed node
	node, err := cm.NodeManager.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node %s: %v", nodeID, err)
	}

	// Update node status
	if err := cm.NodeManager.UpdateNodeStatus(nodeID, StatusDown); err != nil {
		logrus.Warnf("Failed to update node status: %v", err)
	}

	// Reschedule tasks from failed node
	tasks, err := cm.TaskManager.GetTasksByNode(nodeID)
	if err != nil {
		return fmt.Errorf("failed to get tasks for node %s: %v", nodeID, err)
	}

	for _, task := range tasks {
		if task.Status == TaskRunning {
			logrus.Infof("Rescheduling task %s from failed node %s", task.ID, nodeID)
			if err := cm.TaskManager.RestartTask(task.ID); err != nil {
				logrus.Errorf("Failed to restart task %s: %v", task.ID, err)
			}
		}
	}

	logrus.Infof("Successfully handled failure of node %s", nodeID)
	return nil
}

func generateClusterID() string {
	return fmt.Sprintf("cluster-%x", time.Now().UnixNano())[:12]
}

func generateNodeID() string {
	return fmt.Sprintf("node-%x", time.Now().UnixNano())[:12]
}

func generateJoinToken() string {
	return fmt.Sprintf("SWMTKN-1-%x", time.Now().UnixNano())
}

func getLocalNodeID() string {
	return generateNodeID()
}

func getLocalHostname() string {
	hostname, _ := os.Hostname()
	if hostname == "" {
		return "mydocker-host"
	}
	return hostname
}