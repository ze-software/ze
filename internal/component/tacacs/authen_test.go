package tacacs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: AuthenStart PAP packet construction matches RFC 8907 Section 5.1.
// PREVENTS: wrong field layout in authentication START body.
func TestAuthenStartMarshal(t *testing.T) {
	start := NewPAPAuthenStart("admin", "secret", "ssh", "192.168.1.1")
	body, err := start.MarshalBinary()
	require.NoError(t, err)

	// Fixed header: 8 bytes.
	require.True(t, len(body) >= 8, "body must be at least 8 bytes")

	assert.Equal(t, uint8(authenActionLogin), body[0], "action")
	assert.Equal(t, uint8(1), body[1], "priv_lvl")
	assert.Equal(t, uint8(authenTypePAP), body[2], "authen_type")
	assert.Equal(t, uint8(authenServiceLogin), body[3], "authen_service")
	assert.Equal(t, uint8(5), body[4], "user_len")
	assert.Equal(t, uint8(3), body[5], "port_len")
	assert.Equal(t, uint8(11), body[6], "rem_addr_len")
	assert.Equal(t, uint8(6), body[7], "data_len (password)")

	// Variable fields.
	off := 8
	assert.Equal(t, "admin", string(body[off:off+5]))
	off += 5
	assert.Equal(t, "ssh", string(body[off:off+3]))
	off += 3
	assert.Equal(t, "192.168.1.1", string(body[off:off+11]))
	off += 11
	assert.Equal(t, "secret", string(body[off:off+6]))
}

// VALIDATES: PAP authen type uses minor version 1.
// PREVENTS: wrong version byte causing server rejection.
func TestAuthenStartVersion(t *testing.T) {
	pap := NewPAPAuthenStart("user", "pass", "ssh", "1.2.3.4")
	assert.Equal(t, uint8(verMinorOne), pap.Version(), "PAP LOGIN should use minor version 1")

	// Non-PAP action should use default version.
	other := &AuthenStart{Action: authenActionLogin, AuthenType: 0x01} // ASCII
	assert.Equal(t, uint8(verMajor), other.Version(), "non-PAP should use default version")
}

// VALIDATES: AuthenReply parsing handles PASS status.
// PREVENTS: wrong field offsets in reply parsing.
func TestUnmarshalAuthenReplyPass(t *testing.T) {
	// Build a PASS reply with server_msg="OK" and no data.
	body := []byte{
		0x01,       // status: PASS
		0x00,       // flags
		0x00, 0x02, // server_msg_len: 2
		0x00, 0x00, // data_len: 0
		'O', 'K', // server_msg
	}

	reply, err := UnmarshalAuthenReply(body)
	require.NoError(t, err)

	assert.Equal(t, uint8(0x01), reply.Status)
	assert.Equal(t, uint8(0x00), reply.Flags)
	assert.Equal(t, "OK", reply.ServerMsg)
	assert.Empty(t, reply.Data)
}

// VALIDATES: AuthenReply parsing handles FAIL status with message.
// PREVENTS: message field corruption.
func TestUnmarshalAuthenReplyFail(t *testing.T) {
	msg := "Authentication failed"
	body := make([]byte, 6+len(msg))
	body[0] = 0x02 // status: FAIL
	body[1] = 0x00
	body[2] = 0x00
	body[3] = uint8(len(msg))
	body[4] = 0x00
	body[5] = 0x00
	copy(body[6:], msg)

	reply, err := UnmarshalAuthenReply(body)
	require.NoError(t, err)

	assert.Equal(t, uint8(0x02), reply.Status)
	assert.Equal(t, msg, reply.ServerMsg)
}

// VALIDATES: AuthenReply rejects truncated input.
// PREVENTS: out-of-bounds read on malformed reply.
func TestUnmarshalAuthenReplyTruncated(t *testing.T) {
	// Too short for fixed header.
	_, err := UnmarshalAuthenReply([]byte{0x01, 0x00})
	assert.Error(t, err)

	// Header says 10 bytes of server_msg but body is shorter.
	body := []byte{0x01, 0x00, 0x00, 0x0A, 0x00, 0x00, 'a', 'b'}
	_, err = UnmarshalAuthenReply(body)
	assert.Error(t, err)
}

// VALIDATES: AuthenStart with empty fields produces valid body.
// PREVENTS: panic on zero-length user/port/remaddr.
func TestAuthenStartEmptyFields(t *testing.T) {
	start := &AuthenStart{
		Action:        authenActionLogin,
		AuthenType:    authenTypePAP,
		AuthenService: authenServiceLogin,
	}

	body, err := start.MarshalBinary()
	require.NoError(t, err)
	assert.Len(t, body, 8, "empty fields should produce 8-byte body")
	assert.Equal(t, uint8(0), body[4], "user_len should be 0")
	assert.Equal(t, uint8(0), body[5], "port_len should be 0")
	assert.Equal(t, uint8(0), body[6], "rem_addr_len should be 0")
	assert.Equal(t, uint8(0), body[7], "data_len should be 0")
}

// VALIDATES: BLOCKER #1 -- MarshalBinary rejects fields >255 bytes.
// PREVENTS: silent uint8 truncation producing malformed packets.
func TestAuthenStartMarshalFieldTooLong(t *testing.T) {
	long := make([]byte, 256)
	start := &AuthenStart{
		Action:        authenActionLogin,
		AuthenType:    authenTypePAP,
		AuthenService: authenServiceLogin,
		User:          string(long),
	}

	_, err := start.MarshalBinary()
	assert.Error(t, err, "user >255 bytes should be rejected")
	assert.Contains(t, err.Error(), "255")
}
