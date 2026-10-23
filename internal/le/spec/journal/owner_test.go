package specjournal

// VALIDATES: every open journal row a commit adds or changes is listed in a
// spec's owned journal rows table (plan/journal/README.md, "Owned rows").
// PREVENTS: a defect recorded as an open row that no spec will ever fix.

import (
	"maps"
	"strings"
	"testing"
)

const (
	ownerClassPath = "plan/journal/silent-default.md"
	ownerSpecPath  = "plan/release/fixes/spec-refuse-defaults.md"
	ownerOpenRow   = "| 2026-10-23 | - | cli | answered zero | open: owed |\n"
	ownerFixedRow  = "| 2026-10-23 | - | cli | answered zero | fixed: refused now |\n"
)

// ownerSpec renders a spec whose owned journal rows table lists one entry per
// date, each against the silent-default class.
func ownerSpec(dates ...string) string {
	var body strings.Builder
	body.WriteString("# Spec\n\n## Owned journal rows\n\n| Class file | Date | Surface (short) |\n|---|---|---|\n")
	for _, date := range dates {
		body.WriteString("| `silent-default.md` | ")
		body.WriteString(date)
		body.WriteString(" | cli |\n")
	}
	body.WriteString("\n## Next section\n\n| `silent-default.md` | 2026-01-01 | outside the table |\n")
	return body.String()
}

