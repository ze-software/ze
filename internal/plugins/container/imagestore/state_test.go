// Design: docs/architecture/container/image-store.md -- the binding and the generation record in the state store

//go:build linux

package imagestore

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/internal/plugins/container/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// errFakeEngine is the answer of a fake engine to every call a store makes
// no use of, and of a state write it was told to refuse.
var errFakeEngine = errors.New("the fake engine refuses this call")

// fakeEngine is the engine client a store's test hands it: a map of state
// keys, and a switch that makes it refuse every state write.
type fakeEngine struct {
	mu          sync.Mutex
	state       map[string][]byte
	refusePuts  bool
	putsRefused int
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{state: map[string][]byte{}}
}

// link answers an engine link that holds the fake, for a store's engine.
func (f *fakeEngine) link() *engineLink {
	return &engineLink{client: f}
}

func (f *fakeEngine) OwnedBridgesReplace(context.Context, *rpc.OwnedBridgesReplaceInput) (*rpc.OwnedBridgesReplaceOutput, error) {
	return nil, errFakeEngine
}

func (f *fakeEngine) OwnedLinkCreate(context.Context, *rpc.OwnedLinkCreateInput) (*rpc.OwnedLinkCreateOutput, error) {
	return nil, errFakeEngine
}

func (f *fakeEngine) OwnedLinkRemove(context.Context, *rpc.OwnedLinkRemoveInput) (*rpc.OwnedLinkRemoveOutput, error) {
	return nil, errFakeEngine
}

func (f *fakeEngine) FirewallTablesReplace(context.Context, *rpc.FirewallTablesReplaceInput) (*rpc.FirewallTablesReplaceOutput, error) {
	return nil, errFakeEngine
}

func (f *fakeEngine) StateGet(_ context.Context, key string) ([]byte, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	content, found := f.state[key]
	return content, found, nil
}

func (f *fakeEngine) StatePut(_ context.Context, key string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refusePuts {
		f.putsRefused++
		return errFakeEngine
	}
	f.state[key] = data
	return nil
}

// refuse sets whether the fake refuses every state write.
func (f *fakeEngine) refuse(refused bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refusePuts = refused
}

// keys answers a copy of the state the fake holds.
func (f *fakeEngine) keys() map[string][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return maps.Clone(f.state)
}

// boundDigest answers the digest the fake's binding of text names, failing
// the test when it holds none.
func (f *fakeEngine) boundDigest(t *testing.T, text string) string {
	t.Helper()
	reference, err := registry.ParseReference(text)
	if err != nil {
		t.Fatalf("parse %s: %v", text, err)
	}
	digest, bound, err := readBinding(context.Background(), f, reference)
	if err != nil {
		t.Fatalf("read the binding of %s: %v", text, err)
	}
	if !bound {
		t.Fatalf("no binding of %s, keys %v", text, slices.Collect(maps.Keys(f.keys())))
	}
	return digest.String()
}

// TestBindingKeyUsesReferenceDigest proves a binding key carries one segment
// for a reference with slashes and a colon, the SHA-256 of the whole form,
// and that the short and the whole spelling of one image share it.
//
// Method: compare the keys of three spellings of one image and of another
// image, against the key pattern and against each other.
func TestBindingKeyUsesReferenceDigest(t *testing.T) {
	keyOf := func(text string) string {
		reference, err := registry.ParseReference(text)
		if err != nil {
			t.Fatalf("parse %s: %v", text, err)
		}
		return bindingKey(reference)
	}
	short := keyOf("redis")
	for _, spelling := range []string{"redis:latest", "docker.io/library/redis:latest"} {
		if got := keyOf(spelling); got != short {
			t.Errorf("key of %s is %s, want the key of redis, %s", spelling, got, short)
		}
	}
	segment, found := strings.CutPrefix(keyOf("registry.example:5000/team/app:v1"), "meta/container/binding/")
	if !found {
		t.Fatalf("the key of a registry reference does not start meta/container/binding/")
	}
	if len(segment) != digestHexLength || strings.ContainsAny(segment, "/:") {
		t.Errorf("the key segment %q is not 64 hex digits", segment)
	}
	if keyOf("redis:7") == short {
		t.Errorf("redis:7 and redis:latest share one binding key")
	}
}

// TestBindingRecordRefusedWhenUnreadable proves a binding the store did not
// write is an error, never read as no binding: a record of another
// reference, a digest that does not parse, and a field the store never
// writes.
//
// Method: plant each record under the key of redis and read it back.
func TestBindingRecordRefusedWhenUnreadable(t *testing.T) {
	reference, err := registry.ParseReference("redis")
	if err != nil {
		t.Fatalf("parse redis: %v", err)
	}
	digest := "sha256:" + strings.Repeat("a", digestHexLength)
	records := map[string]string{
		"another reference": `{"reference":"docker.io/library/nginx:latest","digest":"` + digest + `"}`,
		"a bad digest":      `{"reference":"docker.io/library/redis:latest","digest":"sha256:XYZ"}`,
		"an unknown field":  `{"reference":"docker.io/library/redis:latest","digest":"` + digest + `","extra":1}`,
		"not JSON":          `{`,
	}
	for name, record := range records {
		engine := newFakeEngine()
		engine.state[bindingKey(reference)] = []byte(record)
		_, bound, err := readBinding(context.Background(), engine, reference)
		if err == nil {
			t.Errorf("%s: read as bound %v with no error", name, bound)
		}
	}
}

// TestStoreWithoutEngineRefusesResolve proves a store the plugin handed no
// engine client answers an error, never an absent image read from a state
// it could not read.
//
// Method: resolve on a store over a fresh data directory with an empty
// engine link.
func TestStoreWithoutEngineRefusesResolve(t *testing.T) {
	store := openStore(t.TempDir(), nil)
	store.engine = &engineLink{}
	answer, err := store.Resolve(context.Background(), container.ResolveRequest{Instance: "web", Reference: "redis"})
	if err == nil {
		t.Fatalf("resolve with no engine answered %#v and no error", answer)
	}
	if !strings.Contains(err.Error(), "state store") {
		t.Errorf("error %q does not name the state store", err)
	}
}
