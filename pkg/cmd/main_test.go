package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"directory-management/internal/server"
)

func TestRouterServesReactRoutes(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/directories/example", nil)

	server.NewRouter(server.Dependencies{
		RequestTimeout: 30 * time.Second,
	}).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET React route returned %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `<div id="root"></div>`) {
		t.Fatal("GET React route did not return the React app")
	}
}
