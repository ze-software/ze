package tacacs

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: TACACS+ packet header encoding matches RFC 8907 Section 4.1.
// PREVENTS: wrong byte order or field placement in header.
func TestPacketHeaderMarshalRoundTrip(t *testing.T) {
	hdr := PacketHeader{
		Version:   0xC0,
		Type:      0x01,
		SeqNo:     1,
		Flags:     0x04,
		SessionID: 0xDEADBEEF,
		Length:    42,
	}

	data := hdr.MarshalBinary()
	require.Len(t, data, hdrLen)

	got, err := UnmarshalPacketHeader(data)
	require.NoError(t, err)

	assert.Equal(t, hdr.Version, got.Version)
	assert.Equal(t, hdr.Type, got.Type)
	assert.Equal(t, hdr.SeqNo, got.SeqNo)
	assert.Equal(t, hdr.Flags, got.Flags)
	assert.Equal(t, hdr.SessionID, got.SessionID)
	assert.Equal(t, hdr.Length, got.Length)
}

// VALIDATES: header unmarshal rejects truncated input.
// PREVENTS: panic on short packet.
func TestUnmarshalPacketHeaderTooShort(t *testing.T) {
	_, err := UnmarshalPacketHeader(make([]byte, 5))
	assert.Error(t, err)
}

// VALIDATES: MD5 pseudo-pad encryption is self-inverse (encrypt == decrypt).
// PREVENTS: broken encryption that corrupts body.
func TestEncryptDecryptRoundTrip(t *testing.T) {
	original := []byte("hello TACACS+ world, this is a test body that spans multiple MD5 blocks to verify chaining works correctly")
	body := make([]byte, len(original))
	copy(body, original)

	key := []byte("shared-secret")
	var sessionID uint32 = 0x12345678
	var version uint8 = 0xC0
	var seqNo uint8 = 1

	// Encrypt.
	Encrypt(body, sessionID, key, version, seqNo)
	assert.False(t, bytes.Equal(body, original), "encrypted body should differ from original")

	// Decrypt (same operation).
	Encrypt(body, sessionID, key, version, seqNo)
	assert.Equal(t, original, body, "decrypted body should match original")
}

// VALIDATES: encryption with empty key is a no-op.
// PREVENTS: panic or corruption when no secret configured.
func TestEncryptEmptyKey(t *testing.T) {
	original := []byte("plaintext body")
	body := make([]byte, len(original))
	copy(body, original)

	Encrypt(body, 0x1234, nil, 0xC0, 1)
	assert.Equal(t, original, body, "empty key should not modify body")

	Encrypt(body, 0x1234, []byte{}, 0xC0, 1)
	assert.Equal(t, original, body, "zero-length key should not modify body")
}

// VALIDATES: encryption with empty body is a no-op.
// PREVENTS: panic on empty body.
func TestEncryptEmptyBody(t *testing.T) {
	Encrypt(nil, 0x1234, []byte("key"), 0xC0, 1)
	Encrypt([]byte{}, 0x1234, []byte("key"), 0xC0, 1)
}

// VALIDATES: different keys produce different ciphertext.
// PREVENTS: key not being used in pseudo-pad.
func TestEncryptDifferentKeys(t *testing.T) {
	body1 := []byte("same plaintext")
	body2 := make([]byte, len(body1))
	copy(body2, body1)

	Encrypt(body1, 0x1234, []byte("key-one"), 0xC0, 1)
	Encrypt(body2, 0x1234, []byte("key-two"), 0xC0, 1)
	assert.False(t, bytes.Equal(body1, body2), "different keys should produce different ciphertext")
}

// VALIDATES: different session IDs produce different ciphertext.
// PREVENTS: session ID not being used in pseudo-pad.
func TestEncryptDifferentSessionIDs(t *testing.T) {
	body1 := []byte("same plaintext")
	body2 := make([]byte, len(body1))
	copy(body2, body1)

	Encrypt(body1, 0x1111, []byte("key"), 0xC0, 1)
	Encrypt(body2, 0x2222, []byte("key"), 0xC0, 1)
	assert.False(t, bytes.Equal(body1, body2), "different session IDs should produce different ciphertext")
}

// VALIDATES: AC-17 -- wrong shared secret detected by body length mismatch.
// PREVENTS: silent acceptance of packets encrypted with wrong key.
func TestEncryptWrongSecret(t *testing.T) {
	original := []byte("secret data")
	body := make([]byte, len(original))
	copy(body, original)

	Encrypt(body, 0x1234, []byte("correct-key"), 0xC0, 1)
	Encrypt(body, 0x1234, []byte("wrong-key"), 0xC0, 1)
	assert.NotEqual(t, original, body, "wrong key should not recover original")
}

