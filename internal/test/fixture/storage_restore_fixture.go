// Design: docs/architecture/testing/ci-format.md -- restore functional scenarios
// Related: storage_tree_fixture.go -- dispatches these scenarios and holds the shared helpers.

package fixture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/pkg/zefs"
)

// storageRestoreRetired is the retirement name the resume fixture records for
// the old tree. A crash leaves whatever name the intent recorded, so a fixed
// one proves replay invents none of its own.
const storageRestoreRetired = storageTreeName + ".replaced-20260101T000000.000000000"

// storageRestoreEndpoint is the SSH endpoint the seed records, as host/port;
// storageRestoreDialed is how an owner refusal prints it, as host:port.
const (
	storageRestoreEndpoint = "127.0.0.1/2222"
	storageRestoreDialed   = "127.0.0.1:2222"
)

// The keys and interrupted states the restore scenarios share.
const (
	storageKeyActive      = "file/active/router.conf"
	storageKeyPassword    = "meta/auth/local/password" //nolint:gosec // a store key name, not a credential
	storagePriorAbsent    = "absent"
	storagePriorPresent   = "present"
	storagePriorSource    = "source"
	storageStatePreMove   = "pre-move"
	storageStateAbsent    = "absent"
	storageStatePublished = "published"
	storageStateUnrelated = "unrelated"
)

// storageIntentFixture is the layout of storage's importIntent. A crash is the
// only producer of an interrupted intent, so a fixture writes it by hand.
type storageIntentFixture struct {
	Policy  string              `json:"policy"`
	Source  string              `json:"source"`
	Digest  string              `json:"digest"`
	Archive string              `json:"archive,omitempty"`
	Stage   string              `json:"stage"`
	Device  uint64              `json:"device"`
	Inode   uint64              `json:"inode"`
	Tree    storagePriorFixture `json:"tree"`
	Seed    storagePriorFixture `json:"seed"`
}

// storagePriorFixture is the layout of storage's priorNode.
type storagePriorFixture struct {
	State   string `json:"state"`
	Retired string `json:"retired,omitempty"`
	Device  uint64 `json:"device,omitempty"`
	Inode   uint64 `json:"inode,omitempty"`
}

// storageWriteTree creates the live tree under dir holding exactly values.
func storageWriteTree(dir string, values map[string][]byte) error {
	store, err := storage.Create(dir)
	if err != nil {
		return err
	}
	for key, value := range values {
		if err := store.WriteKey(key, value); err != nil {
			return errors.Join(err, store.Close())
		}
	}
	return store.Close()
}

// storageWriteBlob creates the artifact at path holding exactly values.
func storageWriteBlob(path string, values map[string][]byte) error {
	store, err := storage.CreateBlob(path)
	if err != nil {
		return err
	}
	for key, value := range values {
		if err := store.WriteKey(key, value); err != nil {
			return errors.Join(err, store.Close())
		}
	}
	return store.Close()
}

// storageTreeEquals opens the tree at path read-only and requires exactly values.
func storageTreeEquals(path string, values map[string][]byte) error {
	store, err := storage.OpenTree(path, false)
	if err != nil {
		return err
	}
	defer store.Close() //nolint:errcheck // read-only comparison.
	keys, err := store.ListKeys("")
	if err != nil {
		return err
	}
	if len(keys) != len(values) {
		return fmt.Errorf("%s holds %d keys, want %d: %v", path, len(keys), len(values), keys)
	}
	for key, want := range values {
		got, err := store.ReadKey(key)
		if err != nil {
			return fmt.Errorf("%s: %s: %w", path, key, err)
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("%s: %s = %q, want %q", path, key, got, want)
		}
	}
	return nil
}

// storageRetiredNames lists the non-lock nodes named <name>.replaced-* in dir.
func storageRetiredNames(dir, name string) ([]string, error) {
	names, err := filepath.Glob(filepath.Join(dir, name+".replaced-*"))
	if err != nil {
		return nil, err
	}
	retired := names[:0]
	for _, candidate := range names {
		if !strings.HasSuffix(candidate, ".lock") {
			retired = append(retired, candidate)
		}
	}
	return retired, nil
}

// storageRequireOutput fails when output lacks any of wants.
func storageRequireOutput(output []byte, wants ...string) error {
	for _, want := range wants {
		if !bytes.Contains(output, []byte(want)) {
			return fmt.Errorf("output lacks %q:\n%s", want, output)
		}
	}
	return nil
}

