package specmeta

import "testing"

// templateSpec reproduces the shape plan/TEMPLATE.md produces: a title, then a
// six-line HTML authoring comment, then the metadata table. That comment pushes
// the Status row to line 12, which is what made 12 specs invisible to the
// inventory when the parser scanned a fixed 10-line window.
const templateSpec = `# Spec: example

<!-- Authoring note line 1
     line 2
     line 3
     line 4
     line 5 -->

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | 2/5 |
| Updated | 2026-08-18 |

Recovery after compaction: ` + "`.claude/rules/post-compaction.md`" + `.

## Task

Some prose.

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|-----------|-------|----------|------------|--------|
| A-1 | something | a basis | a cost | a check | unvalidated |
`

func TestRowsFindsTablePastAFixedWindow(t *testing.T) {
	rows, found := Rows(templateSpec)
	if !found {
		t.Fatal("Rows reported no metadata table for a spec written from the template")
	}
	for field, want := range map[string]string{
		"Status":  "ready",
		"Scope":   "protocol",
		"Depends": "-",
		"Phase":   "2/5",
		"Updated": "2026-08-18",
	} {
		if got := Field(rows, field); got != want {
			t.Errorf("Field(%q) = %q, want %q", field, got, want)
		}
	}
}

// The Assumptions table further down the file ends its header with "| Status |".
// A parser that scanned the whole file would read "unvalidated" from its body
// row, so the scan must stop at the first "## " heading.
func TestRowsStopsAtTheFirstHeading(t *testing.T) {
	rows, _ := Rows(templateSpec)
	if got := Field(rows, "Status"); got != "ready" {
		t.Errorf("Status = %q, want %q: the scan ran past the first heading", got, "ready")
	}
	// The last row kept must be the table's own last row. Anything after it
	// came from past the heading.
	if last := rows[len(rows)-1]; last != "| Updated | 2026-08-18 |" {
		t.Errorf("last row = %q, want the Updated row: the scan ran past the table", last)
	}
	for _, line := range rows {
		if Field([]string{line}, "A-1") != "" {
			t.Errorf("row %q came from the Assumptions table", line)
		}
	}
}

// A spec with no metadata table must be distinguishable from one whose table
// omits a field. Reporting both as "unknown" dresses a zero-information answer
// as data.
func TestRowsReportsAMissingTable(t *testing.T) {
	if rows, found := Rows("# Spec: example\n\nNo table here.\n\n## Task\n"); found {
		t.Errorf("Rows found a table where there is none: %q", rows)
	}
	rows, found := Rows("| Field | Value |\n|---|---|\n| Scope | cli |\n")
	if !found {
		t.Fatal("Rows reported no table for a spec that has one")
	}
	if got := Field(rows, "Status"); got != "" {
		t.Errorf("Field(Status) = %q, want \"\" when the table omits the row", got)
	}
}
