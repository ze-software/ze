// Design: docs/guide/command-reference.md -- show firewall ruleset counters
// Related: backend_linux.go -- applyChain places the counter, GetCounters reads it back
// Related: integration_linux_test.go -- withNftNetNS and the backend helper

//go:build integration && linux

package firewallnft

import (
	"net"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/firewall"
)

// counterProbeTable is the table this file applies. The ze_ prefix is what
// marks it as ze-owned, so Apply reconciles it rather than leaving it alone.
const (
	counterProbeTable = "ze_fw_counter"
	counterProbeChain = "input"
	counterProbeDummy = "zedummy0"
)

// VALIDATES: a term's counter reports the packets THAT TERM matched, read
// back from a real kernel through GetCounters.
// PREVENTS: the counter sitting ahead of the term's matches. nftables
// evaluates a rule from left to right and abandons it at the first expression
// that does not match, so a leading counter increments for every packet the
// CHAIN sees. Measured in a QEMU guest on 2026-09-08: a policy naming lo and
// dummy0 printed `counter packets 3 bytes 120` on BOTH kernel rules, so the
// dummy0 row reported the 3 packets only lo had carried, and an operator
// reading the ruleset to learn which interface carries the traffic got the
// same number for every interface
// (plan/journal/counter-counts-the-wrong-packets.md).
//
// The shape of the proof is the shape of that measurement: two terms on two
// interfaces, traffic that only one of them can match, and the other term's
// counter asserted at zero. The matching term's counter is the positive
// control -- without it, a zero would read the same for a broken dataplane as
// for a probe that never sent anything.
//
// The non-matching term is programmed FIRST so the kernel reaches it. A
// verdict ends the evaluation of the chain, so a term placed after the
// accepting one would count zero whatever the counter's position.
func TestNftIntegrationCounterCountsOnlyTheTermsOwnPackets(t *testing.T) {
	withNftNetNS(t, func() {
		loopbackUp(t)
		addDummyLink(t, counterProbeDummy)

		b := newNftIntegrationBackend(t)
		if err := b.Apply(counterProbeTables()); err != nil {
			t.Fatalf("apply: %v", err)
		}

		const packets = 4
		sendLoopbackPackets(t, packets)

		counters := chainTermCounters(t, b, counterProbeTable, counterProbeChain)

		loopback, ok := counters["loopback-term"]
		if !ok {
			t.Fatalf("chain %q reported no counter for the loopback term, got %v", counterProbeChain, counters)
		}
		if loopback.Packets < packets {
			t.Fatalf("loopback term counted %d packets, want at least %d: the probe's own traffic did not reach the rule, so the zero below proves nothing",
				loopback.Packets, packets)
		}

		dummy, ok := counters["dummy-term"]
		if !ok {
			t.Fatalf("chain %q reported no counter for the dummy term, got %v", counterProbeChain, counters)
		}
		if dummy.Packets != 0 {
			t.Fatalf("dummy term counted %d packets and %d bytes over interface %q, want 0: %d packets arrived on lo and none on %q, so this term matched nothing and its counter is reporting the chain's traffic",
				dummy.Packets, dummy.Bytes, counterProbeDummy, loopback.Packets, counterProbeDummy)
		}
		if dummy.Bytes != 0 {
			t.Fatalf("dummy term counted %d bytes with 0 packets", dummy.Bytes)
		}
	})
}

// counterProbeTables returns the desired state the probe applies: one input
// chain holding one term for an interface no packet arrives on, followed by
// one term for the loopback the probe really sends over.
func counterProbeTables() []firewall.Table {
	return []firewall.Table{{
		Name:   counterProbeTable,
		Family: firewall.FamilyInet,
		Chains: []firewall.Chain{{
			Name:     counterProbeChain,
			IsBase:   true,
			Type:     firewall.ChainFilter,
			Hook:     firewall.HookInput,
			Priority: 0,
			Policy:   firewall.PolicyAccept,
			Terms: []firewall.Term{
				{
					Name:    "dummy-term",
					Matches: []firewall.Match{firewall.MatchInputInterface{Name: counterProbeDummy}},
					Actions: []firewall.Action{firewall.Accept{}},
				},
				{
					Name:    "loopback-term",
					Matches: []firewall.Match{firewall.MatchInputInterface{Name: "lo"}},
					Actions: []firewall.Action{firewall.Accept{}},
				},
			},
		}},
	}}
}

// chainTermCounters reads one chain's counters back from the kernel and keys
// them by term name, which is the unit `show firewall ruleset` prints.
func chainTermCounters(t *testing.T, b *backend, tableName, chainName string) map[string]firewall.TermCounter {
	t.Helper()

	chains, err := b.GetCounters(tableName)
	if err != nil {
		t.Fatalf("GetCounters(%q): %v", tableName, err)
	}
	for _, c := range chains {
		if c.Chain != chainName {
			continue
		}
		terms := make(map[string]firewall.TermCounter, len(c.Terms))
		for _, term := range c.Terms {
			terms[term.Name] = term
		}
		return terms
	}
	t.Fatalf("table %q holds no chain %q", tableName, chainName)
	return nil
}

// loopbackUp brings lo up inside the test namespace and gives it 127.0.0.1/8
// when the kernel has not already done so. A namespace starts with lo down,
// and a down loopback carries no packet for the input chain to count.
func loopbackUp(t *testing.T) {
	t.Helper()

	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Skipf("requires CAP_NET_ADMIN: cannot read lo: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Skipf("requires CAP_NET_ADMIN: cannot bring lo up: %v", err)
	}

	addrs, err := netlink.AddrList(lo, netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("list lo addresses: %v", err)
	}
	if len(addrs) > 0 {
		return
	}
	addr := &netlink.Addr{IPNet: &net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)}}
	if err := netlink.AddrAdd(lo, addr); err != nil {
		t.Fatalf("add 127.0.0.1/8 to lo: %v", err)
	}
}

// addDummyLink creates one dummy interface and brings it up, so the term that
// names it names an interface that exists. The probe sends nothing over it:
// its counter is the one that has to stay at zero.
func addDummyLink(t *testing.T, name string) {
	t.Helper()

	attrs := netlink.NewLinkAttrs()
	attrs.Name = name
	if err := netlink.LinkAdd(&netlink.Dummy{LinkAttrs: attrs}); err != nil {
		t.Skipf("requires CAP_NET_ADMIN: cannot create dummy %q: %v", name, err)
	}
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("read dummy %q back: %v", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("bring dummy %q up: %v", name, err)
	}
}

// sendLoopbackPackets sends count UDP datagrams to 127.0.0.1 and reads every
// one of them back. Reading them back is what proves they crossed the input
// hook, rather than proving only that sendto returned.
func sendLoopbackPackets(t *testing.T, count int) {
	t.Helper()

	receiver, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on 127.0.0.1: %v", err)
	}
	defer receiver.Close() //nolint:errcheck // the namespace goes away with the test

	sender, err := net.Dial("udp4", receiver.LocalAddr().String())
	if err != nil {
		t.Fatalf("dial %v: %v", receiver.LocalAddr(), err)
	}
	defer sender.Close() //nolint:errcheck // the namespace goes away with the test

	if err := receiver.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	buf := make([]byte, 64)
	for i := range count {
		if _, err := sender.Write([]byte("ze-counter-probe")); err != nil {
			t.Fatalf("send packet %d: %v", i, err)
		}
		if _, _, err := receiver.ReadFrom(buf); err != nil {
			t.Fatalf("packet %d was not delivered over lo: %v", i, err)
		}
	}
}
