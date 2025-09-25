package container

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"docker-impl/pkg/image"
	"docker-impl/pkg/store"
	"docker-impl/pkg/types"
)

func TestNewManager(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	imageMgr := image.NewManager(store)
	manager := NewManager(store, imageMgr)

	assert.NotNil(t, manager, "Manager should not be nil")
	assert.Equal(t, store, manager.store, "Store should be set")
	assert.Equal(t, imageMgr, manager.imageMgr, "Image manager should be set")
	assert.NotNil(t, manager.running, "Running containers map should be initialized")
}

func TestCreateContainer(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	imageMgr := image.NewManager(store)

	// Create a test image first
	imageConfig := types.ImageConfig{
		Env: []string{"PATH=/usr/local/bin"},
		Cmd: []string{"/bin/sh"},
	}
	testImage, err := imageMgr.CreateImage("test-image", "latest", imageConfig)
	require.NoError(t, err)

	manager := NewManager(store, imageMgr)

	config := types.ContainerConfig{
		Image: testImage.ID,
		Env:   []string{"CUSTOM_VAR=value"},
		Cmd:   []string{"/bin/echo", "hello"},
	}

	options := types.ContainerCreateOptions{
		Name:   "test-container",
		Config: config,
	}

	container, err := manager.CreateContainer(options)
	require.NoError(t, err)
	require.NotNil(t, container)

	assert.NotEmpty(t, container.ID, "Container ID should not be empty")
	assert.Equal(t, "test-container", container.Name, "Container name should match")
	assert.Equal(t, testImage.ID, container.Image, "Container image should match")
	assert.Equal(t, types.StatusCreated, container.Status, "Container status should be created")
	assert.True(t, time.Since(container.CreatedAt) < time.Minute, "Created time should be recent")
	assert.Equal(t, config.Env, container.Config.Env, "Environment should match")
	assert.Equal(t, config.Cmd, container.Config.Cmd, "Command should match")
}

func TestCreateContainerWithDefaultName(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	imageMgr := image.NewManager(store)

	imageConfig := types.ImageConfig{
		Env: []string{"PATH=/usr/local/bin"},
		Cmd: []string{"/bin/sh"},
	}
	testImage, err := imageMgr.CreateImage("test-image", "latest", imageConfig)
	require.NoError(t, err)

	manager := NewManager(store, imageMgr)

	options := types.ContainerCreateOptions{
		Config: types.ContainerConfig{
			Image: testImage.ID,
			Cmd:   []string{"/bin/sh"},
		},
	}

	container, err := manager.CreateContainer(options)
	require.NoError(t, err)
	require.NotNil(t, container)

	assert.NotEmpty(t, container.Name, "Container should have a default name")
	assert.Equal(t, container.ID[:12], container.Name, "Default name should be first 12 chars of ID")
}

func TestCreateContainerWithNonexistentImage(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	imageMgr := image.NewManager(store)
	manager := NewManager(store, imageMgr)

	options := types.ContainerCreateOptions{
		Config: types.ContainerConfig{
			Image: "nonexistent-image",
		},
	}

	container, err := manager.CreateContainer(options)
	assert.Error(t, err, "Should return error for nonexistent image")
	assert.Nil(t, container, "Should return nil for nonexistent image")
}

func TestGetContainer(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	imageMgr := image.NewManager(store)

	imageConfig := types.ImageConfig{
		Env: []string{"PATH=/usr/local/bin"},
		Cmd: []string{"/bin/sh"},
	}
	testImage, err := imageMgr.CreateImage("test-image", "latest", imageConfig)
	require.NoError(t, err)

	manager := NewManager(store, imageMgr)

	options := types.ContainerCreateOptions{
		Name: "test-container",
		Config: types.ContainerConfig{
			Image: testImage.ID,
		},
	}

	createdContainer, err := manager.CreateContainer(options)
	require.NoError(t, err)

	retrievedContainer, err := manager.GetContainer(createdContainer.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedContainer)

	assert.Equal(t, createdContainer.ID, retrievedContainer.ID, "Retrieved container should match created container")
	assert.Equal(t, createdContainer.Name, retrievedContainer.Name, "Container names should match")
}

func TestGetNonexistentContainer(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	imageMgr := image.NewManager(store)
	manager := NewManager(store, imageMgr)

	container, err := manager.GetContainer("nonexistent-id")
	assert.Error(t, err, "Should return error for nonexistent container")
	assert.Nil(t, container, "Should return nil for nonexistent container")
}

