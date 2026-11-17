//go:build linux

package imagestore

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/container/registry"
)

// testStaged is the staged folder name the apply tests use.
const testStaged = "apply-test"

// testModified is the modification time of every test entry that sets none.
var testModified = time.Date(2024, time.March, 4, 5, 6, 7, 0, time.UTC)

// testEntry is one entry of a test layer. Its owner is the user the test
// runs as unless it names one, so application needs no privilege.
type testEntry struct {
	name     string
	typeflag byte
	body     string
	linkname string
	mode     int64
	uid      int
	gid      int
	modified time.Time
	records  map[string]string
}

// own gives an entry the owner and group the test runs as.
func own(entry testEntry) testEntry {
	entry.uid = os.Getuid()
	entry.gid = os.Getgid()
	entry.modified = testModified
	return entry
}

func fileEntry(name, body string, mode int64) testEntry {
	return own(testEntry{name: name, typeflag: tar.TypeReg, body: body, mode: mode})
}

func folderEntry(name string, mode int64) testEntry {
	return own(testEntry{name: name, typeflag: tar.TypeDir, mode: mode})
}

func symlinkEntry(name, target string) testEntry {
	return own(testEntry{name: name, typeflag: tar.TypeSymlink, linkname: target, mode: 0o777})
}

func hardLinkEntry(name, target string) testEntry {
	return own(testEntry{name: name, typeflag: tar.TypeLink, linkname: target, mode: 0o644})
}

func fifoEntry(name string, mode int64) testEntry {
	return own(testEntry{name: name, typeflag: tar.TypeFifo, mode: mode})
}

func deviceEntry(name string, typeflag byte) testEntry {
	return own(testEntry{name: name, typeflag: typeflag, mode: 0o666})
}

