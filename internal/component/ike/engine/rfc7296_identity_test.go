// VALIDATES: the RFC 7296 Section 3.5 obligations on identification payloads -- which ID
// types an implementation must be configurable to SEND, which it must be configurable to
// ACCEPT, and the one more type an IPv6-capable implementation owes. Each test
// carries an `RFC requirement:` tag binding it to its checklist id.
// PREVENTS: a change that makes the sent ID type a constant rather than a consequence of
// `local-id`, or that drops one of the four types every implementation must accept.
package engine

import (
	"net"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// ridxSendSA builds an SA whose local identity is the configured string, so buildIDPayload
// selects the ID type from operator config alone.
func ridxSendSA(t *testing.T, localID string) *SA {
	t.Helper()
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.LocalID = localID
	return sa
}

// ridxAcceptSA builds a shared-key SA expecting the named remote identity. Shared key
// rather than certificate, because Section 3.5 is about the ID payload itself and a
// certificate would add a second reason for a refusal.
func ridxAcceptSA(t *testing.T, remoteID string, idType uint8, data []byte) *SA {
	t.Helper()
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
	sa.PeerCfg.Auth.PSK = "shared-secret"
	sa.PeerCfg.Auth.RemoteID = remoteID
	ridAssert(sa, idType, data)
	return sa
}

// RFC requirement: RFC7296-3.5-2 positive -- "To assure maximum interoperability,
// implementations MUST be configurable to send at least one of ID_IPV4_ADDR, ID_FQDN,
// ID_RFC822_ADDR, or ID_KEY_ID" (rfc/full/rfc7296.txt:5108-5112, the MUST at :5110-5111).
//
// The obligation is a DISJUNCTION over four types and one disjunct satisfies it. Ze is
// configurable to send ID_IPV4_ADDR (an IPv4 literal in `local-id`) and ID_FQDN (anything
// that is not an IP literal), both selected by encodeIKEID from the operator's leaf.
//
// NOT claimed by this test: the SHOULD immediately following at :5113-5114,
// "Implementations SHOULD be capable of generating and accepting all of these types."
// encodeIKEID has exactly three returns and none produces ID_RFC822_ADDR, ID_KEY_ID or
// ID_DER_ASN1_DN, so an email-shaped `local-id` goes out as ID_FQDN. That fails the
// SHOULD, not this MUST. Do not read a pass here as proof of the stronger sentence.
//
// A Ze-specific constraint belongs in the record, because a reader will otherwise discover
// it as a contradiction. In X.509 mode ValidatePKIRefs
// (internal/component/ike/ipsec/validate.go, ValidatePKIRefs) refuses a `local-id` that is
// not the certificate's common name. The reachable type is therefore bounded by the
// certificate. PSK and EAP modes are unconstrained. The MUST still holds.
//
// The IDData assertion is byte-for-byte on purpose: an implementation that picked the
// right type and sent the wrong octets would interoperate with nobody.
func TestLocalIDTypeFollowsConfiguredIdentity(t *testing.T) {
	cases := []struct {
		localID  string
		wantType uint8
		wantData []byte
	}{
		{"10.0.0.1", wire.IDTypeIPv4Addr, net.IPv4(10, 0, 0, 1).To4()},
		{"192.0.2.42", wire.IDTypeIPv4Addr, net.IPv4(192, 0, 2, 42).To4()},
		{"2001:db8::1", wire.IDTypeIPv6Addr, net.ParseIP("2001:db8::1").To16()},
		{"gw.example.com", wire.IDTypeFQDN, []byte("gw.example.com")},
	}
	for _, tc := range cases {
		t.Run(tc.localID, func(t *testing.T) {
			p := buildIDPayload(ridxSendSA(t, tc.localID), true)
			if p.IDType != tc.wantType {
				t.Fatalf("local-id %q sent ID type %d, want %d", tc.localID, p.IDType, tc.wantType)
			}
			if len(p.IDData) != len(tc.wantData) {
				t.Fatalf("local-id %q sent %d octets, want %d",
					tc.localID, len(p.IDData), len(tc.wantData))
			}
			for i := range tc.wantData {
				if p.IDData[i] != tc.wantData[i] {
					t.Fatalf("local-id %q octet %d = %#x, want %#x",
						tc.localID, i, p.IDData[i], tc.wantData[i])
				}
			}
		})
	}
}

// RFC requirement: RFC7296-3.5-2 negative -- the sent type is a CONSEQUENCE of config and
// not a constant. Without this, an assertion that "an IPv4 literal yields ID_IPV4_ADDR"
// would pass against an encodeIKEID that returned ID_IPV4_ADDR unconditionally for a
// fixture that happens to be an address.
//
// It also pins the fallback: with `local-id` unset buildIDPayload falls back to the peer
// NAME, so a peer named `10.0.0.1` silently sends ID_IPV4_ADDR. That is a genuine operator
// trap and it is worth a red test if it ever changes.
func TestLocalIDIsOperatorControlledNotDerived(t *testing.T) {
	// Two SAs differing ONLY in local-id must produce different types.
	v4 := buildIDPayload(ridxSendSA(t, "10.0.0.1"), true)
	fqdn := buildIDPayload(ridxSendSA(t, "gw.example.com"), true)
	if v4.IDType == fqdn.IDType {
		t.Fatalf("two local-id values produced the same ID type %d; the type is not "+
			"controlled by config", v4.IDType)
	}

	// The unset fallback is a real path, and it reaches the peer name.
	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth.LocalID = ""
	p := buildIDPayload(sa, true)
	if p.IDType != wire.IDTypeFQDN {
		t.Errorf("with local-id unset the payload type is %d, want ID_FQDN from the peer name",
			p.IDType)
	}
	if string(p.IDData) != sa.PeerName {
		t.Errorf("with local-id unset the payload carries %q, want the peer name %q",
			p.IDData, sa.PeerName)
	}

	// An address-shaped peer NAME takes the address branch, with no local-id involved.
	addrNamed := testSAWithKeys(t)
	addrNamed.PeerCfg.Auth.LocalID = ""
	addrNamed.PeerName = "10.0.0.1"
	if got := buildIDPayload(addrNamed, true); got.IDType != wire.IDTypeIPv4Addr {
		t.Errorf("a peer named 10.0.0.1 with no local-id sent type %d, want ID_IPV4_ADDR",
			got.IDType)
	}
}

// RFC requirement: RFC7296-3.5-3 positive -- "MUST be configurable to accept all of these
// four types" (rfc/full/rfc7296.txt:5112, continuing the sentence from :5110). "These four
// types" resolves to ID_IPV4_ADDR, ID_FQDN, ID_RFC822_ADDR and ID_KEY_ID (:5111).
//
// All four reach a real comparison in remoteIDMatches under the operator's `remote-id`
// leaf. ID_KEY_ID is exercised with an opaque non-UTF8 value so the exact-octet arm is
// genuinely taken and not accidentally satisfied by the case-folded text arm.
//
// The anti-vacuity guard is mandatory, and it is the reason this test proves anything.
// checkRemoteIdentity returns nil BEFORE any type test when `remote-id` is empty. A
// sub-test that forgot to set `remote-id` would then pass for every type, including the
// two Ze refuses. Each row therefore also asserts that a deliberately WRONG remote-id is
// refused.
//
// Cross-reference rather than duplication: TestRidComparesEveryIdentityTypeItSupports
// (remote_id_test.go) drives the same five types through the full verifyRemoteAuth path.
// This test is the binding for Section 3.5 and names the four MUST types explicitly.
func TestRemoteIDAcceptsEveryMandatoryType(t *testing.T) {
	opaqueKeyID := []byte{0x00, 0xff, 0x41}
	cases := []struct {
		name     string
		idType   uint8
		remoteID string
		match    []byte
		wrong    string
	}{
		{"ID_IPV4_ADDR", wire.IDTypeIPv4Addr, "172.28.0.3",
			net.IPv4(172, 28, 0, 3).To4(), "172.28.0.4"},
		{"ID_FQDN", wire.IDTypeFQDN, "vpn.example.com",
			[]byte("vpn.example.com"), "other.example.com"},
		{"ID_RFC822_ADDR", wire.IDTypeRFC822Addr, "user@example.com",
			[]byte("user@example.com"), "root@example.com"},
		{"ID_KEY_ID", wire.IDTypeKeyID, string(opaqueKeyID),
			opaqueKeyID, "branch-43"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok := ridxAcceptSA(t, tc.remoteID, tc.idType, tc.match)
			if err := checkRemoteIdentity(ok); err != nil {
				t.Fatalf("%s is a type every implementation must accept, and it was "+
					"refused: %v", tc.name, err)
			}

			// Anti-vacuity: the pass above must depend on the value, not on the check
			// being unreachable. checkRemoteIdentity short-circuits on an empty
			// remote-id, so without this row the whole test would be vacuous.
			bad := ridxAcceptSA(t, tc.wrong, tc.idType, tc.match)
			if err := checkRemoteIdentity(bad); err == nil {
				t.Errorf("%s: remote-id %q accepted an identity that does not match it; "+
					"the acceptance above proves nothing", tc.name, tc.wrong)
			}
		})
	}
}

