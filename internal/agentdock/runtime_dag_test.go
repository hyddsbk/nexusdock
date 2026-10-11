package agentdock

import "testing"

func TestParseRuntimeTaskListPreservesReadyStepCount(t *testing.T) {
	payload := fixturePayload(t, `{
	  "tasks": [{
	    "id": "task_dag", "status": "active", "review_status": "not_started",
	    "completed_step_count": 1, "step_count": 4, "ready_step_count": 2,
	    "updated_at": "2026-10-11T03:00:00Z"
	  }]
	}`)
	tasks, err := parseRuntimeTaskList("node1", payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].ReadyStepCount != 2 {
		t.Fatalf("ready step count = %#v", tasks)
	}
}

func TestParseRuntimeTaskDetailDerivesReadyFrontierFromDependencies(t *testing.T) {
	payload := fixturePayload(t, `{
	  "task": {
	    "id": "task_dag", "title": "dag", "goal": "run graph", "status": "active", "phase": "execute",
	    "updated_at": "2026-10-11T03:00:00Z",
	    "steps": [
	      {"id": "left", "title": "Left", "phase": "execute", "status": "completed"},
	      {"id": "right", "title": "Right", "phase": "execute", "status": "pending"},
	      {"id": "join", "title": "Join", "phase": "verify", "status": "pending", "depends_on": ["left", "right"]}
	    ]
	  }
	}`)
	detail, err := parseRuntimeTaskDetail("node1", "task_dag", payload)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Summary.ReadyStepCount != 1 || detail.Summary.CurrentStep == nil || detail.Summary.CurrentStep.ID != "right" {
		t.Fatalf("ready frontier summary = %#v", detail.Summary)
	}
	if len(detail.Steps[2].DependsOn) != 2 || detail.Steps[2].DependsOn[0] != "left" || detail.Steps[2].DependsOn[1] != "right" {
		t.Fatalf("dependency edges were not preserved: %#v", detail.Steps)
	}
}

func TestParseRuntimeTaskDetailSuppressesReadyFrontierWhenTaskBlocked(t *testing.T) {
	payload := fixturePayload(t, `{
	  "task": {
	    "id": "task_blocked", "status": "blocked", "phase": "execute", "blocker": "external approval",
	    "updated_at": "2026-10-11T03:00:00Z",
	    "steps": [{"id": "ready", "title": "Ready but externally blocked", "status": "pending"}]
	  }
	}`)
	detail, err := parseRuntimeTaskDetail("node1", "task_blocked", payload)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Summary.ReadyStepCount != 0 || detail.Summary.CurrentStep != nil {
		t.Fatalf("blocked task exposed ready frontier: %#v", detail.Summary)
	}
}
