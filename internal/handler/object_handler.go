package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/tryuuu/tryuio/internal/domain"
	"github.com/tryuuu/tryuio/internal/infrastructure"
	"github.com/tryuuu/tryuio/internal/replication"
	"github.com/tryuuu/tryuio/internal/usecase"
)

type ObjectHandler struct {
	usecase    *usecase.ObjectUsecase
	apiKey     string
	replicator *replication.Replicator // nil のときはレプリケーションなし
}

func NewObjectHandler(uc *usecase.ObjectUsecase, apiKey string, replicator *replication.Replicator) *ObjectHandler {
	return &ObjectHandler{usecase: uc, apiKey: apiKey, replicator: replicator}
}

func (h *ObjectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/health":
		w.WriteHeader(http.StatusOK)
		return
	case "/list":
		if !h.authorized(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.handleList(w, r)
		return
	}

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
	// X-Replicated ヘッダーがある場合はループを防ぐため複製しない
	if h.replicator != nil && r.Header.Get("X-Replicated") == "" {
		h.replicator.ReplicatePut(&domain.Object{
			Bucket:      bucket,
			Key:         key,
			ContentType: contentType,
			Body:        body,
		})
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
	if h.replicator != nil && r.Header.Get("X-Replicated") == "" {
		h.replicator.ReplicateDelete(bucket, key)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ObjectHandler) handleList(w http.ResponseWriter, r *http.Request) {
	list, err := h.usecase.List()
	if err != nil {
		http.Error(w, "failed to list objects", http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []string{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// authorized は Authorization: Bearer <key> ヘッダーを検証する。
// Bearer スキームは HTTP 仕様に従い大文字小文字を問わない。
func (h *ObjectHandler) authorized(r *http.Request) bool {
	token, ok := strings.CutPrefix(strings.ToLower(r.Header.Get("Authorization")), "bearer ")
	return ok && token == h.apiKey
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
