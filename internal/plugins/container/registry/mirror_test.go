// Design: docs/architecture/container/image-store.md -- the mirror, proven against two test registries
// Related: client.go -- fromMirrorThenRegistry, the order under test

package registry

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// TestMirrorTriedFirstThenRegistry proves the order: a mirror that holds the
// image serves all of it and the registry is not asked; a mirror that lacks
// it, or serves a blob that does not match its digest, sends the request to
// the registry and the fetch succeeds, with the mirror's failure kept.
func TestMirrorTriedFirstThenRegistry(t *testing.T) {
	layer := []byte("mirrored layer")
	full := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	_, layerDescriptor := putImage(t, full, "app", "1", layer)

	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	putImage(t, r, "app", "1", layer)
	ref := mustReference(t, r.Host()+"/app:1")
	c := newClient(t, Config{Registry: r.Host(), Mirror: full.Host()})
	if _, err := c.Resolve(t.Context(), ref); err != nil {
		t.Fatal(err)
	}
	if _, err := fetchToFile(t, t.Context(), c, ref, blobOf(t, layerDescriptor)); err != nil {
		t.Fatal(err)
	}
	if n := len(r.Requests()); n != 0 {
		t.Errorf("the registry was asked %d times while the mirror held everything", n)
	}
	if c.LastMirrorFailure() != nil {
		t.Errorf("mirror failure = %v, want none", c.LastMirrorFailure())
	}

	empty := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	fallback := newClient(t, Config{Registry: r.Host(), Mirror: empty.Host()})
	if _, err := fallback.Resolve(t.Context(), ref); err != nil {
		t.Fatalf("the registry was not asked after the mirror failed: %v", err)
	}
	if len(empty.Requests()) == 0 {
		t.Error("the empty mirror was never asked")
	}
	if len(r.Requests()) == 0 {
		t.Error("the registry was never asked after the mirror failed")
	}
	checkFailure(t, fallback.LastMirrorFailure(), FailureNotFound)

	full.SetFault(layerDescriptor.Digest, ociregistry.Fault{Body: bytes.Repeat([]byte("m"), len(layer))})
	before := countPath(r, "/v2/app/blobs/"+layerDescriptor.Digest)
	content, err := fetchToFile(t, t.Context(), c, ref, blobOf(t, layerDescriptor))
	if err != nil {
		t.Fatalf("a mismatched mirror blob was not fetched again from the registry: %v", err)
	}
	if !bytes.Equal(content, layer) {
		t.Errorf("blob = %q, want the registry's %q", content, layer)
	}
	if countPath(r, "/v2/app/blobs/"+layerDescriptor.Digest) != before+1 {
		t.Error("the registry was not asked for the blob the mirror served wrong")
	}
	checkFailure(t, c.LastMirrorFailure(), FailureDigestMismatch)
}

// TestMirrorGetsNoRegistryCredentials proves the mirror never sees the
// registry's credentials, whether it serves the fetch or fails it, while the
// registry receives them when it is asked.
func TestMirrorGetsNoRegistryCredentials(t *testing.T) {
	layer := []byte("guarded layer")
	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthBasic, Username: "operator", Password: "mirror-secret"})
	_, layerDescriptor := putImage(t, r, "app", "1", layer)
	ref := mustReference(t, r.Host()+"/app:1")
	credentials := NewCredentials("operator", "mirror-secret")

	for _, mirrorHolds := range []bool{true, false} {
		m := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
		if mirrorHolds {
			putImage(t, m, "app", "1", layer)
		}
		c := newClient(t, Config{Registry: r.Host(), Mirror: m.Host(), Credentials: credentials})
		if _, err := c.Resolve(t.Context(), ref); err != nil {
			t.Fatal(err)
		}
		if _, err := fetchToFile(t, t.Context(), c, ref, blobOf(t, layerDescriptor)); err != nil {
			t.Fatal(err)
		}
		if len(m.Requests()) == 0 {
			t.Fatal("the mirror was never asked")
		}
		for _, request := range m.Requests() {
			if request.Authorization != "" {
				t.Errorf("mirror holds image %v: the mirror received %q", mirrorHolds, request.Authorization)
			}
		}
	}
	sent := false
	for _, request := range r.Requests() {
		sent = sent || request.Authorization == credentials.basic()
	}
	if !sent {
		t.Error("the registry never received its credentials, so the test proves nothing")
	}
}
