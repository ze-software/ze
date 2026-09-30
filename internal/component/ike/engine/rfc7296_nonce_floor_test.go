package engine

import "testing"

// nfloorOctetsMin is RFC 7296 Section 2.10's 128-bit nonce floor, in octets.
const nfloorOctetsMin = 128 / 8

// VALIDATES: every nonce ze sends is at least 128 bits long.
// PREVENTS: a sender-side nonce shorter than the floor the wire codec refuses on receipt,
// which only a peer would notice.
//
// METHOD: the nonces ze emits in IKE_SA_INIT (initiator Ni and responder Nr), in a Child SA
// rekey and in an IKE SA rekey (the rekey nonces decrypted off the wire) are each compared
// with 16 octets. The receive-side refusal of a 15-octet nonce is the wire package's
// TestNonceLengthBounds.
//
// RFC requirement: RFC7296-2.10-2 positive -- every nonce ze sends (IKE_SA_INIT Ni and Nr,
// Child SA rekey Ni and Nr, IKE SA rekey Ni and Nr) is at least 16 octets, 128 bits.
func TestRFC7296EmittedNoncesAreAtLeast128Bits(t *testing.T) {
	nonces := nszEmittedNonces(t)
	if len(nonces) != 6 {
		t.Fatalf("collected %d emitted nonces, want 6", len(nonces))
	}
	for where, nonce := range nonces {
		if len(nonce) < nfloorOctetsMin {
			t.Errorf("%s is %d octets (%d bits), below the 128-bit floor",
				where, len(nonce), len(nonce)*8)
		}
	}
}
