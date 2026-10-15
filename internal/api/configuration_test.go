package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"directory-management/internal/audit"
	"directory-management/internal/configstore"
)

func TestConfigurationCatalogAndRoundTrip(t *testing.T) {
	handler := NewConfigurationHandler(configstore.NewMemoryStore()).Routes()
	catalog := httptest.NewRecorder()
	handler.ServeHTTP(catalog, httptest.NewRequest(http.MethodGet, "/", nil))
	if catalog.Code != http.StatusOK || !strings.Contains(catalog.Body.String(), `"multi-factor-authentication"`) || !strings.Contains(catalog.Body.String(), `"anti-hacking"`) {
		t.Fatalf("catalog status=%d body=%s", catalog.Code, catalog.Body.String())
	}
	for _, id := range []string{"mail-sms", "personalization", "resilience", "privacy-licensing", "login-settings"} {
		if !strings.Contains(catalog.Body.String(), `"`+id+`"`) {
			t.Fatalf("catalog missing %s", id)
		}
	}

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/password-policy", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"minimumLength":14`) {
		t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
	}

	var current struct {
		Values map[string]any `json:"values"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	current.Values["minimumLength"] = float64(18)
	payload, err := json.Marshal(map[string]any{"values": current.Values})
	if err != nil {
		t.Fatal(err)
	}
	put := httptest.NewRecorder()
	handler.ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/password-policy", strings.NewReader(string(payload))))
	if put.Code != http.StatusOK || !strings.Contains(put.Body.String(), `"minimumLength":18`) {
		t.Fatalf("put status=%d body=%s", put.Code, put.Body.String())
	}
}

func TestConfigurationCoversCapabilityCatalog(t *testing.T) {
	definitions := configurationDefinitions()
	for _, capability := range capabilityCatalog {
		if capability.ID == "directory-connection" {
			continue
		}
		if _, ok := definitions[capability.ID]; !ok {
			t.Fatalf("capability %s has no configuration definition", capability.ID)
		}
	}
}

func TestConfigurationRejectsUnknownAndInvalidFields(t *testing.T) {
	handler := NewConfigurationHandler(configstore.NewMemoryStore()).Routes()
	tests := []string{
		`{"values":{"unknown":true}}`,
		`{"values":{"minimumLength":2}}`,
		`{"values":{"enabled":"yes"}}`,
	}
	for _, body := range tests {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/password-policy", strings.NewReader(body)))
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("body=%s status=%d response=%s", body, recorder.Code, recorder.Body.String())
		}
	}
}

func TestMultiFactorAuthenticationRoundTrip(t *testing.T) {
	handler := NewConfigurationHandler(configstore.NewMemoryStore()).Routes()
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/multi-factor-authentication", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), `"Authenticator setup"`) {
		t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
	}
	var current struct {
		Values map[string]any `json:"values"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	current.Values["duo"] = true
	current.Values["idleMinutes"] = float64(8)
	payload, _ := json.Marshal(map[string]any{"values": current.Values})
	put := httptest.NewRecorder()
	handler.ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/multi-factor-authentication", strings.NewReader(string(payload))))
	if put.Code != http.StatusOK || !strings.Contains(put.Body.String(), `"duo":true`) {
		t.Fatalf("put status=%d body=%s", put.Code, put.Body.String())
	}
}

func TestConfigurationSaveCreatesRevisionAndAuditEvent(t *testing.T) {
	auditStore := audit.NewMemoryStore()
	handler := NewConfigurationHandler(configstore.NewMemoryStore(), auditStore).Routes()

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/password-policy", nil))
	var current struct {
		Values map[string]any `json:"values"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	current.Values["minimumLength"] = float64(20)
	payload, _ := json.Marshal(map[string]any{"values": current.Values})
	put := httptest.NewRecorder()
	handler.ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/password-policy", strings.NewReader(string(payload))))
	if put.Code != http.StatusOK || !strings.Contains(put.Body.String(), `"version":1`) {
		t.Fatalf("put status=%d body=%s", put.Code, put.Body.String())
	}

	history := httptest.NewRecorder()
	handler.ServeHTTP(history, httptest.NewRequest(http.MethodGet, "/password-policy/history", nil))
	if history.Code != http.StatusOK || !strings.Contains(history.Body.String(), `"version":1`) || !strings.Contains(history.Body.String(), `"minimumLength":20`) {
		t.Fatalf("history status=%d body=%s", history.Code, history.Body.String())
	}
	events := auditStore.List(audit.Query{Action: "configuration.update", Limit: 10})
	if len(events) != 1 || events[0].Outcome != "success" || events[0].TargetID != "password-policy" {
		t.Fatalf("unexpected audit events: %#v", events)
	}
}

func TestConfigurationValidationFailureIsAudited(t *testing.T) {
	auditStore := audit.NewMemoryStore()
	handler := NewConfigurationHandler(configstore.NewMemoryStore(), auditStore).Routes()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/password-policy", strings.NewReader(`{"values":{"minimumLength":2}}`)))
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	events := auditStore.List(audit.Query{Action: "configuration.update", Outcome: "failure", Limit: 10})
	if len(events) != 1 || events[0].Metadata["reason"] == nil {
		t.Fatalf("expected validation audit event, got %#v", events)
	}
}

func TestConfigurationRejectsStaleWriteAndRestoresRevision(t *testing.T) {
	store := configstore.NewMemoryStore()
	auditStore := audit.NewMemoryStore()
	handler := NewConfigurationHandler(store, auditStore).Routes()

	firstValues := defaultsForTest(t, handler, "password-policy")
	firstValues["minimumLength"] = float64(16)
	first := putConfigurationForTest(t, handler, "password-policy", firstValues, 0)
	if first.Version != 1 {
		t.Fatalf("first version=%d", first.Version)
	}

	secondValues := defaultsForTest(t, handler, "password-policy")
	secondValues["minimumLength"] = float64(24)
	second := putConfigurationForTest(t, handler, "password-policy", secondValues, 1)
	if second.Version != 2 {
		t.Fatalf("second version=%d", second.Version)
	}

	stalePayload, _ := json.Marshal(map[string]any{"values": firstValues, "version": 1})
	stale := httptest.NewRecorder()
	handler.ServeHTTP(stale, httptest.NewRequest(http.MethodPut, "/password-policy", strings.NewReader(string(stalePayload))))
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), `"currentVersion":2`) {
		t.Fatalf("stale status=%d body=%s", stale.Code, stale.Body.String())
	}

	restorePayload, _ := json.Marshal(map[string]any{"targetVersion": 1, "currentVersion": 2})
	restore := httptest.NewRecorder()
	handler.ServeHTTP(restore, httptest.NewRequest(http.MethodPost, "/password-policy/restore", strings.NewReader(string(restorePayload))))
	if restore.Code != http.StatusOK || !strings.Contains(restore.Body.String(), `"version":3`) || !strings.Contains(restore.Body.String(), `"minimumLength":16`) {
		t.Fatalf("restore status=%d body=%s", restore.Code, restore.Body.String())
	}
}

func defaultsForTest(t *testing.T, handler http.Handler, id string) map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/"+id, nil))
	var body struct {
		Values map[string]any `json:"values"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Values
}

func putConfigurationForTest(t *testing.T, handler http.Handler, id string, values map[string]any, version int64) struct {
	Version int64 `json:"version"`
} {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{"values": values, "version": version})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/"+id, strings.NewReader(string(payload))))
	if recorder.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}
