package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"directory-management/internal/configstore"
)

func TestCapabilitiesCatalog(t *testing.T) {
	store := configstore.NewMemoryStore()
	recorder := httptest.NewRecorder()
	NewCapabilitiesHandler(store).Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body struct {
		Capabilities []Capability `json:"capabilities"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	definitions := configurationDefinitions()
	for _, expected := range []string{"policy-configuration", "anti-hacking", "privacy-licensing", "multi-factor-authentication", "mail-sms"} {
		found := false
		for _, item := range body.Capabilities {
			if item.ID == expected {
				found = true
				if item.ID != "directory-connection" && item.Status == "planned" {
					t.Fatalf("%s still planned", item.ID)
				}
				break
			}
		}
		if !found {
			t.Fatalf("response missing %q", expected)
		}
	}
	for _, item := range body.Capabilities {
		if item.ID == "directory-connection" {
			continue
		}
		if _, ok := definitions[item.ID]; ok && item.Status == "planned" {
			t.Fatalf("%s has a configuration definition but status is planned", item.ID)
		}
		if strings.Contains(recorder.Body.String(), `"status":"planned"`) && item.Status == "planned" {
			// allow only if no definition — already checked above
		}
	}
}

func TestCapabilitiesConfiguredWhenSaved(t *testing.T) {
	store := configstore.NewMemoryStore()
	if _, err := store.Put(httptest.NewRequest(http.MethodGet, "/", nil).Context(), "config:multi-factor-authentication", map[string]any{"policy": "selfservice"}); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	NewCapabilitiesHandler(store).Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"id":"multi-factor-authentication"`) || !strings.Contains(recorder.Body.String(), `"status":"configured"`) {
		t.Fatalf("expected configured MFA capability, body=%s", recorder.Body.String())
	}
}
