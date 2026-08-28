// The report's own rendering, which is the half of the old subprocess test
// that matched on printed text (internal/le/consistency/consistency_test.go asserted on
// the tool's combined output).
//
// VALIDATES: the bare command renders the findings the way a person reads them
// -- grouped by check in the order the checks ran, one line per finding with
// its position, and a summary that agrees with the counts.
// PREVENTS: a report whose rendering and whose data disagree: a summary line
// that says zero errors over a list holding one, a group header whose count is
// not the number of rows under it, or a severity that is not colored
// differently from the other one.

package consistency

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// ansiRE matches an SGR escape sequence, so an assertion can be about the text
// without being about the palette.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string { return ansiRE.ReplaceAllString(s, "") }

// sample is a report holding both severities, both position forms (a line and
// a whole file), and two checks, so one rendering exercises every branch.
func sample() Report {
	return Report{
		Findings: []Finding{
			{Severity: SeverityError, Check: "json-kebab-case", File: "a.go", Line: 4, Message: "first"},
			{Severity: SeverityWarn, Check: "json-kebab-case", File: "b.go", Message: "second"},
			{Severity: SeverityWarn, Check: "cross-refs", File: "c.go", Line: 7, Message: "third"},
		},
		Errors:   1,
		Warnings: 2,
	}
}

// TestTextRendersTheWholeReport pins the rendering exactly. The gate's output
// is read by a person and diffed by a reviewer, so its shape is behavior.
func TestTextRendersTheWholeReport(t *testing.T) {
	want := "\n── json-kebab-case (2)\n" +
		"  ERROR a.go:4 — first\n" +
		"  WARN b.go — second\n" +
		"\n── cross-refs (1)\n" +
		"  WARN c.go:7 — third\n" +
		"\nSummary: 1 errors, 2 warnings\n"

	got := plain(sample().Text())

	if got != want {
		t.Errorf("the report rendered\n%q\nwant\n%q", got, want)
	}
}

// TestTextColorsBySeverity pins that the two severities are distinguishable at
// a glance, which is the whole reason the rendering is not a table.
func TestTextColorsBySeverity(t *testing.T) {
	text := sample().Text()

	errorSpan := spanBefore(text, "ERROR")
	warnSpan := spanBefore(text, "WARN")
	if errorSpan == "" || warnSpan == "" {
		t.Fatalf("a severity was rendered with no color: %q", text)
	}
	if errorSpan == warnSpan {
		t.Errorf("both severities are colored %q, so the report reads as one severity", errorSpan)
	}
}

// spanBefore answers the escape sequence immediately before the first
// occurrence of needle.
func spanBefore(text, needle string) string {
	before, _, found := strings.Cut(text, needle)
	if !found {
		return ""
	}
	spans := ansiRE.FindAllString(before, -1)
	if len(spans) == 0 {
		return ""
	}
	return spans[len(spans)-1]
}

// TestTextSaysPassOnlyWhenEmpty is the assertion the old subprocess test made
// negatively: a report holding findings must never print the pass line.
func TestTextSaysPassOnlyWhenEmpty(t *testing.T) {
	const pass = "✓ All consistency checks passed"

	if got := plain(Report{Findings: []Finding{}}.Text()); got != pass+"\n" {
		t.Errorf("an empty report rendered %q, want %q", got, pass+"\n")
	}
	if got := sample().Text(); strings.Contains(got, pass) {
		t.Errorf("a report holding 3 findings printed the pass line:\n%s", got)
	}
}

// TestReportIsStructuredData is AC-7 at the payload: the answer is data with
// one row set in it, so the engine can render it and the row operators have
// something to act on. A tool that answered finished text would fail here.
func TestReportIsStructuredData(t *testing.T) {
	raw, err := json.Marshal(sample())
	if err != nil {
		t.Fatalf("the report does not encode: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("the report encoded to something that is not an object: %v", err)
	}

	rows, ok := decoded["findings"].([]any)
	if !ok || len(rows) != 3 {
		t.Fatalf("findings is not a row set of 3: %v", decoded["findings"])
	}
	first, ok := rows[0].(map[string]any)
	if !ok {
		t.Fatalf("a finding is not an object: %v", rows[0])
	}
	for _, key := range []string{"severity", "check", "file", "line", "message"} {
		if _, held := first[key]; !held {
			t.Errorf("a finding carries no %q key: %v", key, first)
		}
	}
	// A whole-file finding says so by carrying NO line, rather than by carrying
	// a zero that reads like a position.
	second, ok := rows[1].(map[string]any)
	if !ok {
		t.Fatalf("a finding is not an object: %v", rows[1])
	}
	if _, held := second["line"]; held {
		t.Errorf("a file-level finding carries a line: %v", second)
	}
}
