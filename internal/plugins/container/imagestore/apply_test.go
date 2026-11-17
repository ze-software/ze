//go:build linux

package imagestore

import (
	"bytes"
	"compress/gzip"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/ze-software/ze/internal/plugins/container/registry"
)

// regularTree is a layer of every entry type the store creates, with modes
// that carry the set-user, set-group and sticky bits.
func regularTree(t *testing.T) []byte {
	t.Helper()
	etc := folderEntry("etc", 0o755)
	etc.modified = time.Date(2023, time.July, 1, 2, 3, 4, 0, time.UTC)
	return layerTar(t,
		folderEntry("./", 0o755),
		etc,
		fileEntry("etc/hosts", "127.0.0.1 localhost\n", 0o644),
		folderEntry("usr/", 0o755),
		folderEntry("usr/bin/", 0o750),
		fileEntry("usr/bin/tool", "#!/bin/sh\necho tool\n", 0o4755),
		fileEntry("usr/bin/group-tool", "#!/bin/sh\necho group\n", 0o2711),
		folderEntry("tmp/", 0o1777),
		symlinkEntry("bin", "usr/bin"),
		fifoEntry("etc/pipe", 0o640),
		fileEntry("./etc/motd", "welcome\n", 0o600),
	)
}

// rawMode answers the twelve mode bits of a name, read with no link
// followed.
func rawMode(t *testing.T, path string) uint32 {
	t.Helper()
	var stat syscall.Stat_t
	if err := syscall.Lstat(path, &stat); err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	return stat.Mode & 0o7777
}

