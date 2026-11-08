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
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

// xfrmInterfaceStub stands fixed interface facts in place of the iface resolver, and a
// recorder in place of the tunnel-source guard's kernel side (source_guard_test.go). A
// test changes facts or err between applies to move an address or fail a read.
type xfrmInterfaceStub struct {
	facts map[string]xfrmInterfaceFacts
	err   map[string]error
	guard *guardRecorder
}

func stubXFRMInterfaceReads(t *testing.T) *xfrmInterfaceStub {
	t.Helper()
	stub := &xfrmInterfaceStub{facts: map[string]xfrmInterfaceFacts{}, err: map[string]error{}, guard: newGuardRecorder()}
	previousGuards := guards
	guards = newSourceGuards(stub.guard)
	t.Cleanup(func() { guards = previousGuards })
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

// liveCatchAll answers the action of the live IPv4 out catch-all entry of the node-wide
// SPD (if_id 0), never an interface SPD's catch-all, which discards under both values.
func liveCatchAll(t *testing.T, dp *catchAllDP) dataplane.SPAction {
	t.Helper()
	for _, p := range dp.live {
		if p.Priority == int(dataplane.PriorityUnmatched) && p.IfID == 0 && p.Dir == dataplane.SADirOut && p.Src.IP.To4() != nil {
			return p.Action
		}
	}
	t.Fatal("no IPv4 out catch-all entry is live")
	return 0
}

// vti0IfID is the if_id the stubbed vti0 reports: its SPD-ID.
const vti0IfID = 9

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
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
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

	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::2")}}
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
	if len(state.xfrmEntries) != 0 {
		t.Errorf("releaseSPD left the engine recording %d exemptions", len(state.xfrmEntries))
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
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID}
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
		if p.Priority == int(dataplane.PriorityUnmatched) && p.IfID == 0 && p.Action == dataplane.SPActionDiscard && firstDiscard < 0 {
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

// interfaceCatchAllKey renders one entry with its if_id, for comparing the entries of
// one interface SPD.
func interfaceCatchAllKey(p *dataplane.SPParams) string {
	return strconv.Itoa(int(p.Dir)) + " " + p.Src.String() + " -> " + p.Dst.String() +
		" if_id " + strconv.FormatUint(uint64(p.IfID), 10)
}

// liveInterfaceCatchAll answers the interface SPD catch-all entries the fake backend
// holds now: the entries ranked last in an SPD other than the node-wide one.
func liveInterfaceCatchAll(t *testing.T, dp *catchAllDP) []string {
	t.Helper()
	var keys []string
	for _, p := range dp.live {
		if p.Priority != int(dataplane.PriorityUnmatched) || p.IfID == 0 {
			continue
		}
		if p.Action != dataplane.SPActionDiscard {
			t.Errorf("%s: interface catch-all action %d, want discard", interfaceCatchAllKey(&p), p.Action)
		}
		keys = append(keys, interfaceCatchAllKey(&p))
	}
	slices.Sort(keys)
	return keys
}

// wantInterfaceCatchAll is the six entries of one interface SPD's catch-all.
func wantInterfaceCatchAll(ifID uint32) []string {
	in, out, fwd := strconv.Itoa(int(dataplane.SADirIn)), strconv.Itoa(int(dataplane.SADirOut)), strconv.Itoa(int(dataplane.SADirFwd))
	id := " if_id " + strconv.FormatUint(uint64(ifID), 10)
	want := []string{
		in + " 0.0.0.0/0 -> 0.0.0.0/0" + id,
		out + " 0.0.0.0/0 -> 0.0.0.0/0" + id,
		fwd + " 0.0.0.0/0 -> 0.0.0.0/0" + id,
		in + " ::/0 -> ::/0" + id,
		out + " ::/0 -> ::/0" + id,
		fwd + " ::/0 -> ::/0" + id,
	}
	slices.Sort(want)
	return want
}

// RFC requirement: RFC4301-4.4.1-3 positive -- the final entry interfaceSPDCatchAll
// builds for a bound XFRM interface's SPD is DISCARD in, out and fwd for IPv4 and IPv6
// at that interface's if_id, ranked at PriorityUnmatched below every Child SA policy,
// with any address, protocol, port and device, so it matches what the SPD otherwise
// leaves unmatched and discards it.
//
// VALIDATES: AC-1 and AC-2 at the builder: one interface SPD's final entry is six
// DISCARD entries, in, out and fwd for both families, at the interface's if_id, ranked
// last, any protocol and port; the largest if_id is carried unchanged (boundary).
// Method: build the entries for if_id 9 and for 2^32-1 and compare with the expected
// set, then check rank and match width on each.
// PREVENTS: an interface catch-all built in the node-wide SPD (if_id 0), which would
// drop every packet of the node, or one ranked above the interface's Child SAs.
func TestInterfaceSPDCatchAllEntries(t *testing.T) {
	for _, ifID := range []uint32{vti0IfID, math.MaxUint32} {
		got := interfaceSPDCatchAll(ifID)
		keys := make([]string, 0, len(got))
		for i := range got {
			p := &got[i]
			keys = append(keys, interfaceCatchAllKey(p))
			if p.Action != dataplane.SPActionDiscard {
				t.Errorf("%s: action %d, want discard", interfaceCatchAllKey(p), p.Action)
			}
			if p.Priority != int(dataplane.PriorityUnmatched) {
				t.Errorf("%s: priority %d, want %d", interfaceCatchAllKey(p), p.Priority, int(dataplane.PriorityUnmatched))
			}
			if p.Priority <= dataplane.PriorityChildSA {
				t.Errorf("%s: priority %d outranks a Child SA policy (%d)", interfaceCatchAllKey(p), p.Priority, dataplane.PriorityChildSA)
			}
			if !p.SrcPort.IsAny() || !p.DstPort.IsAny() || p.UpperProto != 0 || p.IfIndex != 0 {
				t.Errorf("%s: narrowed to a protocol, port or device", interfaceCatchAllKey(p))
			}
		}
		slices.Sort(keys)
		if want := wantInterfaceCatchAll(ifID); !slices.Equal(keys, want) {
			t.Errorf("if_id %d catch-all:\n got %q\nwant %q", ifID, keys, want)
		}
	}
}

// VALIDATES: a read answering if_id 0 for a bound interface installs nothing for it:
// if_id 0 is the node-wide SPD (boundary: the invalid value below the range).
// Method: stub vti0 with if_id 0, apply under discard, and look for any entry an
// interface plan would have placed.
// PREVENTS: an interface catch-all landing in the node-wide SPD and dropping all traffic.
func TestInterfaceSPDCatchAllRefusesIfIDZero(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{ifIndex: 7}
	state := applyTestState(t)

	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
		t.Fatalf("discard apply: %v", err)
	}
	if got := liveExemptions(dp); len(got) != 0 {
		t.Errorf("exemptions installed for an interface with if_id 0: %q", got)
	}
	if len(state.xfrmEntries) != 0 {
		t.Errorf("the engine holds %d entries for an interface with if_id 0", len(state.xfrmEntries))
	}
}

// RFC requirement: RFC4301-4.4.1-3 positive -- a commit with a peer bound to an XFRM
// interface under `unmatched discard` leaves that interface's SPD holding its final
// DISCARD entry, in, out and fwd for IPv4 and IPv6 at the interface's if_id, beside the
// node-wide DISCARD catch-all.
//
// VALIDATES: wiring row 1 (AC-1): a commit with a peer bound to vti0 under
// `unmatched discard` reaches planXFRMInterfaceEntries and leaves vti0's SPD holding its
// own DISCARD catch-all, beside the if_id 0 exemptions and the DISCARD node catch-all,
// and vti0's IPv6 address under its tunnel-source guard: its rule, its table's
// unreachable default and the nft pass and drop terms naming vti0.
// Method: applyConfig over the fake backend with a stubbed vti0, then read the live set.
// PREVENTS: an interface SPD with no final entry.
func TestApplyInstallsInterfaceSPDCatchAllAndGuard(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
	state := applyTestState(t)

	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
		t.Fatalf("discard apply: %v", err)
	}
	if got, want := liveInterfaceCatchAll(t, dp), wantInterfaceCatchAll(vti0IfID); !slices.Equal(got, want) {
		t.Errorf("interface SPD catch-all:\n got %q\nwant %q", got, want)
	}
	if got := liveExemptions(dp); len(got) != 5 {
		t.Errorf("if_id 0 exemptions: got %q, want the four interface entries and one source entry", got)
	}
	if got := liveCatchAll(t, dp); got != dataplane.SPActionDiscard {
		t.Errorf("node catch-all action %d, want discard", got)
	}
	assertGuarded(t, stub.guard, "2001:db8::1")
}

