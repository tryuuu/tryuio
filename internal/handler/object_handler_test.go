package handler_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tryuuu/tryuio/internal/handler"
	"github.com/tryuuu/tryuio/internal/infrastructure"
	"github.com/tryuuu/tryuio/internal/usecase"
)

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	storage := infrastructure.NewLocalStorage(t.TempDir())
	uc := usecase.NewObjectUsecase(storage)
	return handler.NewObjectHandler(uc)
}

func TestPUT_GET_DELETE(t *testing.T) {
	h := newHandler(t)

	// PUT
	req := httptest.NewRequest(http.MethodPut, "/bucket/images/test.png",
		strings.NewReader("fakepng"))
	req.Header.Set("Content-Type", "image/png")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: got %d, want 200", rec.Code)
	}

	// GET
	req = httptest.NewRequest(http.MethodGet, "/bucket/images/test.png", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET: got %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type: got %q, want %q", ct, "image/png")
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != "fakepng" {
		t.Errorf("body: got %q, want %q", string(body), "fakepng")
	}

	// DELETE
	req = httptest.NewRequest(http.MethodDelete, "/bucket/images/test.png", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: got %d, want 204", rec.Code)
	}

	// GET after DELETE → 404
	req = httptest.NewRequest(http.MethodGet, "/bucket/images/test.png", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET after DELETE: got %d, want 404", rec.Code)
	}
}

func TestGET_NotFound(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/bucket/notexist.png", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestDELETE_NotFound(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/bucket/notexist.png", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestInvalidPath_MissingKey(t *testing.T) {
	h := newHandler(t)
	cases := []string{"/", "/bucket", "//key"}
	for _, path := range cases {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %q: got %d, want 400", path, rec.Code)
		}
	}
}

func TestPathTraversal_Returns400(t *testing.T) {
	h := newHandler(t)
	cases := []string{
		"/bucket/../../etc/passwd",
		"/bucket/../secret.txt",
		"/../bucket/key",
	}
	for _, path := range cases {
		req := httptest.NewRequest(http.MethodPut, path, strings.NewReader("evil"))
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("PUT %q: got %d, want 400", path, rec.Code)
		}
	}
}

func TestMethodNotAllowed(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/bucket/key.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", rec.Code)
	}
}
