package workflow

import (
	"errors"
	"fmt"
)

func isDiagnosticStepKind(kind string) bool {
	switch kind {
	case StepKindDiagnosticFeedbackLoop, StepKindDiagnosticReproduction, StepKindDiagnosticHypothesis, StepKindDiagnosticValidation, StepKindDiagnosticRegression:
		return true
	default:
		return false
	}
}

func validateDiagnosticStepFlow(steps []Step, byID map[string]Step) error {
	var feedbackLoops, reproductions, hypotheses, validations, regressions []Step
	for _, step := range steps {
		if step.Quality == nil || !isDiagnosticStepKind(step.Quality.Kind) {
			continue
		}
		switch step.Quality.Kind {
		case StepKindDiagnosticFeedbackLoop:
			feedbackLoops = append(feedbackLoops, step)
		case StepKindDiagnosticReproduction:
			reproductions = append(reproductions, step)
		case StepKindDiagnosticHypothesis:
			hypotheses = append(hypotheses, step)
		case StepKindDiagnosticValidation:
			validations = append(validations, step)
		case StepKindDiagnosticRegression:
			regressions = append(regressions, step)
		}
	}
	total := len(feedbackLoops) + len(reproductions) + len(hypotheses) + len(validations) + len(regressions)
	if total == 0 {
		return nil
	}
	if len(feedbackLoops) != 1 || len(reproductions) != 1 || len(regressions) != 1 || len(hypotheses) == 0 || len(validations) == 0 {
		return errors.New("diagnostic feedback loop requires exactly one feedback_loop, one reproduction, one regression, and at least one hypothesis and validation step")
	}

	feedback := feedbackLoops[0]
	repro := reproductions[0]
	regression := regressions[0]
	if !stepDependsTransitively(byID, repro.ID, feedback.ID, map[string]bool{}) {
		return fmt.Errorf("diagnostic reproduction step %s must depend on feedback loop %s", repro.ID, feedback.ID)
	}

	validationCountByHypothesis := make(map[string]int, len(hypotheses))
	for _, hypothesis := range hypotheses {
		if !stepDependsTransitively(byID, hypothesis.ID, repro.ID, map[string]bool{}) {
			return fmt.Errorf("diagnostic hypothesis step %s must depend on reproduction %s", hypothesis.ID, repro.ID)
		}
	}
	for _, validation := range validations {
		ancestorHypotheses := make([]string, 0, 2)
		for _, hypothesis := range hypotheses {
			if stepDependsTransitively(byID, validation.ID, hypothesis.ID, map[string]bool{}) {
				ancestorHypotheses = append(ancestorHypotheses, hypothesis.ID)
			}
		}
		if len(ancestorHypotheses) != 1 {
			return fmt.Errorf("diagnostic validation step %s must depend on exactly one falsifiable hypothesis; found %d", validation.ID, len(ancestorHypotheses))
		}
		validationCountByHypothesis[ancestorHypotheses[0]]++
		if !stepDependsTransitively(byID, regression.ID, validation.ID, map[string]bool{}) {
			return fmt.Errorf("diagnostic regression step %s must depend on validation %s", regression.ID, validation.ID)
		}
	}
	for _, hypothesis := range hypotheses {
		if validationCountByHypothesis[hypothesis.ID] == 0 {
			return fmt.Errorf("diagnostic hypothesis step %s requires at least one downstream single-variable validation", hypothesis.ID)
		}
	}
	return nil
}
