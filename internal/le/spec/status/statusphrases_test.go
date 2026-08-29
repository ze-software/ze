// Related: specstatus.go -- StatusPhrases, report.go -- statusPhrases

package specstatus

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeSpec(t *testing.T, root, name, status string) {
	t.Helper()

	body := "| Field | Value |\n|---|---|\n| Status | " + status + " |\n| Updated | 2026-08-29 |\n"
	path := filepath.Join(root, "plan", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create plan/: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// total sums the leading counts of "<n> <status>" phrases.
func total(t *testing.T, phrases []string) int {
	t.Helper()

	sum := 0
	for _, phrase := range phrases {
		count, err := strconv.Atoi(strings.Fields(phrase)[0])
		if err != nil {
			t.Fatalf("phrase %q does not start with a count: %v", phrase, err)
		}
		sum += count
	}
	return sum
}

// VALIDATES: the breakdown names every status the population holds, including
// one the reporting order does not name, so the counts sum to the total.
// PREVENTS: the summary line under-reporting the population it sits beside.
//
// The session-start hook kept its own list of seven statuses with no default,
// so a status outside the list was counted nowhere. On 2026-08-29 that hid two
// done-but-never-closed specs: 231 files, 229 counted. `done` is the status
// that exposed it because reportingOrder deliberately excludes it as terminal,
// which makes it the natural probe for the sorted tail.
func TestStatusPhrasesCountEveryStatus(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "spec-a.md", "in-progress")
	writeSpec(t, root, "spec-b.md", "design")
	writeSpec(t, root, "spec-c.md", "done")
	writeSpec(t, root, "spec-d.md", "done")

	phrases, err := StatusPhrases(root)
	if err != nil {
		t.Fatalf("StatusPhrases: %v", err)
	}

	if got := total(t, phrases); got != 4 {
		t.Errorf("the counts sum to %d over 4 specs: %v", got, phrases)
	}
	if !strings.Contains(strings.Join(phrases, ", "), "2 done") {
		t.Errorf("the breakdown does not name the terminal status: %v", phrases)
	}
}

// VALIDATES: a status the reporting order never heard of is printed after the
// ones it names.
// PREVENTS: an unnamed status being dropped rather than appended, which is the
// only way the sum can silently disagree with the total.
func TestAnUnknownStatusPrintsAfterTheNamedOnes(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "spec-a.md", "in-progress")
	writeSpec(t, root, "spec-b.md", "zzz-invented")

	phrases, err := StatusPhrases(root)
	if err != nil {
		t.Fatalf("StatusPhrases: %v", err)
	}

	joined := strings.Join(phrases, ", ")
	named, invented := strings.Index(joined, "in-progress"), strings.Index(joined, "zzz-invented")
	if invented < 0 {
		t.Fatalf("the invented status was dropped: %v", phrases)
	}
	if named > invented {
		t.Errorf("the invented status printed before the named one: %v", phrases)
	}
}

// VALIDATES: a spec with no metadata table is counted as unparsed rather than
// passed over.
// PREVENTS: an unreadable spec vanishing from the one line a reader trusts,
// which is the row they most need to act on.
func TestASpecWithNoTableCountsAsUnparsed(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "spec-a.md", "ready")
	if err := os.WriteFile(filepath.Join(root, "plan", "spec-broken.md"), []byte("# no table here\n"), 0o600); err != nil {
		t.Fatalf("write the broken spec: %v", err)
	}

	phrases, err := StatusPhrases(root)
	if err != nil {
		t.Fatalf("StatusPhrases: %v", err)
	}

	if got := total(t, phrases); got != 2 {
		t.Errorf("the counts sum to %d over 2 specs: %v", got, phrases)
	}
	if !strings.Contains(strings.Join(phrases, ", "), statusUnparsed) {
		t.Errorf("the breakdown does not name the unreadable spec: %v", phrases)
	}
}
