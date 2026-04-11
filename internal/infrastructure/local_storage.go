package infrastructure

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tryuuu/tryuio/internal/domain"
)

var ErrNotFound = errors.New("object not found")
var ErrInvalidPath = errors.New("invalid path")

type objectMeta struct {
	ContentType string `json:"content_type"`
}

type LocalStorage struct {
	dataDir string
}

func NewLocalStorage(dataDir string) *LocalStorage {
	return &LocalStorage{dataDir: dataDir}
}

func (s *LocalStorage) Put(obj *domain.Object) error {
	path, err := s.safeObjectPath(obj.Bucket, obj.Key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	if err := os.WriteFile(path, obj.Body, 0644); err != nil {
		return err
	}
	meta := objectMeta{ContentType: obj.ContentType}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	return os.WriteFile(path+".meta.json", metaBytes, 0644)
}

func (s *LocalStorage) Get(bucket, key string) (*domain.Object, error) {
	path, err := s.safeObjectPath(bucket, key)
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	obj := &domain.Object{
		Bucket: bucket,
		Key:    key,
		Body:   body,
		Size:   int64(len(body)),
	}
	if metaBytes, err := os.ReadFile(path + ".meta.json"); err == nil {
		var meta objectMeta
		if json.Unmarshal(metaBytes, &meta) == nil {
			obj.ContentType = meta.ContentType
		}
	}
	return obj, nil
}

func (s *LocalStorage) Delete(bucket, key string) error {
	path, err := s.safeObjectPath(bucket, key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	// メタファイルは存在しない場合も無視して削除
	os.Remove(path + ".meta.json")
	return nil
}

// safeObjectPath は bucket/key を検証し、dataDir 配下のパスを返す。
// ../ や絶対パスなどのトラバーサルを拒否する。
func (s *LocalStorage) safeObjectPath(bucket, key string) (string, error) {
	if err := validateSegment(bucket); err != nil {
		return "", fmt.Errorf("invalid bucket: %w", err)
	}
	if err := validateSegment(key); err != nil {
		return "", fmt.Errorf("invalid key: %w", err)
	}
	full := filepath.Join(s.dataDir, bucket, key)
	// filepath.Join が .. を解決した結果が dataDir 配下であることを確認
	rel, err := filepath.Rel(s.dataDir, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", ErrInvalidPath
	}
	return full, nil
}

// List は dataDir 配下の全オブジェクトを "bucket/key" 形式で返す。
// .meta.json ファイルは除外する。dataDir が存在しない場合は空リストを返す。
func (s *LocalStorage) List() ([]string, error) {
	if _, err := os.Stat(s.dataDir); os.IsNotExist(err) {
		return []string{}, nil
	}
	var result []string
	err := filepath.Walk(s.dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || strings.HasSuffix(path, ".meta.json") {
			return nil
		}
		rel, err := filepath.Rel(s.dataDir, path)
		if err != nil {
			return err
		}
		result = append(result, rel)
		return nil
	})
	return result, err
}

// validateSegment は bucket または key の各セグメントを検証する。
func validateSegment(s string) error {
	if s == "" {
		return ErrInvalidPath
	}
	// 絶対パスや .. を含む場合は拒否
	if filepath.IsAbs(s) || strings.Contains(s, "..") {
		return ErrInvalidPath
	}
	return nil
}
