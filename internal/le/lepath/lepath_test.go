// The root is what every le tool resolves a path against, so a wrong answer
// here is a tool reading or writing the wrong tree. These cases pin the three
// ways it can be wrong: the environment ignored, a directory that is not a
// checkout accepted, and a checkout not found from inside it.

package lepath

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

// TestRootFindsTheCheckoutFromInsideIt walks up from this package's own
// directory, which is what a tool run from anywhere in the tree does.
func TestRootFindsTheCheckoutFromInsideIt(t *testing.T) {
	t.Setenv("ZE_REPO_ROOT", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	root, err := Root()
	if err != nil {
		t.Fatalf("Root from %s: %v", mustGetwd(t), err)
	}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(root, marker)); err != nil {
			t.Errorf("Root answered %s, which has no %s", root, marker)
		}
	}
}

// TestRootPrefersTheEnvironment pins the contract internal/le/lepath/lepath.go states:
// ZE_REPO_ROOT wins, because the environment knows about a container mount, a
// worktree or a fixture that the filesystem walk cannot see.
func TestRootPrefersTheEnvironment(t *testing.T) {
	named := t.TempDir()
	t.Setenv("ZE_REPO_ROOT", named)
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	root, err := Root()
	if err != nil {
		t.Fatalf("Root with ZE_REPO_ROOT set: %v", err)
	}
	want, err := filepath.EvalSymlinks(named)
	if err != nil {
		want = named
	}
	got, err := filepath.EvalSymlinks(root)
	if err != nil {
		got = root
	}
	if got != want {
		t.Errorf("Root answered %s, want the named %s: the environment was ignored", got, want)
	}
}

// TestAncestorWithMarkersRefusesADirectoryThatIsNotACheckout is the guard that
// makes go.mod alone insufficient: a vendored module directory has one, and
// answering it would point every tool at the wrong tree.
func TestAncestorWithMarkersRefusesADirectoryThatIsNotACheckout(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	if found := ancestorWithMarkers(dir); found == dir {
		t.Errorf("a directory with go.mod alone was accepted as a checkout: %s", found)
	}

	if err := os.WriteFile(filepath.Join(dir, "feature-gates.txt"), []byte("\n"), 0o600); err != nil {
		t.Fatalf("write feature-gates.txt: %v", err)
	}
	if found := ancestorWithMarkers(dir); found != dir {
		t.Errorf("a directory with both markers was not accepted: got %q, want %s", found, dir)
	}
}

// VALIDATES: Module answers the path go.mod declares, and answers an ERROR
// rather than an empty string when it declares none.
// PREVENTS: a generator writing blank imports rooted at "/internal/...", which
// compiles nowhere and says nothing about why. An empty string would be a
// plausible-looking answer to a caller that only checks the error it never
// received.
func TestModuleReadsGoModAndRefusesOneWithoutIt(t *testing.T) {
	cases := map[string]struct {
		gomod string
		want  string
	}{
		"a plain declaration":    {"module example.test/m\n\ngo 1.26\n", "example.test/m"},
		"leading blank lines":    {"\n\nmodule example.test/m\n", "example.test/m"},
		"extra spacing":          {"module    example.test/m   \ngo 1.26\n", "example.test/m"},
		"no module directive":    {"go 1.26\n\nrequire ()\n", ""},
		"a comment naming it":    {"// module example.test/m\ngo 1.26\n", ""},
		"an empty file entirely": {"", ""},
	}

	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(one.gomod), 0o600); err != nil {
				t.Fatalf("write go.mod: %v", err)
			}

			got, err := Module(dir)
			if one.want == "" {
				if err == nil {
					t.Fatalf("Module answered %q for a go.mod declaring no module", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Module: %v", err)
			}
			if got != one.want {
				t.Errorf("Module answered %q, want %q", got, one.want)
			}
		})
	}

	if _, err := Module(t.TempDir()); err == nil {
		t.Error("Module answered no error for a directory holding no go.mod")
	}
}

