// Design: (none — build/test artifact placement; see plan/spec-session-scoped-build-artifacts.md)

// Package sessionpath answers one question: where do THIS AI session's build and
// test artifacts go?
//
// Concurrent AI sessions share one working tree. Anything written to a fixed path
// (bin/ze, $TMPDIR/ze-functional-*) is therefore either clobbered by a sibling
// session mid-run, or left behind with no owner. This package routes those
// artifacts into the per-session root tmp/s/<session-id>/ that
// scripts/dev/session-scratch.sh already creates and SessionEnd already reaps.
//
// Off-session (a human shell, CI) every helper returns exactly the path used
// before this package existed, so nothing changes for anyone but an AI session.
//
// The session id is resolved by ONE authority, .claude/hooks/lib/session_id.py,
// which exports it as ZE_SESSION_ID (via mk/session.mk) and which the CLI also
// exports as CLAUDE_CODE_SESSION_ID. This package only READS those; it never
// derives an id of its own, because three independent derivations drifted for
// weeks before being consolidated (spec-fixit-session-id-collision).
package sessionpath

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/env"
)

var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.session.id",
	Type:        "string",
	Description: "AI session id scoping build/test artifacts to tmp/s/<id> (set by mk/session.mk; empty means shared paths)",
})

// sidSafe mirrors _SID_SAFE_RE in .claude/hooks/lib/session_id.py: an id is used
// only when it is usable as a filename component. Rejecting rather than
// rewriting is deliberate -- a rewrite would let Go and the hooks disagree about
// which directory a session owns.
var sidSafe = regexp.MustCompile(`\A[A-Za-z0-9._-]+\z`)

// ID returns this session's id, or "" when there is none or it is unsafe.
//
// "" is the off-session answer and every other helper treats it as "use the
// shared path", so a missing or malformed id degrades to today's behavior rather
// than to a path that could escape the scratch root.
func ID() string {
	id := env.Get("ze.session.id")
	if id == "" {
		// The CLI exports this into every child process even when make did not
		// run, so a directly-invoked binary still finds its session.
		id = os.Getenv("CLAUDE_CODE_SESSION_ID")
	}
	// "." and ".." satisfy the filename-component pattern but would escape or
	// alias the scratch root, so they are refused here as they are in
	// scripts/dev/session-scratch.sh.
	if id == "." || id == ".." || !sidSafe.MatchString(id) {
		return ""
	}
	return id
}

// Root returns this session's private directory under baseDir, or "" off-session.
func Root(baseDir string) string {
	id := ID()
	if id == "" {
		return ""
	}
	return filepath.Join(baseDir, "tmp", "s", id)
}

// BinDir returns the directory to build test binaries into: the session's own
// bin/ under tmp/s/<id>/, or the shared <baseDir>/bin off-session.
//
// The final element is always "bin" because ze derives its config/DB directory
// from a parent directory named bin or sbin (internal/core/paths/paths.go
// isBinDir). A binary anywhere else yields "cannot determine database location"
// and breaks commands such as `ze config archive`.
func BinDir(baseDir string) string {
	if root := Root(baseDir); root != "" {
		return filepath.Join(root, "bin")
	}
	return filepath.Join(baseDir, "bin")
}

// sharedBinDir is the checkout's shared bin/, ignoring any session.
func sharedBinDir(baseDir string) string {
	return filepath.Join(baseDir, "bin")
}

