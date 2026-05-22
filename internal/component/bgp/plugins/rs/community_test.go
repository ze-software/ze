package rs

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
)

// buildTestPayload builds a minimal UPDATE payload with the given path attributes.
func buildTestPayload(attrs ...pathAttr) []byte {
	var paBuf []byte
	for _, a := range attrs {
		paBuf = append(paBuf, a.encode()...)
	}

	var buf []byte
	buf = append(buf, 0, 0) // withdrawn routes length = 0
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(paBuf)))
	buf = append(buf, paBuf...)
	return buf
}

type pathAttr struct {
	code  uint8
	value []byte
}

func (a pathAttr) encode() []byte {
	flags := byte(0x40) // Transitive
	if len(a.value) > 255 {
		flags |= 0x10 // Extended length
		buf := []byte{flags, a.code}
		buf = binary.BigEndian.AppendUint16(buf, uint16(len(a.value)))
		return append(buf, a.value...)
	}
	return append([]byte{flags, a.code, byte(len(a.value))}, a.value...)
}

func communityBytes(communities ...[2]uint16) []byte {
	var buf []byte
	for _, c := range communities {
		buf = binary.BigEndian.AppendUint16(buf, c[0])
		buf = binary.BigEndian.AppendUint16(buf, c[1])
	}
	return buf
}

func largeCommunityBytes(communities ...[3]uint32) []byte {
	var buf []byte
	for _, c := range communities {
		buf = binary.BigEndian.AppendUint32(buf, c[0])
		buf = binary.BigEndian.AppendUint32(buf, c[1])
		buf = binary.BigEndian.AppendUint32(buf, c[2])
	}
	return buf
}

func TestParseCommunityPolicyBlacklist(t *testing.T) {
	// 0:64513 -> do-not-announce to ASN 64513
	payload := buildTestPayload(pathAttr{
		code:  8,
		value: communityBytes([2]uint16{0, 64513}),
	})
	policy := ParseCommunityPolicy(payload, 65000)

	assert.True(t, policy.ShouldForwardTo(64512))  // not blacklisted
	assert.False(t, policy.ShouldForwardTo(64513)) // blacklisted
	assert.True(t, policy.ShouldForwardTo(64514))
}

func TestParseCommunityPolicyWhitelist(t *testing.T) {
	// 65000:64513 -> announce-only to ASN 64513 (RS ASN = 65000)
	payload := buildTestPayload(pathAttr{
		code:  8,
		value: communityBytes([2]uint16{65000, 64513}),
	})
	policy := ParseCommunityPolicy(payload, 65000)

	assert.True(t, policy.ShouldForwardTo(64513))
	assert.False(t, policy.ShouldForwardTo(64514))
}

func TestParseCommunityPolicyMultipleWhitelist(t *testing.T) {
	// 65000:64513 + 65000:64514 -> announce only to 64513 and 64514
	payload := buildTestPayload(pathAttr{
		code: 8,
		value: communityBytes(
			[2]uint16{65000, 64513},
			[2]uint16{65000, 64514},
		),
	})
	policy := ParseCommunityPolicy(payload, 65000)

	assert.True(t, policy.ShouldForwardTo(64513))
	assert.True(t, policy.ShouldForwardTo(64514))
	assert.False(t, policy.ShouldForwardTo(64515))
}

func TestParseCommunityPolicyRSBlackhole(t *testing.T) {
	// 65000:0 -> blackhole at RS
	payload := buildTestPayload(pathAttr{
		code:  8,
		value: communityBytes([2]uint16{65000, 0}),
	})
	policy := ParseCommunityPolicy(payload, 65000)

	assert.True(t, policy.RSBlackhole)
	assert.False(t, policy.ShouldForwardTo(64513))
}

func TestParseCommunityPolicyRFC7999(t *testing.T) {
	// 65535:666 -> BLACKHOLE
	payload := buildTestPayload(pathAttr{
		code:  8,
		value: communityBytes([2]uint16{65535, 666}),
	})
	policy := ParseCommunityPolicy(payload, 65000)

	assert.True(t, policy.RFC7999Blackhole)
	assert.True(t, policy.ShouldForwardTo(64513))
}

func TestParseLargeCommunityPrepend(t *testing.T) {
	// 65000:101:64513 -> prepend 1x to ASN 64513
	// 65000:103:0 -> prepend 3x to all
	payload := buildTestPayload(pathAttr{
		code: 32,
		value: largeCommunityBytes(
			[3]uint32{65000, 101, 64513},
			[3]uint32{65000, 103, 0},
		),
	})
	policy := ParseCommunityPolicy(payload, 65000)

	assert.Equal(t, uint8(1), policy.PrependCount(64513))
	assert.Equal(t, uint8(3), policy.PrependCount(64514)) // wildcard
	assert.Equal(t, uint8(3), policy.PrependCount(0))     // wildcard target itself
}

func TestParseCommunityPolicyEmpty(t *testing.T) {
	payload := buildTestPayload()
	policy := ParseCommunityPolicy(payload, 65000)

	assert.False(t, policy.RSBlackhole)
	assert.False(t, policy.RFC7999Blackhole)
	assert.Nil(t, policy.BlacklistASNs)
	assert.Nil(t, policy.WhitelistASNs)
	assert.True(t, policy.ShouldForwardTo(64513))
}

func TestParseCommunityPolicyCombined(t *testing.T) {
	// Standard: 0:64513 (blacklist) + Large: 65000:102:64514 (prepend 2x)
	payload := buildTestPayload(
		pathAttr{code: 8, value: communityBytes([2]uint16{0, 64513})},
		pathAttr{code: 32, value: largeCommunityBytes([3]uint32{65000, 102, 64514})},
	)
	policy := ParseCommunityPolicy(payload, 65000)

	assert.False(t, policy.ShouldForwardTo(64513)) // blacklisted
	assert.True(t, policy.ShouldForwardTo(64514))  // not blacklisted
	assert.Equal(t, uint8(2), policy.PrependCount(64514))
	assert.Equal(t, uint8(0), policy.PrependCount(64515))
}

func TestParseCommunityPolicyNonRSLargeCommunity(t *testing.T) {
	// Large community from a different ASN should be ignored.
	payload := buildTestPayload(pathAttr{
		code:  32,
		value: largeCommunityBytes([3]uint32{64000, 101, 64513}),
	})
	policy := ParseCommunityPolicy(payload, 65000)

	assert.Equal(t, uint8(0), policy.PrependCount(64513))
}

func TestCommunityPolicyBlacklistDoesNotAffectForwardForCorrectBlacklist(t *testing.T) {
	payload := buildTestPayload(pathAttr{
		code:  8,
		value: communityBytes([2]uint16{0, 64513}),
	})
	policy := ParseCommunityPolicy(payload, 65000)

	assert.False(t, policy.ShouldForwardTo(64513))
	assert.True(t, policy.ShouldForwardTo(64512))
	assert.True(t, policy.ShouldForwardTo(64514))
}
