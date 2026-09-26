// Design: docs/architecture/storage-backends.md -- backup artifact.
// Related: blob.go -- CreateBlobPopulated publishes the artifact.
// Related: store.go -- guard.ListKeys and guard.ReadKey walk the source.

package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/zefs"
)

// BackupResult describes one published backup artifact.
type BackupResult struct {
	Path  string // absolute path of the artifact
	Keys  int    // number of keys copied
	Bytes int64  // artifact size on disk
}

// Backup copies every key of source into a new blob artifact at path. The walk
// holds one write guard on source from the first key to the last, so a commit
// lands in the backup whole (version and pointer) or not at all. opts set the
// artifact's spare policy; a caller MUST state it, because an artifact is
// normally exact-fit (zefs.Spare(0)) and the zefs default pads by 10%.
// An existing file at path is refused unless replace is set; every name the
// live store owns in its own folder is refused whatever replace says. The artifact is created 0600: it holds credentials.
func Backup(source Storage, path string, replace bool, opts ...zefs.Option) (result BackupResult, err error) {
	owned, ok := ownedStore(source)
	if !ok {
		return BackupResult{}, fmt.Errorf("backup: unsupported storage %T", source)
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return BackupResult{}, fmt.Errorf("backup %s: %w", path, err)
	}
	if err := refuseStoreName(owned, target); err != nil {
		return BackupResult{}, err
	}

	guard, err := owned.AcquireLock("backup")
	if err != nil {
		return BackupResult{}, fmt.Errorf("backup: lock source store: %w", err)
	}
	defer func() { err = releaseGuard(guard, err) }()

	keys, err := guard.ListKeys("")
	if err != nil {
		return BackupResult{}, fmt.Errorf("backup: list source keys: %w", err)
	}
	artifact, err := CreateBlobPopulated(target, func(seed Storage) error {
		return copyKeys(guard, seed, keys)
	}, replace, opts...)
	if err != nil {
		return BackupResult{}, fmt.Errorf("backup %s: %w", target, err)
	}
	if err := artifact.Close(); err != nil {
		return BackupResult{}, fmt.Errorf("backup %s: close: %w", target, err)
	}
	info, err := os.Stat(target)
	if err != nil {
		return BackupResult{}, fmt.Errorf("backup %s: %w", target, err)
	}
	return BackupResult{Path: target, Keys: len(keys), Bytes: info.Size()}, nil
}

// copyKeys writes every key the source guard holds into the seed under one
// seed guard, so the artifact is encoded once rather than once per key. The
// guarded source bytes are written before the source guard is released.
func copyKeys(source WriteGuard, seed Storage, keys []string) (err error) {
	owned, ok := seed.(*store)
	if !ok {
		return fmt.Errorf("backup: unsupported artifact storage %T", seed)
	}
	target, err := owned.acquire()
	if err != nil {
		return err
	}
	defer func() { err = releaseGuard(target, err) }()
	stamp := time.Now()
	for _, key := range objectsFirst(keys) {
		data, err := source.ReadKey(key)
		if err != nil {
			return fmt.Errorf("read %s: %w", key, err)
		}
		if err := target.write(key, data, stamp); err != nil {
			return fmt.Errorf("write %s: %w", key, err)
		}
	}
	return nil
}

// refuseStoreName refuses a backup path that names something the live store
// owns in its folder. Each one breaks the store in its own way: a file under
// database/ is read as one frame and a blob is two, database.zefs makes every
// live open refuse, a lock sidecar is flocked by inode and a published file
// installs a new one, and the intent, replaced and stage names belong to the
// import protocol. An artifact source has no store folder, so nothing is
// refused for it here.
func refuseStoreName(source *store, target string) error {
	if source.tree == nil {
		return nil
	}
	storeDir := source.tree.folder.Name()
	parent := filepath.Dir(target)
	name := filepath.Base(target)
	if sameDirectory(parent, storeDir) {
		if reason := storeOwnedName(name); reason != "" {
			return fmt.Errorf("backup %s: %s is %s: %w", target, name, reason, errors.ErrUnsupported)
		}
		return nil
	}
	tree := filepath.Join(storeDir, treeName)
	for dir := parent; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if sameDirectory(dir, tree) {
			return fmt.Errorf("backup %s: inside the live tree %s: %w", target, tree, errors.ErrUnsupported)
		}
	}
	return nil
}

// storeOwnedName returns why name belongs to the live store, or "" when it is
// free for an artifact.
func storeOwnedName(name string) string {
	switch {
	case name == treeName:
		return "the live tree"
	case name == blobName:
		return "the canonical blob name every live open refuses"
	case strings.HasSuffix(name, ".lock"):
		return "a lock sidecar"
	case name == importIntentName:
		return "the import intent"
	case strings.HasPrefix(name, treeName+replacedInfix):
		return "a replaced tree"
	case strings.HasPrefix(name, blobName+replacedInfix):
		return "a replaced canonical blob"
	case strings.HasPrefix(name, initStagePrefix):
		return "an init stage"
	case strings.HasPrefix(name, importStagePrefix):
		return "an import stage"
	}
	return ""
}

// sameDirectory reports whether a and b are the same directory by identity, so
// a symlinked spelling of the store folder is still recognized. A path that
// cannot be stat'ed is not the store folder: it does not exist yet.
func sameDirectory(a, b string) bool {
	infoA, err := os.Stat(a)
	if err != nil {
		return false
	}
	infoB, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(infoA, infoB)
}

// CheckArtifactPath refuses an artifact path a daemon RPC MUST NOT act on. The
// path names a file on the daemon's host that an operator typed over SSH, so
// it is refused when relative (the daemon's working directory is not the
// operator's), when it carries a ".." element (the spelling hides where it
// lands), and when it is a symlink (the link, not the operator, would choose
// the file). Each refusal names the rule that refused it.
func CheckArtifactPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%s: the path must be absolute: %w", path, errors.ErrUnsupported)
	}
	for element := range strings.SplitSeq(path, string(filepath.Separator)) {
		if element == ".." {
			return fmt.Errorf("%s: the path must not contain \"..\": %w", path, errors.ErrUnsupported)
		}
	}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%s: the path is a symlink: %w", path, errors.ErrUnsupported)
	}
	return nil
}

// ownedStore returns the store behind s, looking through the config-source
// binding a daemon wraps its live handle in, so a live backup walks the same
// handle and the same lock the daemon commits through.
func ownedStore(s Storage) (*store, bool) {
	if bound, ok := s.(*configStore); ok {
		s = bound.Storage
	}
	owned, ok := s.(*store)
	return owned, ok
}