// VALIDATES: wiring row 2 (AC-2): under `unmatched bypass` vti0's SPD still gets its
// DISCARD catch-all, the node catch-all is BYPASS, and no exemption is installed.
// Method: applyConfig under bypass over the fake backend, then read the live set.
// PREVENTS: an interface catch-all that follows `unmatched` and is listed as bypass
// while the kernel drops what it matches.
func TestApplyInstallsInterfaceSPDCatchAllUnderBypass(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
	state := applyTestState(t)

	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionBypass), applyReload); err != nil {
		t.Fatalf("bypass apply: %v", err)
	}
	if got, want := liveInterfaceCatchAll(t, dp), wantInterfaceCatchAll(vti0IfID); !slices.Equal(got, want) {
		t.Errorf("interface SPD catch-all:\n got %q\nwant %q", got, want)
	}
	if got := liveExemptions(dp); len(got) != 0 {
		t.Errorf("exemptions installed under a bypass catch-all: %q", got)
	}
	if got := liveCatchAll(t, dp); got != dataplane.SPActionBypass {
		t.Errorf("node catch-all action %d, want bypass", got)
	}
	if got := stub.guard.ruleSources(); len(got) != 0 {
		t.Errorf("tunnel-source rules installed under a bypass catch-all: %q", got)
	}
	if stub.guard.publishCount != 0 {
		t.Errorf("the guard published its nft table %d times under a bypass catch-all; it guards nothing", stub.guard.publishCount)
	}
}

