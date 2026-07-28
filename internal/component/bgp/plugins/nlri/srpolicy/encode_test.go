package srpolicy

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestSRPolicyRouteEncoderRegistered verifies both SR-Policy families expose the
// owner-package route encoder through the plugin registry.
//
// VALIDATES: registry.RouteEncoderByFamily returns the SR-Policy route encoder
// for IPv4 and IPv6 SR-Policy.
// PREVENTS: SR-Policy staying decode/config-only while canonical route encoding
// reports unsupported family.
func TestSRPolicyRouteEncoderRegistered(t *testing.T) {
	for _, fam := range []string{"ipv4/sr-policy", "ipv6/sr-policy"} {
		if registry.RouteEncoderByFamily(fam) == nil {
			t.Fatalf("no route encoder registered for %s", fam)
		}
	}
}

// TestSRPolicyNLRIEncoderRegistered verifies NLRI-only SR-Policy key encoding is
// registered and produces the RFC 9830 NLRI bytes.
//
// VALIDATES: registry.EncodeNLRIByFamily reaches the owner-package SR-Policy NLRI
// encoder and encodes distinguisher, color, and endpoint.
// PREVENTS: encode-nlri RPC/update-text paths returning no NLRI encoder for a
// registered SR-Policy family.
func TestSRPolicyNLRIEncoderRegistered(t *testing.T) {
	got, err := registry.EncodeNLRIByFamily("ipv4/sr-policy", strings.Fields("distinguisher 0 color 100 endpoint 10.0.0.1"))
	if err != nil {
		t.Fatalf("EncodeNLRIByFamily returned error: %v", err)
	}
	if got != "6000000000000000640A000001" {
		t.Fatalf("NLRI = %s, want 6000000000000000640A000001", got)
	}
}

// TestSRPolicyEncodeIPv4 verifies canonical owner-package route encoding against
// the existing ExaBGP compatibility SR-Policy IPv4 UPDATE bytes.
//
// VALIDATES: IPv4 SR-Policy route encoding builds the same NLRI and Tunnel
// Encapsulation attribute as the compatibility path.
// PREVENTS: byte drift while wiring SR-Policy into the registry route encoder.
func TestSRPolicyEncodeIPv4(t *testing.T) {
	cmd := "distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 192.0.2.1 preference 100 binding-sid mpls 24000 segment-list weight 1 segment type-a mpls 16001"
	update, nlri, err := EncodeRoute(cmd, "ipv4/sr-policy", 65000, true, true, false)
	if err != nil {
		t.Fatalf("EncodeRoute returned error: %v", err)
	}

	wantUpdate := "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF006902000000524001010040020040050400000064800E1600014904C0000201006000000000000000640A000001C01728000F00240C060000000000640D06100005DC00008000110009060000000000010106000003E81000"
	if got := strings.ToUpper(hex.EncodeToString(update)); got != wantUpdate {
		t.Fatalf("update = %s, want %s", got, wantUpdate)
	}
	if got := strings.ToUpper(hex.EncodeToString(nlri)); got != "6000000000000000640A000001" {
		t.Fatalf("NLRI = %s, want 6000000000000000640A000001", got)
	}
}

// TestSRPolicyEncodeIPv6 verifies canonical owner-package route encoding against
// the existing ExaBGP compatibility SR-Policy IPv6 UPDATE bytes.
//
// VALIDATES: IPv6 SR-Policy route encoding carries an IPv6 endpoint and next-hop
// with SAFI 73 MP_REACH_NLRI.
// PREVENTS: IPv4-only assumptions in the SR-Policy route encoder.
func TestSRPolicyEncodeIPv6(t *testing.T) {
	cmd := "distinguisher 0 color 100 endpoint 2001:db8::1 next-hop 2001:db8::2 preference 100 segment-list weight 1 segment type-b srv6 2001:db8:1::1"
	update, nlri, err := EncodeRoute(cmd, "ipv6/sr-policy", 65000, true, true, false)
	if err != nil {
		t.Fatalf("EncodeRoute returned error: %v", err)
	}

	wantUpdate := "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0085020000006E4001010040020040050400000064800E2E0002491020010DB800000000000000000000000200C0000000000000006420010DB8000000000000000000000001C0172C000F00280C0600000000006480001D0009060000000000010D12000020010DB8000100000000000000000001"
	if got := strings.ToUpper(hex.EncodeToString(update)); got != wantUpdate {
		t.Fatalf("update = %s, want %s", got, wantUpdate)
	}
	if got := strings.ToUpper(hex.EncodeToString(nlri)); got != "C0000000000000006420010DB8000000000000000000000001" {
		t.Fatalf("NLRI = %s, want C0000000000000006420010DB8000000000000000000000001", got)
	}
}

