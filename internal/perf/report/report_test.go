package report

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/perf"
)

func threeResults() []perf.Result {
	return []perf.Result{
		{
			DUTName:             "ze",
			DUTVersion:          "0.1.0",
			Family:              "ipv4/unicast",
			ForceMP:             false,
			Timestamp:           "2026-03-20T10:00:00Z",
			Routes:              100000,
			RoutesSent:          100000,
			RoutesReceived:      100000,
			RoutesLost:          0,
			ConvergenceMs:       1847,
			ConvergenceStddevMs: 52,
			ThroughputAvg:       54112,
			ThroughputAvgStddev: 1200,
			ThroughputPeak:      62000,
			LatencyP50Ms:        2,
			LatencyP90Ms:        5,
			LatencyP99Ms:        12,
			LatencyP99StddevMs:  2,
			LatencyMaxMs:        45,
		},
		{
			DUTName:             "gobgp",
			DUTVersion:          "3.25.0",
			Family:              "ipv4/unicast",
			ForceMP:             false,
			Timestamp:           "2026-03-20T10:05:00Z",
			Routes:              100000,
			RoutesSent:          100000,
			RoutesReceived:      100000,
			RoutesLost:          0,
			ConvergenceMs:       3200,
			ConvergenceStddevMs: 110,
			ThroughputAvg:       31250,
			ThroughputAvgStddev: 800,
			ThroughputPeak:      38000,
			LatencyP50Ms:        3,
			LatencyP90Ms:        8,
			LatencyP99Ms:        22,
			LatencyP99StddevMs:  4,
			LatencyMaxMs:        78,
		},
		{
			DUTName:             "frr",
			DUTVersion:          "9.1",
			Family:              "ipv4/unicast",
			ForceMP:             false,
			Timestamp:           "2026-03-20T10:10:00Z",
			Routes:              100000,
			RoutesSent:          100000,
			RoutesReceived:      100000,
			RoutesLost:          0,
			ConvergenceMs:       11800,
			ConvergenceStddevMs: 450,
			ThroughputAvg:       8474,
			ThroughputAvgStddev: 300,
			ThroughputPeak:      11000,
			LatencyP50Ms:        12,
			LatencyP90Ms:        35,
			LatencyP99Ms:        89,
			LatencyP99StddevMs:  15,
			LatencyMaxMs:        210,
		},
	}
}

// VALIDATES: "Comparison table from multiple results".
// PREVENTS: Missing DUT names or unsorted output in markdown report.
func TestMarkdownReport(t *testing.T) {
	results := threeResults()
	var buf bytes.Buffer

	err := Markdown(results, &buf)
	if err != nil {
		t.Fatalf("Markdown() error: %v", err)
	}

	out := buf.String()

	// All 3 DUT names present
	for _, name := range []string{"ze", "gobgp", "frr"} {
		if !strings.Contains(out, name) {
			t.Errorf("output missing DUT name %q", name)
		}
	}

	// Table header
	if !strings.Contains(out, "DUT") {
		t.Error("output missing table header 'DUT'")
	}

	// Convergence values present (with commas)
	if !strings.Contains(out, "1,847") {
		t.Error("output missing formatted convergence value '1,847'")
	}

	// Table must have header separator row (CommonMark requirement).
	if !strings.Contains(out, "|---") {
		t.Error("output missing table header separator '|---' (CommonMark)")
	}

	// Sorted by convergence: ze (1847) before gobgp (3200) before frr (11800)
	zePos := strings.Index(out, "| ze")
	gobgpPos := strings.Index(out, "| gobgp")
	frrPos := strings.Index(out, "| frr")

	if zePos < 0 || gobgpPos < 0 || frrPos < 0 {
		t.Fatal("could not find all DUT table rows")
	}

	if zePos > gobgpPos || gobgpPos > frrPos {
		t.Errorf("results not sorted by convergence: ze@%d gobgp@%d frr@%d", zePos, gobgpPos, frrPos)
	}
}

