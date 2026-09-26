// Design: docs/architecture/storage-backends.md -- restartable explicit import.
// Related: import.go -- builds the stage and publishes the intent this file replays.
package storage

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/pkg/zefs"
)

// importPolicy says what an import does with its source once the new tree is
// published. The zero value is not a policy: an intent without one refuses.
type importPolicy uint8

const (
	policyUnspecified  importPolicy = iota
	policyKeepSource                // ze data restore <file> full: the artifact stays where it is
	policyRetireSource              // ze init --from: the artifact moves to <source>.replaced-<stamp>
)

func (p importPolicy) String() string {
	switch p {
	case policyKeepSource:
		return "restore"
	case policyRetireSource:
		return "import"
	case policyUnspecified:
	}
	return "unspecified operation"
}

// MarshalText writes the policy's durable spelling.
func (p importPolicy) MarshalText() ([]byte, error) {
	switch p {
	case policyKeepSource:
		return []byte("keep-source"), nil
	case policyRetireSource:
		return []byte("retire-source"), nil
	case policyUnspecified:
	}
	return nil, fmt.Errorf("BUG: import policy %d has no durable spelling", p)
}

// UnmarshalText refuses every spelling but the two policies.
func (p *importPolicy) UnmarshalText(text []byte) error {
	switch string(text) {
	case "keep-source":
		*p = policyKeepSource
	case "retire-source":
		*p = policyRetireSource
	default:
		return fmt.Errorf("unknown import policy %q", text)
	}
	return nil
}

// priorState is what one destination held before the import recorded it.
type priorState uint8

const (
	priorUnspecified priorState = iota
	priorAbsent                 // no node at the destination name
	priorPresent                // a node the import moves to its recorded retirement name
	priorSource                 // the canonical seed is the init source itself (retire-source only)
)

// MarshalText writes the state's durable spelling.
func (s priorState) MarshalText() ([]byte, error) {
	switch s {
	case priorAbsent:
		return []byte("absent"), nil
	case priorPresent:
		return []byte("present"), nil
	case priorSource:
		return []byte("source"), nil
	case priorUnspecified:
	}
	return nil, fmt.Errorf("BUG: prior state %d has no durable spelling", s)
}

// UnmarshalText refuses every spelling but the three states.
func (s *priorState) UnmarshalText(text []byte) error {
	switch string(text) {
	case "absent":
		*s = priorAbsent
	case "present":
		*s = priorPresent
	case "source":
		*s = priorSource
	default:
		return fmt.Errorf("unknown destination state %q", text)
	}
	return nil
}

// priorNode describes one destination before the import moved anything.
type priorNode struct {
	State   priorState `json:"state"`
	Retired string     `json:"retired,omitempty"` // basename the node moves to; present only
	Device  uint64     `json:"device,omitempty"`
	Inode   uint64     `json:"inode,omitempty"`
}

func (n priorNode) identity() nodeIdentity { return nodeIdentity{Device: n.Device, Inode: n.Inode} }

// importIntent is the one immutable record an import writes before any
// destination moves. Progress is never stored: replay derives it from which
// recorded identity sits at which recorded name.
type importIntent struct {
	Policy  importPolicy `json:"policy"`
	Source  string       `json:"source"`
	Digest  string       `json:"digest"`
	Archive string       `json:"archive,omitempty"` // retire-source only
	Stage   string       `json:"stage"`
	Device  uint64       `json:"device"` // the new tree, from stage to database
	Inode   uint64       `json:"inode"`
	Tree    priorNode    `json:"tree"`
	Seed    priorNode    `json:"seed"`
}

func (i importIntent) newTree() nodeIdentity { return nodeIdentity{Device: i.Device, Inode: i.Inode} }

// recovery is the command that finishes this intent.
func (i importIntent) recovery() string {
	if i.Policy == policyKeepSource {
		return "ze data restore " + i.Source + " full"
	}
	return "ze init --from " + i.Source
}

