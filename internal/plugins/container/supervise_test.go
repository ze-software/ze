// Design: docs/architecture/container/runtime.md -- the supervisor: the desired set, container state and the seams it drives

package container

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// eventLog records, in order, what the fakes of this package were asked to
// do, so a test asserts the order of a start and an end. Safe for concurrent
// use.
type eventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *eventLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *eventLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.events)
}

// fakeStore is an image store that answers what the test set. Safe for
// concurrent use.
type fakeStore struct {
	mu       sync.Mutex
	answer   Resolution
	resolves int
}

func (f *fakeStore) setAnswer(answer Resolution) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answer = answer
}

func (f *fakeStore) Resolve(context.Context, ResolveRequest) (Resolution, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolves++
	return f.answer, nil
}

func (f *fakeStore) Hold(string, Ready) func() {
	return func() {}
}

// storeFactoryOf answers the factory that hands the supervisor store.
func storeFactoryOf(store Store) StoreFactory {
	return func(StoreNotices) Store { return store }
}

// fakeLibrary is a runtime library that creates nothing and records each
// step in events.
type fakeLibrary struct {
	events           *eventLog
	initProcess      int
	networkNamespace string
}

func (f *fakeLibrary) create(_ context.Context, request startRequest) (createdContainer, error) {
	f.events.add("create " + request.instance.name)
	return &fakeCreated{library: f, name: request.instance.name}, nil
}

// creates answers how many containers the library was asked to create.
func (f *fakeLibrary) creates() int {
	count := 0
	for _, event := range f.events.all() {
		if len(event) >= len("create ") && event[:len("create ")] == "create " {
			count++
		}
	}
	return count
}

type fakeCreated struct {
	library *fakeLibrary
	name    string
}

func (c *fakeCreated) initProcessID() int           { return c.library.initProcess }
func (c *fakeCreated) networkNamespacePath() string { return c.library.networkNamespace }

func (c *fakeCreated) release() error {
	c.library.events.add("release " + c.name)
	return nil
}

func (c *fakeCreated) destroy() error {
	c.library.events.add("destroy " + c.name)
	return nil
}

// configOf answers a config holding instances as they are, with no rule
// applied: the tests of this file drive the supervisor past the rule set.
func configOf(instances ...instanceConfig) containerConfig {
	return containerConfig{instances: instances}
}

// readyImage is a store answer for an image the store holds.
func readyImage() Ready {
	return Ready{
		Digest:         "sha256:0000000000000000000000000000000000000000000000000000000000000001",
		ImageFolder:    "/store/image",
		WritableFolder: "/store/layer",
	}
}

// TestStoreAbsentThenPresentStartsTheContainer proves the wiring of the image
// store seam, in both directions. The store answers that the image is
// absent: the container takes the waiting-image state, which is no error,
// and nothing is created. The store then reports the image present: the
// container that waited is started and named in the answer.
//
// Method: a store whose answer the test sets, and a library that records
// what it is asked to create.
func TestStoreAbsentThenPresentStartsTheContainer(t *testing.T) {
	const reference = "docker.io/library/redis:7"
	ctx := context.Background()
	events := &eventLog{}
	store := &fakeStore{}
	store.setAnswer(Absent{Reference: reference, Reason: AbsentNeverFetched})
	library := &fakeLibrary{events: events, initProcess: 4242, networkNamespace: "/proc/4242/ns/net"}
	sup := newSupervisor(slogutil.DiscardLogger(), library, storeFactoryOf(store), nil)
	if err := sup.install(configOf(instanceConfig{name: "web", enabled: true})); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	state, err := sup.start(ctx, "web")

	if err != nil {
		t.Fatalf("a start with an absent image answered %v, want the waiting-image state and no error", err)
	}
	if state != StateWaitingImage {
		t.Fatalf("a start with an absent image answered state %s, want %s", state, StateWaitingImage)
	}
	if held, known := sup.state("web"); !known || held != StateWaitingImage {
		t.Errorf("the container holds state %s (known=%v), want %s", held, known, StateWaitingImage)
	}
	if library.creates() != 0 {
		t.Fatalf("the library created %d containers while the image is absent, want 0", library.creates())
	}

	store.setAnswer(readyImage())

	if started := sup.ImagePresent(ctx, "docker.io/library/other:1"); len(started) != 0 {
		t.Fatalf("a notice for another image started %v, want none", started)
	}
	if library.creates() != 0 {
		t.Fatalf("a notice for another image created %d containers, want 0", library.creates())
	}

	started := sup.ImagePresent(ctx, reference)

	if !slices.Equal(started, []string{"web"}) {
		t.Fatalf("the present notice answered %v, want the container that waited: [web]", started)
	}
	if library.creates() != 1 {
		t.Errorf("the library created %d containers after the present notice, want 1", library.creates())
	}
	if held, known := sup.state("web"); !known || held != StateRunning {
		t.Errorf("the container holds state %s (known=%v) after the present notice, want %s", held, known, StateRunning)
	}
}