// VALIDATES: "Self-contained HTML output"
// PREVENTS: External resource dependencies in HTML report.
func TestHTMLReport(t *testing.T) {
	results := threeResults()[:2] // ze + gobgp
	var buf bytes.Buffer

	err := HTML(results, &buf)
	if err != nil {
		t.Fatalf("HTML() error: %v", err)
	}

	out := buf.String()

	for _, tag := range []string{"<html>", "<style>", "<svg ", "<table>", "</html>"} {
		if !strings.Contains(out, tag) {
			t.Errorf("output missing %q", tag)
		}
	}

	for _, name := range []string{"ze", "gobgp"} {
		if !strings.Contains(out, name) {
			t.Errorf("output missing DUT name %q", name)
		}
	}
}

// VALIDATES: "Trend table with delta column"
// PREVENTS: Missing dates or delta values in trend report.
func TestTrendMarkdown(t *testing.T) {
	results := []perf.Result{
		{
			DUTName:             "ze",
			DUTVersion:          "0.1.0",
			Family:              "ipv4/unicast",
			Timestamp:           "2026-03-18T10:00:00Z",
			ConvergenceMs:       2000,
			ConvergenceStddevMs: 50,
			ThroughputAvg:       50000,
			ThroughputAvgStddev: 1000,
			LatencyP99Ms:        15,
			LatencyP99StddevMs:  3,
		},
		{
			DUTName:             "ze",
			DUTVersion:          "0.2.0",
			Family:              "ipv4/unicast",
			Timestamp:           "2026-03-19T10:00:00Z",
			ConvergenceMs:       1900,
			ConvergenceStddevMs: 45,
			ThroughputAvg:       52000,
			ThroughputAvgStddev: 900,
			LatencyP99Ms:        14,
			LatencyP99StddevMs:  2,
		},
		{
			DUTName:             "ze",
			DUTVersion:          "0.3.0",
			Family:              "ipv4/unicast",
			Timestamp:           "2026-03-20T10:00:00Z",
			ConvergenceMs:       1850,
			ConvergenceStddevMs: 40,
			ThroughputAvg:       54000,
			ThroughputAvgStddev: 800,
			LatencyP99Ms:        12,
			LatencyP99StddevMs:  2,
		},
	}

	var buf bytes.Buffer

	err := Trend(results, &buf, "md")
	if err != nil {
		t.Fatalf("Trend() error: %v", err)
	}

	out := buf.String()

	// All 3 dates present
	for _, date := range []string{"2026-03-18", "2026-03-19", "2026-03-20"} {
		if !strings.Contains(out, date) {
			t.Errorf("output missing date %q", date)
		}
	}

	// Delta column present
	if !strings.Contains(out, "Delta") {
		t.Error("output missing Delta column header")
	}

	// Should contain percentage values (negative = improvement for convergence)
	if !strings.Contains(out, "%") {
		t.Error("output missing percentage delta values")
	}
}

// VALIDATES: "Results grouped by family"
// PREVENTS: Mixed family data in single table.
func TestMarkdownGroupsByFamily(t *testing.T) {
	results := []perf.Result{
		{
			DUTName:        "ze",
			DUTVersion:     "0.1.0",
			Family:         "ipv4/unicast",
			ConvergenceMs:  1847,
			ThroughputAvg:  54112,
			LatencyP99Ms:   12,
			RoutesReceived: 100000,
		},
		{
			DUTName:        "ze",
			DUTVersion:     "0.1.0",
			Family:         "ipv6/unicast",
			ConvergenceMs:  2100,
			ThroughputAvg:  48000,
			LatencyP99Ms:   15,
			RoutesReceived: 100000,
		},
		{
			DUTName:        "ze",
			DUTVersion:     "0.1.0",
			Family:         "ipv4/unicast",
			ForceMP:        true,
			ConvergenceMs:  1900,
			ThroughputAvg:  52000,
			LatencyP99Ms:   13,
			RoutesReceived: 100000,
		},
	}

	var buf bytes.Buffer

	err := Markdown(results, &buf)
	if err != nil {
		t.Fatalf("Markdown() error: %v", err)
	}

	out := buf.String()

	// Separate headers for each family group
	if !strings.Contains(out, "## ipv4/unicast") {
		t.Error("missing header for ipv4/unicast")
	}

	if !strings.Contains(out, "## ipv6/unicast") {
		t.Error("missing header for ipv6/unicast")
	}

	// force-mp group should have annotation
	if !strings.Contains(out, "force-mp") {
		t.Error("missing force-mp annotation")
	}
}

