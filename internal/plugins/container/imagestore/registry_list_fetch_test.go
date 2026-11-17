//go:build linux

// Design: docs/architecture/container/image-store.md -- the registry list, applied: a fetch reaches the registry as its entry says

package imagestore

import (
	"context"
	"encoding/base64"
	"net"
	"testing"

	"github.com/ze-software/ze/internal/test/ociregistry"
)

// applyRegistries makes the registry list of entries, the JSON object of the
// list, the running one, through the parse and the apply the config
// transaction calls, and restores the list with no entry when the test ends.
// A test that calls it MUST NOT run in parallel with another: the running
// list is the process's.
func applyRegistries(t *testing.T, entries string) {
	t.Helper()
	list := mustParseRegistries(t, registryBody(entries))
	if err := appliedRegistries.apply(context.Background(), list); err != nil {
		t.Fatalf("apply the registry list: %v", err)
	}
	t.Cleanup(func() {
		if err := appliedRegistries.apply(context.Background(), registryList{}); err != nil {
			t.Errorf("restore the registry list: %v", err)
		}
	})
}

// newConfiguredFetchFixture is newFetchFixture whose store reaches each
// registry as the running registry list says, the way the plugin's store
// does: nothing of the fixture's own client configuration remains.
func newConfiguredFetchFixture(t *testing.T, options ociregistry.Options) fetchFixture {
	t.Helper()
	f := newFetchFixture(t, options)
	f.store.clientConfig = engineClientConfig
	return f
}

// mirrorStatements answers the mirror node, as delivered JSON, that names
// the test registry server by its loopback address and port.
func mirrorStatements(t *testing.T, server *ociregistry.Registry) string {
	t.Helper()
	address, port, err := net.SplitHostPort(server.Host())
	if err != nil {
		t.Fatalf("the test registry's host %q: %v", server.Host(), err)
	}
	return `"mirror":{"address":"` + address + `","port":"` + port + `"}`
}

// TestRegistryEntryMirrorIsAskedFirstOverHTTP proves the configuration alone
// sends a fetch of the hub's short name to a mirror on a loopback address
// and port, over plain HTTP, and that a changed list reaches the next fetch
// of the same store.
//
// Method: two test registries each serve library/redis:latest, with
// different content. The store's client configuration is the plugin's own,
// which reads the running list. With an entry for docker.io whose mirror is
// the first registry, redis is fetched from it. The entry is then replaced
// by one whose mirror is the second, and the same store fetches the second
// registry's image. Without an entry the fetch would dial the public hub
// over TLS.
func TestRegistryEntryMirrorIsAskedFirstOverHTTP(t *testing.T) {
	const whole = "docker.io/library/redis:latest"
	first := newConfiguredFetchFixture(t, ociregistry.Options{Auth: ociregistry.AuthBearer})
	image := first.put(t, "library/redis", "latest", "redis from the first mirror\n")
	second := newFetchFixture(t, ociregistry.Options{Auth: ociregistry.AuthNone})
	moved := second.put(t, "library/redis", "latest", "redis from the second mirror\n")
	if moved.digest == image.digest {
		t.Fatalf("the two mirrors serve the same digest %s", image.digest)
	}

	applyRegistries(t, `{"docker.io":{"transport":"http",`+mirrorStatements(t, first.registry)+`}}`)
	answer, err := first.store.FetchImage(first.ctx, "redis")
	if err != nil {
		t.Fatalf("fetch redis through the configured mirror: %v", err)
	}
	if answer.Reference != whole || answer.Digest != image.digest {
		t.Fatalf("answer %+v, want %s at %s", answer, whole, image.digest)
	}
	if len(first.registry.Requests()) == 0 {
		t.Fatal("the configured mirror was never asked")
	}
	if asked := len(second.registry.Requests()); asked != 0 {
		t.Fatalf("the registry of no entry was asked %d times", asked)
	}

	applyRegistries(t, `{"docker.io":{"transport":"http",`+mirrorStatements(t, second.registry)+`}}`)
	again, err := first.store.FetchImage(first.ctx, "redis")
	if err != nil {
		t.Fatalf("fetch redis after the mirror changed: %v", err)
	}
	if !again.Created || again.Digest != moved.digest {
		t.Errorf("after the list changed the fetch answered %+v, want %s, created", again, moved.digest)
	}
	if len(second.registry.Requests()) == 0 {
		t.Error("the mirror of the changed list was never asked")
	}
}

// TestRegistryCredentialsReachTheRegistryNeverItsMirror proves the
// authentication node's credentials are sent to the registry and never to
// its mirror, through the store's own client configuration.
//
// Method: the registry demands basic credentials and holds the image; the
// mirror is anonymous and holds nothing, so every request goes to the mirror
// first and then to the registry. The entry is named by the registry's host
// and port, as the reference writes it. The fetch succeeds, the mirror saw
// no authorization header, and the registry saw the configured credentials.
// Without the entry's credentials the registry refuses the fetch.
func TestRegistryCredentialsReachTheRegistryNeverItsMirror(t *testing.T) {
	const username, password = "operator", "registry-secret"
	guarded := newConfiguredFetchFixture(t, ociregistry.Options{Auth: ociregistry.AuthBasic, Username: username, Password: password})
	image := guarded.put(t, "app", "1", "guarded\n")
	mirror := newFetchFixture(t, ociregistry.Options{Auth: ociregistry.AuthNone})

	applyRegistries(t, `{"`+guarded.registry.Host()+`":{"transport":"http",`+mirrorStatements(t, mirror.registry)+
		`,"authentication":{"username":"`+username+`","password":"`+password+`"}}}`)
	answer, err := guarded.store.FetchImage(guarded.ctx, image.reference)
	if err != nil {
		t.Fatalf("fetch %s with the configured credentials: %v", image.reference, err)
	}
	if answer.Digest != image.digest {
		t.Fatalf("answer %+v, want %s", answer, image.digest)
	}

	asked := mirror.registry.Requests()
	if len(asked) == 0 {
		t.Fatal("the mirror was never asked, so the test proves nothing about it")
	}
	for _, request := range asked {
		if request.Authorization != "" {
			t.Errorf("the mirror received %q for %s", request.Authorization, request.Path)
		}
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	sent := false
	for _, request := range guarded.registry.Requests() {
		if request.Authorization == want {
			sent = true
		}
	}
	if !sent {
		t.Error("the registry never received the configured credentials")
	}
}
