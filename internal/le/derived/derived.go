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
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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
	// No chmod: os.CreateTemp already creates at 0600, which is the mode every
	// generator wrote before this function existed.
	if err = os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish %s: %w", path, err)
	}
	return nil
}
