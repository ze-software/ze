// Design: docs/architecture/storage-backends.md -- restartable explicit import.
// Detail: import_replay.go -- the durable intent, its classification and replay.
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

// ImportBlob validates and copies every raw key, verifies byte equality, then
// publishes and retires the source. Repeating it resumes only its own unchanged
// destination and refuses an existing tree. Caller MUST Close the returned writer.
func ImportBlob(path, dir string) (Storage, error) {
	return importBlob(path, dir, policyRetireSource, false)
}

// ReplaceImportBlob imports like ImportBlob but moves an existing tree, and an
// unrelated seed, to .replaced-<stamp> under the lifetime owner lock before
// publication. Caller MUST Close the returned writer.
func ReplaceImportBlob(path, dir string) (Storage, error) {
	return importBlob(path, dir, policyRetireSource, true)
}

// RestoreBlob replaces the live tree under dir with every key of the artifact
// at path and leaves the artifact where it is. An existing tree moves to
// database.replaced-<stamp> and an unrelated canonical seed to
// database.zefs.replaced-<stamp>; the intent records both names before either
// moves, so running RestoreBlob again finishes an interrupted restore. It
// refuses with ErrBusy while a daemon owns the store. Caller MUST Close the
// returned writer.
func RestoreBlob(path, dir string) (Storage, error) {
	return importBlob(path, dir, policyKeepSource, true)
}

func importBlob(path, dir string, policy importPolicy, replace bool) (Storage, error) {
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
	result, err := importOwned(absolute, folder, owner, policy, replace)
	if err != nil {
		return nil, errors.Join(err, owner.Close(), folder.Close())
	}
	// The returned store holds its own folder handle and the owner lock.
	if err := folder.Close(); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}

// importOwned runs one import under the destination owner lock. An intent
// already on disk is replayed and never rebuilt: it names the stage and every
// retirement name, so a second stage or a second stamp is never chosen.
func importOwned(path string, folder, owner *os.File, policy importPolicy, replace bool) (*store, error) {
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
	intentPath := filepath.Join(folder.Name(), importIntentName)
	treePath := filepath.Join(folder.Name(), treeName)
	intent, exists, err := readImportIntent(folder)
	if err != nil {
		return nil, fmt.Errorf("%s cannot be read: %w; remove that intent file once %s holds the wanted tree", intentPath, err, treePath)
	}
	if !exists {
		return startImport(path, folder, owner, policy, replace)
	}
	if intent.Source != path {
		return nil, fmt.Errorf("%s records an unfinished %s of %s, not %s: finish it with %s, or remove that intent file once %s holds the wanted tree", intentPath, intent.Policy, intent.Source, path, intent.recovery(), treePath)
	}
	if intent.Policy != policy {
		return nil, fmt.Errorf("%s records an unfinished %s of %s, not a %s: finish it with %s, or remove that intent file once %s holds the wanted tree", intentPath, intent.Policy, intent.Source, policy, intent.recovery(), treePath)
	}
	return replayImport(folder, owner, intent)
}

// startImport verifies the source, records every destination the import will
// move, builds and verifies the stage, publishes the intent, then hands over to
// replayImport, which is the only code that moves a recorded node. Until the
// intent write is attempted a failure removes the stage; from then on the
// intent can name it, so the stage stays for the recovery command.
func startImport(path string, folder, owner *os.File, policy importPolicy, replace bool) (_ *store, retErr error) {
	digest, err := blobDigest(path)
	if err != nil {
		return nil, err
	}
	if err := checkArtifact(path); err != nil {
		return nil, err
	}
	source, err := zefs.Open(path)
	if err != nil {
		return nil, err
	}
	defer source.Close() //nolint:errcheck // readonly source.
	stamp := time.Now().Format("20060102T150405.000000000")
	intent := importIntent{Policy: policy, Source: path, Digest: digest}
	if policy == policyRetireSource {
		intent.Archive = path + replacedInfix + stamp
	}
	intent.Tree, err = recordPrevious(folder, treeName, true, stamp)
	if err != nil {
		return nil, err
	}
	if intent.Tree.State == priorPresent && !replace {
		return nil, fmt.Errorf("database already exists: %s: %w", folder.Name(), fs.ErrExist)
	}
	seed := filepath.Join(folder.Name(), blobName)
	switch {
	case seed == path && policy == policyRetireSource:
		// The init source is the canonical seed; retiring it is the source's
		// own archive, never a previous-seed move.
		intent.Seed = priorNode{State: priorSource}
	case seed == path:
		return nil, fmt.Errorf("%s is the canonical seed name, which a live store refuses: import it with ze init --from %s, or move it to another name and restore that", seed, seed)
	default:
		intent.Seed, err = recordPrevious(folder, blobName, false, stamp)
		if err != nil {
			return nil, err
		}
		if intent.Seed.State == priorPresent && !replace {
			return nil, fmt.Errorf("unrelated seed %s already exists; import that seed explicitly", seed)
		}
	}
	if err := removeStaleStages(folder); err != nil {
		return nil, err
	}
	stage, err := makeStage(folder, importStagePrefix)
	if err != nil {
		return nil, err
	}
	intent.Stage = stage
	recorded := false
	defer func() {
		if !recorded {
			retErr = errors.Join(retErr, removeStage(folder, stage))
		}
	}()
	built, err := buildStage(folder, stage, source)
	if err != nil {
		return nil, err
	}
	intent.Device, intent.Inode = built.Device, built.Inode
	encoded, err := json.Marshal(intent)
	if err != nil {
		return nil, err
	}
	recorded = true
	if err := installBytes(folder, folder, importIntentName, encoded, func(f *os.File) error { return f.Sync() }); err != nil {
		return nil, err
	}
	return replayImport(folder, owner, intent)
}

