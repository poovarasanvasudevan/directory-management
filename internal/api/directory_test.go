package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"directory-management/internal/audit"
	"directory-management/internal/config"
	"directory-management/internal/directory"
)

func TestConfigDoesNotExposePassword(t *testing.T) {
	cfg := config.ActiveDirectoryConfig{Mode: "mock", Host: "ad.example.com", BindPassword: "secret", BaseDN: "DC=example,DC=com"}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/config", nil)
	NewDirectoryHandler(cfg, directory.NewMockService(), audit.NewMemoryStore()).Routes().ServeHTTP(recorder, request)
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	directoryConfig := body["directory"].(map[string]any)
	if _, exists := directoryConfig["bindPassword"]; exists {
		t.Fatal("API exposed bind password")
	}
	if body["credentialsConfigured"] != true {
		t.Fatal("API did not report configured credentials")
	}
}

func TestConnectionEndpoint(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/test", nil)
	cfg := config.ActiveDirectoryConfig{Mode: "mock", BaseDN: "DC=example,DC=com"}
	NewDirectoryHandler(cfg, directory.NewMockService(), audit.NewMemoryStore()).Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("got status %d", recorder.Code)
	}
}

func TestDirectoryBrowserRoundTrip(t *testing.T) {
	cfg := config.ActiveDirectoryConfig{Mode: "mock", BaseDN: "DC=example,DC=com", WriteEnabled: true}
	handler := NewDirectoryHandler(cfg, directory.NewMockService(), audit.NewMemoryStore()).Routes()

	browse := httptest.NewRecorder()
	handler.ServeHTTP(browse, httptest.NewRequest(http.MethodGet, "/browse?parent=DC=example,DC=com", nil))
	if browse.Code != http.StatusOK || !strings.Contains(browse.Body.String(), "OU=Employees") {
		t.Fatalf("browse status=%d body=%s", browse.Code, browse.Body.String())
	}

	createUser := httptest.NewRecorder()
	handler.ServeHTTP(createUser, httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{
		"parentDN":"OU=Employees,DC=example,DC=com",
		"name":"Jamie Lee",
		"attributes":{"mail":"jamie.lee@example.com","displayName":"Jamie Lee"}
	}`)))
	if createUser.Code != http.StatusCreated {
		t.Fatalf("create user status=%d body=%s", createUser.Code, createUser.Body.String())
	}

	createGroup := httptest.NewRecorder()
	handler.ServeHTTP(createGroup, httptest.NewRequest(http.MethodPost, "/groups", strings.NewReader(`{
		"parentDN":"OU=Groups,DC=example,DC=com",
		"name":"Browser Testers",
		"description":"Directory browser test group"
	}`)))
	if createGroup.Code != http.StatusCreated {
		t.Fatalf("create group status=%d body=%s", createGroup.Code, createGroup.Body.String())
	}

	userDN := "CN=Jamie Lee,OU=Employees,DC=example,DC=com"
	groupDN := "CN=Browser Testers,OU=Groups,DC=example,DC=com"
	add := httptest.NewRecorder()
	handler.ServeHTTP(add, httptest.NewRequest(http.MethodPost, "/group/members?dn="+url.QueryEscape(groupDN), strings.NewReader(`{"memberDN":`+jsonString(userDN)+`}`)))
	if add.Code != http.StatusOK {
		t.Fatalf("add member status=%d body=%s", add.Code, add.Body.String())
	}

	members := httptest.NewRecorder()
	handler.ServeHTTP(members, httptest.NewRequest(http.MethodGet, "/group/members?dn="+url.QueryEscape(groupDN), nil))
	if members.Code != http.StatusOK || !strings.Contains(members.Body.String(), userDN) {
		t.Fatalf("list members status=%d body=%s", members.Code, members.Body.String())
	}

	attrs := httptest.NewRecorder()
	handler.ServeHTTP(attrs, httptest.NewRequest(http.MethodPut, "/entry/attributes?dn="+url.QueryEscape(userDN), strings.NewReader(`{"attributes":{"title":"Engineer"}}`)))
	if attrs.Code != http.StatusOK || !strings.Contains(attrs.Body.String(), `"title":"Engineer"`) {
		t.Fatalf("update attrs status=%d body=%s", attrs.Code, attrs.Body.String())
	}
}

func jsonString(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func TestDirectoryWriteGuard(t *testing.T) {
	cfg := config.ActiveDirectoryConfig{Mode: "ldap", BaseDN: "DC=example,DC=com", WriteEnabled: false}
	handler := NewDirectoryHandler(cfg, directory.NewMockService(), audit.NewMemoryStore()).Routes()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{
		"parentDN":"OU=Employees,DC=example,DC=com","name":"Blocked User"
	}`)))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
