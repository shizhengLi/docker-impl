package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

type OverlayDriver struct {
	baseDir     string
	upperDir    string
	workDir     string
	mergedDir   string
	layers      map[string]*Layer
	mu          sync.RWMutex
	mountPoints map[string]string
}

type Layer struct {
	ID        string `json:"id"`
	Parent    string `json:"parent"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	Created   string `json:"created"`
	Path      string `json:"path"`
	DiffID    string `json:"diff_id"`
	ChainID   string `json:"chain_id"`
}

type Diff struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Added    []string `json:"added"`
	Deleted  []string `json:"deleted"`
	Modified []string `json:"modified"`
	Size     int64    `json:"size"`
}

func NewOverlayDriver(baseDir string) (*OverlayDriver, error) {
	driver := &OverlayDriver{
		baseDir:     baseDir,
		layers:      make(map[string]*Layer),
		mountPoints: make(map[string]string),
	}

	if err := driver.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize overlay driver: %v", err)
	}

	return driver, nil
}

func (d *OverlayDriver) init() error {
	dirs := []string{
		d.baseDir,
		filepath.Join(d.baseDir, "layers"),
		filepath.Join(d.baseDir, "diffs"),
		filepath.Join(d.baseDir, "mounts"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}

	logrus.Infof("Overlay driver initialized with base directory: %s", d.baseDir)
	return nil
}

func (d *OverlayDriver) CreateLayer(parentID, diffID string) (*Layer, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	layerID := d.generateLayerID(diffID)
	layerPath := filepath.Join(d.baseDir, "layers", layerID)

	// Create layer directory
	if err := os.MkdirAll(layerPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create layer directory: %v", err)
	}

	// Create layer metadata
	layer := &Layer{
		ID:      layerID,
		Parent:  parentID,
		DiffID:  diffID,
		Size:    0,
		Created: getTimestamp(),
		Path:    layerPath,
	}

	// Calculate chain ID
	if parentID == "" {
		layer.ChainID = diffID
	} else {
		parentLayer, exists := d.layers[parentID]
		if !exists {
			return nil, fmt.Errorf("parent layer not found: %s", parentID)
		}
		layer.ChainID = fmt.Sprintf("%s-%s", parentLayer.ChainID, diffID)
	}

	// Save layer metadata
	if err := d.saveLayerMetadata(layer); err != nil {
		return nil, fmt.Errorf("failed to save layer metadata: %v", err)
	}

	d.layers[layerID] = layer
	logrus.Infof("Created layer: %s", layerID)

	return layer, nil
}

func (d *OverlayDriver) ApplyDiff(layerID string, diff io.Reader) (*Diff, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	layer, exists := d.layers[layerID]
	if !exists {
		return nil, fmt.Errorf("layer not found: %s", layerID)
	}

	// Create diff directory
	diffDir := filepath.Join(d.baseDir, "diffs", layerID)
	if err := os.MkdirAll(diffDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create diff directory: %v", err)
	}

	// Track file changes
	diffStats := &Diff{
		ID:   layerID,
		Type: "overlay",
	}

	// Apply diff (simplified - in real implementation would handle tar streams)
	size, err := d.extractDiff(diff, diffDir, diffStats)
	if err != nil {
		return nil, fmt.Errorf("failed to extract diff: %v", err)
	}

	diffStats.Size = size
	layer.Size = size

	// Update layer metadata
	if err := d.saveLayerMetadata(layer); err != nil {
		return nil, fmt.Errorf("failed to save layer metadata: %v", err)
	}

	logrus.Infof("Applied diff to layer %s: %d bytes, %d added, %d modified",
		layerID, size, len(diffStats.Added), len(diffStats.Modified))

	return diffStats, nil
}

func (d *OverlayDriver) extractDiff(diff io.Reader, targetDir string, diffStats *Diff) (int64, error) {
	// Simplified diff extraction
	// In real implementation, this would handle tar streams with proper file operations
	var totalSize int64

	// Simulate extracting files
	// For demo purposes, we'll just create some example files
	exampleFiles := []struct {
		path    string
		content string
	}{
		{"bin/sh", "#!/bin/sh\necho 'Hello from container'\n"},
		{"etc/hostname", "container-hostname\n"},
		{"etc/hosts", "127.0.0.1 localhost\n::1 localhost\n"},
		{"etc/resolv.conf", "nameserver 8.8.8.8\n"},
	}

	for _, file := range exampleFiles {
		fullPath := filepath.Join(targetDir, file.path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return 0, fmt.Errorf("failed to create directory: %v", err)
		}

		if err := os.WriteFile(fullPath, []byte(file.content), 0644); err != nil {
			return 0, fmt.Errorf("failed to write file: %v", err)
		}

		diffStats.Added = append(diffStats.Added, file.path)
		totalSize += int64(len(file.content))
	}

	return totalSize, nil
}

func (d *OverlayDriver) Mount(layers []string, mountPoint string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	logrus.Infof("Mounting layers %v to %s", layers, mountPoint)

	// Create mount point
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %v", err)
	}

	// Create overlay directories for this mount
	overlayDir := filepath.Join(d.baseDir, "mounts", filepath.Base(mountPoint))
	upperDir := filepath.Join(overlayDir, "upper")
	workDir := filepath.Join(overlayDir, "work")

	for _, dir := range []string{overlayDir, upperDir, workDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create overlay directory: %v", err)
		}
	}

	// Prepare lower directories
	var lowerDirs []string
	for _, layerID := range layers {
		layer, exists := d.layers[layerID]
		if !exists {
			return fmt.Errorf("layer not found: %s", layerID)
		}
		lowerDirs = append(lowerDirs, filepath.Join(d.baseDir, "diffs", layerID))
	}

	lowerDir := strings.Join(lowerDirs, ":")

	// Mount overlay filesystem
	// Note: This requires overlay filesystem support and root privileges
	// For demonstration, we'll simulate the mount
	if err := d.simulateOverlayMount(lowerDir, upperDir, workDir, mountPoint); err != nil {
		return fmt.Errorf("failed to mount overlay: %v", err)
	}

	d.mountPoints[mountPoint] = overlayDir
	logrus.Infof("Mounted overlay filesystem at %s", mountPoint)

	return nil
}

func (d *OverlayDriver) simulateOverlayMount(lowerDir, upperDir, workDir, mountPoint string) error {
	// In a real implementation, this would use the mount syscall:
	// mount("overlay", mountPoint, "overlay", 0,
	//     fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir))

	// For demonstration, we'll create a simple directory structure
	// and copy files from lower layers to simulate overlay behavior

	// Create basic structure
	dirs := []string{
		filepath.Join(mountPoint, "bin"),
		filepath.Join(mountPoint, "etc"),
		filepath.Join(mountPoint, "usr"),
		filepath.Join(mountPoint, "var"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create mount directory: %v", err)
		}
	}

	// Create basic files
	files := []struct {
		path    string
		content string
	}{
		{filepath.Join(mountPoint, "etc", "hostname"), "mydocker-container\n"},
		{filepath.Join(mountPoint, "etc", "hosts"), "127.0.0.1 localhost\n"},
		{filepath.Join(mountPoint, "etc", "resolv.conf"), "nameserver 8.8.8.8\n"},
		{filepath.Join(mountPoint, "bin", "sh"), "#!/bin/sh\n"},
	}

	for _, file := range files {
		if err := os.WriteFile(file.path, []byte(file.content), 0644); err != nil {
			return fmt.Errorf("failed to create mount file: %v", err)
		}
	}

	logrus.Debugf("Simulated overlay mount at %s", mountPoint)
	return nil
}

func (d *OverlayDriver) Unmount(mountPoint string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	logrus.Infof("Unmounting %s", mountPoint)

	// Simulate unmount
	if err := d.simulateOverlayUnmount(mountPoint); err != nil {
		return fmt.Errorf("failed to unmount: %v", err)
	}

	// Clean up overlay directories
	if overlayDir, exists := d.mountPoints[mountPoint]; exists {
		if err := os.RemoveAll(overlayDir); err != nil {
			logrus.Warnf("Failed to remove overlay directory: %v", err)
		}
		delete(d.mountPoints, mountPoint)
	}

	logrus.Infof("Unmounted %s", mountPoint)
	return nil
}

func (d *OverlayDriver) simulateOverlayUnmount(mountPoint string) error {
	// In real implementation, this would use umount syscall
	// For demonstration, just remove the mount point
	return os.RemoveAll(mountPoint)
}

func (d *OverlayDriver) GetLayer(layerID string) (*Layer, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	layer, exists := d.layers[layerID]
	if !exists {
		return nil, fmt.Errorf("layer not found: %s", layerID)
	}

	return layer, nil
}

func (d *OverlayDriver) ListLayers() ([]*Layer, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var layers []*Layer
	for _, layer := range d.layers {
		layers = append(layers, layer)
	}

	return layers, nil
}

func (d *OverlayDriver) DeleteLayer(layerID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	layer, exists := d.layers[layerID]
	if !exists {
		return fmt.Errorf("layer not found: %s", layerID)
	}

	// Remove layer files
	if err := os.RemoveAll(layer.Path); err != nil {
		logrus.Warnf("Failed to remove layer directory: %v", err)
	}

	// Remove diff files
	diffDir := filepath.Join(d.baseDir, "diffs", layerID)
	if err := os.RemoveAll(diffDir); err != nil {
		logrus.Warnf("Failed to remove diff directory: %v", err)
	}

	// Remove from memory
	delete(d.layers, layerID)

	logrus.Infof("Deleted layer: %s", layerID)
	return nil
}

func (d *OverlayDriver) GetDiff(layerID string) (*Diff, error) {
	// Simplified diff generation
	// In real implementation, this would calculate actual differences
	return &Diff{
		ID:       layerID,
		Type:     "overlay",
		Added:    []string{"/bin/sh", "/etc/hostname", "/etc/hosts"},
		Modified: []string{"/etc/resolv.conf"},
		Deleted:  []string{},
	}, nil
}

func (d *OverlayDriver) GetUsageStats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var totalSize int64
	layerCount := len(d.layers)
	mountCount := len(d.mountPoints)

	for _, layer := range d.layers {
		totalSize += layer.Size
	}

	return map[string]interface{}{
		"total_size_bytes": totalSize,
		"layer_count":      layerCount,
		"mount_count":      mountCount,
		"driver":           "overlay",
		"base_dir":         d.baseDir,
	}
}

func (d *OverlayDriver) saveLayerMetadata(layer *Layer) error {
	metadataPath := filepath.Join(layer.Path, "layer.json")
	// In real implementation, this would save JSON metadata
	// For now, just create the directory structure
	return os.MkdirAll(layer.Path, 0755)
}

func (d *OverlayDriver) generateLayerID(diffID string) string {
	// Simplified layer ID generation
	return fmt.Sprintf("sha256:%x", diffID[:32])
}

func (d *OverlayDriver) Cleanup() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	logrus.Info("Cleaning up overlay driver")

	// Unmount all mount points
	for mountPoint := range d.mountPoints {
		if err := d.Unmount(mountPoint); err != nil {
			logrus.Warnf("Failed to unmount %s: %v", mountPoint, err)
		}
	}

	// Remove base directory
	if err := os.RemoveAll(d.baseDir); err != nil {
		logrus.Warnf("Failed to remove base directory: %v", err)
	}

	logrus.Info("Overlay driver cleaned up")
	return nil
}

func getTimestamp() string {
	// Simplified timestamp generation
	return "now"
}