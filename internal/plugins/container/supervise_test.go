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

func (f *fakeLibrary) create(_ context.Context, request *startRequest) (createdContainer, error) {
	f.events.add("create " + request.instance.name)
	return &fakeCreated{library: f, name: request.instance.name, exited: make(chan struct{})}, nil
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

// fakeCreated is one container of fakeLibrary. Its init "exits" when it is
// killed: wait blocks until kill closed exited, and answers the code a kill
// gives, 128 plus SIGKILL.
type fakeCreated struct {
	library  *fakeLibrary
	name     string
	exited   chan struct{}
	killOnce sync.Once
}

func (c *fakeCreated) wait() (int, error) {
	<-c.exited
	return 137, nil
}

func (c *fakeCreated) kill() error {
	c.killOnce.Do(func() { close(c.exited) })
	return nil
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

// fakeClaim is a tunable claim that records its calls in events and refuses
// the claim with refusal while it is set.
type fakeClaim struct {
	events  *eventLog
	refusal error
}

func (c *fakeClaim) claim(context.Context) error {
	c.events.add("claim")
	return c.refusal
}

func (c *fakeClaim) release(context.Context) error {
	c.events.add("release-claim")
	return nil
}

// workerCount answers how many workers the supervisor holds.
func workerCount(sup *supervisor) int {
	sup.mu.Lock()
	defer sup.mu.Unlock()
	return len(sup.workers)
}

// TestTunablesAreClaimedBeforeEveryCreate proves the kernel tunable claim: a
// start claims the tunables before the library creates the container, whose
// network namespace the kernel builds under them; a refused claim refuses
// the create and fails the start with the claim's own fix; the claim is
// released once the config holds no enabled container, and only once.
//
// Method: the worker's own steps, converge and releaseTunablesWhenIdle, over
// a library and a claim that record each call in one log.
//
// MUTATION: create before the claim in createWithTunables and the order
// fails; drop the anyEnabled test from releaseTunablesWhenIdle and the claim
// is released while a container is enabled.
func TestTunablesAreClaimedBeforeEveryCreate(t *testing.T) {
	ctx := context.Background()
	events := &eventLog{}
	store := &fakeStore{}
	store.setAnswer(readyImage())
	library := &fakeLibrary{events: events, initProcess: 4242}
	claim := &fakeClaim{events: events}
	sup := newSupervisor(slogutil.DiscardLogger(), library, storeFactoryOf(store), nil)
	sup.tunables = claim
	if err := sup.install(configOf(instanceConfig{name: "web", enabled: true})); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	sup.converge("web")

	if got, want := events.all(), []string{"claim", "create web", "release web"}; !slices.Equal(got, want) {
		t.Fatalf("a start made %v, want %v: the claim comes before the create", got, want)
	}
	sup.releaseTunablesWhenIdle()
	if slices.Contains(events.all(), "release-claim") {
		t.Fatalf("the claim was released while the config enables a container: %v", events.all())
	}
	if err := sup.stop(ctx, "web"); err != nil {
		t.Fatalf("the stop failed: %v", err)
	}

	if err := sup.install(configOf(instanceConfig{name: "web", enabled: false})); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	sup.releaseTunablesWhenIdle()
	sup.releaseTunablesWhenIdle()
	released := 0
	for _, event := range events.all() {
		if event == "release-claim" {
			released++
		}
	}
	if released != 1 {
		t.Fatalf("the claim was released %d times once no container is enabled, want once: %v", released, events.all())
	}

	claim.refusal = noRuntime("db")
	if err := sup.install(configOf(instanceConfig{name: "db", enabled: true})); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	creates := library.creates()
	sup.converge("db")
	if library.creates() != creates {
		t.Fatalf("a refused claim still created the container: %v", events.all())
	}
	if state, _ := sup.state("db"); state != StateFailed {
		t.Fatalf("a refused claim left state %s, want %s", state, StateFailed)
	}
	findings, err := sup.answerDoctorCheck("container-failed")
	if err != nil {
		t.Fatalf("check container-failed refused: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("the refused claim is reported as %v, want one finding", findings)
	}
	if findings[0].Fix != "set container instance db enabled false" {
		t.Errorf("the refused claim's finding has fix %q, want the claim's own", findings[0].Fix)
	}
}

// TestBootConfigureStartsNothingBeforeBegin proves the boot configure only
// installs: a worker's claim is a call to the engine, which refuses it before
// the startup handshake ended, so no worker exists until begin, and begin
// wakes one for each configured container.
//
// MUTATION: drop the started test from apply and the boot configure starts
// workers before begin.
func TestBootConfigureStartsNothingBeforeBegin(t *testing.T) {
	sup := newSupervisor(slogutil.DiscardLogger(), nil, nil, nil)
	t.Cleanup(sup.close)

	if err := sup.apply(configOf(instanceConfig{name: "web", enabled: true})); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if count := workerCount(sup); count != 0 {
		t.Fatalf("the boot configure started %d workers before begin, want 0", count)
	}

	sup.begin()

	if count := workerCount(sup); count != 1 {
		t.Fatalf("begin left %d workers, want one for the configured container", count)
	}
}
