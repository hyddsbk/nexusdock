package workflow

import (
	"strings"
	"testing"
)

func workflowQuality(kind, outcome, group string, reasons map[string]string) *StepQuality {
	return &StepQuality{Kind: kind, Outcome: outcome, RefactorGroup: group, DependencyReasons: reasons}
}

func qualityTemplate(steps []Step) Template {
	return Template{ID: "quality.template", Version: "1.0.0", Title: "Quality template", CompletionConditions: []string{"verified"}, Steps: steps}
}

func TestStepQualityContractAcceptsTracerBulletTemplate(t *testing.T) {
	template := qualityTemplate([]Step{
		{ID: "inspect", Title: "Inspect seam", Phase: "check", CompletionConditions: []string{"Seam identified."}, Quality: workflowQuality(StepKindDiscovery, "A concrete seam is identified.", "", nil)},
		{ID: "slice", Title: "Ship slice", Phase: "execute", DependsOn: []string{"inspect"}, CompletionConditions: []string{"Behavior passes."}, Quality: workflowQuality(StepKindVerticalSlice, "One complete behavior works end-to-end.", "", map[string]string{"inspect": "The seam must be known first."})},
		{ID: "verify", Title: "Verify", Phase: "verify", DependsOn: []string{"slice"}, CompletionConditions: []string{"Regression passes."}, Quality: workflowQuality(StepKindVerification, "Independent evidence verifies the behavior.", "", map[string]string{"slice": "Verification needs the completed behavior."})},
	})
	if err := validateTemplate(template); err != nil {
		t.Fatal(err)
	}
	h1 := templateHash(template)
	template.Steps[1].Quality.Outcome = "A different complete behavior works end-to-end."
	h2 := templateHash(template)
	if h1 == h2 {
		t.Fatal("quality metadata did not affect stable template hash")
	}
}

func TestStepQualityContractRejectsPartialReasonsAndRedundantEdges(t *testing.T) {
	tests := []struct {
		name  string
		steps []Step
		want  string
	}{
		{"partial", []Step{
			{ID: "a", Title: "A", Phase: "check", CompletionConditions: []string{"A done."}, Quality: workflowQuality(StepKindDiscovery, "A known.", "", nil)},
			{ID: "b", Title: "B", Phase: "execute", DependsOn: []string{"a"}, CompletionConditions: []string{"B done."}},
		}, "partial"},
		{"missing_reason", []Step{
			{ID: "a", Title: "A", Phase: "check", CompletionConditions: []string{"A done."}, Quality: workflowQuality(StepKindDiscovery, "A known.", "", nil)},
			{ID: "b", Title: "B", Phase: "execute", DependsOn: []string{"a"}, CompletionConditions: []string{"B done."}, Quality: workflowQuality(StepKindVerticalSlice, "B works.", "", nil)},
		}, "explain every dependency edge"},
		{"redundant", []Step{
			{ID: "a", Title: "A", Phase: "check", CompletionConditions: []string{"A done."}, Quality: workflowQuality(StepKindDiscovery, "A known.", "", nil)},
			{ID: "b", Title: "B", Phase: "execute", DependsOn: []string{"a"}, CompletionConditions: []string{"B done."}, Quality: workflowQuality(StepKindVerticalSlice, "B works.", "", map[string]string{"a": "B needs A."})},
			{ID: "c", Title: "C", Phase: "verify", DependsOn: []string{"a", "b"}, CompletionConditions: []string{"C done."}, Quality: workflowQuality(StepKindVerification, "C verifies B.", "", map[string]string{"a": "claimed blocker", "b": "C needs B."})},
		}, "redundant dependency a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTemplate(qualityTemplate(tt.steps))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err=%v want %q", err, tt.want)
			}
		})
	}
}

