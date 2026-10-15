package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthEndpoints(t *testing.T) {
	router := NewRouter(Dependencies{RequestTimeout: time.Second})
	for _, path := range []string{"/healthz", "/readyz"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s returned %d", path, recorder.Code)
		}
	}
}

func TestAdminRoutesAreOpen(t *testing.T) {
	protected := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	router := NewRouter(Dependencies{DirectoryAPI: protected, RequestTimeout: time.Second})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/directories", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected open admin route, status = %d", recorder.Code)
	}
}