// layerTar answers an uncompressed layer holding entries, in order.
func layerTar(t testing.TB, entries ...testEntry) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	for _, entry := range entries {
		header := &tar.Header{
			Name:       entry.name,
			Typeflag:   entry.typeflag,
			Linkname:   entry.linkname,
			Mode:       entry.mode,
			Uid:        entry.uid,
			Gid:        entry.gid,
			ModTime:    entry.modified,
			Size:       int64(len(entry.body)),
			PAXRecords: entry.records,
			Format:     tar.FormatPAX,
		}
		if entry.typeflag != tar.TypeReg {
			header.Size = 0
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatalf("write the header of %q: %v", entry.name, err)
		}
		if _, err := writer.Write([]byte(entry.body)); err != nil {
			t.Fatalf("write the body of %q: %v", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close the archive: %v", err)
	}
	return archive.Bytes()
}

// diffDigestOf answers the digest of a decompressed layer.
func diffDigestOf(t testing.TB, archive []byte) registry.Digest {
	t.Helper()
	sum := sha256.Sum256(archive)
	digest, err := registry.ParseDigest(digestTextPrefix + hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("digest of the layer: %v", err)
	}
	return digest
}

// testLimits are bounds no test layer reaches unless it means to.
func testLimits() applyLimits {
	return applyLimits{expandedBytesMax: 64 << 20, entriesMax: 10_000}
}

// appliedStore is a prepared store and an applier on one staged folder.
type appliedStore struct {
	applier *layerApplier
	// dataDir holds the store root, and the markers of a test, outside it.
	dataDir string
	// rootPath is the staged root, as a path for the test to read.
	rootPath string
	// stagedPath is the staged folder, whose content the test may see change.
	stagedPath string
}

// newAppliedStore opens a store and an applier on testStaged, and closes the
// applier when the test ends.
func newAppliedStore(t *testing.T, limits applyLimits) appliedStore {
	t.Helper()
	root := preparedRoot(t)
	return newAppliedStoreIn(t, root, testStaged, limits)
}

// newAppliedStoreIn opens an applier on staged in an open store.
func newAppliedStoreIn(t *testing.T, root *storeRoot, staged string, limits applyLimits) appliedStore {
	t.Helper()
	applier, err := root.newLayerApplier(staged, limits)
	if err != nil {
		t.Fatalf("open the applier: %v", err)
	}
	t.Cleanup(func() {
		if err := applier.close(); err != nil {
			t.Errorf("close the applier: %v", err)
		}
	})
	stagedPath := filepath.Join(root.path, stagingFolder, staged)
	return appliedStore{
		applier:    applier,
		dataDir:    filepath.Dir(root.path),
		rootPath:   filepath.Join(stagedPath, rootfsFolder),
		stagedPath: stagedPath,
	}
}

// applyTar applies an uncompressed layer with its own digest.
func (s appliedStore) applyTar(t *testing.T, archive []byte) error {
	t.Helper()
	return s.applier.apply(bytes.NewReader(archive), registry.CompressionNone, diffDigestOf(t, archive))
}

// mustApply applies a layer that must apply.
func (s appliedStore) mustApply(t *testing.T, entries ...testEntry) {
	t.Helper()
	if err := s.applyTar(t, layerTar(t, entries...)); err != nil {
		t.Fatalf("apply the layer: %v", err)
	}
}

// requireRefusal checks err is a refusal by rule.
func requireRefusal(t *testing.T, err error, rule refusalRule) *layerRefusal {
	t.Helper()
	refusal, ok := errors.AsType[*layerRefusal](err)
	if !ok {
		t.Fatalf("want a refusal by %q, got %v", rule, err)
	}
	if refusal.rule != rule {
		t.Fatalf("want a refusal by %q, got %q (%v)", rule, refusal.rule, err)
	}
	return refusal
}

// checkRefusal fails the test unless err is a layer refusal by rule. It is
// requireRefusal for a test that reads nothing more from the refusal.
func checkRefusal(t *testing.T, err error, rule refusalRule) {
	t.Helper()
	if requireRefusal(t, err, rule) == nil {
		t.Fatalf("want a refusal by %q, got %v", rule, err)
	}
}

// treeEntry is what a test compares of one name of a tree.
type treeEntry struct {
	mode     fs.FileMode
	uid      uint32
	gid      uint32
	modified int64
	inode    uint64
	links    uint64
	content  string
	target   string
}

// readTree answers every name under folder, skipping the names under any
// path of skip, with what a test compares of each. Inodes are recorded only
// when withInodes is set: two trees built apart never share them.
func readTree(t *testing.T, folder string, withInodes bool, skip ...string) map[string]treeEntry {
	t.Helper()
	tree := make(map[string]treeEntry)
	err := filepath.WalkDir(folder, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if slices.Contains(skip, path) {
			return filepath.SkipDir
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatalf("no stat for %s", path)
		}
		entry := treeEntry{mode: info.Mode(), uid: stat.Uid, gid: stat.Gid, modified: info.ModTime().UnixNano()}
		if withInodes {
			entry.inode = stat.Ino
			entry.links = uint64(stat.Nlink) //nolint:unconvert // Nlink is a uint32 on arm64, a uint64 on amd64
		}
		if info.Mode().IsRegular() {
			content, err := os.ReadFile(path) //nolint:gosec // G304: a test's own temporary tree
			if err != nil {
				return err
			}
			entry.content = string(content)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			entry.target, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		name, err := filepath.Rel(folder, path)
		if err != nil {
			return err
		}
		tree[name] = entry
		return nil
	})
	if err != nil {
		t.Fatalf("read the tree %s: %v", folder, err)
	}
	return tree
}

// outsideSnapshot answers the data directory outside the staged folder:
// the store root's other folders and the test's markers beside it. A layer
// application changes none of it.
func (s appliedStore) outsideSnapshot(t *testing.T) map[string]treeEntry {
	t.Helper()
	return readTree(t, s.dataDir, true, s.stagedPath)
}

// requireOutsideUnchanged fails when the outside differs from before.
func (s appliedStore) requireOutsideUnchanged(t *testing.T, before map[string]treeEntry) {
	t.Helper()
	after := s.outsideSnapshot(t)
	if maps.Equal(before, after) {
		return
	}
	for name, entry := range after {
		was, ok := before[name]
		if !ok {
			t.Errorf("outside the root, %s was created: %+v", name, entry)
			continue
		}
		if was != entry {
			t.Errorf("outside the root, %s changed: was %+v, is %+v", name, was, entry)
		}
	}
	for name := range before {
		if _, ok := after[name]; !ok {
			t.Errorf("outside the root, %s was removed", name)
		}
	}
	t.FailNow()
}

// writeMarker writes a marker file beside the store root and answers its
// path.
func (s appliedStore) writeMarker(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(s.dataDir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write the marker %s: %v", path, err)
	}
	return path
}

// requireAbsent fails when name exists under the root.
func requireAbsent(t *testing.T, root, name string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, name)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s: want absent, got %v", name, err)
	}
}

// requireContent fails when name under the root is not a regular file
// holding content.
func requireContent(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s: want a regular file, got %v", name, info.Mode())
	}
	read, err := os.ReadFile(path) //nolint:gosec // G304: a test's own temporary tree
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if string(read) != content {
		t.Fatalf("%s: want %q, got %q", name, content, read)
	}
}