// VALIDATES: ResolveSession publishes the helper's root-relative answer and
// creates scratch with mkdir-compatible permissions only when asked.
// PREVENTS: a native caller changing the path contract or creating state during
// a path-only lookup.
func TestResolveSessionAnswersAndCreatesTheCurrentScratchPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sid-current")
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "")

	oldMask := syscall.Umask(0o027)
	t.Cleanup(func() { syscall.Umask(oldMask) })

	wantDir := filepath.Join(
		"tmp", "session", time.Now().Format("2006-01-02")+"-sid-current")
	want := SessionPaths{
		ID:      "sid-current",
		Dir:     wantDir,
		Scratch: filepath.Join(wantDir, "scratch"),
	}
	got, err := ResolveSession(root, false)
	if err != nil {
		t.Fatalf("resolve the session path: %v", err)
	}
	if got != want {
		t.Fatalf("ResolveSession = %#v, want %#v", got, want)
	}
	if _, err := os.Stat(filepath.Join(root, got.Scratch)); !os.IsNotExist(err) {
		t.Fatalf("a path-only lookup created scratch: %v", err)
	}

	created, err := ResolveSession(root, true)
	if err != nil {
		t.Fatalf("create the session scratch path: %v", err)
	}
	if created != want {
		t.Fatalf("ResolveSession with creation = %#v, want %#v", created, want)
	}
	info, err := os.Stat(filepath.Join(root, created.Scratch))
	if err != nil {
		t.Fatalf("stat the created scratch directory: %v", err)
	}
	if gotMode, wantMode := info.Mode().Perm(), os.FileMode(0o750); gotMode != wantMode {
		t.Errorf("scratch mode = %04o, want %04o from mkdir mode and umask", gotMode, wantMode)
	}
}

// VALIDATES: an existing dated directory is selected before today's spelling,
// and only a directory is eligible.
// PREVENTS: a session moving at midnight or adopting a regular file that only
// resembles its prior state.
func TestResolveSessionSelectsExistingState(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sid-existing")
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "")

	sessionRoot := filepath.Join(root, "tmp", "session")
	regular := filepath.Join(sessionRoot, "2020-01-01-sid-existing")
	if err := os.MkdirAll(sessionRoot, 0o750); err != nil {
		t.Fatalf("create the session root: %v", err)
	}
	if err := os.WriteFile(regular, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write the dated regular file: %v", err)
	}
	legacy := filepath.Join(sessionRoot, "2021-02-03-sid-existing")
	if err := os.Mkdir(legacy, 0o750); err != nil {
		t.Fatalf("create the existing session directory: %v", err)
	}

	got, err := ResolveSession(root, false)
	if err != nil {
		t.Fatalf("resolve existing session state: %v", err)
	}
	want := filepath.Join("tmp", "session", filepath.Base(legacy))
	if got.Dir != want {
		t.Errorf("Dir = %q, want existing state %q", got.Dir, want)
	}
}

// VALIDATES: the shell helper's raw-environment guard remains fail-closed.
// PREVENTS: an unsafe source falling through to a different identity and
// creating a path outside the session root.
func TestResolveSessionRefusesUnsafeEnvironmentIDs(t *testing.T) {
	for _, id := range []string{".", "..", "a/b", "with space", "semi;colon"} {
		t.Run(id, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("CLAUDE_CODE_SESSION_ID", id)
			t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "")

			if _, err := ResolveSession(root, true); err == nil {
				t.Fatalf("ResolveSession accepted unsafe id %q", id)
			}
			if _, err := os.Stat(filepath.Join(root, "tmp")); !os.IsNotExist(err) {
				t.Errorf("unsafe id %q changed the fixture: %v", id, err)
			}
		})
	}
}

