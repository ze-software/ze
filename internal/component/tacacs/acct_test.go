package tacacs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: AcctRequest marshal produces correct wire format for START/STOP.
// PREVENTS: wrong field layout in accounting REQUEST body.
func TestAcctRequestMarshalStartStop(t *testing.T) {
	tests := []struct {
		name  string
		flags uint8
		args  []string
	}{
		{"start", AcctFlagStart, []string{"task_id=1", "service=shell", "cmd=show version"}},
		{"stop", AcctFlagStop, []string{"task_id=1", "elapsed_time=5"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &AcctRequest{
				Flags:         tt.flags,
				AuthenMethod:  AuthenMethodTACACS,
				PrivLvl:       15,
				AuthenType:    authenTypePAP,
				AuthenService: authenServiceLogin,
				User:          "admin",
				Port:          "ssh",
				RemAddr:       "10.0.0.1",
				Args:          tt.args,
			}

			body, err := req.MarshalBinary()
			require.NoError(t, err)

			// Fixed header is 9 bytes for acct.
			require.True(t, len(body) >= 9)
			assert.Equal(t, tt.flags, body[0], "flags")
			assert.Equal(t, uint8(AuthenMethodTACACS), body[1], "authen_method")
			assert.Equal(t, uint8(15), body[2], "priv_lvl")
			assert.Equal(t, uint8(len(tt.args)), body[8], "arg_cnt")
		})
	}
}

// VALIDATES: AcctReply unmarshal parses SUCCESS status.
// PREVENTS: wrong field offsets in accounting reply.
func TestUnmarshalAcctReplySuccess(t *testing.T) {
	// Build: server_msg_len=0, data_len=0, status=SUCCESS.
	body := []byte{0x00, 0x00, 0x00, 0x00, AcctStatusSuccess}

	reply, err := UnmarshalAcctReply(body)
	require.NoError(t, err)

	assert.Equal(t, uint8(AcctStatusSuccess), reply.Status)
	assert.Empty(t, reply.ServerMsg)
	assert.Empty(t, reply.Data)
}

// VALIDATES: AcctReply unmarshal handles ERROR status with message.
// PREVENTS: message field corruption.
func TestUnmarshalAcctReplyError(t *testing.T) {
	msg := "write failed"
	body := make([]byte, 5+len(msg))
	body[0] = 0x00
	body[1] = uint8(len(msg))
	body[2] = 0x00
	body[3] = 0x00
	body[4] = AcctStatusError
	copy(body[5:], msg)

	reply, err := UnmarshalAcctReply(body)
	require.NoError(t, err)

	assert.Equal(t, uint8(AcctStatusError), reply.Status)
	assert.Equal(t, msg, reply.ServerMsg)
}

// VALIDATES: AcctReply rejects truncated input.
// PREVENTS: out-of-bounds on malformed reply.
func TestUnmarshalAcctReplyTruncated(t *testing.T) {
	_, err := UnmarshalAcctReply([]byte{0x00, 0x00})
	assert.Error(t, err)
}

// VALIDATES: AcctRequest with no args produces valid body.
// PREVENTS: off-by-one when arg_cnt is 0.
func TestAcctRequestNoArgs(t *testing.T) {
	req := &AcctRequest{
		Flags:         AcctFlagStart,
		AuthenMethod:  AuthenMethodTACACS,
		PrivLvl:       1,
		AuthenType:    authenTypePAP,
		AuthenService: authenServiceLogin,
		User:          "u",
	}

	body, err := req.MarshalBinary()
	require.NoError(t, err)
	assert.Equal(t, uint8(0), body[8], "arg_cnt should be 0")
	// 9 fixed + 0 arg_lens + 1 user + 0 port + 0 remaddr = 10
	assert.Len(t, body, 10)
}
