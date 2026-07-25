package radius

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	radius "github.com/ze-software/ze/internal/component/radius"
)

// VALIDATES: the mock decodes a User-Password hidden by the production client.
// PREVENTS: the mock silently rejecting every login because it cannot recover
// the plaintext, which would make the functional tests meaningless.
func TestDecodeUserPasswordRoundTrip(t *testing.T) {
	secret := []byte("s3cr3t")
	auth, err := radius.RandomAuthenticator()
	require.NoError(t, err)
	enc := radius.EncodeUserPassword([]byte("hunter2"), secret, auth)
	assert.Equal(t, "hunter2", decodeUserPassword(enc, secret, auth))
}

// VALIDATES: handleRequest accepts a known user (with Filter-Id) and rejects a
// wrong password.
// PREVENTS: the mock accepting everything (or nothing), hiding real backend bugs.
func TestHandleRequestAcceptReject(t *testing.T) {
	secret := []byte("s3cr3t")
	users := userList{{name: "alice", pass: "pw", profiles: []string{"netops"}}}
	auth, err := radius.RandomAuthenticator()
	require.NoError(t, err)

	encode := func(pass string) []byte {
		req := &radius.Packet{
			Code:          radius.CodeAccessRequest,
			Identifier:    7,
			Authenticator: auth,
			Attrs: []radius.Attr{
				{Type: radius.AttrUserName, Value: radius.AttrString("alice")},
				{Type: radius.AttrUserPassword, Value: radius.EncodeUserPassword([]byte(pass), secret, auth)},
			},
		}
		buf := make([]byte, radius.MaxPacketLen)
		n, encErr := req.EncodeTo(buf, 0)
		require.NoError(t, encErr)
		return buf[:n]
	}

	accept, err := radius.Decode(handleRequest(encode("pw"), secret, users, false))
	require.NoError(t, err)
	assert.Equal(t, uint8(radius.CodeAccessAccept), accept.Code)
	assert.Equal(t, "netops", string(accept.FindAttr(radius.AttrFilterID)))

	reject, err := radius.Decode(handleRequest(encode("wrong"), secret, users, false))
	require.NoError(t, err)
	assert.Equal(t, uint8(radius.CodeAccessReject), reject.Code)
}
