package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/scratch"
)

// VALIDATES: the native ensure gate has the producer's exact quiet output, exit, links, and sentinel.
// PREVENTS: the duplicate-then-swap changing a prerequisite that normally prints nothing.
func TestScratchEnsureParity(t *testing.T) {
	fixture := newScratchParityFixture(t)
	setup := func() {
		fixture.reset(t)
	}

	setup()
	old := fixture.runPython(t, "--quiet")
	oldState := fixture.state(t)
	if old.code != 0 || old.stdout != "" || old.stderr != "" {
		t.Fatalf("producer ensure = %#v, want silent exit 0", old)
	}

	setup()
	port := fixture.runGo(false)
	portState := fixture.state(t)
	assertScratchParity(t, old, port, oldState, portState)
}

// VALIDATES: both ensure halves refuse a real tmp directory and write the byte-exact sentinel.
// PREVENTS: the port converting session work or dropping the nested-module guard.
func TestScratchRealDirectoryAndSentinelParity(t *testing.T) {
	fixture := newScratchParityFixture(t)
	setup := func() {
		fixture.reset(t)
		if err := os.MkdirAll(filepath.Join(fixture.root, "tmp"), 0o755); err != nil {
			t.Fatalf("mkdir tmp: %v", err)
		}
		fixture.write(t, filepath.Join(fixture.root, "tmp", "session"), "mine")
	}

	setup()
	old := fixture.runPython(t, "--quiet")
	oldState := fixture.state(t)
	if old.code != 0 {
		t.Fatalf("producer ensure exit = %d", old.code)
	}
	wantStderr := "SKIP     tmp: a real path exists here; run `make ze-scratch-migrate` to convert it to a symlink\n"
	manager := scratch.New(fixture.root, fixture.environment())
	cacheTarget, err := manager.CacheTarget()
	if err != nil {
		t.Fatalf("cache target: %v", err)
	}
	wantStdout := "created  cache -> " + cacheTarget + "\n"
	if old.stderr != wantStderr || old.stdout != wantStdout {
		t.Fatalf("producer output = stdout %q stderr %q, want stdout %q stderr %q", old.stdout, old.stderr, wantStdout, wantStderr)
	}

	setup()
	port := fixture.runGo(false)
	portState := fixture.state(t)
	assertScratchParity(t, old, port, oldState, portState)
	if got := fixture.read(t, filepath.Join(fixture.root, "tmp", "go.mod")); got != scratch.Sentinel {
		t.Fatalf("port sentinel differs:\n%s", got)
	}
}

// VALIDATES: both migration halves skip a symlinked tmp and move the complete real cache.
// PREVENTS: cache migration losing entries, changing status text, or returning the wrong code.
func TestScratchWholeCacheMigrationParity(t *testing.T) {
	fixture := newScratchParityFixture(t)
	setup := func() {
		fixture.reset(t)
		target := scratch.New(fixture.root, fixture.environment()).ScratchTarget()
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatalf("mkdir scratch target: %v", err)
		}
		if err := os.Symlink(target, filepath.Join(fixture.root, "tmp")); err != nil {
			t.Fatalf("symlink tmp: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(fixture.root, "cache"), 0o755); err != nil {
			t.Fatalf("mkdir cache: %v", err)
		}
		fixture.write(t, filepath.Join(fixture.root, "cache", "artifact"), "cache")
	}

	setup()
	old := fixture.runPython(t, "--migrate")
	oldState := fixture.state(t)
	if old.code != 0 {
		t.Fatalf("producer migrate = %#v", old)
	}

	setup()
	port := fixture.runGo(true)
	portState := fixture.state(t)
	assertScratchParity(t, old, port, oldState, portState)
}

// VALIDATES: both migration halves refuse a colliding cache entry without overwriting either copy.
// PREVENTS: parity on text hiding destructive filesystem drift.
func TestScratchCollisionParity(t *testing.T) {
	fixture := newScratchParityFixture(t)
	setup := func() {
		fixture.reset(t)
		target := scratch.New(fixture.root, fixture.environment()).ScratchTarget()
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatalf("mkdir scratch target: %v", err)
		}
		if err := os.Symlink(target, filepath.Join(fixture.root, "tmp")); err != nil {
			t.Fatalf("symlink tmp: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(fixture.root, "cache"), 0o755); err != nil {
			t.Fatalf("mkdir cache: %v", err)
		}
		fixture.write(t, filepath.Join(fixture.root, "cache", "collision"), "source")
		fixture.write(t, filepath.Join(fixture.home, ".cache", "ze", "collision"), "user")
	}

	setup()
	old := fixture.runPython(t, "--migrate")
	oldState := fixture.state(t)
	if old.code != 1 {
		t.Fatalf("producer collision = %#v, want exit 1", old)
	}

	setup()
	port := fixture.runGo(true)
	portState := fixture.state(t)
	assertScratchParity(t, old, port, oldState, portState)
	if got := fixture.read(t, filepath.Join(fixture.root, "cache", "collision")); got != "source" {
		t.Errorf("port source collision = %q", got)
	}
	if got := fixture.read(t, filepath.Join(fixture.home, ".cache", "ze", "collision")); got != "user" {
		t.Errorf("port target collision = %q", got)
	}
}

