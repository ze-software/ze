// Design: docs/architecture/rib/unified-locrib.md -- bounded replay requests.
// Related: route_view_worker.go -- arrival revision admission and retirement.

package sysrib

import (
	"encoding/json"
	"math"
	"testing"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestRouteViewReplayRevisionRefusesWrapWithoutMutation(t *testing.T) {
	client := &routeViewClient{replays: map[uint64]uint64{replay.Broadcast: math.MaxUint64}, wake: make(chan struct{}, 1)}
	if err := client.HandleEvent(routeViewReplayEvent(t, replay.Broadcast)); err == nil {
		t.Fatal("exhausted arrival revision wrapped")
	}
	if got := client.replays[replay.Broadcast]; got != math.MaxUint64 {
		t.Fatalf("refused request changed retained revision to %d", got)
	}
	if len(client.replays) != 1 {
		t.Fatal("refused request changed retained identities")
	}
	if len(client.wake) != 0 {
		t.Fatal("refused request queued new work")
	}
}

func TestRouteViewReplayRevisionPreservesQueueBound(t *testing.T) {
	client := &routeViewClient{replays: make(map[uint64]uint64), wake: make(chan struct{}, 1)}
	if err := client.HandleEvent(routeViewReplayEvent(t, replay.Broadcast)); err != nil {
		t.Fatal(err)
	}
	for identity := uint64(1); identity <= rpc.RouteViewRecordsMax; identity++ {
		if err := client.HandleEvent(routeViewReplayEvent(t, identity)); err == nil {
			t.Fatalf("targeted identity %d entered the broadcast replay queue", identity)
		}
		if err := client.HandleEvent(routeViewReplayEvent(t, replay.Broadcast)); err != nil {
			t.Fatalf("a retained broadcast could not record its newer arrival: %v", err)
		}
	}
	want := uint64(rpc.RouteViewRecordsMax + 1)
	if got := client.replays[replay.Broadcast]; got != want {
		t.Fatalf("duplicate broadcast revision %d, want %d", got, want)
	}
	if err := client.HandleEvent(routeViewReplayEvent(t, rpc.RouteViewRecordsMax+1)); err == nil {
		t.Fatal("another targeted identity entered the broadcast replay queue")
	}
	if len(client.replays) != 1 {
		t.Fatalf("broadcast queue retained %d identities, want 1", len(client.replays))
	}
	if got := client.replays[replay.Broadcast]; got != want {
		t.Fatalf("refused identity damaged duplicate broadcast revision %d", got)
	}
	if len(client.wake) != 1 {
		t.Fatalf("duplicate broadcasts retained %d wake notifications, want 1", len(client.wake))
	}
}

func routeViewReplayEvent(t *testing.T, identity uint64) string {
	t.Helper()
	payload, err := json.Marshal(replay.Request{ReplayID: identity})
	if err != nil {
		t.Fatal(err)
	}
	event, err := json.Marshal(rpc.EventEnvelope{Namespace: sysribevents.Namespace, Event: sysribevents.EventReplayRequest, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	return string(event)
}
