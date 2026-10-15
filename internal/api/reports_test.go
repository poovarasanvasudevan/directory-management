package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"directory-management/internal/reports"
)

func TestReportsCatalogAndCSV(t *testing.T) {
	handler := NewReportsHandler(reports.MockService{}).Routes()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "locked-users") {
		t.Fatalf("catalog response: %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/locked-users/export", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("export status: %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/csv") {
		t.Fatalf("content type: %s", got)
	}
	if !strings.Contains(recorder.Body.String(), "User,Directory,Status,Observed") {
		t.Fatal("CSV header missing")
	}
}

func TestReportsRejectInvalidIdentifier(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewReportsHandler(reports.MockService{}).Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/invalid.id", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}
