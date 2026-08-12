package rpki

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateValid verifies Valid state when origin AS matches a covering VRP.
//
// VALIDATES: Route with matching origin AS and prefix within maxLength is Valid.
// PREVENTS: Valid routes being classified as Invalid or NotFound.
//
// RFC requirement: RFC6811-2-1 positive -- the validation state is set to reflect the result of
// the lookup: a route whose origin AS and prefix length are authorized by a covering VRP is Valid.
func TestValidateValid(t *testing.T) {
	c := newROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	state := c.Validate("10.0.1.0/24", 65001)
	assert.Equal(t, ValidationValid, state)
}

// TestValidateInvalid verifies Invalid state when origin AS does not match.
//
// VALIDATES: Route with wrong origin AS but covered prefix is Invalid.
// PREVENTS: Invalid routes being accepted.
//
// RFC requirement: RFC6811-2-1 negative -- the state reflects the lookup rather than defaulting
// to Valid: the same prefix with a non-authorized origin AS (covered by a VRP that does not match)
// is Invalid, not Valid, so the state is derived from the lookup result.
func TestValidateInvalid(t *testing.T) {
	c := newROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	state := c.Validate("10.0.1.0/24", 65999)
	assert.Equal(t, ValidationInvalid, state)
}

// TestValidateNotFound verifies NotFound state when no VRP covers prefix.
//
// VALIDATES: Route with no covering VRP is NotFound.
// PREVENTS: Non-covered routes being marked Invalid.
func TestValidateNotFound(t *testing.T) {
	c := newROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	state := c.Validate("192.168.0.0/24", 65001)
	assert.Equal(t, ValidationNotFound, state)
}

// TestValidateMaxLengthExceeded verifies Invalid when prefix exceeds maxLength.
//
// VALIDATES: Route /25 is Invalid when VRP maxLength is /24.
// PREVENTS: Over-specific prefixes being accepted.
func TestValidateMaxLengthExceeded(t *testing.T) {
	c := newROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	state := c.Validate("10.0.1.0/25", 65001) // /25 > maxLen /24
	assert.Equal(t, ValidationInvalid, state)
}

// TestValidateAS0 verifies ASN=0 means "no AS authorized" (RFC 6483).
//
// VALIDATES: VRP with ASN=0 causes Invalid for any origin AS.
// PREVENTS: AS0 ROAs being treated as valid authorization.
func TestValidateAS0(t *testing.T) {
	c := newROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 0)) // AS0 = no AS authorized

	state := c.Validate("10.0.1.0/24", 65001)
	assert.Equal(t, ValidationInvalid, state)
}

// TestValidateOriginNone verifies AS_SET yields Invalid when covered.
//
// VALIDATES: OriginNone (from AS_SET) yields Invalid when VRPs exist.
// PREVENTS: AS_SET routes being accepted as Valid.
func TestValidateOriginNone(t *testing.T) {
	c := newROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	state := c.Validate("10.0.1.0/24", OriginNone)
	assert.Equal(t, ValidationInvalid, state)
}

// TestValidateFourOctetAS verifies origin validation compares the full 32-bit AS number.
//
// RFC requirement: RFC6811-3-2 positive -- validation supports four-octet AS numbers: a prefix
// originated by a 32-bit ASN authorized by a VRP carrying the same 32-bit ASN is Valid.
// RFC requirement: RFC6811-3-2 negative -- a four-octet origin AS that differs only in its high
// 16 bits (same low 16) is Invalid, so the ASN is compared as a full uint32 and not truncated to
// 16 bits: 4200000000 (0xFA56EA00) authorizes, 4200065536 (0xFA57EA00, same low 16) does not.
func TestValidateFourOctetAS(t *testing.T) {
	c := newROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 4200000000)) // 32-bit ASN authorizes 10.0.0.0/8

	assert.Equal(t, ValidationValid, c.Validate("10.0.1.0/24", 4200000000))
	assert.Equal(t, ValidationInvalid, c.Validate("10.0.1.0/24", 4200065536))
}

// TestValidateMultipleVRPsOneMatch verifies Valid when any VRP matches.
//
// VALIDATES: Multiple covering VRPs -- Valid if ANY matches.
// PREVENTS: First-VRP-only evaluation.
func TestValidateMultipleVRPsOneMatch(t *testing.T) {
	c := newROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))
	c.Add(makeVRP("10.0.0.0/8", 24, 65002))

	state := c.Validate("10.0.1.0/24", 65002)
	assert.Equal(t, ValidationValid, state)
}

