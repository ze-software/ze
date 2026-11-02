package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestPolicyChainSendsSessionFacts proves the reactor hands the session facts
// it built to the policy chain on both directions, so a plugin filter that
// declared them reads them in the filter-update fields object.
//
// VALIDATES: spec-filter-wire-6-stage-filters phase 2, session facts set by
// the reactor on ingress (runIngressPolicyChain) and egress
// (runEgressPolicyChain, with the route's source).
// PREVENTS: a filter told nothing about the peer, or told a zero.
func TestPolicyChainSendsSessionFacts(t *testing.T) {
	transport := &externalTransport{
		reads: []string{"peer-name", "peer-group", "peer-as", "ibgp", "rs-client", "source-peer-name", "source-peer-as"},
		reply: rpc.FilterUpdateOutput{Action: rpc.FilterAccept},
	}
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
	received := libraryTestBody(100)
	addr := netip.MustParseAddr("10.0.0.4")
	policy := mixedPolicy(t, libraryTypeLP200, policychain.Member{Name: "member"}, policychain.Member{Name: "f", Plugin: "probe"})
	peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001, ImportPolicy: policy})
	src := filterapi.PeerFilterInfo{Address: addr, PeerAS: 65001, LocalAS: 65000, Name: "edge", GroupName: "clients"}

	if in := r.runIngressPolicyChain(peer, addr, &src, testWireUpdate(received), received); !in.accept {
		t.Fatalf("import refused the route: %+v", in)
	}
	dest := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.9"), PeerAS: 65009, LocalAS: 65000,
		Name: "client", RSClient: true}
	session := policyview.ExportSession(dest, src, true)
	if out := r.runEgressPolicyChain(policy, "10.0.0.9", 65009, 65000, false, session, testWireUpdate(received)); !out.accept {
		t.Fatalf("export refused the route: %+v", out)
	}
	if len(transport.sent) != 2 {
		t.Fatalf("the plugin filter was called %d times, want 2", len(transport.sent))
	}
	wants := []string{
		`{"peer-name":"edge","peer-group":"clients","peer-as":65001,"ibgp":false,"rs-client":false}`,
		`{"peer-name":"client","peer-as":65009,"ibgp":false,"rs-client":true,"source-peer-name":"edge","source-peer-as":65001}`,
	}
	for i, want := range wants {
		if got := string(transport.sent[i].Fields); got != want {
			t.Errorf("call %d fields\n got %s\nwant %s", i, got, want)
		}
	}
}
