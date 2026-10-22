package agentdock

import "testing"

func TestTaskSchemaCompatibilityLegacyTaskSchema10TransportRemainsReadable(t *testing.T) {
	payload := fixturePayload(t, `{
	  "task": {
	    "schema_version": 10,
	    "id": "task_v10",
	    "title": "legacy v10",
	    "status": "completed",
	    "phase": "closeout",
	    "updated_at": "2026-10-13T00:00:00Z"
	  }
	}`)
	detail, err := parseRuntimeTaskDetail("node_legacy", "task_v10", payload)
	if err != nil {
		t.Fatalf("schema 10 task transport must remain readable: %v", err)
	}
	if detail.Summary.SchemaVersion != 10 {
		t.Fatalf("schema version = %d, want 10", detail.Summary.SchemaVersion)
	}
}

func TestTaskSchemaCompatibilityLegacyTaskTransportWithoutSchemaRemainsReadable(t *testing.T) {
	payload := fixturePayload(t, `{
	  "task": {
	    "id": "legacy_transport",
	    "title": "legacy transport",
	    "status": "active",
	    "phase": "check",
	    "updated_at": "2026-10-13T00:00:00Z"
	  }
	}`)
	detail, err := parseRuntimeTaskDetail("node_legacy", "legacy_transport", payload)
	if err != nil {
		t.Fatalf("legacy detail transport without schema_version must remain readable: %v", err)
	}
	if detail.Summary.SchemaVersion != 0 {
		t.Fatalf("missing schema transport marker = %d, want 0", detail.Summary.SchemaVersion)
	}
}

func TestTaskSchemaCompatibilityFutureTaskSchemaIsRejectedAtBridge(t *testing.T) {
	payload := fixturePayload(t, `{
	  "task": {
	    "schema_version": 13,
	    "id": "task_future",
	    "title": "future",
	    "status": "active",
	    "phase": "check",
	    "updated_at": "2026-10-13T00:00:00Z"
	  }
	}`)
	_, err := parseRuntimeTaskDetail("node_future", "task_future", payload)
	assertContractError(t, err, "GET /internal/runtime/tasks/task_future", "task.schema_version")
}

func TestTaskSchemaCompatibilityPolicyConstants(t *testing.T) {
	if runtimeMinReadableTaskSchemaVersion != 1 || runtimeCurrentTaskSchemaVersion != 12 {
		t.Fatalf("task schema window = %d..%d, want 1..12", runtimeMinReadableTaskSchemaVersion, runtimeCurrentTaskSchemaVersion)
	}
}

func TestTaskSchemaCompatibilityTaskSchema12RemainsReadable(t *testing.T) {
	payload := fixturePayload(t, `{
	  "task": {
	    "schema_version": 12,
	    "id": "task_v12",
	    "title": "admission-bound task",
	    "status": "active",
	    "phase": "execute",
	    "updated_at": "2026-10-23T00:00:00Z"
	  }
	}`)
	detail, err := parseRuntimeTaskDetail("node_current", "task_v12", payload)
	if err != nil {
		t.Fatalf("schema 12 task transport must remain readable: %v", err)
	}
	if detail.Summary.SchemaVersion != 12 {
		t.Fatalf("schema version = %d, want 12", detail.Summary.SchemaVersion)
	}
}

func TestTaskSchemaCompatibilityRejectsExplicitZero(t *testing.T) {
	payload := fixturePayload(t, `{
	  "task": {
	    "schema_version": 0,
	    "id": "task_zero",
	    "status": "active",
	    "updated_at": "2026-10-13T00:00:00Z"
	  }
	}`)
	_, err := parseRuntimeTaskDetail("node_zero", "task_zero", payload)
	assertContractError(t, err, "GET /internal/runtime/tasks/task_zero", "task.schema_version")
}

func TestTaskSchemaCompatibilityRejectsFutureSchemaInList(t *testing.T) {
	payload := fixturePayload(t, `{
	  "tasks": [{
	    "schema_version": 13,
	    "id": "task_future",
	    "status": "active",
	    "review_status": "not_started",
	    "updated_at": "2026-10-13T00:00:00Z"
	  }]
	}`)
	_, err := parseRuntimeTaskList("node_future", payload)
	assertContractError(t, err, "GET /internal/runtime/tasks", "tasks[0].schema_version")
}
