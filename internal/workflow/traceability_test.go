package workflow

import (
	"strings"
	"testing"
)

func traceabilityTemplate() Template {
	tpl := testTemplate("development.traceability", "1.0.0")
	tpl.RequirementRefs = []string{"spec://feature#R1", "spec://feature#R2"}
	tpl.Steps = []Step{
		{ID: "build", Title: "Build", Phase: "execute", CompletionConditions: []string{"R1 works."}, RequirementRefs: []string{"spec://feature#R1"}},
		{ID: "verify", Title: "Verify", Phase: "verify", CompletionConditions: []string{"R2 is verified."}, RequirementRefs: []string{"spec://feature#R2"}},
	}
	return tpl
}

func TestWorkflowTraceabilityContractRequiresCompleteRequirementMapping(t *testing.T) {
	if err := validateTemplate(traceabilityTemplate()); err != nil {
		t.Fatalf("valid traceability template rejected: %v", err)
	}

	unmapped := traceabilityTemplate()
	unmapped.Steps[1].RequirementRefs = nil
	if err := validateTemplate(unmapped); err == nil || !strings.Contains(err.Error(), "not mapped to any step") {
		t.Fatalf("unmapped requirement err=%v", err)
	}

	unknown := traceabilityTemplate()
	unknown.Steps[1].RequirementRefs = []string{"spec://feature#R3"}
	if err := validateTemplate(unknown); err == nil || !strings.Contains(err.Error(), "unknown requirement ref") {
		t.Fatalf("unknown requirement err=%v", err)
	}
}

func TestWorkflowTraceabilityPersistsThroughRegistry(t *testing.T) {
	registry := NewRegistry(t.TempDir())
	published, err := registry.Publish(traceabilityTemplate())
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := registry.Get(published.ID, published.Version)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.RequirementRefs) != 2 || len(loaded.Steps[0].RequirementRefs) != 1 || loaded.Steps[0].RequirementRefs[0] != "spec://feature#R1" {
		t.Fatalf("traceability fields were not preserved: %#v", loaded)
	}
	if loaded.Hash != templateHash(loaded) {
		t.Fatal("traceability fields are not covered by template hash")
	}
}