// VALIDATES: an unreadable or malformed session root is an error rather than a
// reason to invent a different path.
// PREVENTS: a native consumer falling back to checkout-wide tmp after it could
// not establish which session owned the operation.
func TestResolveSessionFailsClosedOnFilesystemState(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "sid-filesystem")
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "")
	if err := os.Mkdir(filepath.Join(root, "tmp"), 0o750); err != nil {
		t.Fatalf("create tmp: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "tmp", "session"), []byte("not a directory"), 0o600,
	); err != nil {
		t.Fatalf("write malformed session root: %v", err)
	}

	if _, err := ResolveSession(root, false); err == nil {
		t.Fatal("ResolveSession accepted a session root that is not a directory")
	}
}

// VALIDATES: a malformed cache entry is replaced atomically and the resulting
// identity is stable for later callers.
// PREVENTS: a crashed cache write making one session mint a different id on
// every lookup.
func TestResolveSessionHealsMalformedCachedIdentity(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "not-a-jwt")

	process := fixtureSessionProcess{
		pid:     30,
		parents: map[int]int{30: 1},
		start:   map[int]string{30: "4242"},
	}
	key := sessionCacheKey(process, sessionCLIAncestor(process))
	cache := filepath.Join(root, "tmp", "session", ".sid-by-pid-"+key)
	if err := os.MkdirAll(filepath.Dir(cache), 0o750); err != nil {
		t.Fatalf("create cache directory: %v", err)
	}
	if err := os.WriteFile(cache, []byte("../unsafe\n"), 0o600); err != nil {
		t.Fatalf("write malformed cache: %v", err)
	}

	first, err := resolveSession(root, false, process)
	if err != nil {
		t.Fatalf("heal malformed cache: %v", err)
	}
	second, err := resolveSession(root, false, process)
	if err != nil {
		t.Fatalf("read healed cache: %v", err)
	}
	if first.ID == "" || first.ID != second.ID {
		t.Fatalf("cached identity is not stable: first %q, second %q", first.ID, second.ID)
	}
	if !safeSessionID(first.ID) {
		t.Errorf("minted identity %q is unsafe", first.ID)
	}
}

// VALIDATES: the native resolver keeps the canonical environment, process
// argv, and JWT source order without running an external process.
// PREVENTS: a lower-priority credential changing the identity selected by the
// hook payload or the CLI's own --session-id.
func TestResolveSessionIDUsesCanonicalSourceOrder(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"session_id":"from-jwt"}`))
	t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "x."+payload+".x")
	process := fixtureSessionProcess{
		pid:     30,
		parents: map[int]int{30: 20, 20: 1},
		argv:    map[int][]string{20: {"/bin/claude", "--session-id", "from-process"}},
	}

	t.Setenv("CLAUDE_CODE_SESSION_ID", "from-environment")
	id, err := resolveSessionID(t.TempDir(), process)
	if err != nil || id != "from-environment" {
		t.Fatalf("environment source = %q, %v; want from-environment", id, err)
	}

	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	id, err = resolveSessionID(t.TempDir(), process)
	if err != nil || id != "from-process" {
		t.Fatalf("process source = %q, %v; want from-process", id, err)
	}

	process.argv = nil
	id, err = resolveSessionID(t.TempDir(), process)
	if err != nil || id != "from-jwt" {
		t.Fatalf("JWT source = %q, %v; want from-jwt", id, err)
	}
}

type fixtureSessionProcess struct {
	pid       int
	parents   map[int]int
	argv      map[int][]string
	commByPID map[int]string
	start     map[int]string
}

func (process fixtureSessionProcess) PID() int { return process.pid }

func (process fixtureSessionProcess) parentPID(pid int) (int, error) {
	parent, found := process.parents[pid]
	if !found {
		return 0, os.ErrNotExist
	}
	return parent, nil
}

func (process fixtureSessionProcess) Argv(pid int) ([]string, error) {
	return process.argv[pid], nil
}

func (process fixtureSessionProcess) comm(pid int) (string, error) {
	return process.commByPID[pid], nil
}

func (process fixtureSessionProcess) Start(pid int) (string, error) {
	return process.start[pid], nil
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return wd
}
