// VALIDATES: RFC 5880 Section 6.8.6 -- a received Control packet with the A
// bit set is authenticated under the rules of Section 6.7 for the session's
// own bfd.AuthType, for every Auth Type Ze implements, through handleInbound.
// PREVENTS: a verifier that ignores bfd.AuthType, one that checks every
// packet as Keyed SHA1, and one that accepts a packet signed with the shared
// key under another Auth Type than the session's.
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/auth"
	"github.com/ze-software/ze/internal/component/bfd/packet"
)

// rfc5880AuthTypes is every Auth Type Ze implements (RFC 5880 Sections 6.7.2
// to 6.7.4).
var rfc5880AuthTypes = []struct {
	name string
	wire uint8
}{
	{"simple-password", packet.AuthTypeSimplePassword},
	{"keyed-md5", packet.AuthTypeKeyedMD5},
	{"meticulous-keyed-md5", packet.AuthTypeMeticulousKeyedMD5},
	{"keyed-sha1", packet.AuthTypeKeyedSHA1},
	{"meticulous-keyed-sha1", packet.AuthTypeMeticulousKeyedSHA1},
}

// rfc5880SignerOfType builds a signer holding the engine tests' shared key
// id and secret under Auth Type wire.
func rfc5880SignerOfType(t *testing.T, wire uint8) auth.Signer {
	t.Helper()
	signer, err := auth.NewSigner(auth.Settings{
		Type:       wire,
		KeyID:      5,
		Secret:     rfc5880Secret,
		Meticulous: wire == packet.AuthTypeMeticulousKeyedMD5 || wire == packet.AuthTypeMeticulousKeyedSHA1,
	})
	if err != nil {
		t.Fatalf("NewSigner type %d: %v", wire, err)
	}
	return signer
}

// RFC requirement: RFC5880-6.8.6-11 positive -- "If the A bit is set, the
// packet MUST be authenticated under the rules of section 6.7, based on the
// authentication type in use (bfd.AuthType)." For each of the five Auth Types,
// a session configured with it delivers a packet correctly signed under that
// type: handleInbound passes it and the session learns the peer's
// discriminator.
func TestRFC5880AuthenticatedUnderEachSessionAuthType(t *testing.T) {
	for _, at := range rfc5880AuthTypes {
		t.Run(at.name, func(t *testing.T) {
			l, _, key, signer := rfc5880AuthLoop(t, at.wire)
			m := machineFor(t, l, key)

			l.handleInbound(rfc5880SignedInbound(key, signer, m.LocalDiscriminator(), 1000))

			if got := m.RemoteDiscriminator(); got != peerMyDiscr {
				t.Fatalf("a packet signed under the session's own type %s was dropped: RemoteDiscr = %d, want %d",
					at.name, got, peerMyDiscr)
			}
		})
	}
}

// RFC requirement: RFC5880-6.8.6-11 negative -- the authentication is based
// on bfd.AuthType, so a packet signed with the shared key id and secret under
// any OTHER Auth Type fails it. For each configured type, every other type's
// correctly signed packet (a valid digest over its own bytes, including Keyed
// against Meticulous of the same hash) is discarded: the session never learns
// the peer's discriminator.
func TestRFC5880AuthenticatedPacketOfAnotherAuthTypeDiscarded(t *testing.T) {
	for _, session := range rfc5880AuthTypes {
		for _, sent := range rfc5880AuthTypes {
			if sent.wire == session.wire {
				continue
			}
			t.Run(session.name+"/"+sent.name, func(t *testing.T) {
				l, _, key, _ := rfc5880AuthLoop(t, session.wire)
				m := machineFor(t, l, key)

				l.handleInbound(rfc5880SignedInbound(key, rfc5880SignerOfType(t, sent.wire), m.LocalDiscriminator(), 1000))

				if got := m.RemoteDiscriminator(); got != 0 {
					t.Fatalf("session using %s accepted a packet authenticated as %s: RemoteDiscr = %d",
						session.name, sent.name, got)
				}
			})
		}
	}
}
