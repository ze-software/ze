// Design: (none -- new tool, predates documentation)

// Package report generates comparison and trend reports from benchmark results.
package report

import (
	"fmt"

	"github.com/ze-software/ze/internal/perf"
)

// ValidateResults checks every raw row before selection or report output.
// An empty slice is valid: it contains no measurement that can be misranked.
func ValidateResults(results []perf.Result) error {
	for i := range results {
		if err := results[i].ComparisonError(); err != nil {
			return fmt.Errorf("result %d: %w", i+1, err)
		}
	}
	return nil
}
