package cluster

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Task struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Type         TaskType          `json:"type"`
	Image        string            `json:"image"`
	Command      []string          `json:"command"`
	Env          []string          `json:"env"`
	Resources    Resources         `json:"resources"`
	Constraints  []Constraint      `json:"constraints"`
	Placement    Placement         `json:"placement"`
	RestartPolicy RestartPolicy    `json:"restart_policy"`
	Networks     []NetworkConfig   `json:"networks"`
	Volumes      []VolumeConfig    `json:"volumes"`
	Secrets      []SecretConfig    `json:"secrets"`
	Configs      []ConfigConfig    `json:"configs"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	Status       TaskStatus        `json:"status"`
	NodeID       string            `json:"node_id"`
	DesiredState TaskStatus        `json:"desired_state"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
	StartedAt    string            `json:"started_at"`
	CompletedAt  string            `json:"completed_at"`
	ServiceID    string            `json:"service_id"`
	Slot         int               `json:"slot"`
}

type TaskType string

const (
	TaskTypeContainer TaskType = "container"
	TaskTypeService   TaskType = "service"
	TaskTypeJob       TaskType = "job"
)

type TaskStatus string

const (
	TaskNew        TaskStatus = "new"
	TaskPending    TaskStatus = "pending"
	TaskAssigned   TaskStatus = "assigned"
	TaskAccepted   TaskStatus = "accepted"
	TaskPreparing  TaskStatus = "preparing"
	TaskReady      TaskStatus = "ready"
	TaskStarting   TaskStatus = "starting"
	TaskRunning    TaskStatus = "running"
	TaskComplete   TaskStatus = "complete"
	TaskFailed     TaskStatus = "failed"
	TaskShutdown   TaskStatus = "shutdown"
	TaskRejected   TaskStatus = "rejected"
	TaskOrphaned   TaskStatus = "orphaned"
	TaskRemove     TaskStatus = "remove"
)

type Constraint struct {
	Operator string `json:"operator"`
	Key      string `json:"key"`
	Value    string `json:"value"`
}

type Placement struct {
	Constraints []string `json:"constraints"`
	Preferences []Preference `json:"preferences"`
	MaxReplicas int       `json:"max_replicas"`
}

type Preference struct {
	Spread string `json:"spread"`
}

type RestartPolicy struct {
	Condition   string `json:"condition"`
	Delay       string `json:"delay"`
	MaxAttempts int    `json:"max_attempts"`
	Window      string `json:"window"`
}

type NetworkConfig struct {
	Target string `json:"target"`
	Alias  string `json:"alias"`
}

type VolumeConfig struct {
	Source  string `json:"source"`
	Target  string `json:"target"`
	Type    string `json:"type"`
	ReadOnly bool   `json:"read_only"`
}

type SecretConfig struct {
	SecretID string `json:"secret_id"`
	Target   string `json:"target"`
	UID      string `json:"uid"`
	GID      string `json:"gid"`
	Mode     string `json:"mode"`
}

type ConfigConfig struct {
	ConfigID string `json:"config_id"`
	Target   string `json:"target"`
	UID      string `json:"uid"`
	GID      string `json:"gid"`
	Mode     string `json:"mode"`
}

type TaskManager struct {
	tasks    map[string]*Task
	mu       sync.RWMutex
	manager  *ClusterManager
	queue    chan *Task
	workers  int
	stopChan chan struct{}
}

func NewTaskManager(manager *ClusterManager) *TaskManager {
	tm := &TaskManager{
		tasks:    make(map[string]*Task),
		manager:  manager,
		queue:    make(chan *Task, 1000),
		workers:  5,
		stopChan: make(chan struct{}),
	}

	go tm.startWorkers()

	return tm
}

func (tm *TaskManager) CreateTask(task *Task) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	logrus.Infof("Creating task: %s", task.ID)

	// Validate task
	if err := tm.validateTask(task); err != nil {
		return fmt.Errorf("task validation failed: %v", err)
	}

	// Set initial state
	task.Status = TaskNew
	task.DesiredState = TaskRunning
	task.CreatedAt = time.Now().Format(time.RFC3339)
	task.UpdatedAt = time.Now().Format(time.RFC3339)

	// Store task
	tm.tasks[task.ID] = task

	// Queue task for processing
	select {
	case tm.queue <- task:
		logrus.Infof("Task queued: %s", task.ID)
	default:
		logrus.Warnf("Task queue full, task %s will be processed later", task.ID)
		go func() {
			tm.queue <- task
		}()
	}

	return nil
}

