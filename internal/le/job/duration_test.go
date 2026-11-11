// Related: registry.go -- appendDuration writes the row, reportPrevious reads it
// Related: job.go -- Ticket.Release, the one writer of a row
//
// VALIDATES: a claimed job's row carries the seconds admission waited as a
// fourth column, beside the seconds it ran, and the previous-run banner reads
// both the old three-column rows already on disk and the new four-column ones.
// PREVENTS: run time and queue time being indistinguishable again, and a
// machine's existing record turning unreadable the day the column lands.

package job

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestDurationRecordCarriesTheWait releases two real claims, through Admit
// and Ticket.Release, and reads the rows they wrote.
//
// The first claim never queued, so its wait is zero and its row says so: zero
// is a measured value here, not a missing one. The second carries a wait set
// on its ticket, the field queue fills, so the row must hold those whole
// seconds and not the run's.
func TestDurationRecordCarriesTheWait(t *testing.T) {
	unset(t, ParentKey)
	root := fixtureRepo(t)
	adm := admission(t, root)
	adm.Err = &strings.Builder{}

	quick, err := adm.Admit("waited", []string{"true"})
	if err != nil {
		t.Fatal(err)
	}
	if quick.Kind != KindClaimed {
		t.Fatalf("first admission = %s, want a claimed slot", quick.Kind)
	}
	quick.Release(0)

	held, err := adm.Admit("waited", []string{"true"})
	if err != nil {
		t.Fatal(err)
	}
	if held.Kind != KindClaimed {
		t.Fatalf("second admission = %s, want a claimed slot", held.Kind)
	}
	held.Waited = 7*time.Second + 400*time.Millisecond
	held.Release(0)

	body := read(t, filepath.Join(root, filepath.FromSlash(DurationFile)))
	rows := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(rows) != 2 {
		t.Fatalf("duration record holds %d rows, want one per released claim:\n%s", len(rows), body)
	}
	for i, want := range []string{"0", "7"} {
		fields := strings.Split(rows[i], "\t")
		if len(fields) != 4 {
			t.Fatalf("row %d = %q, want label, run seconds, UTC time and waited seconds", i, rows[i])
		}
		if fields[0] != "waited" {
			t.Errorf("row %d label = %q, want waited", i, fields[0])
		}
		if _, err := strconv.Atoi(fields[1]); err != nil {
			t.Errorf("row %d run seconds = %q, not a whole number: %v", i, fields[1], err)
		}
		if _, err := time.Parse(time.RFC3339, fields[2]); err != nil {
			t.Errorf("row %d time = %q, not a UTC time: %v", i, fields[2], err)
		}
		if fields[3] != want {
			t.Errorf("row %d waited = %q, want %q", i, fields[3], want)
		}
	}
}

// TestPreviousRunReadsThreeAndFourColumnRows writes a record that mixes the
// rows written before the wait column existed with rows written after it, and
// asks the banner for three labels.
//
// Each label's last row decides, whatever its width: "new" ends on a
// four-column row whose wait (40) differs from its run (125), so a reader that
// took the last column would print the wait; "mixed" ends on a three-column
// row after a four-column one, so a reader that skipped old rows would print
// the earlier run.
func TestPreviousRunReadsThreeAndFourColumnRows(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	record := "old\t65\t2026-10-01T00:00:00Z\n" +
		"new\t30\t2026-10-01T00:00:00Z\n" +
		"new\t125\t2026-11-11T00:00:00Z\t40\n" +
		"mixed\t10\t2026-11-11T00:00:00Z\t3\n" +
		"mixed\t75\t2026-10-02T00:00:00Z\n"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(DurationFile)), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, row := range []struct{ label, want string }{
		{"old", "[old] previous run took 1m5s (65s)\n"},
		{"new", "[new] previous run took 2m5s (125s)\n"},
		{"mixed", "[mixed] previous run took 1m15s (75s)\n"},
	} {
		adm := admission(t, root)
		banner := &strings.Builder{}
		adm.Err = banner
		adm.reportPrevious(row.label)
		if banner.String() != row.want {
			t.Errorf("banner for %s = %q, want %q", row.label, banner.String(), row.want)
		}
	}
}