// VALIDATES: the interface catch-all is installed before the if_id 0 exemptions hand
// any packet over to its SPD, on a reload that turns the node catch-all to discard.
// Method: apply bypass with vti0 unbound, then discard with vti0 bound, and compare the
// positions of the first interface catch-all and the first exemption in the install log.
// PREVENTS: an instant where traffic is handed to an interface SPD with no final entry.
func TestInterfaceSPDCatchAllPrecedesTheExemptions(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID}
	state := applyTestState(t)

	unbound := testIPsecConfig(applyTestPeer("127.0.0.1"))
	unbound.Unmatched = dataplane.SPActionBypass
	if err := state.applyConfig(unbound, applyReload); err != nil {
		t.Fatalf("unbound apply: %v", err)
	}
	from := len(dp.installed)
	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
		t.Fatalf("discard apply: %v", err)
	}
	firstCatchAll, firstExemption := -1, -1
	for i, p := range dp.installed[from:] {
		if p.Priority == int(dataplane.PriorityUnmatched) && p.IfID == vti0IfID && firstCatchAll < 0 {
			firstCatchAll = i
		}
		if p.Priority == dataplane.PriorityUnmatchedXFRMInterface && firstExemption < 0 {
			firstExemption = i
		}
	}
	if firstCatchAll < 0 || firstExemption < 0 {
		t.Fatalf("the discard apply installed interface catch-all at %d and exemption at %d", firstCatchAll, firstExemption)
	}
	if firstCatchAll > firstExemption {
		t.Errorf("an exemption (install %d) went in before the interface catch-all (install %d)", firstExemption, firstCatchAll)
	}
}

