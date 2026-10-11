package httpx

import (
	"net/http"
	"testing"
)

func TestRuntimeTasksThroughBridgePreservesHandoff(t *testing.T) {
	_, mux, nodeID := runtimeContractTestServer(t, map[string]string{
		"/internal/runtime/tasks": `{"ok":true,"tasks":[{
		  "id":"task_handoff","title":"handoff","goal":"resume safely","status":"active","phase":"execute","review_status":"not_started",
		  "completed_step_count":1,"step_count":2,"ready_step_count":1,
		  "handoff":{"next_step_id":"build","context_ref_count":2,"risk_count":1,"revision":"hof_0123456789abcdef","created_at":"2026-10-11T13:00:00Z"},
		  "updated_at":"2026-10-11T13:00:00Z"
		}]}`,
	})
	payload := runtimeContractRequest(t, mux, http.MethodGet, "/v1/runtime/nodes/"+nodeID+"/tasks")
	items, _ := payload["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %#v", payload["items"])
	}
	task, _ := items[0].(map[string]any)
	handoff, _ := task["handoff"].(map[string]any)
	if handoff == nil || handoff["next_step_id"] != "build" || handoff["context_ref_count"] != float64(2) || handoff["risk_count"] != float64(1) {
		t.Fatalf("handoff = %#v", task["handoff"])
	}
}

func TestRuntimeTaskDetailThroughBridgePreservesFullHandoff(t *testing.T) {
	_, mux, nodeID := runtimeContractTestServer(t, map[string]string{
		"/internal/runtime/tasks/task_handoff": `{"ok":true,"task":{
		  "id":"task_handoff","title":"handoff","goal":"resume safely","status":"active","phase":"execute",
		  "summary":"Inspection complete; resume at build.",
		  "handoff":{"next_step_id":"build","context_refs":["artifact://inspect/latest","commit://abc123"],"risks":["rollback verification pending"],"revision":"hof_0123456789abcdef","created_at":"2026-10-11T13:00:00Z"},
		  "steps":[{"id":"inspect","title":"Inspect","status":"completed"},{"id":"build","title":"Build","status":"pending","depends_on":["inspect"],"context_refs":["spec://feature/current"]}],
		  "conditions":[],"events":[],"updated_at":"2026-10-11T13:00:00Z"
		}}`,
	})
	payload := runtimeContractRequest(t, mux, http.MethodGet, "/v1/runtime/nodes/"+nodeID+"/tasks/task_handoff.json")
	task, _ := payload["task"].(map[string]any)
	handoff, _ := task["handoff"].(map[string]any)
	if handoff == nil || handoff["next_step_id"] != "build" || handoff["revision"] != "hof_0123456789abcdef" {
		t.Fatalf("handoff = %#v", task["handoff"])
	}
	refs, _ := handoff["context_refs"].([]any)
	risks, _ := handoff["risks"].([]any)
	if len(refs) != 2 || len(risks) != 1 {
		t.Fatalf("handoff payload = %#v", handoff)
	}
}