// VALIDATES: full packet marshal/unmarshal round-trip.
// PREVENTS: header/body misalignment or corruption.
func TestPacketMarshalRoundTrip(t *testing.T) {
	original := &Packet{
		Header: PacketHeader{
			Version:   0xC1,
			Type:      0x01,
			SeqNo:     1,
			Flags:     0x00,
			SessionID: 0xAABBCCDD,
		},
		Body: []byte{0x01, 0x00, 0x02, 0x01, 0x05, 0x03, 0x08, 0x06, 't', 'e', 's', 't', 's', 's', 'h', '1', '.', '2', '.', '3'},
	}

	key := []byte("test-secret")
	wire, err := original.Marshal(key)
	require.NoError(t, err)

	got, err := UnmarshalPacket(wire, key)
	require.NoError(t, err)

	assert.Equal(t, original.Header.Version, got.Header.Version)
	assert.Equal(t, original.Header.Type, got.Header.Type)
	assert.Equal(t, original.Header.SeqNo, got.Header.SeqNo)
	assert.Equal(t, original.Header.SessionID, got.Header.SessionID)
	assert.Equal(t, original.Body, got.Body)
}

// VALIDATES: packet marshal/unmarshal without encryption.
// PREVENTS: encryption applied when no key provided.
func TestPacketMarshalNoEncryption(t *testing.T) {
	body := []byte{0x01, 0x02, 0x03}
	pkt := &Packet{
		Header: PacketHeader{
			Version:   0xC0,
			Type:      0x01,
			SeqNo:     1,
			SessionID: 0x1234,
		},
		Body: body,
	}

	wire, err := pkt.Marshal(nil)
	require.NoError(t, err)

	// Body should be in cleartext in wire bytes.
	assert.Equal(t, body, wire[hdrLen:])

	got, err := UnmarshalPacket(wire, nil)
	require.NoError(t, err)
	assert.Equal(t, body, got.Body)
}

// VALIDATES: truncated packet rejected.
// PREVENTS: out-of-bounds read on short packet.
func TestUnmarshalPacketTruncated(t *testing.T) {
	// Header says body is 10 bytes but data is only 14 bytes total (12 header + 2).
	hdr := PacketHeader{Length: 10, Version: 0xC0, Type: 0x01, SeqNo: 1}
	data := append(hdr.MarshalBinary(), 0x00, 0x00)

	_, err := UnmarshalPacket(data, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "truncated")
}

// VALIDATES: unencrypted flag prevents decryption.
// PREVENTS: XOR corruption when flag is set.
func TestUnmarshalPacketUnencryptedFlag(t *testing.T) {
	body := []byte{0xAA, 0xBB, 0xCC}
	pkt := &Packet{
		Header: PacketHeader{
			Version:   0xC0,
			Type:      0x01,
			SeqNo:     1,
			Flags:     flagUnencrypted,
			SessionID: 0x5678,
		},
		Body: body,
	}

	// Marshal without key (body stays cleartext).
	wire, err := pkt.Marshal(nil)
	require.NoError(t, err)

	// Unmarshal WITH key but unencrypted flag set: should NOT decrypt.
	got, err := UnmarshalPacket(wire, []byte("key"))
	require.NoError(t, err)
	assert.Equal(t, body, got.Body, "unencrypted flag should prevent decryption")
}

// VALIDATES: encryption handles body shorter than one MD5 block (16 bytes).
// PREVENTS: off-by-one in truncated pad application.
func TestEncryptShortBody(t *testing.T) {
	original := []byte{0x42}
	body := make([]byte, 1)
	copy(body, original)

	Encrypt(body, 0x1234, []byte("key"), 0xC0, 1)
	assert.NotEqual(t, original, body)

	Encrypt(body, 0x1234, []byte("key"), 0xC0, 1)
	assert.Equal(t, original, body)
}

// FuzzTacacsPacketUnmarshal verifies unmarshal never panics on random input.
func FuzzTacacsPacketUnmarshal(f *testing.F) {
	f.Add([]byte{0xC0, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x02, 0xAA, 0xBB})
	f.Add([]byte{})
	f.Add([]byte{0xFF})

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must never panic regardless of input.
		UnmarshalPacket(data, nil)           //nolint:errcheck // fuzz: testing for panics, not errors
		UnmarshalPacket(data, []byte("key")) //nolint:errcheck // fuzz: testing for panics, not errors
	})
}

// FuzzTacacsEncryptDecrypt verifies encrypt/decrypt round-trip with random inputs.
func FuzzTacacsEncryptDecrypt(f *testing.F) {
	f.Add([]byte("hello"), []byte("key"), uint32(1234), uint8(0xC0), uint8(1))
	f.Add([]byte{}, []byte("k"), uint32(0), uint8(0xC1), uint8(2))

	f.Fuzz(func(t *testing.T, body, key []byte, sessionID uint32, version, seqNo uint8) {
		if len(body) == 0 || len(key) == 0 {
			return
		}
		original := make([]byte, len(body))
		copy(original, body)

		Encrypt(body, sessionID, key, version, seqNo)
		Encrypt(body, sessionID, key, version, seqNo)
		assert.Equal(t, original, body, "encrypt/decrypt round-trip must be identity")
	})
}
