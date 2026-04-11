package infrastructure_test

import (
	"os"
	"testing"

	"github.com/tryuuu/tryuio/internal/domain"
	"github.com/tryuuu/tryuio/internal/infrastructure"
)

func newStorageWithDir(t *testing.T) (*infrastructure.LocalStorage, string) {
	t.Helper()
	dir := t.TempDir()
	return infrastructure.NewLocalStorage(dir), dir
}

func newStorage(t *testing.T) *infrastructure.LocalStorage {
	t.Helper()
	s, _ := newStorageWithDir(t)
	return s
}

func TestPutAndGet(t *testing.T) {
	s := newStorage(t)
	obj := &domain.Object{
		Bucket:      "bucket",
		Key:         "images/test.png",
		ContentType: "image/png",
		Body:        []byte("fakepng"),
	}
	if err := s.Put(obj); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get("bucket", "images/test.png")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got.Body) != "fakepng" {
		t.Errorf("body: got %q, want %q", got.Body, "fakepng")
	}
	if got.ContentType != "image/png" {
		t.Errorf("ContentType: got %q, want %q", got.ContentType, "image/png")
	}
}

func TestGet_NotFound(t *testing.T) {
	s := newStorage(t)
	_, err := s.Get("bucket", "notexist.png")
	if err != infrastructure.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	s := newStorage(t)
	obj := &domain.Object{Bucket: "bucket", Key: "test.txt", Body: []byte("hello")}
	s.Put(obj)

	if err := s.Delete("bucket", "test.txt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := s.Get("bucket", "test.txt")
	if err != infrastructure.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDelete_MetaFileAlsoRemoved(t *testing.T) {
	s, dir := newStorageWithDir(t)
	obj := &domain.Object{Bucket: "bucket", Key: "test.txt", ContentType: "text/plain", Body: []byte("hello")}
	s.Put(obj)

	metaPath := dir + "/bucket/test.txt.meta.json"
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("meta file should exist before Delete: %v", err)
	}

	s.Delete("bucket", "test.txt")

	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		t.Errorf("meta file should be removed after Delete, got err=%v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	s := newStorage(t)
	err := s.Delete("bucket", "notexist.txt")
	if err != infrastructure.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPathTraversal_DotDotInKey(t *testing.T) {
	s := newStorage(t)
	cases := []struct {
		bucket, key string
	}{
		{"bucket", "../../etc/passwd"},
		{"bucket", "../secret.txt"},
		{"../bucket", "key.txt"},
		{"/etc", "passwd"},
		{"bucket", "/etc/passwd"},
	}
	for _, c := range cases {
		obj := &domain.Object{Bucket: c.bucket, Key: c.key, Body: []byte("evil")}
		if err := s.Put(obj); err == nil {
			t.Errorf("Put(%q, %q): expected error, got nil", c.bucket, c.key)
		}
		if _, err := s.Get(c.bucket, c.key); err == nil {
			t.Errorf("Get(%q, %q): expected error, got nil", c.bucket, c.key)
		}
		if err := s.Delete(c.bucket, c.key); err == nil {
			t.Errorf("Delete(%q, %q): expected error, got nil", c.bucket, c.key)
		}
	}
}

func TestContentType_RestoredAfterGet(t *testing.T) {
	s := newStorage(t)
	cases := []struct{ contentType string }{
		{"image/png"},
		{"image/svg+xml"},
		{"application/octet-stream"},
		{""},
	}
	for _, c := range cases {
		key := "test-" + c.contentType + ".bin"
		obj := &domain.Object{Bucket: "b", Key: key, ContentType: c.contentType, Body: []byte("data")}
		s.Put(obj)
		got, _ := s.Get("b", key)
		if got.ContentType != c.contentType {
			t.Errorf("ContentType: got %q, want %q", got.ContentType, c.contentType)
		}
		os.Remove(key) // cleanup
	}
}