// failInterfaceCatchAllInstallDP refuses, while refuse is set, the install of any
// interface SPD catch-all entry, as a kernel would refuse a daemon without
// CAP_NET_ADMIN, and installs every other entry.
type failInterfaceCatchAllInstallDP struct {
	catchAllDP
	refuse bool
}

func (d *failInterfaceCatchAllInstallDP) InstallPolicy(p dataplane.SPParams) error {
	if d.refuse && p.Priority == int(dataplane.PriorityUnmatched) && p.IfID != 0 {
		return errors.New("test: operation not permitted")
	}
	return d.catchAllDP.InstallPolicy(p)
}

// RFC requirement: RFC4301-4.4.1-3 negative -- a commit whose bound XFRM interface's
// SPD cannot receive its final DISCARD entry is not reported applied (applyConfig
// answers an error naming the interface and the catch-all, on the first apply and on a
// reload), and no if_id 0 exemption hands that interface's SPD any traffic.
//
// VALIDATES: a refused interface catch-all fails the apply as a refused node catch-all
// does (TestUnmatchedInstallFailureFailsTheApply), the reload leaving the running node
// catch-all in place, and no exemption of that interface goes in.
// Method: over a backend that refuses only interface catch-all entries, apply a vti0-bound
// discard configuration to a fresh engine (first apply); then, on another engine, apply
// an unbound bypass configuration cleanly and reload to the vti0-bound discard one.
// PREVENTS: an interface SPD with no final entry reported applied, the case where the
// kernel accepts an unmatched inbound packet under a transport-mode or offloaded state.
func TestInterfaceSPDCatchAllInstallFailureFailsTheApply(t *testing.T) {
	t.Run("first apply", func(t *testing.T) {
		dp := &failInterfaceCatchAllInstallDP{refuse: true}
		loadFailClosedBackend(t, dp)
		stub := stubXFRMInterfaceReads(t)
		stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
		state := applyTestState(t)

		err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload)
		if err == nil {
			t.Fatal("an interface SPD whose catch-all the backend refused was reported applied")
		}
		if !strings.Contains(err.Error(), "vti0") || !strings.Contains(err.Error(), "catch-all") {
			t.Errorf("error %q does not name the interface and its catch-all", err)
		}
		if got := liveInterfaceCatchAll(t, &dp.catchAllDP); len(got) != 0 {
			t.Errorf("interface catch-all entries live after a refused install: %q", got)
		}
		if got := liveExemptions(&dp.catchAllDP); len(got) != 0 {
			t.Errorf("exemptions hand traffic to an interface SPD with no final entry: %q", got)
		}
	})
	t.Run("reload", func(t *testing.T) {
		dp := &failInterfaceCatchAllInstallDP{}
		loadFailClosedBackend(t, dp)
		stub := stubXFRMInterfaceReads(t)
		stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
		state := applyTestState(t)

		unbound := testIPsecConfig(applyTestPeer("127.0.0.1"))
		unbound.Unmatched = dataplane.SPActionBypass
		if err := state.applyConfig(unbound, applyReload); err != nil {
			t.Fatalf("unbound apply: %v", err)
		}
		dp.refuse = true
		err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload)
		if err == nil {
			t.Fatal("a reload whose interface SPD catch-all the backend refused was reported applied")
		}
		if !strings.Contains(err.Error(), "vti0") || !strings.Contains(err.Error(), "catch-all") {
			t.Errorf("error %q does not name the interface and its catch-all", err)
		}
		if got := liveExemptions(&dp.catchAllDP); len(got) != 0 {
			t.Errorf("exemptions hand traffic to an interface SPD with no final entry: %q", got)
		}
		if got := liveCatchAll(t, &dp.catchAllDP); got != dataplane.SPActionBypass {
			t.Errorf("node catch-all action %d after a refused reload, want the running bypass", got)
		}
	})
}

