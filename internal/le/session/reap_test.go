// VALIDATES: cleanup removes only provably ended sessions, keeps every source
// of liveness, refuses unsafe roots, and makes dry-run a pure report.
package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/leaction"
)

func TestReapDryAcceptsKeywordAndEnvironment(t *testing.T) {
	if !reapDry(leaction.Arguments{"dry": ""}, func(string) string { return "" }) {
		t.Fatal("dry keyword was ignored")
	}
	if !reapDry(leaction.Arguments{}, func(name string) string {
		if name == "DRY" {
			return "1"
		}
		return ""
	}) {
		t.Fatal("DRY environment was ignored")
	}
	if reapDry(leaction.Arguments{}, func(string) string { return "" }) {
		t.Fatal("ordinary reap became dry")
	}
}

func TestReapKeepsEveryLiveSourceAndRemovesDeadState(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "tmp", "session")
	if err := os.MkdirAll(sessions, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, sid := range []string{"own", "pinned", "argv-live", "transcript-live", "dead"} {
		writeSessionFixture(t, sessions, sid)
	}
	if err := os.WriteFile(filepath.Join(sessions, ".sid-by-pid-44-123"), []byte("pinned\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	deadMarker := filepath.Join(sessions, ".lsp-loaded-dead")
	specMarker := filepath.Join(sessions, ".closure-ack-spec-name")
	for _, path := range []string{deadMarker, specMarker} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	config := filepath.Join(root, "config")
	projects := filepath.Join(config, "projects", "fixture")
	if err := os.MkdirAll(projects, 0o750); err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	writeTranscriptFixture(t, projects, "transcript-live", started.Add(time.Minute))
	writeTranscriptFixture(t, projects, "dead", started.Add(-time.Minute))
	processes := []processFact{
		{PID: 44, Start: "123", Argv: []string{"worker"}},
		{PID: 45, Start: "456", StartedAt: started, Argv: []string{"/usr/bin/claude"}, CLI: true},
		{PID: 46, Start: "789", Argv: []string{"tool", "--session-id", "argv-live"}},
	}
	report, err := reap(root, config, "own", false, fixtureReapOps(processes))
	if err != nil {
		t.Fatal(err)
	}
	if report.RemovedDirs != 1 || report.RemovedMarkers != 1 || report.Kept != 4 {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	if _, err := os.Stat(filepath.Join(sessions, "2026-01-01-dead")); !os.IsNotExist(err) {
		t.Fatalf("dead session remains: %v", err)
	}
	for _, sid := range []string{"own", "pinned", "argv-live", "transcript-live"} {
		if _, err := os.Stat(filepath.Join(sessions, "2026-01-01-"+sid)); err != nil {
			t.Errorf("live session %s: %v", sid, err)
		}
	}
	if _, err := os.Stat(deadMarker); !os.IsNotExist(err) {
		t.Errorf("dead marker remains: %v", err)
	}
	if _, err := os.Stat(specMarker); err != nil {
		t.Errorf("spec marker was removed: %v", err)
	}
}

func TestReapDryRunAndReusedPIDPinRemoveNothing(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "tmp", "session")
	if err := os.MkdirAll(sessions, 0o750); err != nil {
		t.Fatal(err)
	}
	dead := writeSessionFixture(t, sessions, "dead")
	pin := filepath.Join(sessions, ".sid-by-pid-44-old")
	if err := os.WriteFile(pin, []byte("dead\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := reap(root, filepath.Join(root, "config"), "own", true,
		fixtureReapOps([]processFact{{PID: 44, Start: "new"}}))
	if err != nil {
		t.Fatal(err)
	}
	if report.RemovedDirs != 1 || report.RemovedMarkers != 1 || !strings.Contains(report.Text(), "Would remove") {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	for _, path := range []string{dead, pin} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("dry run removed %s: %v", path, err)
		}
	}
}

func TestReapRefusesSymlinkSessionRoot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(target, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "tmp", "session")); err != nil {
		t.Fatal(err)
	}
	if _, err := Reap(root, filepath.Join(root, "config"), false); err == nil ||
		!strings.Contains(err.Error(), "unsafe session root") {
		t.Fatalf("Reap error = %v", err)
	}
}

