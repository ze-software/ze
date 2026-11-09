package iface

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// raLeaseRecord is a learned DHCPv6 record listing addrs until expires.
func raLeaseRecord(expires time.Time, addrs ...string) dnssource.Record {
	record := dnssource.Record{Origin: "dhcpv6", Rank: 200, Interface: "wan0", Received: time.Now(), Expires: expires}
	for _, a := range addrs {
		record.Addresses = append(record.Addresses, netip.MustParseAddr(a))
	}
	return record
}

// installLearned installs a Follower holding records as the engine's, and
// restores the previous one when the test ends.
func installLearned(t *testing.T, records ...dnssource.Record) {
	t.Helper()
	previous := raLearned.Load()
	t.Cleanup(func() { raLearned.Store(previous) })
	var f dnssource.Follower
	f.AcceptSnapshot(&dnssource.Snapshot{Sequence: 1, Records: records})
	setRALearned(&f)
}

// raUnitWithAddress is one advertising unit holding the IPv6 address addr,
// with the rdnss container rdnss (empty for none).
func raUnitWithAddress(addr, rdnss string) string {
	ra := `{"enabled": "true"}`
	if rdnss != "" {
		ra = `{"enabled": "true", "rdnss": ` + rdnss + `}`
	}
	return `{"interface": {"ethernet": {"eth0": {"unit": {"0": {"ipv6": {
		"address": ["` + addr + `"],
		"router-advertisement": ` + ra + `}}}}}}}`
}

// VALIDATES: AC-15 (D-5). A unit with no rdnss server and no dns-source
// advertises the learned IPv6 resolver; once rdnss server is committed the
// sender is updated to the configured one; with neither, it advertises none.
// PREVENTS: a unit with no rdnss server sending no resolver while the WAN
// lease carries one, and a configured server losing to a learned one.
func TestRDNSSPassThroughDefault(t *testing.T) {
	rec := newRAFactoryRecorder(t)
	active := make(map[raUnitKey]raEntry)
	log := slog.New(slog.DiscardHandler)
	expires := time.Now().Add(time.Hour).Truncate(time.Second)
	installLearned(t, raLeaseRecord(expires, "2001:db8::53"))

	reconcileRA(raTestConfig(t, `{"enabled": "true"}`), active, log)
	require.Len(t, rec.started, 1)
	spec := rec.started[0].spec
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("2001:db8::53")}, spec.Advertisement.RDNSS)
	assert.True(t, spec.RDNSSExpires.Equal(expires), "the learned lease bounds the lifetime")

	reconcileRA(raTestConfig(t, `{"enabled": "true", "rdnss": {"server": ["2001:db8:1::53"]}}`), active, log)
	require.Len(t, rec.started[0].updates, 1)
	updated := rec.started[0].updates[0]
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("2001:db8:1::53")}, updated.Advertisement.RDNSS)
	assert.True(t, updated.RDNSSExpires.IsZero(), "a configured server is not bounded by a lease")

	installLearned(t)
	reconcileRA(raTestConfig(t, `{"enabled": "true"}`), active, log)
	require.Len(t, rec.started[0].updates, 2)
	assert.Empty(t, rec.started[0].updates[1].Advertisement.RDNSS, "no server and no lease: no RDNSS")
}

// VALIDATES: AC-7 and D-6 for RA RDNSS. self is the unit's own global or
// unique-local address; a unit holding only a link-local address offers no
// self, and its list falls through to the next class.
// PREVENTS: advertising a link-local resolver with no zone, or none at all
// while a later class has one.
func TestSelfIsAddressOnClientInterface(t *testing.T) {
	rdnss := `{"server": ["2001:db8:9::53"], "dns-source": {"source": ["self", "configured"], "mode": "first-usable"}}`
	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "global address", addr: "2001:db8:1::1/64", want: "2001:db8:1::1"},
		{name: "unique-local address", addr: "fd00:1::1/64", want: "fd00:1::1"},
		{name: "link-local only falls through", addr: "fe80::1/64", want: "2001:db8:9::53"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := newRAFactoryRecorder(t)
			reconcileRA(mustParseIfaceJSON(t, raUnitWithAddress(tt.addr, rdnss)), make(map[raUnitKey]raEntry),
				slog.New(slog.DiscardHandler))
			require.Len(t, rec.started, 1)
			assert.Equal(t, []netip.Addr{netip.MustParseAddr(tt.want)}, rec.started[0].spec.Advertisement.RDNSS)
		})
	}
}

// VALIDATES: AC-27 (D-16) for RA RDNSS. A learned loopback and a zone-less
// link-local address are dropped from the handed-on list and reported for
// the log; the next usable entry is advertised.
// PREVENTS: telling hosts on the link to query ::1 or an unscoped fe80::.
func TestRDNSSDropsHostOnlyAddresses(t *testing.T) {
	cfg := mustParseRA(t, `{"enabled": "true"}`)
	learned := []dnssource.Record{raLeaseRecord(time.Time{}, "::1", "fe80::1", "2001:db8::53")}

	selection := raSelectRDNSS(cfg, "lan0", netip.Addr{}, learned, time.Now())

	assert.Equal(t, []netip.Addr{netip.MustParseAddr("2001:db8::53")}, selection.servers)
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("::1"), netip.MustParseAddr("fe80::1")}, selection.dropped)
	assert.True(t, selection.expires.IsZero(), "a record valid until withdrawn sets no end")
}

