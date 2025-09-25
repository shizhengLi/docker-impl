package image

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"docker-impl/pkg/store"
	"docker-impl/pkg/types"
)

type Manager struct {
	store *store.Store
}

func NewManager(store *store.Store) *Manager {
	return &Manager{
		store: store,
	}
}

func (m *Manager) CreateImage(imageName, tag string, config types.ImageConfig) (*types.Image, error) {
	logrus.Infof("Creating image: %s:%s", imageName, tag)

	imageID := m.generateImageID(imageName, tag)

	image := &types.Image{
		ID:        imageID,
		Name:      imageName,
		Tag:       tag,
		Size:      0,
		CreatedAt: time.Now(),
		Config:    config,
		Layers:    []string{"base-layer"},
		Labels:    config.Labels,
	}

	imagePath := filepath.Join("images", fmt.Sprintf("%s.json", imageID))
	if err := m.store.SaveJSON(imagePath, image); err != nil {
		return nil, fmt.Errorf("failed to save image metadata: %v", err)
	}

	logrus.Infof("Image created successfully: %s", imageID)
	return image, nil
}

func (m *Manager) GetImage(imageID string) (*types.Image, error) {
	imagePath := filepath.Join("images", fmt.Sprintf("%s.json", imageID))

	var image types.Image
	if err := m.store.LoadJSON(imagePath, &image); err != nil {
		return nil, fmt.Errorf("failed to load image: %v", err)
	}

	return &image, nil
}

func (m *Manager) ListImages() ([]*types.Image, error) {
	imagesDir := m.store.GetImagesDir()

	files, err := m.store.ListFiles("images")
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %v", err)
	}

	var images []*types.Image
	for _, file := range files {
		if filepath.Ext(file) == ".json" {
			imageID := file[:len(file)-5]
			image, err := m.GetImage(imageID)
			if err != nil {
				logrus.Warnf("Failed to load image %s: %v", imageID, err)
				continue
			}
			images = append(images, image)
		}
	}

	return images, nil
}

func (m *Manager) RemoveImage(imageID string) error {
	logrus.Infof("Removing image: %s", imageID)

	image, err := m.GetImage(imageID)
	if err != nil {
		return fmt.Errorf("failed to get image: %v", err)
	}

	imagePath := filepath.Join("images", fmt.Sprintf("%s.json", imageID))
	if err := m.store.RemoveFile(imagePath); err != nil {
		return fmt.Errorf("failed to remove image file: %v", err)
	}

	logrus.Infof("Image removed successfully: %s", image.Name)
	return nil
}

func (m *Manager) PullImage(imageName, tag string) (*types.Image, error) {
	logrus.Infof("Pulling image: %s:%s", imageName, tag)

	config := types.ImageConfig{
		Env:        []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		Cmd:        []string{"/bin/sh"},
		WorkingDir: "/",
		Labels: map[string]string{
			"maintainer": "mydocker",
		},
	}

	image, err := m.CreateImage(imageName, tag, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create image during pull: %v", err)
	}

	logrus.Infof("Image pulled successfully: %s", image.ID)
	return image, nil
}

func (m *Manager) BuildImage(options types.ImageBuildOptions) (*types.Image, error) {
	logrus.Infof("Building image with context: %s", options.ContextDir)

	config := types.ImageConfig{
		Env:        []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		Cmd:        []string{"/bin/sh"},
		WorkingDir: "/",
		Labels:     options.Labels,
	}

	tag := "latest"
	if len(options.Tags) > 0 {
		tag = options.Tags[0]
	}

	image, err := m.CreateImage("built-image", tag, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create image during build: %v", err)
	}

	logrus.Infof("Image built successfully: %s", image.ID)
	return image, nil
}

func (m *Manager) TagImage(sourceImageID, targetRepository, targetTag string) error {
	logrus.Infof("Tagging image %s as %s:%s", sourceImageID, targetRepository, targetTag)

	sourceImage, err := m.GetImage(sourceImageID)
	if err != nil {
		return fmt.Errorf("failed to get source image: %v", err)
	}

	newImage := *sourceImage
	newImage.Name = targetRepository
	newImage.Tag = targetTag
	newImage.ID = m.generateImageID(targetRepository, targetTag)

	imagePath := filepath.Join("images", fmt.Sprintf("%s.json", newImage.ID))
	if err := m.store.SaveJSON(imagePath, newImage); err != nil {
		return fmt.Errorf("failed to save tagged image: %v", err)
	}

	logrus.Infof("Image tagged successfully: %s", newImage.ID)
	return nil
}

func (m *Manager) ImageExists(imageID string) bool {
	imagePath := filepath.Join("images", fmt.Sprintf("%s.json", imageID))
	return m.store.FileExists(imagePath)
}

func (m *Manager) GetImageByName(imageName, tag string) (*types.Image, error) {
	images, err := m.ListImages()
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %v", err)
	}

	for _, image := range images {
		if image.Name == imageName && image.Tag == tag {
			return image, nil
		}
	}

	return nil, fmt.Errorf("image not found: %s:%s", imageName, tag)
}

func (m *Manager) generateImageID(name, tag string) string {
	data := fmt.Sprintf("%s:%s:%d", name, tag, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (m *Manager) GetImageDataDir(imageID string) string {
	return filepath.Join(m.store.GetImagesDir(), imageID)
}

func (m *Manager) SaveImageLayers(imageID string, layers []string) error {
	image, err := m.GetImage(imageID)
	if err != nil {
		return fmt.Errorf("failed to get image: %v", err)
	}

	image.Layers = layers

	imagePath := filepath.Join("images", fmt.Sprintf("%s.json", imageID))
	if err := m.store.SaveJSON(imagePath, image); err != nil {
		return fmt.Errorf("failed to save image with layers: %v", err)
	}

	return nil
}

func (m *Manager) GetImageManifest(imageID string) (map[string]interface{}, error) {
	image, err := m.GetImage(imageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %v", err)
	}

	manifest := map[string]interface{}{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.docker.distribution.manifest.v2+json",
		"config": map[string]interface{}{
			"mediaType": "application/vnd.docker.container.image.v1+json",
			"size":      1024,
			"digest":    fmt.Sprintf("sha256:%s", image.ID),
		},
		"layers": []map[string]interface{}{
			{
				"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
				"size":      2048,
				"digest":    "sha256:example-layer-digest",
			},
		},
	}

	return manifest, nil
}