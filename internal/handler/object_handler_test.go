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

const testAPIKey = "test-secret-key"

func newHandler(t *testing.T) http.Handler {
	t.Helper()
	storage := infrastructure.NewLocalStorage(t.TempDir())
	uc := usecase.NewObjectUsecase(storage)
	return handler.NewObjectHandler(uc, testAPIKey)
}

func authPUT(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func authDELETE(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPUT_GET_DELETE(t *testing.T) {
	h := newHandler(t)

	// PUT
	if rec := authPUT(t, h, "/bucket/images/test.png", "fakepng"); rec.Code != http.StatusOK {
		t.Fatalf("PUT: got %d, want 200", rec.Code)
	}

	// GET
	req := httptest.NewRequest(http.MethodGet, "/bucket/images/test.png", nil)
	rec := httptest.NewRecorder()
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
	if rec := authDELETE(t, h, "/bucket/images/test.png"); rec.Code != http.StatusNoContent {
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

func TestPUT_NoAPIKey_Returns401(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodPut, "/bucket/test.png", strings.NewReader("data"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", rec.Code)
	}
}

func TestPUT_BearerCaseInsensitive_Returns200(t *testing.T) {
	h := newHandler(t)
	cases := []string{
		"Bearer " + testAPIKey,
		"bearer " + testAPIKey,
		"BEARER " + testAPIKey,
	}
	for _, auth := range cases {
		req := httptest.NewRequest(http.MethodPut, "/bucket/test.png", strings.NewReader("data"))
		req.Header.Set("Authorization", auth)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("Authorization %q: got %d, want 200", auth, rec.Code)
		}
	}
}

func TestPUT_WrongAPIKey_Returns401(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodPut, "/bucket/test.png", strings.NewReader("data"))
	req.Header.Set("Authorization", "Bearer wrongkey")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", rec.Code)
	}
}

func TestDELETE_NoAPIKey_Returns401(t *testing.T) {
	h := newHandler(t)
	// まず PUT で作成
	authPUT(t, h, "/bucket/test.png", "data")

	req := httptest.NewRequest(http.MethodDelete, "/bucket/test.png", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", rec.Code)
	}
}

func TestGET_NoAPIKey_Returns200(t *testing.T) {
	h := newHandler(t)
	authPUT(t, h, "/bucket/test.png", "data")

	// GET は認証不要
	req := httptest.NewRequest(http.MethodGet, "/bucket/test.png", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got %d, want 200", rec.Code)
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
	if rec := authDELETE(t, h, "/bucket/notexist.png"); rec.Code != http.StatusNotFound {
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
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
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
