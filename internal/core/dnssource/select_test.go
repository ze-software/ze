package dnssource

import (
	"errors"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"
)

func addrs(t *testing.T, texts ...string) []netip.Addr {
	t.Helper()
	out := make([]netip.Addr, len(texts))
	for i, text := range texts {
		out[i] = netip.MustParseAddr(text)
	}
	return out
}

func mustList(t *testing.T, family Family, mode Mode, sources ...Class) List {
	t.Helper()
	list, err := NewList("system dns-source", family, AudienceZe, mode, sources...)
	if err != nil {
		t.Fatalf("NewList: %v", err)
	}
	return list
}

// TestSelectFirstNonEmptyClass: under first-usable the first class holding a
// usable entry of the list's family wins alone, in its own order; a class whose
// entries are all unusable or of the wrong family is skipped; no usable entry
// anywhere answers an empty selection.
//
// VALIDATES: the first-usable rule, the family filter and the validity filter
// (spec-home-cpe-23, Data Flow "Source classes", AC-4's rule).
// PREVENTS: a backup class leaking into the answer while an earlier class is
// usable, and an unusable or wrong-family entry making a class win.
func TestSelectFirstNonEmptyClass(t *testing.T) {
	list := mustList(t, FamilyIPv4, ModeFirstUsable, ClassReceived, ClassConfigured, ClassSelf)

	got := Select(list, Candidates{
		ClassReceived:   addrs(t, "192.0.2.53", "192.0.2.54"),
		ClassConfigured: addrs(t, "198.51.100.53"),
	})
	if want := addrs(t, "192.0.2.53", "192.0.2.54"); !slices.Equal(got.Servers, want) {
		t.Fatalf("servers = %v, want %v (first class only, its order)", got.Servers, want)
	}
	if want := []Class{ClassReceived}; !slices.Equal(got.Used, want) {
		t.Fatalf("used = %v, want %v", got.Used, want)
	}

	// Received holds only entries this IPv4 list cannot use: unspecified,
	// multicast, broadcast and an IPv6 server. Configured wins.
	got = Select(list, Candidates{
		ClassReceived:   addrs(t, "0.0.0.0", "224.0.0.1", "255.255.255.255", "2001:db8::53"),
		ClassConfigured: addrs(t, "198.51.100.53"),
	})
	if want := addrs(t, "198.51.100.53"); !slices.Equal(got.Servers, want) {
		t.Fatalf("servers = %v, want %v (received unusable for IPv4)", got.Servers, want)
	}
	if want := []Class{ClassConfigured}; !slices.Equal(got.Used, want) {
		t.Fatalf("used = %v, want %v", got.Used, want)
	}

	// A class absent from the candidates falls through like an empty one.
	got = Select(list, Candidates{ClassSelf: addrs(t, "192.0.2.1")})
	if want := addrs(t, "192.0.2.1"); !slices.Equal(got.Servers, want) {
		t.Fatalf("servers = %v, want %v (fall through to the last class)", got.Servers, want)
	}

	got = Select(list, Candidates{ClassReceived: addrs(t, "2001:db8::53")})
	if len(got.Servers) != 0 || len(got.Used) != 0 {
		t.Fatalf("selection = %+v, want empty: no usable IPv4 entry", got)
	}

	// The router admits both families and keeps loopback (Ze's own lookups
	// can reach 127.0.0.53).
	router := mustList(t, FamilyAll, ModeFirstUsable, ClassConfigured, ClassReceived)
	got = Select(router, Candidates{ClassConfigured: addrs(t, "127.0.0.53", "2001:db8::53")})
	if want := addrs(t, "127.0.0.53", "2001:db8::53"); !slices.Equal(got.Servers, want) {
		t.Fatalf("router servers = %v, want %v", got.Servers, want)
	}
}

