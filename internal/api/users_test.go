package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"directory-management/internal/audit"
	"directory-management/internal/config"
	"directory-management/internal/directory"
	"directory-management/internal/policies"
)

func TestUserSearchResetUnlockAndAudit(t *testing.T) {
	service := directory.NewMockService()
	auditStore := audit.NewMemoryStore()
	handler := NewUsersHandler(config.ActiveDirectoryConfig{Mode: "mock", ConnectTimeout: 1000000000}, service, auditStore, policies.NewMemoryStore()).Routes()

	search := httptest.NewRecorder()
	handler.ServeHTTP(search, httptest.NewRequest(http.MethodGet, "/?q=aisha", nil))
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), `"aisha.khan"`) {
		t.Fatalf("search status=%d body=%s", search.Code, search.Body.String())
	}

	reset := httptest.NewRecorder()
	handler.ServeHTTP(reset, httptest.NewRequest(http.MethodPost, "/aisha.khan/password-reset", strings.NewReader(`{"password":"LongTemporary!42","forceChange":true}`)))
	if reset.Code != http.StatusOK {
		t.Fatalf("reset status=%d body=%s", reset.Code, reset.Body.String())
	}

	unlock := httptest.NewRecorder()
	handler.ServeHTTP(unlock, httptest.NewRequest(http.MethodPost, "/aisha.khan/unlock", nil))
	if unlock.Code != http.StatusOK {
		t.Fatalf("unlock status=%d body=%s", unlock.Code, unlock.Body.String())
	}

	events := auditStore.List(audit.Query{Limit: 100})
	actions := map[string]bool{}
	for _, event := range events {
		actions[event.Action] = true
	}
	if !actions["directory.account-unlock"] || !actions["directory.password-reset"] {
		t.Fatalf("sensitive operations were not audited: %#v", events)
	}
}

func TestUserResetValidationAndReadOnlyDirectory(t *testing.T) {
	service := directory.NewMockService()
	handler := NewUsersHandler(config.ActiveDirectoryConfig{Mode: "ldap", ConnectTimeout: 1000000000}, service, audit.NewMemoryStore(), policies.NewMemoryStore()).Routes()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/aisha.khan/password-reset", strings.NewReader(`{"password":"short"}`)))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected read-only rejection, status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAuditFiltering(t *testing.T) {
	store := audit.NewMemoryStore()
	store.Record(audit.Event{Actor: "admin", Action: "directory.account-unlock", TargetType: "directory-user", TargetID: "aisha.khan", Outcome: "success"})
	recorder := httptest.NewRecorder()
	NewAuditHandler(store).Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/?q=aisha&outcome=success", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"aisha.khan"`) {
		t.Fatalf("audit status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
