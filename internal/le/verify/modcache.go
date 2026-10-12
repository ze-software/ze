// Design: docs/architecture/testing/verify-freshness-scope.md -- detached-worktree verification lifecycle
// Related: lifecycle.go -- run, sharedCacheLink

package verify

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// modcacheDir is the gokrazy module cache `ze appliance build` reads with
// GOPROXY=off (internal/appliance/cmd_build.go, SetGokGoEnv).
const modcacheDir = "gokrazy/modcache"

// sharedModcacheLink fills the worktree's gokrazy module cache from the source
// checkout's and answers the line that reports what it did.
//
// Git tracks only the gokrazy init source under gokrazy/modcache, and a
// detached worktree carries nothing ignored, so without this every appliance
// build in a verify worktree resolved its modules offline from an empty cache
// and failed with "looks unpopulated", whatever the source checkout held. The
// link is a prerequisite of the appliance suite rather than a correctness input
// of the tree, so a cache that cannot be linked is REPORTED and the run
// continues: the appliance stage then fails with its own cause.
func sharedModcacheLink(root, path string) string {
	var text textbuf.Buffer
	text.Str("verify-worktree: gokrazy module cache: ")
	source := filepath.Join(root, modcacheDir)
	linked, err := linkModcache(source, filepath.Join(path, modcacheDir))
	if err != nil {
		return text.Err(err).Str(" (the appliance suite builds over an unpopulated cache; run `./le setup install` in ").
			Str(root).Str(")").String()
	}
	return text.Str("linked ").Int(int64(linked)).Str(" entries from ").Str(source).String()
}

// linkModcache mirrors source into target and answers how many links it made.
// An entry target lacks becomes a symlink to source's entry; a directory both
// hold is descended into, so the files git tracks in target, which are the
// commit under test, are never replaced. Every link lands on a path
// gokrazy/modcache/.gitignore ignores, so the worktree's status is unchanged.
//
// The recursion descends only into directories target already holds, which are
// the tracked gokrazy source tree, so its depth is bounded by that tree.
func linkModcache(source, target string) (int, error) {
	entries, err := os.ReadDir(source)
	if err != nil {
		return 0, err
	}
	linked := 0
	for _, entry := range entries {
		sourcePath := filepath.Join(source, entry.Name())
		targetPath := filepath.Join(target, entry.Name())
		info, statErr := os.Lstat(targetPath)
		if errors.Is(statErr, fs.ErrNotExist) {
			if linkErr := os.Symlink(sourcePath, targetPath); linkErr != nil {
				return linked, linkErr
			}
			linked++
			continue
		}
		if statErr != nil {
			return linked, statErr
		}
		if !info.IsDir() {
			continue
		}
		if !entry.IsDir() {
			continue
		}
		descended, descendErr := linkModcache(sourcePath, targetPath)
		linked += descended
		if descendErr != nil {
			return linked, descendErr
		}
	}
	return linked, nil
}