type scratchParityFixture struct {
	base   string
	root   string
	tmpdir string
	home   string
	script string
}

type scratchParityAnswer struct {
	stdout string
	stderr string
	code   int
}

type scratchParityState struct {
	tmpLink       string
	cacheLink     string
	tmpSentinel   string
	cacheArtifact string
	cacheSource   string
	cacheTarget   string
}

func newScratchParityFixture(t *testing.T) scratchParityFixture {
	t.Helper()
	base := t.TempDir()
	script, err := filepath.Abs("ensure-links.py")
	if err != nil {
		t.Fatalf("script path: %v", err)
	}
	return scratchParityFixture{
		base: base, root: filepath.Join(base, "checkout"),
		tmpdir: filepath.Join(base, "scratch"), home: filepath.Join(base, "home"),
		script: script,
	}
}

func (f scratchParityFixture) reset(t *testing.T) {
	t.Helper()
	for _, path := range []string{f.root, f.tmpdir, f.home} {
		if err := os.RemoveAll(path); err != nil {
			t.Fatalf("remove %s: %v", path, err)
		}
	}
	if err := os.MkdirAll(f.root, 0o755); err != nil {
		t.Fatalf("mkdir checkout: %v", err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	cmd := exec.CommandContext(context.Background(), "git", "init", "-q") //nolint:gosec // test fixture
	cmd.Dir = f.root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
}

func (f scratchParityFixture) environment() []string {
	environment := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "TMPDIR=") || strings.HasPrefix(entry, "HOME=") || strings.HasPrefix(entry, "XDG_CACHE_HOME=") {
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment, "TMPDIR="+f.tmpdir, "HOME="+f.home)
}

func (f scratchParityFixture) runPython(t *testing.T, argument string) scratchParityAnswer {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not on PATH")
	}
	cmd := exec.CommandContext(context.Background(), "python3", f.script, argument) //nolint:gosec // producer under comparison
	cmd.Dir = f.root
	cmd.Env = f.environment()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("run producer: %v", err)
		}
		code = exit.ExitCode()
	}
	return scratchParityAnswer{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func (f scratchParityFixture) runGo(migrate bool) scratchParityAnswer {
	manager := scratch.New(f.root, f.environment())
	var report scratch.Report
	var code int
	if migrate {
		report, code = manager.Migrate(false)
	} else {
		report, code = manager.Ensure(false)
	}
	var stderr strings.Builder
	for _, result := range report.Results {
		if result.Stderr {
			stderr.WriteString(result.Line)
			stderr.WriteByte('\n')
		}
	}
	return scratchParityAnswer{stdout: report.Text(), stderr: stderr.String(), code: code}
}

func (f scratchParityFixture) state(t *testing.T) scratchParityState {
	t.Helper()
	manager := scratch.New(f.root, f.environment())
	cacheTarget, err := manager.CacheTarget()
	if err != nil {
		t.Fatalf("cache target: %v", err)
	}
	return scratchParityState{
		tmpLink:       readlinkOrEmpty(filepath.Join(f.root, "tmp")),
		cacheLink:     readlinkOrEmpty(filepath.Join(f.root, "cache")),
		tmpSentinel:   readOrEmpty(filepath.Join(f.root, "tmp", "go.mod")),
		cacheArtifact: readOrEmpty(filepath.Join(cacheTarget, "artifact")),
		cacheSource:   readOrEmpty(filepath.Join(f.root, "cache", "collision")),
		cacheTarget:   readOrEmpty(filepath.Join(cacheTarget, "collision")),
	}
}

func (f scratchParityFixture) write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func (f scratchParityFixture) read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path) //nolint:gosec // test-owned fixture
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func readlinkOrEmpty(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return target
}

func readOrEmpty(path string) string {
	body, err := os.ReadFile(path) //nolint:gosec // test-owned fixture
	if err != nil {
		return ""
	}
	return string(body)
}

func assertScratchParity(t *testing.T, old, port scratchParityAnswer, oldState, portState scratchParityState) {
	t.Helper()
	if port != old {
		t.Errorf("port answer = %#v, producer = %#v", port, old)
	}
	if portState != oldState {
		t.Errorf("port state = %#v, producer = %#v", portState, oldState)
	}
}
