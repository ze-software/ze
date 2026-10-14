package capability

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// capsTotaling returns Unknown capabilities whose Len sums to exactly octets
// (octets MUST be 0 or at least 2). Each TLV carries at most 255 value octets.
func capsTotaling(octets int) []Capability {
	var caps []Capability
	for octets > 0 {
		tlv := min(octets, 257)
		if octets-tlv == 1 {
			tlv--
		}
		caps = append(caps, &Unknown{code: 200, Data: bytes.Repeat([]byte{0xAB}, tlv-2)})
		octets -= tlv
	}
	return caps
}

// TestPackOptionalParams proves the one Optional Parameters packer over typed
// parameters.
//
// VALIDATES: the base form while the sum of 2 + value length is at most 255,
// the RFC 9072 two-octet form above it, the size error above 65535, raw
// parameters copied as given, and the result round-trips through
// ParseFromOptionalParams.
// PREVENTS: the D1 off-by-two (choosing the base form from the capability
// total rather than the field length) and a silently empty OPEN when the
// capabilities do not fit.
func TestPackOptionalParams(t *testing.T) {
	tests := []struct {
		name         string
		params       []OptionalParam
		wantOctets   int
		wantExtended bool
	}{
		{"empty", nil, 0, false},
		{"one capability", []OptionalParam{CapabilitiesParam([]Capability{&ASN4{ASN: 65001}})}, 8, false},
		{"several", []OptionalParam{CapabilitiesParam([]Capability{&ASN4{ASN: 65001}, &RouteRefresh{}})}, 10, false},
		{"capTotal 253", []OptionalParam{CapabilitiesParam(capsTotaling(253))}, 255, false},
		{"capTotal 254", []OptionalParam{CapabilitiesParam(capsTotaling(254))}, 257, true},
		{"capTotal 65532", []OptionalParam{CapabilitiesParam(capsTotaling(65532))}, 65535, true},
		{"two params sum 255", []OptionalParam{CapabilitiesParam(capsTotaling(126)), CapabilitiesParam(capsTotaling(125))}, 255, false},
		{"two params sum 256", []OptionalParam{CapabilitiesParam(capsTotaling(126)), CapabilitiesParam(capsTotaling(126))}, 258, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			octets, extended, err := PackOptionalParams(tt.params)
			if err != nil {
				t.Fatalf("PackOptionalParams: %v", err)
			}
			if len(octets) != tt.wantOctets {
				t.Fatalf("octets = %d, want %d", len(octets), tt.wantOctets)
			}
			if extended != tt.wantExtended {
				t.Fatalf("extended = %v, want %v", extended, tt.wantExtended)
			}
			var want []Capability
			for i := range tt.params {
				want = append(want, tt.params[i].capabilities...)
			}
			got, err := ParseFromOptionalParams(octets, extended)
			if err != nil {
				t.Fatalf("ParseFromOptionalParams: %v", err)
			}
			if len(got) != len(want) {
				t.Fatalf("decoded %d capabilities, want %d", len(got), len(want))
			}
			for i := range want {
				if got[i].Code() != want[i].Code() || got[i].Len() != want[i].Len() {
					t.Fatalf("capability %d = code %d len %d, want code %d len %d",
						i, got[i].Code(), got[i].Len(), want[i].Code(), want[i].Len())
				}
			}
		})
	}
}

// TestPackOptionalParamsRaw proves a raw parameter is copied as given beside a
// type-2 one, in both framings.
//
// VALIDATES: Type, length and octets of a non-capability parameter, read back
// octet by octet, below and above 255 octets of parameters.
// PREVENTS: a raw parameter reframed or dropped by the packer.
func TestPackOptionalParamsRaw(t *testing.T) {
	raw := []byte{0x01, 0x02, 0x03}

	small, extended, err := PackOptionalParams([]OptionalParam{
		CapabilitiesParam([]Capability{&RouteRefresh{}}), RawParam(9, raw),
	})
	if err != nil {
		t.Fatal(err)
	}
	if extended {
		t.Fatal("9 octets of parameters took the extended form")
	}
	if want := []byte{2, 2, 2, 0, 9, 3, 1, 2, 3}; !bytes.Equal(small, want) {
		t.Fatalf("base form = % x, want % x", small, want)
	}

	large, extended, err := PackOptionalParams([]OptionalParam{
		CapabilitiesParam(capsTotaling(300)), RawParam(9, raw),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !extended {
		t.Fatal("above 255 octets of parameters took the base form")
	}
	if got := binary.BigEndian.Uint16(large[1:3]); got != 300 {
		t.Fatalf("type-2 Parameter Length = %d, want 300", got)
	}
	tail := large[3+300:]
	if want := []byte{9, 0, 3, 1, 2, 3}; !bytes.Equal(tail, want) {
		t.Fatalf("raw parameter = % x, want % x", tail, want)
	}
}

// TestPackOptionalParamsZeroValue proves the documented zero-value contract:
// a Raw parameter of Type 0 with an empty value, written as 00 00.
func TestPackOptionalParamsZeroValue(t *testing.T) {
	octets, extended, err := PackOptionalParams([]OptionalParam{{}})
	if err != nil {
		t.Fatal(err)
	}
	if extended || !bytes.Equal(octets, []byte{0, 0}) {
		t.Fatalf("zero value = % x extended %v, want 00 00 base", octets, extended)
	}
}

// TestPackOptionalParamsTooLarge proves AC-1x: capabilities totalling 65533
// octets give an Extended Opt. Parm. Length of 65536, which no OPEN can carry.
//
// VALIDATES: the error names the size and the limit and wraps
// ErrOptionalParamsTooLarge; no octets are returned.
// PREVENTS: an OPEN sent with its capabilities silently missing.
func TestPackOptionalParamsTooLarge(t *testing.T) {
	octets, _, err := PackOptionalParams([]OptionalParam{CapabilitiesParam(capsTotaling(65533))})
	if !errors.Is(err, ErrOptionalParamsTooLarge) {
		t.Fatalf("err = %v, want ErrOptionalParamsTooLarge", err)
	}
	if octets != nil {
		t.Fatalf("octets returned beside the error: %d", len(octets))
	}
	if msg := err.Error(); !bytes.Contains([]byte(msg), []byte("65536 octets, limit 65535")) {
		t.Fatalf("error %q does not name the size and the limit", msg)
	}
}

// TestPackOptionalParamsOneAllocation proves the packer's output is the only
// allocation on the reactor's shape: the parameter list, one type-2 parameter
// over a capability list, built inside the measured closure per call.
func TestPackOptionalParamsOneAllocation(t *testing.T) {
	caps := []Capability{&ASN4{ASN: 65001}, &RouteRefresh{}, &Multiprotocol{AFI: 1, SAFI: 1}}
	allocs := testing.AllocsPerRun(100, func() {
		if _, _, err := PackOptionalParams([]OptionalParam{CapabilitiesParam(caps)}); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 1 {
		t.Fatalf("allocations = %v, want 1", allocs)
	}
}
