package sessionpath

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

// testSID is the session id every test below resolves paths for.
const testSID = "sid-one"

// ownedSessionDir is the directory testSID owns under base, found by reading
// each directory's .session-id, after a call that resolved it. before is the
// time captured ahead of that call: the name must be the creation time, so it
// lies between before (truncated to the second) and now.
//
// Spelled out here rather than taken from Root, so the test states the layout
// (<YYYY-MM-DD>-<HHMMSS>, owner in .session-id) instead of agreeing with
// whatever the code computed.
func ownedSessionDir(t *testing.T, base string, before time.Time) string {
	t.Helper()
	after := time.Now()
	root := filepath.Join(base, "tmp", "session")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read the session root: %v", err)
	}
	for _, entry := range entries {
		owner, err := os.ReadFile(filepath.Join(root, entry.Name(), ".session-id"))
		if err != nil || string(owner) != testSID+"\n" {
			continue
		}
		named, err := time.ParseInLocation("2006-01-02-150405", entry.Name(), time.Local)
		if err != nil {
			t.Fatalf("session directory %q is not named <YYYY-MM-DD>-<HHMMSS>: %v", entry.Name(), err)
		}
		if named.Before(before.Truncate(time.Second)) || named.After(after) {
			t.Fatalf("session directory %q is not named by its creation time, between %s and %s",
				entry.Name(), before.Truncate(time.Second).Format(time.RFC3339), after.Format(time.RFC3339))
		}
		return filepath.Join(root, entry.Name())
	}
	t.Fatalf("no directory under %s records %s in .session-id: %v", root, testSID, entries)
	return ""
}

