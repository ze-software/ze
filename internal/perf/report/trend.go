// Design: (none -- new tool, predates documentation)
package report

import (
	"fmt"
	"io"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/perf"
)

const regressionMarker = " **!!**"

// Trend writes a time-series trend table for tracking performance over time.
// Format must be "md" (markdown) or "html".
func Trend(results []perf.Result, w io.Writer, format string) error {
	if format == "md" {
		return trendMarkdown(results, w)
	}

	if format == "html" {
		return trendHTML(results, w)
	}

	return fmt.Errorf("unsupported trend format: %q (use \"md\" or \"html\")", format)
}

func trendMarkdown(results []perf.Result, w io.Writer) error {
	if _, err := fmt.Fprintln(w, "| Date | Version | Convergence | +/- | Throughput | +/- | p99 | +/- | Delta |"); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	if _, err := fmt.Fprintln(w, "|------|---------|-------------|-----|------------|-----|-----|-----|-------|"); err != nil {
		return fmt.Errorf("writing separator: %w", err)
	}

	for i := range results {
		date := extractDate(results[i].Timestamp)
		delta := ""

		if i > 0 {
			delta = convergenceDelta(results[i-1].ConvergenceMs, results[i].ConvergenceMs)
		}

		line := fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s | %s | %s |",
			date,
			results[i].DUTVersion,
			fmtMs(results[i].ConvergenceMs),
			fmtMs(results[i].ConvergenceStddevMs),
			fmtNum(results[i].ThroughputAvg),
			fmtNum(results[i].ThroughputAvgStddev),
			fmtMs(results[i].LatencyP99Ms),
			fmtMs(results[i].LatencyP99StddevMs),
			delta,
		)
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("writing row: %w", err)
		}
	}

	return nil
}

func trendHTML(results []perf.Result, w io.Writer) error {
	if _, err := fmt.Fprint(w, htmlHeader); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	if _, err := fmt.Fprintln(w, "<h2>Performance Trend</h2>"); err != nil {
		return fmt.Errorf("writing title: %w", err)
	}

	if _, err := fmt.Fprintln(w, "<table>"); err != nil {
		return fmt.Errorf("writing table open: %w", err)
	}

	if _, err := fmt.Fprintln(w, "<tr><th>Date</th><th>Version</th><th>Convergence</th><th>+/-</th><th>Throughput</th><th>+/-</th><th>p99</th><th>+/-</th><th>Delta</th></tr>"); err != nil {
		return fmt.Errorf("writing table header: %w", err)
	}

	for i := range results {
		date := extractDate(results[i].Timestamp)
		delta := ""

		if i > 0 {
			delta = convergenceDelta(results[i-1].ConvergenceMs, results[i].ConvergenceMs)
		}

		// Strip markdown bold markers for HTML
		delta = strings.ReplaceAll(delta, "**", "")
		deltaClass := ""

		if strings.Contains(delta, "!!") {
			deltaClass = " class=\"worst\""
		}

		if _, err := fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td%s>%s</td></tr>\n",
			date,
			results[i].DUTVersion,
			fmtMs(results[i].ConvergenceMs),
			fmtMs(results[i].ConvergenceStddevMs),
			fmtNum(results[i].ThroughputAvg),
			fmtNum(results[i].ThroughputAvgStddev),
			fmtMs(results[i].LatencyP99Ms),
			fmtMs(results[i].LatencyP99StddevMs),
			deltaClass,
			delta,
		); err != nil {
			return fmt.Errorf("writing table row: %w", err)
		}
	}

	if _, err := fmt.Fprintln(w, "</table>"); err != nil {
		return fmt.Errorf("writing table close: %w", err)
	}

	if _, err := fmt.Fprint(w, htmlFooter); err != nil {
		return fmt.Errorf("writing footer: %w", err)
	}

	return nil
}

// extractDate returns the date portion of a timestamp (YYYY-MM-DD).
func extractDate(timestamp string) string {
	if len(timestamp) >= 10 {
		return timestamp[:10]
	}

	return timestamp
}

// convergenceDelta computes the percentage change from previous to current convergence.
// Negative means improvement (faster). Positive means regression (slower).
// Regressions (>10%) are flagged with a warning marker.
func convergenceDelta(previous, current int) string {
	if previous == 0 {
		return "N/A"
	}

	pct := (float64(current) - float64(previous)) / float64(previous) * 100.0

	sign := ""
	if pct > 0 {
		sign = "+"
	}

	result := fmt.Sprintf("%s%.1f%%", sign, pct)

	if pct > 10.0 {
		result += regressionMarker
	}

	return result
}
