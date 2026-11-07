// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions
// Related: unmatched_xfrm_interface.go -- the exemption entries under test
// Related: unmatched_failclosed_test.go -- loadFailClosedBackend and catchAllDP

package engine

import (
	"errors"
	"math"
	"net/netip"
	"slices"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// xfrmInterfaceStub stands fixed interface facts in place of the iface resolver. A test
// changes facts or err between applies to move an address or fail a read.
type xfrmInterfaceStub struct {
	facts map[string]xfrmInterfaceFacts
	err   map[string]error
}

func stubXFRMInterfaceReads(t *testing.T) *xfrmInterfaceStub {
	t.Helper()
	stub := &xfrmInterfaceStub{facts: map[string]xfrmInterfaceFacts{}, err: map[string]error{}}
	previous := readXFRMInterfaceFn
	readXFRMInterfaceFn = func(name string) (xfrmInterfaceFacts, error) {
		if err := stub.err[name]; err != nil {
			return xfrmInterfaceFacts{}, err
		}
		facts, ok := stub.facts[name]
		if !ok {
			return xfrmInterfaceFacts{}, errors.New("no such interface: " + name)
		}
		return facts, nil
	}
	t.Cleanup(func() { readXFRMInterfaceFn = previous })
	return stub
}

// exemptionKey renders one exemption entry for set comparison.
func exemptionKey(p *dataplane.SPParams) string {
	return strconv.Itoa(int(p.Dir)) + " " + p.Src.String() + " -> " + p.Dst.String() + " ifindex " + strconv.Itoa(p.IfIndex)
}

// liveExemptions answers the exemption entries the fake backend holds now.
func liveExemptions(dp *catchAllDP) []string {
	var keys []string
	for _, p := range dp.live {
		if p.Priority == dataplane.PriorityUnmatchedXFRMInterface {
			keys = append(keys, exemptionKey(&p))
		}
	}
	slices.Sort(keys)
	return keys
}

// liveCatchAll answers the action of the live IPv4 out catch-all entry.
func liveCatchAll(t *testing.T, dp *catchAllDP) dataplane.SPAction {
	t.Helper()
	for _, p := range dp.live {
		if p.Priority == int(dataplane.PriorityUnmatched) && p.Dir == dataplane.SADirOut && p.Src.IP.To4() != nil {
			return p.Action
		}
	}
	t.Fatal("no IPv4 out catch-all entry is live")
	return 0
}

func vtiBoundConfig(action dataplane.SPAction) *ipsec.IPsecConfig {
	peer := applyTestPeer("127.0.0.1")
	peer.VTIBind = "vti0"
	cfg := testIPsecConfig(peer)
	cfg.Unmatched = action
	return cfg
}

func mustAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	addr, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return addr
}

