// Design: docs/architecture/route-selection.md -- next-hop admission before ranking
// Related: terminal_cover_fixture.go -- parseTerminalCover, IPv4 /32 and IPv6 /128 covers

package fixture

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// VALIDATES: parseTerminalCover maps plain IPv4 addresses to AFI 1 / SAFI 1 /32
// and plain IPv6 addresses to AFI 2 / SAFI 1 /128, and rejects malformed,
// zoned, or IPv4-mapped IPv6 inputs.
// PREVENTS: silent IPv4-only /32 suffixing breaking IPv6 RFC 5549 covers or
// accepting invalid next-hop literals in fixtures.
func TestParseTerminalCover(t *testing.T) {
	t.Parallel()

	v4, err := parseTerminalCover("test", "10.0.0.1")
	if err != nil {
		t.Fatalf("parse IPv4 cover: %v", err)
	}
	wantV4 := terminalCoverSpec{afi: 1, safi: 1, family: family.IPv4Unicast, prefix: "10.0.0.1/32"}
	if v4 != wantV4 {
		t.Fatalf("IPv4 spec = %+v, want %+v", v4, wantV4)
	}

	v6, err := parseTerminalCover("test", "2001:db8::1")
	if err != nil {
		t.Fatalf("parse IPv6 cover: %v", err)
	}
	wantV6 := terminalCoverSpec{afi: 2, safi: 1, family: family.IPv6Unicast, prefix: "2001:db8::1/128"}
	if v6 != wantV6 {
		t.Fatalf("IPv6 spec = %+v, want %+v", v6, wantV6)
	}

	for _, bad := range []string{"", "not-an-ip", "10.0.0.1/32", "fe80::1%eth0", "::ffff:10.0.0.1"} {
		if _, err := parseTerminalCover("test", bad); err == nil {
			t.Errorf("parseTerminalCover(%q) succeeded, want error", bad)
		}
	}
}
