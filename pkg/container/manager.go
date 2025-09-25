package container

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/sirupsen/logrus"
	"docker-impl/pkg/image"
	"docker-impl/pkg/store"
	"docker-impl/pkg/types"
)

type Manager struct {
	store       *store.Store
	imageMgr    *image.Manager
	running     map[string]*exec.Cmd
	mu          sync.Mutex
}

func NewManager(store *store.Store, imageMgr *image.Manager) *Manager {
	return &Manager{
		store:    store,
		imageMgr: imageMgr,
		running:  make(map[string]*exec.Cmd),
	}
}

func (m *Manager) CreateContainer(options types.ContainerCreateOptions) (*types.Container, error) {
	logrus.Infof("Creating container with image: %s", options.Config.Image)

	containerID := m.generateContainerID()
	containerName := options.Name
	if containerName == "" {
		containerName = containerID[:12]
	}

	if !m.imageMgr.ImageExists(options.Config.Image) {
		return nil, fmt.Errorf("image not found: %s", options.Config.Image)
	}

	now := time.Now()
	container := &types.Container{
		ID:          containerID,
		Name:        containerName,
		Image:       options.Config.Image,
		Status:      types.StatusCreated,
		PID:         0,
		CreatedAt:   now,
		Config:      options.Config,
		HostConfig:  options.HostConfig,
		Labels:      options.Labels,
		Driver:      "overlay2",
		Platform:    "linux",
		LogPath:     filepath.Join(m.store.GetContainersDir(), containerID, "container.log"),
		Network: types.NetworkSettings{
			NetworkMode: options.HostConfig.NetworkMode,
		},
		RootFS: types.RootFS{
			Type:   "layers",
			Layers: []string{"base-layer"},
		},
	}

	if err := m.saveContainer(container); err != nil {
		return nil, fmt.Errorf("failed to save container: %v", err)
	}

	logrus.Infof("Container created successfully: %s", containerID)
	return container, nil
}

func (m *Manager) StartContainer(containerID string) error {
	logrus.Infof("Starting container: %s", containerID)

	container, err := m.GetContainer(containerID)
	if err != nil {
		return fmt.Errorf("failed to get container: %v", err)
	}

	if container.Status == types.StatusRunning {
		return fmt.Errorf("container is already running")
	}

	if err := m.setupContainerFS(container); err != nil {
		return fmt.Errorf("failed to setup container filesystem: %v", err)
	}

	cmd, err := m.createContainerProcess(container)
	if err != nil {
		return fmt.Errorf("failed to create container process: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start container process: %v", err)
	}

	m.mu.Lock()
	m.running[containerID] = cmd
	m.mu.Unlock()

	container.Status = types.StatusRunning
	container.PID = cmd.Process.Pid
	container.StartedAt = time.Now()

	if err := m.saveContainer(container); err != nil {
		logrus.Warnf("Failed to save container state: %v", err)
	}

	go m.monitorContainer(containerID, cmd)

	logrus.Infof("Container started successfully: %s", containerID)
	return nil
}

func (m *Manager) StopContainer(containerID string, timeout int) error {
	logrus.Infof("Stopping container: %s", containerID)

	container, err := m.GetContainer(containerID)
	if err != nil {
		return fmt.Errorf("failed to get container: %v", err)
	}

	if container.Status != types.StatusRunning {
		return fmt.Errorf("container is not running")
	}

	m.mu.Lock()
	cmd, exists := m.running[containerID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("container process not found")
	}
	delete(m.running, containerID)
	m.mu.Unlock()

	if timeout > 0 {
		time.Sleep(time.Duration(timeout) * time.Second)
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		logrus.Warnf("Failed to send SIGTERM to container: %v", err)
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill container process: %v", err)
		}
	}

	container.Status = types.StatusStopped
	container.FinishedAt = time.Now()

	if err := m.saveContainer(container); err != nil {
		logrus.Warnf("Failed to save container state: %v", err)
	}

	logrus.Infof("Container stopped successfully: %s", containerID)
	return nil
}

func (m *Manager) RemoveContainer(containerID string, options types.ContainerRemoveOptions) error {
	logrus.Infof("Removing container: %s", containerID)

	container, err := m.GetContainer(containerID)
	if err != nil {
		return fmt.Errorf("failed to get container: %v", err)
	}

	if container.Status == types.StatusRunning && options.Force {
		if err := m.StopContainer(containerID, 0); err != nil {
			logrus.Warnf("Failed to stop container: %v", err)
		}
	} else if container.Status == types.StatusRunning {
		return fmt.Errorf("cannot remove running container without force flag")
	}

	containerPath := filepath.Join("containers", fmt.Sprintf("%s.json", containerID))
	if err := m.store.RemoveFile(containerPath); err != nil {
		return fmt.Errorf("failed to remove container file: %v", err)
	}

	containerDir := filepath.Join(m.store.GetContainersDir(), containerID)
	if err := os.RemoveAll(containerDir); err != nil {
		logrus.Warnf("Failed to remove container directory: %v", err)
	}

	logrus.Infof("Container removed successfully: %s", containerID)
	return nil
}

func (m *Manager) GetContainer(containerID string) (*types.Container, error) {
	containerPath := filepath.Join("containers", fmt.Sprintf("%s.json", containerID))

	var container types.Container
	if err := m.store.LoadJSON(containerPath, &container); err != nil {
		return nil, fmt.Errorf("failed to load container: %v", err)
	}

	return &container, nil
}

