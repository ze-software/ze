// Design: docs/architecture/storage-backends.md -- restartable explicit import.
package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/pkg/zefs"
)

// The existing ZeFS artifact decoder bounds imports to 256 MiB.
const blobImportMax = 256 * 1024 * 1024

type importIntent struct {
	Source  string `json:"source"`
	Digest  string `json:"digest"`
	Stage   string `json:"stage"`
	Archive string `json:"archive"`
	Device  uint64 `json:"device"`
	Inode   uint64 `json:"inode"`
}

// ImportBlob validates and copies every raw key, verifies byte equality, then
// publishes and retires the source. Repeating it resumes only its own unchanged
// destination. Caller MUST Close the returned writer.
func ImportBlob(path, dir string) (Storage, error) {
	sourceDir, name, err := splitStorePath(path)
	if err != nil {
		return nil, err
	}
	sourceFolder, err := openFolder(sourceDir)
	if err != nil {
		return nil, err
	}
	absolute := filepath.Join(sourceFolder.Name(), name)
	if err := sourceFolder.Close(); err != nil {
		return nil, err
	}
	folder, err := ensureFolder(dir)
	if err != nil {
		return nil, err
	}
	owner, err := lockOwner(folder, "database.lock")
	if err != nil {
		folder.Close()
		return nil, err
	} //nolint:errcheck // primary lock error.
	result, err := importOwned(absolute, folder, owner)
	if err != nil {
		owner.Close()
		folder.Close()
	} //nolint:errcheck // primary import error.
	return result, err
}
func importOwned(path string, folder, owner *os.File) (_ *store, retErr error) {
	seed := filepath.Join(folder.Name(), "database.zefs")
	if seed != path {
		if err := nodeStat(folder, "database.zefs"); err == nil {
			return nil, fmt.Errorf("unrelated seed %s already exists; import that seed explicitly", seed)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	sourceFolder, err := openFolder(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer sourceFolder.Close() //nolint:errcheck // directory sync is explicit.
	sourceOwner, err := lockOwner(sourceFolder, filepath.Base(path)+".lock")
	if err != nil {
		return nil, err
	}
	defer sourceOwner.Close() //nolint:errcheck // releases the artifact ownership lock.
	intent, exists, err := readImportIntent(folder)
	if err != nil {
		return nil, err
	}
	if exists {
		if intent.Source != path {
			return nil, fmt.Errorf("unfinished import belongs to %s, not %s", intent.Source, path)
		}
	}
	sourcePath := path
	if exists {
		if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
			sourcePath = intent.Archive
		}
	}
	digest, err := blobDigest(sourcePath)
	if err != nil {
		return nil, err
	}
	if exists {
		if digest != intent.Digest {
			return nil, fmt.Errorf("import source changed: %s", sourcePath)
		}
	}
	report, err := zefs.Check(sourcePath)
	if err != nil {
		return nil, err
	}
	if !report.MagicOK {
		return nil, fmt.Errorf("%w: %s: invalid magic", ErrCorrupt, sourcePath)
	}
	if !report.ContainerOK {
		return nil, fmt.Errorf("%w: %s: %s; entries: %v", ErrCorrupt, sourcePath, report.ContainerError, report.Entries)
	}
	if report.CorruptEntries != 0 {
		return nil, fmt.Errorf("%w: %s: %v", ErrCorrupt, sourcePath, report.Entries)
	}
	source, err := zefs.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer source.Close() //nolint:errcheck // readonly source.
	if err := nodeStat(folder, "database"); err == nil {
		if !exists {
			return nil, fmt.Errorf("database already exists: %s: %w", folder.Name(), fs.ErrExist)
		}
		result, err := openTree(folder, "database", owner, false)
		if err != nil {
			return nil, err
		}
		if err := verifyImportIdentity(result, intent, source); err != nil {
			result.tree.root.Close()
			return nil, err
		} //nolint:errcheck // primary identity error; caller owns folder and lock.
		if err := finishImport(folder, intent, sourcePath); err != nil {
			result.tree.root.Close()
			return nil, err
		} //nolint:errcheck // caller owns folder and lock.
		return result, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if exists {
		// The intent names only this import's private stage. No unrelated path is removed.
		if filepath.Base(intent.Stage) != intent.Stage {
			return nil, fmt.Errorf("invalid import stage identity")
		}
		node, nodeErr := openNode(folder, intent.Stage, true)
		if nodeErr == nil {
			var st unix.Stat_t
			if err := unix.Fstat(int(node.Fd()), &st); err != nil {
				return nil, errors.Join(err, node.Close())
			}
			if err := node.Close(); err != nil {
				return nil, err
			}
			if uint64(st.Dev) != intent.Device {
				return nil, fmt.Errorf("import stage device changed")
			}
			if st.Ino != intent.Inode {
				return nil, fmt.Errorf("import stage identity changed")
			}
		} else if !errors.Is(nodeErr, fs.ErrNotExist) {
			return nil, nodeErr
		}
		if err := removeStage(folder, intent.Stage); err != nil {
			return nil, err
		}
	}
	stage, err := makeStage(folder, "database.import-tmp-")
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			retErr = errors.Join(retErr, removeStage(folder, stage))
		}
	}()
	stageFolder, err := duplicateFolder(folder)
	if err != nil {
		return nil, err
	}
	result, err := openTree(stageFolder, stage, nil, false)
	if err != nil {
		stageFolder.Close()
		return nil, err
	} //nolint:errcheck // staging open failure.
	defer func() {
		if !published {
			retErr = errors.Join(retErr, result.Close())
		}
	}()
	for _, key := range source.List("") {
		value, err := source.ReadFile(key)
		if err != nil {
			return nil, err
		}
		if err := result.WriteKey(key, value); err != nil {
			return nil, err
		}
	}
	if err := equalImport(result, source); err != nil {
		return nil, err
	}
	var st unix.Stat_t
	if err := unix.Fstat(int(result.tree.root.Fd()), &st); err != nil {
		return nil, err
	}
	intent = importIntent{Source: path, Digest: digest, Stage: stage, Archive: path + ".replaced-" + time.Now().Format("20060102T150405.000000000"), Device: uint64(st.Dev), Inode: st.Ino}
	encoded, err := json.Marshal(intent)
	if err != nil {
		return nil, err
	}
	if err := installBytes(folder, folder, "database.import-intent", encoded, func(f *os.File) error { return f.Sync() }); err != nil {
		return nil, err
	}
	if err := result.tree.root.Sync(); err != nil {
		return nil, err
	}
	if err := renameNoReplace(folder, stage, "database"); err != nil {
		return nil, err
	}
	published = true
	if err := folder.Sync(); err != nil {
		result.Close()
		return nil, err
	} //nolint:errcheck // published tree retained for recovery.
	if err := result.tree.published("database"); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if err := finishImport(folder, intent, sourcePath); err != nil {
		result.Close()
		return nil, err
	} //nolint:errcheck // import intent retained for recovery.
	result.owner = owner
	if err := folder.Close(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}
func readImportIntent(folder *os.File) (importIntent, bool, error) {
	file, err := openNode(folder, "database.import-intent", false)
	if errors.Is(err, fs.ErrNotExist) {
		return importIntent{}, false, nil
	}
	if err != nil {
		return importIntent{}, false, err
	}
	defer file.Close() //nolint:errcheck // read-only handle.
	data, err := io.ReadAll(io.LimitReader(file, 16385))
	if err != nil {
		return importIntent{}, false, err
	}
	if len(data) > 16384 {
		return importIntent{}, false, fmt.Errorf("oversized import intent")
	}
	var intent importIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		return intent, false, err
	}
	if !filepath.IsAbs(intent.Source) {
		return intent, false, fmt.Errorf("invalid import source identity")
	}
	if filepath.Base(intent.Stage) != intent.Stage {
		return intent, false, fmt.Errorf("invalid import stage identity")
	}
	if !strings.HasPrefix(intent.Stage, "database.import-tmp-") {
		return intent, false, fmt.Errorf("invalid import stage identity")
	}
	if !strings.HasPrefix(intent.Archive, intent.Source+".replaced-") {
		return intent, false, fmt.Errorf("invalid import archive identity")
	}
	if filepath.Dir(intent.Archive) != filepath.Dir(intent.Source) {
		return intent, false, fmt.Errorf("invalid import archive directory")
	}
	return intent, true, nil
}
func blobDigest(path string) (string, error) {
	folder, err := openFolder(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	defer folder.Close() //nolint:errcheck // read handle.
	file, err := openNode(folder, filepath.Base(path), false)
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck // read handle.
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(file, blobImportMax+1))
	if err != nil {
		return "", err
	}
	if n > blobImportMax {
		return "", fmt.Errorf("blob exceeds import limit: %s", path)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
func equalImport(target *store, source *zefs.BlobStore) error {
	keys, err := target.ListKeys("")
	if err != nil {
		return err
	}
	original := source.List("")
	slices.Sort(original)
	if !slices.Equal(keys, original) {
		return fmt.Errorf("import key set differs")
	}
	for _, key := range original {
		want, err := source.ReadFile(key)
		if err != nil {
			return err
		}
		got, err := target.ReadKey(key)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("import value differs: %s", key)
		}
	}
	return nil
}
func verifyImportIdentity(target *store, intent importIntent, source *zefs.BlobStore) error {
	var st unix.Stat_t
	if err := unix.Fstat(int(target.tree.root.Fd()), &st); err != nil {
		return err
	}
	if uint64(st.Dev) != intent.Device {
		return fmt.Errorf("import destination device changed")
	}
	if st.Ino != intent.Inode {
		return fmt.Errorf("import destination identity changed")
	}
	return equalImport(target, source)
}
func finishImport(folder *os.File, intent importIntent, sourcePath string) error {
	digest, err := blobDigest(sourcePath)
	if err != nil {
		return err
	}
	if digest != intent.Digest {
		return fmt.Errorf("import source changed before retirement: %s", sourcePath)
	}
	if sourcePath == intent.Source {
		parent, err := openFolder(filepath.Dir(intent.Source))
		if err != nil {
			return err
		}
		err = renameNoReplace(parent, filepath.Base(intent.Source), filepath.Base(intent.Archive))
		if err == nil {
			err = parent.Sync()
		}
		err = errors.Join(err, parent.Close())
		if err != nil {
			return err
		}
	}
	if err := unix.Unlinkat(int(folder.Fd()), "database.import-intent", 0); err != nil {
		return err
	}
	return folder.Sync()
}
