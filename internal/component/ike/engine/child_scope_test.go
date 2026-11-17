// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- complete directional state scopes.
// Related: child.go -- installChildDataplane publishes both states and their policies.

package engine

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// childScopeDP records the declared complete-scope backend boundary. afterFirst
// lets an assertion distinguish pre-publication ownership from a borrowed record.
type childScopeDP struct {
	mockDP
	afterFirst func()
}

func (*childScopeDP) SAStateScopeSupported() bool { return true }

func (d *childScopeDP) InstallSA(p dataplane.SAParams) error {
	d.sas = append(d.sas, p)
	if len(d.sas) == 1 {
		if d.afterFirst != nil {
			d.afterFirst()
		}
	}
	return nil
}

// childScopeFixture isolates state construction from negotiation while retaining
// the real installChildDataplane entry point and directional policy projection.
func childScopeFixture(t *testing.T, pairs []tsPair) *ChildSA {
	t.Helper()
	return &ChildSA{
		InboundSPI: 0x55000001, OutboundSPI: 0x55000002,
		LocalAddr: net.ParseIP("192.0.2.1"), RemoteAddr: net.ParseIP("192.0.2.2"),
		IfID: 17, ReqID: 23, Owner: "scope-test", Mode: modeTunnel,
		Keys:      &crypto.ChildSAKeys{},
		Selectors: pairs, SelectorsLocalIsTSi: true,
	}
}

// childScopeInstall runs the production writer without a kernel dependency.
func childScopeInstall(child *ChildSA, dp dataplane.Dataplane) error {
	_, err := installChildDataplane(child, nil, testESPGroup().Proposals[0], dp,
		slogutil.DiscardLogger())
	return err
}

// childScopeRecords requires the ordered scope on one actually published state.
func childScopeRecords(t *testing.T, dp *childScopeDP, index int) []dataplane.SASelector {
	t.Helper()
	if len(dp.sas) != 2 {
		t.Fatalf("published %d states, want both directions", len(dp.sas))
	}
	scope := dp.sas[index].Scope
	if scope == nil {
		t.Fatalf("published state %d has no complete directional scope", index)
	}
	if scope.NonInitialFragments() {
		t.Fatal("state granted noninitial-fragment permission without IKE negotiation")
	}
	if dp.sas[index].IfID != 17 {
		t.Fatalf("state interface id = %d, want 17", dp.sas[index].IfID)
	}
	return scope.Selectors()
}

// TestChildStateScopeExactDirections installs disjoint IPv4 and IPv6 pairs in
// both selector orientations and checks exact records against hand-built values.
func TestChildStateScopeExactDirections(t *testing.T) {
	pairs := []tsPair{
		{I: tsSelector{Net: multiPairPrefix(t, "10.1.0.0/24"), Proto: 17,
			Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 32000}},
			R: tsSelector{Net: multiPairPrefix(t, "10.2.0.0/25"), Proto: 0}},
		{I: tsSelector{Net: multiPairPrefix(t, "2001:db8:1::/64"), Proto: 6},
			R: tsSelector{Net: multiPairPrefix(t, "2001:db8:2::/80"), Proto: 6,
				Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 443}}},
	}
	for _, localIsTSi := range []bool{true, false} {
		t.Run(fmt.Sprint(localIsTSi), func(t *testing.T) {
			child := childScopeFixture(t, pairs)
			child.SelectorsLocalIsTSi = localIsTSi
			// Rekey's key role is deliberately opposite to the selector orientation.
			child.LocalIsInitiator = !localIsTSi
			dp := &childScopeDP{}
			if err := childScopeInstall(child, dp); err != nil {
				t.Fatal(err)
			}
			in := []dataplane.SASelector{
				{Src: pairs[0].R.Net, Dst: pairs[0].I.Net, UpperProto: 17,
					DstPort: dataplane.ExactPortMatch(32000)},
				{Src: pairs[1].R.Net, Dst: pairs[1].I.Net, UpperProto: 6,
					SrcPort: dataplane.ExactPortMatch(443)},
			}
			out := []dataplane.SASelector{
				{Src: pairs[0].I.Net, Dst: pairs[0].R.Net, UpperProto: 17,
					SrcPort: dataplane.ExactPortMatch(32000)},
				{Src: pairs[1].I.Net, Dst: pairs[1].R.Net, UpperProto: 6,
					DstPort: dataplane.ExactPortMatch(443)},
			}
			if !localIsTSi {
				in, out = out, in
			}
			for i, want := range [][]dataplane.SASelector{in, out} {
				if got := childScopeRecords(t, dp, i); !reflect.DeepEqual(got, want) {
					t.Errorf("state %d scope = %+v, want exact directional records %+v", i, got, want)
				}
				if dp.sas[i].Sel != nil {
					t.Errorf("multipair tunnel state %d carries a legacy covering selector", i)
				}
			}
		})
	}
}

