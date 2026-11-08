// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the IPv6 tunnel-source guard
// Related: source_guard.go -- the guard under test
// Related: unmatched_xfrm_interface_test.go -- stubXFRMInterfaceReads installs the recorder

package engine

import (
	"errors"
	"net"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/iprule"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// guardRecorder stands in for the guard's kernel side. It holds the rules, routes and
// nft terms the kernel would hold, and stamps each operation with the fake backend's
// install count, so a test can place it among the SPD installs.
type guardRecorder struct {
	dp           *catchAllDP
	rules        map[netip.Prefix]uint32
	routes       map[iprule.Route]bool
	terms        []firewall.Term
	publishCount int
	stamps       []guardStamp
	failRule     error
	// unguardedAtRuleDelete names a rule deleted while its source BYPASS was still live.
	unguardedAtRuleDelete []string
}

// guardStamp is one guard operation and the number of SPD installs before it.
type guardStamp struct {
	op       string
	installs int
}

func newGuardRecorder() *guardRecorder {
	return &guardRecorder{rules: make(map[netip.Prefix]uint32), routes: make(map[iprule.Route]bool)}
}

func (r *guardRecorder) stamp(op string) {
	installs := 0
	if r.dp != nil {
		installs = len(r.dp.installed)
	}
	r.stamps = append(r.stamps, guardStamp{op: op, installs: installs})
}

func (r *guardRecorder) addRule(rule iprule.Source) error {
	if r.failRule != nil {
		return r.failRule
	}
	r.stamp("rule")
	r.rules[rule.From] = rule.Table
	return nil
}

func (r *guardRecorder) deleteRule(rule iprule.Source) error {
	if r.dp != nil {
		for _, p := range r.dp.live {
			if p.Priority == dataplane.PriorityUnmatchedXFRMInterface && p.Src.String() == rule.From.String() {
				r.unguardedAtRuleDelete = append(r.unguardedAtRuleDelete, rule.From.String())
			}
		}
	}
	delete(r.rules, rule.From)
	return nil
}

func (r *guardRecorder) replaceRoute(route iprule.Route) error {
	for held := range r.routes {
		if held.Table == route.Table && held.Dst == route.Dst {
			delete(r.routes, held)
		}
	}
	r.routes[route] = true
	return nil
}

func (r *guardRecorder) deleteRoute(route iprule.Route) error {
	for held := range r.routes {
		if held.Table == route.Table && held.Dst == route.Dst {
			delete(r.routes, held)
		}
	}
	return nil
}

func (r *guardRecorder) publish(tables []firewall.Table) error {
	r.stamp("publish")
	r.publishCount++
	r.terms = nil
	for _, table := range tables {
		if table.Name != sourceGuardTableName {
			return errors.New("test: unexpected table " + table.Name)
		}
		for _, chain := range table.Chains {
			r.terms = append(r.terms, chain.Terms...)
		}
	}
	return nil
}

// ruleSources answers the sources of the rules held, sorted.
func (r *guardRecorder) ruleSources() []string {
	var out []string
	for from := range r.rules {
		out = append(out, from.String())
	}
	slices.Sort(out)
	return out
}

// assertGuarded checks that addr holds its rule into a table of the guard range, that
// the table ends in the unreachable default, and that the nft table passes it through
// the stubbed vti0 device and drops it through any other.
func assertGuarded(t *testing.T, r *guardRecorder, addr string) {
	t.Helper()
	const device = "vti0"
	from := netip.MustParsePrefix(addr + "/128")
	table, ok := r.rules[from]
	if !ok {
		t.Fatalf("no tunnel-source rule from %s; rules %q", from, r.ruleSources())
	}
	if !iprule.IPsecSourceGuardTables.Contains(table) {
		t.Errorf("the rule from %s looks up table %d, outside the guard range %v", from, table, iprule.IPsecSourceGuardTables)
	}
	unreachable := iprule.Route{Table: table, Dst: netip.MustParsePrefix("::/0"), Unreachable: true, Protocol: rtproto.IPsecSourceGuard}
	if !r.routes[unreachable] {
		t.Errorf("table %d does not end in the unreachable default; routes %v", table, r.routes)
	}
	var pass, drop bool
	for _, term := range r.terms {
		source, own := termGuards(term)
		if source != from {
			continue
		}
		switch {
		case own == device && len(term.Actions) == 1 && term.Actions[0] == firewall.Action(firewall.Accept{}):
			pass = true
		case own == "" && len(term.Actions) == 1 && term.Actions[0] == firewall.Action(firewall.Drop{}):
			drop = true
		}
	}
	if !pass {
		t.Errorf("no nft term passes %s through %s", from, device)
	}
	if !drop {
		t.Errorf("no nft term drops %s through another device", from)
	}
}

// termGuards answers the source prefix a guard term matches and the output device it
// names, empty when it names none.
func termGuards(term firewall.Term) (netip.Prefix, string) {
	var source netip.Prefix
	var device string
	for _, m := range term.Matches {
		switch v := m.(type) {
		case firewall.MatchSourceAddress:
			source = v.Prefix
		case firewall.MatchOutputInterface:
			device = v.Name
		}
	}
	return source, device
}

// VALIDATES: R-2: on a reload that turns the node catch-all to discard, the order is the
// interface catch-all, then the guard (nft table, then rule), then the if_id 0
// exemptions, then the if_id 0 catch-all re-assert; on the reload back to bypass, every
// tunnel-source rule is removed only after its source BYPASS left the backend.
// Method: stamp each guard operation with the fake backend's install count, then
// compare it with the install positions of the SPD entries.
// PREVENTS: an instant holding a source BYPASS without its guard.
func TestXFRMInterfacePlanInstallOrder(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.guard.dp = dp
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
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
	lastCatchAll, firstExemption, firstSource, nodeDiscard := -1, -1, -1, -1
	for i, p := range dp.installed[from:] {
		at := from + i
		if p.Priority == int(dataplane.PriorityUnmatched) && p.IfID == vti0IfID {
			lastCatchAll = at
		}
		if p.Priority == dataplane.PriorityUnmatchedXFRMInterface && firstExemption < 0 {
			firstExemption = at
		}
		if _, isSource := exemptionSource(&p); isSource && p.Priority == dataplane.PriorityUnmatchedXFRMInterface && firstSource < 0 {
			firstSource = at
		}
		if p.Priority == int(dataplane.PriorityUnmatched) && p.IfID == 0 && p.Action == dataplane.SPActionDiscard && nodeDiscard < 0 {
			nodeDiscard = at
		}
	}
	if lastCatchAll < 0 || firstExemption < 0 || firstSource < 0 || nodeDiscard < 0 {
		t.Fatalf("the discard apply installed interface catch-all %d, exemption %d, source exemption %d, node discard %d",
			lastCatchAll, firstExemption, firstSource, nodeDiscard)
	}
	var publishAt, ruleAt = -1, -1
	for _, s := range stub.guard.stamps {
		if s.op == "publish" && publishAt < 0 {
			publishAt = s.installs
		}
		if s.op == "rule" && ruleAt < 0 {
			ruleAt = s.installs
		}
	}
	if publishAt < 0 || ruleAt < 0 {
		t.Fatalf("the guard never published (%d) or installed its rule (%d)", publishAt, ruleAt)
	}
	if publishAt <= lastCatchAll {
		t.Errorf("the guard's nft table went in after %d installs, before the interface catch-all at %d", publishAt, lastCatchAll)
	}
	if ruleAt > firstSource {
		t.Errorf("the source exemption went in at install %d, before its rule (after %d installs)", firstSource, ruleAt)
	}
	if ruleAt > firstExemption {
		t.Errorf("an exemption went in at install %d, before the guard (after %d installs)", firstExemption, ruleAt)
	}
	if nodeDiscard < firstExemption {
		t.Errorf("the node discard went in at install %d, before the exemptions at %d", nodeDiscard, firstExemption)
	}

	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionBypass), applyReload); err != nil {
		t.Fatalf("bypass apply: %v", err)
	}
	if got := stub.guard.ruleSources(); len(got) != 0 {
		t.Errorf("the bypass apply kept tunnel-source rules: %q", got)
	}
	if len(stub.guard.unguardedAtRuleDelete) != 0 {
		t.Errorf("a tunnel-source rule was removed while its source BYPASS was live: %q", stub.guard.unguardedAtRuleDelete)
	}
}

