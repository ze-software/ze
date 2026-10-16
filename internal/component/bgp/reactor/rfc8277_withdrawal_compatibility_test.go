// Design: docs/architecture/wire/nlri.md -- labeled withdrawal on send
// Overview: session_egress_normalize.go -- normalizeTransmittedUpdate, setWithdrawalCompatibility

package reactor

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// compatibilityBody builds an UPDATE body (RFC 4271 Section 4.3) whose one path
// attribute is code 14 or 15 with the given value: flags 0x80, one length octet.
func compatibilityBody(t *testing.T, code byte, valueHex string) []byte {
	t.Helper()
	value, err := hex.DecodeString(valueHex)
	if err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 0, 4+3+len(value))
	body = append(body, 0, 0)
	body = binary.BigEndian.AppendUint16(body, uint16(3+len(value)))
	body = append(body, 0x80, code, byte(len(value)))
	return append(body, value...)
}

// TestWithdrawalCompatibilityOnSend checks every withdrawn labeled NLRI leaves
// with Compatibility 0x800000, whatever label its producer wrote, and that
// nothing else in the body moves. Method: build MP_UNREACH_NLRI bodies the
// way an API withdrawal naming `label 1000` built them (0x003E81), run the
// egress step.
//
// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field SHOULD be
// set to 0x800000."
//
// The test compares the whole body with the expected wire.
//
// RFC requirement: RFC8277-2.4-2 positive -- the egress step sets the Compatibility field of every withdrawn labeled NLRI to 0x800000, for IPv4 and IPv6 under SAFI 4 and SAFI 128, with and without ADD-PATH and for both routes of one MP_UNREACH_NLRI, and a field already 0x800000 stays 0x800000.
// RFC requirement: RFC8277-2.4-2 negative -- the step rewrites nothing else: every octet outside the Compatibility field is unchanged, and a labeled MP_REACH_NLRI announcement, an IPv6 unicast withdrawal and a labeled withdrawal section whose framing does not parse leave the body byte for byte as it was.
func TestWithdrawalCompatibilityOnSend(t *testing.T) {
	addPath := bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{
		AddPathMode: map[capability.Family]capability.AddPathMode{
			{AFI: capability.AFIIPv6, SAFI: capability.SAFI(4)}:   capability.AddPathBoth,
			{AFI: capability.AFIIPv6, SAFI: capability.SAFI(128)}: capability.AddPathBoth,
		},
	}, bgpctx.DirectionSend)

	tests := []struct {
		name     string
		encoding *bgpctx.EncodingContext
		code     byte
		value    string
		want     string
	}{
		{
			// test/exabgp-compat/api/api-vpnv4.ci: withdraw route 1.4.0.0/16 rd 65000:1 label 1000.
			name:  "ipv4/mpls-vpn",
			code:  15,
			value: "000180" + "68" + "003E81" + "0000FDE800000001" + "0104",
			want:  "000180" + "68" + "800000" + "0000FDE800000001" + "0104",
		},
		{
			name:     "ipv6/mpls-vpn with ADD-PATH",
			encoding: addPath,
			code:     15,
			value:    "000280" + "00000009" + "78" + "000011" + "0000FDE800000001" + "20010DB8",
			want:     "000280" + "00000009" + "78" + "800000" + "0000FDE800000001" + "20010DB8",
		},
		{
			name:  "ipv4/mpls-label, two routes",
			code:  15,
			value: "000104" + "20" + "003E81" + "0A" + "30" + "000001" + "0A0101",
			want:  "000104" + "20" + "800000" + "0A" + "30" + "800000" + "0A0101",
		},
		{
			name:     "ipv6/mpls-label with ADD-PATH",
			encoding: addPath,
			code:     15,
			value:    "000204" + "00000007" + "58" + "0000F1" + "20010DB800000000",
			want:     "000204" + "00000007" + "58" + "800000" + "20010DB800000000",
		},
		{
			// Already conformant: the step writes the same octets.
			name:  "ipv4/mpls-label already 0x800000",
			code:  15,
			value: "000104" + "20" + "800000" + "0A",
			want:  "000104" + "20" + "800000" + "0A",
		},
		{
			// An announcement carries its label stack, never a Compatibility field.
			name:  "ipv4/mpls-label announcement untouched",
			code:  14,
			value: "000104" + "04" + "0A000001" + "00" + "20" + "003E81" + "0A",
			want:  "000104" + "04" + "0A000001" + "00" + "20" + "003E81" + "0A",
		},
		{
			// Unicast has no Compatibility field.
			name:  "ipv6/unicast withdrawal untouched",
			code:  15,
			value: "000201" + "40" + "20010DB800000000",
			want:  "000201" + "40" + "20010DB800000000",
		},
		{
			// The length octet runs past the section: framing is the validator's.
			name:  "malformed section untouched",
			code:  15,
			value: "000104" + "20" + "003E81" + "0A" + "40" + "003E81",
			want:  "000104" + "20" + "003E81" + "0A" + "40" + "003E81",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := compatibilityBody(t, tt.code, tt.value)
			want := compatibilityBody(t, tt.code, tt.want)
			normalizeTransmittedUpdate(body, tt.encoding)
			if !bytes.Equal(body, want) {
				t.Fatalf("body after egress:\n got %X\nwant %X", body, want)
			}
		})
	}
}