// TestSelectMergeInOrderDeduplicated: under merge every class contributes in
// list order, a server a previous class listed is not repeated, and the family
// filter applies to every class.
//
// VALIDATES: the merge rule (spec-home-cpe-23 AC-17's rule).
// PREVENTS: merge stopping at the first class, reordering, or repeating a
// server two sources carry.
func TestSelectMergeInOrderDeduplicated(t *testing.T) {
	list := mustList(t, FamilyIPv4, ModeMerge, ClassReceived, ClassConfigured)
	got := Select(list, Candidates{
		ClassReceived:   addrs(t, "192.0.2.53", "2001:db8::53", "192.0.2.53"),
		ClassConfigured: addrs(t, "198.51.100.53", "192.0.2.53"),
	})
	if want := addrs(t, "192.0.2.53", "198.51.100.53"); !slices.Equal(got.Servers, want) {
		t.Fatalf("servers = %v, want %v", got.Servers, want)
	}
	if want := []Class{ClassReceived, ClassConfigured}; !slices.Equal(got.Used, want) {
		t.Fatalf("used = %v, want %v", got.Used, want)
	}

	// A class that only repeats earlier servers contributed nothing.
	got = Select(list, Candidates{
		ClassReceived:   addrs(t, "192.0.2.53"),
		ClassConfigured: addrs(t, "192.0.2.53"),
	})
	if want := []Class{ClassReceived}; !slices.Equal(got.Used, want) {
		t.Fatalf("used = %v, want %v (configured added nothing)", got.Used, want)
	}
}

// TestModeRequiredForMultiSourceList: two sources without a mode are refused
// with the what, why and fix parts naming the list's path; one source without
// a mode, and two with a mode, are accepted; a default must state its mode.
//
// VALIDATES: AC-16's rule and its message (owner decision D-1).
// PREVENTS: Ze silently choosing backup or addition for the operator.
func TestModeRequiredForMultiSourceList(t *testing.T) {
	_, err := NewList("system dns-source", FamilyAll, AudienceZe, ModeUnspecified, ClassConfigured, ClassReceived)
	var refused *ModeRequiredError
	if !errors.As(err, &refused) {
		t.Fatalf("two sources without a mode: err = %v, want *ModeRequiredError", err)
	}
	if want := "system dns-source names 2 sources (configured, received) and no mode"; refused.What() != want {
		t.Fatalf("what = %q, want %q", refused.What(), want)
	}
	if want := "Ze cannot tell whether a later source is a backup or an addition"; refused.Why() != want {
		t.Fatalf("why = %q, want %q", refused.Why(), want)
	}
	if want := "set system dns-source mode first-usable (a later source is used only when the earlier ones have no usable server), or set system dns-source mode merge (every source, in list order)"; refused.Fix() != want {
		t.Fatalf("fix = %q, want %q", refused.Fix(), want)
	}
	if !strings.Contains(err.Error(), "\nset system dns-source mode first-usable") {
		t.Fatalf("error %q does not put the fix on a line of its own", err)
	}

	one, err := NewList("system dns-source", FamilyAll, AudienceZe, ModeUnspecified, ClassReceived)
	if err != nil {
		t.Fatalf("one source without a mode refused: %v", err)
	}
	got := Select(one, Candidates{ClassReceived: addrs(t, "192.0.2.53", "192.0.2.54")})
	if want := addrs(t, "192.0.2.53", "192.0.2.54"); !slices.Equal(got.Servers, want) {
		t.Fatalf("one-source list servers = %v, want %v", got.Servers, want)
	}

	for _, mode := range []Mode{ModeFirstUsable, ModeMerge} {
		if _, err := NewList("system dns-source", FamilyAll, AudienceZe, mode, ClassConfigured, ClassReceived); err != nil {
			t.Fatalf("two sources with mode %v refused: %v", mode, err)
		}
	}

	var empty *EmptyListError
	if _, err := NewList("system dns-source", FamilyAll, AudienceZe, ModeMerge); !errors.As(err, &empty) {
		t.Fatalf("empty list: err = %v, want *EmptyListError", err)
	}
	var repeated *RepeatedSourceError
	if _, err := NewList("system dns-source", FamilyAll, AudienceZe, ModeMerge, ClassReceived, ClassReceived); !errors.As(err, &repeated) {
		t.Fatalf("repeated source: err = %v, want *RepeatedSourceError", err)
	}
	if _, err := NewList("system dns-source", 0, AudienceZe, ModeMerge, ClassReceived); err == nil {
		t.Fatal("a list admitting no family was accepted")
	}

	stated := StatedDefault("system dns-source", FamilyAll, AudienceZe, ModeFirstUsable, ClassConfigured, ClassReceived)
	if !stated.Stated() || stated.Mode() != ModeFirstUsable {
		t.Fatalf("stated default = %+v, want stated and first-usable", stated)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("a stated default with no mode did not panic")
		}
	}()
	StatedDefault("system dns-source", FamilyAll, AudienceZe, ModeUnspecified, ClassReceived)
}

