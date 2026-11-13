// Design: docs/architecture/rib/unified-locrib.md -- publication reset proof.
// Related: route_view_runtime_publication_reset_test.go -- actual worker recovery.

package sysrib

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The proof names a validated batch refused before emission. It is not a
// successful publication and cannot be borrowed by another operation or value.
func TestRouteViewPublicationResetProofRetainsExactRefusal(t *testing.T) {
	redistevents.RegisterProtocol("static")
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	bus := newTestEventBus()
	input := routeViewPublication{Session: opened.Session, Cursor: page.Cursor, Operation: 1, Batch: routeViewOutputBatch()}
	if _, err := authority.publish(t.Context(), owner, input, bus); err != nil {
		t.Fatal(err)
	}
	if err := authority.withSession(t.Context(), owner, opened.Session, func(session *routeViewSession) error {
		return session.invalidate(session.restartRefusal("Controlled selected journal overflow", "Rebuild the current view"))
	}); err != nil {
		t.Fatal(err)
	}
	input.Operation++
	for range 2 {
		if _, err := authority.publish(t.Context(), owner, input, bus); err == nil {
			t.Fatal("fenced publication was acknowledged")
		}
		reset := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 1)
		if reset.Phase != rpc.RouteViewPhaseReset {
			t.Fatalf("proof is not an explicit reset: %+v", reset)
		}
		if got := routeViewRejectedPublication(t, reset); got != input.Operation {
			t.Fatalf("reset proved publication %d, want exact refused operation %d", got, input.Operation)
		}
	}
	changed := input
	changed.Batch = routeViewOutputBatch()
	changed.Batch.Changes[0].Metric++
	if _, err := authority.publish(t.Context(), owner, changed, bus); err == nil {
		t.Fatal("refused identity accepted changed values")
	}
	reset := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 1)
	if got := routeViewRejectedPublication(t, reset); got != 0 {
		t.Fatalf("changed batch borrowed proof for operation %d", got)
	}
	if len(bus.events) != 2 {
		t.Fatalf("refused publications emitted events beyond initial add and fence: %d", len(bus.events))
	}
}

// Full validation and causal eligibility precede proof, including after an
// earlier valid refusal. A delivery failure has already emitted uncertain data.
func TestRouteViewPublicationResetProofCannotBeManufactured(t *testing.T) {
	redistevents.RegisterProtocol("static")
	for _, mode := range []string{"invalid-batch", "invalid-after-proof", "zero-operation", "old-operation", "future-cursor", "next-operation", "delivery", "terminal-reason"} {
		t.Run(mode, func(t *testing.T) {
			var authority routeViewAuthority
			owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
			page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
			bus := &routeViewOutputFaultBus{testEventBus: newTestEventBus()}
			input := routeViewPublication{Session: opened.Session, Cursor: page.Cursor, Operation: 1, Batch: routeViewOutputBatch()}
			if _, err := authority.publish(t.Context(), owner, input, bus); err != nil {
				t.Fatal(err)
			}
			input.Operation++
			if mode == "delivery" {
				bus.failPublish = true
				if _, err := authority.publish(t.Context(), owner, input, bus); err == nil {
					t.Fatal("delivery fault did not refuse actual emission")
				}
				bus.failPublish = false
			} else {
				if err := authority.withSession(t.Context(), owner, opened.Session, func(session *routeViewSession) error {
					reason := session.restartRefusal("Controlled selected journal overflow", "Rebuild the current view")
					if mode == "terminal-reason" {
						reason = errors.New("selected route metadata cannot be inspected")
					}
					return session.invalidate(reason)
				}); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "invalid-after-proof" || mode == "next-operation" {
				if _, err := authority.publish(t.Context(), owner, input, bus); err == nil {
					t.Fatal("fenced publication was acknowledged")
				}
				reset := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 1)
				if routeViewRejectedPublication(t, reset) != input.Operation {
					t.Fatal("valid refused control failed to establish proof")
				}
			}
			switch mode {
			case "invalid-batch", "invalid-after-proof":
				input.Batch.Changes[0].Protocol = "unregistered-reset-proof-protocol"
			case "zero-operation":
				input.Operation = 0
			case "old-operation":
				input.Operation = 1
			case "future-cursor":
				input.Cursor++
			case "next-operation":
				input.Operation++
			case "delivery", "terminal-reason":
			}
			if _, err := authority.publish(t.Context(), owner, input, bus); err == nil {
				t.Fatal("invalid or terminal publication was acknowledged")
			}
			reset, err := authority.read(context.Background(), owner, rpc.RouteViewReadInput{Session: opened.Session, Cursor: page.Cursor, Limit: 1})
			if err != nil {
				return // A terminal refusal grants no reset proof.
			}
			if got := routeViewRejectedPublication(t, reset); got != 0 {
				t.Fatalf("%s manufactured reset proof for operation %d", mode, got)
			}
		})
	}
}

func routeViewRejectedPublication(t *testing.T, page rpc.RouteViewReadOutput) uint64 {
	t.Helper()
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	var proof struct {
		Operation uint64 `json:"rejected-publication"`
	}
	if err := json.Unmarshal(encoded, &proof); err != nil {
		t.Fatal(err)
	}
	return proof.Operation
}