func TestStepQualityContractValidatesWideRefactorGroup(t *testing.T) {
	steps := []Step{
		{ID: "expand", Title: "Expand", Phase: "execute", CompletionConditions: []string{"Both forms work."}, Quality: workflowQuality(StepKindWideRefactorExpand, "New form coexists with old form.", "rename", nil)},
		{ID: "migrate-a", Title: "Migrate A", Phase: "execute", DependsOn: []string{"expand"}, CompletionConditions: []string{"A migrated."}, Quality: workflowQuality(StepKindWideRefactorMigrate, "A uses new form.", "rename", map[string]string{"expand": "Compatibility must exist."})},
		{ID: "migrate-b", Title: "Migrate B", Phase: "execute", DependsOn: []string{"expand"}, CompletionConditions: []string{"B migrated."}, Quality: workflowQuality(StepKindWideRefactorMigrate, "B uses new form.", "rename", map[string]string{"expand": "Compatibility must exist."})},
		{ID: "contract", Title: "Contract", Phase: "verify", DependsOn: []string{"migrate-a", "migrate-b"}, CompletionConditions: []string{"Old form removed."}, Quality: workflowQuality(StepKindWideRefactorContract, "Old form is removed after all callers migrate.", "rename", map[string]string{"migrate-a": "A must migrate first.", "migrate-b": "B must migrate first."})},
	}
	if err := validateTemplate(qualityTemplate(steps)); err != nil {
		t.Fatal(err)
	}
	bad := append([]Step(nil), steps...)
	bad[3].DependsOn = []string{"migrate-a"}
	bad[3].Quality = workflowQuality(StepKindWideRefactorContract, "Remove old form.", "rename", map[string]string{"migrate-a": "A first."})
	if err := validateTemplate(qualityTemplate(bad)); err == nil || !strings.Contains(err.Error(), "must directly depend on migrate step migrate-b") {
		t.Fatalf("err=%v", err)
	}
}

func diagnosticWorkflowSteps() []Step {
	return []Step{
		{ID: "red-loop", Title: "Establish red feedback loop", Phase: "check", CompletionConditions: []string{"The bug is deterministically red."}, Quality: workflowQuality(StepKindDiagnosticFeedbackLoop, "A fast agent-runnable check fails on the target bug.", "", nil)},
		{ID: "repro", Title: "Minimize reproduction", Phase: "check", DependsOn: []string{"red-loop"}, CompletionConditions: []string{"The smallest useful reproduction remains red."}, Quality: workflowQuality(StepKindDiagnosticReproduction, "A minimized reproducer isolates the symptom.", "", map[string]string{"red-loop": "The original failure must be observable before minimization."})},
		{ID: "hypothesis", Title: "State falsifiable hypothesis", Phase: "check", DependsOn: []string{"repro"}, CompletionConditions: []string{"A causal claim and falsifier are explicit."}, Quality: workflowQuality(StepKindDiagnosticHypothesis, "The proposed cause can be disproved by a concrete observation.", "", map[string]string{"repro": "Hypotheses are formed only after the minimal reproduction is established."})},
		{ID: "validate", Title: "Validate one variable", Phase: "execute", DependsOn: []string{"hypothesis"}, CompletionConditions: []string{"Exactly one named variable is changed and measured."}, Quality: workflowQuality(StepKindDiagnosticValidation, "A single-variable experiment supports or refutes the hypothesis.", "", map[string]string{"hypothesis": "The experiment tests the stated falsifiable hypothesis."})},
		{ID: "regression", Title: "Run regression", Phase: "verify", DependsOn: []string{"validate"}, CompletionConditions: []string{"The minimal reproduction is green after the supported change."}, Quality: workflowQuality(StepKindDiagnosticRegression, "The exact minimized reproducer is rerun and passes.", "", map[string]string{"validate": "Regression follows the diagnostic experiment."})},
	}
}

func TestStepQualityContractValidatesDiagnosticFeedbackLoop(t *testing.T) {
	steps := diagnosticWorkflowSteps()
	if err := validateTemplate(qualityTemplate(steps)); err != nil {
		t.Fatalf("valid diagnostic template rejected: %v", err)
	}

	partial := steps[:4]
	if err := validateTemplate(qualityTemplate(partial)); err == nil || !strings.Contains(err.Error(), "requires exactly one feedback_loop") {
		t.Fatalf("partial diagnostic template err=%v", err)
	}

	ambiguous := diagnosticWorkflowSteps()
	secondHypothesis := Step{
		ID: "hypothesis-b", Title: "Second hypothesis", Phase: "check", DependsOn: []string{"repro"},
		CompletionConditions: []string{"A second falsifiable causal claim is explicit."},
		Quality:              workflowQuality(StepKindDiagnosticHypothesis, "A second cause can be falsified.", "", map[string]string{"repro": "It uses the same minimal reproduction."}),
	}
	ambiguous = append(ambiguous[:3], append([]Step{secondHypothesis}, ambiguous[3:]...)...)
	ambiguous[4].DependsOn = []string{"hypothesis", "hypothesis-b"}
	ambiguous[4].Quality.DependencyReasons = map[string]string{"hypothesis": "Test the first cause.", "hypothesis-b": "Also test the second cause."}
	ambiguous[5].DependsOn = []string{"validate"}
	ambiguous[5].Quality.DependencyReasons = map[string]string{"validate": "Regression follows validation."}
	if err := validateTemplate(qualityTemplate(ambiguous)); err == nil || !strings.Contains(err.Error(), "exactly one falsifiable hypothesis") {
		t.Fatalf("ambiguous diagnostic template err=%v", err)
	}
}
