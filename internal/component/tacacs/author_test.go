package tacacs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: AuthorRequest marshal produces correct wire format.
// PREVENTS: wrong field layout in authorization REQUEST body.
func TestAuthorRequestMarshal(t *testing.T) {
	req := &AuthorRequest{
		AuthenMethod:  AuthenMethodTACACS,
		PrivLvl:       15,
		AuthenType:    authenTypePAP,
		AuthenService: authenServiceLogin,
		User:          "admin",
		Port:          "ssh",
		RemAddr:       "10.0.0.1",
		Args:          []string{"service=shell", "cmd=show", "cmd-arg=version"},
	}

	body, err := req.MarshalBinary()
	require.NoError(t, err)

	// Fixed header.
	assert.Equal(t, uint8(AuthenMethodTACACS), body[0], "authen_method")
	assert.Equal(t, uint8(15), body[1], "priv_lvl")
	assert.Equal(t, uint8(authenTypePAP), body[2], "authen_type")
	assert.Equal(t, uint8(authenServiceLogin), body[3], "authen_service")
	assert.Equal(t, uint8(5), body[4], "user_len")
	assert.Equal(t, uint8(3), body[5], "port_len")
	assert.Equal(t, uint8(8), body[6], "rem_addr_len")
	assert.Equal(t, uint8(3), body[7], "arg_cnt")

	// Arg lengths.
	assert.Equal(t, uint8(13), body[8], "arg[0] len (service=shell)")
	assert.Equal(t, uint8(8), body[9], "arg[1] len (cmd=show)")
	assert.Equal(t, uint8(15), body[10], "arg[2] len (cmd-arg=version)")
}

// VALIDATES: AuthorResponse unmarshal parses PASS_ADD status with args.
// PREVENTS: wrong arg parsing in authorization response.
func TestUnmarshalAuthorResponsePassAdd(t *testing.T) {
	// Build: status=PASS_ADD, 1 arg "priv-lvl=15", server_msg="OK", no data.
	arg := "priv-lvl=15"
	msg := "OK"
	body := make([]byte, 6+1+len(msg)+len(arg))
	body[0] = AuthorStatusPassAdd
	body[1] = 1 // arg_cnt
	body[2] = 0
	body[3] = uint8(len(msg)) // server_msg_len
	body[4] = 0
	body[5] = 0 // data_len
	body[6] = uint8(len(arg))
	off := 7
	off += copy(body[off:], msg)
	copy(body[off:], arg)

	resp, err := UnmarshalAuthorResponse(body)
	require.NoError(t, err)

	assert.Equal(t, uint8(AuthorStatusPassAdd), resp.Status)
	assert.Equal(t, msg, resp.ServerMsg)
	require.Len(t, resp.Args, 1)
	assert.Equal(t, "priv-lvl=15", resp.Args[0])
}

// VALIDATES: AuthorResponse unmarshal handles FAIL with no args.
// PREVENTS: crash on zero-arg response.
func TestUnmarshalAuthorResponseFail(t *testing.T) {
	msg := "denied"
	body := make([]byte, 6+len(msg))
	body[0] = AuthorStatusFail
	body[1] = 0 // arg_cnt
	body[2] = 0
	body[3] = uint8(len(msg))
	body[4] = 0
	body[5] = 0
	copy(body[6:], msg)

	resp, err := UnmarshalAuthorResponse(body)
	require.NoError(t, err)

	assert.Equal(t, uint8(AuthorStatusFail), resp.Status)
	assert.Equal(t, msg, resp.ServerMsg)
	assert.Empty(t, resp.Args)
}

// VALIDATES: AuthorResponse rejects truncated input.
// PREVENTS: out-of-bounds on malformed response.
func TestUnmarshalAuthorResponseTruncated(t *testing.T) {
	_, err := UnmarshalAuthorResponse([]byte{0x01, 0x00})
	assert.Error(t, err)
}

// VALIDATES: AuthorRequest with no args produces valid body.
// PREVENTS: off-by-one when arg_cnt is 0.
func TestAuthorRequestNoArgs(t *testing.T) {
	req := &AuthorRequest{
		AuthenMethod:  AuthenMethodTACACS,
		PrivLvl:       1,
		AuthenType:    authenTypePAP,
		AuthenService: authenServiceLogin,
		User:          "user",
		Port:          "ssh",
		RemAddr:       "1.2.3.4",
	}

	body, err := req.MarshalBinary()
	require.NoError(t, err)
	assert.Equal(t, uint8(0), body[7], "arg_cnt should be 0")
	// 8 fixed + 0 arg_lens + 4 user + 3 port + 7 remaddr = 22
	assert.Len(t, body, 22)
}