// TestValidateRefusesUnparseablePrefix verifies a prefix ze cannot parse never reads as
// "the VRP set covers nothing".
//
// VALIDATES: Validate answers a malformed prefix with ValidationInvalid, while the same cache
// still answers well-formed queries with Valid and NotFound as before. The pairing matters: a
// function that refused everything would satisfy the first assertion alone.
// PREVENTS: An unreadable prefix reading as NotFound, which the default not-found action accepts,
// so a route ze never validated enters the Adj-RIB-In as though the cache had cleared it.
func TestValidateRefusesUnparseablePrefix(t *testing.T) {
	c := newROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	for _, prefix := range []string{"not-a-prefix", "10.0.0.0/33", "10.0.0.0", ""} {
		assert.Equal(t, ValidationInvalid, c.Validate(prefix, 65001),
			"unparseable prefix %q is refused, not reported as uncovered", prefix)
	}

	assert.Equal(t, ValidationValid, c.Validate("10.0.1.0/24", 65001),
		"a well-formed authorized prefix still validates")
	assert.Equal(t, ValidationNotFound, c.Validate("192.168.0.0/24", 65001),
		"a well-formed prefix no VRP covers still reads not-found")
}

// TestUnparseablePrefixIsNotAcceptedByDefaultPolicy verifies the refusal survives the policy
// step that turns a validation state into an accept/reject decision.
//
// VALIDATES: With the YANG defaults (invalid: reject, not-found: accept), buildDecisions drops
// the route carrying the state Validate returned for a malformed prefix, and keeps the route
// whose prefix parsed and genuinely has no covering VRP.
// PREVENTS: The state changing while the decision does not, which is the whole defect: a state
// nobody rejects is a state that accepts.
func TestUnparseablePrefixIsNotAcceptedByDefaultPolicy(t *testing.T) {
	c := newROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	rp := &rPKIPlugin{}
	rp.originInvalidAction.Store(uint32(ASPAPolicyReject)) // YANG default
	rp.originNotFoundAction.Store(uint32(ASPAPolicyAccept))

	decisions := rp.buildDecisions([]validationRequest{
		{prefix: "10.0.0.0/33", state: c.Validate("10.0.0.0/33", 65001)},
		{prefix: "192.168.0.0/24", state: c.Validate("192.168.0.0/24", 65001)},
	})

	assert.False(t, decisions[0].Accept, "a prefix ze could not parse is not accepted")
	assert.True(t, decisions[1].Accept, "a parsed, uncovered prefix is still accepted under the default")
}

// TestExtractOriginAS verifies origin AS extraction from AS_PATH attribute.
//
// VALIDATES: Rightmost AS in final AS_SEQUENCE segment is extracted.
// PREVENTS: Wrong origin AS causing incorrect validation.
func TestExtractOriginAS(t *testing.T) {
	// Build raw path attributes: ORIGIN(1) + AS_PATH(2)
	// ORIGIN: flags=0x40, type=1, len=1, value=0 (IGP)
	origin := []byte{0x40, 0x01, 0x01, 0x00}

	// AS_PATH: flags=0x40, type=2, len=10
	// AS_SEQUENCE (type=2), count=2: [65001, 65002]
	asPath := []byte{0x40, 0x02, 0x0A}
	asPathVal := []byte{
		0x02, 0x02, // AS_SEQUENCE, 2 ASNs
	}
	asn1 := make([]byte, 4)
	binary.BigEndian.PutUint32(asn1, 65001)
	asn2 := make([]byte, 4)
	binary.BigEndian.PutUint32(asn2, 65002)
	asPathVal = append(asPathVal, asn1...)
	asPathVal = append(asPathVal, asn2...)
	asPath = append(asPath, asPathVal...)

	rawHex := hex.EncodeToString(append(origin, asPath...))
	result := extractOriginAS(rawHex)
	assert.Equal(t, uint32(65002), result, "origin AS should be rightmost in AS_SEQUENCE")
}

// TestExtractOriginASEmpty verifies empty AS_PATH yields OriginNone.
//
// VALIDATES: Empty attributes or no AS_PATH returns OriginNone.
// PREVENTS: Panic on empty input.
func TestExtractOriginASEmpty(t *testing.T) {
	assert.Equal(t, OriginNone, extractOriginAS(""))
	assert.Equal(t, OriginNone, extractOriginAS("invalid"))
}

// TestExtractOriginASSet verifies AS_SET yields OriginNone.
//
// VALIDATES: Final AS_SET segment returns OriginNone per RFC 6811.
// PREVENTS: AS_SET origin being treated as valid.
func TestExtractOriginASSet(t *testing.T) {
	// AS_PATH with AS_SET (type=1)
	asPath := []byte{0x40, 0x02, 0x06} // flags, type=2, len=6
	asPathVal := []byte{
		0x01, 0x01, // AS_SET, 1 ASN
	}
	asn := make([]byte, 4)
	binary.BigEndian.PutUint32(asn, 65001)
	asPathVal = append(asPathVal, asn...)
	asPath = append(asPath, asPathVal...)

	rawHex := hex.EncodeToString(asPath)
	result := extractOriginAS(rawHex)
	assert.Equal(t, OriginNone, result)
}
