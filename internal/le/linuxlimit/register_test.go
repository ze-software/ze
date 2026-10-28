// Design: docs/architecture/linux-limitations.md

package linuxlimit

import (
	"strings"
	"testing"
)

// VALIDATES: each form check of Parse refuses the malformed page
// it exists for, and accepts the well-formed variants.
// PREVENTS: an entry escaping the version comparison through a stray heading,
// a repeated field, a missing Status or Effect, an invalid pin or an impossible date.
func TestParseLinuxLimitsForm(t *testing.T) {
	const head = "# Register\n\n## Entries\n\n"
	const good = "### LL-GOOD\n\n- **Kind:** kernel\n- **Effect:** blocks\n" +
		"- **Status:** verified\n- **Checked:** Linux 7.2.9, 2026-10-20\n"
	cases := []struct {
		name string
		page string
		want string // a substring of the one problem expected, or "" for none
	}{
		{"well formed", head + good, ""},
		{"kernel pin X.Y", head + strings.Replace(good, "7.2.9", "7.3", 1), ""},
		{"heading after the entries", head + good + "\n## Left out\n\n### Anything\n", ""},
		{"stray heading", head + good + "\n### Not an id\n", "heading"},
		{"duplicate Kind", head + strings.Replace(good, "- **Effect:**", "- **Kind:** kernel\n- **Effect:**", 1), "more than one Kind"},
		{"duplicate Checked", head + good + "- **Checked:** Linux 7.2.9, 2026-10-21\n", "more than one Checked"},
		{"missing Status", head + strings.Replace(good, "- **Status:** verified\n", "", 1), "no Status"},
		{"bad Status", head + strings.Replace(good, "verified", "maybe", 1), "Status"},
		{"missing Effect", head + strings.Replace(good, "- **Effect:** blocks\n", "", 1), "no Effect"},
		{"bad Effect", head + strings.Replace(good, "blocks", "hinders", 1), "Effect"},
		{"impossible date", head + strings.Replace(good, "2026-10-20", "2026-13-45", 1), "date"},
		{"unrecognized Checked", head + strings.Replace(good, "Linux 7.2.9, 2026-10-20", "checked recently", 1), "Checked"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entries, problems := Parse(tc.page)
			if tc.want == "" {
				if len(problems) != 0 {
					t.Fatalf("want no problem, got %q", problems)
				}
				return
			}
			if len(problems) != 1 || !strings.Contains(problems[0], tc.want) {
				t.Fatalf("want one problem containing %q, got %q", tc.want, problems)
			}
			if len(entries) != 0 {
				t.Fatalf("invalid register returned usable entries: %+v", entries)
			}
		})
	}
}

// TestParsePreservesGateFields checks the values the gate consumes, including
// requirement IDs and source evidence on continuation lines and a module pin.
func TestParsePreservesGateFields(t *testing.T) {
	page := "## Entries\n### LL-KERNEL\n" +
		"- **Kind:** kernel\n- **Effect:** blocks\n- **Status:** verified\n" +
		"- **Checked:** Linux 7.2.9, 2026-10-20\n" +
		"- **Blocks:** RFC1-1-1\n  RFC1-1-2\n" +
		"- **Evidence:** `net/example.c` `example`\n  `return -EINVAL;`\n" +
		"- **Re-check:** read example\n  at the new release\n" +
		"### LL-LIBRARY\n- **Kind:** library\n- **Effect:** constrains design\n" +
		"- **Status:** unverified\n- **Checked:** module example.org/lib v1.2.3, 2026-10-21\n"
	entries, problems := Parse(page)
	if len(problems) != 0 {
		t.Fatalf("Parse: %v", problems)
	}
	if len(entries) != 2 {
		t.Fatalf("want two entries, got %d", len(entries))
	}
	want := Entry{
		ID: "LL-KERNEL", Kind: "kernel", Effect: "blocks", Status: "verified",
		Checked: "Linux 7.2.9, 2026-10-20", Kernel: "7.2.9",
		Blocks: "RFC1-1-1\nRFC1-1-2", Evidence: "`net/example.c` `example`\n`return -EINVAL;`",
		ReCheck: "read example\nat the new release",
	}
	if entries[0] != want {
		t.Fatalf("kernel entry: got %+v, want %+v", entries[0], want)
	}
	want = Entry{
		ID: "LL-LIBRARY", Kind: "library", Effect: "constrains design", Status: "unverified",
		Checked: "module example.org/lib v1.2.3, 2026-10-21",
		Module:  "example.org/lib", Version: "v1.2.3",
	}
	if entries[1] != want {
		t.Fatalf("library entry: got %+v, want %+v", entries[1], want)
	}
}

// TestParseRefusesAmbiguousFields ensures malformed or repeated evidence cannot
// leave usable entries behind for a caller to admit accidentally.
func TestParseRefusesAmbiguousFields(t *testing.T) {
	const good = "### LL-GOOD\n- **Kind:** kernel\n- **Effect:** blocks\n" +
		"- **Status:** verified\n- **Checked:** Linux 7.2.9, 2026-10-20\n"
	cases := []struct {
		name  string
		extra string
		want  string
	}{
		{"duplicate id", good, "LL-GOOD has more than one heading"},
		{"duplicate Blocks", "- **Blocks:** RFC1-1-1\n- **Blocks:** RFC2-1-1\n", "more than one Blocks"},
		{"indented duplicate", "- **Evidence:** first\n  - **Evidence:** second\n", "indented field"},
		{"duplicate Evidence", "- **Evidence:** first\n- **Evidence:** second\n", "more than one Evidence"},
		{"duplicate Re-check", "- **Re-check:** first\n- **Re-check:** second\n", "more than one Re-check"},
		{"malformed field", "- **Evidence**: hidden\n", "malformed field"},
		{"unknown field", "- **Evidnce:** hidden\n", "unknown field"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entries, problems := Parse("## Entries\n" + good + tc.extra)
			if len(entries) != 0 {
				t.Fatalf("invalid register returned usable entries: %+v", entries)
			}
			if len(problems) != 1 {
				t.Fatalf("want one problem, got %q", problems)
			}
			if !strings.Contains(problems[0], tc.want) {
				t.Fatalf("want problem containing %q, got %q", tc.want, problems)
			}
		})
	}
}

// TestValidID shares the register's accepted language with annotation and
// approval readers without accepting whitespace or a heading as an ID.
func TestValidID(t *testing.T) {
	for _, id := range []string{"LL-A", "LL-XFRM-STATE-1", "LL-0", "LL--"} {
		if !ValidID(id) {
			t.Errorf("valid register ID %q refused", id)
		}
	}
	for _, id := range []string{"", "LL-", "LL-lower", "ll-UPPER", " LL-A", "LL-A ", "LL-A\n", "LL-A;reason", "### LL-A", "LL_A"} {
		if ValidID(id) {
			t.Errorf("invalid register ID %q accepted", id)
		}
	}
}
