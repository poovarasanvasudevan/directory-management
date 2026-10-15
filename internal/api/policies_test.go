package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"directory-management/internal/policies"
)

func TestPolicyCRUD(t *testing.T) {
	handler := NewPolicyHandler(policies.NewMemoryStore()).Routes()
	create := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Finance recovery","scope":"Finance","domain":"ADSELFSERVICE","permissions":["Reset Password"],"status":"Active","ouDNs":["OU=Finance,DC=example,DC=com"],"groupDNs":[]}`))
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), "Finance recovery") {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var policy policies.Policy
	if err := json.Unmarshal(created.Body.Bytes(), &policy); err != nil {
		t.Fatal(err)
	}

	update := httptest.NewRequest(http.MethodPut, "/"+strconv.FormatInt(policy.ID, 10), strings.NewReader(`{"name":"Finance recovery","scope":"Finance team","domain":"ADSELFSERVICE","permissions":["Reset Password","Unlock Account"],"status":"Review","ouDNs":["OU=Finance,DC=example,DC=com"],"groupDNs":[]}`))
	updated := httptest.NewRecorder()
	handler.ServeHTTP(updated, update)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), "Finance team") {
		t.Fatalf("update status=%d body=%s", updated.Code, updated.Body.String())
	}

	deleted := httptest.NewRecorder()
	handler.ServeHTTP(deleted, httptest.NewRequest(http.MethodDelete, "/"+strconv.FormatInt(policy.ID, 10), nil))
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d", deleted.Code)
	}
}

func TestPolicyValidation(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewPolicyHandler(policies.NewMemoryStore()).Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"","permissions":[]}`)))
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", recorder.Code)
	}
}
