// Design: docs/architecture/ospf/ospf-12-auth.md -- cryptographic sequence number per neighbor
// Related: auth_keystore.go -- authStore.verify records the sequence only after the digest verifies.
//
// VALIDATES: RFC 2328 Section D.3: the neighbor's cryptographic sequence number is set from a
// packet only when that packet "is accepted as authentic".
// PREVENTS: a forged packet with a high sequence number poisoning the stored value, so every
// later genuine packet from that neighbor is discarded as a replay.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
)

// RFC requirement: RFC2328-D.3-2 negative -- a packet that fails authentication does not set the stored sequence number: after a genuine AuType 2 Hello at sequence 10, a Hello at sequence 1000 whose digest does not verify is discarded with reason "digest-mismatch" and leaves the neighbor's slot at 10, so a genuine Hello at sequence 12 still verifies and becomes the stored value (authStore.verify, auth_keystore.go).
func TestRFC2328FailedAuthenticationLeavesSequenceAlone(t *testing.T) {
	// Goal: prove the "accepted as authentic" condition of Section D.3. Method: sign one
	// packet with a secret the chain does not hold and check the replay slot is unchanged.
	s := newAuthStore()
	s.configure(authCfg(keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "topsecret"}))
	peer := ridOf("2.2.2.2")
	slot := replayKey{iface: "eth0", rid: peer, pktType: replaySlotNeighbor}

	sign := func(secret string, seq uint64) []byte {
		p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello, AuType: packet.AuTypeCryptographic}, Hello: &packet.Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 10, DeadInterval: 40}}
		buf := make([]byte, p.EncodedLen())
		n := p.WriteTo(buf, 0)
		signed, err := packet.Sign(buf[:n], packet.AuTypeCryptographic, packet.AuthKey{KeyID: 1, Algorithm: "hmac-sha-256", Secret: []byte(secret)}, seq, [4]byte{})
		require.NoError(t, err)
		return signed
	}

	_, ok := s.verify("eth0", peer, [4]byte{}, sign("topsecret", 10))
	require.True(t, ok, "genuine Hello at sequence 10 accepted")
	require.Equal(t, uint64(10), s.recvSeq[slot], "the accepted sequence is stored")

	reason, forged := s.verify("eth0", peer, [4]byte{}, sign("wrongsecret", 1000))
	assert.False(t, forged, "a Hello whose digest does not verify is discarded")
	assert.Equal(t, "digest-mismatch", reason)
	assert.Equal(t, uint64(10), s.recvSeq[slot], "a packet that failed authentication left the stored sequence alone")

	reason, genuine := s.verify("eth0", peer, [4]byte{}, sign("topsecret", 12))
	assert.True(t, genuine, "genuine Hello at sequence 12 verifies: the forged 1000 was never stored (reason %q)", reason)
	assert.Equal(t, uint64(12), s.recvSeq[slot], "the genuine sequence 12 becomes the stored value")
}
