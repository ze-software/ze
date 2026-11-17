// Design: docs/architecture/container/image-store.md -- digest and size checks, proven against the test registry
// Related: blob.go -- copyVerified, the blob check under test
// Related: manifest.go -- manifestOnce, the manifest check under test

package registry

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// TestManifestDigestMismatchRefused proves a manifest whose bytes hash to
// another digest is refused naming both digests: by tag, against the digest
// the registry declares, and by digest, against the digest asked for.
func TestManifestDigestMismatchRefused(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	manifest, _ := putImage(t, r, "app", "1", []byte("layer"))
	tampered := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","layers":[]}`)
	received := digestOf(tampered).String()
	c := newClient(t, Config{Registry: r.Host()})

	for _, tc := range []struct{ name, key, reference string }{
		{name: "by tag", key: "1", reference: r.Host() + "/app:1"},
		{name: "by digest", key: manifest.Digest, reference: r.Host() + "/app@" + manifest.Digest},
	} {
		r.SetFault(tc.key, ociregistry.Fault{Body: tampered})
		_, err := c.Resolve(t.Context(), mustReference(t, tc.reference))
		checkFailure(t, err, FailureDigestMismatch)
		if !strings.Contains(err.Error(), manifest.Digest) || !strings.Contains(err.Error(), received) {
			t.Errorf("%s: error = %v, want both %s and %s named", tc.name, err, manifest.Digest, received)
		}
		r.SetFault(tc.key, ociregistry.Fault{})
	}
	if _, err := c.Resolve(t.Context(), mustReference(t, r.Host()+"/app:1")); err != nil {
		t.Errorf("the untampered manifest was refused: %v", err)
	}
}

// TestBlobDigestMismatchRefused proves a blob of the right size whose bytes
// hash to another digest is refused, and that nothing of it is left in the
// file for the store to read.
func TestBlobDigestMismatchRefused(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	layer := []byte("genuine layer")
	_, layerDescriptor := putImage(t, r, "app", "1", layer)
	forged := bytes.Repeat([]byte("f"), len(layer))
	r.SetFault(layerDescriptor.Digest, ociregistry.Fault{Body: forged})
	c := newClient(t, Config{Registry: r.Host()})

	content, err := fetchToFile(t, t.Context(), c, mustReference(t, r.Host()+"/app:1"), blobOf(t, layerDescriptor))
	checkFailure(t, err, FailureDigestMismatch)
	if !strings.Contains(err.Error(), digestOf(forged).String()) {
		t.Errorf("error = %v, want the received digest named", err)
	}
	if len(content) != 0 {
		t.Errorf("a refused blob left %d bytes in the file", len(content))
	}
}

// TestBlobSizeMismatchRefused proves a blob shorter or longer than its
// declared size is refused, and a longer one is cut before its excess is
// written.
func TestBlobSizeMismatchRefused(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	layer := []byte("sized layer")
	_, layerDescriptor := putImage(t, r, "app", "1", layer)
	c := newClient(t, Config{Registry: r.Host()})
	ref := mustReference(t, r.Host()+"/app:1")

	for _, body := range [][]byte{layer[:len(layer)-1], append(append([]byte(nil), layer...), 'x')} {
		r.SetFault(layerDescriptor.Digest, ociregistry.Fault{Body: body})
		content, err := fetchToFile(t, t.Context(), c, ref, blobOf(t, layerDescriptor))
		checkFailure(t, err, FailureDigestMismatch)
		if len(content) != 0 {
			t.Errorf("a %d-byte answer for a %d-byte blob left %d bytes", len(body), len(layer), len(content))
		}
	}
}

// TestDigestMismatchNotRetried proves a blob with a wrong digest is asked
// for once: another attempt would fetch another file.
func TestDigestMismatchNotRetried(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	layer := []byte("asked once")
	_, layerDescriptor := putImage(t, r, "app", "1", layer)
	r.SetFault(layerDescriptor.Digest, ociregistry.Fault{Body: bytes.Repeat([]byte("w"), len(layer))})
	c := newClient(t, Config{Registry: r.Host()})

	_, err := fetchToFile(t, t.Context(), c, mustReference(t, r.Host()+"/app:1"), blobOf(t, layerDescriptor))
	checkFailure(t, err, FailureDigestMismatch)
	if n := countPath(r, "/v2/app/blobs/"+layerDescriptor.Digest); n != 1 {
		t.Errorf("blob requests = %d, want 1", n)
	}
}

// TestManifestSizeBound proves the manifest bound: 4 MiB is read and one byte
// more is refused before it is parsed.
func TestManifestSizeBound(t *testing.T) {
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	r.PutManifest("app", "at-bound", ociregistry.MediaOCIManifest, bytes.Repeat([]byte(" "), manifestSizeMax))
	r.PutManifest("app", "over-bound", ociregistry.MediaOCIManifest, bytes.Repeat([]byte(" "), manifestSizeMax+1))
	c := newClient(t, Config{Registry: r.Host()})
	ref := mustReference(t, r.Host()+"/app:1")

	answer, err := c.fetchManifest(t.Context(), ref, "at-bound", expected{size: -1})
	if err != nil {
		t.Fatalf("a manifest of 4 MiB was refused: %v", err)
	}
	if len(answer.body) != manifestSizeMax {
		t.Errorf("read %d bytes, want %d", len(answer.body), manifestSizeMax)
	}
	_, err = c.fetchManifest(t.Context(), ref, "over-bound", expected{size: -1})
	checkFailure(t, err, FailureRefused)
}

// TestNonSHA256DigestRefused proves a digest of another algorithm is refused
// naming it, in a reference and in a layer descriptor of a manifest.
func TestNonSHA256DigestRefused(t *testing.T) {
	sha512 := "sha512:" + strings.Repeat("ab", 64)
	if _, err := ParseDigest(sha512); err == nil || !strings.Contains(err.Error(), "sha512") {
		t.Errorf("ParseDigest(%s) = %v, want a refusal naming sha512", sha512, err)
	}

	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	config := r.PutBlob(ociregistry.MediaOCIConfig, []byte(`{"architecture":"amd64","os":"linux"}`))
	body := `{"schemaVersion":2,"mediaType":"` + ociregistry.MediaOCIManifest + `",` +
		`"config":{"mediaType":"` + config.MediaType + `","digest":"` + config.Digest + `","size":` + itoa(config.Size) + `},` +
		`"layers":[{"mediaType":"` + ociregistry.MediaOCILayerGzip + `","digest":"` + sha512 + `","size":3}]}`
	r.PutManifest("app", "1", ociregistry.MediaOCIManifest, []byte(body))
	c := newClient(t, Config{Registry: r.Host()})

	_, err := c.Resolve(t.Context(), mustReference(t, r.Host()+"/app:1"))
	checkFailure(t, err, FailureRefused)
	if !strings.Contains(err.Error(), "sha512") {
		t.Errorf("error = %v, want the algorithm named", err)
	}
}

// itoa writes a size for a hand-built manifest. The sizes of a test are
// small, so the conversion to int loses nothing.
func itoa(n int64) string {
	return strconv.Itoa(int(n))
}