// VALIDATES: the exemption set of one bound XFRM interface is exactly the entries the
// kernel's if_id 0 lookups need to let traffic routed into it reach its own if_id:
// out and fwd for both families scoped to the interface index, and an out entry for
// each IPv6 address it holds as the source. Every entry sits at if_id 0, bypasses, and
// ranks below every Child SA policy and every operator entry but above the catch-all.
// No in entry, and no IPv4 source entry, which would only widen the bypass.
// Method: build the set for an interface with one IPv4, one global IPv6 and one
// link-local IPv6 address, and compare it with the expected set.
// PREVENTS: an IPv6 ping over vti0 lost to the if_id 0 DISCARD catch-all
// (scenario vti-ipv6-inner-ipv4-outer), and an exemption that outranks what the
// operator or a negotiation decided.
func TestXFRMInterfaceExemptionSelectors(t *testing.T) {
	got := xfrmInterfaceExemptions(xfrmInterfaceFacts{
		ifIndex:   7,
		addresses: []netip.Addr{mustAddr(t, "10.0.0.1"), mustAddr(t, "2001:db8::1"), mustAddr(t, "fe80::1")},
	})
	out, fwd := strconv.Itoa(int(dataplane.SADirOut)), strconv.Itoa(int(dataplane.SADirFwd))
	want := []string{
		out + " 0.0.0.0/0 -> 0.0.0.0/0 ifindex 7",
		fwd + " 0.0.0.0/0 -> 0.0.0.0/0 ifindex 7",
		out + " ::/0 -> ::/0 ifindex 7",
		fwd + " ::/0 -> ::/0 ifindex 7",
		out + " 2001:db8::1/128 -> ::/0 ifindex 0",
		out + " fe80::1/128 -> ::/0 ifindex 0",
	}
	slices.Sort(want)
	keys := make([]string, 0, len(got))
	for i := range got {
		p := &got[i]
		keys = append(keys, exemptionKey(p))
		if p.IfID != 0 {
			t.Errorf("%s: if_id %d, want 0: the catch-all it stands above is at if_id 0", exemptionKey(p), p.IfID)
		}
		if p.Action != dataplane.SPActionBypass {
			t.Errorf("%s: action %d, want bypass", exemptionKey(p), p.Action)
		}
		if p.Priority != dataplane.PriorityUnmatchedXFRMInterface {
			t.Errorf("%s: priority %d, want %d", exemptionKey(p), p.Priority, dataplane.PriorityUnmatchedXFRMInterface)
		}
		if !p.SrcPort.IsAny() || !p.DstPort.IsAny() || p.UpperProto != 0 {
			t.Errorf("%s: narrowed to a protocol or port", exemptionKey(p))
		}
	}
	slices.Sort(keys)
	if !slices.Equal(keys, want) {
		t.Errorf("exemption set:\n got %q\nwant %q", keys, want)
	}

	// Lower value, higher precedence: a Child SA policy and the largest operator order
	// are searched first, the catch-all last. Read from the built entry, so the ranking
	// judged is the one installed.
	if len(got) == 0 {
		t.Fatal("no exemption entry built")
	}
	exemption := got[0].Priority
	if exemption <= dataplane.PriorityChildSA {
		t.Errorf("exemption priority %d outranks a Child SA policy (%d)", exemption, dataplane.PriorityChildSA)
	}
	if exemption <= math.MaxInt32 {
		t.Errorf("exemption priority %d outranks an operator entry of order %d", exemption, math.MaxInt32)
	}
	if exemption >= int(dataplane.PriorityUnmatched) {
		t.Errorf("exemption priority %d does not outrank the catch-all (%d)", exemption, int(dataplane.PriorityUnmatched))
	}
}

