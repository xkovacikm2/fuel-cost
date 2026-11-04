package webui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedAppAndPWAAssets(t *testing.T) {
	handler := Handler()
	cases := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/", contentType: "text/html", contains: "spotreba-app"},
		{path: "/vehicles/42/edit", contentType: "text/html", contains: "spotreba-app"},
		{path: "/manifest", contentType: "application/manifest+json", contains: "Spotreba"},
		{path: "/manifest.json", contentType: "application/manifest+json", contains: "Spotreba"},
		{path: "/service-worker", contentType: "javascript", contains: "addEventListener"},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200", tc.path, response.Code)
			}
			if !strings.Contains(response.Header().Get("Content-Type"), tc.contentType) {
				t.Errorf("GET %s content type = %q", tc.path, response.Header().Get("Content-Type"))
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), tc.contains) {
				t.Errorf("GET %s body does not contain %q", tc.path, tc.contains)
			}
		})
	}
}

func TestIndexLinksTheRailsManifestPath(t *testing.T) {
	response := httptest.NewRecorder()
	Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(response.Body.String(), `href="/manifest.json"`) {
		t.Fatal("index does not link to /manifest.json")
	}
}

func TestEmbeddedHandlerRejectsUnknownAPIPaths(t *testing.T) {
	response := httptest.NewRecorder()
	Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown API status = %d, want 404", response.Code)
	}
}
