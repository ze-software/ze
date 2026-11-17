// Design: docs/functional-tests.md -- the seed of the published image the container tests fetch
// Related: seed.go -- the seed under test

package testcontainerseed

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// testRepository is the repository the upstream registry of a test serves.
const testRepository = "library/redis"

// upstream is the registry a test seeds from, standing for the public one: an
// index, pinned by digest, of one image per architecture.
type upstream struct {
	registry *ociregistry.Registry
	index    ociregistry.Descriptor
	// manifests are the index entries by architecture.
	manifests map[string]ociregistry.Descriptor
}

// layerOf answers the bytes of the one layer of an architecture's image.
func layerOf(architecture string) []byte {
	return []byte("the layer of " + architecture)
}

// digestOfBytes answers the sha256:<hex> digest of content.
func digestOfBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return digestAlgorithm + hex.EncodeToString(sum[:])
}

// startUpstream starts a registry with the hub's anonymous bearer challenge
// that serves, by digest only, an index of one image per architecture.
func startUpstream(t *testing.T, architectures ...string) upstream {
	t.Helper()
	registry, err := ociregistry.Start(ociregistry.Options{Auth: ociregistry.AuthBearer})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(registry.Close)
	source := upstream{registry: registry, manifests: map[string]ociregistry.Descriptor{}}
	entries := make([]ociregistry.Descriptor, 0, len(architectures))
	for _, architecture := range architectures {
		entry, err := registry.PutImage(testRepository, "", ociregistry.Image{
			Architecture: architecture,
			Layers:       []ociregistry.Layer{{MediaType: ociregistry.MediaOCILayerTar, Content: layerOf(architecture)}},
		})
		if err != nil {
			t.Fatal(err)
		}
		source.manifests[architecture] = entry
		entries = append(entries, entry)
	}
	source.index, err = registry.PutIndex(testRepository, "", entries)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

// pin answers the pin of the upstream's index, reached over the registry's
// own address.
func (u upstream) pin() pin {
	return pin{endpoint: u.registry.URL(), registry: hubName, repository: testRepository, digest: u.index.Digest}
}

// blobRequests answers how many of the upstream's requests since the first
// skip asked for a blob.
func (u upstream) blobRequests(skip int) []string {
	var paths []string
	for _, request := range u.registry.Requests()[skip:] {
		if strings.Contains(request.Path, "/blobs/") {
			paths = append(paths, request.Path)
		}
	}
	return paths
}

// layoutFile answers the file of digest's blob in the layout at dir.
func layoutFile(t *testing.T, dir, digest string) string {
	t.Helper()
	name, err := ociregistry.LayoutBlobPath(digest)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, name)
}

// requireAbsent fails the test unless nothing is at path.
func requireAbsent(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s: want nothing there, stat answered %v", path, err)
	}
}

