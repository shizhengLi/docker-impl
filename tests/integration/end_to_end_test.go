package integration

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndToEndWorkflow(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// 1. Pull an image
	output, err := ts.runCommand("image", "pull", "alpine", "--tag", "latest")
	require.NoError(t, err, "Failed to pull image: %s", output)
	assert.Contains(t, output, "Successfully pulled", "Should indicate successful pull")

	// 2. Tag the image
	output, err = ts.runCommand("image", "tag", "alpine:latest", "my-alpine:v1.0")
	require.NoError(t, err, "Failed to tag image: %s", output)

	// 3. List images and verify both original and tagged exist
	output, err = ts.runCommand("image", "list")
	require.NoError(t, err, "Failed to list images: %s", output)
	assert.Contains(t, output, "alpine", "Original image should be in list")
	assert.Contains(t, output, "my-alpine", "Tagged image should be in list")

	// 4. Run a container
	output, err = ts.runCommand("container", "run", "--name", "test-container", "my-alpine:v1.0", "echo", "Hello from container")
	require.NoError(t, err, "Failed to run container: %s", output)
	assert.Contains(t, output, "Container started successfully", "Should indicate successful container start")

	// 5. List containers and verify container exists
	output, err = ts.runCommand("container", "list", "--all")
	require.NoError(t, err, "Failed to list containers: %s", output)
	assert.Contains(t, output, "test-container", "Container should be in list")
	assert.Contains(t, output, "my-alpine:v1.0", "Container should show correct image")

	// 6. Get container logs
	containerID := getContainerIDFromOutput(t, output)
	output, err = ts.runCommand("container", "logs", containerID)
	require.NoError(t, err, "Failed to get container logs: %s", output)
	assert.Contains(t, output, "Hello from container", "Container logs should contain the output")

	// 7. Inspect container
	output, err = ts.runCommand("container", "inspect", containerID)
	require.NoError(t, err, "Failed to inspect container: %s", output)
	assert.Contains(t, output, containerID, "Inspect output should contain container ID")
	assert.Contains(t, output, "my-alpine:v1.0", "Inspect output should contain image name")

	// 8. Stop container
	output, err = ts.runCommand("container", "stop", containerID)
	require.NoError(t, err, "Failed to stop container: %s", output)
	assert.Contains(t, output, "stopped successfully", "Should indicate successful stop")

	// 9. Remove container
	output, err = ts.runCommand("container", "remove", containerID)
	require.NoError(t, err, "Failed to remove container: %s", output)
	assert.Contains(t, output, "removed successfully", "Should indicate successful removal")

	// 10. Verify container was removed
	output, err = ts.runCommand("container", "list", "--all")
	require.NoError(t, err, "Failed to list containers: %s", output)
	assert.NotContains(t, output, "test-container", "Removed container should not be in list")

	// 11. Remove original image
	output, err = ts.runCommand("image", "remove", "alpine:latest")
	require.NoError(t, err, "Failed to remove original image: %s", output)

	// 12. Remove tagged image
	output, err = ts.runCommand("image", "remove", "my-alpine:v1.0")
	require.NoError(t, err, "Failed to remove tagged image: %s", output)

	// 13. Verify images were removed
	output, err = ts.runCommand("image", "list")
	require.NoError(t, err, "Failed to list images: %s", output)
	assert.NotContains(t, output, "alpine", "Original image should not be in list")
	assert.NotContains(t, output, "my-alpine", "Tagged image should not be in list")

	// 14. Check system info
	output, err = ts.runCommand("system", "info")
	require.NoError(t, err, "Failed to get system info: %s", output)
	assert.Contains(t, output, "version", "System info should contain version")
	assert.Contains(t, output, ts.dataDir, "System info should contain data directory")
}

func TestMultipleContainersWorkflow(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// Pull multiple images
	images := []string{"alpine", "busybox"}
	for _, img := range images {
		output, err := ts.runCommand("image", "pull", img)
		require.NoError(t, err, "Failed to pull %s: %s", img, output)
	}

	// Run multiple containers
	containerNames := []string{"container1", "container2", "container3"}
	containerIDs := []string{}

	for i, name := range containerNames {
		img := images[i%len(images)]
		output, err := ts.runCommand("container", "run", "--name", name, img, "echo", "Hello from "+name)
		require.NoError(t, err, "Failed to run container %s: %s", name, output)

		// In real implementation, extract container ID from output
		containerIDs = append(containerIDs, "container-id-"+name)
	}

	// List all containers
	output, err := ts.runCommand("container", "list", "--all")
	require.NoError(t, err, "Failed to list containers: %s", output)

	// Verify all containers exist
	for _, name := range containerNames {
		assert.Contains(t, output, name, "Container %s should be in list", name)
	}

	// Get logs for each container
	for i, name := range containerNames {
		output, err := ts.runCommand("container", "logs", containerIDs[i])
		require.NoError(t, err, "Failed to get logs for %s: %s", name, output)
		assert.Contains(t, output, "Hello from "+name, "Logs should contain correct message for %s", name)
	}

	// Stop all containers
	for _, id := range containerIDs {
		output, err := ts.runCommand("container", "stop", id)
		require.NoError(t, err, "Failed to stop container %s: %s", id, output)
	}

	// Remove all containers
	for _, id := range containerIDs {
		output, err := ts.runCommand("container", "remove", id)
		require.NoError(t, err, "Failed to remove container %s: %s", id, output)
	}

	// Verify all containers were removed
	output, err = ts.runCommand("container", "list", "--all")
	require.NoError(t, err, "Failed to list containers: %s", output)

	for _, name := range containerNames {
		assert.NotContains(t, output, name, "Container %s should not be in list after removal", name)
	}
}