// storageRestoreFullScenario proves AC-5, AC-6 and AC-22 through the binary:
// a corrupt artifact, the canonical seed and an owned store are each refused
// with the tree untouched; then the restore replaces the tree, retires the old
// tree and an unrelated seed at recorded names, and leaves the artifact as it was.
func storageRestoreFullScenario(ctx context.Context) error {
	old := map[string][]byte{
		zefs.KeySSHDefault.Pattern: []byte(storageRestoreEndpoint),
		storageKeyActive:           []byte("old config\n"),
	}
	historyDigest := fmt.Sprintf("%x", sha256.Sum256([]byte("history")))
	backup := map[string][]byte{
		zefs.KeySSHDefault.Pattern:        []byte(storageRestoreEndpoint),
		storageKeyActive:                  []byte("bgp { }\n"),
		storageKeyPassword:                {0, 1, 2, 0xff},
		"custom/unregistered/nested/key":  []byte("unknown keys survive"),
		"file/20260926-101500.000/r.conf": []byte("sha256:" + historyDigest),
		"object/" + historyDigest:         []byte("history"),
	}
	if err := storageWriteTree(".", old); err != nil {
		return err
	}
	if err := storageWriteBlob("backup.zefs", backup); err != nil {
		return err
	}
	before, err := os.ReadFile("backup.zefs")
	if err != nil {
		return err
	}
	restore := func(source string) ([]byte, error) {
		return storageCommand(ctx, "", "data", "--path", storageTreeName, "restore", source, "full")
	}
	if err := os.WriteFile("bad.zefs", []byte("not a zefs artifact"), 0o600); err != nil {
		return err
	}
	if output, err := restore("bad.zefs"); err == nil {
		return fmt.Errorf("a corrupt artifact was restored:\n%s", output)
	}
	owner, err := storage.OpenTree(storageTreeName, true)
	if err != nil {
		return err
	}
	output, err := restore("backup.zefs")
	closeErr := owner.Close()
	if err == nil {
		return errors.Join(fmt.Errorf("restore ran beside the owner:\n%s", output), closeErr)
	}
	if err := storageRequireOutput(output, storageRestoreDialed, "stop the daemon"); err != nil {
		return errors.Join(err, closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if err := storageTreeEquals(storageTreeName, old); err != nil {
		return fmt.Errorf("a refused restore changed the tree: %w", err)
	}
	// The unrelated seed arrives by rename: CreateBlob refuses the canonical
	// name beside a live tree.
	if err := storageWriteBlob("seed.zefs", map[string][]byte{"meta/seed/key": []byte("unrelated")}); err != nil {
		return err
	}
	if err := os.Rename("seed.zefs", storage.BlobName); err != nil {
		return err
	}
	output, err = restore(storage.BlobName)
	if err == nil {
		return fmt.Errorf("the canonical seed was restored:\n%s", output)
	}
	if err := storageRequireOutput(output, "ze init --from", "ze data restore <new-name> full"); err != nil {
		return err
	}
	if _, err := os.Lstat(storage.BlobName); err != nil {
		return fmt.Errorf("a refused restore moved the seed: %w", err)
	}
	output, err = storageRequireCommand(ctx, "", "data", "--path", storageTreeName, "restore", "backup.zefs", "full")
	if err != nil {
		return err
	}
	if err := storageRequireOutput(output, "6 keys", "database.replaced-"); err != nil {
		return err
	}
	if err := storageTreeEquals(storageTreeName, backup); err != nil {
		return err
	}
	retired, err := storageRetiredNames(".", storageTreeName)
	if err != nil {
		return err
	}
	if len(retired) != 1 {
		return fmt.Errorf("want one retired tree, found %v", retired)
	}
	if err := storageTreeEquals(retired[0], old); err != nil {
		return err
	}
	seeds, err := storageRetiredNames(".", storage.BlobName)
	if err != nil {
		return err
	}
	if len(seeds) != 1 {
		return fmt.Errorf("want one retired seed, found %v", seeds)
	}
	if err := storageBlobEquals(seeds[0], map[string][]byte{"meta/seed/key": []byte("unrelated")}); err != nil {
		return err
	}
	return storageRestoreSourceKept("backup.zefs", before)
}

// storageRestoreSourceKept requires the artifact byte-unchanged at its own
// name, with no archive beside it and no intent left behind.
func storageRestoreSourceKept(source string, before []byte) error {
	after, err := os.ReadFile(source) //nolint:gosec // the fixture reads the artifact it wrote
	if err != nil {
		return err
	}
	if !bytes.Equal(before, after) {
		return fmt.Errorf("restore changed its source %s", source)
	}
	archives, err := storageRetiredNames(filepath.Dir(source), filepath.Base(source))
	if err != nil {
		return err
	}
	if len(archives) != 0 {
		return fmt.Errorf("restore retired its source: %v", archives)
	}
	intent := filepath.Join(filepath.Dir(source), "database.import-intent")
	if _, err := os.Lstat(intent); !errors.Is(err, fs.ErrNotExist) {
		return errors.Join(fmt.Errorf("%s survived the restore", intent), err)
	}
	return nil
}

// storageNodeIdentity answers the device and inode of path without following it.
func storageNodeIdentity(path string) (uint64, uint64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, 0, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("%s: no device and inode on this platform", path)
	}
	return uint64(st.Dev), st.Ino, nil //nolint:unconvert,gosec // Dev is int32 on darwin and openbsd
}

// storageInterruptedRestore leaves dir as a keep-source restore that crashed
// after its intent became durable, at state: "pre-move", "absent",
// "published", or "unrelated" (absent, then an unrelated tree at database).
// The intent's layout is storage's importIntent, written by hand because a
// crash is the only producer of this state.
func storageInterruptedRestore(dir, state string, old, backup map[string][]byte) (string, error) {
	if err := os.Mkdir(dir, 0o700); err != nil {
		return "", err
	}
	folder, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	folder, err = filepath.Abs(folder)
	if err != nil {
		return "", err
	}
	if err := storageWriteTree(folder, old); err != nil {
		return "", err
	}
	source := filepath.Join(folder, "backup.zefs")
	if err := storageWriteBlob(source, backup); err != nil {
		return "", err
	}
	stage := "database.import-tmp-fixture"
	if err := os.Mkdir(filepath.Join(folder, stage), 0o700); err != nil {
		return "", err
	}
	stageStore, err := storage.OpenTree(filepath.Join(folder, stage), true)
	if err != nil {
		return "", err
	}
	for key, value := range backup {
		if err := stageStore.WriteKey(key, value); err != nil {
			return "", errors.Join(err, stageStore.Close())
		}
	}
	if err := stageStore.Close(); err != nil {
		return "", err
	}
	data, err := os.ReadFile(source) //nolint:gosec // the fixture reads the artifact it wrote
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	stageDevice, stageInode, err := storageNodeIdentity(filepath.Join(folder, stage))
	if err != nil {
		return "", err
	}
	treeDevice, treeInode, err := storageNodeIdentity(filepath.Join(folder, storageTreeName))
	if err != nil {
		return "", err
	}
	intent := storageIntentFixture{
		Policy: "keep-source",
		Source: source,
		Digest: hex.EncodeToString(digest[:]),
		Stage:  stage,
		Device: stageDevice,
		Inode:  stageInode,
		Tree:   storagePriorFixture{State: storagePriorPresent, Retired: storageRestoreRetired, Device: treeDevice, Inode: treeInode},
		Seed:   storagePriorFixture{State: storagePriorAbsent},
	}
	encoded, err := json.Marshal(intent)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(folder, "database.import-intent"), encoded, 0o600); err != nil {
		return "", err
	}
	move := func(from, to string) error { return os.Rename(filepath.Join(folder, from), filepath.Join(folder, to)) }
	switch state {
	case storageStatePreMove:
		return source, nil
	case storageStateAbsent:
		return source, move(storageTreeName, storageRestoreRetired)
	case storageStatePublished:
		if err := move(storageTreeName, storageRestoreRetired); err != nil {
			return "", err
		}
		return source, move(stage, storageTreeName)
	case storageStateUnrelated:
		if err := move(storageTreeName, storageRestoreRetired); err != nil {
			return "", err
		}
		return source, os.Mkdir(filepath.Join(folder, storageTreeName), 0o700)
	}
	return "", fmt.Errorf("unknown interrupted state %q", state)
}

