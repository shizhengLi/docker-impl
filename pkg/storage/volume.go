package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Volume struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Driver      string            `json:"driver"`
	Mountpoint  string            `json:"mountpoint"`
	CreatedAt   string            `json:"created_at"`
	Status      map[string]string `json:"status"`
	Labels      map[string]string `json:"labels"`
	Options     map[string]string `json:"options"`
	Scope       string            `json:"scope"`
	UsageData   *UsageData        `json:"usage_data"`
}

type UsageData struct {
	Size        int64   `json:"size"`
	RefCount    int     `json:"ref_count"`
	LastUsed    string  `json:"last_used"`
	AccessCount int     `json:"access_count"`
}

type VolumeManager struct {
	baseDir   string
	volumes   map[string]*Volume
	mounts    map[string][]string // volumeID -> containerIDs
	mu        sync.RWMutex
	driver    VolumeDriver
}

type VolumeDriver interface {
	Create(name string, options map[string]string) (*Volume, error)
	Remove(volume *Volume) error
	Mount(volume *Volume, target string) error
	Unmount(volume *Volume, target string) error
	GetPath(volume *Volume) string
	Usage(volume *Volume) (*UsageData, error)
}

type LocalVolumeDriver struct {
	baseDir string
}

func NewLocalVolumeDriver(baseDir string) *LocalVolumeDriver {
	return &LocalVolumeDriver{baseDir: baseDir}
}

func (d *LocalVolumeDriver) Create(name string, options map[string]string) (*Volume, error) {
	volumePath := filepath.Join(d.baseDir, name)

	if err := os.MkdirAll(volumePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create volume directory: %v", err)
	}

	volume := &Volume{
		ID:         generateVolumeID(name),
		Name:       name,
		Driver:     "local",
		Mountpoint: volumePath,
		CreatedAt:  time.Now().Format(time.RFC3339),
		Status:     make(map[string]string),
		Labels:     make(map[string]string),
		Options:    options,
		Scope:      "local",
		UsageData: &UsageData{
			Size:        0,
			RefCount:    0,
			LastUsed:    "",
			AccessCount: 0,
		},
	}

	// Set default options
	if _, ok := options["type"]; !ok {
		options["type"] = "local"
	}

	logrus.Infof("Created local volume: %s at %s", name, volumePath)
	return volume, nil
}

func (d *LocalVolumeDriver) Remove(volume *Volume) error {
	if volume.UsageData.RefCount > 0 {
		return fmt.Errorf("volume %s is still in use (%d references)", volume.Name, volume.UsageData.RefCount)
	}

	if err := os.RemoveAll(volume.Mountpoint); err != nil {
		return fmt.Errorf("failed to remove volume directory: %v", err)
	}

	logrus.Infof("Removed local volume: %s", volume.Name)
	return nil
}

func (d *LocalVolumeDriver) Mount(volume *Volume, target string) error {
	// For local volumes, we just ensure the target directory exists
	if err := os.MkdirAll(target, 0755); err != nil {
		return fmt.Errorf("failed to create mount target: %v", err)
	}

	logrus.Infof("Mounted local volume %s to %s", volume.Name, target)
	return nil
}

func (d *LocalVolumeDriver) Unmount(volume *Volume, target string) error {
	// For local volumes, unmount is a no-op
	logrus.Infof("Unmounted local volume %s from %s", volume.Name, target)
	return nil
}

func (d *LocalVolumeDriver) GetPath(volume *Volume) string {
	return volume.Mountpoint
}

func (d *LocalVolumeDriver) Usage(volume *Volume) (*UsageData, error) {
	size, err := d.calculateDirectorySize(volume.Mountpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate volume size: %v", err)
	}

	volume.UsageData.Size = size
	return volume.UsageData, nil
}

func (d *LocalVolumeDriver) calculateDirectorySize(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

func NewVolumeManager(baseDir string) (*VolumeManager, error) {
	vm := &VolumeManager{
		baseDir: baseDir,
		volumes: make(map[string]*Volume),
		mounts:   make(map[string][]string),
		driver:  NewLocalVolumeDriver(baseDir),
	}

	if err := vm.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize volume manager: %v", err)
	}

	return vm, nil
}

