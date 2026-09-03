// Design: docs/guide/radius.md -- RADIUS admin CHAP credential
// Related: authenticator.go -- the credential branch that calls this
// Related: attr.go -- EncodeCHAPPassword, which builds the 17-octet value
// RFC: rfc/short/rfc2865.md -- CHAP-Password (Section 5.3), CHAP-Challenge (Section 5.40)

package radius

import (
	"crypto/md5" //nolint:gosec // RFC 2865 Section 2.2 requires MD5 for the CHAP response
	"fmt"
	"io"
)

const (
	// chapChallengeLen is the challenge width. RFC 2865 Section 2.2: "the NAS
	// generates a random challenge (preferably 16 octets)".
	chapChallengeLen = 16
	// chapSeedLen covers the CHAP Identifier and the challenge, which one read
	// of the random source supplies together.
	chapSeedLen = 1 + chapChallengeLen
)

// chapCredential builds the credential attributes a CHAP Access-Request
// carries: CHAP-Password (RFC 2865 Section 5.3) and CHAP-Challenge (Section
// 5.40). It returns no User-Password, because RFC 2865 Section 4.1 states that
// "An Access-Request MUST NOT contain both a User-Password and a
// CHAP-Password."
//
// Admin login has no PPP peer, so ze produces both halves itself. RFC 2865
// Section 2.2 describes the server side and does not constrain where the
// challenge came from. It says the server "looks up a password based on the
// User-Name, encrypts the challenge using MD5 on the CHAP ID octet, that
// password, and the CHAP challenge (from the CHAP-Challenge attribute if
// present, otherwise from the Request Authenticator), and compares that result
// to the CHAP-Password".
//
// The challenge goes in attribute 60 rather than the Request Authenticator.
// Section 5.40 makes that placement a MAY, and the Request Authenticator
// already carries reply verification, which the client checks on the way back.
//
// A random-source failure returns an error and no attributes. A zero challenge
// or a zero identifier is predictable, so it never reaches the wire.
func chapCredential(random io.Reader, password string) ([]Attr, error) {
	seed := make([]byte, chapSeedLen)
	if _, err := io.ReadFull(random, seed); err != nil {
		return nil, fmt.Errorf("radius: CHAP challenge: %w", err)
	}
	identifier := seed[0]
	challenge := seed[1:]
	return []Attr{
		{Type: AttrCHAPPassword, Value: EncodeCHAPPassword(identifier, chapResponse(identifier, password, challenge))},
		{Type: AttrCHAPChallenge, Value: challenge},
	}, nil
}

// chapResponse computes the 16-octet CHAP Response the CHAP-Password attribute
// carries after its identifier octet: MD5 over the identifier, then the
// password, then the challenge, in that order.
//
// RFC 2865 Section 2.2 states that the server "encrypts the challenge using MD5
// on the CHAP ID octet, that password, and the CHAP challenge ... and compares
// that result to the CHAP-Password".
func chapResponse(identifier uint8, password string, challenge []byte) []byte {
	h := md5.New() //nolint:gosec // RFC 2865 Section 2.2 mandates MD5
	h.Write([]byte{identifier})
	h.Write([]byte(password))
	h.Write(challenge)
	return h.Sum(nil)
}
