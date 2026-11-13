// Design: docs/guide/redistribution.md -- live connected removal.
// Related: bootstrap_test.go -- real-runner clean removal and ingress barriers.

package connected

import (
	"maps"
	"net/netip"
	"reflect"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"
	connectedevents "github.com/ze-software/ze/internal/plugins/connected/events"
)

// This supplements the real-runner barriers with deterministic calls after
// retirement: SDK shutdown can reject late deliveries before these guards run.
func TestConnectedRetirementFencesLaterOperations(t *testing.T) {
	for _, operation := range []string{"lateAdd", "lateRemove", "replay", "secondretire"} {
		t.Run(operation, func(t *testing.T) {
			observer := newRouteObserver(nil)
			var published []redistevents.RouteChangeBatch
			observer.publish = func(batch *redistevents.RouteChangeBatch) (int, error) {
				value := *batch
				value.Entries = slices.Clone(batch.Entries)
				published = append(published, value)
				return 1, nil
			}
			owned := netip.MustParsePrefix("192.0.2.0/24")
			canary := netip.MustParsePrefix("203.0.113.0/24")
			observer.handleAddrAdded(makePayload("192.0.2.1", 24))
			observer.handleAddrAdded(makePayload("203.0.113.1", 24))
			if len(published) != 2 {
				t.Fatalf("initial publications=%d, want useful owned and canary Adds", len(published))
			}
			for index, prefix := range []netip.Prefix{owned, canary} {
				batch := published[index]
				if batch.Protocol != connectedevents.ProtocolID || batch.ReplayID != 0 || len(batch.Entries) != 1 || batch.Entries[0].Action != redistevents.ActionAdd || batch.Entries[0].Prefix != prefix {
					t.Fatalf("initial publication %d=%+v, want connected Add for %s", index, batch, prefix)
				}
				if len(observer.prefixes[prefix]) != 1 {
					t.Fatalf("initial inventory for %s=%+v, want one covering address", prefix, observer.prefixes[prefix])
				}
			}
			inventory := make(map[netip.Prefix]map[connectedAddr]struct{}, len(observer.prefixes))
			for prefix, addresses := range observer.prefixes {
				inventory[prefix] = maps.Clone(addresses)
			}
			if err := observer.retire(); err != nil {
				t.Fatalf("retire useful observer: %v", err)
			}
			if !observer.retired {
				t.Fatal("successful retirement did not fence the observer")
			}
			if len(published) != 4 {
				t.Fatalf("retirement publications=%d, want two Adds and two Removes", len(published))
			}
			removed := make(map[netip.Prefix]int)
			for _, batch := range published[2:] {
				if batch.Protocol != connectedevents.ProtocolID || batch.ReplayID != 0 || len(batch.Entries) != 1 || batch.Entries[0].Action != redistevents.ActionRemove {
					t.Fatalf("retirement publication=%+v, want incremental connected Remove", batch)
				}
				removed[batch.Entries[0].Prefix]++
			}
			for _, prefix := range []netip.Prefix{owned, canary} {
				if removed[prefix] != 1 {
					t.Fatalf("retirement removals for %s=%d, want exactly one", prefix, removed[prefix])
				}
			}
			switch operation {
			case "lateAdd":
				observer.handleAddrAdded(makePayload("198.51.100.1", 24))
			case "lateRemove":
				observer.handleAddrRemoved(makePayload("192.0.2.1", 24))
			case "replay":
				if err := observer.reemitAll(73); err != nil {
					t.Fatalf("retired observer replay: %v", err)
				}
			case "secondretire":
				if err := observer.retire(); err != nil {
					t.Fatalf("second retirement: %v", err)
				}
			default:
				t.Fatalf("unknown post-retirement operation %q", operation)
			}
			if len(published) != 4 {
				t.Fatalf("%s after retirement published extra routes: %+v", operation, published[4:])
			}
			// Inventory remains owned until Run joins delivery and local cleanup
			// reads it. Both the target prefix and other-prefix canary survive.
			if !reflect.DeepEqual(observer.prefixes, inventory) {
				t.Fatalf("%s after retirement changed inventory: got %+v, want %+v", operation, observer.prefixes, inventory)
			}
		})
	}
}
