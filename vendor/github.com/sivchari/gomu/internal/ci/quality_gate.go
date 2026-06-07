package ci

import (
	"fmt"

	"github.com/sivchari/gomu/internal/report"
)

// QualityGateResult represents the result of quality gate evaluation.
type QualityGateResult struct {
	Pass          bool    `json:"pass"`
	MutationScore float64 `json:"mutationScore"`
	Reason        string  `json:"reason"`
}

// QualityGateEvaluator evaluates quality gates.
type QualityGateEvaluator struct {
	enabled          bool
	minMutationScore float64
}

// NewQualityGateEvaluator creates a new quality gate evaluator.
func NewQualityGateEvaluator(enabled bool, minMutationScore float64) *QualityGateEvaluator {
	return &QualityGateEvaluator{
		enabled:          enabled,
		minMutationScore: minMutationScore,
	}
}

// Evaluate evaluates the quality gate against mutation testing results.
func (e *QualityGateEvaluator) Evaluate(summary *report.Summary) *QualityGateResult {
	if summary == nil || summary.TotalMutants == 0 {
		return &QualityGateResult{
			Pass:          true,
			MutationScore: 0.0,
			Reason:        "No mutants generated (skipped)",
		}
	}

	// Use the correctly calculated mutation score from statistics (excludes NOT_VIABLE mutants)
	mutationScore := summary.Statistics.Score

	if !e.enabled {
		return &QualityGateResult{
			Pass:          true,
			MutationScore: mutationScore,
			Reason:        "Quality gate disabled",
		}
	}

	// Check if score meets threshold
	if mutationScore >= e.minMutationScore {
		return &QualityGateResult{
			Pass:          true,
			MutationScore: mutationScore,
			Reason:        "Mutation score meets minimum threshold",
		}
	}

	return &QualityGateResult{
		Pass:          false,
		MutationScore: mutationScore,
		Reason:        fmt.Sprintf("Mutation score %.1f%% is below minimum threshold of %.1f%%", mutationScore, e.minMutationScore),
	}
}
