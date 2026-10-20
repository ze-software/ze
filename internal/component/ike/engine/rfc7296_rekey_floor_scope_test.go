// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- a rekey answers the scope in use only inside the proposal
// RFC: rfc/short/rfc7296.md -- RFC7296-2.9-1

package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// TestRFC7296RekeyFloorIsAnsweredOnlyInsideTheProposal drives narrowChildSelectors on a
// peer-initiated rekey, where the scope in use (the floor) is answered back whenever the
// proposal still covers it.
//
// VALIDATES: a proposal covering the floor in prefix, port and protocol is answered
// with the floor exactly; a proposal of TCP port 80 over the floor's prefixes, whose
// floor is any protocol and any port, is refused rather than answered with the floor.
// PREVENTS: the floor test comparing prefixes alone, which answered an any/any scope to
// a TCP/80 proposal, wider than what the initiator proposed.
func TestRFC7296RekeyFloorIsAnsweredOnlyInsideTheProposal(t *testing.T) {
	// RFC requirement: RFC7296-2.9-1 positive -- a rekey proposal of 10.0.0.0/8 any/any on both halves, over a scope in use of 10.1.0.0/16 <-> 10.2.0.0/16 any/any, is answered with exactly that scope in use: one pair, those prefixes, any port, any protocol.
	// RFC requirement: RFC7296-2.9-1 negative -- a rekey proposal of TCP port 80 over the prefixes of an any/any scope in use draws TS_UNACCEPTABLE and records no answer, never the any/any scope, which would be wider than the proposal.
	floor := []tsPair{{
		I: tsSelector{Net: mustNet(t, "10.1.0.0/16"), Port: ipsec.AnyPort()},
		R: tsSelector{Net: mustNet(t, "10.2.0.0/16"), Port: ipsec.AnyPort()},
	}}

	wide := &SA{PeerCfg: ipsec.SiteToSitePeer{Name: "ppr"}}
	err := narrowChildSelectors(wide,
		selectorsToWire([]tsSelector{{Net: mustNet(t, "10.0.0.0/8"), Port: ipsec.AnyPort()}}, wire.PayloadTypeTSi),
		selectorsToWire([]tsSelector{{Net: mustNet(t, "10.0.0.0/8"), Port: ipsec.AnyPort()}}, wire.PayloadTypeTSr),
		floor)
	if err != nil {
		t.Fatalf("a rekey proposal covering the scope in use was refused: %v", err)
	}
	if len(wide.NegotiatedPairs) != 1 {
		t.Fatalf("answered %d pairs, want the one pair of the scope in use", len(wide.NegotiatedPairs))
	}
	got := wide.NegotiatedPairs[0]
	if got.I.Net.String() != "10.1.0.0/16" || got.R.Net.String() != "10.2.0.0/16" {
		t.Errorf("answered %v <-> %v, want the scope in use 10.1.0.0/16 <-> 10.2.0.0/16", got.I.Net, got.R.Net)
	}
	if got.I != floor[0].I || got.R.Port != floor[0].R.Port || got.R.Proto != floor[0].R.Proto {
		t.Errorf("answered pair %+v differs from the scope in use %+v in port or protocol", got, floor[0])
	}

	tcp80 := &SA{PeerCfg: ipsec.SiteToSitePeer{Name: "ppr"}}
	err = narrowChildSelectors(tcp80,
		selectorsToWire([]tsSelector{{Net: mustNet(t, "10.1.0.0/16"), Port: ipsec.AnyPort(), Proto: 6}}, wire.PayloadTypeTSi),
		selectorsToWire([]tsSelector{{Net: mustNet(t, "10.2.0.0/16"), Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 80}, Proto: 6}}, wire.PayloadTypeTSr),
		floor)
	if !errors.Is(err, errTSUnacceptable) {
		t.Errorf("a TCP/80 proposal against an any/any scope in use answered err=%v, want TS_UNACCEPTABLE", err)
	}
	for _, p := range tcp80.NegotiatedPairs {
		t.Errorf("answered %v port %v proto %d <-> %v port %v proto %d to a TCP/80 proposal",
			p.I.Net, p.I.Port, p.I.Proto, p.R.Net, p.R.Port, p.R.Proto)
	}
}