// TestSRPolicyInteropExaBGPSubTLVBytes verifies Ze's sub-TLV encoding matches
// ExaBGP's byte oracle from tests/unit/test_sr_policy.py.
//
// VALIDATES: sub-TLV wire bytes are interoperable with ExaBGP.
// PREVENTS: encoding drift between Ze and ExaBGP (except the documented
// S-bit difference: Ze sets S=0 per RFC 9830 §2.4.4.2.1; ExaBGP sets S=1
// on is_last segments).
func TestSRPolicyInteropExaBGPSubTLVBytes(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want string // hex substring that must appear in the Tunnel Encap attribute value
	}{
		{
			"preference_100",
			// ExaBGP: PreferenceSubTLV(100).pack() == b'\x0c\x06\x00\x00\x00\x00\x00\x64'
			"distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 192.0.2.1 preference 100",
			"0C06000000000064",
		},
		{
			"priority_10",
			// ExaBGP: PrioritySubTLV(10).pack() == b'\x0f\x02\x0a\x00'
			"distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 192.0.2.1 priority 10",
			"0F020A00",
		},
		{
			"binding_sid_null",
			// ExaBGP: BindingSIDSubTLV(label=None).pack() == 4 bytes: type=0x0D, len=2, flags=0, reserved=0
			"distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 192.0.2.1 binding-sid null",
			"0D020000",
		},
		{
			"binding_sid_mpls_24000_s_bit_zero",
			// ExaBGP: BindingSIDSubTLV(24000).pack() == b'\x0d\x06\x10\x00\x05\xdc\x01\x00'
			// Ze differs: S-bit=0 per RFC 9830 → 05DC0000 (not 05DC0100).
			"distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 192.0.2.1 binding-sid mpls 24000",
			"0D06100005DC0000",
		},
		{
			"segment_type_a_16001_s_bit_zero",
			// ExaBGP: SegmentTypeA(16001).pack(is_last=False) == b'\x01\x06\x00\x00\x03\xe8\x10\x00'
			// Ze matches ExaBGP is_last=False (S=0); ExaBGP is_last=True would be 03E81100.
			"distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 192.0.2.1 segment-list weight 1 segment type-a mpls 16001",
			"0106000003E81000",
		},
		{
			"weight_1",
			// ExaBGP: WeightSubSubTLV(1).pack() == b'\x09\x06\x00\x00\x00\x00\x00\x01'
			"distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 192.0.2.1 segment-list weight 1 segment type-a mpls 16001",
			"0906000000000001",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			update, _, err := EncodeRoute(tt.cmd, "ipv4/sr-policy", 65000, true, true, false)
			if err != nil {
				t.Fatalf("EncodeRoute: %v", err)
			}
			got := strings.ToUpper(hex.EncodeToString(update))
			if !strings.Contains(got, tt.want) {
				t.Fatalf("update hex does not contain %s\ngot: %s", tt.want, got)
			}
		})
	}
}

// TestSRPolicyEncodeWithPriority verifies a full UPDATE with priority sub-TLV.
//
// VALIDATES: priority keyword produces correct wire bytes in a complete UPDATE.
// PREVENTS: priority parsed but not encoded.
func TestSRPolicyEncodeWithPriority(t *testing.T) {
	cmd := "distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 192.0.2.1 preference 100 priority 10 binding-sid mpls 24000 segment-list weight 1 segment type-a mpls 16001"
	update, _, err := EncodeRoute(cmd, "ipv4/sr-policy", 65000, true, true, false)
	if err != nil {
		t.Fatalf("EncodeRoute: %v", err)
	}
	got := strings.ToUpper(hex.EncodeToString(update))
	// Verify all sub-TLVs present: preference, binding-sid, priority, segment-list.
	for _, sub := range []string{
		"0C06000000000064", // preference 100
		"0D06100005DC0000", // binding-sid mpls 24000 (S=0)
		"0F020A00",         // priority 10
		"0106000003E81000", // segment type-a 16001 (S=0)
	} {
		if !strings.Contains(got, sub) {
			t.Errorf("missing sub-TLV %s in:\n%s", sub, got)
		}
	}
}

// TestSRPolicyEncodeBindingSIDNull verifies binding-sid null produces a 2-byte
// value (flags + reserved only, no MPLS label).
//
// VALIDATES: binding-sid null interop with ExaBGP.
// PREVENTS: binding-sid null rejected or encoded with a label.
func TestSRPolicyEncodeBindingSIDNull(t *testing.T) {
	cmd := "distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 192.0.2.1 binding-sid null"
	update, _, err := EncodeRoute(cmd, "ipv4/sr-policy", 65000, true, true, false)
	if err != nil {
		t.Fatalf("EncodeRoute: %v", err)
	}
	got := strings.ToUpper(hex.EncodeToString(update))
	if !strings.Contains(got, "0D020000") {
		t.Fatalf("binding-sid null sub-TLV not found in:\n%s", got)
	}
	// Must NOT contain a 6-byte binding-sid (with MPLS label).
	if strings.Contains(got, "0D061000") {
		t.Fatal("binding-sid null should not contain MPLS label bytes")
	}
}

// TestSRPolicyEncodeRejectsInvalidInput verifies bad SR-Policy route and NLRI
// input fails at the owner-package parser boundary.
//
// VALIDATES: invalid SR-Policy encode inputs name the missing or offending token.
// PREVENTS: malformed SR-Policy commands emitting partial UPDATE or NLRI bytes.
func TestSRPolicyEncodeRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown", args: strings.Fields("distinguisher 0 color 100 endpoint 10.0.0.1 bogus value"), want: "bogus"},
		{name: "dangling", args: strings.Fields("distinguisher 0 color 100 endpoint"), want: "endpoint"},
		{name: "missing color", args: strings.Fields("distinguisher 0 endpoint 10.0.0.1"), want: "requires distinguisher, color, and endpoint"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := EncodeNLRIHex("ipv4/sr-policy", tt.args); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("EncodeNLRIHex error = %v, want token %q", err, tt.want)
			}
		})
	}

	if _, _, err := EncodeRoute("distinguisher 0 color 100 endpoint 10.0.0.1", "ipv4/sr-policy", 65000, true, true, false); err == nil || !strings.Contains(err.Error(), "next-hop") {
		t.Fatalf("EncodeRoute missing next-hop error = %v, want next-hop", err)
	}
}
