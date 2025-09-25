package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mydockerBinary = "mydocker"
	testImageName   = "integration-test"
	testTagName     = "latest"
)

type TestSuite struct {
	tempDir    string
	dataDir    string
	binaryPath string
}

func setupTestSuite(t *testing.T) *TestSuite {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")

	binaryPath := filepath.Join(tempDir, mydockerBinary)

	// Build the binary
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/mydocker")
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	return &TestSuite{
		tempDir:    tempDir,
		dataDir:    dataDir,
		binaryPath: binaryPath,
	}
}

func (ts *TestSuite) runCommand(t *testing.T, args ...string) (string, error) {
	cmd := exec.Command(ts.binaryPath, args...)
	cmd.Env = append(os.Environ(), "MYDOCKER_DATA_DIR="+ts.dataDir)

	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (ts *TestSuite) cleanup() {
	if ts.tempDir != "" {
		os.RemoveAll(ts.tempDir)
	}
}

func TestImagePull(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	output, err := ts.runCommand("image", "pull", testImageName, "--tag", testTagName)
	require.NoError(t, err, "Failed to pull image: %s", output)

	// Verify image was pulled
	output, err = ts.runCommand("image", "list")
	require.NoError(t, err, "Failed to list images: %s", output)
	assert.Contains(t, output, testImageName, "Pulled image should be in the list")
	assert.Contains(t, output, testTagName, "Pulled tag should be in the list")
}

func TestImageBuild(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// Create a temporary directory with Dockerfile
	dockerfileDir := t.TempDir()
	dockerfilePath := filepath.Join(dockerfileDir, "Dockerfile")
	dockerfileContent := `FROM alpine
RUN echo "test" > /test.txt
`
	err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644)
	require.NoError(t, err, "Failed to create Dockerfile")

	output, err := ts.runCommand("image", "build", "--tag", "test-build:latest", dockerfileDir)
	require.NoError(t, err, "Failed to build image: %s", output)

	// Verify image was built
	output, err = ts.runCommand("image", "list")
	require.NoError(t, err, "Failed to list images: %s", output)
	assert.Contains(t, output, "test-build", "Built image should be in the list")
}

func TestImageRemove(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// First pull an image
	output, err := ts.runCommand("image", "pull", testImageName, "--tag", testTagName)
	require.NoError(t, err, "Failed to pull image: %s", output)

	// Get the image ID
	output, err = ts.runCommand("image", "list")
	require.NoError(t, err, "Failed to list images: %s", output)

	// Remove the image
	output, err = ts.runCommand("image", "remove", testImageName)
	require.NoError(t, err, "Failed to remove image: %s", output)

	// Verify image was removed
	output, err = ts.runCommand("image", "list")
	require.NoError(t, err, "Failed to list images: %s", output)
	assert.NotContains(t, output, testImageName, "Removed image should not be in the list")
}

func TestContainerRun(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// First pull an image
	output, err := ts.runCommand("image", "pull", testImageName, "--tag", testTagName)
	require.NoError(t, err, "Failed to pull image: %s", output)

	// Run a container
	output, err = ts.runCommand("container", "run", testImageName, "echo", "hello world")
	require.NoError(t, err, "Failed to run container: %s", output)

	// List containers
	output, err = ts.runCommand("container", "list", "--all")
	require.NoError(t, err, "Failed to list containers: %s", output)
	assert.Contains(t, output, testImageName, "Container should be in the list")
}

func TestContainerStartStop(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// First pull an image and create a container
	output, err := ts.runCommand("image", "pull", testImageName, "--tag", testTagName)
	require.NoError(t, err, "Failed to pull image: %s", output)

	output, err = ts.runCommand("container", "run", testImageName, "sleep", "10")
	require.NoError(t, err, "Failed to run container: %s", output)

	// Get container ID from list
	output, err = ts.runCommand("container", "list", "--all")
	require.NoError(t, err, "Failed to list containers: %s", output)

	// Stop the container (this will extract container ID properly in real implementation)
	containerID := "test-container-id" // This would be extracted from output in real implementation
	output, err = ts.runCommand("container", "stop", containerID)
	require.NoError(t, err, "Failed to stop container: %s", output)
}

func TestContainerRemove(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// First pull an image and create a container
	output, err := ts.runCommand("image", "pull", testImageName, "--tag", testTagName)
	require.NoError(t, err, "Failed to pull image: %s", output)

	output, err = ts.runCommand("container", "run", testImageName, "echo", "test")
	require.NoError(t, err, "Failed to run container: %s", output)

	// Remove the container
	containerID := "test-container-id" // This would be extracted from output in real implementation
	output, err = ts.runCommand("container", "remove", containerID)
	require.NoError(t, err, "Failed to remove container: %s", output)

	// Verify container was removed
	output, err = ts.runCommand("container", "list", "--all")
	require.NoError(t, err, "Failed to list containers: %s", output)
	assert.NotContains(t, output, containerID, "Removed container should not be in the list")
}

func TestContainerLogs(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// First pull an image and create a container
	output, err := ts.runCommand("image", "pull", testImageName, "--tag", testTagName)
	require.NoError(t, err, "Failed to pull image: %s", output)

	output, err = ts.runCommand("container", "run", testImageName, "echo", "test message")
	require.NoError(t, err, "Failed to run container: %s", output)

	// Get container logs
	containerID := "test-container-id" // This would be extracted from output in real implementation
	output, err = ts.runCommand("container", "logs", containerID)
	require.NoError(t, err, "Failed to get container logs: %s", output)
	assert.Contains(t, output, "test message", "Container logs should contain the test message")
}

