package storage

import (
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"
)

type StorageManager struct {
	overlayDriver *OverlayDriver
	volumeManager *VolumeManager
	baseDir       string
	mu            sync.RWMutex
}

type StorageConfig struct {
	RootDir          string `json:"root_dir"`
	OverlayDriver    string `json:"overlay_driver"`
	VolumeDriver     string `json:"volume_driver"`
	EnableQuotas     bool   `json:"enable_quotas"`
	EnableEncryption bool   `json:"enable_encryption"`
}

type ImageLayer struct {
	ID        string `json:"id"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	Created   string `json:"created"`
	ChainID   string `json:"chain_id"`
	DiffID    string `json:"diff_id"`
	ParentID  string `json:"parent_id"`
}

type ContainerStorage struct {
	ID           string   `json:"id"`
	ImageID      string   `json:"image_id"`
	LayerIDs     []string `json:"layer_ids"`
	MountPoint   string   `json:"mount_point"`
	VolumeMounts []VolumeMount `json:"volume_mounts"`
	Size         int64    `json:"size"`
	Created      string   `json:"created"`
}

type VolumeMount struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only"`
}

func NewStorageManager(config *StorageConfig) (*StorageManager, error) {
	if config == nil {
		config = &StorageConfig{
			RootDir:       "/var/lib/mydocker",
			OverlayDriver: "overlay",
			VolumeDriver:  "local",
		}
	}

	sm := &StorageManager{
		baseDir: config.RootDir,
	}

	if err := sm.init(config); err != nil {
		return nil, fmt.Errorf("failed to initialize storage manager: %v", err)
	}

	return sm, nil
}

func (sm *StorageManager) init(config *StorageConfig) error {
	// Create base directories
	dirs := []string{
		sm.baseDir,
		filepath.Join(sm.baseDir, "overlay"),
		filepath.Join(sm.baseDir, "volumes"),
		filepath.Join(sm.baseDir, "images"),
		filepath.Join(sm.baseDir, "containers"),
	}

	for _, dir := range dirs {
		if err := createDirectoryIfNotExists(dir); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}

	// Initialize overlay driver
	overlayDir := filepath.Join(sm.baseDir, "overlay")
	overlayDriver, err := NewOverlayDriver(overlayDir)
	if err != nil {
		return fmt.Errorf("failed to create overlay driver: %v", err)
	}
	sm.overlayDriver = overlayDriver

	// Initialize volume manager
	volumeDir := filepath.Join(sm.baseDir, "volumes")
	volumeManager, err := NewVolumeManager(volumeDir)
	if err != nil {
		return fmt.Errorf("failed to create volume manager: %v", err)
	}
	sm.volumeManager = volumeManager

	logrus.Infof("Storage manager initialized with base directory: %s", sm.baseDir)
	return nil
}

func (sm *StorageManager) CreateImageLayer(parentID, diffID string, diff io.Reader) (*ImageLayer, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	logrus.Infof("Creating image layer with parent %s", parentID)

	// Create layer
	layer, err := sm.overlayDriver.CreateLayer(parentID, diffID)
	if err != nil {
		return nil, fmt.Errorf("failed to create layer: %v", err)
	}

	// Apply diff
	diffStats, err := sm.overlayDriver.ApplyDiff(layer.ID, diff)
	if err != nil {
		sm.overlayDriver.DeleteLayer(layer.ID)
		return nil, fmt.Errorf("failed to apply diff: %v", err)
	}

	// Convert to image layer
	imageLayer := &ImageLayer{
		ID:       layer.ID,
		Digest:   layer.Digest,
		Size:     diffStats.Size,
		Created:  layer.Created,
		ChainID:  layer.ChainID,
		DiffID:   layer.DiffID,
		ParentID: layer.Parent,
	}

	logrus.Infof("Created image layer: %s (%d bytes)", imageLayer.ID, imageLayer.Size)
	return imageLayer, nil
}

func (sm *StorageManager) GetImageLayer(layerID string) (*ImageLayer, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	layer, err := sm.overlayDriver.GetLayer(layerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get layer: %v", err)
	}

	return &ImageLayer{
		ID:       layer.ID,
		Digest:   layer.Digest,
		Size:     layer.Size,
		Created:  layer.Created,
		ChainID:  layer.ChainID,
		DiffID:   layer.DiffID,
		ParentID: layer.Parent,
	}, nil
}

func (sm *StorageManager) ListImageLayers() ([]*ImageLayer, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	layers, err := sm.overlayDriver.ListLayers()
	if err != nil {
		return nil, fmt.Errorf("failed to list layers: %v", err)
	}

	var imageLayers []*ImageLayer
	for _, layer := range layers {
		imageLayers = append(imageLayers, &ImageLayer{
			ID:       layer.ID,
			Digest:   layer.Digest,
			Size:     layer.Size,
			Created:  layer.Created,
			ChainID:  layer.ChainID,
			DiffID:   layer.DiffID,
			ParentID: layer.Parent,
		})
	}

	return imageLayers, nil
}

func (sm *StorageManager) DeleteImageLayer(layerID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	logrus.Infof("Deleting image layer: %s", layerID)

	if err := sm.overlayDriver.DeleteLayer(layerID); err != nil {
		return fmt.Errorf("failed to delete layer: %v", err)
	}

	logrus.Infof("Deleted image layer: %s", layerID)
	return nil
}

