package scratch

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/session"
)

// launcherProbeKey turns this test binary into a process parked under a
// named launcher path, for the scanner half of TestTrimNamedLaunchers.
const launcherProbeKey = "ZE_TEST_NAMED_LAUNCHER_PROBE"

// agedDir makes a directory (and its parents) and sets its mtime age before
// now. Call it after writing the directory's contents, which refresh it.
func agedDir(t *testing.T, path string, now time.Time, age time.Duration) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatal(err)
	}
	stamp := now.Add(-age)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	return path
}

// agedFile writes a small file with an mtime age before now.
func agedFile(t *testing.T, path string, now time.Time, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	stamp := now.Add(-age)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}

// recordingRemove answers a remover that records each path and removes it.
func recordingRemove(t *testing.T, removed *[]string) func(string) error {
	t.Helper()
	return func(path string) error {
		*removed = append(*removed, path)
		return os.RemoveAll(path)
	}
}

// refusingRemove answers a remover that fails the test on any call.
func refusingRemove(t *testing.T) func(string) error {
	t.Helper()
	return func(path string) error {
		t.Errorf("remove(%s) was called", path)
		return nil
	}
}

// claudeCLI is the process fact that makes liveness judgeable.
func claudeCLI(now time.Time) session.Process {
	return session.Process{PID: 1, Start: "cli", StartedAt: now.Add(-10 * time.Hour), Argv: []string{"claude"}, CLI: true}
}

// judgeIn binds session.Judge to a checkout and a Claude config directory
// whose projects/ exists, so Reap's rules judge rather than refuse.
func judgeIn(t *testing.T, root string) func([]session.Process) (session.Judgement, error) {
	t.Helper()
	configDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(configDir, "projects", "p"), 0o750); err != nil {
		t.Fatal(err)
	}
	return func(processes []session.Process) (session.Judgement, error) {
		return session.Judge(root, configDir, processes)
	}
}

func sortedBases(paths []string) []string {
	bases := make([]string, 0, len(paths))
	for _, path := range paths {
		bases = append(bases, filepath.Base(path))
	}
	slices.Sort(bases)
	return bases
}

