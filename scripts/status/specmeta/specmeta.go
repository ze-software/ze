// Design: (none -- build tool) -- metadata table parser for plan/spec-*.md
//
// Package specmeta reads the metadata table at the top of a spec file.
//
// The logic lives here rather than in spec_status.go so it is importable by a
// test: spec_status.go is //go:build ignore and cannot be compiled into a test
// package. The specbucket package next to it exists for the same reason.
package specmeta

import (
	"regexp"
	"strings"
)

// headerRE matches the header row of a spec's metadata table.
var headerRE = regexp.MustCompile(`^\|\s*Field\s*\|\s*Value\s*\|`)

// Rows returns the body rows of a spec's metadata table and reports whether
// that table was found.
//
// The scan is anchored on the "| Field | Value |" header row rather than on a
// line count, because plan/TEMPLATE.md opens with a six-line authoring comment
// that pushes the table past any fixed window. It stops at the first "## "
// heading and at the first line that leaves the table, so the trailing header
// rows of the Assumptions, TDD and Interop tables further down the file are
// never matched.
func Rows(content string) ([]string, bool) {
	var rows []string
	found := false
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, "## ") {
			break
		}
		if !found {
			if headerRE.MatchString(line) {
				found = true
			}
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			break
		}
		rows = append(rows, line)
	}
	return rows, found
}

// Field returns the value of one metadata field from the rows Rows returned.
// The rows look like "| Status | design |". It returns "" when no row names
// the field.
func Field(rows []string, field string) string {
	pattern := regexp.MustCompile(`^\|\s*` + regexp.QuoteMeta(field) + `\s*\|\s*([^|]*?)\s*\|`)
	for _, line := range rows {
		if m := pattern.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}