func (sm *StorageManager) CreateContainerStorage(containerID, imageID string, layerIDs []string, volumeMounts []VolumeMount) (*ContainerStorage, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	logrus.Infof("Creating container storage for %s", containerID)

	// Create container mount point
	mountPoint := filepath.Join(sm.baseDir, "containers", containerID, "rootfs")
	containerStorage := &ContainerStorage{
		ID:           containerID,
		ImageID:      imageID,
		LayerIDs:     layerIDs,
		MountPoint:   mountPoint,
		VolumeMounts: volumeMounts,
		Created:      getTimestamp(),
	}

	// Mount overlay filesystem
	if err := sm.overlayDriver.Mount(layerIDs, mountPoint); err != nil {
		return nil, fmt.Errorf("failed to mount overlay: %v", err)
	}

	// Mount volumes
	for _, volumeMount := range volumeMounts {
		targetPath := filepath.Join(mountPoint, volumeMount.Target)
		if err := sm.volumeManager.MountVolume(volumeMount.Name, containerID, targetPath); err != nil {
			logrus.Warnf("Failed to mount volume %s: %v", volumeMount.Name, err)
		}
	}

	// Calculate total size
	totalSize, err := sm.calculateContainerSize(containerStorage)
	if err != nil {
		logrus.Warnf("Failed to calculate container size: %v", err)
	}
	containerStorage.Size = totalSize

	logrus.Infof("Created container storage for %s at %s (%d bytes)", containerID, mountPoint, totalSize)
	return containerStorage, nil
}

func (sm *StorageManager) GetContainerStorage(containerID string) (*ContainerStorage, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Load container storage from metadata
	metadataPath := filepath.Join(sm.baseDir, "containers", containerID, "storage.json")
	if _, err := filepath.Stat(metadataPath); err != nil {
		return nil, fmt.Errorf("container storage not found: %v", err)
	}

	// In real implementation, this would load from JSON
	// For now, return basic structure
	return &ContainerStorage{
		ID:         containerID,
		MountPoint: filepath.Join(sm.baseDir, "containers", containerID, "rootfs"),
	}, nil
}

func (sm *StorageManager) RemoveContainerStorage(containerID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	logrus.Infof("Removing container storage for %s", containerID)

	containerStorage, err := sm.GetContainerStorage(containerID)
	if err != nil {
		return fmt.Errorf("failed to get container storage: %v", err)
	}

	// Unmount volumes
	for _, volumeMount := range containerStorage.VolumeMounts {
		if err := sm.volumeManager.UnmountVolume(volumeMount.Name, containerID); err != nil {
			logrus.Warnf("Failed to unmount volume %s: %v", volumeMount.Name, err)
		}
	}

	// Unmount overlay filesystem
	if err := sm.overlayDriver.Unmount(containerStorage.MountPoint); err != nil {
		logrus.Warnf("Failed to unmount overlay: %v", err)
	}

	// Remove container directory
	containerDir := filepath.Join(sm.baseDir, "containers", containerID)
	if err := removeAll(containerDir); err != nil {
		logrus.Warnf("Failed to remove container directory: %v", err)
	}

	logrus.Infof("Removed container storage for %s", containerID)
	return nil
}

func (sm *StorageManager) CreateVolume(name string, options map[string]string, labels map[string]string) (*Volume, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.volumeManager.CreateVolume(name, options, labels)
}

func (sm *StorageManager) RemoveVolume(name string, force bool) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.volumeManager.RemoveVolume(name, force)
}

func (sm *StorageManager) GetVolume(name string) (*Volume, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.volumeManager.GetVolume(name)
}

func (sm *StorageManager) ListVolumes() ([]*Volume, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.volumeManager.ListVolumes()
}

func (sm *StorageManager) PruneVolumes() (int64, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.volumeManager.PruneVolumes()
}

func (sm *StorageManager) MountVolume(name, containerID, target string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.volumeManager.MountVolume(name, containerID, target)
}

func (sm *StorageManager) UnmountVolume(name, containerID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	return sm.volumeManager.UnmountVolume(name, containerID)
}

func (sm *StorageManager) GetStorageStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	overlayStats := sm.overlayDriver.GetUsageStats()
	volumeStats := sm.volumeManager.GetUsageStats()

	return map[string]interface{}{
		"overlay_driver": overlayStats,
		"volume_manager": volumeStats,
		"base_dir":       sm.baseDir,
	}
}

func (sm *StorageManager) Cleanup() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	logrus.Info("Cleaning up storage manager")

	if sm.overlayDriver != nil {
		sm.overlayDriver.Cleanup()
	}

	logrus.Info("Storage manager cleaned up")
	return nil
}

func (sm *StorageManager) calculateContainerSize(container *ContainerStorage) (int64, error) {
	var totalSize int64

	// Add layer sizes
	for _, layerID := range container.LayerIDs {
		layer, err := sm.overlayDriver.GetLayer(layerID)
		if err != nil {
			logrus.Warnf("Failed to get layer %s: %v", layerID, err)
			continue
		}
		totalSize += layer.Size
	}

	// Add volume sizes
	for _, volumeMount := range container.VolumeMounts {
		volume, err := sm.volumeManager.GetVolume(volumeMount.Name)
		if err != nil {
			logrus.Warnf("Failed to get volume %s: %v", volumeMount.Name, err)
			continue
		}
		totalSize += volume.UsageData.Size
	}

	return totalSize, nil
}

func createDirectoryIfNotExists(path string) error {
	if _, err := filepath.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

func removeAll(path string) error {
	return os.RemoveAll(path)
}