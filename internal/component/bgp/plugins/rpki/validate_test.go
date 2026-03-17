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
func TestValidateValid(t *testing.T) {
	c := NewROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	state := c.Validate("10.0.1.0/24", 65001)
	assert.Equal(t, ValidationValid, state)
}

// TestValidateInvalid verifies Invalid state when origin AS does not match.
//
// VALIDATES: Route with wrong origin AS but covered prefix is Invalid.
// PREVENTS: Invalid routes being accepted.
func TestValidateInvalid(t *testing.T) {
	c := NewROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	state := c.Validate("10.0.1.0/24", 65999)
	assert.Equal(t, ValidationInvalid, state)
}

// TestValidateNotFound verifies NotFound state when no VRP covers prefix.
//
// VALIDATES: Route with no covering VRP is NotFound.
// PREVENTS: Non-covered routes being marked Invalid.
func TestValidateNotFound(t *testing.T) {
	c := NewROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	state := c.Validate("192.168.0.0/24", 65001)
	assert.Equal(t, ValidationNotFound, state)
}

// TestValidateMaxLengthExceeded verifies Invalid when prefix exceeds maxLength.
//
// VALIDATES: Route /25 is Invalid when VRP maxLength is /24.
// PREVENTS: Over-specific prefixes being accepted.
func TestValidateMaxLengthExceeded(t *testing.T) {
	c := NewROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	state := c.Validate("10.0.1.0/25", 65001) // /25 > maxLen /24
	assert.Equal(t, ValidationInvalid, state)
}

// TestValidateAS0 verifies ASN=0 means "no AS authorized" (RFC 6483).
//
// VALIDATES: VRP with ASN=0 causes Invalid for any origin AS.
// PREVENTS: AS0 ROAs being treated as valid authorization.
func TestValidateAS0(t *testing.T) {
	c := NewROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 0)) // AS0 = no AS authorized

	state := c.Validate("10.0.1.0/24", 65001)
	assert.Equal(t, ValidationInvalid, state)
}

// TestValidateOriginNone verifies AS_SET yields Invalid when covered.
//
// VALIDATES: OriginNone (from AS_SET) yields Invalid when VRPs exist.
// PREVENTS: AS_SET routes being accepted as Valid.
func TestValidateOriginNone(t *testing.T) {
	c := NewROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	state := c.Validate("10.0.1.0/24", OriginNone)
	assert.Equal(t, ValidationInvalid, state)
}

// TestValidateMultipleVRPsOneMatch verifies Valid when any VRP matches.
//
// VALIDATES: Multiple covering VRPs -- Valid if ANY matches.
// PREVENTS: First-VRP-only evaluation.
func TestValidateMultipleVRPsOneMatch(t *testing.T) {
	c := NewROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))
	c.Add(makeVRP("10.0.0.0/8", 24, 65002))

	state := c.Validate("10.0.1.0/24", 65002)
	assert.Equal(t, ValidationValid, state)
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