// nodeIdentity is the device and inode of one directory entry. Byte equality
// never proves which node an intent recorded; only this does.
type nodeIdentity struct {
	Device uint64
	Inode  uint64
}

// identify reports the identity of name under folder without following a
// symlink, refusing a node of the wrong type or an unsafe mode.
func identify(folder *os.File, name string, directory bool) (nodeIdentity, bool, error) {
	node, err := openNode(folder, name, directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nodeIdentity{}, false, nil
	}
	if err != nil {
		return nodeIdentity{}, false, err
	}
	defer node.Close() //nolint:errcheck // read-only identity probe.
	var st unix.Stat_t
	if err := unix.Fstat(int(node.Fd()), &st); err != nil {
		return nodeIdentity{}, false, err
	}
	return nodeIdentity{Device: uint64(st.Dev), Inode: st.Ino}, true, nil //nolint:unconvert // Dev is int32 on darwin and openbsd
}

// readImportIntent reads and validates the intent. A missing or unknown field
// refuses: absence is never implied.
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
		return importIntent{}, false, errors.New("oversized import intent")
	}
	var intent importIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		return intent, false, err
	}
	if err := validIntent(folder, intent); err != nil {
		return intent, false, err
	}
	return intent, true, nil
}

func validIntent(folder *os.File, intent importIntent) error {
	if intent.Policy == policyUnspecified {
		return errors.New("import intent records no source policy")
	}
	if !filepath.IsAbs(intent.Source) {
		return errors.New("invalid import source identity")
	}
	if digest, err := hex.DecodeString(intent.Digest); err != nil || len(digest) != 32 {
		return errors.New("invalid import source digest")
	}
	if filepath.Base(intent.Stage) != intent.Stage {
		return errors.New("invalid import stage identity")
	}
	if !strings.HasPrefix(intent.Stage, importStagePrefix) {
		return errors.New("invalid import stage identity")
	}
	if err := validArchive(intent); err != nil {
		return err
	}
	if err := validPrior(intent.Tree, treeName); err != nil {
		return fmt.Errorf("invalid previous tree: %w", err)
	}
	seed := filepath.Join(folder.Name(), blobName)
	if intent.Seed.State == priorSource {
		if intent.Policy != policyRetireSource {
			return errors.New("invalid canonical seed: only an init retires the seed as its source")
		}
		if intent.Source != seed {
			return errors.New("invalid canonical seed: the source is not the canonical seed")
		}
		return nil
	}
	if intent.Source == seed {
		return errors.New("invalid canonical seed: the source is the canonical seed")
	}
	if err := validPrior(intent.Seed, blobName); err != nil {
		return fmt.Errorf("invalid previous seed: %w", err)
	}
	return nil
}

func validArchive(intent importIntent) error {
	if intent.Policy == policyKeepSource {
		if intent.Archive != "" {
			return errors.New("invalid import archive: a restore keeps its source")
		}
		return nil
	}
	if !strings.HasPrefix(intent.Archive, intent.Source+replacedInfix) {
		return errors.New("invalid import archive identity")
	}
	if filepath.Dir(intent.Archive) != filepath.Dir(intent.Source) {
		return errors.New("invalid import archive directory")
	}
	return nil
}

func validPrior(node priorNode, name string) error {
	switch node.State {
	case priorAbsent:
		if node.Retired != "" {
			return errors.New("an absent destination records a retirement name")
		}
		return nil
	case priorPresent:
		if filepath.Base(node.Retired) != node.Retired {
			return errors.New("the retirement name is not a folder entry")
		}
		if !strings.HasPrefix(node.Retired, name+replacedInfix) {
			return fmt.Errorf("the retirement name %q is not %s%s<stamp>", node.Retired, name, replacedInfix)
		}
		return nil
	case priorSource, priorUnspecified:
	}
	return errors.New("the destination state is missing or not allowed here")
}