func (vm *VolumeManager) init() error {
	dirs := []string{
		vm.baseDir,
		filepath.Join(vm.baseDir, "volumes"),
		filepath.Join(vm.baseDir, "metadata"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}

	// Load existing volumes
	if err := vm.loadVolumes(); err != nil {
		logrus.Warnf("Failed to load existing volumes: %v", err)
	}

	logrus.Infof("Volume manager initialized with base directory: %s", vm.baseDir)
	return nil
}

func (vm *VolumeManager) CreateVolume(name string, options map[string]string, labels map[string]string) (*Volume, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Check if volume already exists
	if _, exists := vm.volumes[name]; exists {
		return nil, fmt.Errorf("volume %s already exists", name)
	}

	// Create volume using driver
	volume, err := vm.driver.Create(name, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %v", err)
	}

	// Apply labels
	volume.Labels = labels

	// Save volume metadata
	if err := vm.saveVolumeMetadata(volume); err != nil {
		vm.driver.Remove(volume)
		return nil, fmt.Errorf("failed to save volume metadata: %v", err)
	}

	vm.volumes[name] = volume
	logrus.Infof("Created volume: %s", name)

	return volume, nil
}

func (vm *VolumeManager) RemoveVolume(name string, force bool) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	volume, exists := vm.volumes[name]
	if !exists {
		return fmt.Errorf("volume %s not found", name)
	}

	if volume.UsageData.RefCount > 0 && !force {
		return fmt.Errorf("volume %s is in use by %d containers", name, volume.UsageData.RefCount)
	}

	// Remove from all mounts
	for containerID, volumeIDs := range vm.mounts {
		for i, vid := range volumeIDs {
			if vid == volume.ID {
				vm.mounts[containerID] = append(volumeIDs[:i], volumeIDs[i+1:]...)
				break
			}
		}
	}

	// Remove volume
	if err := vm.driver.Remove(volume); err != nil {
		return fmt.Errorf("failed to remove volume: %v", err)
	}

	// Remove metadata
	metadataPath := filepath.Join(vm.baseDir, "metadata", name+".json")
	if err := os.Remove(metadataPath); err != nil {
		logrus.Warnf("Failed to remove volume metadata: %v", err)
	}

	delete(vm.volumes, name)
	logrus.Infof("Removed volume: %s", name)

	return nil
}

func (vm *VolumeManager) MountVolume(name, containerID, target string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	volume, exists := vm.volumes[name]
	if !exists {
		return fmt.Errorf("volume %s not found", name)
	}

	// Mount volume
	if err := vm.driver.Mount(volume, target); err != nil {
		return fmt.Errorf("failed to mount volume: %v", err)
	}

	// Update usage data
	volume.UsageData.RefCount++
	volume.UsageData.LastUsed = time.Now().Format(time.RFC3339)
	volume.UsageData.AccessCount++

	// Record mount
	vm.mounts[containerID] = append(vm.mounts[containerID], volume.ID)

	// Save metadata
	if err := vm.saveVolumeMetadata(volume); err != nil {
		logrus.Warnf("Failed to save volume metadata: %v", err)
	}

	logrus.Infof("Mounted volume %s to container %s at %s", name, containerID, target)
	return nil
}

func (vm *VolumeManager) UnmountVolume(name, containerID string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	volume, exists := vm.volumes[name]
	if !exists {
		return fmt.Errorf("volume %s not found", name)
	}

	// Find and remove mount
	if volumeIDs, containerExists := vm.mounts[containerID]; containerExists {
		for i, vid := range volumeIDs {
			if vid == volume.ID {
				vm.mounts[containerID] = append(volumeIDs[:i], volumeIDs[i+1:]...)
				break
			}
		}
	}

	// Unmount volume
	target := vm.driver.GetPath(volume)
	if err := vm.driver.Unmount(volume, target); err != nil {
		return fmt.Errorf("failed to unmount volume: %v", err)
	}

	// Update usage data
	if volume.UsageData.RefCount > 0 {
		volume.UsageData.RefCount--
	}

	// Save metadata
	if err := vm.saveVolumeMetadata(volume); err != nil {
		logrus.Warnf("Failed to save volume metadata: %v", err)
	}

	logrus.Infof("Unmounted volume %s from container %s", name, containerID)
	return nil
}

