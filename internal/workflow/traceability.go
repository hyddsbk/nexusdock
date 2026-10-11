package workflow

import (
	"fmt"
	"strings"
)

const (
	maxTemplateRequirementRefs = 32
	maxStepRequirementRefs     = 12
	maxRequirementRefRunes     = 512
)

func normalizeRequirementRefs(label string, values []string, maxItems int) ([]string, error) {
	if len(values) > maxItems {
		return nil, fmt.Errorf("%s cannot exceed %d", label, maxItems)
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, ref := range values {
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" || trimmed != ref || strings.ContainsAny(ref, "\r\n") || len([]rune(ref)) > maxRequirementRefRunes {
			return nil, fmt.Errorf("%s has invalid requirement ref %q", label, ref)
		}
		if _, exists := seen[ref]; exists {
			return nil, fmt.Errorf("%s has duplicate requirement ref %q", label, ref)
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
	}
	return out, nil
}

func validateTraceabilityContract(requirementRefs []string, steps []Step) error {
	refs, err := normalizeRequirementRefs("template", requirementRefs, maxTemplateRequirementRefs)
	if err != nil {
		return err
	}
	if len(refs) == 0 {
		for _, step := range steps {
			if len(step.RequirementRefs) > 0 {
				return fmt.Errorf("step %s has requirement_refs but template requirement_refs is empty", step.ID)
			}
		}
		return nil
	}

	known := make(map[string]struct{}, len(refs))
	mapped := make(map[string]bool, len(refs))
	for _, ref := range refs {
		known[ref] = struct{}{}
	}
	for _, step := range steps {
		stepRefs, err := normalizeRequirementRefs("step "+step.ID, step.RequirementRefs, maxStepRequirementRefs)
		if err != nil {
			return err
		}
		for _, ref := range stepRefs {
			if _, ok := known[ref]; !ok {
				return fmt.Errorf("step %s references unknown requirement ref %q", step.ID, ref)
			}
			mapped[ref] = true
		}
	}
	for _, ref := range refs {
		if !mapped[ref] {
			return fmt.Errorf("requirement ref %q is not mapped to any step", ref)
		}
	}
	return nil
}