// VALIDATES: R-5: a source exemption waits for its guard. While the rule cannot be
// installed, the interface-index exemptions go in and the source exemption does not;
// once the rule installs, the next address event installs it.
// Method: fail the recorder's rule install, apply discard, read the live exemptions,
// then clear the failure and run the address worker.
// PREVENTS: a source BYPASS live with nothing stopping its address leaving in the clear.
func TestSourceExemptionWaitsForItsGuard(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
	stub.guard.failRule = errors.New("test: operation not permitted")
	state := applyTestState(t)

	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
		t.Fatalf("discard apply: %v", err)
	}
	got := liveExemptions(dp)
	if len(got) != 4 {
		t.Fatalf("with the guard's rule refused: got %q, want the four interface-index entries only", got)
	}
	for _, key := range got {
		if strings.Contains(key, "2001:db8::1/128") {
			t.Fatalf("the source exemption went in without its guard: %q", got)
		}
	}

	stub.guard.failRule = nil
	state.followAddress()
	if got := liveExemptions(dp); len(got) != 5 {
		t.Errorf("after the guard installed: got %q, want the four interface entries and the source entry", got)
	}
	assertGuarded(t, stub.guard, "2001:db8::1")
}

// VALIDATES: AC-12: a peer whose IKE local address is an address of a bound XFRM
// interface is refused at verify, naming the peer, the address, the interface and the
// fix; any other local address passes. On the apply path the same address gets no
// guard and no source exemption (the paired check).
// Method: call refuseTunnelAddressAsIKEEndpoint over a stubbed vti0, then apply the
// clashing configuration and read the guard and the live exemptions.
// PREVENTS: a guard rule that steers the peer's own IKE traffic into its tunnel.
func TestGuardRefusesTunnelAddressAsIKEEndpoint(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}

	clash := vtiBoundConfig(dataplane.SPActionDiscard)
	var peerName string
	for name := range clash.Peers {
		peer := clash.Peers[name]
		peer.LocalAddress = "2001:db8::1"
		clash.Peers[name] = peer
		peerName = name
	}
	err := refuseTunnelAddressAsIKEEndpoint(clash)
	if err == nil {
		t.Fatal("a peer whose local address is vti0's address was accepted")
	}
	for _, want := range []string{peerName, "2001:db8::1", "vti0", "fix:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal %q does not name %q", err, want)
		}
	}
	if err := refuseTunnelAddressAsIKEEndpoint(vtiBoundConfig(dataplane.SPActionDiscard)); err != nil {
		t.Errorf("a peer with a WAN local address was refused: %v", err)
	}

	state := applyTestState(t)
	if err := state.applyConfig(clash, applyReload); err != nil {
		t.Fatalf("discard apply: %v", err)
	}
	if got := stub.guard.ruleSources(); len(got) != 0 {
		t.Errorf("the IKE local address got a tunnel-source rule: %q", got)
	}
	for _, key := range liveExemptions(dp) {
		if strings.Contains(key, "2001:db8::1/128") {
			t.Errorf("the IKE local address got a source exemption: %q", key)
		}
	}
}

