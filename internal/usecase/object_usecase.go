package usecase

import (
	"github.com/tryuuu/tryuio/internal/domain"
	"github.com/tryuuu/tryuio/internal/repository"
)

type ObjectUsecase struct {
	repo repository.ObjectRepository
}

func NewObjectUsecase(repo repository.ObjectRepository) *ObjectUsecase {
	return &ObjectUsecase{repo: repo}
}

func (u *ObjectUsecase) Put(bucket, key, contentType string, body []byte) error {
	obj := &domain.Object{
		Bucket:      bucket,
		Key:         key,
		ContentType: contentType,
		Size:        int64(len(body)),
		Body:        body,
	}
	return u.repo.Put(obj)
}

func (u *ObjectUsecase) Get(bucket, key string) (*domain.Object, error) {
	return u.repo.Get(bucket, key)
}

func (u *ObjectUsecase) Delete(bucket, key string) error {
	return u.repo.Delete(bucket, key)
}
