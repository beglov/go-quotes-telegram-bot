package subscribers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const defaultFilePerm = 0o600

// Store предоставляет операции по работе с подписчиками, храня их в JSON-файле.
type Store struct {
	mu          sync.RWMutex
	filePath    string
	subscribers map[int64]struct{}
}

// NewStore создает новое файловое хранилище подписчиков. Если файл отсутствует или поврежден,
// он будет переинициализирован пустым списком.
func NewStore(ctx context.Context, filePath string) (*Store, error) {
	if filePath == "" {
		return nil, errors.New("file path is required")
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	store := &Store{
		filePath:    filePath,
		subscribers: make(map[int64]struct{}),
	}

	if err := store.loadFromFile(); err != nil {
		return nil, err
	}

	return store, nil
}

// Subscribe добавляет chatID в список подписчиков.
func (s *Store) Subscribe(ctx context.Context, chatID int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.subscribers[chatID]; exists {
		return nil
	}

	s.subscribers[chatID] = struct{}{}
	return s.persistLocked()
}

// Unsubscribe удаляет chatID из списка подписчиков.
func (s *Store) Unsubscribe(ctx context.Context, chatID int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.subscribers[chatID]; !exists {
		return nil
	}

	delete(s.subscribers, chatID)
	return s.persistLocked()
}

// List возвращает отсортированный список chatID всех подписчиков.
func (s *Store) List(ctx context.Context) ([]int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]int64, 0, len(s.subscribers))
	for id := range s.subscribers {
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	return ids, nil
}

func (s *Store) loadFromFile() error {
	data, err := os.ReadFile(s.filePath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return s.writeFile([]int64{})
	case err != nil:
		return fmt.Errorf("read subscribers file: %w", err)
	}

	if len(data) == 0 {
		return s.writeFile([]int64{})
	}

	var ids []int64
	if err := json.Unmarshal(data, &ids); err != nil {
		// Файл поврежден — переинициализируем его пустым списком.
		if writeErr := s.writeFile([]int64{}); writeErr != nil {
			return fmt.Errorf("reset corrupted subscribers file: %v (initial error: %w)", writeErr, err)
		}
		return nil
	}

	for _, id := range ids {
		s.subscribers[id] = struct{}{}
	}

	return nil
}

func (s *Store) persistLocked() error {
	ids := make([]int64, 0, len(s.subscribers))
	for id := range s.subscribers {
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	return s.writeFile(ids)
}

func (s *Store) writeFile(ids []int64) error {
	data, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal subscribers: %w", err)
	}

	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, defaultFilePerm); err != nil {
		return fmt.Errorf("write temp subscribers file: %w", err)
	}

	if err := os.Rename(tmpFile, s.filePath); err != nil {
		return fmt.Errorf("replace subscribers file: %w", err)
	}

	return nil
}
