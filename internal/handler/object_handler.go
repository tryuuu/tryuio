package handler

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/tryuuu/tryuio/internal/infrastructure"
	"github.com/tryuuu/tryuio/internal/usecase"
)

type ObjectHandler struct {
	usecase *usecase.ObjectUsecase
	apiKey  string
}

func NewObjectHandler(uc *usecase.ObjectUsecase, apiKey string) *ObjectHandler {
	return &ObjectHandler{usecase: uc, apiKey: apiKey}
}

func (h *ObjectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bucket, key, ok := parsePath(r.URL.Path)
	if !ok {
		http.Error(w, "invalid path: expected /<bucket>/<key>", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		if !h.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.handlePut(w, r, bucket, key)
	case http.MethodGet:
		h.handleGet(w, r, bucket, key)
	case http.MethodDelete:
		if !h.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.handleDelete(w, r, bucket, key)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// authorized は Authorization: Bearer <key> ヘッダーを検証する。
// Bearer スキームは HTTP 仕様に従い大文字小文字を問わない。
func (h *ObjectHandler) authorized(r *http.Request) bool {
	token, ok := strings.CutPrefix(strings.ToLower(r.Header.Get("Authorization")), "bearer ")
	return ok && token == h.apiKey
}

func (h *ObjectHandler) handlePut(w http.ResponseWriter, r *http.Request, bucket, key string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}
	contentType := r.Header.Get("Content-Type")
	if err := h.usecase.Put(bucket, key, contentType, body); err != nil {
		if errors.Is(err, infrastructure.ErrInvalidPath) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to store object", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ObjectHandler) handleGet(w http.ResponseWriter, r *http.Request, bucket, key string) {
	obj, err := h.usecase.Get(bucket, key)
	if err != nil {
		if errors.Is(err, infrastructure.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, infrastructure.ErrInvalidPath) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to get object", http.StatusInternalServerError)
		return
	}
	if obj.ContentType != "" {
		w.Header().Set("Content-Type", obj.ContentType)
	}
	w.WriteHeader(http.StatusOK)
	w.Write(obj.Body)
}

func (h *ObjectHandler) handleDelete(w http.ResponseWriter, r *http.Request, bucket, key string) {
	if err := h.usecase.Delete(bucket, key); err != nil {
		if errors.Is(err, infrastructure.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, infrastructure.ErrInvalidPath) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to delete object", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parsePath は /<bucket>/<key> 形式のパスを分割する。
// key はスラッシュを含むことができる（例: images/foo.png）。
func parsePath(path string) (bucket, key string, ok bool) {
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
