// Design: docs/architecture/container/runtime.md -- the created call, the ended call and the description contribution

package container

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// fakeParticipant records each call in events and answers createdErr at the
// created call.
type fakeParticipant struct {
	name       string
	events     *eventLog
	createdErr error
}

func (p *fakeParticipant) Name() string { return p.name }

func (p *fakeParticipant) Contribute(_ context.Context, instance string) ([]FileMount, error) {
	p.events.add("contribute " + instance)
	return nil, nil
}

func (p *fakeParticipant) Created(_ context.Context, call CreatedCall) error {
	p.events.add("created " + call.Instance + " " + strconv.Itoa(call.InitProcessID) + " " + call.NetworkNamespacePath)
	return p.createdErr
}

func (p *fakeParticipant) Ended(_ context.Context, instance string) {
	p.events.add("ended " + instance)
}

// participantFixture is a supervisor holding one enabled container, web,
// whose image the store holds, with one participant and a recording library.
func participantFixture(t *testing.T, createdErr error) (*supervisor, *eventLog) {
	t.Helper()
	events := &eventLog{}
	store := &fakeStore{}
	store.setAnswer(readyImage())
	library := &fakeLibrary{events: events, initProcess: 4242, networkNamespace: "/proc/4242/ns/net"}
	participant := &fakeParticipant{name: "network", events: events, createdErr: createdErr}
	sup := newSupervisor(slogutil.DiscardLogger(), library, storeFactoryOf(store), []Participant{participant})
	if err := sup.install(configOf(instanceConfig{name: "web", enabled: true})); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	return sup, events
}

// TestCreatedCallRunsBeforeTheRelease proves the wiring of the participant
// seam: a participant contributes before the container is created, gets the
// created call once, with the container's name, its init process id and its
// network namespace path, after the create and before the entrypoint is
// released, and gets the ended call once after the container is destroyed.
//
// Method: one event log shared by a recording library and a recording
// participant, compared with the whole expected order.
func TestCreatedCallRunsBeforeTheRelease(t *testing.T) {
	ctx := context.Background()
	sup, events := participantFixture(t, nil)

	state, err := sup.start(ctx, "web")

	if err != nil {
		t.Fatalf("the start failed: %v", err)
	}
	if state != StateRunning {
		t.Fatalf("the start answered state %s, want %s", state, StateRunning)
	}
	started := []string{
		"contribute web",
		"create web",
		"created web 4242 /proc/4242/ns/net",
		"release web",
	}
	if got := events.all(); !slices.Equal(got, started) {
		t.Fatalf("the start made these calls:\n  %v\nwant:\n  %v", got, started)
	}

	if err := sup.stop(ctx, "web"); err != nil {
		t.Fatalf("the stop failed: %v", err)
	}

	ended := append(slices.Clone(started), "destroy web", "ended web")
	if got := events.all(); !slices.Equal(got, ended) {
		t.Fatalf("the start and the stop made these calls:\n  %v\nwant:\n  %v", got, ended)
	}
}

// TestCreatedCallFailureDestroysTheContainer proves a container without its
// network never runs: a participant that refuses the created call fails the
// start, the entrypoint is never released, the container is destroyed, the
// ended call is still made, and the error names the participant and its
// reason.
func TestCreatedCallFailureDestroysTheContainer(t *testing.T) {
	// The reason does not hold the participant's name, so the name assertion
	// below is satisfied only by the supervisor adding it.
	refusal := errors.New("no address left")
	sup, events := participantFixture(t, refusal)

	_, err := sup.start(context.Background(), "web")

	if !errors.Is(err, refusal) {
		t.Fatalf("the start answered %v, want an error that carries the participant's reason", err)
	}
	if !strings.Contains(err.Error(), "network") {
		t.Errorf("the start's error %q does not name the participant, network", err)
	}
	want := []string{
		"contribute web",
		"create web",
		"created web 4242 /proc/4242/ns/net",
		"destroy web",
		"ended web",
	}
	if got := events.all(); !slices.Equal(got, want) {
		t.Fatalf("the failed start made these calls:\n  %v\nwant:\n  %v", got, want)
	}
}