// TestApplyRegularTreeKeepsOwnerModeTime proves files, folders, links and a
// pipe read back as written: content, owner, the twelve mode bits, the
// link target, and the times, a folder's set after its children.
//
// Method: apply one layer of every type, then read each name back. The
// folder etc is given a time older than its children: had its time been
// set before the files under it were created, it would read the time of
// the last creation instead.
func TestApplyRegularTreeKeepsOwnerModeTime(t *testing.T) {
	store := newAppliedStore(t, testLimits())
	if err := store.applyTar(t, regularTree(t)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	root := store.rootPath

	requireContent(t, root, "etc/hosts", "127.0.0.1 localhost\n")
	requireContent(t, root, "etc/motd", "welcome\n")
	requireContent(t, root, "usr/bin/tool", "#!/bin/sh\necho tool\n")

	modes := map[string]uint32{
		".":                  0o755,
		"etc":                0o755,
		"etc/hosts":          0o644,
		"etc/motd":           0o600,
		"etc/pipe":           0o640,
		"usr/bin":            0o750,
		"usr/bin/tool":       0o4755,
		"usr/bin/group-tool": 0o2711,
		"tmp":                0o1777,
	}
	for name, want := range modes {
		if got := rawMode(t, filepath.Join(root, name)); got != want {
			t.Errorf("%s: mode %#o, want %#o", name, got, want)
		}
	}

	tree := readTree(t, root, false)
	for name, entry := range tree {
		if entry.uid != uint32(os.Getuid()) {
			t.Errorf("%s: owner %d, want %d", name, entry.uid, os.Getuid())
		}
		if entry.gid != uint32(os.Getgid()) {
			t.Errorf("%s: group %d, want %d", name, entry.gid, os.Getgid())
		}
	}
	if tree["bin"].target != "usr/bin" {
		t.Errorf("bin: target %q, want usr/bin", tree["bin"].target)
	}
	if tree["etc/pipe"].mode&os.ModeNamedPipe == 0 {
		t.Errorf("etc/pipe: mode %v, want a named pipe", tree["etc/pipe"].mode)
	}
	for _, name := range []string{"etc/hosts", "usr/bin/tool", "bin", "etc/pipe", "tmp", "."} {
		if got := time.Unix(0, tree[name].modified).UTC(); !got.Equal(testModified) {
			t.Errorf("%s: modified %v, want %v", name, got, testModified)
		}
	}
	wantEtc := time.Date(2023, time.July, 1, 2, 3, 4, 0, time.UTC)
	if got := time.Unix(0, tree["etc"].modified).UTC(); !got.Equal(wantEtc) {
		t.Errorf("etc: modified %v, want %v, set after its children", got, wantEtc)
	}
}

// TestApplyCompressionsProduceSameTree proves an uncompressed, a gzip and a
// zstd layer of one archive give one tree.
//
// Method: compress one archive three ways, apply each into its own staged
// root of one store, each against the digest of the uncompressed archive,
// and compare the three trees name by name: type, mode, owner, time,
// content and link target.
func TestApplyCompressionsProduceSameTree(t *testing.T) {
	archive := regularTree(t)
	diffID := diffDigestOf(t, archive)

	var gzipped bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzipped)
	if _, err := gzipWriter.Write(archive); err != nil {
		t.Fatalf("gzip: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzip: %v", err)
	}
	var zstded bytes.Buffer
	zstdWriter, err := zstd.NewWriter(&zstded)
	if err != nil {
		t.Fatalf("zstd: %v", err)
	}
	if _, err := zstdWriter.Write(archive); err != nil {
		t.Fatalf("zstd: %v", err)
	}
	if err := zstdWriter.Close(); err != nil {
		t.Fatalf("zstd: %v", err)
	}

	root := preparedRoot(t)
	layers := []struct {
		staged      string
		blob        []byte
		compression registry.Compression
	}{
		{"plain", archive, registry.CompressionNone},
		{"gzip", gzipped.Bytes(), registry.CompressionGzip},
		{"zstd", zstded.Bytes(), registry.CompressionZstd},
	}
	trees := make([]map[string]treeEntry, 0, len(layers))
	for _, layer := range layers {
		store := newAppliedStoreIn(t, root, layer.staged, testLimits())
		if err := store.applier.apply(bytes.NewReader(layer.blob), layer.compression, diffID); err != nil {
			t.Fatalf("apply the %s layer: %v", layer.staged, err)
		}
		trees = append(trees, readTree(t, store.rootPath, false))
	}
	if len(trees[0]) < 10 {
		t.Fatalf("the plain tree holds %d names, want the whole layer", len(trees[0]))
	}
	for i, tree := range trees[1:] {
		if !maps.Equal(trees[0], tree) {
			t.Errorf("the %s tree differs from the plain one:\nplain %+v\n%s %+v", layers[i+1].staged, trees[0], layers[i+1].staged, tree)
		}
	}

	unaccepted := newAppliedStoreIn(t, root, "unspecified", testLimits())
	err = unaccepted.applier.apply(bytes.NewReader(archive), registry.CompressionUnspecified, diffID)
	checkRefusal(t, err, ruleCompression)
}

// TestApplyDiffDigestMismatchRefused proves a decompressed layer that does
// not hash to the digest the configuration declares refuses the image, and
// that the same layer against its own digest applies.
//
// Method: apply one archive against the digest of another, then the same
// archive against its own digest in a second staged root. The refusal must
// name both digests.
func TestApplyDiffDigestMismatchRefused(t *testing.T) {
	archive := layerTar(t, fileEntry("hello", "hello\n", 0o644))
	other := diffDigestOf(t, layerTar(t, fileEntry("hello", "other\n", 0o644)))

	root := preparedRoot(t)
	refused := newAppliedStoreIn(t, root, "refused", testLimits())
	err := refused.applier.apply(bytes.NewReader(archive), registry.CompressionNone, other)
	refusal := requireRefusal(t, err, ruleDiffDigest)
	if !strings.Contains(refusal.detail, other.String()) {
		t.Errorf("the refusal %q does not name the declared digest %s", refusal.detail, other)
	}
	if !strings.Contains(refusal.detail, diffDigestOf(t, archive).String()) {
		t.Errorf("the refusal %q does not name the digest received", refusal.detail)
	}

	applied := newAppliedStoreIn(t, root, "applied", testLimits())
	if err := applied.applyTar(t, archive); err != nil {
		t.Fatalf("apply against its own digest: %v", err)
	}
	requireContent(t, applied.rootPath, "hello", "hello\n")
}
