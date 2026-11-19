// Design: docs/architecture/container/image-store.md -- the mirror, proven against two test registries
// Related: client.go -- fromMirrorThenRegistry, the order under test

package registry

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// TestMirrorPathPrefixesEveryMirrorRequest proves a mirror's path is placed
// before the /v2/ path of every request to the mirror, with or without a
// trailing slash, and never before a request to the registry.
//
// Method: a server answers the test registry's content under /cache only,
// and records the path of each request it receives. The registry itself
// holds the image too, at its root. With the mirror path /cache/ the whole
// fetch is served by the mirror, and every path it saw starts with
// /cache/v2/. With no mirror path the mirror answers nothing, and the
// registry serves the fetch at its unprefixed paths.
func TestMirrorPathPrefixesEveryMirrorRequest(t *testing.T) {
	layer := []byte("layer behind a path")
	content := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	_, layerDescriptor := putImage(t, content, "app", "1", layer)
	var mu sync.Mutex
	var paths []string
	under := http.StripPrefix("/cache", content)
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		mu.Lock()
		paths = append(paths, request.URL.Path)
		mu.Unlock()
		under.ServeHTTP(w, request)
	}))
	t.Cleanup(mirror.Close)
	mirrorHost := strings.TrimPrefix(mirror.URL, "http://")

	r := startRegistry(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	putImage(t, r, "app", "1", layer)
	ref := mustReference(t, r.Host()+"/app:1")

	c := newClient(t, Config{Registry: r.Host(), Mirror: mirrorHost, MirrorPath: "/cache/"})
	if _, err := c.Resolve(t.Context(), ref); err != nil {
		t.Fatal(err)
	}
	if _, err := fetchToFile(t, t.Context(), c, ref, blobOf(t, layerDescriptor)); err != nil {
		t.Fatal(err)
	}
	if n := len(r.Requests()); n != 0 {
		t.Errorf("the registry was asked %d times while the mirror held everything under its path", n)
	}
	mu.Lock()
	seen := len(paths)
	for _, path := range paths {
		if !strings.HasPrefix(path, "/cache/v2/") {
			t.Errorf("the mirror was asked for %q, want a path under /cache/v2/", path)
		}
	}
	mu.Unlock()
	if seen == 0 {
		t.Fatal("the mirror was never asked")
	}

	unprefixed := newClient(t, Config{Registry: r.Host(), Mirror: mirrorHost})
	if _, err := unprefixed.Resolve(t.Context(), ref); err != nil {
		t.Fatalf("the registry was not asked after the mirror failed: %v", err)
	}
	for _, request := range r.Requests() {
		if !strings.HasPrefix(request.Path, "/v2/") {
			t.Errorf("the registry was asked for %q, want an unprefixed /v2/ path", request.Path)
		}
	}
	if len(r.Requests()) == 0 {
		t.Error("the registry was never asked when the mirror path was not set")
	}
}

// TestMirrorPathRule covers what New refuses of a mirror path: one that does
// not start with a slash, one that starts with two slashes, one that holds a
// backslash, one that holds a query or a fragment, and one set with no
// mirror. A path of one slash is the mirror's root.
func TestMirrorPathRule(t *testing.T) {
	const mirror = "mirror.example:5000"
	refused := map[string]Config{
		"no leading slash": {Mirror: mirror, MirrorPath: "cache"},
		"two slashes":      {Mirror: mirror, MirrorPath: "//evil.example/cache"},
		"a backslash":      {Mirror: mirror, MirrorPath: `/\evil.example/cache`},
		"a query":          {Mirror: mirror, MirrorPath: "/cache?x=1"},
		"a fragment":       {Mirror: mirror, MirrorPath: "/cache#top"},
		"no mirror":        {MirrorPath: "/cache"},
	}
	for name, config := range refused {
		config.Registry = "registry.example"
		config.Transport = TransportHTTPS
		config.Architecture = ArchitectureAMD64
		c, err := New(config)
		if err == nil {
			c.Close()
			t.Errorf("%s: New accepted the mirror path %q", name, config.MirrorPath)
		}
	}
	root := newClient(t, Config{Registry: "registry.example", Mirror: mirror, MirrorPath: "/"})
	if root.mirror.prefix != "" {
		t.Errorf("a mirror path of one slash gives the prefix %q, want none", root.mirror.prefix)
	}
}

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