// TestWithdrawalCompatibilityEveryWriter sends the api-vpnv4.ci withdrawal,
// Compatibility 0x003E81, through each session writer an UPDATE funnels
// through: the raw diagnostic write, the parsed send, a filter override and the
// relay of received bytes.
//
// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field SHOULD be
// set to 0x800000."
//
// Each writer MUST put 0x800000 on the wire and leave the borrowed source
// bytes alone (egressWrite checks the second).
//
// RFC requirement: RFC8277-2.4-2 positive -- the api-vpnv4.ci VPN-IPv4 withdrawal sent with Compatibility 0x003E81 reaches the wire with 0x800000 and every other octet unchanged through each of the raw, parsed, filter-override and relay session writers.
func TestWithdrawalCompatibilityEveryWriter(t *testing.T) {
	in := compatibilityBody(t, 15, "000180"+"68"+"003E81"+"0000FDE800000001"+"0104")
	want := compatibilityBody(t, 15, "000180"+"68"+"800000"+"0000FDE800000001"+"0104")
	for _, rail := range egressRails {
		t.Run(rail, func(t *testing.T) {
			got := egressWrite(t, rail, in)
			if !bytes.Equal(got, want) {
				t.Fatalf("%s writer sent:\n got %X\nwant %X", rail, got, want)
			}
		})
	}
}

// TestWithdrawalCompatibilityAllocatesNothing checks the egress step costs no
// heap allocation for a labeled withdrawal, with and without ADD-PATH: it runs
// on every UPDATE a session writes. Method: testing.AllocsPerRun over
// normalizeTransmittedUpdate on a VPN-IPv4 and an ADD-PATH VPN-IPv6 withdrawal.
func TestWithdrawalCompatibilityAllocatesNothing(t *testing.T) {
	addPath := bgpctx.NewEncodingContext(nil, &capability.EncodingCaps{
		AddPathMode: map[capability.Family]capability.AddPathMode{
			{AFI: capability.AFIIPv6, SAFI: capability.SAFI(128)}: capability.AddPathBoth,
		},
	}, bgpctx.DirectionSend)
	for _, tt := range []struct {
		name     string
		encoding *bgpctx.EncodingContext
		value    string
	}{
		{name: "ipv4/mpls-vpn", value: "000180" + "68" + "003E81" + "0000FDE800000001" + "0104"},
		{name: "ipv6/mpls-vpn ADD-PATH", encoding: addPath, value: "000280" + "00000009" + "78" + "000011" + "0000FDE800000001" + "20010DB8"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := compatibilityBody(t, 15, tt.value)
			allocs := testing.AllocsPerRun(1000, func() {
				normalizeTransmittedUpdate(body, tt.encoding)
			})
			if allocs != 0 {
				t.Fatalf("egress step allocated %v times per labeled withdrawal, want 0", allocs)
			}
		})
	}
}

// BenchmarkWithdrawalCompatibility measures the egress walk over a two-route
// labeled unicast withdrawal.
func BenchmarkWithdrawalCompatibility(b *testing.B) {
	value, err := hex.DecodeString("000104" + "20" + "003E81" + "0A" + "30" + "000001" + "0A0101")
	if err != nil {
		b.Fatal(err)
	}
	body := make([]byte, 0, 4+3+len(value))
	body = append(body, 0, 0)
	body = binary.BigEndian.AppendUint16(body, uint16(3+len(value)))
	body = append(body, 0x80, 15, byte(len(value)))
	body = append(body, value...)
	b.ReportAllocs()
	for b.Loop() {
		normalizeTransmittedUpdate(body, nil)
	}
}