func (m *Manager) ListContainers(options types.ContainerListOptions) ([]*types.Container, error) {
	files, err := m.store.ListFiles("containers")
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}

	var containers []*types.Container
	for _, file := range files {
		if filepath.Ext(file) == ".json" {
			containerID := file[:len(file)-5]
			container, err := m.GetContainer(containerID)
			if err != nil {
				logrus.Warnf("Failed to load container %s: %v", containerID, err)
				continue
			}

			if !options.All && container.Status != types.StatusRunning {
				continue
			}

			containers = append(containers, container)
		}
	}

	return containers, nil
}

func (m *Manager) GetContainerLogs(containerID string) (string, error) {
	container, err := m.GetContainer(containerID)
	if err != nil {
		return "", fmt.Errorf("failed to get container: %v", err)
	}

	if _, err := os.Stat(container.LogPath); os.IsNotExist(err) {
		return "", nil
	}

	logData, err := os.ReadFile(container.LogPath)
	if err != nil {
		return "", fmt.Errorf("failed to read log file: %v", err)
	}

	return string(logData), nil
}

func (m *Manager) saveContainer(container *types.Container) error {
	containerPath := filepath.Join("containers", fmt.Sprintf("%s.json", container.ID))
	return m.store.SaveJSON(containerPath, container)
}

func (m *Manager) generateContainerID() string {
	data := fmt.Sprintf("container-%d", time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (m *Manager) setupContainerFS(container *types.Container) error {
	containerDir := filepath.Join(m.store.GetContainersDir(), container.ID)
	if err := os.MkdirAll(containerDir, 0755); err != nil {
		return fmt.Errorf("failed to create container directory: %v", err)
	}

	rootfsDir := filepath.Join(containerDir, "rootfs")
	if err := os.MkdirAll(rootfsDir, 0755); err != nil {
		return fmt.Errorf("failed to create rootfs directory: %v", err)
	}

	return nil
}

func (m *Manager) createContainerProcess(container *types.Container) (*exec.Cmd, error) {
	containerDir := filepath.Join(m.store.GetContainersDir(), container.ID)
	rootfsDir := filepath.Join(containerDir, "rootfs")

	cmd := exec.Command("/bin/sh")
	if len(container.Config.Cmd) > 0 {
		cmd = exec.Command(container.Config.Cmd[0], container.Config.Cmd[1:]...)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Chroot:     rootfsDir,
	}

	cmd.Env = container.Config.Env
	cmd.Dir = container.Config.WorkingDir
	if cmd.Dir == "" {
		cmd.Dir = "/"
	}

	logFile, err := os.Create(container.LogPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %v", err)
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	return cmd, nil
}

func (m *Manager) monitorContainer(containerID string, cmd *exec.Cmd) {
	err := cmd.Wait()

	m.mu.Lock()
	delete(m.running, containerID)
	m.mu.Unlock()

	container, err := m.GetContainer(containerID)
	if err != nil {
		logrus.Errorf("Failed to get container %s: %v", containerID, err)
		return
	}

	if err != nil {
		container.Status = types.StatusExited
		logrus.Errorf("Container %s exited with error: %v", containerID, err)
	} else {
		if cmd.ProcessState.Success() {
			container.Status = types.StatusExited
		} else {
			container.Status = types.StatusDead
		}
	}

	container.FinishedAt = time.Now()
	container.PID = 0

	if err := m.saveContainer(container); err != nil {
		logrus.Warnf("Failed to save container state: %v", err)
	}

	logrus.Infof("Container %s finished with status: %s", containerID, container.Status)
}

func (m *Manager) GetContainerStats(containerID string) (map[string]interface{}, error) {
	container, err := m.GetContainer(containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get container: %v", err)
	}

	if container.Status != types.StatusRunning {
		return nil, fmt.Errorf("container is not running")
	}

	stats := map[string]interface{}{
		"id":      container.ID,
		"name":    container.Name,
		"status":  container.Status,
		"pid":     container.PID,
		"image":   container.Image,
		"uptime":  time.Since(container.StartedAt).String(),
	}

	return stats, nil
}

func (m *Manager) ExecContainer(containerID string, cmd []string) error {
	container, err := m.GetContainer(containerID)
	if err != nil {
		return fmt.Errorf("failed to get container: %v", err)
	}

	if container.Status != types.StatusRunning {
		return fmt.Errorf("container is not running")
	}

	execCmd := exec.Command(cmd[0], cmd[1:]...)
	execCmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}

	return execCmd.Run()
}

func (m *Manager) ResizeContainerTTY(containerID string, height, width uint16) error {
	container, err := m.GetContainer(containerID)
	if err != nil {
		return fmt.Errorf("failed to get container: %v", err)
	}

	if container.Status != types.StatusRunning {
		return fmt.Errorf("container is not running")
	}

	m.mu.Lock()
	cmd, exists := m.running[containerID]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("container process not found")
	}

	if cmd.Process == nil {
		return fmt.Errorf("container process not available")
	}

	ws := &struct {
		Height uint16
		Width  uint16
	}{
		Height: height,
		Width:  width,
	}

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(cmd.Process.Pid),
		uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)

	if errno != 0 {
		return fmt.Errorf("failed to resize TTY: %v", errno)
	}

	return nil
}