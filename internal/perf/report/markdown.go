// Design: (none -- new tool, predates documentation)
package report

import (
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/perf"
)

// groupKey identifies a unique group of results for comparison.
type groupKey struct {
	Family  string
	ForceMP bool
}

type comparisonKey struct {
	DUTName string
	Family  string
	ForceMP bool
}

type selectedResult struct {
	result perf.Result
	index  int
}

// latestComparisonResults keeps the newest result for each DUT within a
// comparison table group. RFC3339 timestamps sort lexicographically, so string
// comparison is enough here.
func latestComparisonResults(results []perf.Result) []perf.Result {
	selected := make(map[comparisonKey]selectedResult)

	for i := range results {
		key := comparisonKey{
			DUTName: results[i].DUTName,
			Family:  results[i].Family,
			ForceMP: results[i].ForceMP,
		}

		prev, ok := selected[key]
		if !ok || results[i].Timestamp > prev.result.Timestamp ||
			(results[i].Timestamp == prev.result.Timestamp && i > prev.index) {
			selected[key] = selectedResult{result: results[i], index: i}
		}
	}

	ordered := make([]selectedResult, 0, len(selected))
	for key := range selected {
		ordered = append(ordered, selected[key])
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].index < ordered[j].index
	})

	deduped := make([]perf.Result, len(ordered))
	for i := range ordered {
		deduped[i] = ordered[i].result
	}

	return deduped
}

// groupResults groups results by Family + ForceMP combination.
func groupResults(results []perf.Result) map[groupKey][]perf.Result {
	results = latestComparisonResults(results)

	groups := make(map[groupKey][]perf.Result)

	for i := range results {
		key := groupKey{Family: results[i].Family, ForceMP: results[i].ForceMP}
		groups[key] = append(groups[key], results[i])
	}

	return groups
}

// sortedGroupKeys returns group keys in deterministic order.
func sortedGroupKeys(groups map[groupKey][]perf.Result) []groupKey {
	keys := make([]groupKey, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Family != keys[j].Family {
			return keys[i].Family < keys[j].Family
		}

		return !keys[i].ForceMP && keys[j].ForceMP
	})

	return keys
}

// sortByConvergence sorts results by ConvergenceMs ascending.
func sortByConvergence(results []perf.Result) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].ConvergenceMs < results[j].ConvergenceMs
	})
}

// Markdown writes a markdown comparison report of performance results.
// Results are grouped by Family + ForceMP and sorted by convergence time.
func Markdown(results []perf.Result, w io.Writer) error {
	groups := groupResults(results)
	keys := sortedGroupKeys(groups)

	for i, key := range keys {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil { //nolint:errcheck // output
				return fmt.Errorf("writing separator: %w", err)
			}
		}

		header := fmt.Sprintf("## %s", key.Family)
		if key.ForceMP {
			header += " [force-mp]"
		}

		if _, err := fmt.Fprintln(w, header); err != nil { //nolint:errcheck // output
			return fmt.Errorf("writing header: %w", err)
		}

		if _, err := fmt.Fprintln(w); err != nil { //nolint:errcheck // output
			return fmt.Errorf("writing newline: %w", err)
		}

		grouped := groups[key]
		sortByConvergence(grouped)

		if _, err := fmt.Fprintln(w, "| DUT | Version | Convergence | +/- | Throughput | +/- | p99 | +/- | Withdrawal | +/- |"); err != nil { //nolint:errcheck // output
			return fmt.Errorf("writing table header: %w", err)
		}

		if _, err := fmt.Fprintln(w, "|-----|---------|-------------|-----|------------|-----|-----|-----|------------|-----|"); err != nil { //nolint:errcheck // output
			return fmt.Errorf("writing table separator: %w", err)
		}

		for j := range grouped {
			r := &grouped[j]
			var tb textbuf.Buffer
			line := tb.Str("| ").Str(r.DUTName).Str(" | ").Str(r.DUTVersion).
				Str(" | ").Str(fmtMs(r.ConvergenceMs)).Str(" | ").Str(fmtMs(r.ConvergenceStddevMs)).
				Str(" | ").Str(fmtNum(r.ThroughputAvg)).Str(" | ").Str(fmtNum(r.ThroughputAvgStddev)).
				Str(" | ").Str(fmtMs(r.LatencyP99Ms)).Str(" | ").Str(fmtMs(r.LatencyP99StddevMs)).
				Str(" | ").Str(fmtMs(r.WithdrawalMs)).Str(" | ").Str(fmtMs(r.WithdrawalStddevMs)).Str(" |").String()
			if _, err := fmt.Fprintln(w, line); err != nil { //nolint:errcheck // output
				return fmt.Errorf("writing table row: %w", err)
			}
		}
	}

	return nil
}

// fmtNum formats an integer with comma grouping (e.g., 54112 -> "54,112").
func fmtNum(n int) string {
	if n < 0 {
		var tb textbuf.Buffer
		return tb.Byte('-').Str(fmtNum(-n)).String()
	}

	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}

	var b textbuf.Buffer

	remainder := len(s) % 3
	if remainder > 0 {
		b.Str(s[:remainder])
	}

	for i := remainder; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.Byte(',')
		}

		b.Str(s[i : i+3])
	}

	return b.String()
}

func fmtMs(n int) string {
	var tb textbuf.Buffer
	return tb.Str(fmtNum(n)).Str("ms").String()
}