// TestChildStateScopeOwnsBothDirectionsBeforePublication mutates the raw child
// during the first backend call; both already-owned scopes must remain unchanged.
func TestChildStateScopeOwnsBothDirectionsBeforePublication(t *testing.T) {
	pair := tunnelBindingPair(t, "10.1.0.0/24", "10.2.0.0/24")
	child := childScopeFixture(t, []tsPair{pair})
	dp := &childScopeDP{afterFirst: func() { child.Selectors[0].I.Net.IP[0] = 11 }}
	if err := childScopeInstall(child, dp); err != nil {
		t.Fatal(err)
	}
	in, out := childScopeRecords(t, dp, 0), childScopeRecords(t, dp, 1)
	if got := in[0].Dst.String(); got != "10.1.0.0/24" {
		t.Fatalf("inbound scope borrowed the child prefix: %s", got)
	}
	if got := out[0].Src.String(); got != "10.1.0.0/24" {
		t.Fatalf("outbound scope was built after first state publication: %s", got)
	}
}

// TestChildStateScopeExpandedBound checks the 64-record success boundary and
// refuses the next expanded record before any state or policy can be published.
func TestChildStateScopeExpandedBound(t *testing.T) {
	for _, count := range []int{64, 65} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			pairs := make([]tsPair, count)
			for i := range pairs {
				pairs[i] = tunnelBindingPair(t, fmt.Sprintf("10.1.%d.0/24", i), "10.2.0.0/24")
			}
			dp := &childScopeDP{}
			err := childScopeInstall(childScopeFixture(t, pairs), dp)
			if count == 64 {
				if err != nil {
					t.Fatal(err)
				}
				for i := range 2 {
					if got := len(childScopeRecords(t, dp, i)); got != count {
						t.Errorf("state %d has %d records, want %d", i, got, count)
					}
				}
				return
			}
			if !errors.Is(err, dataplane.ErrSAOptionRefused) {
				t.Fatalf("65 expanded records returned %v, want scope refusal", err)
			}
			if len(dp.sas) != 0 {
				t.Fatalf("invalid scope published %d states", len(dp.sas))
			}
			if len(dp.policies) != 0 {
				t.Fatalf("invalid scope published %d policies", len(dp.policies))
			}
		})
	}
}

// TestChildStateScopeInvalidIsAtomic places malformed or absent records at the
// writer boundary; no directional state may survive validation failure.
func TestChildStateScopeInvalidIsAtomic(t *testing.T) {
	for _, invalid := range []string{"host bits", "family mismatch", "absent prefix", "empty expanded set"} {
		t.Run(invalid, func(t *testing.T) {
			pair := tunnelBindingPair(t, "10.1.0.0/24", "10.2.0.0/24")
			switch invalid {
			case "host bits":
				pair.I.Net.IP = net.ParseIP("10.1.0.1").To4()
			case "family mismatch":
				pair.R.Net = multiPairPrefix(t, "2001:db8:2::/64")
			case "absent prefix":
				pair.R.Net = nil
			case "empty expanded set":
				pair.R.Proto = 6 // Disjoint protocols have no expanded policy record.
			}
			dp := &childScopeDP{}
			err := childScopeInstall(childScopeFixture(t, []tsPair{pair}), dp)
			if !errors.Is(err, dataplane.ErrSAOptionRefused) {
				t.Fatalf("invalid scope returned %v, want scope refusal", err)
			}
			if len(dp.sas) != 0 {
				t.Fatalf("invalid scope published %d states", len(dp.sas))
			}
			if len(dp.policies) != 0 {
				t.Fatalf("invalid scope published %d policies", len(dp.policies))
			}
		})
	}
}

// TestChildStateScopeTransportLegacyIdentity requires transport's canonical
// singleton scope to match its legacy selector in each installed direction.
func TestChildStateScopeTransportLegacyIdentity(t *testing.T) {
	pair := tunnelBindingPair(t, "192.0.2.1/32", "192.0.2.2/32")
	child := childScopeFixture(t, []tsPair{pair})
	child.Mode = modeTransport
	dp := &childScopeDP{}
	if err := childScopeInstall(child, dp); err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		records := childScopeRecords(t, dp, i)
		if !reflect.DeepEqual(dp.sas[i].Sel, &records[0]) {
			t.Errorf("transport state %d legacy selector = %+v, want scope singleton %+v", i, dp.sas[i].Sel, records[0])
		}
	}
}

// TestChildStateScopeUndeclaredBackend preserves the optional backend boundary.
func TestChildStateScopeUndeclaredBackend(t *testing.T) {
	child := childScopeFixture(t, []tsPair{tunnelBindingPair(t, "10.1.0.0/24", "10.2.0.0/24")})
	dp := &mockDP{}
	if err := childScopeInstall(child, dp); err != nil {
		t.Fatal(err)
	}
	for i := range dp.sas {
		if dp.sas[i].Scope != nil {
			t.Errorf("undeclared backend received explicit scope on state %d", i)
		}
	}
}