// failInterfaceCatchAllRemovalDP refuses the removal of any interface SPD catch-all
// entry, as a kernel would refuse a daemon without CAP_NET_ADMIN.
type failInterfaceCatchAllRemovalDP struct {
	catchAllDP
}

func (d *failInterfaceCatchAllRemovalDP) RemovePolicyParams(p dataplane.SPParams) error {
	if p.Priority == int(dataplane.PriorityUnmatched) && p.IfID != 0 {
		return errors.New("test: operation not permitted")
	}
	return d.catchAllDP.RemovePolicyParams(p)
}

// RFC requirement: RFC4301-4.4.1-3 positive -- when releaseSPD cannot remove a bound
// XFRM interface's SPD catch-all, the node-wide SPD keeps its DISCARD final entry
// installed, and the SPD ledger names every interface catch-all entry left behind.
//
// VALIDATES: AC-8 for the interface SPD: releaseSPD removes the interface catch-all
// with the exemptions, before the node catch-all; when that removal fails, the SPD
// ledger holds the entry and the node catch-all stays beneath it. The clean leg also
// proves the tunnel-source guard leaves: no rule, no route and no nft term remain.
// Method: apply discard, release over a backend that removes cleanly, then apply and
// release again over one that refuses the interface catch-all removal.
// PREVENTS: an interface catch-all that outlives the daemon unreported.
func TestReleaseRemovesInterfaceSPDAndGuard(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		dp := &catchAllDP{}
		loadFailClosedBackend(t, dp)
		stub := stubXFRMInterfaceReads(t)
		stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
		state := applyTestState(t)
		if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
			t.Fatalf("discard apply: %v", err)
		}
		assertGuarded(t, stub.guard, "2001:db8::1")
		state.releaseSPD(dp, state.log)
		if got := stub.guard.ruleSources(); len(got) != 0 {
			t.Errorf("releaseSPD left tunnel-source rules: %q", got)
		}
		if len(stub.guard.routes) != 0 {
			t.Errorf("releaseSPD left tunnel-source table routes: %v", stub.guard.routes)
		}
		if len(stub.guard.terms) != 0 {
			t.Errorf("releaseSPD left the nft guard terms: %d", len(stub.guard.terms))
		}
		if got := liveInterfaceCatchAll(t, dp); len(got) != 0 {
			t.Errorf("releaseSPD left the interface catch-all installed: %q", got)
		}
		if got := liveExemptions(dp); len(got) != 0 {
			t.Errorf("releaseSPD left exemptions installed: %q", got)
		}
	})
	t.Run("refused", func(t *testing.T) {
		dp := &failInterfaceCatchAllRemovalDP{}
		loadFailClosedBackend(t, dp)
		stub := stubXFRMInterfaceReads(t)
		stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID}
		state := applyTestState(t)
		if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
			t.Fatalf("discard apply: %v", err)
		}
		catchAll := interfaceSPDCatchAll(vti0IfID)
		t.Cleanup(func() {
			for i := range catchAll {
				spdLeft.record(&catchAll[i], nil)
			}
		})
		state.releaseSPD(dp, state.log)
		left := spdLeft.describe(&xfrmCommandDP{})
		for i := range catchAll {
			command := dataplane.XFRMPolicyDeleteCommand(&catchAll[i])
			if !slices.Contains(left, command) {
				t.Errorf("the SPD ledger does not name the refused removal %q; it holds %q", command, left)
			}
		}
		if got := liveExemptions(&dp.catchAllDP); len(got) != 0 {
			t.Errorf("releaseSPD left exemptions installed: %q", got)
		}
		if got := liveCatchAll(t, &dp.catchAllDP); got != dataplane.SPActionDiscard {
			t.Errorf("node catch-all action %d after a refused interface removal, want it kept as discard", got)
		}
	})
}
