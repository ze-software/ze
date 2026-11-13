// Design: docs/architecture/rib/unified-locrib.md -- authenticated reset proof.
// Related: register_route_view_output.go -- refusal before publication decoding.

package sysrib

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A malformed attempt cannot borrow proof established by a previous valid
// refusal, even when an adapter rejects it before the common evaluator runs.
// Another authenticated generation cannot revoke the current owner's proof.
func TestRouteViewPublishMalformedAdapterRevokesResetProof(t *testing.T) {
	for _, adapter := range []string{"bad-json", "bad-batch", "bad-native"} {
		for _, caller := range []string{"owner", "foreign"} {
			t.Run(adapter+"/"+caller, func(t *testing.T) {
				host, owner, input, received := routeViewOutputHost(t)
				if _, err := routeViewPublishNative(host, t.Context(), owner, input); err != nil {
					t.Fatal(err)
				}
				if err := routeViews.withSession(t.Context(), owner, input.Session, func(session *routeViewSession) error {
					return session.invalidate(session.restartRefusal("Controlled journal overflow", "Rebuild the current selected table"))
				}); err != nil {
					t.Fatal(err)
				}
				input.Operation++
				if _, err := routeViewPublishNative(host, t.Context(), owner, input); err == nil {
					t.Fatal("actual fenced publication was acknowledged")
				}
				page := routeViewReadTest(t, &routeViews, owner, input.Session, input.Cursor, 1)
				if got := routeViewRejectedPublication(t, page); got != input.Operation {
					t.Fatalf("valid refused control proved operation %d, want %d", got, input.Operation)
				}
				requestOwner := owner
				if caller == "foreign" {
					requestOwner = routeViewOwner()
				}
				var err error
				switch adapter {
				case "bad-json":
					_, err = routeViewPublishHandler(host, t.Context(), requestOwner, json.RawMessage(`{`))
				case "bad-batch":
					wire, encodeErr := json.Marshal(rpc.RouteViewPublishInput{Session: input.Session, Cursor: input.Cursor, Operation: input.Operation, Batch: json.RawMessage(`{}`)})
					if encodeErr != nil {
						t.Fatal(encodeErr)
					}
					_, err = routeViewPublishHandler(host, t.Context(), requestOwner, wire)
				case "bad-native":
					_, err = routeViewPublishNative(host, t.Context(), requestOwner, struct{}{})
				}
				if err == nil {
					t.Fatal("malformed adapter input was accepted")
				}
				page = routeViewReadTest(t, &routeViews, owner, input.Session, input.Cursor, 1)
				want := uint64(0)
				if caller == "foreign" {
					want = input.Operation
				}
				if got := routeViewRejectedPublication(t, page); got != want {
					t.Fatalf("%s malformed attempt left proof %d, want %d", caller, got, want)
				}
				if got := len(received()); got != 2 {
					t.Fatalf("malformed adapter attempt changed actual output beyond initial add/fence: %d batches", got)
				}
			})
		}
	}
}

// An actual uncertain delivery remains terminal even if a later maintenance
// reset has a benign cause. A new refusal must not rehabilitate that lease.
func TestRouteViewDeliveryFailureCannotBecomeResetProof(t *testing.T) {
	redistevents.RegisterProtocol("static")
	var authority routeViewAuthority
	owner, opened := routeViewOpenTest(t, &authority, locrib.NewRIB())
	page := routeViewReadTest(t, &authority, owner, opened.Session, 0, 1)
	bus := &routeViewOutputFaultBus{testEventBus: newTestEventBus()}
	input := routeViewPublication{Session: opened.Session, Cursor: page.Cursor, Operation: 1, Batch: routeViewOutputBatch()}
	if _, err := authority.publish(t.Context(), owner, input, bus); err != nil {
		t.Fatal(err)
	}
	bus.failPublish = true
	input.Operation++
	if _, err := authority.publish(t.Context(), owner, input, bus); err == nil {
		t.Fatal("actual delivery failure was acknowledged")
	}
	bus.failPublish = false
	if err := authority.withSession(t.Context(), owner, opened.Session, func(session *routeViewSession) error {
		return session.invalidate(session.restartRefusal("Controlled later journal expiry", "Rebuild the selected table"))
	}); err != nil {
		t.Fatal(err)
	}
	input.Operation++
	if _, err := authority.publish(t.Context(), owner, input, bus); err == nil {
		t.Fatal("terminal lease accepted a later publication")
	}
	reset := routeViewReadTest(t, &authority, owner, opened.Session, page.Cursor, 1)
	if got := routeViewRejectedPublication(t, reset); got != 0 {
		t.Fatalf("later benign invalidation rehabilitated delivery failure with proof %d", got)
	}
	if got := len(bus.events); got != 3 {
		t.Fatalf("terminal lease emitted beyond initial add, attempted delivery and scoped fence: %d events", got)
	}
}
