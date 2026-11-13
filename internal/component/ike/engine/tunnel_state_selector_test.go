// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- binding an inbound state to its negotiated scope.
// Related: fragment_boundary_integration_linux_test.go -- real-kernel sibling-SA boundary.

package engine

import (
	"net"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// tunnelBindingDP declares the same state-selector contract as Linux XFRM.
type tunnelBindingDP struct{ mockDP }

func (*tunnelBindingDP) InboundStateSelectorSupported() bool { return true }

// tunnelBindingChild exercises the real Child constructor on a declared backend.
func tunnelBindingChild(t *testing.T, pairs []tsPair) (*ChildSA, *mockDP) {
	t.Helper()
	sa := testSA()
	sa.IsInitiator = true
	sa.NegotiatedPairs = pairs
	sa.NegotiatedTSi, sa.NegotiatedTSr = pairs[0].I.Net, pairs[0].R.Net
	dp := &tunnelBindingDP{}
	child, err := createFirstChildSA(sa, testESPGroup(), "192.0.2.1", "192.0.2.2", 7, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(child.Clear)
	return child, &dp.mockDP
}

// tunnelBindingPair describes ordinary UDP service traffic in initiator orientation.
func tunnelBindingPair(t *testing.T, local, remote string) tsPair {
	t.Helper()
	return tsPair{
		I: tsSelector{Net: multiPairPrefix(t, local), Proto: 17,
			Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 5000}},
		R: tsSelector{Net: multiPairPrefix(t, remote), Proto: 17},
	}
}

// tunnelBindingSelector requires the real installer to populate the inbound state.
func tunnelBindingSelector(t *testing.T, dp *mockDP) *dataplane.SASelector {
	t.Helper()
	if len(dp.sas) != 2 {
		t.Fatalf("installed %d states, want inbound and outbound", len(dp.sas))
	}
	if dp.sas[0].Dir != dataplane.SADirIn {
		t.Fatalf("first installed state direction = %d, want inbound", dp.sas[0].Dir)
	}
	if dp.sas[0].Sel == nil {
		t.Fatal("single-pair tunnel inbound state has no selector binding it to its own negotiated scope")
	}
	if dp.sas[1].Sel != nil {
		t.Fatal("outbound tunnel state carries an inbound selector, restricting cross-family resolution")
	}
	return dp.sas[0].Sel
}

// TestTunnelSinglePairStateBindsItsNegotiatedScope checks both IKE roles and inner
// families through createFirstChildSA, keeping IPv4 outer endpoints in every case.
func TestTunnelSinglePairStateBindsItsNegotiatedScope(t *testing.T) {
	for _, family := range []struct{ name, local, remote string }{
		{"ipv4", "10.2.0.0/16", "10.1.0.0/16"},
		{"ipv6", "2001:db8:2::/64", "2001:db8:1::/64"},
	} {
		for _, initiator := range []bool{true, false} {
			role := "responder"
			if initiator {
				role = "initiator"
			}
			t.Run(family.name+"/"+role, func(t *testing.T) {
				pair := tunnelBindingPair(t, family.local, family.remote)
				sa := testSA()
				sa.IsInitiator = initiator
				sa.NegotiatedPairs = []tsPair{pair}
				sa.NegotiatedTSi, sa.NegotiatedTSr = pair.I.Net, pair.R.Net
				dp := &tunnelBindingDP{}
				child, err := createFirstChildSA(sa, testESPGroup(), "192.0.2.1", "192.0.2.2", 7, dp, slogutil.DiscardLogger())
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(child.Clear)
				sel := tunnelBindingSelector(t, &dp.mockDP)
				want := &dataplane.SASelector{
					Src: pair.R.Net, Dst: pair.I.Net, UpperProto: 17,
					SrcPort: dataplane.AnyPortMatch(), DstPort: dataplane.ExactPortMatch(5000),
				}
				if !initiator {
					want.Src, want.Dst = pair.I.Net, pair.R.Net
					want.SrcPort, want.DstPort = want.DstPort, want.SrcPort
				}
				if !reflect.DeepEqual(sel, want) {
					t.Errorf("inbound state selector = %+v, want its own negotiated selector %+v", sel, want)
				}
			})
		}
	}
	t.Run("undeclared backend keeps policy enforcement", func(t *testing.T) {
		pair := tunnelBindingPair(t, "10.2.0.0/16", "10.1.0.0/16")
		_, dp := multiPairFixture(t, false, []tsPair{pair})
		if dp.sas[0].Sel != nil {
			t.Fatal("an undeclared backend was given an explicit state selector it may not support")
		}
	})
}

// TestTunnelSinglePairRekeyKeepsEachStateScope installs a same-scope or widened
// successor while retaining the predecessor's immutable kernel-install parameters.
func TestTunnelSinglePairRekeyKeepsEachStateScope(t *testing.T) {
	for _, widen := range []bool{false, true} {
		name := "same scope"
		if widen {
			name = "widened scope"
		}
		t.Run(name, func(t *testing.T) {
			pair := tunnelBindingPair(t, "10.2.0.0/16", "10.1.0.0/16")
			old, oldDP := tunnelBindingChild(t, []tsPair{pair})
			oldSelector := tunnelBindingSelector(t, oldDP)
			if widen {
				pair.I.Port = ipsec.PortSelector{Form: ipsec.PortAny}
			}
			next := newRekeyedChild(old, 0x6f020001, 0x6f020002, old.Keys, true, []tsPair{pair}, modeTunnel)
			dp := &tunnelBindingDP{}
			if err := installChildSA(next, testESPGroup().Proposals[0], dp, slogutil.DiscardLogger()); err != nil {
				t.Fatal(err)
			}
			selector := tunnelBindingSelector(t, &dp.mockDP)
			wantPort := dataplane.ExactPortMatch(5000)
			if widen {
				wantPort = dataplane.AnyPortMatch()
			}
			if selector.DstPort != wantPort {
				t.Errorf("successor local port = %+v, want %+v", selector.DstPort, wantPort)
			}
			if oldSelector.DstPort != dataplane.ExactPortMatch(5000) {
				t.Errorf("predecessor local port changed to %+v during rekey", oldSelector.DstPort)
			}
			if next.ReqID != old.ReqID {
				t.Fatal("rekey changed the shared request id that supports overlapping old and new states")
			}
		})
	}
}

// TestTunnelMultiplePairsKeepTheirPolicySet preserves arbitrary and mixed-family
// negotiation while the general multiple-selector state-binding problem stays open.
func TestTunnelMultiplePairsKeepTheirPolicySet(t *testing.T) {
	for _, secondLocal := range []string{"10.3.0.0/16", "2001:db8:3::/64"} {
		t.Run(secondLocal, func(t *testing.T) {
			secondRemote := "10.4.0.0/16"
			if secondLocal == "2001:db8:3::/64" {
				secondRemote = "2001:db8:4::/64"
			}
			pairs := []tsPair{
				tunnelBindingPair(t, "10.2.0.0/16", "10.1.0.0/16"),
				tunnelBindingPair(t, secondLocal, secondRemote),
			}
			_, dp := tunnelBindingChild(t, pairs)
			if dp.sas[0].Sel != nil {
				t.Fatal("one state selector flattened several negotiated pairs")
			}
			if len(dp.policies) != 4 {
				t.Fatalf("installed %d policies, want both pairs in both directions", len(dp.policies))
			}
		})
	}
	t.Run("missing prefixes become concrete host selectors", func(t *testing.T) {
		dp := &tunnelBindingDP{}
		child, err := createFirstChildSA(testSA(), testESPGroup(), "192.0.2.1", "192.0.2.2", 7, dp, slogutil.DiscardLogger())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(child.Clear)
		if len(dp.sas) != 2 {
			t.Fatalf("installed %d states, want inbound and outbound", len(dp.sas))
		}
		sel := tunnelBindingSelector(t, &dp.mockDP)
		want := &dataplane.SASelector{
			Src: multiPairPrefix(t, "192.0.2.2/32"), Dst: multiPairPrefix(t, "192.0.2.1/32"),
			UpperProto: 0, SrcPort: dataplane.AnyPortMatch(), DstPort: dataplane.AnyPortMatch(),
		}
		if !reflect.DeepEqual(sel, want) {
			t.Fatalf("constructor fallback state selector = %+v, want concrete host scope %+v", sel, want)
		}
	})
}

// TestTunnelParallelSinglePairStatesKeepEqualScope retains both independently
// created SAs on identical selectors, the parallel-SA case RFC 4301 permits.
func TestTunnelParallelSinglePairStatesKeepEqualScope(t *testing.T) {
	pair := tunnelBindingPair(t, "10.2.0.0/16", "10.1.0.0/16")
	first, firstDP := tunnelBindingChild(t, []tsPair{pair})
	second, secondDP := tunnelBindingChild(t, []tsPair{pair})
	if first.InboundSPI == second.InboundSPI {
		t.Fatal("parallel fixture produced one inbound SPI twice")
	}
	if first.ReqID != second.ReqID {
		t.Fatal("parallel SAs on identical selectors no longer share their policy binding")
	}
	if !reflect.DeepEqual(tunnelBindingSelector(t, firstDP), tunnelBindingSelector(t, secondDP)) {
		t.Fatal("parallel SAs on identical negotiated selectors received different state scopes")
	}
}

// TestTunnelSinglePairMOBIKEKeepsItsInnerStateScope moves actual Child endpoints
// through migrateMobikePair and requires its negotiated inner scope to stay fixed.
func TestTunnelSinglePairMOBIKEKeepsItsInnerStateScope(t *testing.T) {
	pair := tunnelBindingPair(t, "10.2.0.0/16", "10.1.0.0/16")
	child, installed := tunnelBindingChild(t, []tsPair{pair})
	sel := tunnelBindingSelector(t, installed)
	want := *sel
	sa := testSA()
	sa.peerEndpoint = &net.UDPAddr{IP: net.ParseIP("192.0.2.4"), Port: 4500}
	ps := &PeerSession{}
	dp := &mbDataplane{}
	local := &net.UDPAddr{IP: net.ParseIP("192.0.2.3"), Port: 4500}
	if err := ps.migrateMobikePair(sa, child, childInstalledPolicies(child), dp, local); err != nil {
		t.Fatal(err)
	}
	if dp.count != 1 {
		t.Fatalf("recorded %d migrations, want one", dp.count)
	}
	if !child.LocalAddr.Equal(local.IP) || !child.RemoteAddr.Equal(sa.peerEndpoint.IP) {
		t.Fatal("MOBIKE did not adopt the new outer endpoints")
	}
	if !reflect.DeepEqual(sel, &want) {
		t.Fatal("MOBIKE changed the inbound state's negotiated inner selector")
	}
}