// VALIDATES: "comparison outputs keep the latest result per DUT in each group"
// PREVENTS: duplicated DUT rows in markdown, HTML, and performance.md reports.
func TestComparisonOutputsKeepLatestResultPerDUT(t *testing.T) {
	results := []perf.Result{
		{
			DUTName:             "ze",
			DUTVersion:          "0.2.0",
			Family:              "ipv4/unicast",
			Timestamp:           "2026-03-20T11:00:00Z",
			ConvergenceMs:       300,
			ConvergenceStddevMs: 1,
			ThroughputAvg:       1000,
			ThroughputAvgStddev: 10,
			LatencyP50Ms:        100,
			LatencyP99Ms:        200,
			LatencyP99StddevMs:  5,
			LatencyMaxMs:        210,
			Routes:              100000,
			RoutesSent:          100000,
			RoutesReceived:      100000,
			Repeat:              3,
			WarmupRuns:          1,
		},
		{
			DUTName:             "bird",
			DUTVersion:          "2.0.0",
			Family:              "ipv4/unicast",
			Timestamp:           "2026-03-20T10:30:00Z",
			ConvergenceMs:       250,
			ConvergenceStddevMs: 2,
			ThroughputAvg:       1100,
			ThroughputAvgStddev: 11,
			LatencyP50Ms:        90,
			LatencyP99Ms:        180,
			LatencyP99StddevMs:  4,
			LatencyMaxMs:        190,
			Routes:              100000,
			RoutesSent:          100000,
			RoutesReceived:      100000,
			Repeat:              3,
			WarmupRuns:          1,
		},
		{
			DUTName:             "ze",
			DUTVersion:          "0.1.0",
			Family:              "ipv4/unicast",
			Timestamp:           "2026-03-20T10:00:00Z",
			ConvergenceMs:       400,
			ConvergenceStddevMs: 3,
			ThroughputAvg:       900,
			ThroughputAvgStddev: 12,
			LatencyP50Ms:        110,
			LatencyP99Ms:        220,
			LatencyP99StddevMs:  6,
			LatencyMaxMs:        230,
			Routes:              100000,
			RoutesSent:          100000,
			RoutesReceived:      100000,
			Repeat:              3,
			WarmupRuns:          1,
		},
	}

	tests := []struct {
		name      string
		render    func([]perf.Result, io.Writer) error
		rowPrefix string
		want      string
		dontWant  string
		count     string
	}{
		{
			name:      "markdown",
			render:    Markdown,
			rowPrefix: "| ze |",
			want:      "| ze | 0.2.0 | 300ms |",
			dontWant:  "| ze | 0.1.0 | 400ms |",
			count:     "| ze |",
		},
		{
			name:      "html",
			render:    HTML,
			rowPrefix: "<tr><td>ze</td>",
			want:      "<tr><td>ze</td><td>0.2.0</td>",
			dontWant:  "<tr><td>ze</td><td>0.1.0</td>",
			count:     "<tr><td>ze</td>",
		},
		{
			name:      "performance doc",
			render:    PerformanceDoc,
			rowPrefix: "| ze |",
			want:      "| ze | 300ms | 1ms | 1,000 | 10 | 100ms | 200ms | 5ms | 210ms | 0 |",
			dontWant:  "| ze | 400ms | 3ms | 900 | 12 | 110ms | 220ms | 6ms | 230ms | 0 |",
			count:     "| ze |",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			if err := tt.render(results, &buf); err != nil {
				t.Fatalf("render error: %v", err)
			}

			out := buf.String()

			if got := strings.Count(out, tt.count); got != 1 {
				t.Fatalf("expected exactly one ze row, got %d\n%s", got, out)
			}

			if !strings.Contains(out, tt.want) {
				t.Fatalf("missing latest ze result %q\n%s", tt.want, out)
			}

			if strings.Contains(out, tt.dontWant) {
				t.Fatalf("found stale ze result %q\n%s", tt.dontWant, out)
			}

			if !strings.Contains(out, tt.rowPrefix) {
				t.Fatalf("missing ze row prefix %q\n%s", tt.rowPrefix, out)
			}
		})
	}

}
