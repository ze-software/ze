package ifacera

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/ndp"
)

// rdnssTestLifetime is the unit lifetime every case advertises.
const rdnssTestLifetime = 1800

func rdnssSpec(expires time.Time, servers ...string) iface.RASenderSpec {
	spec := iface.RASenderSpec{
		Advertisement: ndp.RAConfig{RouterLifetime: 1800, RDNSSLifetime: rdnssTestLifetime},
		RDNSSExpires:  expires,
	}
	for _, s := range servers {
		spec.Advertisement.RDNSS = append(spec.Advertisement.RDNSS, netip.MustParseAddr(s))
	}
	return spec
}

// VALIDATES: AC-8 and AC-15, the withdrawal half (RFC 8106 Section 5.1: "A
// value of zero means that the RDNSS addresses MUST no longer be used."). A
// resolver an update drops is sent once in an RDNSS option of Lifetime 0, at
// once (urgent), beside the servers still advertised, and not again after.
// PREVENTS: a host keeping a withdrawn resolver until its last advertised
// lifetime runs out, and a withdrawal that repeats forever.
func TestRDNSSWithdrawnAdvertisedLifetimeZero(t *testing.T) {
	now := time.Now()
	adv := &advertiser{spec: rdnssSpec(time.Time{}, "2001:db8::53")}
	adv.sent(adv.build(now, false))

	adv.update(rdnssSpec(time.Time{}, "2001:db8:1::53"))
	if !adv.urgent(now, false) {
		t.Fatal("a withdrawn resolver must be sent before the periodic schedule")
	}
	cfg := adv.build(now, false)
	if !slices.Equal(cfg.RDNSS, []netip.Addr{netip.MustParseAddr("2001:db8:1::53")}) || cfg.RDNSSLifetime != 1800 {
		t.Fatalf("live RDNSS = %v lifetime %d, want [2001:db8:1::53] 1800", cfg.RDNSS, cfg.RDNSSLifetime)
	}
	if !slices.Equal(cfg.RDNSSWithdrawn, []netip.Addr{netip.MustParseAddr("2001:db8::53")}) {
		t.Fatalf("withdrawn = %v, want [2001:db8::53]", cfg.RDNSSWithdrawn)
	}
	adv.sent(cfg)
	if next := adv.build(now, false); len(next.RDNSSWithdrawn) != 0 {
		t.Fatalf("withdrawal repeated: %v", next.RDNSSWithdrawn)
	}

	// The last resolver withdrawn, with no other source: one zero-lifetime option.
	adv.update(rdnssSpec(time.Time{}))
	cfg = adv.build(now, false)
	if len(cfg.RDNSS) != 0 || !slices.Equal(cfg.RDNSSWithdrawn, []netip.Addr{netip.MustParseAddr("2001:db8:1::53")}) {
		t.Fatalf("live %v withdrawn %v, want none and [2001:db8:1::53]", cfg.RDNSS, cfg.RDNSSWithdrawn)
	}
	buf := make([]byte, ndp.RALen(cfg))
	if n := ndp.BuildRA(buf, 0, cfg); n != len(buf) {
		t.Fatalf("BuildRA wrote %d of %d octets", n, len(buf))
	}

	// A resolver dropped and advertised again in one step is live, not withdrawn.
	adv.sent(cfg)
	adv.update(rdnssSpec(time.Time{}, "2001:db8::53"))
	adv.update(rdnssSpec(time.Time{}, "2001:db8::54"))
	adv.update(rdnssSpec(time.Time{}, "2001:db8::53", "2001:db8::54"))
	if got := adv.build(now, false).RDNSSWithdrawn; len(got) != 0 {
		t.Fatalf("a re-advertised resolver was withdrawn: %v", got)
	}
}

// VALIDATES: AC-8, the lifetime half. A learned resolver is advertised with a
// Lifetime no larger than what remains of its lease at each send, rounded
// down, and 0 once the lease has ended; a server with no end keeps the
// unit's lifetime.
// PREVENTS: a host told to use a resolver past the lease that supplied it.
func TestRDNSSLifetimeFollowsLease(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		expires time.Time
		want    uint32
	}{
		{name: "no end", expires: time.Time{}, want: 1800},
		{name: "lease longer than lifetime", expires: now.Add(time.Hour), want: 1800},
		{name: "lease shorter, rounded down", expires: now.Add(600*time.Second + 900*time.Millisecond), want: 600},
		{name: "lease ended", expires: now.Add(-time.Second), want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adv := &advertiser{spec: rdnssSpec(tt.expires, "2001:db8::53")}
			if got := adv.build(now, false).RDNSSLifetime; got != tt.want {
				t.Fatalf("RDNSS lifetime = %d, want %d", got, tt.want)
			}
		})
	}
}
