package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"taskmgr/internal/task"
)

var ErrNotFound = errors.New("task not found")

type Storage interface {
	Load(ctx context.Context) (task.List, error)
	Save(ctx context.Context, tasks task.List) error
}

type JSONStorage struct {
	path string
}

func NewJSONStorage(path string) *JSONStorage {
	return &JSONStorage{path: path}
}

func (s *JSONStorage) Load(ctx context.Context) (task.List, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return task.List{}, nil
		}
		return nil, fmt.Errorf("read storage: %w", err)
	}
	var tasks task.List
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("parse storage: %w", err)
	}
	return tasks, nil
}

func (s *JSONStorage) Save(ctx context.Context, tasks task.List) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