// TestSeedCopiesPinnedPlatforms proves the seed writes a layout that holds
// the pinned index under the latest tag with the linux/amd64 and linux/arm64
// images whole, and no other entry of the index.
//
// Method: seed from an upstream whose index names a third architecture, load
// the folder into a second registry with the loader the fixture uses, and
// read the folder for what must and must not be there.
func TestSeedCopiesPinnedPlatforms(t *testing.T) {
	source := startUpstream(t, "amd64", "arm64", "riscv64")
	dir := filepath.Join(t.TempDir(), "layout")
	var out bytes.Buffer
	if err := seed(t.Context(), source.pin(), dir, &out); err != nil {
		t.Fatal(err)
	}

	served, err := ociregistry.Start(ociregistry.Options{Auth: ociregistry.AuthNone})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(served.Close)
	descriptor, platforms, err := served.PutLayout(testRepository, seedTag, dir)
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.Digest != source.index.Digest {
		t.Fatalf("the layout records the index %s, want the pinned %s", descriptor.Digest, source.index.Digest)
	}
	if len(platforms) != 2 {
		t.Fatalf("the layout holds the platforms %v, want amd64 and arm64", platforms)
	}
	for i, architecture := range seedArchitectures {
		if platforms[i].Architecture != architecture {
			t.Fatalf("platform %d of the layout is %v, want %s", i, platforms[i], architecture)
		}
		layer, err := os.ReadFile(layoutFile(t, dir, digestOfBytes(layerOf(architecture))))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(layer, layerOf(architecture)) {
			t.Fatalf("the %s layer of the layout is %q", architecture, layer)
		}
	}
	requireAbsent(t, layoutFile(t, dir, source.manifests["riscv64"].Digest))
	requireAbsent(t, layoutFile(t, dir, digestOfBytes(layerOf("riscv64"))))

	listing, err := os.ReadFile(filepath.Join(dir, ociregistry.LayoutIndexFile))
	if err != nil {
		t.Fatal(err)
	}
	if want := `"` + annotationImageName + `":"docker.io/library/redis:latest"`; !strings.Contains(string(listing), want) {
		t.Fatalf("index.json is %s, want it to hold %s", listing, want)
	}
	marker, err := os.ReadFile(filepath.Join(dir, ociregistry.LayoutMarkerFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(marker) != ociregistry.LayoutMarker {
		t.Fatalf("the layout marker is %q", marker)
	}
	partial, err := filepath.Glob(filepath.Join(filepath.Dir(layoutFile(t, dir, source.index.Digest)), "*"+partialSuffix))
	if err != nil {
		t.Fatal(err)
	}
	if len(partial) != 0 {
		t.Fatalf("the seed left partial files: %v", partial)
	}
	if !strings.Contains(out.String(), "container-seed: layout "+dir) {
		t.Fatalf("the report does not name the folder: %s", out.String())
	}
	if !strings.Contains(out.String(), "7 fetched, 0 already there") {
		t.Fatalf("the report does not count the index, two manifests, two configurations and two layers as fetched: %s", out.String())
	}
}

// TestSeedFetchesNothingAlreadyThere proves the seed is resumable: an object
// the layout holds, with the right digest, is not asked for again, and one
// that went missing is the only one fetched.
//
// Method: seed twice and count the upstream's requests of the second run,
// then remove one layer file, seed a third time and read which blob was asked.
func TestSeedFetchesNothingAlreadyThere(t *testing.T) {
	source := startUpstream(t, "amd64", "arm64")
	dir := filepath.Join(t.TempDir(), "layout")
	var first bytes.Buffer
	if err := seed(t.Context(), source.pin(), dir, &first); err != nil {
		t.Fatal(err)
	}

	before := len(source.registry.Requests())
	var second bytes.Buffer
	if err := seed(t.Context(), source.pin(), dir, &second); err != nil {
		t.Fatal(err)
	}
	if after := len(source.registry.Requests()); after != before {
		t.Fatalf("the second run sent %d requests, want none: %v", after-before, source.registry.Requests()[before:])
	}
	if !strings.Contains(second.String(), "0 fetched, 7 already there") {
		t.Fatalf("the second run reports: %s", second.String())
	}

	layer := digestOfBytes(layerOf("arm64"))
	if err := os.Remove(layoutFile(t, dir, layer)); err != nil {
		t.Fatal(err)
	}
	var third bytes.Buffer
	if err := seed(t.Context(), source.pin(), dir, &third); err != nil {
		t.Fatal(err)
	}
	asked := source.blobRequests(before)
	if len(asked) == 0 {
		t.Fatal("the third run asked for no blob, and one layer was missing")
	}
	for _, path := range asked {
		if !strings.HasSuffix(path, "/blobs/"+layer) {
			t.Fatalf("the third run asked for %s, want only the missing layer %s", path, layer)
		}
	}
	restored, err := os.ReadFile(layoutFile(t, dir, layer))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, layerOf("arm64")) {
		t.Fatalf("the restored layer is %q", restored)
	}
	if !strings.Contains(third.String(), "1 fetched, 6 already there") {
		t.Fatalf("the third run reports: %s", third.String())
	}
}

// TestSeedRefusesWrongBytes proves a blob that is not what its descriptor
// names, by digest or by size, stops the seed, stays out of the layout, and
// leaves no index.json, so the folder is never taken for a whole seed.
//
// Method: make the upstream answer other bytes for one layer, seed, and read
// the error and the folder.
func TestSeedRefusesWrongBytes(t *testing.T) {
	honest := layerOf("amd64")
	cases := []struct {
		name string
		body []byte
		want string
	}{
		{name: "other bytes of the same length", body: bytes.Repeat([]byte{'x'}, len(honest)), want: "hashes to"},
		{name: "one byte short", body: honest[:len(honest)-1], want: "does not hold"},
		{name: "one byte long", body: append(append([]byte{}, honest...), 'x'), want: "does not hold"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := startUpstream(t, "amd64", "arm64")
			layer := digestOfBytes(honest)
			source.registry.SetFault(layer, ociregistry.Fault{Body: c.body})
			dir := filepath.Join(t.TempDir(), "layout")
			var out bytes.Buffer
			err := seed(t.Context(), source.pin(), dir, &out)
			if err == nil {
				t.Fatal("the seed accepted a layer that is not the one its descriptor names")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("the seed answered %q, want it to hold %q", err, c.want)
			}
			if !strings.Contains(err.Error(), layer) {
				t.Fatalf("the seed answered %q, want it to name the layer %s", err, layer)
			}
			requireAbsent(t, layoutFile(t, dir, layer))
			requireAbsent(t, layoutFile(t, dir, layer)+partialSuffix)
			requireAbsent(t, filepath.Join(dir, ociregistry.LayoutIndexFile))
		})
	}
}

