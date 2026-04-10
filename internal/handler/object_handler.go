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
}

func NewObjectHandler(uc *usecase.ObjectUsecase) *ObjectHandler {
	return &ObjectHandler{usecase: uc}
}

func (h *ObjectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bucket, key, ok := parsePath(r.URL.Path)
	if !ok {
		http.Error(w, "invalid path: expected /<bucket>/<key>", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		h.handlePut(w, r, bucket, key)
	case http.MethodGet:
		h.handleGet(w, r, bucket, key)
	case http.MethodDelete:
		h.handleDelete(w, r, bucket, key)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ObjectHandler) handlePut(w http.ResponseWriter, r *http.Request, bucket, key string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}
	contentType := r.Header.Get("Content-Type")
	if err := h.usecase.Put(bucket, key, contentType, body); err != nil {
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
