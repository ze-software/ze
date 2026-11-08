//go:build linux

package ifacera

import (
	"bytes"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/ndp"
)

func testAdvertisement() ndp.RAConfig {
	return ndp.RAConfig{
		CurHopLimit:    64,
		Managed:        true,
		RouterLifetime: 1800,
		Prefixes: []ndp.PrefixInfo{
			{Prefix: netip.MustParsePrefix("2001:db8:1::/64"), OnLink: true, Autonomous: true, ValidLifetime: 7200, PreferredLifetime: 3600},
		},
		RDNSS:         []netip.Addr{netip.MustParseAddr("2001:4860:4860::8888")},
		RDNSSLifetime: 1800,
	}
}

// VALIDATES: spec AC-5 and RFC 4861 Section 6.2.5. A sender that stops sends
// ONE advertisement carrying Router Lifetime 0, and keeps the rest of the
// advertisement so hosts read one consistent message.
// PREVENTS: hosts keeping Ze in their default router list for the whole router
// lifetime after it stopped advertising, and the return of the three-in-one-tick
// burst that Section 6.2.6's rate limit reads as consecutive multicast
// advertisements sent inside MIN_DELAY_BETWEEN_RAS.
func TestRAFinalZeroLifetime(t *testing.T) {
	t.Run("one final advertisement retires the router", func(t *testing.T) {
		var sent []ndp.RAConfig
		var solicited []bool
		s := &Sender{}
		s.sendFinal(func(cfg ndp.RAConfig, sol bool) {
			sent = append(sent, cfg)
			solicited = append(solicited, sol)
		}, testAdvertisement(), false, time.Time{})

		// One, not a burst. Section 6.2.5 permits one or more, and one leaves
		// no pair of consecutive multicast advertisements for Section 6.2.6's
		// rate limit to bind. radvd, FRR and BIRD each send one.
		require.Len(t, sent, 1)
		for i, cfg := range sent {
			assert.Equal(t, uint16(0), cfg.RouterLifetime, "final advertisement %d must carry Router Lifetime 0", i)
			assert.False(t, solicited[i], "a final advertisement answers no solicitation")
			// The rest of the message is unchanged: RFC 4861 Section 4.2 says
			// the Router Lifetime applies only to default-router usefulness.
			assert.Equal(t, uint8(64), cfg.CurHopLimit)
			assert.True(t, cfg.Managed)
			assert.Len(t, cfg.Prefixes, 1)
			assert.Len(t, cfg.RDNSS, 1)
		}
	})

	t.Run("a teardown inside the window waits the rate limit out", func(t *testing.T) {
		// RFC 4861 Section 6.2.6 rate limits consecutive multicast
		// advertisements to one every MIN_DELAY_BETWEEN_RAS, and Ze reads that
		// as reaching the final one. lastSent is placed so the remainder is
		// short enough for a unit test and long enough to measure.
		const remaining = 80 * time.Millisecond
		lastSent := time.Now().Add(remaining - ndp.MinDelayBetweenRAs)

		count := 0
		start := time.Now()
		s := &Sender{}
		s.sendFinal(func(ndp.RAConfig, bool) { count++ }, testAdvertisement(), false, lastSent)

		assert.Equal(t, 1, count, "the teardown still sends its one advertisement")
		assert.GreaterOrEqual(t, time.Since(start), remaining/2,
			"the final advertisement left before the rate-limit window closed")
	})

	t.Run("nothing is sent while the link is down", func(t *testing.T) {
		count := 0
		s := &Sender{}
		s.sendFinal(func(ndp.RAConfig, bool) { count++ }, testAdvertisement(), true, time.Time{})
		assert.Zero(t, count, "a down link carries nothing, so no final advertisement is attempted")
	})
}

// VALIDATES: zeroLifetime changes the Router Lifetime and nothing else, and
// leaves the caller's advertisement untouched.
// PREVENTS: the final advertisement mutating the periodic one, which would
// make every later advertisement retire the router.
func TestZeroLifetimeLeavesTheSourceAlone(t *testing.T) {
	original := testAdvertisement()
	final := zeroLifetime(original)

	assert.Equal(t, uint16(0), final.RouterLifetime)
	assert.Equal(t, uint16(1800), original.RouterLifetime, "the periodic advertisement must not change")
	assert.Equal(t, original.Prefixes, final.Prefixes)
	assert.Equal(t, original.RDNSS, final.RDNSS)
	assert.Equal(t, original.CurHopLimit, final.CurHopLimit)
}

// VALIDATES: the Source Link-layer Address option is filled from the resolver's
// hardware address, and left out when there is none or it is not six octets
// (RFC 4861 Section 4.6.1 and Section 4.2, which lets a router omit it).
// PREVENTS: a malformed option on a link whose address is not an IEEE 802 one,
// which makes a receiver mis-parse every option after it.
func TestParseMAC(t *testing.T) {
	tests := []struct {
		name string
		mac  string
		want []byte
	}{
		{"IEEE 802 address", "02:11:22:33:44:55", []byte{0x02, 0x11, 0x22, 0x33, 0x44, 0x55}},
		{"no address", "", nil},
		{"not an address", "not-a-mac", nil},
		{"infiniband address is not six octets", "00:11:22:33:44:55:66:77:88:99:aa:bb:cc:dd:ee:ff:00:11:22:33", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMAC(tt.mac)
			if tt.want == nil {
				// A 20-octet address parses, and the encoder drops it because
				// it does not fit one 8-octet option unit.
				cfg := ndp.RAConfig{SourceLinkLayerAddress: got}
				assert.Equal(t, 16, ndp.RALen(cfg), "an address that is not six octets emits no option")
				return
			}
			assert.Equal(t, tt.want, got)
			cfg := ndp.RAConfig{SourceLinkLayerAddress: got}
			assert.Equal(t, 24, ndp.RALen(cfg), "a six-octet address emits one 8-octet option")
		})
	}
}

// VALIDATES: a WAN the CE unit names that does not resolve is reported at WARN,
// naming the LAN and the WAN, and once only while it keeps failing with the
// same error; it still reads as holding no default route.
// METHOD: ask the resolver three times about a WAN no device carries, with a
// text logger, and count the WARN lines.
// PREVENTS: a Router Lifetime held at 0 with nothing in the log saying why,
// which is what the silent skip of an unresolvable WAN produced.
func TestWANResolverWarnsOnceForAnUnresolvableWAN(t *testing.T) {
	var out bytes.Buffer
	resolver := newWANResolver("lan0", slog.New(slog.NewTextHandler(&out, nil)))
	defaults := newWANDefaults()
	defaults.seed(nil)

	for range 3 {
		if resolver.holdsDefault([]string{"zenowan0"}, defaults) {
			t.Fatal("a WAN that does not resolve must not hold a default route")
		}
	}
	logged := out.String()
	if n := strings.Count(logged, "level=WARN"); n != 1 {
		t.Fatalf("want one WARN for the unresolvable WAN, got %d:\n%s", n, logged)
	}
	for _, want := range []string{"iface=lan0", "wan=zenowan0", "Router Lifetime 0"} {
		if !strings.Contains(logged, want) {
			t.Fatalf("the WARN does not carry %q:\n%s", want, logged)
		}
	}
}