// TestSeedRefusesIndexOfOtherDigest proves the index is held to the pin: bytes
// that do not hash to the pinned digest are refused and not stored.
//
// Method: make the upstream answer another body for the pinned digest.
func TestSeedRefusesIndexOfOtherDigest(t *testing.T) {
	source := startUpstream(t, "amd64", "arm64")
	source.registry.SetFault(source.index.Digest, ociregistry.Fault{Body: []byte(`{"manifests":[]}`)})
	dir := filepath.Join(t.TempDir(), "layout")
	var out bytes.Buffer
	err := seed(t.Context(), source.pin(), dir, &out)
	if err == nil {
		t.Fatal("the seed accepted an index that does not hash to the pin")
	}
	if !strings.Contains(err.Error(), "hashes to") {
		t.Fatalf("the seed answered %q, want a digest refusal", err)
	}
	requireAbsent(t, layoutFile(t, dir, source.index.Digest))
	requireAbsent(t, filepath.Join(dir, ociregistry.LayoutIndexFile))
}

// TestSeedRefusesIndexWithoutSeedPlatform proves an index that lacks one of
// the two architectures is refused, so a seed never holds half of what the
// tests run on.
//
// Method: seed from an upstream whose index names amd64 only.
func TestSeedRefusesIndexWithoutSeedPlatform(t *testing.T) {
	source := startUpstream(t, "amd64")
	dir := filepath.Join(t.TempDir(), "layout")
	var out bytes.Buffer
	err := seed(t.Context(), source.pin(), dir, &out)
	if err == nil {
		t.Fatal("the seed accepted an index with no arm64 image")
	}
	if !strings.Contains(err.Error(), "names no linux/arm64 image") {
		t.Fatalf("the seed answered %q, want it to name the missing platform", err)
	}
	requireAbsent(t, filepath.Join(dir, ociregistry.LayoutIndexFile))
}

// TestParsePinReadsTheTrackedPin proves the pin the test tree tracks is in the
// form the seed reads, and names the hub's registry endpoint.
//
// Method: parse test/interop-container/redis.pin of this checkout.
func TestParsePinReadsTheTrackedPin(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "test", "interop-container", "redis.pin"))
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := parsePin(string(text))
	if err != nil {
		t.Fatal(err)
	}
	if pinned.endpoint != hubEndpoint {
		t.Fatalf("endpoint %s, want %s", pinned.endpoint, hubEndpoint)
	}
	if pinned.registry != hubName {
		t.Fatalf("registry %s, want %s", pinned.registry, hubName)
	}
	if pinned.repository != testRepository {
		t.Fatalf("repository %s, want %s", pinned.repository, testRepository)
	}
	if !strings.HasPrefix(pinned.digest, digestAlgorithm) {
		t.Fatalf("digest %s", pinned.digest)
	}
}

// TestParsePinRefusesOtherForms proves a pin file that is not the one line
// the seed reads is refused, and that another registry keeps its own name as
// its endpoint.
//
// Method: parse each malformed text, then one line naming another registry.
func TestParsePinRefusesOtherForms(t *testing.T) {
	digest := digestAlgorithm + strings.Repeat("a", 64)
	for _, text := range []string{
		"",
		"docker.io/library/redis@" + digest,
		"docker.io/library/redis@" + digest + "\n\n",
		"docker.io/library/redis:8\n",
		"docker.io/library/redis@sha256:abc\n",
		"redis@" + digest + "\n",
		"/library/redis@" + digest + "\n",
		"docker.io/@" + digest + "\n",
		"docker.io/library/redis@" + digest + " comment\n",
	} {
		if got, err := parsePin(text); err == nil {
			t.Fatalf("parsePin(%q) answered %+v, want a refusal", text, got)
		}
	}
	pinned, err := parsePin("registry.example:5000/team/image@" + digest + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if pinned.endpoint != "https://registry.example:5000" {
		t.Fatalf("endpoint %s", pinned.endpoint)
	}
	if pinned.repository != "team/image" {
		t.Fatalf("repository %s", pinned.repository)
	}
}

// TestSeedCommandIsTheRegisteredOne proves the line a missing seed is
// reported with is the command this package registers.
//
// Method: compare the text the registry fixture prints with the registered
// name and its one word.
func TestSeedCommandIsTheRegisteredOne(t *testing.T) {
	if want := "./le " + name + " " + wordFetch; ociregistry.SeedCommand != want {
		t.Fatalf("ociregistry.SeedCommand is %q, and the command is %q", ociregistry.SeedCommand, want)
	}
}

// TestRunRefusesAnythingButFetch proves the command starts no transfer for a
// word it does not know or for no word, and answers the usage for a help
// word.
//
// Method: run each command line that must not reach the network.
func TestRunRefusesAnythingButFetch(t *testing.T) {
	if code := run(nil); code != 2 {
		t.Fatalf("no word: exit %d, want 2", code)
	}
	if code := run([]string{"other"}); code != 2 {
		t.Fatalf("an unknown word: exit %d, want 2", code)
	}
	if code := run([]string{wordFetch, "again"}); code != 2 {
		t.Fatalf("a second word: exit %d, want 2", code)
	}
	if code := run([]string{"help"}); code != 0 {
		t.Fatalf("help: exit %d, want 0", code)
	}
}
