package agentdock

import (
	"strings"
	"testing"
)

func TestParseRuntimeTaskDetailPreservesEvidenceSemantics(t *testing.T) {
	payload := fixturePayload(t, `{
	  "task": {
	    "id": "tsk_p12",
	    "title": "P12",
	    "goal": "preserve semantic evidence",
	    "status": "active",
	    "phase": "closeout",
	    "updated_at": "2026-10-13T05:00:00Z",
	    "steps": [{
	      "id": "inspect",
	      "title": "Inspect",
	      "status": "completed",
	      "phase": "verify",
	      "evidence_records": [{
	        "id": "ev_0123456789abcdef",
	        "statement": "No direct edge was observed inside the bounded static scan.",
	        "fact_status": "observed",
	        "source_refs": ["artifact://scan/p12"],
	        "coverage": "partial",
	        "limitations": ["Dynamic dispatch was outside the boundary."],
	        "untrusted_content": true,
	        "partial_result": true,
	        "created_at": "2026-10-13T04:59:00Z"
	      }]
	    }],
	    "unknowns": [{
	      "id": "unk_0123456789abcdef",
	      "question": "Could a dynamic edge still exist?",
	      "blocking": false,
	      "status": "open",
	      "revision": 1,
	      "opened_step_id": "inspect",
	      "evidence_refs": ["evidence://ev_0123456789abcdef"],
	      "opened_at": "2026-10-13T04:59:30Z"
	    }],
	    "final_review": {
	      "status": "pass",
	      "summary": "Bounded claim accepted.",
	      "verified_facts": ["Only the bounded static observation is claimed."],
	      "acknowledged_unknown_ids": ["unk_0123456789abcdef"],
	      "review_revision": "rev_p12",
	      "reviewed_at": "2026-10-13T05:00:00Z"
	    }
	  }
	}`)
	detail, err := parseRuntimeTaskDetail("node1", "tsk_p12", payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Steps) != 1 || len(detail.Steps[0].EvidenceRecords) != 1 {
		t.Fatalf("evidence records lost: %#v", detail.Steps)
	}
	record := detail.Steps[0].EvidenceRecords[0]
	if record.ID != "ev_0123456789abcdef" || record.FactStatus != "observed" ||
		record.Coverage != "partial" || !record.UntrustedContent || !record.PartialResult ||
		len(record.Limitations) != 1 {
		t.Fatalf("evidence semantics changed: %#v", record)
	}
	if len(detail.Unknowns) != 1 || detail.Unknowns[0].Status != "open" ||
		detail.Unknowns[0].Revision != 1 || detail.Unknowns[0].Blocking {
		t.Fatalf("unknown ledger lost: %#v", detail.Unknowns)
	}
	if detail.Summary.UnknownCount != 1 || detail.Summary.OpenUnknownCount != 1 {
		t.Fatalf("unknown summary counts = %#v", detail.Summary)
	}
	if detail.FinalReview == nil || len(detail.FinalReview.AcknowledgedUnknownIDs) != 1 ||
		detail.FinalReview.AcknowledgedUnknownIDs[0] != "unk_0123456789abcdef" {
		t.Fatalf("final-review unknown acknowledgement lost: %#v", detail.FinalReview)
	}
}

func TestParseRuntimeEvidenceSemanticsRejectsInvalidContracts(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "invalid fact status",
			payload: `{"task":{"id":"tsk_p12","status":"active","updated_at":"2026-10-13T05:00:00Z","steps":[{"id":"inspect","status":"completed","evidence_records":[{"id":"ev_0123456789abcdef","statement":"x","fact_status":"guessed","source_refs":["artifact://x"],"coverage":"partial","created_at":"2026-10-13T04:59:00Z"}]}]}}`,
			want:    "fact_status",
		},
		{
			name:    "partial cannot be complete",
			payload: `{"task":{"id":"tsk_p12","status":"active","updated_at":"2026-10-13T05:00:00Z","steps":[{"id":"inspect","status":"completed","evidence_records":[{"id":"ev_0123456789abcdef","statement":"x","fact_status":"observed","source_refs":["artifact://x"],"coverage":"complete","partial_result":true,"created_at":"2026-10-13T04:59:00Z"}]}]}}`,
			want:    "partial result",
		},
		{
			name:    "unknown revision zero",
			payload: `{"task":{"id":"tsk_p12","status":"active","updated_at":"2026-10-13T05:00:00Z","unknowns":[{"id":"unk_0123456789abcdef","question":"q","status":"open","revision":0,"opened_step_id":"inspect","opened_at":"2026-10-13T04:59:00Z"}]}}`,
			want:    "revision",
		},
		{
			name:    "resolved missing evidence",
			payload: `{"task":{"id":"tsk_p12","status":"active","updated_at":"2026-10-13T05:00:00Z","unknowns":[{"id":"unk_0123456789abcdef","question":"q","status":"resolved","revision":2,"opened_step_id":"inspect","resolution":"done","resolved_step_id":"verify","opened_at":"2026-10-13T04:59:00Z","resolved_at":"2026-10-13T05:00:00Z"}]}}`,
			want:    "resolution evidence",
		},
		{
			name:    "invalid evidence identity",
			payload: `{"task":{"id":"tsk_p12","status":"active","updated_at":"2026-10-13T05:00:00Z","steps":[{"id":"inspect","status":"completed","evidence_records":[{"id":"ev_not-server-owned","statement":"x","fact_status":"observed","source_refs":["artifact://x"],"coverage":"partial","created_at":"2026-10-13T04:59:00Z"}]}]}}`,
			want:    "ev_ identity",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseRuntimeTaskDetail("node1", "tsk_p12", fixturePayload(t, tc.payload))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestParseRuntimeTaskListPreservesUnknownCounts(t *testing.T) {
	payload := fixturePayload(t, `{
	  "tasks": [{
	    "id":"tsk_p12","title":"P12","goal":"g","status":"active","phase":"execute",
	    "review_status":"not_started","completed_step_count":1,"step_count":2,"ready_step_count":1,
	    "unknown_count":3,"open_unknown_count":2,"event_count":0,
	    "updated_at":"2026-10-13T05:00:00Z"
	  }]
	}`)
	tasks, err := parseRuntimeTaskList("node1", payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].UnknownCount != 3 || tasks[0].OpenUnknownCount != 2 {
		t.Fatalf("unknown counts lost: %#v", tasks)
	}
}
