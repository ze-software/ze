// Design: docs/architecture/core-design.md -- a derived artifact declares its inputs and its rebuild
//
// Package derived holds every file the checkout DERIVES from its own tree.
//
// An artifact is a pure function of the tree, so a committed copy of one is a
// second declaration of what the tree already says, and the two disagree the
// moment an input moves. Ze answers that with a registry rather than with a
// byte comparison: a write to an input DELETES the artifact, and a read of it
// REBUILDS it. Nothing compares a re-render against a stored copy, because
// there is no stored copy.
//
// A generator registers itself from its own init(), beside its leroot
// registration. Nothing here learns an artifact's name, and no hook, stage or
// switch elsewhere enumerates one.
package derived

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// SessionStartPolicy says whether the session-start hook renders an artifact
// the tree does not hold.
//
// The hook runs under a FIXED 5 second timeout (`.claude/settings.json`), and a
// hook killed at that timeout loses every artifact after the kill point AND the
// whole session-start message, the BLOCKING LSP notice and the
// verification-debt warning included. So the budget is the property each
// artifact declares against, and there is no Unspecified answer: a generator
// that says nothing would inherit a render nobody measured, which is exactly
// how the 5.32s measurement below happened.
type SessionStartPolicy uint8

const (
	// SessionStartUnspecified is the zero value and is never valid. Register
	// refuses it.
	SessionStartUnspecified SessionStartPolicy = iota

	// SessionStartBuild renders the artifact at session start when the tree
	// does not hold it. It is for an artifact a search reads WITHOUT naming
	// its path: `grep -rn ResolveBGPTree` reaches no hook, so an absent index
	// answers "no match" for a tree the author cannot see. The three
	// documentation indexes are that class, and they render in about a second
	// between them.
	SessionStartBuild

	// SessionStartDefer leaves the artifact absent for preMaterializeDerived
	// to build. It is for an artifact every reader NAMES: a `./le rfc ...`
	// command and a site build each name their path, so the read hook covers
	// them and nothing reads an absent file as an empty answer.
	//
	// Measured on this checkout on 2026-09-12, with a le built from the tree:
	// the hook takes 2.68s and 2.03s with all eight artifacts present, and
	// 5.32s with the five RFC artifacts absent, against a 5 second timeout.
	// The family is absent often rather than rarely, because its predicate
	// covers every `*.go` write in every session.
	SessionStartDefer
)

// Artifact is one derived file: where it lives, what feeds it, and how to
// rebuild it.
//
// Feeds and Rebuild both take the checkout root, because a caller can hold a
// throwaway tree rather than the session's own checkout, and because a
// predicate that reads an input file has to find it.
type Artifact struct {
	// Path is the output, relative to the checkout root, in slash form.
	Path string

	// Feeds reports whether writing path can change this artifact's content.
	// path is relative to root, in slash form.
	Feeds func(root, path string) bool

	// Rebuild writes the artifact from the tree at root.
	Rebuild func(root string) error

	// Complete reports whether root holds this artifact WHOLE. Nil means a stat
	// of Path is the answer, and a reader takes that answer only after the stat
	// has already found the path.
	//
	// Nil is exact for a single FILE, because WriteAtomic publishes one with a
	// rename: the name exists only once the render behind it finished. A
	// DIRECTORY has no such moment. Its files are written one at a time, so the
	// directory exists from the first of them, and a run that stopped half way
	// leaves a directory that is present and short. A reader would then take a
	// partial set for the whole answer, which is the silent wrong value this
	// package exists to remove (ai/rules/principles.md).
	//
	// So a directory artifact MUST answer this itself, and it MUST name the
	// thing that tells it the last run FINISHED. What it cannot answer cheaply
	// is a member deleted by hand after a complete run: an exact member set
	// costs the render it would be deciding whether to run.
	Complete func(root string) bool

	// SessionStart says whether hookSessionStart renders this artifact when the
	// tree does not hold it. Every registration states it; the zero value is
	// refused, because the hook's budget is fixed and a render nobody costed is
	// how it gets spent.
	SessionStart SessionStartPolicy
}

// registry holds every registered artifact, in registration order.
//
// Safe for concurrent use. Register runs from init(), and All is read by hooks
// that can run at any time, so the two are serialized rather than assumed to be
// ordered by the program's startup.
var registry struct {
	mutex     sync.RWMutex
	artifacts []Artifact
}

