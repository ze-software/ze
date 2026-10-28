// Design: docs/guide/l2tp.md -- the test RADIUS servers answer as a server updated for BlastRADIUS
// Related: rfc2865_handler_test.go -- startMockRADIUS signs through signAccessAnswer
// Related: rfc2866_zero_marked_login_test.go -- startLoginRADIUS signs through signAccessAnswer

package l2tpauthradius

import (
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // RFC 2869 Section 5.14 mandates HMAC-MD5.
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/radius"
)

// signAccessAnswer returns the encoded answer resp with a Message-Authenticator
// inserted first and signed under secret over the packet carrying reqAuth, as
// RFC 2869 Section 5.14 defines it. An Accounting-Response is returned
// unchanged. The caller computes the Response Authenticator afterwards, over
// the returned bytes. Ze's config reader requires the Message-Authenticator by
// default, so a test server that answers an Access-Request MUST sign.
func signAccessAnswer(resp []byte, reqAuth [radius.AuthenticatorLen]byte, secret []byte) []byte {
	if resp[0] == radius.CodeAccountingResp {
		return resp
	}
	const attrLen = 2 + radius.AuthenticatorLen
	signed := make([]byte, len(resp)+attrLen)
	copy(signed, resp[:radius.HeaderLen])
	signed[radius.HeaderLen] = radius.AttrMessageAuthenticator
	signed[radius.HeaderLen+1] = attrLen
	copy(signed[radius.HeaderLen+attrLen:], resp[radius.HeaderLen:])
	binary.BigEndian.PutUint16(signed[2:4], uint16(len(signed)))
	copy(signed[4:radius.HeaderLen], reqAuth[:])
	mac := hmac.New(md5.New, secret)
	mac.Write(signed)
	copy(signed[radius.HeaderLen+2:radius.HeaderLen+attrLen], mac.Sum(nil))
	return signed
}