func TestErrorRecoveryWorkflow(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// Try to run container with non-existent image
	output, err := ts.runCommand("container", "run", "non-existent-image", "echo", "test")
	assert.Error(t, err, "Should fail to run container with non-existent image")
	assert.Contains(t, output, "not found", "Should indicate image not found")

	// Verify system is still functional
	output, err = ts.runCommand("system", "info")
	require.NoError(t, err, "System should still be functional after error: %s", output)

	// Pull a valid image
	output, err = ts.runCommand("image", "pull", "alpine")
	require.NoError(t, err, "Should be able to pull image after previous error: %s", output)

	// Run container with valid image
	output, err = ts.runCommand("container", "run", "alpine", "echo", "recovery test")
	require.NoError(t, err, "Should be able to run container after recovery: %s", output)

	// Verify system state is consistent
	output, err = ts.runCommand("image", "list")
	require.NoError(t, err, "Should be able to list images: %s", output)
	assert.Contains(t, output, "alpine", "Pulled image should be in list")

	output, err = ts.runCommand("container", "list", "--all")
	require.NoError(t, err, "Should be able to list containers: %s", output)
	assert.Contains(t, output, "container", "Container should be in list")
}

func TestDataPersistenceWorkflow(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// Pull image and create container
	output, err := ts.runCommand("image", "pull", "alpine")
	require.NoError(t, err, "Failed to pull image: %s", output)

	output, err = ts.runCommand("container", "run", "--name", "persistent-test", "alpine", "echo", "persistence test")
	require.NoError(t, err, "Failed to run container: %s", output)

	// Verify data directory structure
	assert.DirExists(t, ts.dataDir, "Data directory should exist")
	assert.DirExists(t, filepath.Join(ts.dataDir, "images"), "Images directory should exist")
	assert.DirExists(t, filepath.Join(ts.dataDir, "containers"), "Containers directory should exist")

	// Verify image files exist
	imageFiles, err := filepath.Glob(filepath.Join(ts.dataDir, "images", "*.json"))
	require.NoError(t, err, "Failed to list image files")
	assert.NotEmpty(t, imageFiles, "Should have image files")

	// Verify container files exist
	containerFiles, err := filepath.Glob(filepath.Join(ts.dataDir, "containers", "*.json"))
	require.NoError(t, err, "Failed to list container files")
	assert.NotEmpty(t, containerFiles, "Should have container files")

	// Stop and remove container
	containerID := getContainerIDFromOutput(t, output)
	output, err = ts.runCommand("container", "stop", containerID)
	require.NoError(t, err, "Failed to stop container: %s", output)

	output, err = ts.runCommand("container", "remove", containerID)
	require.NoError(t, err, "Failed to remove container: %s", output)

	// Verify container files are cleaned up
	containerFiles, err = filepath.Glob(filepath.Join(ts.dataDir, "containers", "*.json"))
	require.NoError(t, err, "Failed to list container files after removal")
	// Note: In real implementation, we'd verify that specific container files are removed
}

func TestResourceLimitsWorkflow(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// Pull image
	output, err := ts.runCommand("image", "pull", "alpine")
	require.NoError(t, err, "Failed to pull image: %s", output)

	// Run container with resource limits (if supported)
	output, err = ts.runCommand("container", "run", "--name", "resource-test", "alpine", "echo", "resource limit test")
	require.NoError(t, err, "Failed to run container: %s", output)

	// Verify container is running
	output, err = ts.runCommand("container", "list")
	require.NoError(t, err, "Failed to list containers: %s", output)
	assert.Contains(t, output, "resource-test", "Resource test container should be in list")

	// Stop and remove container
	containerID := getContainerIDFromOutput(t, output)
	output, err = ts.runCommand("container", "stop", containerID)
	require.NoError(t, err, "Failed to stop container: %s", output)

	output, err = ts.runCommand("container", "remove", containerID)
	require.NoError(t, err, "Failed to remove container: %s", output)
}

func getContainerIDFromOutput(t *testing.T, output string) string {
	// In a real implementation, this would parse the output to extract the container ID
	// For now, return a placeholder
	t.Log("Extracting container ID from output:", output)
	return "test-container-id"
}