package agentdock

import "testing"

func TestParseRuntimeTaskDetailPreservesStepQualityAndTwoAxisReview(t *testing.T) {
	payload := fixturePayload(t, `{"task":{"id":"task_01","title":"Review","goal":"verify","status":"active","phase":"closeout","updated_at":"2026-10-11T00:00:00Z","steps":[{"id":"slice","title":"Ship slice","status":"completed","phase":"execute","completion_conditions":["Behavior passes."],"quality":{"kind":"vertical_slice","outcome":"One behavior works end-to-end.","dependency_reasons":{}}}],"final_review":{"status":"pass","summary":"verified","review_revision":"rev_two_axis","intent_spec":{"status":"pass","evidence":["acceptance passed"]},"engineering_policy":{"status":"pass","evidence":["policy passed"]}}}}`)
	detail, err := parseRuntimeTaskDetail("node1", "task_01", payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Steps) != 1 || detail.Steps[0].Quality == nil || detail.Steps[0].Quality.Kind != "vertical_slice" {
		t.Fatalf("missing step quality: %#v", detail.Steps)
	}
	if detail.FinalReview == nil || detail.FinalReview.IntentSpec == nil || detail.FinalReview.EngineeringPolicy == nil {
		t.Fatalf("missing two-axis review: %#v", detail.FinalReview)
	}
}
