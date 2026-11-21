// VALIDATES: a spec's file bullets yield every back-quoted path of their path
// list, and none of the spans of their description (BulletPaths), so the
// source files SpecSourceFiles answers are every file a bullet names.
// PREVENTS: the design pages of a bullet's second and later files never being
// derived, and a symbol or a path the description cites read as a file the
// spec changes.
package speccitation

import (
	"slices"
	"testing"
)

// TestSpecSourceFilesTakesEveryPathOfABullet is the defect the design judge
// met: a bullet naming two files gave only the first. Method: Files to Modify
// holds a two-path bullet whose description cites a third file and a symbol,
// a bullet whose ` -- ` description cites a file, and one whose colon
// description does; Files to Create holds a bullet with a parenthesis between
// its paths, and one whose parenthesis cites a file and whose later file is
// written as a bare name beside the first. The answer is every path-list file,
// in order, the bare name resolved, and no description or parenthesis file.
func TestSpecSourceFilesTakesEveryPathOfABullet(t *testing.T) {
	text := "## Files to Modify\n" +
		"- `internal/le/spec/citation/anchors.go`, `internal/le/spec/design/specfile.go` - read every path, " +
		"as `internal/le/doc/readrecord/readrecord.go` reads `Sections`\n" +
		"- `internal/le/spec/design/documents.go` -- mirrors `internal/le/spec/design/check.go`\n" +
		"- `internal/le/spec/design/judge.go`: `internal/le/spec/design/records.go`\n\n" +
		"## Files to Create\n" +
		"- `internal/le/spec/citation/bullets_test.go` (the bullet tests), `internal/le/spec/design/documents_test.go` and its fixture\n" +
		"- `internal/le/spec/citation/anchors_test.go` (beside `internal/le/spec/design/check.go`), `declared_test.go`\n"
	want := []string{
		"internal/le/spec/citation/anchors.go", "internal/le/spec/design/specfile.go",
		"internal/le/spec/design/documents.go", "internal/le/spec/design/judge.go",
		"internal/le/spec/citation/bullets_test.go", "internal/le/spec/design/documents_test.go",
		"internal/le/spec/citation/anchors_test.go", "internal/le/spec/citation/declared_test.go",
	}
	if got := specSourceFiles(text); !slices.Equal(got, want) {
		t.Errorf("specSourceFiles answered\n  %q\nwant\n  %q", got, want)
	}
}

// bq back-quotes path, so a row naming a path the checkout may not hold
// carries no back-quoted path literal for the doc link check to resolve.
func bq(path string) string { return "`" + path + "`" }

// TestBulletPathsStopsAtTheDescription pins which spans of a bullet are its
// files. Method: one row per form the specs under plan/ write: the template's
// ` - `, ` -- `, an em dash, a colon after the last path, prose and a
// parenthesis between paths, prose before the separator, an indented item,
// two lines that are no file bullet, spans inside a parenthesis, slashless
// spans after a file and after a directory, and slashless spans with no path
// before them.
func TestBulletPathsStopsAtTheDescription(t *testing.T) {
	for _, row := range []struct {
		line string
		want []string
	}{
		{line: "- `a.go`, `b.go` - uses `c.go` and `Symbol`", want: []string{"a.go", "b.go"}},
		{line: "- `a.go` -- uses `c.go`", want: []string{"a.go"}},
		{line: "- `a.go` — uses `c.go`", want: []string{"a.go"}},
		{line: "- `a.go`: `Symbol`, `c.go`", want: []string{"a.go"}},
		{line: "- `a.go` (the monitor), `b.go`, and one file per rule: `c.go`, `d.go`", want: []string{"a.go", "b.go", "c.go", "d.go"}},
		{line: "- `a.go` and its users - use `c.go`", want: []string{"a.go"}},
		{line: "  - `a.go`, `b.go`", want: []string{"a.go", "b.go"}},
		{line: "- the prose names `a.go`", want: nil},
		{line: "| `a.go` | a table cell |", want: nil},
		// A span inside a parenthesis says why the file is listed, never a
		// file: the forms of spec-login-identity-for-looking-glass-and-gnmi.md
		// and spec-feature-maturity-declared.md.
		{
			line: "- " + bq("docs/guide/gnmi.md") + " (declared by " + bq("internal/component/gnmi/tls.go") + "), " +
				bq("docs/guide/authentication.md"),
			want: []string{"docs/guide/gnmi.md", "docs/guide/authentication.md"},
		},
		{
			line: "- " + bq("internal/le/site/datapages.go") + " (Design: " + bq("website/AI.md") + ") - " +
				bq("featureCard") + " drops " + bq("status"),
			want: []string{"internal/le/site/datapages.go"},
		},
		{line: "- `a.go` (uses (nested) `b.go`) - c", want: []string{"a.go"}},
		// A slashless span after a path is a file beside it, the form of
		// spec-host-resource-trend-alerts.md and the pki and eap specs; after
		// a directory it lies in that directory.
		{
			line: "- " + bq("internal/component/host/trend.go") + " (the monitor and the rule registry), " +
				bq("trend_persist.go") + ", " + bq("trend_doctor.go"),
			want: []string{"internal/component/host/trend.go", "internal/component/host/trend_persist.go",
				"internal/component/host/trend_doctor.go"},
		},
		{
			line: "- " + bq("internal/core/stats/") + ", " + bq("level.go"),
			want: []string{"internal/core/stats/", "internal/core/stats/level.go"},
		},
		// A slashless span with no path before it names a checkout-root file
		// and is answered as written.
		{line: "- `README.md`, `Makefile` - the root files", want: []string{"README.md", "Makefile"}},
	} {
		if got := BulletPaths(row.line); !slices.Equal(got, row.want) {
			t.Errorf("BulletPaths(%q) = %q, want %q", row.line, got, row.want)
		}
	}
}