func TestReapFailsClosedWithoutTranscriptsWhileCLIIsRunning(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "tmp", "session")
	if err := os.MkdirAll(sessions, 0o750); err != nil {
		t.Fatal(err)
	}
	dead := writeSessionFixture(t, sessions, "dead")
	processes := []processFact{{PID: 45, Start: "456", StartedAt: time.Now(), CLI: true}}
	report, err := reap(root, filepath.Join(root, "missing-config"), "own", false, fixtureReapOps(processes))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.Text(), "Removed nothing") {
		t.Fatalf("notice = %q", report.Text())
	}
	if _, err := os.Stat(dead); err != nil {
		t.Fatalf("blind reaper removed session: %v", err)
	}
}

func TestCleanScratchRemovesOnlyCurrentSessionAndRefusesSymlinkRoot(t *testing.T) {
	t.Run("current only", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_SESSION_ID", "current")
		root := t.TempDir()
		sessions := filepath.Join(root, "tmp", "session")
		current := writeSessionFixture(t, sessions, "current")
		sibling := writeSessionFixture(t, sessions, "sibling")
		report, err := cleanScratch(root)
		if err != nil || !report.Removed || report.Text() != "" {
			t.Fatalf("clean = %#v, err %v", report, err)
		}
		if _, err := os.Stat(current); !os.IsNotExist(err) {
			t.Fatalf("current session remains: %v", err)
		}
		if _, err := os.Stat(sibling); err != nil {
			t.Fatalf("sibling removed: %v", err)
		}
	})
	t.Run("symlink root", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_SESSION_ID", "current")
		root := t.TempDir()
		target := filepath.Join(root, "target")
		if err := os.MkdirAll(filepath.Join(target, "2026-01-01-current"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, "tmp", "session")); err != nil {
			t.Fatal(err)
		}
		if _, err := cleanScratch(root); err == nil || !strings.Contains(err.Error(), "unsafe session root") {
			t.Fatalf("cleanScratch error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(target, "2026-01-01-current")); err != nil {
			t.Fatalf("symlink target changed: %v", err)
		}
	})
}

func fixtureReapOps(processes []processFact) reapOps {
	return reapOps{
		processes: func() ([]processFact, error) { return processes, nil },
		removeDir: os.RemoveAll,
		remove:    os.Remove,
	}
}

func writeSessionFixture(t *testing.T, sessions, sid string) string {
	t.Helper()
	path := filepath.Join(sessions, "2026-01-01-"+sid)
	if err := os.MkdirAll(filepath.Join(path, "bin"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "bin", "ze"), []byte("binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeTranscriptFixture(t *testing.T, projects, sid string, at time.Time) {
	t.Helper()
	path := filepath.Join(projects, sid+".jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: the portable ps elapsed-time field parses in every shape ps emits,
// and a shape it never emits is refused rather than read as an age.
// PREVENTS: the reaper going blind again. `etimes` is Linux-only, so on macOS
// the whole process scan exited non-zero, no session looked either dead or
// running, and the command reported nothing to remove over eleven live
// directories -- a clean-tree answer produced by having examined nothing.
func TestElapsedSecondsReadsEveryShapePSEmits(t *testing.T) {
	for _, row := range []struct {
		value string
		want  int64
	}{
		{"00:00", 0},
		{"00:01", 1},
		{"01:00", 60},
		{"59:59", 3599},
		{"01:00:00", 3600},
		{"23:59:59", 86399},
		{"1-00:00:00", 86400},
		{"02-04:31:48", 189108},
		{"  02:30  ", 150},
	} {
		got, ok := elapsedSeconds(row.value)
		if !ok || got != row.want {
			t.Errorf("elapsedSeconds(%q) = %d, %v; want %d, true", row.value, got, ok, row.want)
		}
	}
}

func TestElapsedSecondsRefusesWhatPSNeverEmits(t *testing.T) {
	for _, value := range []string{"", "   ", "12", "1:2:3:4", "aa:bb", "-1:00", "x-01:00:00", "01:-1"} {
		if got, ok := elapsedSeconds(value); ok {
			t.Errorf("elapsedSeconds(%q) = %d, true; want a refusal", value, got)
		}
	}
}