// TestReceivedAddressesOrderAndExpiry: the received class lists unexpired
// plain-DNS records by rank, interface, unit, each in its source's order.
//
// VALIDATES: the received-class order (rank DHCPv4, DHCPv6, PPP) and that an
// expired or encrypted-transport record never reaches a plain-DNS consumer.
// PREVENTS: a stale lease being selected after its expiry.
func TestReceivedAddressesOrderAndExpiry(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	records := []Record{
		{Origin: "ppp", Rank: 300, Interface: "pppoe0", Addresses: addrs(t, "192.0.2.3")},
		{Origin: "dhcpv4", Rank: 100, Interface: "eth1", Unit: "0", Addresses: addrs(t, "192.0.2.2")},
		{Origin: "dhcpv4", Rank: 100, Interface: "eth0", Unit: "0", Addresses: addrs(t, "192.0.2.1", "192.0.2.9")},
		{Origin: "dhcpv6", Rank: 200, Interface: "eth0", Unit: "0", Addresses: addrs(t, "2001:db8::1"), Expires: now},
		{Origin: "dnr", Rank: 600, Addresses: addrs(t, "192.0.2.99"), Transport: []string{"dot"}},
	}
	got := ReceivedAddresses(records, now)
	if want := addrs(t, "192.0.2.1", "192.0.2.9", "192.0.2.2", "192.0.2.3"); !slices.Equal(got, want) {
		t.Fatalf("received = %v, want %v", got, want)
	}
	if records[0].Origin != "ppp" {
		t.Fatal("ReceivedAddresses reordered its input")
	}
	if !records[3].Expired(now) || records[3].Expired(now.Add(-time.Second)) {
		t.Fatal("Expired is not inclusive of its end")
	}
}

// TestReceivedAddressesFallback: a Fallback record lists only while no other
// record lists an address. Method: the same fallback record over each other
// record shape, live and not.
//
// VALIDATES: spec-home-cpe-23 D-13 as a fallback, in the core rule every
// consumer reads (ReceivedAddresses and List.Received).
// PREVENTS: the host's resolver joining a lease's servers, and a record that
// lists nothing (expired, empty, encrypted transport) hiding the fallback.
func TestReceivedAddressesFallback(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	host := Record{Origin: "host", Rank: 500, Fallback: true, Addresses: addrs(t, "127.0.0.11")}
	cases := []struct {
		name  string
		other Record
		want  []string
	}{
		{"live lease hides the fallback",
			Record{Origin: "ppp", Rank: 300, Interface: "pppoe0", Addresses: addrs(t, "192.0.2.53")},
			[]string{"192.0.2.53"}},
		{"expired lease does not",
			Record{Origin: "dhcpv4", Rank: 100, Interface: "eth0", Unit: "0", Addresses: addrs(t, "192.0.2.1"), Expires: now},
			[]string{"127.0.0.11"}},
		{"empty lease does not",
			Record{Origin: "ra", Rank: 400, Interface: "eth0", Unit: "0"},
			[]string{"127.0.0.11"}},
		{"encrypted-transport record does not",
			Record{Origin: "dnr", Rank: 600, Addresses: addrs(t, "192.0.2.99"), Transport: []string{"dot"}},
			[]string{"127.0.0.11"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ReceivedAddresses([]Record{host, tc.other}, now)
			if want := addrs(t, tc.want...); !slices.Equal(got, want) {
				t.Fatalf("received = %v, want %v", got, want)
			}
		})
	}
	if got, want := ReceivedAddresses([]Record{host}, now), addrs(t, "127.0.0.11"); !slices.Equal(got, want) {
		t.Fatalf("fallback alone: received = %v, want %v", got, want)
	}
}