// RFC requirement: RFC7296-3.5-3 negative -- acceptance is a property of the four types
// and not of a check that accepts everything. ID_DER_ASN1_DN (9) and ID_DER_ASN1_GN (10)
// are refused by the not-comparable branch of assertedIdentity rather than by a value
// mismatch. The refusal names the types Ze can compare.
//
// Section 4 of RFC 7296 requires PKIX acceptance where the ID is ID_DER_ASN1_DN
// (RFC7296-4-4). That is a different obligation on a different surface, and it is proven
// separately. ID_DER_ASN1_GN is required by neither.
func TestRemoteIDRefusesTypesItCannotCompare(t *testing.T) {
	for _, tc := range []struct {
		name   string
		idType uint8
	}{
		{"ID_DER_ASN1_GN", wire.IDTypeDERASN1GN},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa := ridxAcceptSA(t, "vpn.example.com", tc.idType, []byte{0x30, 0x00})
			err := checkRemoteIdentity(sa)
			if err == nil {
				t.Fatalf("%s authenticated, and ze cannot compare it", tc.name)
			}
			if !strings.Contains(err.Error(), "cannot compare") {
				t.Errorf("%s was refused for the wrong reason (want the not-comparable "+
					"branch, not a value mismatch): %v", tc.name, err)
			}
		})
	}
}