// storageRestoreResumeScenario proves AC-23 through the binary: at each
// interrupted state an ordinary open reports the unfinished restore and prints
// its command, that command finishes it with the source kept and the old tree
// at its recorded name, and an unrelated tree at database is refused and kept.
func storageRestoreResumeScenario(ctx context.Context) error {
	old := map[string][]byte{storageKeyActive: []byte("old config\n")}
	backup := map[string][]byte{
		storageKeyActive:   []byte("bgp { }\n"),
		storageKeyPassword: []byte("restored"),
	}
	for _, state := range []string{storageStatePreMove, storageStateAbsent, storageStatePublished, storageStateUnrelated} {
		source, err := storageInterruptedRestore(state, state, old, backup)
		if err != nil {
			return fmt.Errorf("%s: %w", state, err)
		}
		tree := filepath.Join(filepath.Dir(source), storageTreeName)
		output, err := storageCommand(ctx, "", "data", "--path", tree, "list")
		if err == nil {
			return fmt.Errorf("%s: an ordinary open ignored the unfinished restore:\n%s", state, output)
		}
		command := "ze data restore " + source + " full"
		if err := storageRequireOutput(output, "unfinished import", command); err != nil {
			return fmt.Errorf("%s: %w", state, err)
		}
		before, err := os.ReadFile(source) //nolint:gosec // the fixture reads the artifact it wrote
		if err != nil {
			return err
		}
		output, err = storageCommand(ctx, "", "data", "--path", tree, "restore", source, "full")
		if state == storageStateUnrelated {
			if err == nil {
				return fmt.Errorf("unrelated: replay moved an unrelated tree:\n%s", output)
			}
			if _, statErr := os.Lstat(filepath.Join(filepath.Dir(source), "database.import-intent")); statErr != nil {
				return fmt.Errorf("unrelated: the refusal removed the intent: %w", statErr)
			}
			if _, statErr := os.Lstat(tree); statErr != nil {
				return fmt.Errorf("unrelated: the refusal moved the unrelated tree: %w", statErr)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("%s: ze data restore: %w\n%s", state, err, output)
		}
		if err := storageTreeEquals(tree, backup); err != nil {
			return fmt.Errorf("%s: %w", state, err)
		}
		if err := storageTreeEquals(filepath.Join(filepath.Dir(source), storageRestoreRetired), old); err != nil {
			return fmt.Errorf("%s: %w", state, err)
		}
		if err := storageRestoreSourceKept(source, before); err != nil {
			return fmt.Errorf("%s: %w", state, err)
		}
	}
	return nil
}

// storageRestoreConfigScenario proves AC-7 through the binary: the artifact's
// config becomes the device's new active version, and credentials and every
// other key of the tree stay byte-unchanged.
func storageRestoreConfigScenario(ctx context.Context) error {
	device := resolve.DefaultConfig(nil)
	if err := storageWriteTree(".", map[string][]byte{
		"file/active/" + device:    []byte("old config\n"),
		storageKeyPassword:         []byte("device secret"),
		zefs.KeySSHDefault.Pattern: []byte(storageRestoreEndpoint),
	}); err != nil {
		return err
	}
	if err := storageWriteBlob("backup.zefs", map[string][]byte{
		"file/active/" + device: []byte("bgp { }\n"),
		storageKeyPassword:      []byte("artifact secret"),
	}); err != nil {
		return err
	}
	output, err := storageRequireCommand(ctx, "", "data", "--path", storageTreeName, "restore", "backup.zefs", "config")
	if err != nil {
		return err
	}
	if err := storageRequireOutput(output, "committed as "+device); err != nil {
		return err
	}
	store, err := storage.OpenTree(storageTreeName, false)
	if err != nil {
		return err
	}
	defer store.Close() //nolint:errcheck // read-only comparison.
	for key, want := range map[string]string{
		"file/active/" + device:    "bgp { }\n",
		storageKeyPassword:         "device secret",
		zefs.KeySSHDefault.Pattern: storageRestoreEndpoint,
	} {
		got, err := store.ReadKey(key)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		if string(got) != want {
			return fmt.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	active, err := store.ReadKey("meta/config/" + device + "/active")
	if err != nil {
		return fmt.Errorf("the restore moved no active pointer: %w", err)
	}
	if len(active) == 0 {
		return errors.New("the active pointer is empty")
	}
	return nil
}