// FindPrebuiltDir returns the first directory holding EVERY name: this session's
// bin/ (where the runner would have built them), then the shared bin/. Returns
// "" when no single directory has them all.
//
// Session scoping exists to stop one session's BUILD overwriting another's
// binary. Reading a binary someone already built clobbers nothing, so a
// ZE_TEST_NO_BUILD lookup falls back to the shared bin/ -- otherwise a perfectly
// good cross-compiled or make-built binary at <baseDir>/bin/ze would be reported
// missing.
//
// It resolves a DIRECTORY rather than each binary independently, because .ci
// tests exec `ze` and `ze-stripped` by BARE NAME and the runner puts one
// directory on their PATH (runner_exec.go). Resolving ze from one directory and
// ze-test from another would satisfy both stat calls and still leave a test
// exec'ing a sibling binary that is not there.
func FindPrebuiltDir(baseDir string, names ...string) string {
	// No names means nothing was asked for, so no directory can satisfy it.
	// Without this, the "every name present" loop is vacuously true and the
	// first candidate is reported as a hit even when it does not exist.
	if len(names) == 0 {
		return ""
	}
	seen := make(map[string]bool, 2)
	for _, dir := range []string{BinDir(baseDir), sharedBinDir(baseDir)} {
		if seen[dir] {
			continue
		}
		seen[dir] = true
		complete := true
		for _, name := range names {
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				complete = false
				break
			}
		}
		if complete {
			return dir
		}
	}
	return ""
}

// scratchRoot returns the parent directory for this session's temporary working
// directories, or "" off-session.
//
// "" is what os.MkdirTemp treats as "use the system temp dir", so callers can
// pass the result straight through and keep their current behavior when no
// session is active.
func scratchRoot(baseDir string) string {
	return Root(baseDir)
}

// DefaultScratchRoot is EnsureScratchRoot for callers that do not know the
// checkout root.
//
// It prefers CLAUDE_PROJECT_DIR -- the same repo-root variable
// .claude/hooks/lib/session_id.py and scripts/dev/session-scratch.sh use, so all
// three agree on which directory a session owns -- and falls back to walking up
// from the working directory for go.mod. The fallback is load-bearing, not
// belt-and-braces: the CLI does NOT export CLAUDE_PROJECT_DIR into the shell
// that runs make, so the tests this package exists to contain would otherwise
// keep writing to the system temp dir with the routing silently inert.
//
// Returns "" when no session is active or no checkout root can be found (CI, or
// inside the QEMU VM), which callers pass to os.MkdirTemp as "use the system
// temp dir" -- their behavior before this package existed.
func DefaultScratchRoot() string {
	if base := os.Getenv("CLAUDE_PROJECT_DIR"); base != "" {
		return EnsureScratchRoot(base)
	}
	base := repoRoot()
	if base == "" {
		return ""
	}
	return EnsureScratchRoot(base)
}

// moduleDirective is the module line of the ze checkout's own go.mod.
//
// repoRoot matches on it rather than on the mere PRESENCE of a go.mod, because
// tmp/ carries a tracked sentinel module (`module ze-tmp-scratch`, written by
// scripts/dev/ensure-links.py) that stops `go list ./...` descending into the
// caches -- and those caches hold further foreign go.mod files. A
// presence-only walk starting anywhere under tmp/ therefore stops at the
// sentinel and reports tmp/ as the checkout root, making the scratch root
// tmp/tmp/s/<id>: a real directory, returned with no error, and outside
// everything SessionEnd removes.
const moduleDirective = "module github.com/ze-software/ze"

// repoRoot walks up from the working directory to the ze checkout root.
// Mirrors internal/test/cli.FindBaseDir, which cannot be reused here without an
// import cycle (that package imports the runner, which imports this).
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if isRepoRoot(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// isRepoRoot reports whether dir holds the ze module's own go.mod.
func isRepoRoot(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod")) //nolint:gosec // walking to the checkout root
	if err != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == moduleDirective {
			return true
		}
	}
	return false
}

// EnsureScratchRoot is ScratchRoot with the directory created, for callers about
// to hand the result to os.MkdirTemp (which requires an existing parent).
//
// A creation failure is reported as "" rather than an error: falling back to the
// system temp dir keeps the test running, and the only thing lost is the
// automatic session-end cleanup.
func EnsureScratchRoot(baseDir string) string {
	root := scratchRoot(baseDir)
	if root == "" {
		return ""
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return ""
	}
	return root
}
