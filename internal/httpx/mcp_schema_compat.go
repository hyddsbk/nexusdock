package httpx

import (
	"encoding/json"
	"reflect"
)

// openAICompatibleSchemaProjection returns a transport-only projection of a
// JSON Schema that stays within OpenAI's supported strict-tool subset.
//
// NexusDock remains a transparent router: this projection only broadens the
// schema advertised to the client when a provider uses unsupported composition
// keywords. The target AgentDock remains authoritative for runtime validation.
func openAICompatibleSchemaProjection(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	if !schemaNeedsOpenAIProjection(schema) {
		return schema
	}

	encoded, err := json.Marshal(schema)
	if err != nil {
		projected, _ := projectOpenAICompatibleSchemaValue(schema).(map[string]any)
		return projected
	}

	var normalized map[string]any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		projected, _ := projectOpenAICompatibleSchemaValue(schema).(map[string]any)
		return projected
	}

	projected, _ := projectOpenAICompatibleSchemaValue(normalized).(map[string]any)
	return projected
}

func schemaNeedsOpenAIProjection(value any) bool {
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

	var walk func(reflect.Value) bool
	walk = func(current reflect.Value) bool {
		if !current.IsValid() {
			return false
		}
		for current.Kind() == reflect.Interface || current.Kind() == reflect.Pointer {
			if current.IsNil() {
				return false
			}
			current = current.Elem()
		}
		switch current.Kind() {
		case reflect.Map:
			iter := current.MapRange()
			for iter.Next() {
				key := iter.Key()
				if key.IsValid() && key.Kind() == reflect.String && blocked[key.String()] {
					return true
				}
				if walk(iter.Value()) {
					return true
				}
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < current.Len(); i++ {
				if walk(current.Index(i)) {
					return true
				}
			}
		}
		return false
	}

	return walk(reflect.ValueOf(value))
}

func projectOpenAICompatibleSchemaValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		projected := make(map[string]any, len(typed))
		for key, child := range typed {
			switch key {
			case "allOf", "not", "dependentRequired", "dependentSchemas", "if", "then", "else":
				continue
			case "const":
				projected["enum"] = []any{projectOpenAICompatibleSchemaValue(child)}
			default:
				projected[key] = projectOpenAICompatibleSchemaValue(child)
			}
		}
		return projected
	case []any:
		projected := make([]any, len(typed))
		for i, child := range typed {
			projected[i] = projectOpenAICompatibleSchemaValue(child)
		}
		return projected
	default:
		return value
	}
}