// TestTrimNamedLaunchers proves the launcher rule on injected process facts.
//
// VALIDATES: bin/le-<name>/ is removed exactly when its le and its directory
// are both a day old and no process argv names it (AC-8); bin/le, bin/ze*,
// the platform launchers, a young or running named build, and a symbolic link
// are kept (AC-9). A process started from bin/le-<name>/le shows that path in
// the real scanner's argv (A-4).
// PREVENTS: the trim deleting a binary a peer session runs, or the shared
// platform launcher every session falls back to.
func TestTrimNamedLaunchers(t *testing.T) {
	if os.Getenv(launcherProbeKey) != "" {
		_, _ = io.Copy(io.Discard, os.Stdin) //nolint:errcheck // parked until the parent closes stdin
		return
	}
	now := time.Now()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	day := 25 * time.Hour
	named := func(name string, binaryAge, dirAge time.Duration) {
		dir := gotoolchain.NamedLauncherDir(root, name)
		if binaryAge != 0 {
			agedFile(t, filepath.Join(dir, "le"), now, binaryAge)
		}
		agedDir(t, dir, now, dirAge)
	}
	named("old", day, day)
	named("a", day, day)
	named("young", time.Hour, time.Hour)
	named("fresh-binary", time.Hour, day)
	named("fresh-dir", day, time.Hour)
	named("running", day, day)
	named("Darwin-arm64", day, day)
	named("Linux-x86_64", day, day)
	agedDir(t, gotoolchain.NamedLauncherDir(root, "no-binary"), now, day)
	for _, file := range []string{"le", "ze", "ze-linux-arm64"} {
		agedFile(t, filepath.Join(bin, file), now, day)
	}
	target := agedDir(t, filepath.Join(root, "elsewhere"), now, day)
	if err := os.Symlink(target, gotoolchain.NamedLauncherDir(root, "link")); err != nil {
		t.Fatal(err)
	}
	agedDir(t, bin, now, day)

	processes := []session.Process{
		claudeCLI(now),
		{PID: 2, Start: "r", Argv: []string{filepath.Join(gotoolchain.NamedLauncherDir(root, "running"), "le"), "scratch", "store-trim"}},
		{PID: 3, Start: "ab", Argv: []string{"bin/le-ab/le", "go", "test"}},
	}
	var removed []string
	rows := trimLiveness(t.Context(), root, &livenessPass{
		now: now, processes: processes, judge: judgeIn(t, root),
		startedAt: func(int) time.Time { return time.Time{} }, remove: recordingRemove(t, &removed),
	})
	row := rowNamed(t, rows, launcherStore)
	want := []string{"le-a", "le-no-binary", "le-old"}
	if got := sortedBases(removed); !slices.Equal(got, want) {
		t.Errorf("removed %v, want %v", got, want)
	}
	if row.EntriesRemoved != len(want) || row.Error != "" {
		t.Errorf("launchers row = %+v, want %d removed and no error", row, len(want))
	}
	for _, kept := range []string{"le", "ze", "ze-linux-arm64", "le-young", "le-fresh-binary", "le-fresh-dir", "le-running", "le-Darwin-arm64", "le-Linux-x86_64", "le-link"} {
		if !exists(t, filepath.Join(bin, kept)) {
			t.Errorf("bin/%s was removed", kept)
		}
	}
	if !exists(t, target) {
		t.Error("the target of a symbolic link under bin/ was removed")
	}

	t.Run("a process started from a named launcher shows its path", func(t *testing.T) {
		// A copied system binary is killed by macOS code signing, so the probe
		// is this test binary, copied under the launcher path and parked on
		// its stdin until the cleanup closes it.
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(executable) //nolint:gosec // the running test binary
		if err != nil {
			t.Fatal(err)
		}
		probeRoot := t.TempDir()
		binary := filepath.Join(gotoolchain.NamedLauncherDir(probeRoot, "probe"), "le")
		if err := os.MkdirAll(filepath.Dir(binary), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(binary, content, 0o700); err != nil { //nolint:gosec // an executable copy the test runs
			t.Fatal(err)
		}
		command := exec.CommandContext(t.Context(), binary, "-test.run=^TestTrimNamedLaunchers$")
		command.Env = append(os.Environ(), launcherProbeKey+"=1")
		stdin, err := command.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = stdin.Close()  //nolint:errcheck // releases the parked probe
			_ = command.Wait() //nolint:errcheck // the probe's exit is not under test
		})
		scanned, err := session.ScanProcesses()
		if err != nil {
			t.Fatal(err)
		}
		if !launcherNamedInArgv(scanned, "probe") {
			t.Errorf("the scanner does not show %s in any argv, so a running named le would be judged idle", binary)
		}
	})
}

