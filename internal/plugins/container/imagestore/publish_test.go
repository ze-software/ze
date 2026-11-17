// Design: docs/architecture/container/image-store.md -- publication, the completion mark, staging at start

package imagestore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/internal/plugins/container/registry"
)

// testDigest builds the digest of one hex pair repeated, so each test names
// distinct images without hashing anything.
func testDigest(t *testing.T, pair string) registry.Digest {
	t.Helper()
	digest, err := registry.ParseDigest(digestTextPrefix + strings.Repeat(pair, digestHexLength/2))
	if err != nil {
		t.Fatalf("test digest of %q: %v", pair, err)
	}
	return digest
}

// testFacts are the facts of a fetch that published reference.
func testFacts(reference string) imageFacts {
	return imageFacts{
		Reference:     reference,
		Platform:      "linux/amd64",
		Registry:      "registry-1.docker.io",
		Fetched:       time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC),
		ExpandedBytes: 4096,
		Entries:       3,
	}
}

// stageImage writes what an applied image holds under staging/name: the
// root with one marker file whose content tells the stagings apart, the
// manifest and the configuration.
func stageImage(t *testing.T, storePath, name, marker string) {
	t.Helper()
	folder := filepath.Join(storePath, stagingFolder, name)
	if err := os.MkdirAll(filepath.Join(folder, rootfsFolder), 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(rootfsFolder, "marker"): marker,
		manifestName:                          `{"schemaVersion":2}`,
		configName:                            `{"architecture":"amd64","os":"linux"}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// preparedRoot prepares a store root under a fresh data directory and closes
// it when the test ends.
func preparedRoot(t *testing.T) *storeRoot {
	t.Helper()
	root, err := prepareStoreRoot(filepath.Join(t.TempDir(), storeFolder))
	if err != nil {
		t.Fatalf("prepare the store root: %v", err)
	}
	t.Cleanup(func() {
		if err := root.close(); err != nil {
			t.Errorf("close the store root: %v", err)
		}
	})
	return root
}

// TestPublishIsOneRenameAndRefusesExisting proves publication moves the
// staged folder into images/ whole, by one rename, and that a second
// publication of one digest does not replace the first.
//
// Method: stage an image and publish it, then check the image folder's root
// is the same directory the staging held (a rename moves the inode, a copy
// would make a new one), the mark is there and the listing reads it. Stage a
// second image with other content under the same digest and publish it: the
// refusal must say the target exists, the first image's content and mark
// must be unchanged, and the second staging must stay for its caller.
func TestPublishIsOneRenameAndRefusesExisting(t *testing.T) {
	root := preparedRoot(t)
	digest := testDigest(t, "ab")
	imageFolder := filepath.Join(root.path, imagesFolder, digestFolder(digest))

	stageImage(t, root.path, "first", "first")
	stagedRoot, err := os.Stat(filepath.Join(root.path, stagingFolder, "first", rootfsFolder))
	if err != nil {
		t.Fatal(err)
	}
	if err := root.publish("first", digest, testFacts("docker.io/library/redis:latest")); err != nil {
		t.Fatalf("first publication: %v", err)
	}

	publishedRoot, err := os.Stat(filepath.Join(imageFolder, rootfsFolder))
	if err != nil {
		t.Fatalf("the published image has no root: %v", err)
	}
	if !os.SameFile(stagedRoot, publishedRoot) {
		t.Error("the published root is not the staged directory: publication copied instead of renaming")
	}
	if _, err := os.Stat(filepath.Join(root.path, stagingFolder, "first")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the staged folder is still in staging after publication: %v", err)
	}
	if _, err := os.Stat(filepath.Join(imageFolder, markName)); err != nil {
		t.Errorf("the published image has no completion mark: %v", err)
	}

	stageImage(t, root.path, "second", "second")
	err = root.publish("second", digest, testFacts("docker.io/library/redis:7"))
	if err == nil {
		t.Fatal("a second publication of one digest succeeded, want a refusal")
	}
	if !errors.Is(err, fs.ErrExist) {
		t.Errorf("the refusal is %v, want one that says the target exists", err)
	}

	marker, err := os.ReadFile(filepath.Join(imageFolder, rootfsFolder, "marker"))
	if err != nil {
		t.Fatal(err)
	}
	if string(marker) != "first" {
		t.Errorf("the published image holds %q after the refused publication, want first", marker)
	}
	if _, err := os.Stat(filepath.Join(root.path, stagingFolder, "second", rootfsFolder, "marker")); err != nil {
		t.Errorf("the refused staging was not left for its caller: %v", err)
	}

	rows, err := root.images()
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("the listing holds %d rows, want the one image: %+v", len(rows), rows)
	}
	if rows[0].Digest != digest.String() {
		t.Errorf("the listed digest is %q, want %q", rows[0].Digest, digest.String())
	}
	if rows[0].Reference != "docker.io/library/redis:latest" {
		t.Errorf("the listed reference is %q, want the first publication's", rows[0].Reference)
	}
	if rows[0].State != container.ImageStatePresent {
		t.Errorf("the listed state is %d, want present", rows[0].State)
	}
}

// TestListIgnoresFolderWithoutMark proves a folder with no completion mark
// is not an image, however it is named, and that an empty store lists an
// empty, non-nil set of rows.
//
// Method: list a fresh store through the command's operation. Then place a
// digest-named folder with a root and no mark under images/, publish one
// real image beside it, and list again: only the published image is a row.
func TestListIgnoresFolderWithoutMark(t *testing.T) {
	dataDir := t.TempDir()
	store := openStore(dataDir, nil)

	rows, err := store.ListImages(context.Background())
	if err != nil {
		t.Fatalf("listing an empty store: %v", err)
	}
	if rows == nil {
		t.Error("an empty store answered nil rows, want an empty list")
	}
	if len(rows) != 0 {
		t.Errorf("an empty store answered %d rows, want none", len(rows))
	}

	storePath := filepath.Join(dataDir, storeFolder)
	unmarked := testDigest(t, "cd")
	if err := os.MkdirAll(filepath.Join(storePath, imagesFolder, digestFolder(unmarked), rootfsFolder), 0o700); err != nil {
		t.Fatal(err)
	}

	published := testDigest(t, "ef")
	root, err := openStoreRoot(storePath)
	if err != nil {
		t.Fatal(err)
	}
	stageImage(t, storePath, "published", "published")
	publishErr := root.publish("published", published, testFacts("registry.example/app:1"))
	if err := root.close(); err != nil {
		t.Fatal(err)
	}
	if publishErr != nil {
		t.Fatalf("publication: %v", publishErr)
	}

	rows, err = store.ListImages(context.Background())
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("the listing holds %d rows, want only the published image: %+v", len(rows), rows)
	}
	if rows[0].Digest != published.String() {
		t.Errorf("the listed digest is %q, want the published %q, never the unmarked %q",
			rows[0].Digest, published.String(), unmarked.String())
	}
}

// TestStartEmptiesStagingAndKeepsCheckedBlobs proves a start empties
// staging and keeps a blob named by its digest, and that only the start
// empties it: an operation later in the same run leaves staging alone, so a
// fetch in flight keeps its work.
//
// Method: prepare a store, then leave in it what a stopped run leaves: a
// staged root, a partial download in staging, and a checked blob. Open the
// store again, as a new run does, and look at both folders. Then place a
// folder in staging and run the listing in that same run.
func TestStartEmptiesStagingAndKeepsCheckedBlobs(t *testing.T) {
	dataDir := t.TempDir()
	openStore(dataDir, nil)

	storePath := filepath.Join(dataDir, storeFolder)
	staging := filepath.Join(storePath, stagingFolder)
	stageImage(t, storePath, "stopped-apply", "half")
	if err := os.WriteFile(filepath.Join(staging, "download-partial"), []byte("part"), 0o600); err != nil {
		t.Fatal(err)
	}
	blob := filepath.Join(storePath, blobsFolder, digestFolder(testDigest(t, "12")))
	if err := os.WriteFile(blob, []byte("checked"), 0o600); err != nil {
		t.Fatal(err)
	}

	store := openStore(dataDir, nil)

	left, err := os.ReadDir(staging)
	if err != nil {
		t.Fatalf("staging after the start: %v", err)
	}
	if len(left) != 0 {
		names := make([]string, 0, len(left))
		for _, entry := range left {
			names = append(names, entry.Name())
		}
		t.Errorf("staging holds %v after the start, want nothing", names)
	}
	content, err := os.ReadFile(blob)
	if err != nil {
		t.Fatalf("the checked blob is gone after the start: %v", err)
	}
	if string(content) != "checked" {
		t.Errorf("the checked blob holds %q after the start, want its content unchanged", content)
	}

	inFlight := filepath.Join(staging, "fetch-in-flight")
	if err := os.Mkdir(inFlight, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ListImages(context.Background()); err != nil {
		t.Fatalf("listing: %v", err)
	}
	if _, err := os.Stat(inFlight); err != nil {
		t.Errorf("an operation after the start emptied staging under a fetch in flight: %v", err)
	}
}
