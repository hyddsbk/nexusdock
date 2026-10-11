package agentdock

import "testing"

func TestParseRuntimeTaskDetailPreservesTraceability(t *testing.T) {
	payload := fixturePayload(t, `{
	  "task": {
	    "id": "task_trace", "title": "trace", "goal": "implement spec", "status": "active", "phase": "closeout",
	    "updated_at": "2026-10-12T00:00:00Z",
	    "requirement_refs": ["spec://feature#R1", "spec://feature#R2"],
	    "steps": [
	      {
	        "id": "build", "title": "Build", "phase": "execute", "status": "completed",
	        "requirement_refs": ["spec://feature#R1"],
	        "execution_evidence": {"evidence_refs": ["commit://agentdock/abc123", "test://taskstate/TestR1"]}
	      },
	      {
	        "id": "verify", "title": "Verify", "phase": "verify", "status": "completed",
	        "requirement_refs": ["spec://feature#R2"],
	        "execution_evidence": {"evidence_refs": ["test://integration/TestR2"]}
	      }
	    ],
	    "final_review": {
	      "status": "pass", "summary": "covered", "verified_facts": ["requirements covered"],
	      "intent_spec": {"status": "pass", "evidence": ["coverage passed"]},
	      "engineering_policy": {"status": "pass", "evidence": ["tests passed"]},
	      "traceability": {
	        "required_count": 2, "covered_count": 2,
	        "items": [
	          {"requirement_ref": "spec://feature#R1", "step_ids": ["build"], "evidence_refs": ["commit://agentdock/abc123", "test://taskstate/TestR1"]},
	          {"requirement_ref": "spec://feature#R2", "step_ids": ["verify"], "evidence_refs": ["test://integration/TestR2"]}
	        ]
	      },
	      "review_revision": "rev_0123456789abcdef"
	    }
	  }
	}`)
	detail, err := parseRuntimeTaskDetail("node1", "task_trace", payload)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Summary.RequirementRefCount != 2 || len(detail.Summary.RequirementRefs) != 2 {
		t.Fatalf("task requirement refs = %#v", detail.Summary)
	}
	if len(detail.Steps) != 2 || len(detail.Steps[0].RequirementRefs) != 1 || detail.Steps[0].ExecutionEvidence == nil {
		t.Fatalf("step traceability lost: %#v", detail.Steps)
	}
	if detail.FinalReview == nil || detail.FinalReview.Traceability == nil ||
		detail.FinalReview.Traceability.RequiredCount != 2 ||
		detail.FinalReview.Traceability.CoveredCount != 2 ||
		len(detail.FinalReview.Traceability.Items) != 2 {
		t.Fatalf("final review traceability lost: %#v", detail.FinalReview)
	}
}

func TestParseRuntimeTaskListPreservesTraceabilitySummary(t *testing.T) {
	payload := fixturePayload(t, `{
	  "tasks": [{
	    "id": "task_trace", "status": "active", "review_status": "not_started",
	    "requirement_refs": ["spec://feature#R1"], "requirement_ref_count": 1,
	    "completed_step_count": 0, "step_count": 1, "ready_step_count": 1,
	    "current_step": {
	      "id": "build", "title": "Build", "status": "pending",
	      "requirement_refs": ["spec://feature#R1"]
	    },
	    "updated_at": "2026-10-12T00:00:00Z"
	  }]
	}`)
	tasks, err := parseRuntimeTaskList("node1", payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].RequirementRefCount != 1 || len(tasks[0].RequirementRefs) != 1 {
		t.Fatalf("list traceability summary = %#v", tasks)
	}
	if tasks[0].CurrentStep == nil || len(tasks[0].CurrentStep.RequirementRefs) != 1 {
		t.Fatalf("current step traceability = %#v", tasks[0].CurrentStep)
	}
}
