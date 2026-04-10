package infrastructure

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tryuuu/tryuio/internal/domain"
)

var ErrNotFound = errors.New("object not found")

type LocalStorage struct {
	dataDir string
}

func NewLocalStorage(dataDir string) *LocalStorage {
	return &LocalStorage{dataDir: dataDir}
}

func (s *LocalStorage) Put(obj *domain.Object) error {
	path := s.objectPath(obj.Bucket, obj.Key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	return os.WriteFile(path, obj.Body, 0644)
}

func (s *LocalStorage) Get(bucket, key string) (*domain.Object, error) {
	path := s.objectPath(bucket, key)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &domain.Object{
		Bucket: bucket,
		Key:    key,
		Body:   body,
		Size:   int64(len(body)),
	}, nil
}

func (s *LocalStorage) Delete(bucket, key string) error {
	path := s.objectPath(bucket, key)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *LocalStorage) objectPath(bucket, key string) string {
	return filepath.Join(s.dataDir, bucket, key)
}
