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
	"syscall"
	"testing"
	"time"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/procscan"
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
func claudeCLI(now time.Time) procscan.Process {
	return procscan.Process{PID: 1, Start: "cli", StartedAt: now.Add(-10 * time.Hour), Argv: []string{"claude"}, CLI: true}
}

// judgeIn binds session.Judge to a checkout and a Claude config directory
// whose projects/ exists, so Reap's rules judge rather than refuse.
func judgeIn(t *testing.T, root string) func([]procscan.Process) (session.Judgement, error) {
	t.Helper()
	configDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(configDir, "projects", "p"), 0o750); err != nil {
		t.Fatal(err)
	}
	return func(processes []procscan.Process) (session.Judgement, error) {
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
// a young or running named build, and a symbolic link are kept (AC-9). A
// leftover bin/le-<uname -s>-<uname -m> from the launcher before bin/builds
// (AC-10) is judged like a named build: removed when idle and unnamed, kept
// while young or named in an argv. A process started from bin/le-<name>/le
// shows that path in the real scanner's argv (A-4).
// PREVENTS: the trim deleting a binary a peer session runs, or keeping a
// dead platform launcher forever.
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
	named("Linux-aarch64", day, day)
	named("Darwin-arm64", day, day)
	named("Linux-x86_64", time.Hour, time.Hour)
	agedDir(t, gotoolchain.NamedLauncherDir(root, "no-binary"), now, day)
	for _, file := range []string{"le", "ze", "ze-linux-arm64"} {
		agedFile(t, filepath.Join(bin, file), now, day)
	}
	target := agedDir(t, filepath.Join(root, "elsewhere"), now, day)
	if err := os.Symlink(target, gotoolchain.NamedLauncherDir(root, "link")); err != nil {
		t.Fatal(err)
	}
	agedDir(t, bin, now, day)

	processes := []procscan.Process{
		claudeCLI(now),
		{PID: 2, Start: "r", Argv: []string{filepath.Join(gotoolchain.NamedLauncherDir(root, "running"), "le"), "scratch", "store-trim"}},
		{PID: 3, Start: "ab", Argv: []string{"bin/le-ab/le", "go", "test"}},
		{PID: 4, Start: "d", Argv: []string{"bin/le-Darwin-arm64/le", "go", "test"}},
	}
	var removed []string
	rows := trimLiveness(t.Context(), root, &livenessPass{
		now: now, processes: processes, judge: judgeIn(t, root),
		startedAt: func(int) time.Time { return time.Time{} }, remove: recordingRemove(t, &removed),
	})
	row := rowNamed(t, rows, launcherStore)
	want := []string{"le-Linux-aarch64", "le-a", "le-no-binary", "le-old"}
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
		scanned, err := procscan.Scan()
		if err != nil {
			t.Fatal(err)
		}
		if !pathNamedInArgv(scanned, strings.TrimPrefix(gotoolchain.NamedLauncherDir("", "probe"), string(filepath.Separator))) {
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
	processes := []procscan.Process{
		claudeCLI(now),
		{PID: 2, Start: "w", Argv: []string{"le", "work", "liveargv"}},
	}
	var removed []string
	rows := trimLiveness(t.Context(), root, &livenessPass{
		now: now, processes: processes,
		judge: func(processes []procscan.Process) (session.Judgement, error) {
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
			judge: func(processes []procscan.Process) (session.Judgement, error) {
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
			now: now, processes: []procscan.Process{claudeCLI(now)}, judge: judgeIn(t, root),
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
		for name, judge := range map[string]func([]procscan.Process) (session.Judgement, error){
			noSessionRoot:         judgeIn(t, root),
			"could not be judged": func([]procscan.Process) (session.Judgement, error) { return session.Judgement{}, errors.New("broken") },
		} {
			rows := trimLiveness(t.Context(), root, &livenessPass{
				now: now, processes: []procscan.Process{claudeCLI(now)}, judge: judge,
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
	for _, name := range []string{"20261001T000000Z-aaaaaaa", "20261002T000000Z-bbbbbbb", "20261003T000000Z-ccccccc"} {
		agedBuild(t, filepath.Join(gotoolchain.BuildsDir(root), name), now, day)
	}
	agedFile(t, filepath.Join(root, "bin", "gomu"), now, day)
	dead := filepath.Join(root, "tmp", "session", "2026-01-01-dead")
	agedDir(t, filepath.Join(dead, scratchSubdir, "testbin-pid-999999-x"), now, day)
	agedDir(t, dead, now, day)
	entry := filepath.Join(gotoolchain.GoCache(root), "ab", "abguard-a")
	agedFile(t, entry, now, day)

	noCLI := []procscan.Process{{PID: 2, Start: "x", Argv: []string{"sh"}}}
	rows := trimLiveness(t.Context(), root, &livenessPass{
		now: now, processes: noCLI, judge: judgeIn(t, root),
		startedAt: func(int) time.Time { return time.Time{} }, remove: refusingRemove(t),
	})
	for _, name := range []string{launcherStore, sessionStore, testbinStore, buildsStore, agentFilesStore} {
		if got := rowNamed(t, rows, name).Skipped; got != noSessionVisible {
			t.Errorf("%s row skipped = %q, want %q", name, got, noSessionVisible)
		}
	}

	// A fresh HOME with no .zt, and longer than 19 bytes, so the test-temp row is a
	// skip and nothing of the user's is trimmed.
	t.Setenv("HOME", t.TempDir())
	telemetryOffUnderHome(t)
	report, _ := trimStoresScanning(root, storeTrimOn, func() ([]procscan.Process, error) { return noCLI, nil })
	if got := rowNamed(t, report.Stores, checkoutCache); got.Skipped != "" || got.SizeBefore == 0 {
		t.Errorf("checkout cache row = %+v, want it measured: the caches trim without a session", got)
	}
	for _, name := range []string{launcherStore, sessionStore, testbinStore, buildsStore, agentFilesStore} {
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

// telemetryOffUnderHome turns Go telemetry off for the HOME the test set.
//
// trimStoresScanning runs `go env GOCACHE`, and the go command keeps its
// telemetry under os.UserConfigDir, which follows HOME. In the default local
// mode it forks a detached upload child (x/telemetry start.go, parent) that
// outlives `go env` and keeps writing there. t.TempDir's cleanup then races
// that child and fails with "directory not empty", which a loaded host makes
// likely. With the mode file reading off, the go command writes nothing and
// starts no child, so the HOME stays the test's own. XDG_CONFIG_HOME is
// cleared first so that on Linux the mode file lands under the test HOME and
// never in the developer's own config.
func telemetryOffUnderHome(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", "")
	config, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("resolve the config dir under the test HOME: %v", err)
	}
	dir := filepath.Join(config, "go", "telemetry")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mode"), []byte("off"), 0o600); err != nil {
		t.Fatal(err)
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

	processes := []procscan.Process{
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

// TestTestTempOpenFailureNamesItsFix proves the row error of an old entry
// that cannot be opened.
//
// VALIDATES: a permission refusal, the shape a root-owned leftover of a
// `sudo -E` run gives (EPERM on chmod), names `sudo chown -R $USER <base>`;
// any other failure is reported as it is, with no chown remedy.
// PREVENTS: an hourly trim red on every run with no hint that root made the
// entry. A non-root test cannot make a directory it does not own, so the
// refusal is built as chmod returns it.
func TestTestTempOpenFailureNamesItsFix(t *testing.T) {
	base := "/home/u/.zt"
	refused := &fs.PathError{Op: "chmod", Path: base + "/TestRoot1", Err: syscall.EPERM}
	if got := testTempOpenFailure(refused, base); got != refused.Error()+"; sudo chown -R $USER "+base {
		t.Errorf("testTempOpenFailure(EPERM) = %q, want the error and the chown remedy", got)
	}
	broken := &fs.PathError{Op: "lstat", Path: base + "/TestIO1", Err: syscall.EIO}
	if got := testTempOpenFailure(broken, base); got != broken.Error() {
		t.Errorf("testTempOpenFailure(EIO) = %q, want %q", got, broken.Error())
	}
}

// lockedEntry makes a test temp entry holding a 0o000 directory with a file
// in it, as a killed permission test leaves it, and ages the entry. Cleanup
// opens the directory again so t.TempDir() can remove whatever is left.
func lockedEntry(t *testing.T, path string, now time.Time, age time.Duration) string {
	t.Helper()
	locked := filepath.Join(path, "locked")
	if err := os.MkdirAll(locked, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "data"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(locked, 0o700); err != nil && !errors.Is(err, fs.ErrNotExist) { //nolint:gosec // the test must open its own 0o000 directory to remove it
			t.Error(err)
		}
	})
	return agedDir(t, path, now, age)
}

// TestTrimTestTempLeftovers proves the test temp row of the store trim.
//
// VALIDATES: a go-build* work directory past goBuildIdleAge and any other
// entry past testTempIdleAge are removed; a go-build* entry younger than its
// floor and a t.TempDir() parent past goBuildIdleAge but younger than
// testTempIdleAge are kept, and the row counts both. An old entry holding a
// 0o000 directory is opened and removed with no error; a young one is kept
// and named as contention, never as the row's error. A home whose base cannot
// be used is a skip that says why, never a trim of some other directory.
// PREVENTS: the temp dirs of killed test runs piling up under ~/.zt, which no
// test binary removes once its process is gone, a live run losing its temp
// dir to the shorter go-build floor, and a killed permission test's leftover
// turning every hourly trim red.
func TestTrimTestTempLeftovers(t *testing.T) {
	now := time.Now()
	base := t.TempDir()
	stale := agedDir(t, filepath.Join(base, "TestKilled123"), now, testTempIdleAge+time.Hour)
	work := agedDir(t, filepath.Join(base, "go-build456"), now, goBuildIdleAge+time.Hour)
	building := agedDir(t, filepath.Join(base, "go-build789"), now, 30*time.Minute)
	slow := agedDir(t, filepath.Join(base, "TestSlow321"), now, goBuildIdleAge+time.Hour)
	live := agedDir(t, filepath.Join(base, "TestRunning789"), now, time.Hour)
	var removed []string
	row := StoreTrim{Name: testTempStore, Path: base}
	trimTestTemp(t.Context(), &row, &livenessPass{now: now, remove: recordingRemove(t, &removed)})
	if got, want := sortedBases(removed), sortedBases([]string{stale, work}); !slices.Equal(got, want) {
		t.Errorf("removed %v, want %v", got, want)
	}
	for _, kept := range []string{building, slow, live} {
		if !exists(t, kept) {
			t.Errorf("%s was removed while younger than its floor", kept)
		}
	}
	if row.EntriesRemoved != 2 || row.Kept != 3 || row.Error != "" {
		t.Errorf("test temp row = %+v, want 2 removed, 3 kept, no error", row)
	}

	t.Run("a 0o000 leftover is removed when old and contention when young", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a 0o000 directory, so no EACCES can arise")
		}
		base := t.TempDir()
		lockedOld := lockedEntry(t, filepath.Join(base, "TestLocked111"), now, testTempIdleAge+time.Hour)
		lockedYoung := lockedEntry(t, filepath.Join(base, "TestHolding222"), now, time.Hour)
		var removed []string
		row := StoreTrim{Name: testTempStore, Path: base}
		trimTestTemp(t.Context(), &row, &livenessPass{now: now, remove: recordingRemove(t, &removed)})
		if exists(t, lockedOld) {
			t.Errorf("%s, older than %s, was not removed", lockedOld, testTempIdleAge)
		}
		if !exists(t, lockedYoung) {
			t.Errorf("%s was removed while younger than %s", lockedYoung, testTempIdleAge)
		}
		if row.Error != "" {
			t.Errorf("test temp row error = %q, want none: a 0o000 leftover is no failure", row.Error)
		}
		if row.Contended != lockedYoung || row.EntriesRemoved != 1 || row.Kept != 1 {
			t.Errorf("test temp row = %+v, want 1 removed, 1 kept, contended %s", row, lockedYoung)
		}
	})

	t.Setenv("HOME", "relative")
	if skipped := testTempRow(); skipped.Skipped == "" || skipped.Path != "" {
		t.Errorf("test temp row for a relative home = %+v, want a skip with no path", skipped)
	}
	t.Setenv("HOME", "/h")
	if named := testTempRow(); named.Path != "/h/.zt" || named.Skipped != "" {
		t.Errorf("test temp row for /h = %+v, want path /h/.zt", named)
	}
}

// agedBuild makes a runnable build directory, bin/builds/<name>/le, both aged.
func agedBuild(t *testing.T, dir string, now time.Time, age time.Duration) {
	t.Helper()
	agedFile(t, filepath.Join(dir, "le"), now, age)
	agedDir(t, dir, now, age)
}

// trimRowIn runs the liveness pass over root with a Claude CLI visible plus
// extra, and answers the named row and the sorted bases of what it removed.
func trimRowIn(t *testing.T, root, store string, now time.Time, extra ...procscan.Process) (StoreTrim, []string) {
	t.Helper()
	var removed []string
	rows := trimLiveness(t.Context(), root, &livenessPass{
		now: now, processes: append([]procscan.Process{claudeCLI(now)}, extra...), judge: judgeIn(t, root),
		startedAt: func(int) time.Time { return time.Time{} }, remove: recordingRemove(t, &removed),
	})
	return rowNamed(t, rows, store), sortedBases(removed)
}

// TestTrimKeepsNewestTwoBuilds proves the build retention rule on injected
// process facts.
//
// VALIDATES: spec-le-built-from-head AC-12. Per platform, the newest two
// runnable builds stay and older ones go, unless a running process names one;
// a build-named directory with no le, the lock and an unknown entry stay; the
// legacy binary goes once a build of this checkout's platform exists.
// PREVENTS: bin/builds growing by one build per commit forever, and the trim
// removing the build the launcher runs, the one rollback falls back to, or a
// superseded build a long command still runs.
func TestTrimKeepsNewestTwoBuilds(t *testing.T) {
	now := time.Now()
	root := t.TempDir()
	builds := gotoolchain.BuildsDir(root)
	day := 25 * time.Hour
	for _, name := range []string{
		"20261001T000000Z-aaaaaaa", "20261002T000000Z-bbbbbbb", "20261003T000000Z-ccccccc",
		"20261004T000000Z-ddddddd", "20261005T000000Z-eeeeeee",
		"20261001T000000Z-aaaaaaa-linux-arm64", "20261002T000000Z-bbbbbbb-linux-arm64",
		"20261003T000000Z-ccccccc-linux-arm64",
	} {
		agedBuild(t, filepath.Join(builds, name), now, day)
	}
	agedDir(t, filepath.Join(builds, "20260901T000000Z-0000000"), now, day)
	agedFile(t, filepath.Join(builds, gotoolchain.BuildLockEntry, "pid"), now, day)
	agedFile(t, filepath.Join(builds, ".other", "x"), now, day)
	agedFile(t, filepath.Join(builds, gotoolchain.BuildLegacyEntry, "le"), now, day)
	running := procscan.Process{PID: 2, Start: "r", Argv: []string{filepath.Join(builds, "20261002T000000Z-bbbbbbb", "le"), "go", "test"}}

	row, removed := trimRowIn(t, root, buildsStore, now, running)
	want := []string{gotoolchain.BuildLegacyEntry, "20261001T000000Z-aaaaaaa", "20261001T000000Z-aaaaaaa-linux-arm64", "20261003T000000Z-ccccccc"}
	slices.Sort(want)
	if !slices.Equal(removed, want) {
		t.Errorf("removed %v, want %v", removed, want)
	}
	if row.EntriesRemoved != len(want) || row.Error != "" {
		t.Errorf("builds row = %+v, want %d removed and no error", row, len(want))
	}
	for _, kept := range []string{
		"20261002T000000Z-bbbbbbb", "20261004T000000Z-ddddddd", "20261005T000000Z-eeeeeee",
		"20261002T000000Z-bbbbbbb-linux-arm64", "20261003T000000Z-ccccccc-linux-arm64",
		"20260901T000000Z-0000000", gotoolchain.BuildLockEntry, ".other",
	} {
		if !exists(t, filepath.Join(builds, kept)) {
			t.Errorf("bin/builds/%s was removed", kept)
		}
	}
	runnable, err := gotoolchain.RunnableBuild(filepath.Join(builds, "20261005T000000Z-eeeeeee"))
	if err != nil {
		t.Fatalf("RunnableBuild after the trim: %v", err)
	}
	if !runnable {
		t.Errorf("the newest build 20261005T000000Z-eeeeeee is no longer runnable after the trim")
	}

	t.Run("the legacy binary stays while it is the only le of this platform, or runs", func(t *testing.T) {
		root := t.TempDir()
		builds := gotoolchain.BuildsDir(root)
		agedBuild(t, filepath.Join(builds, "20261001T000000Z-aaaaaaa-linux-arm64"), now, day)
		agedFile(t, filepath.Join(builds, gotoolchain.BuildLegacyEntry, "le"), now, day)
		if _, removed := trimRowIn(t, root, buildsStore, now); len(removed) != 0 {
			t.Errorf("removed %v with no build of this checkout's platform", removed)
		}
		agedBuild(t, filepath.Join(builds, "20261001T000000Z-aaaaaaa"), now, day)
		legacy := procscan.Process{PID: 3, Start: "l", Argv: []string{filepath.Join(builds, gotoolchain.BuildLegacyEntry, "le"), "spec", "status"}}
		if _, removed := trimRowIn(t, root, buildsStore, now, legacy); len(removed) != 0 {
			t.Errorf("removed %v while a process runs the legacy binary", removed)
		}
	})
}

// TestTrimBuildLeftovers proves the rules for what the launcher leaves beside
// its builds.
//
// VALIDATES: spec-le-built-from-head AC-12. A staging directory goes once
// nothing in it was written for an hour and no process (its compiler) names
// it; a rolled-back build goes once a newer build of its platform landed; the
// record of a HEAD that did not build goes once a build of another commit
// landed after it; an unknown .failed- file stays.
// PREVENTS: a dead builder's half-compiled tree staying forever, and the trim
// removing the record that stops the launcher rebuilding a rejected HEAD on
// every call.
func TestTrimBuildLeftovers(t *testing.T) {
	now := time.Now()
	root := t.TempDir()
	builds := gotoolchain.BuildsDir(root)
	hour := time.Hour
	agedBuild(t, filepath.Join(builds, "20261002T000000Z-bbbbbbb"), now, 4*hour)
	agedBuild(t, filepath.Join(builds, "20261003T000000Z-ccccccc"), now, 2*hour)
	staging := func(pid, file string, fileAge, dirAge time.Duration) {
		dir := filepath.Join(builds, gotoolchain.BuildStagingPrefix+pid)
		if file != "" {
			agedFile(t, filepath.Join(dir, file), now, fileAge)
		}
		agedDir(t, dir, now, dirAge)
	}
	staging("11", "", 0, 10*time.Minute)
	staging("12", "young.go", 10*time.Minute, 2*hour)
	staging("13", "le", 2*hour, 2*hour)
	staging("14", "le", 2*hour, 2*hour)
	aside := func(name string) {
		agedBuild(t, filepath.Join(builds, gotoolchain.BuildFailedPrefix+name), now, 3*hour)
	}
	aside("20261001T000000Z-aaaaaaa")
	aside("20261001T000000Z-aaaaaaa.77")
	aside("20261004T000000Z-ddddddd")
	aside("20261001T000000Z-aaaaaaa-linux-arm64")
	other := strings.Repeat("1", 40)
	recent := strings.Repeat("2", 40)
	record := func(name string, age time.Duration) {
		agedFile(t, filepath.Join(builds, gotoolchain.BuildFailedPrefix+name), now, age)
	}
	record(other+".log", 3*hour)
	record(other+".reported", 3*hour)
	record(recent+".log", hour)
	record("ccccccc"+strings.Repeat("0", 33)+".log", 3*hour)
	record(other+".unknown", 3*hour)
	compiling := procscan.Process{PID: 4, Start: "g", Argv: []string{"compile", "-o", filepath.Join(builds, gotoolchain.BuildStagingPrefix+"14", "le")}}

	row, removed := trimRowIn(t, root, buildsStore, now, compiling)
	want := []string{
		gotoolchain.BuildFailedPrefix + other + ".log", gotoolchain.BuildFailedPrefix + other + ".reported",
		gotoolchain.BuildFailedPrefix + "20261001T000000Z-aaaaaaa", gotoolchain.BuildFailedPrefix + "20261001T000000Z-aaaaaaa.77",
		gotoolchain.BuildStagingPrefix + "13",
	}
	slices.Sort(want)
	if !slices.Equal(removed, want) {
		t.Errorf("removed %v, want %v", removed, want)
	}
	if row.Error != "" {
		t.Errorf("builds row error %q", row.Error)
	}
}

// TestTrimRemovesIdleAgentBinFiles proves the agent-files rule on injected
// process facts.
//
// VALIDATES: spec-le-built-from-head AC-13 (O-8). An entry directly in bin/
// that is not le's or ze's goes once nothing in it was written for a day and
// no running process names it; le, le-*, le.*, ze*, builds, a young entry, a
// directory with one young file, a named one and a symbolic link stay.
// PREVENTS: agents' mutation-test binaries and probes filling bin/
// (2026-10-25: gomu, gomu-rfc-99e7, gomu-9b5c1d68-r2/, rfc-vet-probe), and the
// trim removing a binary somebody runs or one of le's own.
func TestTrimRemovesIdleAgentBinFiles(t *testing.T) {
	now := time.Now()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	day := 25 * time.Hour
	for _, file := range []string{"gomu", "gomu-rfc-99e7", "rfc-vet-probe", "le", "le.new.5", "ze", "ze-ospfvf"} {
		agedFile(t, filepath.Join(bin, file), now, day)
	}
	agedFile(t, filepath.Join(bin, "young-probe"), now, time.Hour)
	tree := func(name, leaf string, leafAge time.Duration) {
		agedFile(t, filepath.Join(bin, name, "deep", leaf), now, leafAge)
		agedDir(t, filepath.Join(bin, name, "deep"), now, day)
		agedDir(t, filepath.Join(bin, name), now, day)
	}
	tree("gomu-9b5c1d68-r2", "old", day)
	tree("gomu-live", "young", time.Hour)
	tree("le-named", "le", time.Hour)
	agedBuild(t, filepath.Join(gotoolchain.BuildsDir(root), "20261005T000000Z-eeeeeee"), now, day)
	agedDir(t, gotoolchain.BuildsDir(root), now, day)
	target := agedDir(t, filepath.Join(root, "elsewhere"), now, day)
	if err := os.Symlink(target, filepath.Join(bin, "linked")); err != nil {
		t.Fatal(err)
	}
	running := procscan.Process{PID: 5, Start: "m", Argv: []string{filepath.Join(bin, "gomu-rfc-99e7"), "run"}}

	row, removed := trimRowIn(t, root, agentFilesStore, now, running)
	want := []string{"gomu", "gomu-9b5c1d68-r2", "rfc-vet-probe"}
	if !slices.Equal(removed, want) {
		t.Errorf("removed %v, want %v", removed, want)
	}
	if row.EntriesRemoved != len(want) || row.Error != "" {
		t.Errorf("agent-files row = %+v, want %d removed and no error", row, len(want))
	}
	if row.Kept != 4 {
		t.Errorf("agent-files row kept %d, want 4 (gomu-rfc-99e7, young-probe, gomu-live, linked): le's and ze's entries are not judged", row.Kept)
	}
	for _, kept := range []string{"gomu-rfc-99e7", "young-probe", "gomu-live", "le", "le.new.5", "ze", "ze-ospfvf", "le-named", "builds", "linked"} {
		if !exists(t, filepath.Join(bin, kept)) {
			t.Errorf("bin/%s was removed", kept)
		}
	}
	if !exists(t, target) {
		t.Error("the target of a symbolic link under bin/ was removed")
	}
}

// TestPathNamedInArgvNeedsWholeNames proves the argv match the liveness rows
// share stops at name boundaries.
//
// VALIDATES: a path matches as whole names, after a separator or `=` and
// before a separator, a colon or the end; never inside a longer name.
// PREVENTS: a process running bin/gomu-rfc keeping bin/gomu forever, or one
// running bin/gomu failing to keep it.
func TestPathNamedInArgvNeedsWholeNames(t *testing.T) {
	for _, tc := range []struct {
		argument string
		want     bool
	}{
		{"bin/gomu", true},
		{"/abs/repo/bin/gomu", true},
		{"-o=bin/gomu", true},
		{"/abs/repo/bin/gomu/le", true},
		{"bin/gomu:rest", true},
		{"bin/gomu-rfc", false},
		{"bin/gomu.log", false},
		{"cabin/gomu", false},
		{"cabin/gomu bin/gomu", true},
		{"bin/gom", false},
	} {
		got := pathNamedInArgv([]procscan.Process{{Argv: []string{tc.argument}}}, "bin/gomu")
		if got != tc.want {
			t.Errorf("pathNamedInArgv(%q, bin/gomu) = %v, want %v", tc.argument, got, tc.want)
		}
	}
}
