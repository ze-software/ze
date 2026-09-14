// Design: rfc/short/rfc5082.md -- GTSM, the TTL 255 rule for related ICMP messages
// Overview: gtsm.go -- SetPeers and the table it builds
//
// These tests read what this package asks the kernel for, through the two
// seams SetPeers writes through, so they run on any platform and touch no
// kernel. The proofs that the kernel then behaves as RFC 5082 requires are the
// tagged tests in gtsm_rfc5082_linux_test.go.

package gtsm

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

// captureSeams redirects both kernel writers into the returned recorder and
// resets the package's reconcile state, so each test starts from "nothing is
// installed" whatever ran before it.
type capture struct {
	tables       [][]firewall.Table
	routesWanted [][]Peer
	routesGone   [][]Peer
}

func captureSeams(t *testing.T) *capture {
	t.Helper()

	c := &capture{}
	routes, publish, previous := applyRoutes, publishFilter, current

	applyRoutes = func(wanted, gone []Peer) error {
		c.routesWanted = append(c.routesWanted, wanted)
		c.routesGone = append(c.routesGone, gone)
		return nil
	}
	publishFilter = func(tables []firewall.Table) error {
		c.tables = append(c.tables, tables)
		return nil
	}
	current = nil

	t.Cleanup(func() {
		applyRoutes, publishFilter, current = routes, publish, previous
	})
	return c
}

func gtsmPeer() Peer {
	return Peer{
		Addr:     netip.MustParseAddr("192.0.2.1"),
		Port:     179,
		HopLimit: 255,
		Floor:    255,
	}
}

// TestGTSMPublishesTheICMPFilterTableForAPeer drives the entry point the BGP
// reactor calls and reads the table that reaches the firewall component.
//
// It is the wiring proof for the filter half: a peer set in, one ze_gtsm table
// out, carrying the terms that name that peer.
func TestGTSMPublishesTheICMPFilterTableForAPeer(t *testing.T) {
	c := captureSeams(t)

	if err := SetPeers([]Peer{gtsmPeer()}); err != nil {
		t.Fatalf("SetPeers: %v", err)
	}

	if len(c.tables) != 1 {
		t.Fatalf("published %d times, want 1", len(c.tables))
	}
	tables := c.tables[0]
	if len(tables) != 1 {
		t.Fatalf("published %d tables, want 1", len(tables))
	}
	if tables[0].Name != filterTableName {
		t.Errorf("table name = %q, want %q", tables[0].Name, filterTableName)
	}
	if tables[0].Family != firewall.FamilyInet {
		t.Errorf("table family = %v, want inet", tables[0].Family)
	}
	if len(tables[0].Chains) != 1 {
		t.Fatalf("table carries %d chains, want 1", len(tables[0].Chains))
	}

	chain := tables[0].Chains[0]
	if !chain.IsBase || chain.Hook != firewall.HookInput {
		t.Errorf("chain is base %v at hook %v, want a base chain at the input hook", chain.IsBase, chain.Hook)
	}
	if chain.Policy != firewall.PolicyAccept {
		t.Errorf("chain policy = %v, want accept: this table decides only what it names", chain.Policy)
	}

	wantTerms := len(relatedICMPTypes) * len(quotedPortSides)
	if len(chain.Terms) != wantTerms {
		t.Fatalf("chain carries %d terms, want %d (one per related ICMP type per quoted port side)", len(chain.Terms), wantTerms)
	}
}

// TestGTSMEveryDropTermRequiresTheQuotedBGPPort is the RFC5082-3-4 guard on
// the terms this package builds. An ICMP error that quotes no TCP header on
// the peer's BGP port belongs to no GTSM session, so no term may be able to
// match it.
func TestGTSMEveryDropTermRequiresTheQuotedBGPPort(t *testing.T) {
	peer := gtsmPeer()
	terms := peerTerms(peer)
	if len(terms) == 0 {
		t.Fatal("peerTerms built nothing for a GTSM peer")
	}

	for _, term := range terms {
		var quoted, ttl, source, icmpType bool
		for _, m := range term.Matches {
			switch v := m.(type) {
			case firewall.MatchICMPErrorQuotedTCPPort:
				quoted = v.Port == peer.Port && v.Side != firewall.QuotedPortUnspecified
			case firewall.MatchIPv4TTLBelow:
				ttl = v.Floor == peer.Floor
			case firewall.MatchSourceAddress:
				source = v.Prefix.Addr() == peer.Addr && v.Prefix.Bits() == peer.Addr.BitLen()
			case firewall.MatchICMPType:
				icmpType = true
			}
		}
		if !quoted {
			t.Errorf("term %q does not require a quoted TCP header on port %d", term.Name, peer.Port)
		}
		if !ttl {
			t.Errorf("term %q does not require a TTL below the peer's floor %d", term.Name, peer.Floor)
		}
		if !source {
			t.Errorf("term %q does not require the peer's own address as the source", term.Name)
		}
		if !icmpType {
			t.Errorf("term %q names no ICMP type, so it would read a payload as a quoted header", term.Name)
		}
		if len(term.Actions) != 1 {
			t.Fatalf("term %q carries %d actions, want one drop", term.Name, len(term.Actions))
		}
		if _, ok := term.Actions[0].(firewall.Drop); !ok {
			t.Errorf("term %q does not drop", term.Name)
		}
	}
}

