package gostaticcheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
)

// TestJudgeRunsWithPathIndependentCompiles proves the
// environment Staticcheck runs under carries -trimpath in GOFLAGS, keeps every
// inherited flag, never adds it twice, and pins the gate's own cache, and that
// the GOFLAGS entry is the toolchain's (gotoolchain.TrimpathGoFlags) rather
// than one the gate composes for itself.
//
// Method: build the environment from four inherited GOFLAGS shapes and read
// the entry the child sees, which is the LAST one for a key, then compare it
// with the toolchain's answer over the same environment. Without -trimpath
// every compile action ID carries the tree's absolute directory, so a verify
// run in a fresh tree compiles the whole module cold and the matrix exceeds
// its deadline.
func TestJudgeRunsWithPathIndependentCompiles(t *testing.T) {
	cases := []struct {
		name    string
		environ []string
		want    string
	}{
		{"no GOFLAGS", []string{"HOME=/h", "GOENV=off"}, "-trimpath"},
		{"inherited flags kept", []string{"GOFLAGS=-mod=vendor"}, "-mod=vendor -trimpath"},
		{"already present", []string{"GOFLAGS=-trimpath -mod=vendor"}, "-trimpath -mod=vendor"},
		{"last inherited entry wins", []string{"GOFLAGS=-race", "GOFLAGS=-mod=vendor"}, "-mod=vendor -trimpath"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := judgeEnvironment(tc.environ, "/c", t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if flags := lastValue(got, "GOFLAGS"); flags != tc.want {
				t.Errorf("GOFLAGS = %q, want %q", flags, tc.want)
			}
			toolchain, err := gotoolchain.TrimpathGoFlags(t.Context(), tc.environ, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if entry := "GOFLAGS=" + lastValue(got, "GOFLAGS"); entry != toolchain {
				t.Errorf("Staticcheck GOFLAGS entry %q is not the toolchain's %q", entry, toolchain)
			}
			if cache := lastValue(got, cacheKey); cache != "/c" {
				t.Errorf("STATICCHECK_CACHE = %q, want /c", cache)
			}
			if !slices.Equal(got[:len(tc.environ)], tc.environ) {
				t.Errorf("inherited environment changed: %q", got[:len(tc.environ)])
			}
		})
	}
}

// TestJudgeCompilesBeforeTheDeadlineStarts proves Judge compiles the widest row
// Staticcheck will load before the Staticcheck deadline starts, with the
// arguments Staticcheck's own load passes and the environment it runs under.
//
// Method: a fake `go` on PATH records its arguments and GOFLAGS, then takes
// longer than the deadline Judge is given; a fake `staticcheck` answers clean at
// once. A Judge that compiled inside the deadline, or not at all, reports the
// matrix unjudged or leaves the record without the widest row's line. A change to a
// package most of the module imports leaves every dependent to compile again,
// and piece 1 of the next verify paid that compile inside a bound sized for the
// type check alone (warmBuildCache).
func TestJudgeCompilesBeforeTheDeadlineStarts(t *testing.T) {
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "go-calls.txt")
	// sleep(external): the fake compiler must outlast the Staticcheck deadline
	// to prove the deadline does not include the compile; no condition exists
	// to wait on because the delay IS the property under test.
	// `go env GOFLAGS` is the toolchain asking for the effective value
	// (gotoolchain.EffectiveGoFlags); the fake answers it empty at once and
	// records only the compile.
	fakeGo := "#!/bin/sh\nif [ \"$1\" = env ]; then echo; exit 0; fi\necho \"GOFLAGS=$GOFLAGS $*\" >> " + record + "\nsleep 1.5\n"
	fakeStaticcheck := "#!/bin/sh\ncat > /dev/null\nexit 0\n"
	for name, body := range map[string]string{"go": fakeGo, "staticcheck": fakeStaticcheck} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil { //nolint:gosec // an executable test double
			t.Fatalf("write the fake %s: %v", name, err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOFLAGS", "")

	matrix := Matrix{
		{Name: "core_only", Tags: []string{"ze_a"}},
		{Name: "without_ze_c", Tags: []string{"ze_a", "ze_b"}, Omits: "ze_c"},
		{Name: "without_ze_b", Tags: []string{"ze_a", "ze_c"}, Omits: "ze_b"},
	}
	verdict, judged, err := Judge(t.TempDir(), matrix, Scope("fake"), time.Second)
	if !judged {
		t.Fatalf("matrix unjudged (%v): the compile ran inside the Staticcheck deadline", err)
	}
	if !verdict.Passed {
		t.Fatalf("verdict failed over a clean fake Staticcheck: %+v", verdict)
	}

	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("no compile ran before Staticcheck: %v", err)
	}
	const load = "list -e -compiled=true -test=true -export=true -deps=true -find=false -buildvcs=false -pgo=off "
	want := []string{"GOFLAGS=-trimpath " + load + "-tags=ze_a,ze_b -- ./..."}
	if got := strings.Split(strings.TrimSpace(string(raw)), "\n"); !slices.Equal(got, want) {
		t.Errorf("compiles before Staticcheck:\n got %q\nwant %q", got, want)
	}
}

// TestJudgeMarksTheCacheOnlyAfterAVerdict proves Judge leaves its cache warm
// only when Staticcheck reached a verdict over it: a clean exit and a
// type-check failure both mark it, and a run killed at its deadline leaves it
// cold, even when an earlier run had marked it.
//
// Method: fake `go` and `staticcheck` binaries on PATH; the fake Staticcheck
// exits 0, exits 1 with a diagnostic, or outlives the deadline Judge is given.
// CacheCold is then asked about the tree's cache. A killed run leaves entries
// behind, and verify runs 41289750811 and 41295105997 restored such a partial
// cache in every shard, were bounded as warm, and were killed again.
func TestJudgeMarksTheCacheOnlyAfterAVerdict(t *testing.T) {
	cases := []struct {
		name        string
		staticcheck string
		markedFirst bool
		judged      bool
		cold        bool
	}{
		{"a clean verdict marks the cache", "#!/bin/sh\ncat > /dev/null\nexit 0\n", false, true, false},
		{"a failing verdict marks the cache", "#!/bin/sh\ncat > /dev/null\necho 'a.go:1:1: broken'\nexit 1\n", false, true, false},
		// sleep(external): the fake Staticcheck must outlive the deadline so
		// Judge kills it; the delay IS the property under test.
		{"a killed run clears the mark", "#!/bin/sh\ncat > /dev/null\nexec sleep 5\n", true, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bin := t.TempDir()
			fakeGo := "#!/bin/sh\nif [ \"$1\" = env ]; then echo; exit 0; fi\nexit 0\n"
			for name, body := range map[string]string{"go": fakeGo, "staticcheck": tc.staticcheck} {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil { //nolint:gosec // an executable test double
					t.Fatalf("write the fake %s: %v", name, err)
				}
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("GOFLAGS", "")

			const scope = Scope("the scope of this run")
			tree := t.TempDir()
			if tc.markedFirst {
				if err := os.MkdirAll(CacheDir(tree), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(CacheDir(tree), judgedMarker), []byte(scope), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			matrix := Matrix{{Name: "core_only", Tags: []string{"ze_a"}}}
			_, judged, err := Judge(tree, matrix, scope, 500*time.Millisecond)
			if judged != tc.judged {
				t.Fatalf("judged = %v (err %v), want %v", judged, err, tc.judged)
			}
			cold, err := CacheCold(CacheDir(tree), scope)
			if err != nil {
				t.Fatal(err)
			}
			if cold != tc.cold {
				t.Errorf("after the run the cache answers cold = %v, want %v", cold, tc.cold)
			}
		})
	}
}

// TestACacheIsWarmOnlyForTheScopeItsVerdictCovered proves a verdict mark
// holds the cache warm only for a run over the inputs and rows that verdict
// covered: a new commit, an uncommitted Go edit and another piece of the
// matrix each find the cache cold, and a page under docs/, which no Go package
// load reads, leaves it warm.
//
// Method: a git checkout holding one Go file; the scope of one piece marks the
// cache as a verdict would (markJudged); CacheCold is then asked with the scope
// ScopeOf answers after each change. Verify run 41356999610 at 3819ae5430
// marked every piece's cache, run 41379939815 at 8653bf598c read those marks as
// warm over a tree the push had moved, and every piece exceeded its 9m0s or
// 10m30s bound.
func TestACacheIsWarmOnlyForTheScopeItsVerdictCovered(t *testing.T) {
	tree := t.TempDir()
	scopeGit(t, tree, "init", "--quiet")
	scopeGit(t, tree, "config", "user.email", "test@example.com")
	scopeGit(t, tree, "config", "user.name", "test")
	// The repository ignores /cache as this one does: the cache lives inside
	// the tree (CacheDir), and the mark written into it is no input.
	scopeWrite(t, tree, ".gitignore", "/cache\n")
	scopeWrite(t, tree, "go.mod", "module example.com/scope\n\ngo 1.26\n")
	scopeWrite(t, tree, "a.go", "package scope\n")
	scopeGit(t, tree, "add", ".")
	scopeGit(t, tree, "commit", "--quiet", "-m", "first")

	piece := Matrix{{Name: "core_only", Tags: []string{"ze_a"}}}
	other := Matrix{{Name: "without_ze_b", Tags: []string{"ze_a", "ze_c"}, Omits: "ze_b"}}
	cache := CacheDir(tree)
	if err := os.MkdirAll(cache, 0o750); err != nil {
		t.Fatal(err)
	}
	judged := scopeOf(t, tree, piece)
	if err := markJudged(cache, judged); err != nil {
		t.Fatal(err)
	}

	coldFor := func(scope Scope) bool {
		t.Helper()
		cold, err := CacheCold(cache, scope)
		if err != nil {
			t.Fatal(err)
		}
		return cold
	}
	if coldFor(scopeOf(t, tree, piece)) {
		t.Fatal("the cache a verdict over this very scope marked answers cold")
	}

	scopeWrite(t, tree, "docs/page.md", "a page no Go package load reads\n")
	if coldFor(scopeOf(t, tree, piece)) {
		t.Error("a page under docs/ cooled the cache, though no Go package load reads it")
	}
	if !coldFor(scopeOf(t, tree, other)) {
		t.Error("a verdict over one piece held the cache warm for another piece")
	}
	if !coldFor("") {
		t.Error("the zero Scope, which names no inputs, found the cache warm")
	}

	scopeWrite(t, tree, "a.go", "package scope\n\n// A is new.\nconst A = 1\n")
	if !coldFor(scopeOf(t, tree, piece)) {
		t.Error("an uncommitted Go edit left the cache warm for a verdict over the old source")
	}
	scopeGit(t, tree, "commit", "--quiet", "-am", "second")
	if !coldFor(scopeOf(t, tree, piece)) {
		t.Error("a verdict at the previous commit held the cache warm for the next one")
	}

	if _, err := ScopeOf(t.TempDir(), piece); err == nil {
		t.Error("ScopeOf named the inputs of a directory git cannot describe")
	}
}

// scopeOf answers ScopeOf over a fixture that must have one.
func scopeOf(t *testing.T, tree string, matrix Matrix) Scope {
	t.Helper()
	scope, err := ScopeOf(tree, matrix)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

// scopeGit runs one git command in a fixture checkout.
func scopeGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, out)
	}
}

// scopeWrite writes one fixture file, creating its directory.
func scopeWrite(t *testing.T, dir, name, body string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// lastValue answers the value the child process reads for key: the last entry.
func lastValue(environ []string, key string) string {
	value := ""
	for _, entry := range environ {
		if rest, found := strings.CutPrefix(entry, key+"="); found {
			value = rest
		}
	}
	return value
}
