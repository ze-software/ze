// VALIDATES: cleanup removes only provably ended sessions, keeps every source
// of liveness, refuses unsafe roots, and makes dry-run a pure report.
package session

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/sessiondir"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/le/procscan"
)

func TestReapDryAcceptsKeywordAndEnvironment(t *testing.T) {
	if !reapDry(leaction.Arguments{"dry": {""}}, func(string) string { return "" }) {
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
	// An agent's spec claim goes with its dead session and stays with a live one.
	deadAgentClaim := filepath.Join(sessions, ".session-dead+agent-one")
	liveAgentClaim := filepath.Join(sessions, ".session-own+agent-one")
	for _, path := range []string{deadMarker, specMarker, deadAgentClaim, liveAgentClaim} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// A session that wrote a flat marker within IdleAge is never dead, so the
	// dead session's markers are as old as its directory.
	for _, path := range []string{deadMarker, deadAgentClaim} {
		ageTree(t, path, 2*IdleAge)
	}
	config := filepath.Join(root, "config")
	projects := filepath.Join(config, "projects", "fixture")
	if err := os.MkdirAll(projects, 0o750); err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	writeTranscriptFixture(t, projects, "transcript-live", started.Add(time.Minute))
	writeTranscriptFixture(t, projects, "dead", started.Add(-time.Minute))
	processes := []procscan.Process{
		{PID: 44, Start: "123", Argv: []string{"worker"}},
		{PID: 45, Start: "456", StartedAt: started, Argv: []string{"/usr/bin/claude"}, CLI: true},
		{PID: 46, Start: "789", Argv: []string{"tool", "--session-id", "argv-live"}},
	}
	report, err := reap(root, config, "own", false, fixtureReapOps(processes))
	if err != nil {
		t.Fatal(err)
	}
	if report.RemovedDirs != 1 || report.RemovedMarkers != 2 || report.Kept != 4 {
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
	if _, err := os.Stat(deadAgentClaim); !os.IsNotExist(err) {
		t.Errorf("dead session's agent claim remains: %v", err)
	}
	if _, err := os.Stat(liveAgentClaim); err != nil {
		t.Errorf("live session's agent claim was removed: %v", err)
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
	// A Claude CLI and a projects directory, so the reaper judges rather than
	// refuses: with no CLI visible it removes nothing, dry or not.
	config := filepath.Join(root, "config")
	if err := os.MkdirAll(filepath.Join(config, "projects", "fixture"), 0o750); err != nil {
		t.Fatal(err)
	}
	report, err := reap(root, config, "own", true, fixtureReapOps([]procscan.Process{
		{PID: 44, Start: "new"},
		{PID: 45, Start: "456", StartedAt: time.Now(), Argv: []string{"/usr/bin/claude"}, CLI: true},
	}))
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
	processes := []procscan.Process{{PID: 45, Start: "456", StartedAt: time.Now(), CLI: true}}
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

func fixtureReapOps(processes []procscan.Process) reapOps {
	return reapOps{
		processes: func() ([]procscan.Process, error) { return processes, nil },
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
	// The reaper reads the owner from .session-id, never from the name.
	if err := os.WriteFile(filepath.Join(path, sessiondir.IDFile), []byte(sid+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A session that wrote within IdleAge is never dead, so the fixture's
	// whole tree is older: each test that needs a recent write makes one.
	ageTree(t, path, 2*IdleAge)
	return path
}

// ageTree sets every mtime under path, path included, to age before now.
//
//nolint:unparam // every caller ages past the idle window today; the age stays a parameter so a test of the window's edge can pick its own
func ageTree(t *testing.T, path string, age time.Duration) {
	t.Helper()
	stamp := time.Now().Add(-age)
	err := filepath.WalkDir(path, func(entry string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return os.Chtimes(entry, stamp, stamp)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: the reaper takes a session directory's owner from its
// .session-id, not from its name, and keeps a dated directory whose owner is
// not recorded, counting it as unowned.
// PREVENTS: a creation-time name read as a session id (every new directory
// would look dead), or a directory of unknown owner removed while its session
// still runs.
func TestReapReadsTheOwnerFromItsIDFileAndKeepsUnownedDirectories(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "tmp", "session")
	owned := filepath.Join(sessions, "2026-10-30-142305")
	unowned := filepath.Join(sessions, "2026-10-30-142305-2")
	dead := filepath.Join(sessions, "2026-10-30-142306")
	for _, dir := range []string{owned, unowned, dead} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	for dir, sid := range map[string]string{owned: "own", dead: "dead"} {
		if err := os.WriteFile(filepath.Join(dir, sessiondir.IDFile), []byte(sid+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// The dead session wrote nothing within IdleAge, and a Claude CLI and a
	// projects directory make the reaper judge rather than refuse.
	ageTree(t, dead, 2*IdleAge)
	config := filepath.Join(root, "config")
	if err := os.MkdirAll(filepath.Join(config, "projects", "fixture"), 0o750); err != nil {
		t.Fatal(err)
	}
	cli := procscan.Process{PID: 45, Start: "456", StartedAt: time.Now(), Argv: []string{"/usr/bin/claude"}, CLI: true}
	report, err := reap(root, config, "own", false, fixtureReapOps([]procscan.Process{cli}))
	if err != nil {
		t.Fatal(err)
	}
	if report.RemovedDirs != 1 || report.Kept != 1 || report.Unowned != 1 {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	if !strings.Contains(report.Text(), "1 with no readable .session-id") {
		t.Errorf("text %q does not report the unowned directory", report.Text())
	}
	if _, err := os.Stat(dead); !os.IsNotExist(err) {
		t.Errorf("dead session remains: %v", err)
	}
	for _, dir := range []string{owned, unowned} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("%s was removed: %v", dir, err)
		}
	}
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

// VALIDATES: a session directory written within IdleAge, whose owner no proof
// of life names, is kept and counted apart from the running ones, in the
// report's numbers and text and in Judge's Undecided; only a directory whose
// owner a proof of life names counts as running.
// PREVENTS: "kept N running" counting a directory nothing shows to run.
// Method: an "own" directory (this process's id proves it) and an idle
// directory holding one fresh file, with a Claude CLI running; reap and Judge
// judge the two. It needs no unwalkable tree, so it runs as root too.
func TestARecentlyWrittenDirectoryIsCountedApartFromRunningOnes(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "own")
	world := newClaimWorld(t)
	own := writeSessionFixture(t, world.sessions, "own")
	busy := writeSessionFixture(t, world.sessions, "busy")
	fresh := filepath.Join(busy, "fresh")
	if err := os.WriteFile(fresh, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	processes := []procscan.Process{world.cli}

	report, err := reap(world.root, world.config, "own", false, fixtureReapOps(processes))
	if err != nil {
		t.Fatal(err)
	}
	want := []UndecidedCount{{Cause: UndecidedRecentTree, Count: 1}}
	if report.RemovedDirs != 0 || report.Kept != 1 || !slices.Equal(report.Undecided, want) {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	text := report.Text()
	for _, phrase := range []string{
		"kept 1 running;",
		"kept 1 whose owner is neither proven running nor dead (1 written within the idle age)",
	} {
		if !strings.Contains(text, phrase) {
			t.Errorf("text %q does not say %q", text, phrase)
		}
	}
	assertPresent(t, own, busy, fresh)

	judgement, err := Judge(world.root, world.config, processes)
	if err != nil {
		t.Fatal(err)
	}
	if len(judgement.Undecided) != 1 {
		t.Fatalf("Undecided = %#v, want the busy directory alone", judgement.Undecided)
	}
	recent := judgement.Undecided[0]
	if recent.Path != busy || recent.Session != "busy" || recent.Cause != UndecidedRecentTree || recent.Reason == "" {
		t.Errorf("Undecided[0] = %#v, want %s written within the idle age, with the reason", recent, busy)
	}
}

// VALIDATES: a session directory whose tree cannot be walked, and one written
// within IdleAge, are kept and counted apart from the running ones, by cause,
// in the report's numbers and text and in Judge's Undecided; only a directory
// whose owner a proof of life names counts as running.
// PREVENTS: "kept N running" counting directories nothing shows to run, and an
// unwalkable directory kept with no word of why.
// Method: an "own" directory (this process's id proves it), an idle directory
// holding a 0o000 subdirectory, and an idle directory holding one fresh file,
// with a Claude CLI running; reap and Judge judge the three.
func TestUndecidedDirectoriesAreCountedApartFromRunningOnes(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root walks a directory whatever its mode, so the tree cannot be made unwalkable")
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "own")
	world := newClaimWorld(t)
	own := writeSessionFixture(t, world.sessions, "own")
	locked := writeSessionFixture(t, world.sessions, "locked")
	sealed := filepath.Join(locked, "sealed")
	if err := os.Mkdir(sealed, 0o750); err != nil {
		t.Fatal(err)
	}
	ageTree(t, locked, 2*IdleAge)
	if err := os.Chmod(sealed, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(sealed, 0o700); err != nil { //nolint:gosec // the test must open its own 0o000 directory to remove it
			t.Error(err)
		}
	})
	busy := writeSessionFixture(t, world.sessions, "busy")
	if err := os.WriteFile(filepath.Join(busy, "fresh"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	processes := []procscan.Process{world.cli}

	report, err := reap(world.root, world.config, "own", false, fixtureReapOps(processes))
	if err != nil {
		t.Fatal(err)
	}
	want := []UndecidedCount{{Cause: UndecidedRecentTree, Count: 1}, {Cause: UndecidedUnwalkable, Count: 1}}
	if report.RemovedDirs != 0 || report.Kept != 1 || !slices.Equal(report.Undecided, want) {
		t.Fatalf("report = %#v, text %q", report, report.Text())
	}
	text := report.Text()
	for _, phrase := range []string{
		"kept 1 running;",
		"kept 2 whose owner is neither proven running nor dead (1 written within the idle age, 1 cannot be walked)",
	} {
		if !strings.Contains(text, phrase) {
			t.Errorf("text %q does not say %q", text, phrase)
		}
	}
	assertPresent(t, own, locked, sealed, busy)

	judgement, err := Judge(world.root, world.config, processes)
	if err != nil {
		t.Fatal(err)
	}
	if len(judgement.Undecided) != 2 {
		t.Fatalf("Undecided = %#v, want the busy and the locked directory", judgement.Undecided)
	}
	recent, unwalkable := judgement.Undecided[0], judgement.Undecided[1]
	if recent.Path != busy || recent.Session != "busy" || recent.Cause != UndecidedRecentTree {
		t.Errorf("Undecided[0] = %#v, want %s written within the idle age", recent, busy)
	}
	if unwalkable.Path != locked || unwalkable.Session != "locked" || unwalkable.Cause != UndecidedUnwalkable ||
		!strings.Contains(unwalkable.Reason, "cannot be walked") {
		t.Errorf("Undecided[1] = %#v, want %s that cannot be walked, with the reason", unwalkable, locked)
	}
}