// importProgress is where a valid intent's recorded nodes sit now. It is
// derived before any replay mutation and never persisted.
type importProgress struct {
	published   bool // the new tree is at database and the stage name is absent
	retired     bool // the source is at its archive (retire-source only)
	treeRetired bool // the previous tree is at its retirement name, or there was none
	seedRetired bool // the previous seed is at its retirement name, or there was none
}

// classifyImport matches every recorded location against the table of
// permitted states and refuses any other combination before anything moves.
func classifyImport(folder *os.File, intent importIntent) (importProgress, error) {
	var progress importProgress
	retired, err := locateSource(intent)
	if err != nil {
		return progress, err
	}
	progress.retired = retired
	tree, treePresent, err := identify(folder, treeName, true)
	if err != nil {
		return progress, err
	}
	stage, stagePresent, err := identify(folder, intent.Stage, true)
	if err != nil {
		return progress, err
	}
	treePath := filepath.Join(folder.Name(), treeName)
	if stagePresent && stage != intent.newTree() {
		return progress, fmt.Errorf("the stage %s is not the one this intent recorded", filepath.Join(folder.Name(), intent.Stage))
	}
	isNew := treePresent && tree == intent.newTree()
	isOld := treePresent && intent.Tree.State == priorPresent && tree == intent.Tree.identity()
	if treePresent && !isNew && !isOld {
		return progress, fmt.Errorf("%s is neither the tree this intent replaces nor the one it publishes; an unrelated replacement is never moved", treePath)
	}
	progress.treeRetired, err = locatePrevious(folder, intent.Tree, true, isOld)
	if err != nil {
		return progress, fmt.Errorf("previous tree: %w", err)
	}
	progress.seedRetired = true
	if intent.Seed.State != priorSource {
		seed, seedPresent, err := identify(folder, blobName, false)
		if err != nil {
			return progress, err
		}
		seedIsOld := seedPresent && intent.Seed.State == priorPresent && seed == intent.Seed.identity()
		if seedPresent && !seedIsOld {
			return progress, fmt.Errorf("%s is not the seed this intent recorded; an unrelated replacement is never moved", filepath.Join(folder.Name(), blobName))
		}
		progress.seedRetired, err = locatePrevious(folder, intent.Seed, false, seedIsOld)
		if err != nil {
			return progress, fmt.Errorf("previous seed: %w", err)
		}
	}
	if isNew {
		if stagePresent {
			return progress, errors.New("the new tree is published and its stage name is still present")
		}
		if !progress.treeRetired || !progress.seedRetired {
			return progress, errors.New("the new tree is published while a previous destination is still at its original name")
		}
		progress.published = true
		return progress, nil
	}
	if !stagePresent {
		return progress, fmt.Errorf("the stage %s is missing and %s is not the published tree", filepath.Join(folder.Name(), intent.Stage), treePath)
	}
	if progress.retired {
		return progress, errors.New("the source is retired but the new tree is not published")
	}
	if !progress.treeRetired && intent.Seed.State == priorPresent && progress.seedRetired {
		return progress, errors.New("the previous seed moved before the previous tree")
	}
	return progress, nil
}

// locateSource answers whether the source is at its archive. Policy alone
// never sets it: the names on disk do, and both or neither refuse.
func locateSource(intent importIntent) (bool, error) {
	original, err := present(intent.Source)
	if err != nil {
		return false, err
	}
	if intent.Policy == policyKeepSource {
		if !original {
			return false, fmt.Errorf("the source %s is missing; a restore has no archive to fall back to", intent.Source)
		}
		return false, sourceUnchanged(intent.Source, intent)
	}
	archived, err := present(intent.Archive)
	if err != nil {
		return false, err
	}
	switch {
	case original && !archived:
		return false, sourceUnchanged(intent.Source, intent)
	case !original && archived:
		return true, sourceUnchanged(intent.Archive, intent)
	case original:
		return false, fmt.Errorf("both %s and its archive %s exist", intent.Source, intent.Archive)
	}
	return false, fmt.Errorf("neither %s nor its archive %s exists", intent.Source, intent.Archive)
}

