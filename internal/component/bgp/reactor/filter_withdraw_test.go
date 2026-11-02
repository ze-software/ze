package reactor

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// verdictWithdrawalOf answers the withdrawal buildWithdrawalPayload makes of body:
// the bytes a withdraw verdict must leave a policy step with.
func verdictWithdrawalOf(t *testing.T, body []byte) []byte {
	t.Helper()
	buf := make([]byte, len(body)+4)
	n := buildWithdrawalPayload(body, buf, nil)
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
		suppress, override := r.exportFilterForBody(peer, received)
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