// TestTrimSessionsAgePlusReap proves the session rule over the real Reap
// judgement.
//
// VALIDATES: a session directory is removed exactly when Reap's rules call it
// dead AND it is a day old; a dead young one, a live old one (argv) and a live
// old one (fresh transcript) are kept (AC-10).
// PREVENTS: the trim removing a live session's state, which no build
// regenerates.
func TestTrimSessionsAgePlusReap(t *testing.T) {
	now := time.Now()
	root := t.TempDir()
	sessions := filepath.Join(root, "tmp", "session")
	day := 25 * time.Hour
	deadOld := agedDir(t, filepath.Join(sessions, "2026-01-01-deadold"), now, day)
	deadYoung := agedDir(t, filepath.Join(sessions, "2026-01-01-deadyoung"), now, time.Hour)
	liveArgv := agedDir(t, filepath.Join(sessions, "2026-01-01-liveargv"), now, day)
	liveTranscript := agedDir(t, filepath.Join(sessions, "2026-01-01-livetranscript"), now, day)
	notSession := agedDir(t, filepath.Join(sessions, "scratch-notdated"), now, day)

	configDir := t.TempDir()
	agedFile(t, filepath.Join(configDir, "projects", "p", "livetranscript.jsonl"), now, time.Minute)
	processes := []session.Process{
		claudeCLI(now),
		{PID: 2, Start: "w", Argv: []string{"le", "work", "liveargv"}},
	}
	var removed []string
	rows := trimLiveness(t.Context(), root, &livenessPass{
		now: now, processes: processes,
		judge: func(processes []session.Process) (session.Judgement, error) {
			return session.Judge(root, configDir, processes)
		},
		startedAt: func(int) time.Time { return time.Time{} }, remove: recordingRemove(t, &removed),
	})
	if !slices.Equal(removed, []string{deadOld}) {
		t.Errorf("removed %v, want only %s", removed, deadOld)
	}
	for _, kept := range []string{deadYoung, liveArgv, liveTranscript, notSession} {
		if !exists(t, kept) {
			t.Errorf("%s was removed", kept)
		}
	}
	row := rowNamed(t, rows, sessionStore)
	if row.EntriesRemoved != 1 || row.Kept != 3 || row.Error != "" {
		t.Errorf("sessions row = %+v, want 1 removed, 3 kept, no error", row)
	}

	t.Run("Reap refusing to judge removes nothing", func(t *testing.T) {
		rows := trimLiveness(t.Context(), root, &livenessPass{
			now: now, processes: processes,
			judge: func(processes []session.Process) (session.Judgement, error) {
				return session.Judge(root, t.TempDir(), processes)
			},
			startedAt: func(int) time.Time { return time.Time{} }, remove: refusingRemove(t),
		})
		if got := rowNamed(t, rows, sessionStore).Skipped; !strings.Contains(got, "Removed nothing") {
			t.Errorf("sessions row skipped = %q, want Reap's notice", got)
		}
	})
	t.Run("a directory a peer already removed is gone, not removed by this run", func(t *testing.T) {
		root := t.TempDir()
		dir := agedDir(t, filepath.Join(root, "tmp", "session", "2026-01-01-peergone"), now, day)
		rows := trimLiveness(t.Context(), root, &livenessPass{
			now: now, processes: []session.Process{claudeCLI(now)}, judge: judgeIn(t, root),
			startedAt: func(int) time.Time { return time.Time{} },
			remove:    func(string) error { return fs.ErrNotExist },
		})
		row := rowNamed(t, rows, sessionStore)
		if row.EntriesRemoved != 0 || row.SizeAfter != 0 || row.Error != "" {
			t.Errorf("sessions row = %+v for %s, want 0 removed, nothing left, no error", row, dir)
		}
	})

	t.Run("with no session directory the testbin row says so", func(t *testing.T) {
		root := t.TempDir()
		for name, judge := range map[string]func([]session.Process) (session.Judgement, error){
			noSessionRoot:         judgeIn(t, root),
			"could not be judged": func([]session.Process) (session.Judgement, error) { return session.Judgement{}, errors.New("broken") },
		} {
			rows := trimLiveness(t.Context(), root, &livenessPass{
				now: now, processes: []session.Process{claudeCLI(now)}, judge: judge,
				startedAt: func(int) time.Time { return time.Time{} }, remove: refusingRemove(t),
			})
			if got := rowNamed(t, rows, testbinStore).Skipped; !strings.Contains(got, name) {
				t.Errorf("testbins row skipped = %q, want it to say %q", got, name)
			}
		}
	})
}

// TestTrimSkipsLivenessWithoutSessions proves the no-Claude guard.
//
// VALIDATES: with no Claude CLI in the process table the launcher, session and
// testbin rows skip with "no session process visible" and nothing is removed,
// while the cache pass still runs and measures its stores (AC-11).
// PREVENTS: a QEMU guest or a container, which sees the checkout and none of
// its sessions, judging every session dead.
func TestTrimSkipsLivenessWithoutSessions(t *testing.T) {
	now := time.Now()
	root := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	day := 25 * time.Hour
	agedFile(t, filepath.Join(gotoolchain.NamedLauncherDir(root, "old"), "le"), now, day)
	agedDir(t, gotoolchain.NamedLauncherDir(root, "old"), now, day)
	dead := filepath.Join(root, "tmp", "session", "2026-01-01-dead")
	agedDir(t, filepath.Join(dead, scratchSubdir, "testbin-pid-999999-x"), now, day)
	agedDir(t, dead, now, day)
	entry := filepath.Join(gotoolchain.GoCache(root), "ab", "abguard-a")
	agedFile(t, entry, now, day)

	noCLI := []session.Process{{PID: 2, Start: "x", Argv: []string{"sh"}}}
	rows := trimLiveness(t.Context(), root, &livenessPass{
		now: now, processes: noCLI, judge: judgeIn(t, root),
		startedAt: func(int) time.Time { return time.Time{} }, remove: refusingRemove(t),
	})
	for _, name := range []string{launcherStore, sessionStore, testbinStore} {
		if got := rowNamed(t, rows, name).Skipped; got != noSessionVisible {
			t.Errorf("%s row skipped = %q, want %q", name, got, noSessionVisible)
		}
	}

	report, _ := trimStoresScanning(root, storeTrimOn, func() ([]session.Process, error) { return noCLI, nil })
	if got := rowNamed(t, report.Stores, checkoutCache); got.Skipped != "" || got.SizeBefore == 0 {
		t.Errorf("checkout cache row = %+v, want it measured: the caches trim without a session", got)
	}
	for _, name := range []string{launcherStore, sessionStore, testbinStore} {
		if got := rowNamed(t, report.Stores, name).Skipped; got != noSessionVisible {
			t.Errorf("trimStores %s row skipped = %q, want %q", name, got, noSessionVisible)
		}
	}
	for _, path := range []string{gotoolchain.NamedLauncherDir(root, "old"), dead} {
		if !exists(t, path) {
			t.Errorf("%s was removed with no session process visible", path)
		}
	}
}

