// VALIDATES: spec-ospfv3-2-wire AC-11 -- the AS-External-LSA round-trips the E/F/T
// flags, the metric, the prefix, and the optional Forwarding Address / External
// Route Tag / Referenced Link State ID for every flag combination, in RFC order.
// PREVENTS: reading the optional fields in the wrong order or mis-gating them, AND
// the (fixed) wire-layout bug where the E/F/T flags were placed at body offset 6
// and the Referenced LS Type was truncated to 8 bits. RFC 5340 §A.4.7 puts the
// flags in byte 0 (sharing the 32-bit word with the 24-bit Metric) and the
// Referenced LS Type in a 16-bit field at offset 6; the golden vector below and a
// 16-bit Referenced LS Type value (which an 8-bit field could not hold) lock that.

package packet

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

func TestOSPFv3ExternalOptionalFields(t *testing.T) {
	fwd := [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}
	refLSID := [4]byte{0, 0, 0, 7}

	cases := []struct {
		name string
		body ExternalLSA
	}{
		{"no optionals", ExternalLSA{ExternalType2: false, Metric: 0x000064, Prefix: makePrefix(t, 48, 0, 0)}},
		{"type2 only", ExternalLSA{ExternalType2: true, Metric: 0x00ffff, Prefix: makePrefix(t, 64, types.OptPrefixNU, 0)}},
		{"forwarding addr", ExternalLSA{ExternalType2: true, Metric: 1, Prefix: makePrefix(t, 64, 0, 0), HasForwardingAddr: true, ForwardingAddr: fwd}},
		{"route tag", ExternalLSA{Metric: 2, Prefix: makePrefix(t, 32, 0, 0), HasRouteTag: true, ExternalRouteTag: 0xfeedcafe}},
		// 16-bit Referenced LS Type (0x2003 Inter-Area-Prefix) -- an 8-bit field could
		// not represent it, so this case proves the field is a full 16 bits at offset 6.
		{"referenced lsid", ExternalLSA{Metric: 3, Prefix: makePrefix(t, 96, 0, 0), ReferencedLSType: types.LSTypeInterAreaPrefix, ReferencedLSID: refLSID}},
		{"all optionals", ExternalLSA{
			ExternalType2: true, Metric: 0x0c0c0c, Prefix: makePrefix(t, 128, types.OptPrefixLA, 0),
			HasForwardingAddr: true, ForwardingAddr: fwd,
			HasRouteTag: true, ExternalRouteTag: 0x11223344,
			ReferencedLSType: types.LSTypeIntraAreaPrefix, ReferencedLSID: refLSID,
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lsa := LSA{Header: sampleLSAHeader(t, types.LSTypeASExternal, "0.0.0.1"), External: &tc.body}
			wire := encodeLSA(t, lsa)

			decoded, err := DecodeLSA(wire)
			if err != nil {
				t.Fatalf("DecodeLSA external: %v", err)
			}
			got, err := decoded.DecodeExternal()
			if err != nil {
				t.Fatalf("DecodeExternal: %v", err)
			}
			if got.ExternalType2 != tc.body.ExternalType2 || got.Metric != tc.body.Metric {
				t.Fatalf("E/metric: got %v/%#x want %v/%#x", got.ExternalType2, got.Metric, tc.body.ExternalType2, tc.body.Metric)
			}
			assertPrefixEqual(t, got.Prefix, tc.body.Prefix)
			if got.HasForwardingAddr != tc.body.HasForwardingAddr || got.ForwardingAddr != tc.body.ForwardingAddr {
				t.Fatalf("forwarding addr: got %v/%x want %v/%x", got.HasForwardingAddr, got.ForwardingAddr, tc.body.HasForwardingAddr, tc.body.ForwardingAddr)
			}
			if got.HasRouteTag != tc.body.HasRouteTag || got.ExternalRouteTag != tc.body.ExternalRouteTag {
				t.Fatalf("route tag: got %v/%#x want %v/%#x", got.HasRouteTag, got.ExternalRouteTag, tc.body.HasRouteTag, tc.body.ExternalRouteTag)
			}
			if got.ReferencedLSType != tc.body.ReferencedLSType || got.ReferencedLSID != tc.body.ReferencedLSID {
				t.Fatalf("referenced: got %d/%v want %d/%v", got.ReferencedLSType, got.ReferencedLSID, tc.body.ReferencedLSType, tc.body.ReferencedLSID)
			}

			// Re-encode from the typed body must match the original wire bytes.
			reLSA := LSA{Header: decoded.Header, External: &got}
			out := encodeLSA(t, reLSA)
			if !bytes.Equal(out, wire) {
				t.Fatalf("external re-encode drift:\n got % x\nwant % x", out, wire)
			}
		})
	}
}

// TestOSPFv3ExternalGoldenWire locks the RFC 5340 §A.4.7 byte layout with a hardcoded
// vector: flags in byte 0 (E=0x04), 24-bit Metric in bytes 1-3, PrefixLength@4,
// PrefixOptions@5, the 16-bit Referenced LS Type@6-7, the AddressPrefix@8, then the
// 32-bit Referenced Link State ID. A round-trip test alone cannot catch a flags/offset
// or field-width error; this golden vector does.
func TestOSPFv3ExternalGoldenWire(t *testing.T) {
	body := ExternalLSA{
		ExternalType2:    true,
		Metric:           0x0000c8,
		Prefix:           makePrefix(t, 64, 0, 0),
		ReferencedLSType: types.LSTypeInterAreaPrefix, // 0x2003 -- needs 16 bits
		ReferencedLSID:   [4]byte{0, 0, 0, 7},
	}
	buf := make([]byte, body.EncodedLen())
	n := body.WriteTo(buf, 0)
	want := []byte{
		0x04,             // flags: E (no F, no T)
		0x00, 0x00, 0xc8, // 24-bit metric = 200
		0x40,       // PrefixLength 64
		0x00,       // PrefixOptions
		0x20, 0x03, // Referenced LS Type 0x2003 (16-bit)
		0x20, 0x01, 0x0d, 0xb8, 0x12, 0x34, 0x56, 0x78, // /64 AddressPrefix
		0x00, 0x00, 0x00, 0x07, // Referenced Link State ID
	}
	if n != len(want) || !bytes.Equal(buf[:n], want) {
		t.Fatalf("external golden:\n got % x\nwant % x", buf[:n], want)
	}
	got, err := DecodeExternalLSA(want)
	if err != nil {
		t.Fatalf("decode golden: %v", err)
	}
	if !got.ExternalType2 || got.Metric != 0x0000c8 ||
		got.ReferencedLSType != types.LSTypeInterAreaPrefix || got.ReferencedLSID != [4]byte{0, 0, 0, 7} {
		t.Fatalf("decoded golden mismatch: %+v", got)
	}
}

// TestOSPFv3ExternalTruncated proves a crafted external claiming a Forwarding Address
// (F-bit) but ending before the 16-byte address is rejected, not panicked (AC-18 /
// Security Review "Crafted LSA Length").
func TestOSPFv3ExternalTruncated(t *testing.T) {
	body := ExternalLSA{
		Metric:            1,
		Prefix:            makePrefix(t, 0, 0, 0), // /0 -> no address bytes
		HasForwardingAddr: true,
		ForwardingAddr:    [16]byte{0x20, 0x01},
	}
	buf := make([]byte, body.EncodedLen())
	body.WriteTo(buf, 0)
	if _, err := DecodeExternalLSA(buf[:len(buf)-4]); !errors.Is(err, ErrTruncated) {
		t.Fatalf("truncated external: err = %v, want ErrTruncated", err)
	}
}