// VALIDATES: the engine keeps the exemptions in step with the configuration and the
// interface: installed by the apply that makes the catch-all discard, rebuilt when the
// interface's address moves, kept when a read fails, removed when the catch-all turns
// to bypass, and removed when the engine releases its SPD.
// Method: drive applyConfig, followAddress and releaseSPD over a fake backend and a
// stubbed iface read, and compare the live exemption entries after each step.
// PREVENTS: an exemption that outlives its address or the daemon, a transient read
// that drops a working tunnel, and a bypass catch-all left with redundant entries.
func TestXFRMExemptionsFollowApplyAddressAndRelease(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{ifIndex: 7, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
	state := applyTestState(t)

	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
		t.Fatalf("discard apply: %v", err)
	}
	out, fwd := strconv.Itoa(int(dataplane.SADirOut)), strconv.Itoa(int(dataplane.SADirFwd))
	base := []string{
		out + " 0.0.0.0/0 -> 0.0.0.0/0 ifindex 7",
		fwd + " 0.0.0.0/0 -> 0.0.0.0/0 ifindex 7",
		out + " ::/0 -> ::/0 ifindex 7",
		fwd + " ::/0 -> ::/0 ifindex 7",
	}
	want := slices.Sorted(slices.Values(append(slices.Clone(base), out+" 2001:db8::1/128 -> ::/0 ifindex 0")))
	if got := liveExemptions(dp); !slices.Equal(got, want) {
		t.Fatalf("after the discard apply:\n got %q\nwant %q", got, want)
	}
	if got := liveCatchAll(t, dp); got != dataplane.SPActionDiscard {
		t.Fatalf("catch-all action %d, want discard", got)
	}

	stub.facts["vti0"] = xfrmInterfaceFacts{ifIndex: 7, addresses: []netip.Addr{mustAddr(t, "2001:db8::2")}}
	state.followAddress()
	want = slices.Sorted(slices.Values(append(slices.Clone(base), out+" 2001:db8::2/128 -> ::/0 ifindex 0")))
	if got := liveExemptions(dp); !slices.Equal(got, want) {
		t.Fatalf("after the address moved:\n got %q\nwant %q", got, want)
	}

	stub.err["vti0"] = errors.New("transient netlink fault")
	state.followAddress()
	if got := liveExemptions(dp); !slices.Equal(got, want) {
		t.Fatalf("a failed read changed the exemptions:\n got %q\nwant %q", got, want)
	}
	delete(stub.err, "vti0")

	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionBypass), applyReload); err != nil {
		t.Fatalf("bypass apply: %v", err)
	}
	if got := liveExemptions(dp); len(got) != 0 {
		t.Fatalf("a bypass catch-all kept exemptions: %q", got)
	}

	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
		t.Fatalf("second discard apply: %v", err)
	}
	if got := liveExemptions(dp); len(got) != len(want) {
		t.Fatalf("the second discard apply installed %q, want %q", got, want)
	}
	state.releaseSPD(dp, state.log)
	if got := liveExemptions(dp); len(got) != 0 {
		t.Errorf("releaseSPD left exemptions installed: %q", got)
	}
	if len(state.xfrmExempt) != 0 {
		t.Errorf("releaseSPD left the engine recording %d exemptions", len(state.xfrmExempt))
	}
}

// VALIDATES: a reload that turns the catch-all from bypass to discard installs the
// bound interface's exemptions before the discard, so no instant holds a DISCARD
// catch-all with the tunnel's traffic unexempted.
// Method: apply bypass, then discard, and compare the positions of the first exemption
// install and the discard catch-all install in the backend's install log.
// PREVENTS: a reload that drops a working route-based tunnel's traffic while it runs.
func TestXFRMExemptionsPrecedeADiscardCatchAllOnReload(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{ifIndex: 7}
	state := applyTestState(t)

	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionBypass), applyReload); err != nil {
		t.Fatalf("bypass apply: %v", err)
	}
	from := len(dp.installed)
	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
		t.Fatalf("discard apply: %v", err)
	}
	firstExemption, firstDiscard := -1, -1
	for i, p := range dp.installed[from:] {
		if p.Priority == dataplane.PriorityUnmatchedXFRMInterface && firstExemption < 0 {
			firstExemption = i
		}
		if p.Priority == int(dataplane.PriorityUnmatched) && p.Action == dataplane.SPActionDiscard && firstDiscard < 0 {
			firstDiscard = i
		}
	}
	if firstExemption < 0 || firstDiscard < 0 {
		t.Fatalf("the discard apply installed exemption at %d and discard catch-all at %d", firstExemption, firstDiscard)
	}
	if firstExemption > firstDiscard {
		t.Errorf("the discard catch-all (install %d) went in before the exemptions (install %d)", firstDiscard, firstExemption)
	}
}

// VALIDATES: no exemption is installed for a configuration whose peers bind no XFRM
// interface: a policy-based peer's traffic is decided at if_id 0 already.
func TestXFRMExemptionsNeedABoundInterface(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stubXFRMInterfaceReads(t)
	state := applyTestState(t)

	cfg := testIPsecConfig(applyTestPeer("127.0.0.1"))
	cfg.Unmatched = dataplane.SPActionDiscard
	if err := state.applyConfig(cfg, applyReload); err != nil {
		t.Fatalf("discard apply: %v", err)
	}
	if got := liveExemptions(dp); len(got) != 0 {
		t.Errorf("exemptions installed with no bound interface: %q", got)
	}
}