// Register declares one derived artifact. A generator calls it from its own
// init().
//
// Every refusal here is a programmer error a build cannot reach past: an
// artifact with no predicate is invalidated by nothing, one with no rebuild is
// materialized by nothing, and two artifacts under one path would each answer
// for the other's content. Each of those is silently inert rather than loud, so
// the refusal is a panic at startup rather than a value a caller has to read.
func Register(artifact Artifact) {
	if artifact.Path == "" {
		panic("BUG: derived.Register with no output path")
	}
	// The path is the one thing the hooks REMOVE and REBUILD, and they join it
	// to a checkout root they were handed. An absolute path ignores that root
	// and a `..` segment climbs out of it, so either one turns an invalidation
	// into a deletion outside the tree the session is working in.
	if filepath.IsAbs(artifact.Path) {
		panic("BUG: derived.Register(" + artifact.Path + ") is absolute, not relative to a checkout root")
	}
	if slices.Contains(strings.Split(artifact.Path, "/"), "..") {
		panic("BUG: derived.Register(" + artifact.Path + ") climbs out of the checkout root")
	}
	if artifact.Feeds == nil {
		panic("BUG: derived.Register(" + artifact.Path + ") with no Feeds predicate")
	}
	if artifact.Rebuild == nil {
		panic("BUG: derived.Register(" + artifact.Path + ") with no Rebuild function")
	}
	// No default, deliberately. A silent "build it at session start" is what
	// put a 194-shard render inside a 5 second hook on 2026-09-11, and a
	// registration that says nothing about the budget is the shape that did it.
	if artifact.SessionStart == SessionStartUnspecified {
		panic("BUG: derived.Register(" + artifact.Path + ") declares no SessionStart policy: " +
			"pass SessionStartBuild when a search that names no path has to find it, " +
			"and SessionStartDefer when every reader names its path")
	}

	registry.mutex.Lock()
	defer registry.mutex.Unlock()

	for index := range registry.artifacts {
		if registry.artifacts[index].Path == artifact.Path {
			panic("BUG: derived.Register(" + artifact.Path + ") is registered twice")
		}
	}
	registry.artifacts = append(registry.artifacts, artifact)
}

// All answers every registered artifact, in registration order. The caller
// receives a copy, so a reader cannot change the registry by editing what it
// was handed.
func All() []Artifact {
	registry.mutex.RLock()
	defer registry.mutex.RUnlock()
	return slices.Clone(registry.artifacts)
}

// Whole reports whether the tree at root holds this artifact entire, and
// raises when the path cannot be read at all.
//
// Two questions, in this order, because the second presumes the first. The stat
// answers whether anything is there. Complete then answers whether what is
// there is the whole of it, which is a question a stat cannot settle for a
// DIRECTORY: its files are written one at a time, so a run that stopped half
// way leaves a present, short directory that a reader takes for the whole
// answer (Artifact.Complete).
func (a Artifact) Whole(root string) (bool, error) {
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(a.Path))); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if a.Complete == nil {
		return true, nil
	}
	return a.Complete(root), nil
}

// EnsureAll renders every registered artifact the tree at root does not hold
// whole, and answers the first failure.
//
// This is the IN-PROCESS half of the read side. The materialize hook covers a
// reader that SPELLS an artifact's path in a shell command, and its own comment
// names what it cannot reach: a Go caller that opens the file from inside its
// own process names no command, so an artifact a concurrent write removed a
// second ago is simply absent to it. The site build is that reader. It
// publishes docs/features/rfc-status.md as a page of the documentation site, so
// without this a build run after any write to an RFC summary, an audit verdict
// or a tagged test publishes a site with that page missing.
//
// Every artifact is ensured rather than a named few, because the site reads the
// checkout as a corpus and a list here would be a second declaration of the
// registry (ai/rules/principles.md). An artifact the tree already holds costs
// one stat.
//
// A Rebuild decides for itself what an unfamiliar tree means: rebuildLedger
// answers nothing for a checkout that holds no rfc/short/, rather than refusing
// it. So nothing here judges whether the artifact belongs to this root.
func EnsureAll(root string) error {
	for _, artifact := range All() {
		whole, err := artifact.Whole(root)
		if err != nil {
			return fmt.Errorf("read the derived artifact %s: %w", artifact.Path, err)
		}
		if whole {
			continue
		}
		if err := artifact.Rebuild(root); err != nil {
			return fmt.Errorf("build the derived artifact %s: %w", artifact.Path, err)
		}
	}
	return nil
}

// WriteAtomic publishes a derived artifact with one rename, so a reader sees
// the whole previous content or the whole new content and never a prefix.
//
// It lives here because every registered artifact needs it for one reason: the
// hooks put a rebuild on the path of an ordinary `grep`, so a write and a read
// of the same artifact now race by design rather than by accident. A bare
// os.WriteFile truncates first, and a grep that lands in that window reads a
// short file as a complete answer.
//
// The temporary file is created in the artifact's own directory, because a
// rename is atomic only within one filesystem.
func WriteAtomic(path string, content []byte) (err error) {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create the temporary file for %s: %w", path, err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(temporaryPath)
		}
	}()

	if _, err = temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write %s: %w", temporaryPath, err)
	}
	// The content is on disk before the rename publishes the name, so a crash
	// between the two leaves the previous artifact rather than an empty one.
	if err = temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync %s: %w", temporaryPath, err)
	}
	if err = temporary.Close(); err != nil {
		return fmt.Errorf("close %s: %w", temporaryPath, err)
	}
	// 0644, because a derived artifact stands where a TRACKED file stood and a
	// checkout gives that file 0644. os.CreateTemp creates at 0600, and every
	// artifact registered here is a page a person or a tool reads: the RFC
	// generator wrote 0644 and said so ("a generated page, world-readable by
	// design") until it moved onto this function on 2026-09-11, at which point
	// docs/features/rfc-status.md and 194 shards became unreadable to any
	// reader that is not this user, with nothing saying so.
	if err = os.Chmod(temporaryPath, 0o644); err != nil { //nolint:gosec // a generated page, world-readable by design
		return fmt.Errorf("set the mode of %s: %w", temporaryPath, err)
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish %s: %w", path, err)
	}
	return nil
}