func present(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func sourceUnchanged(path string, intent importIntent) error {
	digest, err := blobDigest(path)
	if err != nil {
		return err
	}
	if digest != intent.Digest {
		return fmt.Errorf("the source %s changed since the intent recorded it", path)
	}
	return nil
}

// locatePrevious answers whether a recorded destination is at its retirement
// name. Exactly one of its two names holds the recorded identity.
func locatePrevious(folder *os.File, prior priorNode, directory, atOriginal bool) (bool, error) {
	if prior.State != priorPresent {
		return true, nil
	}
	moved, found, err := identify(folder, prior.Retired, directory)
	if err != nil {
		return false, err
	}
	if found && moved != prior.identity() {
		return false, fmt.Errorf("%s holds another node than the one recorded", filepath.Join(folder.Name(), prior.Retired))
	}
	switch {
	case atOriginal && !found:
		return false, nil
	case !atOriginal && found:
		return true, nil
	case atOriginal:
		return false, fmt.Errorf("the recorded node is at both its name and %s", prior.Retired)
	}
	return false, fmt.Errorf("the recorded node is at neither its name nor %s", prior.Retired)
}

// replayImport completes a published intent. It classifies first, then
// verifies the source and the stage or new tree, moves each previous
// destination still at its original name, publishes, and completes by policy.
// Every refusal keeps the intent, the stage and every recorded node.
func replayImport(folder, owner *os.File, intent importIntent) (*store, error) {
	progress, err := classifyImport(folder, intent)
	if err != nil {
		return nil, intent.refusal(folder, err)
	}
	var source *zefs.BlobStore
	if !progress.retired {
		if err := checkArtifact(intent.Source); err != nil {
			return nil, err
		}
		source, err = zefs.Open(intent.Source)
		if err != nil {
			return nil, err
		}
		defer source.Close() //nolint:errcheck // readonly source.
	}
	if !progress.published {
		if err := publishStage(folder, intent, progress, source); err != nil {
			return nil, intent.refusal(folder, err)
		}
	}
	result, err := openImported(folder, intent)
	if err != nil {
		return nil, intent.refusal(folder, err)
	}
	// A retired init source preserves writes made to the tree after retirement.
	if !progress.retired && progress.published {
		if err := equalImport(result, source); err != nil {
			return nil, errors.Join(intent.refusal(folder, fmt.Errorf("the published tree differs from its source: %w", err)), result.Close())
		}
	}
	if err := finishImport(folder, intent, progress.retired); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	result.owner = owner
	return result, nil
}

// publishStage verifies the stage against the source, moves the recorded
// previous tree then the recorded seed, and renames the stage to database.
// Each effect is synced before the next starts.
func publishStage(folder *os.File, intent importIntent, progress importProgress, source *zefs.BlobStore) error {
	stageFolder, err := duplicateFolder(folder)
	if err != nil {
		return err
	}
	stage, err := openTree(stageFolder, intent.Stage, nil, true)
	if err != nil {
		return errors.Join(err, stageFolder.Close())
	}
	err = equalImport(stage, source)
	if closeErr := stage.Close(); closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	if err != nil {
		return fmt.Errorf("the stage differs from its source: %w", err)
	}
	if !progress.treeRetired {
		if err := moveRecorded(folder, treeName, intent.Tree.Retired, intent.Tree.identity(), true); err != nil {
			return err
		}
	}
	if !progress.seedRetired {
		if err := moveRecorded(folder, blobName, intent.Seed.Retired, intent.Seed.identity(), false); err != nil {
			return err
		}
	}
	return moveRecorded(folder, intent.Stage, treeName, intent.newTree(), true)
}

// moveRecorded renames name to target when name still holds the recorded
// identity, never over an existing target, and syncs the folder.
func moveRecorded(folder *os.File, name, target string, want nodeIdentity, directory bool) error {
	got, found, err := identify(folder, name, directory)
	if err != nil {
		return err
	}
	if !found || got != want {
		return fmt.Errorf("%s no longer holds the recorded node", filepath.Join(folder.Name(), name))
	}
	if err := zefs.RenameNoReplace(folder, name, target); err != nil {
		return err
	}
	return folder.Sync()
}

// openImported opens the published tree on its own folder handle and proves it
// is the recorded new tree.
func openImported(folder *os.File, intent importIntent) (*store, error) {
	treeFolder, err := duplicateFolder(folder)
	if err != nil {
		return nil, err
	}
	result, err := openTree(treeFolder, treeName, nil, false)
	if err != nil {
		return nil, errors.Join(err, treeFolder.Close())
	}
	var st unix.Stat_t
	if err := unix.Fstat(int(result.tree.root.Fd()), &st); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	if (nodeIdentity{Device: uint64(st.Dev), Inode: st.Ino}) != intent.newTree() { //nolint:unconvert // Dev is int32 on darwin and openbsd
		return nil, errors.Join(fmt.Errorf("%s is not the recorded new tree", result.tree.root.Name()), result.Close())
	}
	return result, nil
}

// finishImport retires an init source, then removes the intent. Retirement
// comes first so a crash between the two leaves the completed-init state: the
// source at its archive and the tree with the intent's identity. A restore
// keeps its source, so only the intent goes.
func finishImport(folder *os.File, intent importIntent, retired bool) error {
	if intent.Policy == policyRetireSource {
		parent, err := openFolder(filepath.Dir(intent.Source))
		if err != nil {
			return err
		}
		err = retireSource(parent, intent, retired)
		err = errors.Join(err, parent.Close())
		if err != nil {
			return err
		}
	}
	if err := unix.Unlinkat(int(folder.Fd()), importIntentName, 0); err != nil {
		return err
	}
	return folder.Sync()
}

// refusal names the intent, the conflict and the two ways out.
func (i importIntent) refusal(folder *os.File, err error) error {
	return fmt.Errorf("%s: %w; resolve that and run %s again, or remove that intent file once %s holds the wanted tree", filepath.Join(folder.Name(), importIntentName), err, i.recovery(), filepath.Join(folder.Name(), treeName))
}

// pendingImport answers ErrImportPending while an intent beside the store is
// unfinished or unreadable, whether or not a tree exists. It moves nothing.
// The one exception is a retiring import whose tree is published and whose
// source is already archived: that tree is the operator's store now, and the
// remaining cleanup waits for the explicit recovery command.
func pendingImport(dir string) error {
	intentPath := filepath.Join(dir, importIntentName)
	treePath := filepath.Join(dir, treeName)
	found, err := present(intentPath)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	folder, err := openFolder(dir)
	if err != nil {
		return err
	}
	defer folder.Close() //nolint:errcheck // read-only probe.
	intent, exists, err := readImportIntent(folder)
	if err != nil {
		return fmt.Errorf("%w: %s cannot be read: %w; remove that intent file once %s holds the wanted tree", ErrImportPending, intentPath, err, treePath)
	}
	if !exists {
		return nil
	}
	done, err := completedInit(folder, intent)
	if done {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %s records an unfinished %s of %s (%w); finish it with %s, or remove that intent file once %s holds the wanted tree", ErrImportPending, intentPath, intent.Policy, intent.Source, err, intent.recovery(), treePath)
	}
	return fmt.Errorf("%w: %s records an unfinished %s of %s; finish it with %s, or remove that intent file once %s holds the wanted tree", ErrImportPending, intentPath, intent.Policy, intent.Source, intent.recovery(), treePath)
}

// completedInit reports the one intent state an ordinary open may use: a
// retiring import that published its tree and archived its source.
func completedInit(folder *os.File, intent importIntent) (bool, error) {
	if intent.Policy != policyRetireSource {
		return false, nil
	}
	original, err := present(intent.Source)
	if err != nil {
		return false, err
	}
	if original {
		return false, nil
	}
	progress, err := classifyImport(folder, intent)
	if err != nil {
		return false, err
	}
	return progress.published && progress.retired, nil
}
