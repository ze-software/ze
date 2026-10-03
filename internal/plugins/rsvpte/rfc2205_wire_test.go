// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RFC 2205 Section 3 and
// Appendix B obligations the codec meets: the object classes a node recognizes,
// and the construction check every received message goes through.
package rsvpte

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawObject supplies structurally valid known objects, or a four-byte opaque
// body for an unknown class. INTEGRITY recognition does not test authentication.
func rawObject(classNum uint8) objEncoder {
	return func(b []byte) int {
		switch classNum {
		case ClassAdspec:
			return encodeAdspec(b, 1500, serviceControlledLoad)
		case ClassIntegrity:
			// RFC 2747 Section 2.1: flags, reserved, 48-bit key ID,
			// 64-bit sequence number and a 16-byte digest.
			encodeObjectHeader(b, objectHeader{Length: objHdrLen + 32, ClassNum: classNum, CType: 1})
			clear(b[objHdrLen : objHdrLen+32])
			b[objHdrLen+7] = 1
			b[objHdrLen+15] = 1
			return objHdrLen + 32
		case ClassScope:
			encodeObjectHeader(b, objectHeader{Length: objHdrLen + 4, ClassNum: classNum, CType: CTypeIPv4})
			copy(b[objHdrLen:objHdrLen+4], []byte{10, 0, 0, 1})
			return objHdrLen + 4
		case ClassPolicyData:
			// RFC 2750 Section 3.1: an empty option/element list starts
			// at offset 8, after Data Offset and the reserved field.
			encodeObjectHeader(b, objectHeader{Length: objHdrLen + 4, ClassNum: classNum, CType: 1})
			binary.BigEndian.PutUint16(b[objHdrLen:objHdrLen+2], objHdrLen+4)
			clear(b[objHdrLen+2 : objHdrLen+4])
			return objHdrLen + 4
		case ClassResvConfirm:
			return encodeResvConfirm(b, netip.MustParseAddr("10.0.0.9"))
		}
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

// RFC requirement: RFC2205-3-1 positive — INTEGRITY, POLICY_DATA and ADSPEC in a PATH, and SCOPE and RESV_CONFIRM before STYLE in a RESV, are recognized without losing the mandatory decoded objects.
// MUTATION: remove INTEGRITY, SCOPE or POLICY_DATA from classKnownUnprocessed,
// or classify ADSPEC or RESV_CONFIRM as unknown in DecodeMessage.
func TestRFC2205KnownClassesRecognized(t *testing.T) {
	objects := conformantMessages()
	t.Run("PATH", func(t *testing.T) {
		encoders := []objEncoder{rawObject(ClassIntegrity)}
		for _, obj := range objects[MsgTypePath] {
			if obj.name == "SENDER_TEMPLATE" {
				encoders = append(encoders, rawObject(ClassPolicyData))
			}
			encoders = append(encoders, obj.encode)
		}
		encoders = append(encoders, rawObject(ClassAdspec))
		want, err := DecodeMessage(encodeWithout(MsgTypePath, objects[MsgTypePath], ""))
		require.NoError(t, err)
		msg, err := DecodeMessage(encodeMessage(MsgTypePath, defaultIPTTL, encoders))
		require.NoError(t, err)
		assert.False(t, msg.HasUnknownObject, "known optional classes are recognized")
		assert.True(t, msg.HasSession, "the message's own objects decode around them")
		assert.True(t, msg.HasHop)
		assert.True(t, msg.HasTimeValues)
		assert.True(t, msg.HasSenderTemplate)
		assert.True(t, msg.HasSenderTSpec)
		assert.True(t, msg.HasLabelRequest)
		assert.Equal(t, want.Session, msg.Session)
		assert.Equal(t, want.Hop, msg.Hop)
		assert.Equal(t, want.TimeValues, msg.TimeValues)
		assert.Equal(t, want.SenderTemplate, msg.SenderTemplate)
		assert.Equal(t, want.SenderTSpec, msg.SenderTSpec)
		assert.Equal(t, want.LabelRequest, msg.LabelRequest)
		assert.True(t, msg.HasAdspec)
	})
	t.Run("RESV", func(t *testing.T) {
		var encoders []objEncoder
		for _, obj := range objects[MsgTypeResv] {
			if obj.name == "STYLE" {
				encoders = append(encoders, rawObject(ClassResvConfirm), rawObject(ClassScope))
			}
			encoders = append(encoders, obj.encode)
		}
		want, err := DecodeMessage(encodeWithout(MsgTypeResv, objects[MsgTypeResv], ""))
		require.NoError(t, err)
		msg, err := DecodeMessage(encodeMessage(MsgTypeResv, defaultIPTTL, encoders))
		require.NoError(t, err)
		assert.False(t, msg.HasUnknownObject, "known optional classes are recognized")
		assert.True(t, msg.HasSession)
		assert.True(t, msg.HasHop)
		assert.True(t, msg.HasTimeValues)
		assert.True(t, msg.HasStyle)
		assert.Equal(t, want.Session, msg.Session)
		assert.Equal(t, want.Hop, msg.Hop)
		assert.Equal(t, want.TimeValues, msg.TimeValues)
		assert.Equal(t, want.Style, msg.Style)
		assert.Equal(t, want.FlowDescriptors, msg.FlowDescriptors)
		assert.True(t, msg.HasResvConfirm)
		assert.Equal(t, netip.MustParseAddr("10.0.0.9"), msg.ResvConfirm)
	})
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
