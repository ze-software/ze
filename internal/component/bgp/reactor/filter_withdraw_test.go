package reactor

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// verdictWithdrawalOf answers the withdrawal buildWithdrawalPayload makes of body:
// the bytes a withdraw verdict must leave a policy step with.
func verdictWithdrawalOf(t *testing.T, body []byte) []byte {
	t.Helper()
	buf := make([]byte, len(body)+4)
	n, err := buildWithdrawalPayload(body, buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("the test body has no withdrawal")
	}
	return buf[:n]
}

// TestWithdrawVerdictReactorRails proves a member's withdraw verdict is
// applied as a withdrawal on every reactor rail, never read as a reject: on
// import the step accepts the UPDATE rewritten as the withdrawal of its
// routes (a reject drops it, leaving an earlier version in the RIB); on the
// forwarded export step it answers withdraw, not a failure; on the
// originated export rail it sends the withdrawal instead of the UPDATE.
// Method: one external member answering withdraw, run through each step with
// the shared library-test body.
func TestWithdrawVerdictReactorRails(t *testing.T) {
	transport := &externalTransport{reads: []string{"local-preference"}, reply: rpc.FilterUpdateOutput{Action: rpc.FilterWithdraw}}
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
	received := libraryTestBody(100)
	want := verdictWithdrawalOf(t, received)
	if bytes.Equal(want, received) {
		t.Fatal("precondition: the test body announces a route")
	}
	addr := netip.MustParseAddr("10.0.0.4")
	policy := mixedPolicy(t, "", policychain.Member{Name: "f", Plugin: "probe"})

	t.Run("import", func(t *testing.T) {
		peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001, ImportPolicy: policy})
		in := r.runIngressPolicyChain(peer, addr, nil, testWireUpdate(received), received)
		if !in.accept || !bytes.Equal(in.modifiedPayload, want) {
			t.Fatalf("import = accept %v body %x, want accept with the withdrawal %x", in.accept, in.modifiedPayload, want)
		}
	})
	t.Run("forwarded export", func(t *testing.T) {
		res := r.runEgressPolicyChain(policy, addr.String(), 65001, 65000, false, policyview.Session{EBGP: true}, testWireUpdate(received))
		if res.accept || res.failed || !res.withdraw {
			t.Fatalf("export step = %+v, want a withdraw that is neither an accept nor a failure", res)
		}
	})
	t.Run("originated export", func(t *testing.T) {
		peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001, ExportPolicy: policy})
		suppress, override, _ := r.exportFilterForBody(peer, received)
		if suppress || !bytes.Equal(override, want) {
			t.Fatalf("originated export = suppress %v body %x, want the withdrawal %x", suppress, override, want)
		}
	})
}

