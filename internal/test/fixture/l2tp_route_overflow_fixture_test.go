// Design: docs/functional-tests.md -- fixture decisions have unit controls.
// Related: l2tp_route_overflow_fixture.go -- the daemon path is the draft .ci.
package fixture

import (
	"encoding/binary"
	"encoding/json"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRouteOverflowRadiusProfiles decodes the fixture's actual attribute
// bytes with the production wire parser; the negative really carries65, not
// a scenario label or an accidentally truncated64-route packet.
func TestRouteOverflowRadiusProfiles(t *testing.T) {
	for profile := range 2 {
		attributes := routeOverflowAttributes(profile)
		packet := make([]byte, 20+len(attributes))
		packet[0] = radius.CodeAccessAccept
		binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
		copy(packet[20:], attributes)
		decoded, err := radius.Decode(packet)
		if err != nil {
			t.Fatal(err)
		}
		routes := decoded.FindAllAttr(radius.AttrFramedRoute)
		if len(routes) != 64+profile {
			t.Fatalf("profile%d carries %d route attributes", profile, len(routes))
		}
		for index, route := range routes {
			want := routeOverflowPrefix(profile, index).String() + " 0.0.0.0 7"
			if string(route) != want {
				t.Fatalf("profile%d route%d = %q, want %q", profile, index, route, want)
			}
		}
	}
}

// TestRouteOverflowObserverRejectsChangedReplay proves the fixture accepts
// its useful control and refuses missing routes, extra routes and changed
// native source tokens. These parser controls make no daemon-path claim.
func TestRouteOverflowObserverRejectsChangedReplay(t *testing.T) {
	state := new(routeOverflowObservation)
	entries := []redistevents.RouteChangeEntry{{Action: redistevents.ActionAdd, Prefix: routeOverflowPrefix(0, 0), SourceInstance: 41}}
	// The real expected host is distinct from all64 framed prefixes.
	entries[0].Prefix = netip.MustParsePrefix(routeOverflowNeighbor + "/32")
	for index := range 64 {
		entries = append(entries, redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: routeOverflowPrefix(0, index), Metric: 7, SourceInstance: 41})
	}
	routeOverflowReceiveBatch(t, state, 0, entries)
	before, err := state.usefulNeighbor()
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"valid", "missing", "extra", "source"} {
		t.Run(change, func(t *testing.T) {
			observed := &routeOverflowObservation{incremental: slices.Clone(before)}
			replay := slices.Clone(before)
			switch change {
			case "valid":
			case "missing":
				replay = replay[:len(replay)-1]
			case "extra":
				replay = append(replay, replay[0])
			case "source":
				replay[0].SourceInstance++
			}
			routeOverflowReceiveBatch(t, observed, routeOverflowReplay, replay)
			err := observed.unchanged(before, true)
			if change == "valid" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatalf("accepted %s replay", change)
			}
		})
	}
}

func routeOverflowReceiveBatch(t *testing.T, state *routeOverflowObservation, replay uint64, entries []redistevents.RouteChangeEntry) {
	t.Helper()
	payload, err := json.Marshal(redistevents.RouteChangeBatch{Protocol: 1, AFI: 1, SAFI: 1, ReplayID: replay, Entries: entries})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := json.Marshal(rpc.EventEnvelope{Namespace: "l2tp", Event: "route-change", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.receive(string(envelope)); err != nil {
		t.Fatal(err)
	}
}
