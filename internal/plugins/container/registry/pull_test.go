// Design: docs/architecture/container/image-store.md -- the registry requests, proven against the test registry
// Related: client.go -- the requests under test

package registry

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/fetch"
	"github.com/ze-software/ze/internal/test/ociregistry"
)

// startRegistry starts a test registry, stopped when the test ends.
func startRegistry(t *testing.T, options ociregistry.Options) *ociregistry.Registry {
	t.Helper()
	r, err := ociregistry.Start(options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	return r
}

// newClient builds a client with short retry and stall bounds. An unset
// transport is HTTP and an unset architecture amd64, the test registry's
// defaults.
func newClient(t *testing.T, config Config) *Client {
	t.Helper()
	if config.Transport == TransportUnspecified {
		config.Transport = TransportHTTP
	}
	if config.Architecture == ArchitectureUnspecified {
		config.Architecture = ArchitectureAMD64
	}
	c, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	c.retryDelay = time.Millisecond
	c.stallTimeout = 200 * time.Millisecond
	t.Cleanup(c.Close)
	return c
}

// mustReference parses text or fails the test.
func mustReference(t *testing.T, text string) Reference {
	t.Helper()
	ref, err := ParseReference(text)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

// putImage stores a one-layer amd64 image under repository:tag and answers
// the manifest's descriptor and the layer's.
func putImage(t *testing.T, r *ociregistry.Registry, repository, tag string, layer []byte) (ociregistry.Descriptor, ociregistry.Descriptor) {
	t.Helper()
	manifest, err := r.PutImage(repository, tag, ociregistry.Image{
		Architecture: "amd64",
		Layers:       []ociregistry.Layer{{MediaType: ociregistry.MediaOCILayerGzip, Content: layer}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return manifest, r.PutBlob(ociregistry.MediaOCILayerGzip, layer)
}

// blobOf builds the Blob a descriptor names.
func blobOf(t *testing.T, descriptor ociregistry.Descriptor) Blob {
	t.Helper()
	digest, err := ParseDigest(descriptor.Digest)
	if err != nil {
		t.Fatal(err)
	}
	return Blob{digest: digest, size: descriptor.Size}
}

// fetchToFile fetches blob into a file and answers what the file holds after.
func fetchToFile(t *testing.T, ctx context.Context, c *Client, ref Reference, blob Blob) ([]byte, error) {
	t.Helper()
	file, err := os.Create(filepath.Join(t.TempDir(), "blob"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck // test cleanup
	fetchErr := c.FetchBlob(ctx, ref, blob, file)
	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return content, fetchErr
}

// requireFailure fails the test unless err is an Error of the class want.
func requireFailure(t *testing.T, err error, want Failure) *Error {
	t.Helper()
	var failure *Error
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v, want a registry Error of class %v", err, want)
	}
	if failure.Failure() != want {
		t.Fatalf("error class = %v (%v), want %v", failure.Failure(), err, want)
	}
	return failure
}

// checkFailure fails the test unless err is a registry Error of class want.
// It is requireFailure for a test that reads nothing more from the error.
func checkFailure(t *testing.T, err error, want Failure) {
	t.Helper()
	if requireFailure(t, err, want) == nil {
		t.Fatalf("error = %v, want a registry Error of class %v", err, want)
	}
}

// countPath answers how many requests the registry received for path.
func countPath(r *ociregistry.Registry, path string) int {
	count := 0
	for _, request := range r.Requests() {
		if request.Path == path {
			count++
		}
	}
	return count
}

// TestPullAnonymousManifestAndBlobs proves the plain path: a registry with no
// challenge serves the manifest, the configuration and a layer, and the
// client answers each as fetched, sending no authorization at all.
func TestPullAnonymousManifestAndBlobs(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	layer := []byte("the bytes of one layer")
	manifest, layerDescriptor := putImage(t, r, "team/app", "1.0", layer)
	c := newClient(t, Config{Registry: r.Host()})
	ref := mustReference(t, r.Host()+"/team/app:1.0")

	image, err := c.Resolve(t.Context(), ref)
	if err != nil {
		t.Fatal(err)
	}
	if image.Digest().String() != manifest.Digest {
		t.Errorf("image digest = %s, want the manifest's %s", image.Digest(), manifest.Digest)
	}
	if digestOf(image.Manifest()).String() != manifest.Digest {
		t.Error("the manifest answered is not the manifest stored")
	}
	if !bytes.Contains(image.Config(), []byte(`"architecture":"amd64"`)) {
		t.Errorf("config = %s, want the stored amd64 configuration", image.Config())
	}
	if len(image.Layers()) != 1 {
		t.Fatalf("layers = %d, want 1", len(image.Layers()))
	}
	got := image.Layers()[0]
	if got.Compression() != CompressionGzip || got.Blob().Size() != int64(len(layer)) || got.Blob().Digest().String() != layerDescriptor.Digest {
		t.Errorf("layer = %v %d %s, want gzip %d %s", got.Compression(), got.Blob().Size(), got.Blob().Digest(), len(layer), layerDescriptor.Digest)
	}
	content, err := fetchToFile(t, t.Context(), c, ref, got.Blob())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, layer) {
		t.Errorf("blob = %q, want %q", content, layer)
	}
	for _, request := range r.Requests() {
		if request.Authorization != "" {
			t.Errorf("anonymous request %s carried an authorization", request.Path)
		}
	}
}

// TestRetryOnServerErrorThenSuccess proves a server error is retried and the
// next answer used, and that the retry is bounded: a registry that answers
// 500 to every attempt fails the fetch after three requests.
func TestRetryOnServerErrorThenSuccess(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	layer := []byte("retried layer")
	_, layerDescriptor := putImage(t, r, "app", "1", layer)
	c := newClient(t, Config{Registry: r.Host()})
	ref := mustReference(t, r.Host()+"/app:1")
	path := "/v2/app/blobs/" + layerDescriptor.Digest

	r.SetFault(layerDescriptor.Digest, ociregistry.Fault{ServerErrors: 1})
	content, err := fetchToFile(t, t.Context(), c, ref, blobOf(t, layerDescriptor))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, layer) {
		t.Errorf("blob = %q, want %q", content, layer)
	}
	if n := countPath(r, path); n != 2 {
		t.Errorf("blob requests = %d, want 2: one server error, one answer", n)
	}

	r.SetFault(layerDescriptor.Digest, ociregistry.Fault{ServerErrors: attemptMax})
	_, err = fetchToFile(t, t.Context(), c, ref, blobOf(t, layerDescriptor))
	checkFailure(t, err, FailureUnreachable)
	if n := countPath(r, path); n != 2+attemptMax {
		t.Errorf("blob requests after a persistent server error = %d, want %d", n-2, attemptMax)
	}
}

// TestBlobStallTimesOut proves a transfer that stops sending ends with the
// stall error of the core stall reader, and leaves the file empty.
func TestBlobStallTimesOut(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	layer := bytes.Repeat([]byte("s"), 64)
	_, layerDescriptor := putImage(t, r, "app", "1", layer)
	r.SetFault(layerDescriptor.Digest, ociregistry.Fault{StallAfter: 4})
	c := newClient(t, Config{Registry: r.Host()})

	started := time.Now()
	content, err := fetchToFile(t, t.Context(), c, mustReference(t, r.Host()+"/app:1"), blobOf(t, layerDescriptor))
	checkFailure(t, err, FailureUnreachable)
	if !errors.Is(err, fetch.ErrStalled) {
		t.Errorf("error = %v, want the stall error", err)
	}
	if len(content) != 0 {
		t.Errorf("a stalled blob left %d bytes in the file", len(content))
	}
	if elapsed := time.Since(started); elapsed > 30*time.Second {
		t.Errorf("the stalled fetch took %v", elapsed)
	}
}

// TestFetchCancelStopsTransfer proves canceling the context ends a transfer
// in flight at once, with the context's error and no retry, long before the
// stall timeout would.
func TestFetchCancelStopsTransfer(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	layer := bytes.Repeat([]byte("c"), 64)
	_, layerDescriptor := putImage(t, r, "app", "1", layer)
	r.SetFault(layerDescriptor.Digest, ociregistry.Fault{StallAfter: 4})
	c := newClient(t, Config{Registry: r.Host()})
	c.stallTimeout = time.Minute

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	time.AfterFunc(100*time.Millisecond, cancel)
	started := time.Now()
	_, err := fetchToFile(t, ctx, c, mustReference(t, r.Host()+"/app:1"), blobOf(t, layerDescriptor))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want the cancellation", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Errorf("the canceled fetch took %v", elapsed)
	}
	if n := countPath(r, "/v2/app/blobs/"+layerDescriptor.Digest); n != 1 {
		t.Errorf("blob requests = %d, want 1: a canceled transfer is not retried", n)
	}
	if strings.Contains(err.Error(), "stalled") {
		t.Errorf("error = %v, want the cancellation, not a stall", err)
	}
}