// VALIDATES: AC-8, the lifetime half. The end that bounds the advertised
// lifetime is the earliest among the learned servers, each server ending
// with the last unexpired record that lists it.
// PREVENTS: advertising a resolver for longer than any lease supplies it.
func TestRDNSSLifetimeBoundedByLease(t *testing.T) {
	cfg := mustParseRA(t, `{"enabled": "true", "rdnss": {"dns-source": {"source": ["received"]}}}`)
	now := time.Now()
	soon, later, latest := now.Add(time.Minute), now.Add(time.Hour), now.Add(2*time.Hour)
	learned := []dnssource.Record{
		raLeaseRecord(soon, "2001:db8::53", "2001:db8::54"),
		raLeaseRecord(latest, "2001:db8::53"),
		raLeaseRecord(later, "2001:db8::54"),
	}

	selection := raSelectRDNSS(cfg, "lan0", netip.Addr{}, learned, now)

	require.Len(t, selection.servers, 2)
	assert.True(t, selection.expires.Equal(later), "::54 ends at its later record, before ::53")
}

// VALIDATES: AC-16b and the refusal of radius for the RDNSS list: config
// verify refuses radius, and two sources with no mode, naming the path.
// PREVENTS: a list that means nothing on a LAN unit reaching the sender.
func TestRDNSSDNSSourceRefused(t *testing.T) {
	for _, body := range []string{
		`{"enabled": "true", "rdnss": {"dns-source": {"source": ["radius"]}}}`,
		`{"enabled": "true", "rdnss": {"dns-source": {"source": ["configured", "received"]}}}`,
	} {
		_, err := parseIfaceConfig(raUnitJSON(body))
		require.Error(t, err, body)
		assert.Contains(t, err.Error(), raRDNSSSourcePath, body)
	}
	_, err := parseIfaceConfig(raUnitJSON(`{"enabled": "true", "rdnss": {"dns-source": {"source": ["configured", "received"], "mode": "merge"}}}`))
	require.NoError(t, err, "the same list with mode is accepted")
}

// VALIDATES: finding 4 of the 23b review, RFC 8106 Section 5.1 on the
// advertising side. A link-local resolver learned on wan0 carries wan0 as its
// zone; a lan0 unit refuses it, reports it as off its link, and advertises
// the next usable entry. The same address zoned to lan0 is the unit's own
// link and is advertised, and a unit bound to no device refuses both.
// PREVENTS: a LAN host told to query fe80::1 on its own link, where the WAN
// resolver is not.
//
// RFC requirement: RFC8106-5.1-2 positive -- a link-local resolver zoned to another device than the unit's is left out of the unit's RDNSS servers and reported off its link, while one zoned to the unit's own device is kept.
func TestRDNSSRefusesLinkLocalOfAnotherLink(t *testing.T) {
	cfg := mustParseRA(t, `{"enabled": "true"}`)
	wanLinkLocal := netip.MustParseAddr("fe80::1%wan0")
	lanLinkLocal := netip.MustParseAddr("fe80::2%lan0")
	global := netip.MustParseAddr("2001:db8::53")
	learned := []dnssource.Record{raLeaseRecord(time.Time{}, "fe80::1%wan0", "fe80::2%lan0", "2001:db8::53")}

	t.Run("lan0 unit", func(t *testing.T) {
		selection := raSelectRDNSS(cfg, "lan0", netip.Addr{}, learned, time.Now())
		assert.Equal(t, []netip.Addr{lanLinkLocal, global}, selection.servers)
		assert.Equal(t, []netip.Addr{wanLinkLocal}, selection.offLink)
		assert.Empty(t, selection.dropped, "a zoned link-local is not host-only")
	})
	t.Run("wan0 unit", func(t *testing.T) {
		selection := raSelectRDNSS(cfg, "wan0", netip.Addr{}, learned, time.Now())
		assert.Equal(t, []netip.Addr{wanLinkLocal, global}, selection.servers)
		assert.Equal(t, []netip.Addr{lanLinkLocal}, selection.offLink)
	})
	t.Run("unit bound to no device", func(t *testing.T) {
		selection := raSelectRDNSS(cfg, "", netip.Addr{}, learned, time.Now())
		assert.Equal(t, []netip.Addr{global}, selection.servers)
		assert.Equal(t, []netip.Addr{wanLinkLocal, lanLinkLocal}, selection.offLink)
	})
	t.Run("configured link-local of another link", func(t *testing.T) {
		configured := mustParseRA(t, `{"enabled": "true", "rdnss": {"server": ["fe80::9%wan0", "2001:db8::99"]}}`)
		selection := raSelectRDNSS(configured, "lan0", netip.Addr{}, nil, time.Now())
		assert.Equal(t, []netip.Addr{netip.MustParseAddr("2001:db8::99")}, selection.servers)
		assert.Equal(t, []netip.Addr{netip.MustParseAddr("fe80::9%wan0")}, selection.offLink)
	})
}
