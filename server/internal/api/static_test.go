package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestServerServesEmbeddedSPAAndPWAAssets(t *testing.T) {
	handler, _ := newTestHandler(t)
	for _, tc := range []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/", contentType: "text/html", contains: "spotreba-app"},
		{path: "/vehicles/42/edit", contentType: "text/html", contains: "spotreba-app"},
		{path: "/manifest", contentType: "application/manifest+json", contains: "Spotreba"},
		{path: "/service-worker", contentType: "javascript", contains: "spotreba-v2"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			response := apiRequest(handler, http.MethodGet, tc.path, "", nil)
			if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), tc.contentType) || !strings.Contains(response.Body.String(), tc.contains) {
				t.Fatalf("GET %s = %d, %s, %q", tc.path, response.Code, response.Header().Get("Content-Type"), response.Body.String())
			}
		})
	}
	unknownAPI := apiRequest(handler, http.MethodGet, "/api/not-found", "", nil)
	if unknownAPI.Code != http.StatusNotFound {
		t.Fatalf("unknown API path status = %d, want 404", unknownAPI.Code)
	}
}