// TestSelectDropsHostOnlyAddressesForHandedOutLists: a list Ze hands to
// another host drops a loopback address and a zone-less IPv6 link-local
// address, reports each drop, and falls through to the next class when
// nothing else in a class is usable; the same candidates on a list Ze asks
// itself keep both.
//
// VALIDATES: D-16 and AC-27 (spec-home-cpe-23b): the host-only drop, the
// Dropped report the consumer logs, and the fall-through it causes.
// PREVENTS: a DHCP client, an RA host or a subscriber being handed
// 127.0.0.53 or fe80::1, which only Ze can reach.
func TestSelectDropsHostOnlyAddressesForHandedOutLists(t *testing.T) {
	candidates := Candidates{
		ClassConfigured: addrs(t, "127.0.0.1"),
		ClassReceived:   addrs(t, "127.0.0.53", "fe80::1", "169.254.1.1", "192.0.2.53"),
	}
	handedOn, err := NewList("service dhcp-server dns-source", FamilyAll, AudienceHandedOn,
		ModeFirstUsable, ClassConfigured, ClassReceived)
	if err != nil {
		t.Fatalf("NewList: %v", err)
	}
	got := Select(handedOn, candidates)
	if want := addrs(t, "169.254.1.1", "192.0.2.53"); !slices.Equal(got.Servers, want) {
		t.Fatalf("handed-on servers = %v, want %v (configured 127.0.0.1 must fall through)", got.Servers, want)
	}
	if want := []Class{ClassReceived}; !slices.Equal(got.Used, want) {
		t.Fatalf("handed-on used = %v, want %v", got.Used, want)
	}
	if want := addrs(t, "127.0.0.1", "127.0.0.53", "fe80::1"); !slices.Equal(got.Dropped, want) {
		t.Fatalf("handed-on dropped = %v, want %v", got.Dropped, want)
	}

	zoned := Select(handedOn, Candidates{ClassReceived: addrs(t, "fe80::1%eth0")})
	if want := addrs(t, "fe80::1%eth0"); !slices.Equal(zoned.Servers, want) {
		t.Fatalf("zoned link-local = %v, want %v kept", zoned.Servers, want)
	}

	router, err := NewList("system dns-source", FamilyAll, AudienceZe, ModeMerge, ClassConfigured, ClassReceived)
	if err != nil {
		t.Fatalf("NewList: %v", err)
	}
	kept := Select(router, candidates)
	if want := addrs(t, "127.0.0.1", "127.0.0.53", "fe80::1", "169.254.1.1", "192.0.2.53"); !slices.Equal(kept.Servers, want) {
		t.Fatalf("router servers = %v, want %v", kept.Servers, want)
	}
	if len(kept.Dropped) != 0 {
		t.Fatalf("router dropped = %v, want none", kept.Dropped)
	}
}

// TestNewListRefusesUnspecifiedAudience: a list that does not say who
// receives its servers is a Ze defect, refused at construction.
//
// VALIDATES: errNoAudience. PREVENTS: a consumer forgetting its audience
// and handing out host-only addresses by the permissive zero.
func TestNewListRefusesUnspecifiedAudience(t *testing.T) {
	if _, err := NewList("system dns-source", FamilyAll, AudienceUnspecified, ModeMerge, ClassReceived); !errors.Is(err, errNoAudience) {
		t.Fatalf("NewList with no audience: err = %v, want errNoAudience", err)
	}
}
