// Related: ra.go -- the encoder whose wire output this file pins.

package ppp

import (
	"encoding/hex"
	"net/netip"
	"testing"
)

// VALIDATES: ppp.BuildRA emits exactly the bytes the BNG subscriber path put on
// the wire before the encoder moved to internal/core/ndp (spec AC-10).
// PREVENTS: a silent wire change for live BNG subscribers when the encoder is
// extracted, delegated, or refactored.
//
// The golden strings are derived by hand from the RFC field layouts, NOT
// captured from BuildRA. Capturing the current output would make the test agree
// with whatever the code does; deriving it makes the test an independent
// statement about the wire.
//
// RA header, RFC 4861 Section 4.2:
//
//	octet 0      Type = 134 (0x86)
//	octet 1      Code = 0
//	octets 2-3   Checksum (kernel computes it for raw sockets, sent as 0)
//	octet 4      Cur Hop Limit
//	octet 5      M (0x80) | O (0x40) | 6-bit Reserved (zero)
//	octets 6-7   Router Lifetime, seconds
//	octets 8-11  Reachable Time, milliseconds
//	octets 12-15 Retrans Timer, milliseconds
//
// RDNSS option, RFC 8106 Section 5.1:
//
//	octet 0      Type = 25 (0x19)
//	octet 1      Length in 8-octet units: 1 + 2*addresses
//	octets 2-3   Reserved (zero)
//	octets 4-7   Lifetime, seconds
//	octets 8+    16 octets per resolver address
func TestBuildRAParity(t *testing.T) {
	dns1 := netip.MustParseAddr("2001:4860:4860::8888")
	dns2 := netip.MustParseAddr("2001:4860:4860::8844")

	tests := []struct {
		name string
		cfg  RAConfig
		want string
	}{
		{
			// The config raSenderLoop (ra_linux.go) actually sends: hop limit 64,
			// M and O set, router lifetime 1800, no prefix, no RDNSS.
			name: "bng fixed config",
			cfg: RAConfig{
				CurHopLimit:    64,
				Managed:        true,
				OtherConfig:    true,
				RouterLifetime: 1800,
			},
			//    type code chksum hop  flags  lifetime reachable   retrans
			want: "8600" + "0000" + "40" + "c0" + "0708" + "00000000" + "00000000",
		},
		{
			name: "managed only, no other-config",
			cfg: RAConfig{
				CurHopLimit:    64,
				Managed:        true,
				RouterLifetime: 600,
			},
			want: "8600" + "0000" + "40" + "80" + "0258" + "00000000" + "00000000",
		},
		{
			name: "no flags, zero hop limit means unspecified",
			cfg: RAConfig{
				RouterLifetime: 600,
			},
			want: "8600" + "0000" + "00" + "00" + "0258" + "00000000" + "00000000",
		},
		{
			name: "reachable and retransmit timers carried",
			cfg: RAConfig{
				CurHopLimit:    64,
				OtherConfig:    true,
				RouterLifetime: 1800,
				ReachableTime:  30000,
				RetransTimer:   1000,
			},
			want: "8600" + "0000" + "40" + "40" + "0708" + "00007530" + "000003e8",
		},
		{
			name: "one RDNSS resolver",
			cfg: RAConfig{
				CurHopLimit:    64,
				Managed:        true,
				OtherConfig:    true,
				RouterLifetime: 1800,
				RDNSS:          []netip.Addr{dns1},
				RDNSSLifetime:  3600,
			},
			want: "8600" + "0000" + "40" + "c0" + "0708" + "00000000" + "00000000" +
				// RDNSS: type 25, length 3 (one address), reserved, lifetime 3600
				"19" + "03" + "0000" + "00000e10" +
				"20014860486000000000000000008888",
		},
		{
			name: "two RDNSS resolvers share one option",
			cfg: RAConfig{
				CurHopLimit:    64,
				RouterLifetime: 1800,
				RDNSS:          []netip.Addr{dns1, dns2},
				RDNSSLifetime:  0,
			},
			want: "8600" + "0000" + "40" + "00" + "0708" + "00000000" + "00000000" +
				// RDNSS: length 5 (two addresses), lifetime 0 means stop using them
				"19" + "05" + "0000" + "00000000" +
				"20014860486000000000000000008888" +
				"20014860486000000000000000008844",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf [256]byte
			n := BuildRA(buf[:], tt.cfg)
			got := hex.EncodeToString(buf[:n])
			if got != tt.want {
				t.Errorf("BuildRA wire bytes changed\n got %s\nwant %s", got, tt.want)
			}
			if want := len(tt.want) / 2; n != want {
				t.Errorf("BuildRA returned %d bytes, want %d", n, want)
			}
		})
	}
}
