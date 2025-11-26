package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Storage interface {
	Get(pluginID, key string) ([]byte, bool, error)
	Set(pluginID, key string, value []byte) error
	Delete(pluginID, key string) error
	Clear(pluginID string) error
	Close() error
}

type FileStorage struct {
	basePath string
	mu       sync.RWMutex
	data     map[string]map[string][]byte
}

func NewFileStorage(basePath string) (*FileStorage, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	s := &FileStorage{basePath: basePath, data: make(map[string]map[string][]byte)}
	if err := s.loadAll(); err != nil {
		return nil, fmt.Errorf("load existing data: %w", err)
	}
	return s, nil
}

func (s *FileStorage) loadAll() error {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginID := entry.Name()
		data, err := os.ReadFile(filepath.Join(s.basePath, pluginID, "data.json"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read data for %s: %w", pluginID, err)
		}

		var pluginData map[string][]byte
		if err := json.Unmarshal(data, &pluginData); err != nil {
			return fmt.Errorf("parse data for %s: %w", pluginID, err)
		}
		s.data[pluginID] = pluginData
	}
	return nil
}

func (s *FileStorage) Get(pluginID, key string) ([]byte, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pluginData, ok := s.data[pluginID]; ok {
		value, exists := pluginData[key]
		return value, exists, nil
	}
	return nil, false, nil
}

func (s *FileStorage) Set(pluginID, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data[pluginID] == nil {
		s.data[pluginID] = make(map[string][]byte)
	}
	s.data[pluginID][key] = value
	return s.persist(pluginID)
}

func (s *FileStorage) Delete(pluginID, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pluginData, ok := s.data[pluginID]; ok {
		delete(pluginData, key)
		return s.persist(pluginID)
	}
	return nil
}

func (s *FileStorage) Clear(pluginID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, pluginID)
	return os.RemoveAll(filepath.Join(s.basePath, pluginID))
}

func (s *FileStorage) persist(pluginID string) error {
	pluginPath := filepath.Join(s.basePath, pluginID)
	if err := os.MkdirAll(pluginPath, 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(s.data[pluginID])
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(pluginPath, "data.json"), data, 0o644)
}

func (s *FileStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for pluginID := range s.data {
		if err := s.persist(pluginID); err != nil {
			return fmt.Errorf("persist %s: %w", pluginID, err)
		}
	}
	return nil
}

type MemoryStorage struct {
	mu   sync.RWMutex
	data map[string]map[string][]byte
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{data: make(map[string]map[string][]byte)}
}

func (s *MemoryStorage) Get(pluginID, key string) ([]byte, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pluginData, ok := s.data[pluginID]; ok {
		value, exists := pluginData[key]
		return value, exists, nil
	}
	return nil, false, nil
}

func (s *MemoryStorage) Set(pluginID, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data[pluginID] == nil {
		s.data[pluginID] = make(map[string][]byte)
	}
	s.data[pluginID][key] = value
	return nil
}

func (s *MemoryStorage) Delete(pluginID, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pluginData, ok := s.data[pluginID]; ok {
		delete(pluginData, key)
	}
	return nil
}

func (s *MemoryStorage) Clear(pluginID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, pluginID)
	return nil
}

func (s *MemoryStorage) Close() error { return nil }