// VALIDATES: wiring row 3: an interface address event re-plans the guard: the rule and
// nft terms follow vti0's IPv6 address when it moves, and the old address's rule goes.
// Method: apply discard, move the stubbed address, run the address worker.
// PREVENTS: a guard left on an address the interface no longer holds, and a new address
// with a source BYPASS and no guard.
func TestAddressEventFollowsTheGuard(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
	state := applyTestState(t)
	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
		t.Fatalf("discard apply: %v", err)
	}
	assertGuarded(t, stub.guard, "2001:db8::1")

	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::2")}}
	state.followAddress()
	assertGuarded(t, stub.guard, "2001:db8::2")
	if got := stub.guard.ruleSources(); !slices.Equal(got, []string{"2001:db8::2/128"}) {
		t.Errorf("rules after the address moved: %q, want only 2001:db8::2/128", got)
	}
	for _, term := range stub.guard.terms {
		if source, _ := termGuards(term); source == netip.MustParsePrefix("2001:db8::1/128") {
			t.Errorf("the nft table still guards the address vti0 lost: term %s", term.Name)
		}
	}
}

// VALIDATES: wiring row 4: a Child SA bound to vti0 adds a route into vti0 for its IPv6
// remote selector to vti0's guard table, an IPv4 selector or an unbound Child SA adds
// none, the route survives one of two peers leaving, and it leaves with the last; a
// route announced before the guard exists enters the table when the guard is built.
// Method: call guardTunnelRoute around applies and read the recorded routes.
// PREVENTS: traffic from a tunnel address refused as unreachable while a Child SA covers
// its destination, and a stale route steering into a tunnel that is gone.
func TestChildSAMaintainsTheGuardTable(t *testing.T) {
	dp := &catchAllDP{}
	loadFailClosedBackend(t, dp)
	stub := stubXFRMInterfaceReads(t)
	stub.facts["vti0"] = xfrmInterfaceFacts{device: "vti0", ifIndex: 7, ifID: vti0IfID, addresses: []netip.Addr{mustAddr(t, "2001:db8::1")}}
	state := applyTestState(t)
	log := state.log

	remote := &net.IPNet{IP: net.ParseIP("2001:db8:100::"), Mask: net.CIDRMask(48, 128)}
	early := &ChildSA{IfID: vti0IfID, TSRemote: remote}
	guardTunnelRoute(true, "early", early, log)

	if err := state.applyConfig(vtiBoundConfig(dataplane.SPActionDiscard), applyReload); err != nil {
		t.Fatalf("discard apply: %v", err)
	}
	table := stub.guard.rules[netip.MustParsePrefix("2001:db8::1/128")]
	tunnel := iprule.Route{Table: table, Dst: netip.MustParsePrefix("2001:db8:100::/48"), LinkIndex: 7, Protocol: rtproto.IPsecSourceGuard}
	if !stub.guard.routes[tunnel] {
		t.Fatalf("a route announced before the guard existed is not in its table: %v", stub.guard.routes)
	}

	guardTunnelRoute(true, "second", &ChildSA{IfID: vti0IfID, TSRemote: remote}, log)
	guardTunnelRoute(true, "v4", &ChildSA{IfID: vti0IfID, TSRemote: &net.IPNet{IP: net.ParseIP("10.0.0.0").To4(), Mask: net.CIDRMask(8, 32)}}, log)
	guardTunnelRoute(true, "unbound", &ChildSA{TSRemote: &net.IPNet{IP: net.ParseIP("2001:db8:200::"), Mask: net.CIDRMask(48, 128)}}, log)
	if len(stub.guard.routes) != 2 {
		t.Errorf("routes after an IPv4 and an unbound Child SA: %v, want the unreachable default and the one tunnel route", stub.guard.routes)
	}

	guardTunnelRoute(false, "early", early, log)
	if !stub.guard.routes[tunnel] {
		t.Error("the tunnel route left while a second peer's Child SA still carries it")
	}
	guardTunnelRoute(false, "second", early, log)
	if stub.guard.routes[tunnel] {
		t.Error("the tunnel route stayed after the last Child SA carrying it went")
	}
}
