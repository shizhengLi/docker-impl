package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"
)

const (
	DefaultDataDir = "/var/lib/mydocker"
	ImagesDir      = "images"
	ContainersDir  = "containers"
)

type Store struct {
	dataDir string
	mu      sync.RWMutex
}

func NewStore(dataDir string) (*Store, error) {
	if dataDir == "" {
		dataDir = DefaultDataDir
	}

	store := &Store{
		dataDir: dataDir,
	}

	if err := store.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize store: %v", err)
	}

	return store, nil
}

func (s *Store) init() error {
	dirs := []string{
		s.dataDir,
		filepath.Join(s.dataDir, ImagesDir),
		filepath.Join(s.dataDir, ContainersDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}

	logrus.Infof("Store initialized with data directory: %s", s.dataDir)
	return nil
}

func (s *Store) SaveJSON(path string, data interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fullPath := filepath.Join(s.dataDir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %v", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %v", err)
	}

	return nil
}

func (s *Store) LoadJSON(path string, data interface{}) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fullPath := filepath.Join(s.dataDir, path)
	file, err := os.Open(fullPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(data); err != nil {
		return fmt.Errorf("failed to decode JSON: %v", err)
	}

	return nil
}

func (s *Store) FileExists(path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fullPath := filepath.Join(s.dataDir, path)
	if _, err := os.Stat(fullPath); err != nil {
		return false
	}
	return true
}

func (s *Store) RemoveFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fullPath := filepath.Join(s.dataDir, path)
	if err := os.RemoveAll(fullPath); err != nil {
		return fmt.Errorf("failed to remove file: %v", err)
	}

	return nil
}

func (s *Store) ListFiles(path string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fullPath := filepath.Join(s.dataDir, path)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %v", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

func (s *Store) GetDataDir() string {
	return s.dataDir
}

func (s *Store) GetImagesDir() string {
	return filepath.Join(s.dataDir, ImagesDir)
}

func (s *Store) GetContainersDir() string {
	return filepath.Join(s.dataDir, ContainersDir)
}