// recordPrevious describes the node at name before an import moves it: absent,
// or present with its identity and the retirement name it will take, which
// must be free now.
func recordPrevious(folder *os.File, name string, directory bool, stamp string) (priorNode, error) {
	id, present, err := identify(folder, name, directory)
	if err != nil {
		return priorNode{}, err
	}
	if !present {
		return priorNode{State: priorAbsent}, nil
	}
	retired := name + replacedInfix + stamp
	if err := nodeStat(folder, retired); err == nil {
		return priorNode{}, fmt.Errorf("retirement name %s already exists: %w", filepath.Join(folder.Name(), retired), fs.ErrExist)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return priorNode{}, err
	}
	return priorNode{State: priorPresent, Retired: retired, Device: id.Device, Inode: id.Inode}, nil
}

// buildStage copies every key of source into the stage, compares the whole
// stage with the source, syncs every frame and directory and the folder that
// holds the stage name, and answers the stage's identity.
func buildStage(folder *os.File, stage string, source *zefs.BlobStore) (nodeIdentity, error) {
	stageFolder, err := duplicateFolder(folder)
	if err != nil {
		return nodeIdentity{}, err
	}
	result, err := openTree(stageFolder, stage, nil, false)
	if err != nil {
		return nodeIdentity{}, errors.Join(err, stageFolder.Close())
	}
	defer result.Close() //nolint:errcheck // the stage is reopened by replayImport.
	for _, key := range source.List("") {
		value, err := source.ReadFile(key)
		if err != nil {
			return nodeIdentity{}, err
		}
		if err := result.WriteKey(key, value); err != nil {
			return nodeIdentity{}, err
		}
	}
	if err := equalImport(result, source); err != nil {
		return nodeIdentity{}, err
	}
	if err := result.tree.barrier(result.tree.root); err != nil {
		return nodeIdentity{}, err
	}
	if err := folder.Sync(); err != nil {
		return nodeIdentity{}, err
	}
	var st unix.Stat_t
	if err := unix.Fstat(int(result.tree.root.Fd()), &st); err != nil {
		return nodeIdentity{}, err
	}
	return nodeIdentity{Device: uint64(st.Dev), Inode: st.Ino}, nil //nolint:unconvert // Dev is int32 on darwin and openbsd
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

// checkArtifact runs zefs.Check over a blob artifact and refuses one whose
// magic, container or any entry fails, naming the failing entries. Every
// reader of an artifact runs it before it reads a key.
func checkArtifact(path string) error {
	report, err := zefs.Check(path)
	if err != nil {
		return err
	}
	if !report.MagicOK {
		return fmt.Errorf("%w: %s: invalid magic", ErrCorrupt, path)
	}
	if !report.ContainerOK {
		return fmt.Errorf("%w: %s: %s; entries: %v", ErrCorrupt, path, report.ContainerError, report.Entries)
	}
	if report.CorruptEntries != 0 {
		return fmt.Errorf("%w: %s: %v", ErrCorrupt, path, report.Entries)
	}
	return nil
}