// setSession points the resolver at id for the duration of one test.
// env caches os.Environ() on first read, so the cache must be reset too
// (ai/rules/go-standards.md, "Cache").
func setSession(t *testing.T, id string) {
	t.Helper()
	t.Setenv("ZE_SESSION_ID", id)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// VALIDATES: AC-4 -- two concurrent sessions never share a binary directory.
// PREVENTS: one session's build overwriting another session's binary mid-test.
func TestSessionBinDirIsolatesSessions(t *testing.T) {
	base := t.TempDir()

	setSession(t, "sid-one")
	before := time.Now()
	first := BinDir(base)

	setSession(t, "sid-two")
	second := BinDir(base)

	if first == second {
		t.Fatalf("two sessions share a bin dir: %q", first)
	}
	for _, got := range []string{first, second} {
		if filepath.Base(got) != "bin" {
			// ze derives its config/DB dir from a parent dir named bin/sbin
			// (internal/core/paths/paths.go isBinDir); anything else breaks
			// commands like `ze config archive`.
			t.Errorf("bin dir must be named bin, got %q", got)
		}
	}
	if want := filepath.Join(ownedSessionDir(t, base, before), "bin"); first != want {
		t.Errorf("BinDir = %q, want %q", first, want)
	}
}

// VALIDATES: AC-2, AC-6 -- humans and CI keep today's exact paths.
// PREVENTS: changing behavior for anyone not running inside an AI session.
func TestSessionPathsFallBackToShared(t *testing.T) {
	base := t.TempDir()
	setSession(t, "")

	if got, want := BinDir(base), filepath.Join(base, "bin"); got != want {
		t.Errorf("BinDir = %q, want shared %q", got, want)
	}
	// "" is what os.MkdirTemp treats as "use the system temp dir", so an
	// off-session runner keeps its current scratch location unchanged.
	if got := scratchRoot(base); got != "" {
		t.Errorf("scratchRoot = %q, want %q (system temp)", got, "")
	}
	if got := ID(); got != "" {
		t.Errorf("ID = %q, want empty", got)
	}
}

// VALIDATES: AC-10 -- an unsafe id can never escape the scratch root.
// PREVENTS: path traversal via a hostile or malformed session id.
func TestSessionPathsRejectUnsafeID(t *testing.T) {
	base := t.TempDir()
	for _, id := range []string{"..", ".", "a/b", "../../etc", "with space", "semi;colon"} {
		setSession(t, id)
		if got := ID(); got != "" {
			t.Errorf("ID(%q) = %q, want rejected", id, got)
		}
		if got, want := BinDir(base), filepath.Join(base, "bin"); got != want {
			t.Errorf("BinDir with unsafe id %q = %q, want shared %q", id, got, want)
		}
	}
}

// VALIDATES: AC-8 -- test runtime scratch is owned by the session.
// PREVENTS: orphaned $TMPDIR/ze-* dirs nobody can attribute or clean.
func TestScratchRootUnderSession(t *testing.T) {
	base := t.TempDir()
	setSession(t, "sid-one")

	before := time.Now()
	got := scratchRoot(base)
	if want := ownedSessionDir(t, base, before); got != want {
		t.Errorf("scratchRoot = %q, want %q", got, want)
	}
	if got, want := ID(), "sid-one"; got != want {
		t.Errorf("ID = %q, want %q", got, want)
	}
}

// VALIDATES: AC-13 -- the dated directory is LOOKED UP, never recomputed.
// PREVENTS: a resolver that rebuilds <today>-<id> from the clock moving a
// session's directory at midnight and orphaning the binaries it is running.
func TestRootPrefersAnExistingDatedDirectory(t *testing.T) {
	base := t.TempDir()
	setSession(t, "sid-one")

	root := filepath.Join(base, "tmp", "session")
	planted := filepath.Join(root, time.Now().AddDate(0, 0, -1).Format("2006-01-02")+"-sid-one")
	if err := os.MkdirAll(planted, 0o750); err != nil {
		t.Fatal(err)
	}

	if got := Root(base); got != planted {
		t.Errorf("Root = %q, want the directory this session already owns %q", got, planted)
	}
	if got, want := BinDir(base), filepath.Join(planted, "bin"); got != want {
		t.Errorf("BinDir = %q, want %q", got, want)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("resolving the session directory created another one: %v", entries)
	}
}

// VALIDATES: Root creates the session directory the first time it is asked,
// named <YYYY-MM-DD>-<HHMMSS> and owned through .session-id, and answers that
// same directory on every later call; scratch is not created by Root.
// PREVENTS: a creation-time name being recomputed on each call, which would
// hand one session a new directory every second.
func TestRootCreatesTheDirectoryOnceAndOwnsIt(t *testing.T) {
	base := t.TempDir()
	setSession(t, "sid-one")

	before := time.Now()
	first := Root(base)
	if want := ownedSessionDir(t, base, before); first != want {
		t.Errorf("Root = %q, want %q", first, want)
	}
	if again := Root(base); again != first {
		t.Errorf("second Root = %q, want the first %q", again, first)
	}
	if _, err := os.Stat(filepath.Join(first, "scratch")); !os.IsNotExist(err) {
		t.Errorf("Root created scratch: stat err = %v", err)
	}
}

// VALIDATES: the CLAUDE_CODE_SESSION_ID fallback (A-2).
// PREVENTS: Go and Make disagreeing when only the CLI variable is exported.
func TestClaudeSessionIDFallback(t *testing.T) {
	t.Setenv("ZE_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "from-claude")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	if got, want := ID(), "from-claude"; got != want {
		t.Errorf("ID = %q, want %q", got, want)
	}
}

// VALIDATES: AC-8 via the anchor that actually applies -- the CLI does NOT export
// CLAUDE_PROJECT_DIR into the shell that runs make, so the go.mod walk is what
// routes runner scratch into the session dir in practice.
// PREVENTS: the routing silently going inert and leaving unowned $TMPDIR/ze-* dirs.
func TestDefaultScratchRootFallsBackToRepoRoot(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte(moduleDirective+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "internal", "deep")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	t.Setenv("CLAUDE_PROJECT_DIR", "")
	setSession(t, "sid-one")

	before := time.Now()
	got := DefaultScratchRoot()
	want := ownedSessionDir(t, repo, before)
	// t.TempDir may hand back a symlinked path (/var -> /private/var on macOS);
	// compare resolved paths so the assertion is about the location, not the spelling.
	gotResolved, _ := filepath.EvalSymlinks(got)
	wantResolved, _ := filepath.EvalSymlinks(want)
	if gotResolved != wantResolved {
		t.Errorf("DefaultScratchRoot = %q, want %q", got, want)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("scratch root not created: %v", err)
	}
}

// VALIDATES: AC-2/AC-6 for the fallback path -- off-session it must stay "" even
// when a checkout root IS findable, so humans and CI keep the system temp dir.
func TestDefaultScratchRootEmptyOffSession(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte(moduleDirective+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	setSession(t, "")

	if got := DefaultScratchRoot(); got != "" {
		t.Errorf("DefaultScratchRoot = %q, want %q off-session", got, "")
	}
}

// VALIDATES: LE_TEST_NO_BUILD keeps working under a session when the binaries
// were pre-built into the SHARED bin/ by a manual or cross-compile.
// PREVENTS: session scoping turning a good prebuilt binary into a "missing
// binary" error -- scoping is about builds clobbering, not about reads.
func TestFindPrebuiltDirPrefersSessionThenShared(t *testing.T) {
	base := t.TempDir()
	shared := filepath.Join(base, "bin")
	if err := os.MkdirAll(shared, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"ze", "ze-stripped"} {
		if err := os.WriteFile(filepath.Join(shared, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	setSession(t, "sid-one")

	if got, want := FindPrebuiltDir(base, "ze", "ze-stripped"), shared; got != want {
		t.Errorf("FindPrebuiltDir = %q, want shared %q", got, want)
	}

	// A session dir holding only SOME of the names must not win: .ci tests exec
	// siblings by bare name off one directory, so a partial set would strand them.
	sessionBin := BinDir(base)
	if err := os.MkdirAll(sessionBin, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionBin, "ze"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := FindPrebuiltDir(base, "ze", "ze-stripped"), shared; got != want {
		t.Errorf("partial session dir won: FindPrebuiltDir = %q, want %q", got, want)
	}

	// Complete the session set -> it wins.
	if err := os.WriteFile(filepath.Join(sessionBin, "ze-stripped"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := FindPrebuiltDir(base, "ze", "ze-stripped"), sessionBin; got != want {
		t.Errorf("FindPrebuiltDir = %q, want session %q", got, want)
	}

	// Nothing anywhere -> "" so the caller reports the miss.
	if got := FindPrebuiltDir(base, "ze-nonexistent"); got != "" {
		t.Errorf("FindPrebuiltDir = %q, want empty", got)
	}
}

// VALIDATES: the checkout root is found past tmp/'s tracked sentinel module.
// PREVENTS: repoRoot stopping at tmp/go.mod (`module ze-tmp-scratch`, written by
// internal/le/scratch/move.go so `go list ./...` skips the caches) and reporting
// tmp/ as the root -- which puts the scratch root at
// tmp/tmp/session/<date>-<HHMMSS>, a real directory returned with no error and
// owned by nothing.
func TestRepoRootSkipsTmpSentinelModule(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte(moduleDirective+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The sentinel: a REAL go.mod for a different module, exactly as the repo ships.
	session := filepath.Join(repo, "tmp", "session", "2026-10-30-"+testSID)
	if err := os.MkdirAll(session, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "tmp", "go.mod"), []byte("module ze-tmp-scratch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(session)

	got, _ := filepath.EvalSymlinks(repoRoot())
	want, _ := filepath.EvalSymlinks(repo)
	if got != want {
		t.Errorf("repoRoot from under tmp/ = %q, want the checkout root %q", got, want)
	}

	t.Setenv("CLAUDE_PROJECT_DIR", "")
	setSession(t, "sid-one")
	scratch, _ := filepath.EvalSymlinks(DefaultScratchRoot())
	wantScratch, _ := filepath.EvalSymlinks(session)
	if scratch != wantScratch {
		t.Errorf("DefaultScratchRoot = %q, want %q (not nested under tmp/)", scratch, wantScratch)
	}
}

// VALIDATES: asking for no binaries finds no directory.
// PREVENTS: the "every name present" loop being vacuously true, so the first
// candidate is reported as a hit even when it does not exist.
func TestFindPrebuiltDirNoNames(t *testing.T) {
	base := t.TempDir()
	setSession(t, "sid-one")
	if got := FindPrebuiltDir(base); got != "" {
		t.Errorf("FindPrebuiltDir with no names = %q, want empty", got)
	}
}