// TestTrimOrphanTestbins proves the testbin rule, pid reuse included.
//
// VALIDATES: testbin-pid-<pid>-<label>/ in a kept session directory, or in its
// scratch/, is removed when it is six hours old and its pid is not running, or
// runs a process that started after the directory was made; it is kept when
// young, when its owner started before it, when the start time cannot be
// measured, and testbin-<suffix>/ is always kept (AC-12).
// PREVENTS: a reused pid keeping an orphan forever, or a long functional run
// losing its binaries mid-suite.
func TestTrimOrphanTestbins(t *testing.T) {
	now := time.Now()
	root := t.TempDir()
	sessionDir := filepath.Join(root, "tmp", "session", "2026-01-01-livesession")
	scratch := filepath.Join(sessionDir, scratchSubdir)
	old := 7 * time.Hour
	made := now.Add(-old)
	testbin := func(parent, name string, age time.Duration) string {
		return agedDir(t, filepath.Join(parent, name), now, age)
	}
	dead := testbin(scratch, "testbin-pid-111-dead", old)
	owned := testbin(scratch, "testbin-pid-222-owned", old)
	reused := testbin(scratch, "testbin-pid-333-reused", old)
	young := testbin(scratch, "testbin-pid-444-young", time.Hour)
	unknown := testbin(scratch, "testbin-pid-555-unknown", old)
	measured := testbin(scratch, "testbin-pid-666-measured", old)
	jitter := testbin(scratch, "testbin-pid-888-jitter", old)
	suffix := testbin(scratch, "testbin-keep", 30*24*time.Hour)
	topLevel := testbin(sessionDir, "testbin-pid-777-toplevel", old)
	agedDir(t, scratch, now, 0)
	agedDir(t, sessionDir, now, 30*24*time.Hour)

	processes := []session.Process{
		claudeCLI(now),
		{PID: 2, Start: "s", Argv: []string{"le", "livesession"}},
		{PID: 222, Start: "a", StartedAt: made.Add(-time.Hour)},
		{PID: 333, Start: "b", StartedAt: now.Add(-time.Hour)},
		{PID: 555, Start: "c"},
		{PID: 666, Start: "d"},
		{PID: 888, Start: "e", StartedAt: made.Add(30 * time.Second)},
	}
	startedAt := func(pid int) time.Time {
		if pid == 666 {
			return now.Add(-time.Hour)
		}
		return time.Time{}
	}
	var removed []string
	rows := trimLiveness(t.Context(), root, &livenessPass{
		now: now, processes: processes, judge: judgeIn(t, root),
		startedAt: startedAt, remove: recordingRemove(t, &removed),
	})
	want := sortedBases([]string{dead, reused, measured, topLevel})
	if got := sortedBases(removed); !slices.Equal(got, want) {
		t.Errorf("removed %v, want %v", got, want)
	}
	for _, kept := range []string{owned, young, unknown, jitter, suffix, sessionDir} {
		if !exists(t, kept) {
			t.Errorf("%s was removed", kept)
		}
	}
	row := rowNamed(t, rows, testbinStore)
	if row.EntriesRemoved != len(want) || row.Kept != 4 || row.Error != "" {
		t.Errorf("testbins row = %+v, want %d removed, 4 kept, no error", row, len(want))
	}
}