func TestListContainers(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	imageMgr := image.NewManager(store)

	imageConfig := types.ImageConfig{
		Env: []string{"PATH=/usr/local/bin"},
		Cmd: []string{"/bin/sh"},
	}
	testImage, err := imageMgr.CreateImage("test-image", "latest", imageConfig)
	require.NoError(t, err)

	manager := NewManager(store, imageMgr)

	// Create test containers
	for i := 1; i <= 3; i++ {
		options := types.ContainerCreateOptions{
			Name:   fmt.Sprintf("test-container-%d", i),
			Config: types.ContainerConfig{
				Image: testImage.ID,
				Cmd:   []string{"/bin/sh"},
			},
		}
		_, err := manager.CreateContainer(options)
		require.NoError(t, err)
	}

	containers, err := manager.ListContainers(types.ContainerListOptions{})
	require.NoError(t, err)
	require.Len(t, containers, 3, "Should have 3 containers")

	// Test with All=true
	allContainers, err := manager.ListContainers(types.ContainerListOptions{All: true})
	require.NoError(t, err)
	require.Len(t, allContainers, 3, "Should have 3 containers with All=true")
}

func TestRemoveContainer(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	imageMgr := image.NewManager(store)

	imageConfig := types.ImageConfig{
		Env: []string{"PATH=/usr/local/bin"},
		Cmd: []string{"/bin/sh"},
	}
	testImage, err := imageMgr.CreateImage("test-image", "latest", imageConfig)
	require.NoError(t, err)

	manager := NewManager(store, imageMgr)

	options := types.ContainerCreateOptions{
		Name: "test-container",
		Config: types.ContainerConfig{
			Image: testImage.ID,
		},
	}

	container, err := manager.CreateContainer(options)
	require.NoError(t, err)

	removeOptions := types.ContainerRemoveOptions{}
	err = manager.RemoveContainer(container.ID, removeOptions)
	require.NoError(t, err)

	_, err = manager.GetContainer(container.ID)
	assert.Error(t, err, "Should not be able to get removed container")
}

func TestRemoveNonexistentContainer(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	imageMgr := image.NewManager(store)
	manager := NewManager(store, imageMgr)

	err = manager.RemoveContainer("nonexistent-id", types.ContainerRemoveOptions{})
	assert.Error(t, err, "Should return error for nonexistent container")
}

func TestGetContainerLogs(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	imageMgr := image.NewManager(store)

	imageConfig := types.ImageConfig{
		Env: []string{"PATH=/usr/local/bin"},
		Cmd: []string{"/bin/sh"},
	}
	testImage, err := imageMgr.CreateImage("test-image", "latest", imageConfig)
	require.NoError(t, err)

	manager := NewManager(store, imageMgr)

	options := types.ContainerCreateOptions{
		Name: "test-container",
		Config: types.ContainerConfig{
			Image: testImage.ID,
		},
	}

	container, err := manager.CreateContainer(options)
	require.NoError(t, err)

	logs, err := manager.GetContainerLogs(container.ID)
	require.NoError(t, err)
	assert.Empty(t, logs, "New container should have empty logs")
}

func TestGetContainerLogsNonexistent(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	imageMgr := image.NewManager(store)
	manager := NewManager(store, imageMgr)

	logs, err := manager.GetContainerLogs("nonexistent-id")
	assert.Error(t, err, "Should return error for nonexistent container")
	assert.Empty(t, logs, "Should return empty logs for nonexistent container")
}

func TestGetContainerStats(t *testing.T) {
	tempDir := t.TempDir()
	store, err := store.NewStore(tempDir)
	require.NoError(t, err)

	imageMgr := image.NewManager(store)

	imageConfig := types.ImageConfig{
		Env: []string{"PATH=/usr/local/bin"},
		Cmd: []string{"/bin/sh"},
	}
	testImage, err := imageMgr.CreateImage("test-image", "latest", imageConfig)
	require.NoError(t, err)

	manager := NewManager(store, imageMgr)

	options := types.ContainerCreateOptions{
		Name: "test-container",
		Config: types.ContainerConfig{
			Image: testImage.ID,
		},
	}

	container, err := manager.CreateContainer(options)
	require.NoError(t, err)

	// Container is not running, should get error
	stats, err := manager.GetContainerStats(container.ID)
	assert.Error(t, err, "Should return error for non-running container")
	assert.Nil(t, stats, "Should return nil for non-running container")
}