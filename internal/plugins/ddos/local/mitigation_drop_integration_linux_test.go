//go:build linux

// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- vector to firewall term
// Overview: responder.go -- applyMitigation, the producer whose rule this proves
// Related: match.go -- buildDropTerm, the term the rule is lowered from
//
// A drop rule that reads back from the kernel proves the rule exists and says
// nothing about what it drops. This proof offers traffic to the victim and reads
// the answer where only a datagram the input hook let through can arrive: a UDP
// socket bound to the victim address. It runs in a user and network namespace of
// its own (userns.Enter), which holds CAP_NET_ADMIN, so it needs no privilege and
// no nftables table reaches the machine the test runs on.

package local

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/test/userns"

	// The nft backend registers itself, which is what lets firewall.ApplyAll
	// load it on demand for the responder's table.
	_ "github.com/ze-software/ze/internal/plugins/firewall/nft"
)

// dropProofWait bounds every wait of the proof. A datagram on lo is delivered,
// or counted by the drop rule, within microseconds; the bound is the test's own.
const dropProofWait = 3 * time.Second

// TestLocalMitigationDropsTheVectorAndNothingElse proves that the on-host drop
// for a UDP flood vector drops the vector's datagrams to the victim and leaves
// the victim's other traffic alone. Goal: the rule applyMitigation installs,
// lowered by the nft backend into the kernel, still drops what the vector names
// whatever the order of its matches. Method: the victim address is put on lo
// with a socket on the vector's port and one on another port; both receive a
// datagram before the mitigation, the responder then installs the drop through
// the real firewall registry and backend, and a datagram to each port is sent
// again. The rule's own counter must reach one (it matched the vector's
// datagram, and a matched rule drops), the other port must receive its
// datagram, and the vector's socket must hold nothing.
//
// MUTATION: drop the MatchDestinationPort append from buildDropTerm; the rule
// then drops the datagram to the other port too, and the other port's receive
// fails.
func TestLocalMitigationDropsTheVectorAndNothingElse(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	victim := netip.MustParseAddr("192.0.2.1")
	dropProofVictimOnLo(t, victim)
	vector := dropProofListen(t, victim, 53)
	other := dropProofListen(t, victim, 54)
	sender := dropProofSender(t, victim)

	dropProofSend(t, sender, victim, 53)
	dropProofSend(t, sender, victim, 54)
	dropProofReceive(t, vector, "the vector's port before the mitigation")
	dropProofReceive(t, other, "the other port before the mitigation")

	r := newResponder(&Config{ResponseLevel: responseEnforce}, nil)
	t.Cleanup(func() {
		r.removeMitigation()
		if err := firewall.CloseBackend(); err != nil {
			t.Errorf("close the firewall backend: %v", err)
		}
	})
	target := ddosevent.VectorTuple{DstPrefix: netip.PrefixFrom(victim, 32), Proto: 17, DstPort: 53}
	r.applyMitigation(target, ddosevent.FamilyUDPFlood, ddosevent.DirectionLocal, false, "detected")
	if active, _ := r.status(); !active {
		t.Fatal("the responder installed no drop rule for the vector")
	}

	dropProofSend(t, sender, victim, 53)
	dropProofAwaitOneDropped(t)
	dropProofSend(t, sender, victim, 54)
	dropProofReceive(t, other, "the other port under the mitigation")
	if dropProofReadable(t, vector) {
		t.Fatal("the vector's datagram reached the victim's socket under the mitigation")
	}
}

// dropProofVictimOnLo puts the victim address on lo, so a datagram to it is
// delivered locally and crosses the input hook the mitigation's chain sits on.
func dropProofVictimOnLo(t *testing.T, victim netip.Addr) {
	t.Helper()
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("find lo: %v", err)
	}
	addr := &netlink.Addr{IPNet: &net.IPNet{IP: victim.AsSlice(), Mask: net.CIDRMask(32, 32)}}
	if err := netlink.AddrAdd(lo, addr); err != nil {
		t.Fatalf("add %s to lo: %v", victim, err)
	}
}

func dropProofListen(t *testing.T, addr netip.Addr, port int) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: addr.AsSlice(), Port: port})
	if err != nil {
		t.Fatalf("listen on %s port %d: %v", addr, port, err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // best-effort cleanup
	return conn
}

// dropProofSender opens the socket the datagrams leave from, bound to the victim
// address on an ephemeral port, so each one is local traffic on lo.
func dropProofSender(t *testing.T, victim netip.Addr) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: victim.AsSlice()})
	if err != nil {
		t.Fatalf("open the sender: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // best-effort cleanup
	return conn
}

func dropProofSend(t *testing.T, sender *net.UDPConn, victim netip.Addr, port int) {
	t.Helper()
	if _, err := sender.WriteToUDP([]byte("flood"), &net.UDPAddr{IP: victim.AsSlice(), Port: port}); err != nil {
		t.Fatalf("send to %s port %d: %v", victim, port, err)
	}
}

// dropProofReceive fails unless one datagram arrives on conn within the bound.
func dropProofReceive(t *testing.T, conn *net.UDPConn, what string) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(dropProofWait)); err != nil {
		t.Fatalf("set the read deadline: %v", err)
	}
	buf := make([]byte, 64)
	if _, _, err := conn.ReadFromUDP(buf); err != nil {
		t.Fatalf("no datagram reached %s: %v", what, err)
	}
}

// dropProofReadable reports whether conn holds a datagram now, without waiting.
func dropProofReadable(t *testing.T, conn *net.UDPConn) bool {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now()); err != nil {
		t.Fatalf("set the read deadline: %v", err)
	}
	buf := make([]byte, 64)
	_, _, err := conn.ReadFromUDP(buf)
	return err == nil
}

// dropProofAwaitOneDropped waits until the counter of the mitigation's rule reads
// one packet. The counter sits between the rule's matches and its drop, so it
// counts only the datagrams the rule matched, each of which it dropped.
func dropProofAwaitOneDropped(t *testing.T) {
	t.Helper()
	const want = 1
	deadline := time.Now().Add(dropProofWait)
	got := dropProofDropped(t)
	for got != want && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		got = dropProofDropped(t)
	}
	if got != want {
		t.Fatalf("the mitigation's rule matched %d datagrams, want %d", got, want)
	}
}

// dropProofDropped sums the counters of every rule in the responder's table.
func dropProofDropped(t *testing.T) uint64 {
	t.Helper()
	conn := new(nftables.Conn)
	chains, err := conn.ListChains()
	if err != nil {
		t.Fatalf("list the nftables chains: %v", err)
	}
	var packets uint64
	var found bool
	for _, chain := range chains {
		if chain.Table == nil || chain.Table.Name != tableName || chain.Table.Family != nftables.TableFamilyIPv4 {
			continue
		}
		found = true
		rules, err := conn.GetRules(chain.Table, chain)
		if err != nil {
			t.Fatalf("read the rules of %s %s: %v", tableName, chain.Name, err)
		}
		for _, rule := range rules {
			for _, e := range rule.Exprs {
				if counter, ok := e.(*expr.Counter); ok {
					packets += counter.Packets
				}
			}
		}
	}
	if !found {
		t.Fatalf("the kernel holds no chain of table ip %s", tableName)
	}
	return packets
}
