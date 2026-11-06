package dockerhost

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// VALIDATES: the image is keyed by the runtime kernel's bytes, so a kernel rebuilt
// under the same cache entry name builds a new image, and the same bytes reuse it.
// PREVENTS: `docker-lab command` refusing with "rebuild the image" while the
// rebuild answers the old image, because kernel builds are not reproducible and the
// entry's name does not change when an evicted entry is rebuilt.
func TestDockerHostBuildKeysTheImageByKernelBytes(t *testing.T) {
	repo := fakeRepo(t)
	recorder := newRecordingBuilder(t)
	tree := t.TempDir()
	recorder.kernelTree = func(string, string) (string, error) { return tree, nil }
	vmlinuz := filepath.Join(tree, "vmlinuz")
	if err := os.WriteFile(vmlinuz, []byte("kernel built first"), 0o600); err != nil {
		t.Fatal(err)
	}
	req := buildRequest{repo: repo, arch: "arm64", pins: fullPins(t), sizeGiB: imageSizeGiBDefault}

	first, err := recorder.build(context.Background(), req)
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	same, err := recorder.build(context.Background(), req)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	if same.Built || same.Path != first.Path {
		t.Errorf("the same kernel bytes built again: built %v, path %s, want %s", same.Built, same.Path, first.Path)
	}
	if err := os.WriteFile(vmlinuz, []byte("kernel built again"), 0o600); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := recorder.build(context.Background(), req)
	if err != nil {
		t.Fatalf("build over the rebuilt kernel: %v", err)
	}
	if !rebuilt.Built || rebuilt.Path == first.Path {
		t.Errorf("a rebuilt kernel answered the old image: built %v, path %s", rebuilt.Built, rebuilt.Path)
	}
}

// VALIDATES: a runtime tree with no kernel to digest refuses before any image work.
// PREVENTS: an image keyed by nothing, which every later kernel would share.
func TestDockerHostBuildRefusesATreeWithNoKernel(t *testing.T) {
	recorder := newRecordingBuilder(t)
	empty := t.TempDir()
	recorder.kernelTree = func(string, string) (string, error) { return empty, nil }
	req := buildRequest{repo: fakeRepo(t), arch: "arm64", pins: fullPins(t), sizeGiB: imageSizeGiBDefault}
	_, err := recorder.build(context.Background(), req)
	if err == nil {
		t.Fatal("a runtime tree with no vmlinuz was accepted")
	}
	if len(recorder.gokRuns) != 0 || len(recorder.containerRuns) != 0 {
		t.Fatal("the refusal reached image construction")
	}
	for _, want := range []string{"rm -rf \"" + empty + "\"", "./le setup docker host build arch arm64"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal lacks the command %q: %v", want, err)
		}
	}
}

// VALIDATES: a kernel this user cannot read is reported as unreadable, with the
// fix that matches, and the refusal tells nobody to remove the entry.
// PREVENTS: a permission error costing a thirty-minute kernel build.
func TestDockerHostBuildKeepsAnUnreadableKernel(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a file of any mode")
	}
	recorder := newRecordingBuilder(t)
	tree := t.TempDir()
	vmlinuz := filepath.Join(tree, runtimeKernelFile)
	if err := os.WriteFile(vmlinuz, []byte("kernel"), 0o000); err != nil { //nolint:gosec // the unreadable mode is the fixture
		t.Fatal(err)
	}
	recorder.kernelTree = func(string, string) (string, error) { return tree, nil }
	req := buildRequest{repo: fakeRepo(t), arch: "arm64", pins: fullPins(t), sizeGiB: imageSizeGiBDefault}
	_, err := recorder.build(context.Background(), req)
	if err == nil {
		t.Fatal("an unreadable kernel was accepted")
	}
	if strings.Contains(err.Error(), "rm -rf") {
		t.Errorf("the refusal removes a built kernel over a read error: %v", err)
	}
	if !strings.Contains(err.Error(), vmlinuz) {
		t.Errorf("the refusal does not name the file: %v", err)
	}
}

// VALIDATES: a new image bounds its arch's directory: the oldest entries go, and
// the image just built and an entry inside the eviction grace stay.
// PREVENTS: every kernel rebuild leaving a multi-gigabyte image behind for good.
// METHOD: age real entries with Chtimes, build once through the recording builder,
// and read the directory.
func TestDockerHostBuildEvictsOldImages(t *testing.T) {
	recorder := newRecordingBuilder(t)
	req := buildRequest{repo: fakeRepo(t), arch: "arm64", pins: fullPins(t), sizeGiB: imageSizeGiBDefault}
	images := filepath.Join(recorder.cacheDir, req.arch)
	old := time.Now().Add(-48 * time.Hour)
	names := []string{"old-a", "old-b", "old-c", "old-d"}
	for index, name := range names {
		entry := filepath.Join(images, name)
		writeFile(t, filepath.Join(entry, imageFileName), []byte("image"))
		at := old.Add(time.Duration(index) * time.Hour)
		if err := os.Chtimes(entry, at, at); err != nil {
			t.Fatal(err)
		}
	}
	fresh := filepath.Join(images, "fresh")
	writeFile(t, filepath.Join(fresh, imageFileName), []byte("image"))

	built, err := recorder.build(context.Background(), req)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !built.Built {
		t.Fatal("the build reused an image, so nothing asked for eviction")
	}
	if _, err := os.Stat(built.Path); err != nil {
		t.Errorf("the image just built is gone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fresh, imageFileName)); err != nil {
		t.Errorf("an entry inside the eviction grace is gone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(images, "old-a")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the oldest image was kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(images, "old-d")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an image past the bound and the grace was kept: %v", err)
	}
}