func TestSystemInfo(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	output, err := ts.runCommand("system", "info")
	require.NoError(t, err, "Failed to get system info: %s", output)
	assert.Contains(t, output, "version", "System info should contain version")
	assert.Contains(t, output, "data_dir", "System info should contain data_dir")
}

func TestContainerInspect(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// First pull an image and create a container
	output, err := ts.runCommand("image", "pull", testImageName, "--tag", testTagName)
	require.NoError(t, err, "Failed to pull image: %s", output)

	output, err = ts.runCommand("container", "run", testImageName, "echo", "test")
	require.NoError(t, err, "Failed to run container: %s", output)

	// Inspect the container
	containerID := "test-container-id" // This would be extracted from output in real implementation
	output, err = ts.runCommand("container", "inspect", containerID)
	require.NoError(t, err, "Failed to inspect container: %s", output)
	assert.Contains(t, output, containerID, "Container inspect should contain container ID")
	assert.Contains(t, output, "Config", "Container inspect should contain config")
}

func TestImageTag(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// First pull an image
	output, err := ts.runCommand("image", "pull", testImageName, "--tag", testTagName)
	require.NoError(t, err, "Failed to pull image: %s", output)

	// Tag the image with a new name
	output, err = ts.runCommand("image", "tag", testImageName, "new-name:v1")
	require.NoError(t, err, "Failed to tag image: %s", output)

	// Verify the tagged image exists
	output, err = ts.runCommand("image", "list")
	require.NoError(t, err, "Failed to list images: %s", output)
	assert.Contains(t, output, "new-name", "Tagged image should be in the list")
	assert.Contains(t, output, "v1", "Tagged version should be in the list")
}

func TestErrorHandling(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// Test pulling non-existent image
	output, err := ts.runCommand("image", "pull", "non-existent-image")
	assert.Error(t, err, "Should fail to pull non-existent image")
	assert.Contains(t, output, "not found", "Should contain not found error")

	// Test running container with non-existent image
	output, err = ts.runCommand("container", "run", "non-existent-image")
	assert.Error(t, err, "Should fail to run container with non-existent image")
	assert.Contains(t, output, "not found", "Should contain not found error")

	// Test removing non-existent container
	output, err = ts.runCommand("container", "remove", "non-existent-container")
	assert.Error(t, err, "Should fail to remove non-existent container")
}

func TestConcurrentOperations(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// Pull multiple images concurrently
	done := make(chan bool, 3)
	errChan := make(chan error, 3)

	images := []string{"alpine", "ubuntu", "busybox"}

	for _, img := range images {
		go func(imageName string) {
			defer func() { done <- true }()
			output, err := ts.runCommand("image", "pull", imageName)
			if err != nil {
				errChan <- fmt.Errorf("failed to pull %s: %s", imageName, output)
				return
			}
		}(img)
	}

	// Wait for all operations to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Check for errors
	close(errChan)
	for err := range errChan {
		t.Errorf("Concurrent operation error: %v", err)
	}

	// Verify all images were pulled
	output, err := ts.runCommand("image", "list")
	require.NoError(t, err, "Failed to list images: %s", output)

	for _, img := range images {
		assert.Contains(t, output, img, "Image %s should be in the list", img)
	}
}

func TestPersistentData(t *testing.T) {
	ts := setupTestSuite(t)
	defer ts.cleanup()

	// Pull an image
	output, err := ts.runCommand("image", "pull", testImageName, "--tag", testTagName)
	require.NoError(t, err, "Failed to pull image: %s", output)

	// Verify data directory exists
	assert.DirExists(t, ts.dataDir, "Data directory should exist")
	assert.DirExists(t, filepath.Join(ts.dataDir, "images"), "Images directory should exist")
	assert.DirExists(t, filepath.Join(ts.dataDir, "containers"), "Containers directory should exist")

	// Check that image files were created
	imageFiles, err := os.ReadDir(filepath.Join(ts.dataDir, "images"))
	require.NoError(t, err, "Failed to read images directory")
	assert.NotEmpty(t, imageFiles, "Images directory should contain files")
}

func BenchmarkImagePull(b *testing.B) {
	ts := &TestSuite{
		tempDir:    b.TempDir(),
		dataDir:    filepath.Join(b.TempDir(), "data"),
		binaryPath: filepath.Join(b.TempDir(), "mydocker"),
	}

	// Build the binary
	cmd := exec.Command("go", "build", "-o", ts.binaryPath, "./cmd/mydocker")
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	output, err := cmd.CombinedOutput()
	require.NoError(b, err, "Failed to build binary: %s", string(output))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := ts.runCommand(b, "image", "pull", "alpine")
		require.NoError(b, err)
	}
}

func BenchmarkContainerRun(b *testing.B) {
	ts := &TestSuite{
		tempDir:    b.TempDir(),
		dataDir:    filepath.Join(b.TempDir(), "data"),
		binaryPath: filepath.Join(b.TempDir(), "mydocker"),
	}

	// Build the binary
	cmd := exec.Command("go", "build", "-o", ts.binaryPath, "./cmd/mydocker")
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	output, err := cmd.CombinedOutput()
	require.NoError(b, err, "Failed to build binary: %s", string(output))

	// Pull image first
	_, err = ts.runCommand(b, "image", "pull", "alpine")
	require.NoError(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := ts.runCommand(b, "container", "run", "alpine", "echo", "test")
		require.NoError(b, err)
	}
}