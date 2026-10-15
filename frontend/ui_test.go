package frontend

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesReactApp(t *testing.T) {
	handler := Handler()

	for _, route := range []string{"/", "/directories/example"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, route, nil)
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s returned %d", route, recorder.Code)
		}
		if !strings.Contains(recorder.Body.String(), `<div id="root"></div>`) {
			t.Fatalf("GET %s did not return the React app", route)
		}
	}
}
