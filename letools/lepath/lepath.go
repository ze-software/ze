// Design: docs/architecture/core-design.md -- le's checkout discovery
//
// Package lepath answers one question for every le tool: which checkout am I
// working in. The answer is a contract the Python le already publishes
// (scripts/le/paths.py), and this package states the same contract in Go so the
// two halves of the migration cannot disagree about where the tree is.
//
// ZE_REPO_ROOT wins when it is set, because the environment knows things the
// filesystem cannot: a container that mounted the tree elsewhere, a worktree, a
// test fixture standing in for a checkout. Otherwise the root is discovered by
// walking up for the two markers below.
package lepath

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/env"
)

// RootKey is the dot-notation spelling of ZE_REPO_ROOT. env.Get matches
// case-insensitively and treats a dot and an underscore as the same character,
// so this key reads the variable the Python le exports.
const RootKey = "ze.repo.root"

var rootEntry = env.MustRegister(env.EnvEntry{
	Key:         RootKey,
	Type:        "string",
	Default:     "",
	Description: "the Ze checkout le works in; discovered from the markers when unset",
	// Private keeps the key out of `ze env list`. It is le's variable, and a
	// tool imported into ze must not advertise a build-host path to an
	// operator.
	Private: true,
})

// markers identify a Ze checkout. go.mod alone is not enough, because a
// vendored module directory has one. feature-gates.txt is Ze's own rather than
// any Go project's, so the pair is unambiguous.
var markers = [...]string{"go.mod", "feature-gates.txt"}

// ErrNoCheckout says no ancestor of either search start carries both markers.
var ErrNoCheckout = errors.New("lepath: no Ze checkout found (looked for go.mod beside feature-gates.txt)")

// Root answers the checkout this process works in.
//
// The search order is ZE_REPO_ROOT, then the working directory and its
// ancestors, then the directory holding the running executable and its
// ancestors. The Python le starts its walk at its own source file; a compiled
// binary has no source path at run time, so the executable's directory stands
// in for it and covers a binary run from outside the tree.
func Root() (string, error) {
	if named := env.Get(rootEntry.Key); named != "" {
		return filepath.Abs(named)
	}

	if cwd, err := os.Getwd(); err == nil {
		if found := ancestorWithMarkers(cwd); found != "" {
			return found, nil
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return "", ErrNoCheckout
	}
	if found := ancestorWithMarkers(filepath.Dir(exe)); found != "" {
		return found, nil
	}
	return "", ErrNoCheckout
}

// ancestorWithMarkers answers the nearest ancestor of start that carries every
// marker, or "" when none does. The walk is bounded by the filesystem: each
// step shortens the path by one element, and the loop ends when filepath.Dir
// stops changing it, which happens at the root.
func ancestorWithMarkers(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if hasMarkers(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func hasMarkers(dir string) bool {
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err != nil {
			return false
		}
	}
	return true
}
