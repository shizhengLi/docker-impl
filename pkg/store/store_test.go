package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreInitialization(t *testing.T) {
	tempDir := t.TempDir()

	store, err := NewStore(tempDir)
	require.NoError(t, err, "Store should initialize without error")
	require.NotNil(t, store, "Store should not be nil")

	assert.DirExists(t, tempDir, "Data directory should exist")
	assert.DirExists(t, filepath.Join(tempDir, ImagesDir), "Images directory should exist")
	assert.DirExists(t, filepath.Join(tempDir, ContainersDir), "Containers directory should exist")
}

func TestStoreSaveAndLoadJSON(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	require.NoError(t, err)

	type TestData struct {
		Name      string    `json:"name"`
		Value     int       `json:"value"`
		Timestamp time.Time `json:"timestamp"`
	}

	testData := TestData{
		Name:      "test",
		Value:     42,
		Timestamp: time.Now(),
	}

	path := "test/data.json"
	err = store.SaveJSON(path, testData)
	require.NoError(t, err, "Should save JSON without error")

	var loadedData TestData
	err = store.LoadJSON(path, &loadedData)
	require.NoError(t, err, "Should load JSON without error")

	assert.Equal(t, testData.Name, loadedData.Name, "Name should match")
	assert.Equal(t, testData.Value, loadedData.Value, "Value should match")
	assert.Equal(t, testData.Timestamp.Unix(), loadedData.Timestamp.Unix(), "Timestamp should match")
}

func TestStoreFileExists(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	require.NoError(t, err)

	path := "test/exists.json"
	testData := map[string]string{"key": "value"}

	err = store.SaveJSON(path, testData)
	require.NoError(t, err)

	assert.True(t, store.FileExists(path), "File should exist")
	assert.False(t, store.FileExists("test/nonexistent.json"), "Nonexistent file should not exist")
}

func TestStoreRemoveFile(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	require.NoError(t, err)

	path := "test/remove.json"
	testData := map[string]string{"key": "value"}

	err = store.SaveJSON(path, testData)
	require.NoError(t, err)

	assert.True(t, store.FileExists(path), "File should exist before removal")

	err = store.RemoveFile(path)
	require.NoError(t, err)

	assert.False(t, store.FileExists(path), "File should not exist after removal")
}

func TestStoreListFiles(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	require.NoError(t, err)

	files := []string{"test1.json", "test2.json", "test3.json"}
	for _, file := range files {
		testData := map[string]string{"filename": file}
		err := store.SaveJSON(file, testData)
		require.NoError(t, err)
	}

	listedFiles, err := store.ListFiles("")
	require.NoError(t, err)

	assert.Len(t, listedFiles, 3, "Should list exactly 3 files")

	for _, file := range files {
		assert.Contains(t, listedFiles, file, "Should contain file: "+file)
	}
}

func TestStoreGetDataDir(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	require.NoError(t, err)

	assert.Equal(t, tempDir, store.GetDataDir(), "Data directory should match")
	assert.Equal(t, filepath.Join(tempDir, ImagesDir), store.GetImagesDir(), "Images directory should match")
	assert.Equal(t, filepath.Join(tempDir, ContainersDir), store.GetContainersDir(), "Containers directory should match")
}

func TestStoreLoadNonexistentFile(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	require.NoError(t, err)

	var data map[string]string
	err = store.LoadJSON("test/nonexistent.json", &data)
	assert.Error(t, err, "Should return error for nonexistent file")
}

func TestStoreSaveToInvalidPath(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	require.NoError(t, err)

	testData := map[string]string{"key": "value"}
	err = store.SaveJSON("/invalid/path/data.json", testData)
	assert.Error(t, err, "Should return error for invalid path")
}