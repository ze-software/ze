// Design: docs/architecture/testing/qemu-integration.md -- WireGuard kernel evidence.
// Related: wireguardproof.go -- the receiver-side handshake and traffic oracle.
package testdeployment

import "testing"

// TestWireGuardReceiptRequiresMatchingPeer proves that an outbound-only counter,
// absent handshake, malformed output or another peer cannot satisfy the oracle.
func TestWireGuardReceiptRequiresMatchingPeer(t *testing.T) {
	for _, test := range []struct {
		name, handshake, transfer string
		valid                     bool
	}{
		{"received", "peer 123", "peer 128 128", true},
		{"no handshake", "peer 0", "peer 128 128", false},
		{"sent only", "peer 123", "peer 0 128", false},
		{"wrong handshake peer", "other 123", "peer 128 128", false},
		{"wrong receiving peer", "peer 123", "other 128 128", false},
		{"missing handshake", "", "peer 128 128", false},
		{"missing counter", "peer 123", "peer", false},
		{"invalid timestamp", "peer no", "peer 128 128", false},
		{"invalid bytes", "peer 123", "peer no 128", false},
		{"extra peer", "peer 123 other 124", "peer 128 128", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := requireWireGuardReceipt("peer", test.handshake, test.transfer)
			if (err == nil) != test.valid {
				t.Fatalf("receipt verdict: %v; want valid=%t", err, test.valid)
			}
		})
	}
}
