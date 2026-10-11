package agentdock

import "testing"

func TestParseRuntimeTaskListPreservesCompactHandoff(t *testing.T) {
	payload := fixturePayload(t, `{
	  "tasks": [{
	    "id": "task_handoff", "title": "handoff", "goal": "resume safely",
	    "status": "active", "phase": "execute", "review_status": "not_started",
	    "completed_step_count": 1, "step_count": 2, "ready_step_count": 1,
	    "handoff": {
	      "next_step_id": "build",
	      "context_ref_count": 2,
	      "risk_count": 1,
	      "revision": "hof_0123456789abcdef",
	      "created_at": "2026-10-11T13:00:00Z"
	    },
	    "updated_at": "2026-10-11T13:00:00Z"
	  }]
	}`)
	tasks, err := parseRuntimeTaskList("node1", payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Handoff == nil {
		t.Fatalf("handoff missing: %#v", tasks)
	}
	h := tasks[0].Handoff
	if h.NextStepID != "build" || h.ContextRefCount != 2 || h.RiskCount != 1 || h.Revision != "hof_0123456789abcdef" {
		t.Fatalf("handoff = %#v", h)
	}
}

func TestParseRuntimeTaskDetailPreservesFullHandoff(t *testing.T) {
	payload := fixturePayload(t, `{
	  "task": {
	    "id": "task_handoff", "title": "handoff", "goal": "resume safely",
	    "status": "active", "phase": "execute",
	    "summary": "Inspection complete; resume at build.",
	    "handoff": {
	      "next_step_id": "build",
	      "context_refs": ["artifact://inspect/latest", "commit://abc123"],
	      "risks": ["Production activation still needs rollback verification."],
	      "revision": "hof_0123456789abcdef",
	      "created_at": "2026-10-11T13:00:00Z"
	    },
	    "steps": [
	      {"id":"inspect","title":"Inspect","status":"completed"},
	      {"id":"build","title":"Build","status":"pending","depends_on":["inspect"],"context_refs":["spec://feature/current"]}
	    ],
	    "conditions": [], "events": [],
	    "updated_at": "2026-10-11T13:00:00Z"
	  }
	}`)
	detail, err := parseRuntimeTaskDetail("node1", "task_handoff", payload)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Summary.Handoff == nil {
		t.Fatalf("handoff missing: %#v", detail.Summary)
	}
	h := detail.Summary.Handoff
	if h.NextStepID != "build" || len(h.ContextRefs) != 2 || len(h.Risks) != 1 || h.Revision != "hof_0123456789abcdef" {
		t.Fatalf("handoff = %#v", h)
	}
	if detail.Summary.ReadyStepCount != 1 || detail.Summary.CurrentStep == nil || detail.Summary.CurrentStep.ID != "build" {
		t.Fatalf("derived frontier = %#v", detail.Summary)
	}
}

func TestParseRuntimeTaskHandoffRequiresRevision(t *testing.T) {
	payload := fixturePayload(t, `{
	  "tasks": [{
	    "id": "task_handoff", "status": "active", "review_status": "not_started",
	    "handoff": {"next_step_id": "build"},
	    "updated_at": "2026-10-11T13:00:00Z"
	  }]
	}`)
	if _, err := parseRuntimeTaskList("node1", payload); err == nil {
		t.Fatal("handoff without revision was accepted")
	}
}
