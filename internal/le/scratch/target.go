// Design: docs/contributing/running-commands.md -- the per-user cache every tool shares
// Related: scratch.go -- cacheTarget, the one place the target is derived

package scratch

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
)

// CacheTarget answers the durable per-user cache directory for environ:
// $XDG_CACHE_HOME/ze, else $HOME/.cache/ze. Environ uses os.Environ syntax with
// last-value-wins lookup. `le exec` reads it to point gopls, golangci-lint and
// staticcheck at the same tree the checkout's cache/ link reaches, so the
// target is derived once, by cacheTarget, and never spelled a second time.
func CacheTarget(environ []string) (string, error) {
	return New("", environ).cacheTarget()
}

// SameDirectory answers whether two paths name one directory: spelled alike,
// or resolving to one. The checkout's cache/go-cache therefore counts as the
// per-user target it links to. A path that does not exist, or a link that
// dangles, is linked nowhere, so it is not the other path unless spelled
// alike: an absent cache/ leaves scratch clean cleaning the ambient row and
// `le setup` writing the GOCACHE setting.
//
// The error says a path could not be resolved for another reason (a
// directory this user cannot search), so sameness can be neither proved nor
// refused, and the caller names the failure on its row.
func SameDirectory(first, second string) (bool, error) {
	if filepath.Clean(first) == filepath.Clean(second) {
		return true, nil
	}
	firstReal, firstMissing, err := resolveDirectory(first)
	if err != nil {
		return false, err
	}
	secondReal, secondMissing, err := resolveDirectory(second)
	if err != nil {
		return false, err
	}
	if firstMissing {
		return false, nil
	}
	if secondMissing {
		return false, nil
	}
	return firstReal == secondReal, nil
}

// resolveDirectory answers path with every link followed, or that nothing is
// there (missing), or the error that kept it from being resolved.
func resolveDirectory(path string) (resolved string, missing bool, err error) {
	resolved, err = filepath.EvalSymlinks(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", true, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("cannot resolve %s: %w", path, err)
	}
	return resolved, false, nil
}
