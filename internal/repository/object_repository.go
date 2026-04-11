package repository

import "github.com/tryuuu/tryuio/internal/domain"

type ObjectRepository interface {
	Put(obj *domain.Object) error
	Get(bucket, key string) (*domain.Object, error)
	Delete(bucket, key string) error
	// List は dataDir 配下の全オブジェクトを "bucket/key" 形式で返す。
	List() ([]string, error)
}