func (vm *VolumeManager) GetVolume(name string) (*Volume, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	volume, exists := vm.volumes[name]
	if !exists {
		return nil, fmt.Errorf("volume %s not found", name)
	}

	return volume, nil
}

func (vm *VolumeManager) ListVolumes() ([]*Volume, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	var volumes []*Volume
	for _, volume := range vm.volumes {
		volumes = append(volumes, volume)
	}

	return volumes, nil
}

func (vm *VolumeManager) PruneVolumes() (int64, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	var reclaimedSpace int64
	var removedVolumes []string

	for name, volume := range vm.volumes {
		if volume.UsageData.RefCount == 0 {
			reclaimedSpace += volume.UsageData.Size
			removedVolumes = append(removedVolumes, name)
		}
	}

	// Remove unused volumes
	for _, name := range removedVolumes {
		if err := vm.driver.Remove(vm.volumes[name]); err != nil {
			logrus.Warnf("Failed to remove volume %s: %v", name, err)
			continue
		}

		metadataPath := filepath.Join(vm.baseDir, "metadata", name+".json")
		if err := os.Remove(metadataPath); err != nil {
			logrus.Warnf("Failed to remove volume metadata: %v", err)
		}

		delete(vm.volumes, name)
	}

	logrus.Infof("Pruned %d volumes, reclaimed %d bytes", len(removedVolumes), reclaimedSpace)
	return reclaimedSpace, nil
}

func (vm *VolumeManager) saveVolumeMetadata(volume *Volume) error {
	metadataPath := filepath.Join(vm.baseDir, "metadata", volume.Name+".json")
	data, err := json.MarshalIndent(volume, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal volume metadata: %v", err)
	}

	return os.WriteFile(metadataPath, data, 0644)
}

func (vm *VolumeManager) loadVolumes() error {
	metadataDir := filepath.Join(vm.baseDir, "metadata")

	files, err := os.ReadDir(metadataDir)
	if err != nil {
		return fmt.Errorf("failed to read metadata directory: %v", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			metadataPath := filepath.Join(metadataDir, file.Name())
			data, err := os.ReadFile(metadataPath)
			if err != nil {
				logrus.Warnf("Failed to read volume metadata %s: %v", file.Name(), err)
				continue
			}

			var volume Volume
			if err := json.Unmarshal(data, &volume); err != nil {
				logrus.Warnf("Failed to unmarshal volume metadata %s: %v", file.Name(), err)
				continue
			}

			vm.volumes[volume.Name] = &volume
			logrus.Debugf("Loaded volume: %s", volume.Name)
		}
	}

	return nil
}

func (vm *VolumeManager) GetUsageStats() map[string]interface{} {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	var totalSize int64
	totalVolumes := len(vm.volumes)
	totalMounts := 0
	inUseVolumes := 0

	for _, volume := range vm.volumes {
		totalSize += volume.UsageData.Size
		if volume.UsageData.RefCount > 0 {
			inUseVolumes++
		}
		totalMounts += volume.UsageData.RefCount
	}

	return map[string]interface{}{
		"total_size_bytes":   totalSize,
		"total_volumes":      totalVolumes,
		"in_use_volumes":     inUseVolumes,
		"unused_volumes":     totalVolumes - inUseVolumes,
		"total_mounts":       totalMounts,
		"driver":             "local",
		"base_dir":           vm.baseDir,
	}
}

func generateVolumeID(name string) string {
	// Simplified volume ID generation
	return fmt.Sprintf("vol-%x", name)[:12]
}