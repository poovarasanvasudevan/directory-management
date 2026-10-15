package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"directory-management/internal/configstore"
)

func TestSettingsRoundTrip(t *testing.T) {
	handler := NewSettingsHandler(configstore.NewMemoryStore()).Routes()
	put := httptest.NewRecorder()
	handler.ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/login", strings.NewReader(`{"settings":{"showWebDirectory":false,"requireCaptcha":true}}`)))
	if put.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", put.Code, put.Body.String())
	}
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/login", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"requireCaptcha":true`) {
		t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
	}
}
