// Design: docs/architecture/plugin/rib-storage-design.md -- validation publication ownership.
package adj_rib_in

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestValidationPausedDebtPrecedesNextMutation(t *testing.T) {
	for _, ordinary := range []bool{false, true} {
		name := "explicit-command"
		if ordinary {
			name = "ordinary-state"
		}
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
			r := newTestManager(t)
			peer := netip.MustParseAddr("192.0.2.1")
			first := compactRouteKey{Fam: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.113.0/24")}
			second := compactRouteKey{Fam: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.114.0/24")}
			r.ribIn[peer] = newSeqMap()
			for index, key := range []compactRouteKey{first, second} {
				r.ribIn[peer].Put(key, uint64(index+1), &RawRoute{Family: key.Fam})
			}
			r.peerUp[peer] = true
			r.validationEnabled, r.validationGate = true, true
			failure := errors.New("engine refused validation change")
			calls := 0
			r.emitValidation = func([]ribevents.ValidationRoute) error {
				calls++
				return failure
			}
			if _, _, err := r.rejectRoutesCommand([]string{peer.String(), "ipv4/unicast", first.Prefix.String(), "0"}); !errors.Is(err, failure) {
				t.Fatalf("initial refusal = %v", err)
			}
			if _, present := r.ribIn[peer].Get(first); present {
				t.Fatal("initial command did not remove its route before publication")
			}
			if ordinary {
				r.handleStructuredState(&rpc.StructuredEvent{EventType: rpc.EventKindState,
					State: rpc.SessionStateDown, PeerAddress: peer.String()})
			} else {
				if _, _, err := r.rejectRoutesCommand([]string{peer.String(), "ipv4/unicast", second.Prefix.String(), "0"}); !errors.Is(err, failure) {
					t.Fatalf("next command refusal = %v", err)
				}
			}
			routes := r.ribIn[peer]
			present := false
			if routes != nil {
				_, present = routes.Get(second)
			}
			if !present || !r.peerUp[peer] || len(r.validationChanges) != 1 {
				t.Errorf("paused debt admitted mutation: survivor=%v peer-up=%v owed=%d", present, r.peerUp[peer], len(r.validationChanges))
			}
			if ordinary {
				if calls != 1 {
					t.Errorf("ordinary intake retried publication %d times", calls)
				}
				if err := r.validationFailure(); !errors.Is(err, failure) {
					t.Errorf("ordinary refusal lost the terminal publication cause: %v", err)
				}
				if ribevents.RouteEligible(ribevents.ValidationRoute{Peer: peer, Family: second.Fam, Prefix: second.Prefix}, 0) {
					t.Error("ordinary refusal left the validation lookup open")
				}
				return
			}
			var delivered []ribevents.ValidationRoute
			r.emitValidation = func(changes []ribevents.ValidationRoute) error {
				delivered = append(delivered, changes...)
				return nil
			}
			if _, _, err := r.rejectRoutesCommand([]string{peer.String(), "ipv4/unicast", second.Prefix.String(), "0"}); err != nil {
				t.Fatal("explicit retry did not resume mutation:", err)
			}
			if len(delivered) != 2 || delivered[0].Prefix != first.Prefix || delivered[1].Prefix != second.Prefix {
				t.Fatalf("retry must settle old debt before publishing the next mutation: %+v", delivered)
			}
		})
	}
}