// RFC requirement: RFC7296-3.5-4 positive -- "IPv6-capable implementations MUST
// additionally be configurable to accept ID_IPV6_ADDR" (rfc/full/rfc7296.txt:5114-5115).
//
// The antecedent holds, and it is established from code rather than assumed. encodeIKEID
// emits ID_IPV6_ADDR for an IPv6 literal, and assertedAddr, assertedIdentity and
// remoteIDMatches all handle the type. Both ends of one configuration are asserted here,
// so "configurable" is proven for send and accept together.
//
// The MAY at :5115-5117 ("IPv6-only implementations MAY be configurable to send only
// ID_IPV6_ADDR instead of ID_IPV4_ADDR") is not claimed.
//
// One deliberate widening belongs in the record. assertedAddr returns addr.Unmap(), and
// remoteIDMatches unmaps the configured value too. A peer sending ::ffff:10.0.0.1 as a
// 16-octet ID_IPV6_ADDR therefore matches a remote-id of 10.0.0.1. That is documented at
// the head of remote_id.go and does not violate this row.
func TestRemoteIDAcceptsIPv6Identity(t *testing.T) {
	v6 := net.ParseIP("2001:db8::1").To16()

	// Accept half.
	sa := ridxAcceptSA(t, "2001:db8::1", wire.IDTypeIPv6Addr, v6)
	if err := checkRemoteIdentity(sa); err != nil {
		t.Fatalf("an ID_IPV6_ADDR identity was refused: %v", err)
	}
	// Anti-vacuity for the accept half.
	wrong := ridxAcceptSA(t, "2001:db8::2", wire.IDTypeIPv6Addr, v6)
	if err := checkRemoteIdentity(wrong); err == nil {
		t.Error("remote-id 2001:db8::2 accepted the identity 2001:db8::1")
	}

	// Send half: the same configuration reaches the wire as ID_IPV6_ADDR.
	p := buildIDPayload(ridxSendSA(t, "2001:db8::1"), true)
	if p.IDType != wire.IDTypeIPv6Addr {
		t.Errorf("local-id 2001:db8::1 sent ID type %d, want ID_IPV6_ADDR", p.IDType)
	}
	if len(p.IDData) != 16 {
		t.Errorf("ID_IPV6_ADDR carried %d octets, want 16", len(p.IDData))
	}
}

// RFC requirement: RFC7296-3.5-4 negative -- acceptance depends on the payload being a
// well-formed address of the asserted type, not on the type octet alone. RFC 7296
// Section 3.5 fixes ID_IPV6_ADDR at 16 octets and ID_IPV4_ADDR at 4, and assertedAddr
// enforces both. A truncated or over-long payload is therefore refused rather than
// silently zero-padded into a different address.
//
// The ID_IPV4_ADDR row pins the per-type length table rather than a single constant: a
// length check that tested only "16" would accept a 16-octet ID_IPV4_ADDR.
func TestIPv6IdentityLengthIsEnforced(t *testing.T) {
	for _, tc := range []struct {
		name   string
		idType uint8
		data   []byte
	}{
		{"ipv6 too short", wire.IDTypeIPv6Addr, make([]byte, 4)},
		{"ipv6 one short", wire.IDTypeIPv6Addr, make([]byte, 15)},
		{"ipv6 one long", wire.IDTypeIPv6Addr, make([]byte, 17)},
		{"ipv6 empty", wire.IDTypeIPv6Addr, nil},
		{"ipv4 at ipv6 length", wire.IDTypeIPv4Addr, make([]byte, 16)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa := ridxAcceptSA(t, "2001:db8::1", tc.idType, tc.data)
			err := checkRemoteIdentity(sa)
			if err == nil {
				t.Fatalf("a %d-octet payload of type %d authenticated", len(tc.data), tc.idType)
			}
			if !strings.Contains(err.Error(), "cannot compare") {
				t.Errorf("the refusal did not come from the not-comparable branch: %v", err)
			}
		})
	}
}
