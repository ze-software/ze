// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RFC 2205 Section 3 and
// Appendix B obligations the codec meets: the object classes a node recognizes,
// and the construction check every received message goes through.
package rsvpte

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawObject returns an encoder for an object of the given class with a four-byte
// zero body, the smallest well-formed object of a class ze reads no body for.
func rawObject(classNum uint8) objEncoder {
	return func(b []byte) int {
		encodeObjectHeader(b, objectHeader{Length: objHdrLen + 4, ClassNum: classNum, CType: 1})
		clear(b[objHdrLen : objHdrLen+4])
		return objHdrLen + 4
	}
}

// pathWithExtraObjects encodes a conformant PATH with the given objects added
// after the mandatory ones.
func pathWithExtraObjects(classes ...uint8) []byte {
	objects := conformantMessages()[MsgTypePath]
	encoders := make([]objEncoder, 0, len(objects)+len(classes))
	for _, obj := range objects {
		encoders = append(encoders, obj.encode)
	}
	for _, classNum := range classes {
		encoders = append(encoders, rawObject(classNum))
	}
	return encodeMessage(MsgTypePath, defaultIPTTL, encoders)
}

// RFC requirement: RFC2205-3-1 positive — a PATH carrying INTEGRITY, SCOPE, ADSPEC, POLICY_DATA and RESV_CONFIRM objects decodes as a known message: none of the five is classified as an unknown class.
func TestRFC2205KnownClassesRecognized(t *testing.T) {
	msg, err := DecodeMessage(pathWithExtraObjects(ClassIntegrity, ClassScope, ClassAdspec, ClassPolicyData, ClassResvConfirm))
	require.NoError(t, err)
	assert.False(t, msg.HasUnknownObject, "every Section 3 class ze reads no body for is still recognized")
	assert.True(t, msg.HasSession, "the message's own objects decode around them")
	assert.True(t, msg.HasSenderTemplate)
}

// RFC requirement: RFC2205-3-1 negative — a PATH carrying an object of a class Section 3 does not list, with a zero high-order bit, is classified as unknown and names that class.
func TestRFC2205UnlistedClassNotRecognized(t *testing.T) {
	const unlisted uint8 = 2 // 0bbbbbbb, assigned to no class in RFC 2205 or RFC 3209.
	msg, err := DecodeMessage(pathWithExtraObjects(unlisted))
	require.NoError(t, err)
	require.True(t, msg.HasUnknownObject, "an unlisted class is not recognized")
	assert.Equal(t, unlisted, msg.UnknownObject.ClassNum)
}

// RFC requirement: RFC2205-4-4 positive — a PATH built with every object its BNF requires passes the construction check and decodes.
func TestRFC2205WellFormedMessageVerified(t *testing.T) {
	msg, err := DecodeMessage(encodeWithout(MsgTypePath, conformantMessages()[MsgTypePath], ""))
	require.NoError(t, err)
	assert.True(t, msg.HasSession)
	assert.True(t, msg.HasHop)
	assert.True(t, msg.HasTimeValues)
}

// RFC requirement: RFC2205-4-4 negative — a PATH that omits its required SESSION object fails the construction check with the absent-object error.
func TestRFC2205MissingRequiredObjectRefused(t *testing.T) {
	_, err := DecodeMessage(encodeWithout(MsgTypePath, conformantMessages()[MsgTypePath], "SESSION"))
	require.Error(t, err)
	assert.ErrorIs(t, err, errObjectAbsent)
}
