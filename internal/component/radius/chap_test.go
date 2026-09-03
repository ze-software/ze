package radius

import (
	"encoding/hex"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
)

// errReader is a random source that always fails. crypto/rand.Reader cannot be
// made to fail on demand, and AC-9 is about what ze does when it does.
type errReader struct{}

var errNoEntropy = errors.New("no entropy")

func (errReader) Read([]byte) (int, error) { return 0, errNoEntropy }

// truncatedReader supplies one octet and then ends, which is the other way a
// source fails to produce a full challenge. io.ReadFull turns it into
// io.ErrUnexpectedEOF, and the octet it did deliver must not become a
// one-octet challenge padded with zeros.
type truncatedReader struct{ delivered bool }

func (r *truncatedReader) Read(p []byte) (int, error) {
	if r.delivered || len(p) == 0 {
		return 0, io.EOF
	}
	r.delivered = true
	p[0] = 0xAA
	return 1, nil
}

// TestRadiusAdminChapResponseDigest pins the CHAP Response against vectors
// computed OUTSIDE this package, so the test cannot agree with a wrong
// producer. Both vectors were computed twice and independently, by
// `python3 -c "hashlib.md5(...)"` and by `openssl dgst -md5`, over the byte
// string identifier || password || challenge.
//
// VALIDATES: AC-5 -- the CHAP Response is MD5(identifier, password, challenge),
// in that order.
// PREVENTS: a digest that hashes the three inputs in another order, or over the
// hidden password rather than the plaintext one. Every such variant is a
// 16-octet value a unit test comparing lengths would accept, and every one of
// them makes a real RADIUS server answer Access-Reject.
//
// RFC 2865 Section 2.2 states that the server "encrypts the challenge using MD5
// on the CHAP ID octet, that password, and the CHAP challenge (from the
// CHAP-Challenge attribute if present, otherwise from the Request
// Authenticator), and compares that result to the CHAP-Password".
func TestRadiusAdminChapResponseDigest(t *testing.T) {
	challenge, err := hex.DecodeString("0102030405060708090a0b0c0d0e0f10")
	require.NoError(t, err)

	tests := []struct {
		name     string
		password string
		want     string
	}{
		{"password", "Hello", "f30a3da4592d46dfe5358518dc83b689"},
		// Boundary: a login with no password still produces a well-formed
		// 16-octet response, over the identifier and the challenge alone.
		{"empty password", "", "df14f6a05c60dac0a2a48cf28431301d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chapResponse(0x16, tt.password, challenge)
			assert.Equal(t, tt.want, hex.EncodeToString(got))
			assert.Len(t, got, 16, "RFC 2865 Section 5.3: the String field is 16 octets")
		})
	}
}

// TestRadiusAdminChapCredentialShape pins the wire widths RFC 2865 states.
//
// VALIDATES: the CHAP-Password value is 17 octets (attribute Length 19,
// Section 5.3) and the CHAP-Challenge value is 16 octets (attribute Length 18,
// Section 5.40). The identifier octet leads the CHAP-Password value.
// PREVENTS: a response copied into the value without its identifier octet, or a
// challenge of a width Section 2.2 does not prefer.
func TestRadiusAdminChapCredentialShape(t *testing.T) {
	attrs, err := chapCredential(fixedRandom(0x16, 0x01), "Hello")
	require.NoError(t, err)
	require.Len(t, attrs, 2)

	assert.Equal(t, uint8(AttrCHAPPassword), attrs[0].Type)
	require.Len(t, attrs[0].Value, 17)
	assert.Equal(t, byte(0x16), attrs[0].Value[0], "the identifier leads the value")
	assert.Equal(t, 19, 2+len(attrs[0].Value), "RFC 2865 Section 5.3: Length 19")

	assert.Equal(t, uint8(AttrCHAPChallenge), attrs[1].Type)
	require.Len(t, attrs[1].Value, 16)
	assert.Equal(t, 18, 2+len(attrs[1].Value), "RFC 2865 Section 5.40: Length >= 7")
}

// TestRadiusAdminChapFailsClosedOnRandomError drives the failure from the entry
// point an operator reaches, not from the helper: a random source that cannot
// supply a challenge must abort the login before anything is sent, so no
// predictable challenge and no zero identifier ever reach a server.
//
// VALIDATES: AC-9 -- Authenticate returns an error, the result is not
// authenticated, and the server receives no packet at all.
// PREVENTS: a generation failure falling through to an all-zero challenge and
// an all-zero identifier, which is a credential an attacker can precompute.
func TestRadiusAdminChapFailsClosedOnRandomError(t *testing.T) {
	sources := []struct {
		name   string
		reader io.Reader
	}{
		{"read error", errReader{}},
		{"truncated source", &truncatedReader{}},
	}

	for _, src := range sources {
		t.Run(src.name, func(t *testing.T) {
			key := []byte("testing123")
			srv := newRequestCaptureServer(t, key, []Attr{{Type: AttrFilterID, Value: []byte("admin")}})
			defer srv.close()

			a := testAuthenticator(t, srv.addr, key, ExtractedConfig{
				ProfileAttr: AttrFilterID, AuthMethod: AuthMethodCHAP,
			})
			a.random = src.reader

			res, err := a.Authenticate(aaa.AuthRequest{Username: "alice", Password: "pw"})
			require.Error(t, err)
			assert.False(t, res.Authenticated)
			assert.NotErrorIs(t, err, aaa.ErrAuthRejected,
				"a challenge that cannot be generated is an infra failure, so the chain tries the next backend")
			srv.noRequest(t)
		})
	}
}
