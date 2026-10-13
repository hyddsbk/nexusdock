package httpx

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/uvwt/nexusdock/internal/auth"
	"github.com/uvwt/nexusdock/internal/config"
	"github.com/uvwt/nexusdock/internal/core"
)

func TestOAuthDiscoveryUsesConfiguredPublicURLAsAuthoritativeOrigin(t *testing.T) {
	db, err := core.OpenSQLite(t.Context(), ":memory:", 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := core.EnsureSchema(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	authService := auth.NewService(db)
	mcpTokenStore, err := auth.NewMCPTokenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(
		config.Config{PublicURL: "https://nexus.clw-groups.com"},
		nil,
		slog.Default(),
		WithSystemDatabase(db),
		WithWebAuthentication(authService),
		WithMCPTokenStore(mcpTokenStore),
	)
	handler := server.Handler()

	for _, path := range []string{
		"/.well-known/oauth-protected-resource/mcp",
		"/.well-known/oauth-authorization-server",
	} {
		req := httptest.NewRequest(http.MethodGet, "http://internal-nexus:18777"+path, nil)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, res.Code, res.Body.String())
		}
		body := res.Body.String()
		if strings.Contains(body, "http://internal-nexus:18777") {
			t.Fatalf("%s leaked internal HTTP origin: %s", path, body)
		}
		if !strings.Contains(body, "https://nexus.clw-groups.com") {
			t.Fatalf("%s did not use configured public URL: %s", path, body)
		}
	}
}
