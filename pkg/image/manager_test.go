package image

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"docker-impl/pkg/store"
	"docker-impl/pkg/types"
)

func TestNewManager(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)
	assert.NotNil(t, manager, "Manager should not be nil")
	assert.Equal(t, store, manager.store, "Store should be set")
}

func TestCreateImage(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	config := types.ImageConfig{
		Env:        []string{"PATH=/usr/local/bin"},
		Cmd:        []string{"/bin/sh"},
		WorkingDir: "/",
		Labels: map[string]string{
			"test": "label",
		},
	}

	image, err := manager.CreateImage("test-image", "latest", config)
	require.NoError(t, err)
	require.NotNil(t, image)

	assert.NotEmpty(t, image.ID, "Image ID should not be empty")
	assert.Equal(t, "test-image", image.Name, "Image name should match")
	assert.Equal(t, "latest", image.Tag, "Image tag should match")
	assert.Equal(t, config, image.Config, "Config should match")
	assert.True(t, time.Since(image.CreatedAt) < time.Minute, "Created time should be recent")
	assert.Len(t, image.Layers, 1, "Should have one layer")
	assert.Equal(t, config.Labels, image.Labels, "Labels should match")
}

func TestGetImage(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	config := types.ImageConfig{
		Env: []string{"PATH=/usr/local/bin"},
		Cmd: []string{"/bin/sh"},
	}

	createdImage, err := manager.CreateImage("test-image", "latest", config)
	require.NoError(t, err)

	retrievedImage, err := manager.GetImage(createdImage.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedImage)

	assert.Equal(t, createdImage.ID, retrievedImage.ID, "Retrieved image should match created image")
	assert.Equal(t, createdImage.Name, retrievedImage.Name, "Retrieved image name should match")
	assert.Equal(t, createdImage.Tag, retrievedImage.Tag, "Retrieved image tag should match")
}

func TestGetNonexistentImage(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	image, err := manager.GetImage("nonexistent-id")
	assert.Error(t, err, "Should return error for nonexistent image")
	assert.Nil(t, image, "Should return nil for nonexistent image")
}

func TestListImages(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	config1 := types.ImageConfig{Env: []string{"PATH=/bin"}}
	config2 := types.ImageConfig{Env: []string{"PATH=/usr/bin"}}

	image1, err := manager.CreateImage("test1", "latest", config1)
	require.NoError(t, err)

	image2, err := manager.CreateImage("test2", "v1", config2)
	require.NoError(t, err)

	images, err := manager.ListImages()
	require.NoError(t, err)
	require.Len(t, images, 2, "Should have 2 images")

	var image1Found, image2Found bool
	for _, img := range images {
		if img.ID == image1.ID {
			image1Found = true
		}
		if img.ID == image2.ID {
			image2Found = true
		}
	}

	assert.True(t, image1Found, "Should find image1")
	assert.True(t, image2Found, "Should find image2")
}

func TestRemoveImage(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	config := types.ImageConfig{Env: []string{"PATH=/bin"}}
	image, err := manager.CreateImage("test-image", "latest", config)
	require.NoError(t, err)

	err = manager.RemoveImage(image.ID)
	require.NoError(t, err)

	assert.False(t, manager.ImageExists(image.ID), "Image should not exist after removal")

	_, err = manager.GetImage(image.ID)
	assert.Error(t, err, "Should not be able to get removed image")
}

func TestRemoveNonexistentImage(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	err = manager.RemoveImage("nonexistent-id")
	assert.Error(t, err, "Should return error for nonexistent image")
}

func TestPullImage(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	image, err := manager.PullImage("alpine", "latest")
	require.NoError(t, err)
	require.NotNil(t, image)

	assert.Equal(t, "alpine", image.Name, "Image name should be alpine")
	assert.Equal(t, "latest", image.Tag, "Image tag should be latest")
	assert.Contains(t, image.Config.Env, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "Should have default PATH")
}

func TestTagImage(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	config := types.ImageConfig{Env: []string{"PATH=/bin"}}
	sourceImage, err := manager.CreateImage("source", "latest", config)
	require.NoError(t, err)

	err = manager.TagImage(sourceImage.ID, "target", "v1")
	require.NoError(t, err)

	targetImage, err := manager.GetImageByName("target", "v1")
	require.NoError(t, err)
	require.NotNil(t, targetImage)

	assert.Equal(t, "target", targetImage.Name, "Target image name should be correct")
	assert.Equal(t, "v1", targetImage.Tag, "Target image tag should be correct")
}

func TestGetImageByName(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	config := types.ImageConfig{Env: []string{"PATH=/bin"}}
	_, err = manager.CreateImage("test-image", "latest", config)
	require.NoError(t, err)

	image, err := manager.GetImageByName("test-image", "latest")
	require.NoError(t, err)
	require.NotNil(t, image)

	assert.Equal(t, "test-image", image.Name, "Image name should match")
	assert.Equal(t, "latest", image.Tag, "Image tag should match")
}

func TestGetImageByNameNotFound(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	image, err := manager.GetImageByName("nonexistent", "latest")
	assert.Error(t, err, "Should return error for nonexistent image")
	assert.Nil(t, image, "Should return nil for nonexistent image")
}

func TestBuildImage(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	options := types.ImageBuildOptions{
		ContextDir: "/tmp",
		Dockerfile: "Dockerfile",
		Tags:       []string{"test-build:latest"},
		Labels: map[string]string{
			"build": "test",
		},
	}

	image, err := manager.BuildImage(options)
	require.NoError(t, err)
	require.NotNil(t, image)

	assert.Equal(t, "built-image", image.Name, "Image name should be built-image")
	assert.Equal(t, "latest", image.Tag, "Image tag should be latest")
	assert.Equal(t, "test", image.Labels["build"], "Build label should be set")
}

func TestImageExists(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	assert.False(t, manager.ImageExists("nonexistent"), "Nonexistent image should not exist")

	config := types.ImageConfig{Env: []string{"PATH=/bin"}}
	image, err := manager.CreateImage("test", "latest", config)
	require.NoError(t, err)

	assert.True(t, manager.ImageExists(image.ID), "Created image should exist")
}

func TestSaveImageLayers(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	config := types.ImageConfig{Env: []string{"PATH=/bin"}}
	image, err := manager.CreateImage("test", "latest", config)
	require.NoError(t, err)

	layers := []string{"layer1", "layer2", "layer3"}
	err = manager.SaveImageLayers(image.ID, layers)
	require.NoError(t, err)

	retrievedImage, err := manager.GetImage(image.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedImage)

	assert.Equal(t, layers, retrievedImage.Layers, "Image layers should match saved layers")
}

func TestGetImageManifest(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	manager := NewManager(store)

	config := types.ImageConfig{Env: []string{"PATH=/bin"}}
	image, err := manager.CreateImage("test", "latest", config)
	require.NoError(t, err)

	manifest, err := manager.GetImageManifest(image.ID)
	require.NoError(t, err)
	require.NotNil(t, manifest)

	assert.Equal(t, 2.0, manifest["schemaVersion"], "Schema version should be 2")
	assert.Contains(t, manifest, "config", "Manifest should contain config")
	assert.Contains(t, manifest, "layers", "Manifest should contain layers")
}