func (tm *TaskManager) GetTask(taskID string) (*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

func (tm *TaskManager) ListTasks() ([]*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tasks := make([]*Task, 0, len(tm.tasks))
	for _, task := range tm.tasks {
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (tm *TaskManager) UpdateTask(taskID string, updates *Task) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Update fields
	if updates.Name != "" {
		task.Name = updates.Name
	}
	if updates.DesiredState != "" {
		task.DesiredState = updates.DesiredState
	}
	if updates.Labels != nil {
		task.Labels = updates.Labels
	}

	task.UpdatedAt = time.Now().Format(time.RFC3339)

	logrus.Infof("Updated task: %s", taskID)
	return nil
}

func (tm *TaskManager) RemoveTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Check if task is running
	if task.Status == TaskRunning {
		return fmt.Errorf("cannot remove running task: %s", taskID)
	}

	delete(tm.tasks, taskID)
	logrus.Infof("Removed task: %s", taskID)

	return nil
}

func (tm *TaskManager) StartTask(taskID string) error {
	return tm.UpdateTask(taskID, &Task{DesiredState: TaskRunning})
}

func (tm *TaskManager) StopTask(taskID string) error {
	return tm.UpdateTask(taskID, &Task{DesiredState: TaskComplete})
}

func (tm *TaskManager) RestartTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Stop task
	task.DesiredState = TaskComplete
	task.UpdatedAt = time.Now().Format(time.RFC3339)

	// Create new task with same configuration
	newTask := *task
	newTask.ID = generateTaskID()
	newTask.Status = TaskNew
	newTask.DesiredState = TaskRunning
	newTask.CreatedAt = time.Now().Format(time.RFC3339)
	newTask.UpdatedAt = time.Now().Format(time.RFC3339)
	newTask.StartedAt = ""
	newTask.CompletedAt = ""

	// Store new task
	tm.tasks[newTask.ID] = &newTask

	// Queue new task
	tm.queue <- &newTask

	logrus.Infof("Restarted task %s as %s", taskID, newTask.ID)
	return nil
}

func (tm *TaskManager) GetTasksByNode(nodeID string) ([]*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var nodeTasks []*Task
	for _, task := range tm.tasks {
		if task.NodeID == nodeID {
			nodeTasks = append(nodeTasks, task)
		}
	}

	return nodeTasks, nil
}

func (tm *TaskManager) GetTasksByStatus(status TaskStatus) ([]*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var statusTasks []*Task
	for _, task := range tm.tasks {
		if task.Status == status {
			statusTasks = append(statusTasks, task)
		}
	}

	return statusTasks, nil
}

func (tm *TaskManager) GetStats() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := map[string]interface{}{
		"total_tasks": len(tm.tasks),
		"queue_length": len(tm.queue),
	}

	// Count tasks by status
	statusCounts := make(map[TaskStatus]int)
	for _, task := range tm.tasks {
		statusCounts[task.Status]++
	}

	statusMap := make(map[string]int)
	for status, count := range statusCounts {
		statusMap[string(status)] = count
	}

	stats["tasks_by_status"] = statusMap
	stats["workers"] = tm.workers

	return stats
}

func (tm *TaskManager) startWorkers() {
	for i := 0; i < tm.workers; i++ {
		go tm.worker(i)
	}
}

func (tm *TaskManager) worker(id int) {
	logrus.Infof("Task worker %d started", id)

	for {
		select {
		case task := <-tm.queue:
			tm.processTask(task)
		case <-tm.stopChan:
			logrus.Infof("Task worker %d stopped", id)
			return
		}
	}
}

func (tm *TaskManager) processTask(task *Task) {
	logrus.Infof("Processing task %s (worker)", task.ID)

	// Update task status
	tm.updateTaskStatus(task.ID, TaskPending)

	// Select node for task
	node, err := tm.manager.NodeManager.SelectNodeForTask(task)
	if err != nil {
		logrus.Errorf("Failed to select node for task %s: %v", task.ID, err)
		tm.updateTaskStatus(task.ID, TaskFailed)
		return
	}

	// Assign task to node
	task.NodeID = node.ID
	tm.updateTaskStatus(task.ID, TaskAssigned)

	// Send task to node (simulation)
	if err := tm.sendTaskToNode(task, node); err != nil {
		logrus.Errorf("Failed to send task %s to node %s: %v", task.ID, node.ID, err)
		tm.updateTaskStatus(task.ID, TaskFailed)
		return
	}

	// Update task status
	tm.updateTaskStatus(task.ID, TaskRunning)
	task.StartedAt = time.Now().Format(time.RFC3339)

	logrus.Infof("Task %s started on node %s", task.ID, node.ID)
}

func (tm *TaskManager) sendTaskToNode(task *Task, node *Node) error {
	// In real implementation, this would send the task to the node via API
	// For simulation, we'll just wait and simulate success
	time.Sleep(100 * time.Millisecond)

	// Simulate task completion
	go func() {
		time.Sleep(5 * time.Second) // Simulate task running time

		tm.mu.Lock()
		task, exists := tm.tasks[task.ID]
		if exists {
			task.Status = TaskComplete
			task.CompletedAt = time.Now().Format(time.RFC3339)
			task.UpdatedAt = time.Now().Format(time.RFC3339)
		}
		tm.mu.Unlock()

		logrus.Infof("Task %s completed", task.ID)
	}()

	return nil
}

func (tm *TaskManager) updateTaskStatus(taskID string, status TaskStatus) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task, exists := tm.tasks[taskID]; exists {
		task.Status = status
		task.UpdatedAt = time.Now().Format(time.RFC3339)
	}
}

func (tm *TaskManager) validateTask(task *Task) error {
	if task.ID == "" {
		return fmt.Errorf("task ID is required")
	}

	if task.Name == "" {
		return fmt.Errorf("task name is required")
	}

	if task.Image == "" {
		return fmt.Errorf("task image is required")
	}

	if task.Resources.CPU <= 0 {
		return fmt.Errorf("task CPU must be positive")
	}

	if task.Resources.Memory <= 0 {
		return fmt.Errorf("task memory must be positive")
	}

	return nil
}

func (tm *TaskManager) Shutdown() {
	close(tm.stopChan)
	logrus.Info("Task manager shutdown")
}

func generateTaskID() string {
	return fmt.Sprintf("task-%x", time.Now().UnixNano())[:12]
}