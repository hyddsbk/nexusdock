package httpx

import (
	"encoding/json"
	"testing"

	"github.com/uvwt/nexusdock/internal/agentdock"
)

func TestNexusCentralToolsAvoidUnsupportedOpenAIComposition(t *testing.T) {
	for _, tool := range nexusToolDefinitionsWithApps(true) {
		assertOpenAICompatibleSchema(t, tool.Name+".inputSchema", tool.InputSchema)
		assertOpenAICompatibleSchema(t, tool.Name+".outputSchema", tool.OutputSchema)
	}
}

func TestNodeMCPToolProjectsUnsupportedProviderSchema(t *testing.T) {
	descriptor := agentdock.ToolDescriptor{
		Name: "compat_probe",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{"type": "string", "const": "run"},
				"items": map[string]any{
					"type": "array",
					"items": map[string]any{
						"oneOf": []map[string]any{
							{
								"type": "object",
								"properties": map[string]any{
									"kind": map[string]any{"type": "string", "const": "alpha"},
								},
							},
						},
					},
				},
			},
			"allOf": []any{
				map[string]any{
					"if": map[string]any{
						"properties": map[string]any{
							"action": map[string]any{"const": "run"},
						},
					},
					"then": map[string]any{"required": []string{"items"}},
				},
			},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status": map[string]any{"type": "string", "const": "ok"},
			},
		},
	}

	tool := nodeMCPToolWithApps(descriptor, true)
	assertOpenAICompatibleSchema(t, tool.Name+".inputSchema", tool.InputSchema)
	assertOpenAICompatibleSchema(t, tool.Name+".outputSchema", tool.OutputSchema)
}

func assertOpenAICompatibleSchema(t *testing.T, path string, value any) {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	var normalized any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		t.Fatalf("normalize %s: %v", path, err)
	}

	blocked := map[string]bool{
		"allOf":             true,
		"not":               true,
		"dependentRequired": true,
		"dependentSchemas":  true,
		"if":                true,
		"then":              true,
		"else":              true,
		"const":             true,
	}

	var walk func(string, any)
	walk = func(current string, item any) {
		switch typed := item.(type) {
		case map[string]any:
			for key, child := range typed {
				if blocked[key] {
					t.Fatalf("%s contains unsupported OpenAI JSON Schema keyword %q", current, key)
				}
				walk(current+"."+key, child)
			}
		case []any:
			for i, child := range typed {
				walk(current, child)
				_ = i
			}
		}
	}
	walk(path, normalized)
}
