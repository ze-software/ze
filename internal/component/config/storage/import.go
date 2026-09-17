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

// importStagePrefix names the private stage an import builds beside the tree,
// and importIntentName the durable record of an import that has not finished.
const (
	importStagePrefix = "database.import-tmp-"
	importIntentName  = "database.import-intent"
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
// destination and refuses an existing tree. Caller MUST Close the returned writer.
func ImportBlob(path, dir string) (Storage, error) {
	return importBlob(path, dir, false)
}

// ReplaceImportBlob imports like ImportBlob but moves an existing tree, and an
// unrelated seed, to .replaced-<stamp> under the lifetime owner lock before
// publication. Caller MUST Close the returned writer.
func ReplaceImportBlob(path, dir string) (Storage, error) {
	return importBlob(path, dir, true)
}

func importBlob(path, dir string, replace bool) (Storage, error) {
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
	owner, err := lockOwner(folder, lockName)
	if err != nil {
		return nil, errors.Join(err, folder.Close())
	}
	result, err := importOwned(absolute, folder, owner, replace)
	if err != nil {
		return nil, errors.Join(err, owner.Close(), folder.Close())
	}
	return result, nil
}

func importOwned(path string, folder, owner *os.File, replace bool) (_ *store, retErr error) {
	seed := filepath.Join(folder.Name(), blobName)
	unrelatedSeed := false
	if seed != path {
		if err := nodeStat(folder, blobName); err == nil {
			if !replace {
				return nil, fmt.Errorf("unrelated seed %s already exists; import that seed explicitly", seed)
			}
			unrelatedSeed = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	intentPath := filepath.Join(folder.Name(), importIntentName)
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
			return nil, fmt.Errorf("%s records an unfinished import of %s, not %s: finish it with ze init --from %s, or remove that intent file once %s holds the wanted tree", intentPath, intent.Source, path, intent.Source, filepath.Join(folder.Name(), treeName))
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
		if exists {
			return nil, fmt.Errorf("%s records an import of %s whose source cannot be read: %w; remove that intent file once %s holds the wanted tree", intentPath, path, err, filepath.Join(folder.Name(), treeName))
		}
		return nil, err
	}
	if exists {
		if digest != intent.Digest {
			return nil, fmt.Errorf("%s records an import of %s whose bytes changed: re-run ze init --from with the original blob, or remove that intent file once %s holds the wanted tree", intentPath, sourcePath, filepath.Join(folder.Name(), treeName))
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
	existingTree := false
	if err := nodeStat(folder, treeName); err == nil {
		switch {
		case exists:
			return resumeImport(folder, owner, intent, source, sourcePath == intent.Archive)
		case replace:
			existingTree = true
		default:
			return nil, fmt.Errorf("database already exists: %s: %w", folder.Name(), fs.ErrExist)
		}
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
			if uint64(st.Dev) != intent.Device { //nolint:unconvert // Dev is int32 on darwin and openbsd
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
	if err := removeStaleStages(folder); err != nil {
		return nil, err
	}
	stage, err := makeStage(folder, importStagePrefix)
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
		return nil, errors.Join(err, stageFolder.Close())
	}
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
	intent = importIntent{Source: path, Digest: digest, Stage: stage, Archive: path + replacedInfix + time.Now().Format("20060102T150405.000000000"), Device: uint64(st.Dev), Inode: st.Ino} //nolint:unconvert // Dev is int32 on darwin and openbsd
	encoded, err := json.Marshal(intent)
	if err != nil {
		return nil, err
	}
	if err := installBytes(folder, folder, importIntentName, encoded, func(f *os.File) error { return f.Sync() }); err != nil {
		return nil, err
	}
	if err := result.tree.root.Sync(); err != nil {
		return nil, err
	}
	// The old tree and an unrelated seed move aside only now, once the stage is
	// complete and verified and the intent is durable: a copy failure or a crash
	// before this point leaves database/ as it was. A crash after that point
	// falls in one of two windows, and the intent names the stage in both.
	// Before moveAside(treeName): the old tree is still under database/ with
	// the intent beside it, ze start opens the old tree, and the next import
	// is refused from verifyImportIdentity as "names another tree"; the
	// operator removes the intent file and the stage it names, as that refusal
	// says, and re-runs the import. After moveAside(treeName) and before the
	// rename: database/ is absent and the old tree is database.replaced-*, so
	// detect reports ErrImportPending and ze start refuses rather than
	// auto-creating an empty tree over it; the next ze init --from of the same
	// blob finds the intent, checks the stage identity above, rebuilds the
	// stage from the source, and publishes.
	if existingTree {
		if err := moveAside(folder, treeName); err != nil {
			return nil, err
		}
	}
	if unrelatedSeed {
		if err := moveAside(folder, blobName); err != nil {
			return nil, err
		}
	}
	if err := zefs.RenameNoReplace(folder, stage, treeName); err != nil {
		return nil, err
	}
	published = true
	if err := folder.Sync(); err != nil {
		// The published tree is retained for recovery.
		return nil, errors.Join(err, result.Close())
	}
	if err := result.tree.published(treeName); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if err := finishImport(folder, intent, false); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	result.owner = owner
	if err := folder.Close(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}

func readImportIntent(folder *os.File) (importIntent, bool, error) {
	file, err := openNode(folder, importIntentName, false)
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
	if !strings.HasPrefix(intent.Stage, importStagePrefix) {
		return intent, false, fmt.Errorf("invalid import stage identity")
	}
	if !strings.HasPrefix(intent.Archive, intent.Source+replacedInfix) {
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

// resumeImport completes an import whose tree is already published. The tree
// is proved to be this import's stage by device and inode. Once the source is
// retired the import is finished and only the intent remains, so the tree,
// which a daemon may have changed since, is not compared to the archive.
func resumeImport(folder, owner *os.File, intent importIntent, source *zefs.BlobStore, retired bool) (*store, error) {
	result, err := openTree(folder, treeName, owner, false)
	if err != nil {
		return nil, err
	}
	// The caller owns folder and lock; only the tree root is closed here.
	if err := verifyImportIdentity(result, intent); err != nil {
		return nil, errors.Join(err, result.tree.root.Close())
	}
	if !retired {
		if err := equalImport(result, source); err != nil {
			err = fmt.Errorf("%s: %w; the published tree differs from its source %s: re-run ze init --from with the original blob to import it again, or remove that intent file once %s holds the wanted tree", filepath.Join(folder.Name(), importIntentName), err, intent.Source, filepath.Join(folder.Name(), treeName))
			return nil, errors.Join(err, result.tree.root.Close())
		}
	}
	if err := finishImport(folder, intent, retired); err != nil {
		return nil, errors.Join(err, result.tree.root.Close())
	}
	return result, nil
}

func verifyImportIdentity(target *store, intent importIntent) error {
	var st unix.Stat_t
	if err := unix.Fstat(int(target.tree.root.Fd()), &st); err != nil {
		return err
	}
	intentPath := filepath.Join(target.tree.folder.Name(), importIntentName)
	stagePath := filepath.Join(target.tree.folder.Name(), intent.Stage)
	if uint64(st.Dev) != intent.Device { //nolint:unconvert // Dev is int32 on darwin and openbsd
		return fmt.Errorf("%s names another tree than %s (device changed): when no database.replaced-* sits beside it, that tree is the one in place before the import and the import did not start replacing it; remove the intent file and the stage %s and re-run ze init --from %s, or keep the tree and remove only the intent file and that stage", intentPath, target.tree.root.Name(), stagePath, intent.Source)
	}
	if st.Ino != intent.Inode {
		return fmt.Errorf("%s names another tree than %s (inode changed): when no database.replaced-* sits beside it, that tree is the one in place before the import and the import did not start replacing it; remove the intent file and the stage %s and re-run ze init --from %s, or keep the tree and remove only the intent file and that stage", intentPath, target.tree.root.Name(), stagePath, intent.Source)
	}
	return nil
}

// removeStaleStages removes every database.import-tmp-* stage under folder
// before a new one is built. A stage outlives its import when the operator
// answers a refusal by removing the intent file alone, and nothing else would
// ever remove it. The caller holds the owner lock, so no other import is
// writing a stage at the same time.
func removeStaleStages(folder *os.File) error {
	listing, err := duplicateFolder(folder)
	if err != nil {
		return err
	}
	defer listing.Close() //nolint:errcheck // readonly listing.
	// The listing ends at EOF or on the first operating-system error.
	for {
		entries, err := listing.ReadDir(128)
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("list %s: %w", folder.Name(), err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasPrefix(name, importStagePrefix) {
				continue
			}
			if err := removeStage(folder, name); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) {
			return folder.Sync()
		}
	}
}

// finishImport retires the source, then removes the intent. Retirement comes
// first so a crash between the two leaves a state resumeImport recognizes as
// finished: the source is gone, the archive holds its bytes, and the tree keeps
// the intent's identity.
func finishImport(folder *os.File, intent importIntent, retired bool) error {
	parent, err := openFolder(filepath.Dir(intent.Source))
	if err != nil {
		return err
	}
	err = retireSource(parent, intent, retired)
	err = errors.Join(err, parent.Close())
	if err != nil {
		return err
	}
	if err := unix.Unlinkat(int(folder.Fd()), importIntentName, 0); err != nil {
		return err
	}
	return folder.Sync()
}

// retireSource renames the source to its archive, then moves the source's lock
// file beside the archive. Every blob artifact keeps its lock next to it, so a
// lock left at the retired name would name an artifact that is gone, and the
// held lock refuses a concurrent creator of that name until it moves. A
// resumed import holds a fresh lock at the source name while the archive's
// lock can already exist; the fresh one is removed then.
func retireSource(parent *os.File, intent importIntent, retired bool) error {
	source := filepath.Base(intent.Source)
	archive := filepath.Base(intent.Archive)
	if !retired {
		digest, err := blobDigest(intent.Source)
		if err != nil {
			return err
		}
		if digest != intent.Digest {
			return fmt.Errorf("import source changed before retirement: %s", intent.Source)
		}
		if err := zefs.RenameNoReplace(parent, source, archive); err != nil {
			return err
		}
	}
	err := zefs.RenameNoReplace(parent, source+".lock", archive+".lock")
	if errors.Is(err, fs.ErrExist) {
		err = unix.Unlinkat(int(parent.Fd()), source+".lock", 0)
	}
	if err != nil {
		return err
	}
	return parent.Sync()
}
