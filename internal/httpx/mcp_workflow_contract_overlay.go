package httpx

import mcpcontract "github.com/uvwt/agentdock-protocol/mcpcontract"

// Temporary compatibility overlay for the standards-first Task Admission rollout.
// Upstream agentdock-protocol does not yet carry these workflow fields; keep this
// isolated so it can be removed once the canonical protocol contains the same contract.
func nexusCentralInputSchema(name string) (map[string]any, bool) {
	schema, ok := mcpcontract.InputSchema(name)
	if !ok || name != mcpcontract.ToolWorkflowTemplateManage {
		return schema, ok
	}
	schema = cloneSchemaMap(schema)
	properties, _ := schema["properties"].(map[string]any)
	if properties == nil {
		properties = map[string]any{}
		schema["properties"] = properties
	}
	properties["node_id"] = map[string]any{
		"type":        "string",
		"description": "Target AgentDock node ID. Required for action=match so the workflow receipt is persisted beside the frozen Task Admission.",
	}
	properties["admission_ref"] = map[string]any{
		"type":        "string",
		"description": "Passing frozen Task Admission reference. Required for action=match.",
	}
	return schema, true
}

func nexusCentralOutputSchema(name string) (map[string]any, bool) {
	schema, ok := mcpcontract.OutputSchema(name)
	if !ok || name != mcpcontract.ToolWorkflowTemplateManage {
		return schema, ok
	}
	schema = cloneSchemaMap(schema)
	properties, _ := schema["properties"].(map[string]any)
	if properties == nil {
		properties = map[string]any{}
		schema["properties"] = properties
	}
	properties["admission_ref"] = map[string]any{
		"type":        "string",
		"description": "Frozen Task Admission reference consumed by workflow matching.",
	}
	properties["admission_digest"] = map[string]any{
		"type":        "string",
		"description": "SHA-256 digest of the frozen Admission contract.",
	}
	properties["workflow_match_digest"] = map[string]any{
		"type":        "string",
		"description": "SHA-256 digest of the workflow-match receipt persisted by the target AgentDock node.",
	}
	return schema, true
}

func cloneSchemaMap(value map[string]any) map[string]any {
	return cloneSchemaValue(value).(map[string]any)
}

func cloneSchemaValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, child := range typed {
			cloned[key] = cloneSchemaValue(child)
		}
		return cloned
	case []any:
		cloned := make([]any, len(typed))
		for index, child := range typed {
			cloned[index] = cloneSchemaValue(child)
		}
		return cloned
	case []string:
		return append([]string(nil), typed...)
	default:
		return typed
	}
}
