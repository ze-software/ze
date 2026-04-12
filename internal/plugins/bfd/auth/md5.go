// Design: rfc/short/rfc5880.md -- Keyed / Meticulous Keyed MD5 (Section 6.7.3)
//
// Keyed MD5 and Meticulous Keyed MD5 construction: thin wrappers that
// delegate to the generic digest helpers in sha1.go. MD5 output is 16
// bytes; RFC 5880 §6.7.3 lays out the same body layout as the SHA1
// variants.
//
// RFC 5880 §6.7.2 recommends SHA1 over MD5 but MD5 remains widely
// deployed so ze implements both. Simple Password (§6.7.2, Type 1)
// is rejected at config parse time and has no signer/verifier here.
package auth

import (
	"crypto/md5" //nolint:gosec // RFC 5880 §6.7.3 mandates MD5; not used for general cryptographic integrity

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
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