// TestPolicyDryRunWithdrawVerdict proves show policy test reports a member's
// withdraw verdict as a withdraw, not a reject: the action reads withdraw, the
// member's trace entry reads withdraw, and the text after is the withdrawal
// the peer is sent, carrying the prefix as withdrawn and no attribute.
// Method: one external member answering withdraw on the import chain.
func TestPolicyDryRunWithdrawVerdict(t *testing.T) {
	r := New(&Config{})
	r.filterTransportSeam = &externalTransport{reads: []string{"local-preference"}, reply: rpc.FilterUpdateOutput{Action: rpc.FilterWithdraw}}
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	settings.ImportPolicy = mixedPolicy(t, "", policychain.Member{Name: "f", Plugin: "probe"})
	r.peers[settings.PeerKey()] = NewPeer(settings)

	result, err := (&reactorAPIAdapter{r: r}).PolicyDryRun("192.0.2.1", directionImport, "", libraryTestBody(100), true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if result.Action != dryRunActionWithdraw {
		t.Fatalf("action = %q, want withdraw (trace %+v)", result.Action, result.Trace)
	}
	if len(result.Trace) != 1 || result.Trace[0].Action != dryRunActionWithdraw {
		t.Fatalf("trace = %+v, want one withdraw entry", result.Trace)
	}
	if result.TextAfter == "" || strings.Contains(result.TextAfter, "local-preference") {
		t.Fatalf("text after = %q, want the withdrawal with no attribute", result.TextAfter)
	}
}

// twoFamilyBody is an UPDATE whose MP_REACH_NLRI announces 2001:db8::/32
// (IPv6 unicast) while its MP_UNREACH_NLRI withdraws 198.51.100.0/24 (IPv4
// multicast): two families, which one MP_UNREACH_NLRI cannot carry (RFC 7606
// Section 3(g)). withdrawals are the two UPDATEs that withdraw every route it
// names, one family each.
func twoFamilyBody() (body []byte, withdrawals [2][]byte) {
	body = []byte{
		0x00, 0x00, // Withdrawn Routes Length
		0x00, 0x35, // Total Path Attribute Length 53
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x02, 0x00, // AS_PATH empty
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64, // LOCAL_PREF 100
		// MP_REACH_NLRI: AFI 2 SAFI 1, NH 2001:db8::1, reserved, 2001:db8::/32.
		0x80, 0x0e, 0x1a, 0x00, 0x02, 0x01, 0x10,
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01,
		0x00, 0x20, 0x20, 0x01, 0x0d, 0xb8,
		// MP_UNREACH_NLRI: AFI 1 SAFI 2, 198.51.100.0/24.
		0x80, 0x0f, 0x07, 0x00, 0x01, 0x02, 0x18, 0xc6, 0x33, 0x64,
	}
	withdrawals[0] = []byte{0x00, 0x00, 0x00, 0x0b, 0x80, 0x0f, 0x08, 0x00, 0x02, 0x01, 0x20, 0x20, 0x01, 0x0d, 0xb8}
	withdrawals[1] = []byte{0x00, 0x00, 0x00, 0x0a, 0x80, 0x0f, 0x07, 0x00, 0x01, 0x02, 0x18, 0xc6, 0x33, 0x64}
	return body, withdrawals
}

// TestWithdrawVerdictTwoFamilies proves a withdraw verdict withdraws every
// route of an UPDATE naming two MP families, as two UPDATEs, on import and on
// the originated export rail: before, the conversion refused the shape, so
// import dropped the UPDATE (the earlier IPv6 version stayed in the RIB, and
// the IPv4 multicast route the source withdrew stayed too) and export
// suppressed it. Method: one external member answering withdraw; on import
// the UPDATE goes through notifyMessageReceiver and the consumer must receive
// both withdrawals in order; on export exportFilterForBody must answer the
// first as the override and the second as the follow-up.
func TestWithdrawVerdictTwoFamilies(t *testing.T) {
	body, want := twoFamilyBody()
	addr := netip.MustParseAddr("10.0.0.5")
	policy := mixedPolicy(t, "", policychain.Member{Name: "f", Plugin: "probe"})

	t.Run("import", func(t *testing.T) {
		transport := &externalTransport{reads: []string{"local-preference"}, reply: rpc.FilterUpdateOutput{Action: rpc.FilterWithdraw}}
		source := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001, ImportPolicy: policy})
		source.state.Store(int32(PeerStateEstablished))
		var delivered [][]byte
		r := &Reactor{clock: source.clock, config: &Config{LocalAS: 65000},
			peers:               map[netip.AddrPort]*Peer{source.Settings().PeerKey(): source},
			recentUpdates:       newRecentUpdateCache(100),
			attrModHandlers:     attrModHandlersWithDefaults(),
			filterTransportSeam: transport,
			orderedIngressSteps: []orderedIngressStep{{name: policyChainStepName, policyChain: true}},
			messageReceiver: &testDeliveryReceiver{consumerCount: 1, onReceived: func(_ plugin.PeerInfo, msg bgptypes.RawMessage) {
				delivered = append(delivered, bytes.Clone(msg.RawBytes))
			}}}
		wu := wireu.NewWireUpdate(body, 0)
		wu.SetSourceID(source.SourceID())

		r.notifyMessageReceiver(addr, msgtype.TypeUPDATE, body, wu, 0, rpc.DirectionReceived,
			BufHandle{ID: noPoolBufID, Buf: body}, nil, "", 0)

		if len(delivered) != 2 || !bytes.Equal(delivered[0], want[0]) || !bytes.Equal(delivered[1], want[1]) {
			t.Fatalf("consumer received %x, want the two withdrawals %x", delivered, want)
		}
	})
	t.Run("originated export", func(t *testing.T) {
		transport := &externalTransport{reads: []string{"local-preference"}, reply: rpc.FilterUpdateOutput{Action: rpc.FilterWithdraw}}
		r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
		peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001, ExportPolicy: policy})
		suppress, override, follow := r.exportFilterForBody(peer, body)
		if suppress || !bytes.Equal(override, want[0]) || !bytes.Equal(follow, want[1]) {
			t.Fatalf("originated export = suppress %v override %x follow %x, want the withdrawals %x", suppress, override, follow, want)
		}
	})
}
