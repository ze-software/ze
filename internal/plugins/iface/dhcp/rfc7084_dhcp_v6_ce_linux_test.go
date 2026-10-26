//go:build linux

// Design: docs/features/interfaces.md -- DHCPv6 client on the WAN unit
//
// RFC 7084 Section 4.2 obligations an IPv6 CE router meets through the DHCPv6
// client: acting as a requesting router for prefix delegation, requesting and
// honoring SOL_MAX_RT, and taking a delegated prefix whatever its length and
// whether or not the Reply also leases an address.
//
// VALIDATES: the Solicit of a unit with pd carries an IA_PD beside the IA_NA
// and lists SOL_MAX_RT in its Option Request option; a Reply's SOL_MAX_RT
// becomes the client's; a Reply carrying only an IA_PD whose prefix is longer
// than the hint is taken as the lease.
// PREVENTS: a CE router that never asks for a delegated prefix, or refuses
// the one the server chose to give.

package ifacedhcp

import (
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
)

// TestRFC7084RequestingRouterSolicitsIAPD builds the Solicit of a unit
// configured for prefix delegation.
func TestRFC7084RequestingRouterSolicitsIAPD(t *testing.T) {
	// RFC requirement: RFC7084-4.2-5 positive -- the Solicit of a unit with pd configured carries an IA_PD option, the request of a requesting router.
	// RFC requirement: RFC7084-4.2-19 positive -- that Solicit carries both an IA_NA and an IA_PD, the IA_PD under its own IAID.
	w := testWorker(t, dHCPConfig{PDRequested: true})
	solicit, err := w.buildSolicit()
	if err != nil {
		t.Fatal(err)
	}
	pd := solicit.Options.OneIAPD()
	if pd == nil {
		t.Fatal("the Solicit carries no IA_PD")
	}
	na := solicit.Options.OneIANA()
	if na == nil {
		t.Fatal("the Solicit carries no IA_NA")
	}
	if na.IaId == pd.IaId {
		t.Errorf("IA_PD IAID %v equals the IA_NA's", pd.IaId)
	}
}

// TestRFC7084SolMaxRTRequestedAndHonoured reads the Option Request option of
// the Solicit, then offers a Reply carrying SOL_MAX_RT.
func TestRFC7084SolMaxRTRequestedAndHonoured(t *testing.T) {
	// RFC requirement: RFC7084-4.2-17 positive -- the Solicit's Option Request option lists SOL_MAX_RT, and a Reply carrying SOL_MAX_RT 3600 sets the client's SOL_MAX_RT to 3600 seconds.
	w := testWorker(t, dHCPConfig{})
	solicit, err := w.buildSolicit()
	if err != nil {
		t.Fatal(err)
	}
	if oro := solicit.Options.RequestedOptions(); !slices.Contains(oro, dhcpv6.OptionSolMaxRT) {
		t.Errorf("Solicit ORO %v lacks SOL_MAX_RT", oro)
	}
	w.replyMatcher()(testWorkerAnswer(w, dhcpv6.MessageTypeReply, solMaxRTOption(3600)))
	if w.solMaxRT != 3600*time.Second {
		t.Errorf("SOL_MAX_RT %v after a Reply carrying 3600, want 1h0m0s", w.solMaxRT)
	}
}

// TestRFC7084DelegatedPrefixOtherThanHintWithoutAddresses solicits with a /48
// hint, then answers with a Reply holding a /56 IA_PD and no IA_NA.
func TestRFC7084DelegatedPrefixOtherThanHintWithoutAddresses(t *testing.T) {
	// RFC requirement: RFC7084-4.2-21 positive -- with a /48 hint in the Solicit, a Reply delegating 2001:db8:dddd::/56 is taken and the lease holds that /56.
	// RFC requirement: RFC7084-4.2-23 positive -- a client that requests both IA_NA and IA_PD takes a Reply carrying an IA_PD and no address as its lease.
	w := testWorker(t, dHCPConfig{PDRequested: true, PDLength: 48})
	solicit, err := w.buildSolicit()
	if err != nil {
		t.Fatal(err)
	}
	if solicit.Options.OneIANA() == nil {
		t.Fatal("the Solicit carries no IA_NA, so the case is not the one the requirement names")
	}
	hint := solicit.Options.OneIAPD().Options.Prefixes()
	if len(hint) != 1 {
		t.Fatalf("Solicit IA_PD prefixes %v, want one hint", hint)
	}
	if ones, _ := hint[0].Prefix.Mask.Size(); ones != 48 {
		t.Fatalf("hint length /%d, want /48", ones)
	}

	iapd := &dhcpv6.OptIAPD{IaId: w.iaidPD, T1: 1800 * time.Second, T2: 2880 * time.Second}
	iapd.Options.Add(&dhcpv6.OptIAPrefix{
		PreferredLifetime: 1800 * time.Second,
		ValidLifetime:     3600 * time.Second,
		Prefix:            &net.IPNet{IP: net.ParseIP("2001:db8:dddd::"), Mask: net.CIDRMask(56, 128)},
	})
	at := time.Unix(0, 0)
	if !w.takeLease(testWorkerAnswer(w, dhcpv6.MessageTypeReply, iapd), at) {
		t.Fatal("takeLease refused a Reply carrying a /56 IA_PD and no address")
	}
	want := netip.MustParsePrefix("2001:db8:dddd::/56")
	live := w.lease.live(v6KindPrefix, at)
	if len(live) != 1 || live[0].prefix != want {
		t.Errorf("leased prefixes %v, want %v", live, want)
	}
}
