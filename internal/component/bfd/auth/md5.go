// Design: rfc/short/rfc5880.md -- Keyed / Meticulous Keyed MD5 (Section 6.7.3)
//
// Keyed MD5 and Meticulous Keyed MD5 construction: thin wrappers that
// delegate to the generic digest helpers in sha1.go. MD5 output is 16
// bytes; RFC 5880 §6.7.3 lays out the same body layout as the SHA1
// variants.
//
// RFC 5880 §6.7 requires both SHA1 types and leaves the rest optional;
// MD5 remains widely deployed so ze implements it too. Simple Password
// (§6.7.2, Type 1) lives in simple.go.
package auth

import (
	"crypto/md5" //nolint:gosec // RFC 5880 §6.7.3 mandates MD5; not used for general cryptographic integrity

	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// md5Sum is a digestFunc adapter over stdlib md5.Sum.
func md5Sum(b []byte) []byte { h := md5.Sum(b); return h[:] } //nolint:gosec // see file-level comment

func newMD5Signer(cfg Settings) *digestSigner {
	return newDigestSigner(cfg, packet.AuthLenKeyedMD5, md5Sum)
}

func newMD5Verifier(cfg Settings) *digestVerifier {
	meticulous := cfg.Type == packet.AuthTypeMeticulousKeyedMD5 || cfg.Meticulous
	return newDigestVerifier(cfg, packet.AuthLenKeyedMD5, md5Sum, meticulous)
}