// TestUnownedOpenRows proves the ownership gate over the rows a commit adds or
// changes against HEAD. Method: seed a repository, write the working tree each
// case describes, and read the verdict for the commit's file population. An
// added open row with no owner is refused, an owner listed by a spec at HEAD or
// by a spec this commit carries clears it, a spec left out of the commit or
// removed by it owns nothing, a fixed row needs no owner, a row HEAD already
// holds is never judged, and each open row of a date needs its own entry.
func TestUnownedOpenRows(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		head    map[string]string
		tree    map[string]string
		paths   []string
		removed []string
		unowned int
	}{
		{
			name:    "added open row with no owner",
			tree:    map[string]string{ownerClassPath: journalTableHead + ownerOpenRow},
			paths:   []string{ownerClassPath},
			unowned: 1,
		},
		{
			name:  "owner committed at HEAD",
			head:  map[string]string{ownerSpecPath: ownerSpec("2026-10-23")},
			tree:  map[string]string{ownerClassPath: journalTableHead + ownerOpenRow},
			paths: []string{ownerClassPath},
		},
		{
			name: "owner carried by the same commit",
			tree: map[string]string{
				ownerClassPath: journalTableHead + ownerOpenRow,
				ownerSpecPath:  ownerSpec("2026-10-23"),
			},
			paths: []string{ownerClassPath, ownerSpecPath},
		},
		{
			name: "owner left out of the commit",
			tree: map[string]string{
				ownerClassPath: journalTableHead + ownerOpenRow,
				ownerSpecPath:  ownerSpec("2026-10-23"),
			},
			paths:   []string{ownerClassPath},
			unowned: 1,
		},
		{
			name:    "owner removed by the commit",
			head:    map[string]string{ownerSpecPath: ownerSpec("2026-10-23")},
			tree:    map[string]string{ownerClassPath: journalTableHead + ownerOpenRow},
			paths:   []string{ownerClassPath, ownerSpecPath},
			removed: []string{ownerSpecPath},
			unowned: 1,
		},
		{
			name:    "entry outside the owned table",
			head:    map[string]string{ownerSpecPath: ownerSpec()},
			tree:    map[string]string{ownerClassPath: journalTableHead + ownerOpenRow},
			paths:   []string{ownerClassPath},
			unowned: 1,
		},
		{
			name:    "entry whose date does not parse",
			head:    map[string]string{ownerSpecPath: ownerSpec("2026-10-32")},
			tree:    map[string]string{ownerClassPath: journalTableHead + "| 2026-10-32 | - | cli | x | open: owed |\n"},
			paths:   []string{ownerClassPath},
			unowned: 1,
		},
		{
			name:  "fixed row needs no owner",
			tree:  map[string]string{ownerClassPath: journalTableHead + ownerFixedRow},
			paths: []string{ownerClassPath},
		},
		{
			name:  "unowned row already at HEAD",
			head:  map[string]string{ownerClassPath: journalTableHead + ownerOpenRow},
			tree:  map[string]string{ownerClassPath: journalTableHead + ownerOpenRow + ownerFixedRow},
			paths: []string{ownerClassPath},
		},
		{
			name:    "row reopened against HEAD",
			head:    map[string]string{ownerClassPath: journalTableHead + ownerFixedRow},
			tree:    map[string]string{ownerClassPath: journalTableHead + strings.Replace(ownerFixedRow, "fixed: ", "open: ", 1)},
			paths:   []string{ownerClassPath},
			unowned: 1,
		},
		{
			// A legacy unowned row shares the new row's date: both are named,
			// so the printed fix clears the key rather than half of it.
			name: "legacy unowned row on the new row's date",
			head: map[string]string{ownerClassPath: journalTableHead + ownerOpenRow},
			tree: map[string]string{
				ownerClassPath: journalTableHead + ownerOpenRow + "| 2026-10-23 | - | web | second | open: owed |\n",
				ownerSpecPath:  ownerSpec("2026-10-23"),
			},
			paths:   []string{ownerClassPath, ownerSpecPath},
			unowned: 2,
		},
		{
			name: "legacy and new row of one date, both owned",
			head: map[string]string{ownerClassPath: journalTableHead + ownerOpenRow},
			tree: map[string]string{
				ownerClassPath: journalTableHead + ownerOpenRow + "| 2026-10-23 | - | web | second | open: owed |\n",
				ownerSpecPath:  ownerSpec("2026-10-23", "2026-10-23"),
			},
			paths: []string{ownerClassPath, ownerSpecPath},
		},
		{
			name: "closure removes a spec whose row is still open",
			head: map[string]string{
				ownerClassPath: journalTableHead + ownerOpenRow,
				ownerSpecPath:  ownerSpec("2026-10-23"),
			},
			paths:   []string{ownerSpecPath},
			removed: []string{ownerSpecPath},
			unowned: 1,
		},
		{
			name: "closure removes a spec whose rows are fixed",
			head: map[string]string{
				ownerClassPath: journalTableHead + ownerFixedRow,
				ownerSpecPath:  ownerSpec("2026-10-23"),
			},
			paths:   []string{ownerSpecPath},
			removed: []string{ownerSpecPath},
		},
		{
			name: "table entry deleted from a spec",
			head: map[string]string{
				ownerClassPath: journalTableHead + ownerOpenRow,
				ownerSpecPath:  ownerSpec("2026-10-23"),
			},
			tree:    map[string]string{ownerSpecPath: ownerSpec()},
			paths:   []string{ownerSpecPath},
			unowned: 1,
		},
		{
			name: "heading inside a code fence keeps the table open",
			head: map[string]string{ownerSpecPath: "# Spec\n\n## Owned journal rows\n\n```\n# not a heading\n```\n\n" +
				"| `silent-default.md` | 2026-10-23 | cli |\n"},
			tree:  map[string]string{ownerClassPath: journalTableHead + ownerOpenRow},
			paths: []string{ownerClassPath},
		},
		{
			name: "two open rows of one date, one entry",
			head: map[string]string{ownerSpecPath: ownerSpec("2026-10-23")},
			tree: map[string]string{ownerClassPath: journalTableHead + ownerOpenRow +
				"| 2026-10-23 | - | web | second | open: owed |\n"},
			paths:   []string{ownerClassPath},
			unowned: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			seed := map[string]string{"plan/journal/README.md": "# Journal\n"}
			maps.Copy(seed, test.head)
			root := journalFixture(t, seed)
			for path, body := range test.tree {
				writeJournalFile(t, root, path, body)
			}
			unowned, err := UnownedOpenRows(root, test.paths, test.removed)
			if err != nil {
				t.Fatalf("UnownedOpenRows: %v", err)
			}
			if len(unowned) != test.unowned {
				t.Fatalf("unowned = %+v, want %d row(s)", unowned, test.unowned)
			}
		})
	}
}

// TestUnownedOpenRowsTextNamesTheFix proves the refusal says what is wrong, why,
// and the exact table line that clears it. Method: render the verdict for one
// unowned row and look for each of the three parts.
func TestUnownedOpenRowsTextNamesTheFix(t *testing.T) {
	t.Parallel()
	text := UnownedText([]UnownedRow{{Path: ownerClassPath, Line: 3, Class: "silent-default",
		Date: "2026-10-23", Surface: "cli", Open: 1}})
	for _, want := range []string{
		"plan/journal/silent-default.md line 3",
		"no spec owns it",
		"| `silent-default.md` | 2026-10-23 | cli |",
		"## Owned journal rows",
		"fixed: ",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("UnownedText = %q, want it to contain %q", text, want)
		}
	}
}