// TestGTSMTermsAndRoutesAreValidForTheFirewallComponent runs the published
// table through the firewall component's own validation, which is what
// RegisterTables applies. A term this package builds that the component
// refuses would reach the kernel as nothing at all.
func TestGTSMTermsAreAcceptedByTheFirewallValidator(t *testing.T) {
	tables := filterTables([]Peer{gtsmPeer()})
	if len(tables) != 1 {
		t.Fatalf("built %d tables, want 1", len(tables))
	}
	if err := firewall.ValidateTables(tables); err != nil {
		t.Fatalf("the firewall component refuses the table this package builds: %v", err)
	}
}

// TestGTSMWithdrawsEverythingWhenNoPeerEnablesIt is the other half of the
// wiring: the peer set is the whole truth, so a reconcile that names no peer
// withdraws the table and the route rather than leaving either standing.
func TestGTSMWithdrawsEverythingWhenNoPeerEnablesIt(t *testing.T) {
	c := captureSeams(t)

	peer := gtsmPeer()
	if err := SetPeers([]Peer{peer}); err != nil {
		t.Fatalf("SetPeers: %v", err)
	}
	if err := SetPeers(nil); err != nil {
		t.Fatalf("SetPeers withdraw: %v", err)
	}

	if len(c.tables) != 2 {
		t.Fatalf("published %d times, want 2", len(c.tables))
	}
	if c.tables[1] != nil {
		t.Errorf("withdraw published %d tables, want none", len(c.tables[1]))
	}
	if len(c.routesGone) != 2 || len(c.routesGone[1]) != 1 || c.routesGone[1][0].Addr != peer.Addr {
		t.Errorf("withdraw did not hand the peer's route back for removal: %v", c.routesGone)
	}
}

// TestGTSMPublishesNothingWhenItHasNothingToSay proves a deployment that
// configures no GTSM peer never reaches the firewall component at all, so the
// nftables backend is not loaded on a box the operator asked nothing of.
func TestGTSMPublishesNothingWhenItHasNothingToSay(t *testing.T) {
	c := captureSeams(t)

	if err := SetPeers(nil); err != nil {
		t.Fatalf("SetPeers: %v", err)
	}

	if len(c.tables) != 0 || len(c.routesWanted) != 0 {
		t.Errorf("an empty peer set reached the kernel: %d publishes, %d route applies", len(c.tables), len(c.routesWanted))
	}
}

// TestGTSMBuildsNoFilterTermsWithoutAFloor covers the boundary of the receive
// half. A peer configured for the transmit half alone has no floor to compare
// against, and a term built from a zero floor would be a rule that never
// fires and a claim the firewall validator refuses.
func TestGTSMBuildsNoFilterTermsWithoutAFloor(t *testing.T) {
	peer := gtsmPeer()
	peer.Floor = 0

	if terms := peerTerms(peer); terms != nil {
		t.Errorf("built %d terms for a peer with no TTL floor, want none", len(terms))
	}
	if tables := filterTables([]Peer{peer}); tables != nil {
		t.Errorf("built a table with no terms in it: %v", tables)
	}
}

// TestGTSMBuildsNoFilterTermsForAnIPv6Peer records which half of the receive
// rule this package owns. The IPv6 half is performed by the kernel against
// the IPV6_MINHOPCOUNT that network.SetIPMinTTL installs, so a rule here would
// be a second answer to a question already answered.
func TestGTSMBuildsNoFilterTermsForAnIPv6Peer(t *testing.T) {
	peer := Peer{Addr: netip.MustParseAddr("2001:db8::1"), Port: 179, HopLimit: 255, Floor: 255}

	if terms := peerTerms(peer); terms != nil {
		t.Errorf("built %d terms for an IPv6 peer, want none", len(terms))
	}
}

// TestGTSMHandsTheRouteHalfEveryPeer is the wiring proof for the transmit
// half, which is the one cell of RFC5082-3-2 that both families share.
func TestGTSMHandsTheRouteHalfEveryPeer(t *testing.T) {
	c := captureSeams(t)

	v4 := gtsmPeer()
	v6 := Peer{Addr: netip.MustParseAddr("2001:db8::1"), Port: 179, HopLimit: 255, Floor: 255}
	if err := SetPeers([]Peer{v6, v4}); err != nil {
		t.Fatalf("SetPeers: %v", err)
	}

	if len(c.routesWanted) != 1 {
		t.Fatalf("applied routes %d times, want 1", len(c.routesWanted))
	}
	if len(c.routesWanted[0]) != 2 {
		t.Fatalf("applied %d routes, want one for each peer", len(c.routesWanted[0]))
	}
	// Sorted by address, so the kernel rule and route order does not move
	// between two applies that named the same peers.
	if c.routesWanted[0][0].Addr != v4.Addr || c.routesWanted[0][1].Addr != v6.Addr {
		t.Errorf("route order = %v, want it sorted by address", c.routesWanted[0])
	}
	for _, p := range c.routesWanted[0] {
		if p.HopLimit != 255 {
			t.Errorf("peer %s carries hop limit %d, want the 255 RFC 5082 Section 3 mandates", p.Addr, p.HopLimit)
		}
	}
}
