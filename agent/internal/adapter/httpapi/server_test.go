package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type readyHealth struct{}

func (readyHealth) Ready() bool { return true }

type agentCatalogStub struct{}

func (agentCatalogStub) DefaultAgentID() string { return "default" }
func (agentCatalogStub) ListAgentIDs() []string { return []string{"alpha", "default"} }

func TestHandlerAllowsLocalOrigin(t *testing.T) {
	srv := NewServer(Services{Health: readyHealth{}}, "")
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5173" {
		t.Fatalf("expected allow origin header, got %q", got)
	}
	if got := rec.Header().Get("Vary"); got == "" {
		t.Fatalf("expected vary header to be set")
	}
}

func TestHandlerHandlesLocalPreflight(t *testing.T) {
	srv := NewServer(Services{Health: readyHealth{}}, "")
	req := httptest.NewRequest(http.MethodOptions, "/v1/chat/completions", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected allow origin header, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "authorization,content-type" {
		t.Fatalf("expected request headers to be echoed, got %q", got)
	}
}

func TestHandlerRejectsRemotePreflight(t *testing.T) {
	srv := NewServer(Services{Health: readyHealth{}}, "")
	req := httptest.NewRequest(http.MethodOptions, "/healthz", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allow origin header, got %q", got)
	}
}

func TestIsLocalOrigin(t *testing.T) {
	tests := []struct {
		origin string
		want   bool
	}{
		{origin: "http://127.0.0.1:5173", want: true},
		{origin: "http://localhost:4173", want: true},
		{origin: "http://[::1]:3000", want: true},
		{origin: "http://0.0.0.0:5173", want: true},
		{origin: "https://example.com", want: false},
		{origin: "null", want: false},
	}

	for _, tc := range tests {
		if got := isLocalOrigin(tc.origin); got != tc.want {
			t.Fatalf("origin %q: expected %v, got %v", tc.origin, tc.want, got)
		}
	}
}

func TestHandlerServesBundledWebUIAndSpaFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html><body>console</body></html>"), 0o644); err != nil {
		t.Fatalf("write index failed: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatalf("mkdir assets failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte("console.log('ok');"), 0o644); err != nil {
		t.Fatalf("write asset failed: %v", err)
	}

	srv := NewServer(Services{Health: readyHealth{}}, dir)

	rootReq := httptest.NewRequest(http.MethodGet, "/", nil)
	rootRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rootRec, rootReq)
	if rootRec.Code != http.StatusOK {
		t.Fatalf("expected root status %d, got %d", http.StatusOK, rootRec.Code)
	}
	if !strings.Contains(rootRec.Body.String(), "console") {
		t.Fatalf("expected root body to contain webui markup, got %q", rootRec.Body.String())
	}

	routeReq := httptest.NewRequest(http.MethodGet, "/app/agents/default/tasks", nil)
	routeRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(routeRec, routeReq)
	if routeRec.Code != http.StatusOK {
		t.Fatalf("expected route status %d, got %d", http.StatusOK, routeRec.Code)
	}
	if !strings.Contains(routeRec.Body.String(), "console") {
		t.Fatalf("expected SPA fallback to serve index, got %q", routeRec.Body.String())
	}

	assetReq := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	assetRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(assetRec, assetReq)
	if assetRec.Code != http.StatusOK {
		t.Fatalf("expected asset status %d, got %d", http.StatusOK, assetRec.Code)
	}
	if !strings.Contains(assetRec.Body.String(), "console.log") {
		t.Fatalf("expected asset body, got %q", assetRec.Body.String())
	}
}

func TestHandlerPreservesReservedRoutesWhenServingWebUI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html><body>console</body></html>"), 0o644); err != nil {
		t.Fatalf("write index failed: %v", err)
	}

	srv := NewServer(Services{Health: readyHealth{}}, dir)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected health status %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("expected json content type, got %q", got)
	}
	if strings.Contains(rec.Body.String(), "console") {
		t.Fatalf("expected health route to bypass spa handler, got %q", rec.Body.String())
	}
}

func TestRuntimeAgentsEndpoint(t *testing.T) {
	srv := NewServer(Services{
		Health:  readyHealth{},
		Catalog: agentCatalogStub{},
	}, "")
	req := httptest.NewRequest(http.MethodGet, "/v1/runtime/agents", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "\"default_agent\":\"default\"") {
		t.Fatalf("expected default agent in body, got %q", body)
	}
	if !strings.Contains(body, "\"agent_id\":\"alpha\"") {
		t.Fatalf("expected agent list in body, got %q", body)
	}
}
