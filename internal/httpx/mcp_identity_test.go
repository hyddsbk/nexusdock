package httpx

import (
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestInitializeMCPGatewayUsesContractAwareVersion(t *testing.T) {
	server := &Server{mcpAppsEnabledState: true}
	server.initializeMCPGateway()

	version := server.mcp.currentVersion()
	if !strings.HasPrefix(version, "1+tools.") {
		t.Fatalf("MCP version = %q, want contract-aware identity", version)
	}
	if version == "1" {
		t.Fatal("MCP version regressed to static identity")
	}
}

func TestContractAwareMCPVersionCarriesToolFingerprint(t *testing.T) {
	hash := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	got := contractAwareMCPVersion("1", hash)
	if got != "1+tools.0123456789ab" {
		t.Fatalf("contractAwareMCPVersion() = %q", got)
	}
}

func TestMCPToolContractHashIsOrderIndependent(t *testing.T) {
	a := &mcpsdk.Tool{Name: "a", InputSchema: map[string]any{"type": "object"}}
	b := &mcpsdk.Tool{Name: "b", InputSchema: map[string]any{"type": "object"}}
	left := mcpToolContractHash([]*mcpsdk.Tool{a, b})
	right := mcpToolContractHash([]*mcpsdk.Tool{b, a})
	if left == "" || left != right {
		t.Fatalf("tool hash depends on registration order: %q != %q", left, right)
	}
}

func TestMCPToolContractHashChangesWithSchema(t *testing.T) {
	base := &mcpsdk.Tool{
		Name: "demo",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
			},
		},
	}
	changed := &mcpsdk.Tool{
		Name: "demo",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
				"limit": map[string]any{"type": "integer"},
			},
		},
	}
	if mcpToolContractHash([]*mcpsdk.Tool{base}) == mcpToolContractHash([]*mcpsdk.Tool{changed}) {
		t.Fatal("tool schema change did not change MCP contract hash")
	}
}

func TestMCPToolContractHashIgnoresPresentationUIButTracksExecutionMeta(t *testing.T) {
	base := &mcpsdk.Tool{
		Name:        "demo",
		InputSchema: map[string]any{"type": "object"},
		Meta: mcpsdk.Meta{
			"ui":                     map[string]any{"resourceUri": "ui://one"},
			"file_arg_rewrite_paths": []string{"file"},
		},
	}
	uiChanged := &mcpsdk.Tool{
		Name:        "demo",
		InputSchema: map[string]any{"type": "object"},
		Meta: mcpsdk.Meta{
			"ui":                     map[string]any{"resourceUri": "ui://two"},
			"file_arg_rewrite_paths": []string{"file"},
		},
	}
	executionChanged := &mcpsdk.Tool{
		Name:        "demo",
		InputSchema: map[string]any{"type": "object"},
		Meta: mcpsdk.Meta{
			"ui":                     map[string]any{"resourceUri": "ui://one"},
			"file_arg_rewrite_paths": []string{"other"},
		},
	}

	baseHash := mcpToolContractHash([]*mcpsdk.Tool{base})
	if baseHash != mcpToolContractHash([]*mcpsdk.Tool{uiChanged}) {
		t.Fatal("presentation-only UI metadata changed MCP contract hash")
	}
	if baseHash == mcpToolContractHash([]*mcpsdk.Tool{executionChanged}) {
		t.Fatal("execution metadata change did not change MCP contract hash")
	}
	if !strings.HasPrefix(baseHash, "sha256:") {
		t.Fatalf("unexpected contract hash %q", baseHash)
	}
}
