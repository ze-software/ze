// VALIDATES: a Child SA whose installed proposal does not resolve through the
// crypto registry is published to the inventory as UNKNOWN transforms
// PREVENTS: a reader outside the IKE component sizing ESP from Transform ID 0
// (ai/rules/principles.md: a silently wrong value)
package engine

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
)

// TestInventoryTunnelCarriesTheTransformError proves tunnelOf hands the reader the
// engine's resolve error and leaves every transform field unset, so the tunnel reads
// as up with unknown transforms rather than up with ENCR 0 and AUTH_NONE.
//
// Method: a PeerInfo in the shape Info produces when resolveESPTransforms fails
// (the encryption name set from the proposal, the typed fields unset, the error
// named), converted once; the up facts must still pass through.
func TestInventoryTunnelCarriesTheTransformError(t *testing.T) {
	resolveErr := crypto.ErrUnsupportedAlgorithm
	info := PeerInfo{
		PeerName:        "site-c",
		HasChild:        true,
		ChildRemoteAddr: netip.MustParseAddr("198.51.100.7"),
		ChildMode:       modeTunnel,
		ESPEncryption:   "aes128gcm",
		ESPTransformErr: resolveErr,
	}

	tunnel := tunnelOf(&info)

	if !tunnel.Up {
		t.Fatal("a peer holding a Child SA was reported down")
	}
	if !errors.Is(tunnel.TransformErr, resolveErr) {
		t.Fatalf("TransformErr = %v; want the engine's resolve error", tunnel.TransformErr)
	}
	if tunnel.EncryptionName != "" {
		t.Errorf("EncryptionName = %q; an unresolved transform names nothing", tunnel.EncryptionName)
	}
	if tunnel.IntegrityName != "" {
		t.Errorf("IntegrityName = %q; an unresolved transform names nothing", tunnel.IntegrityName)
	}
	if tunnel.Encryption != 0 || tunnel.EncryptionKeyBits != 0 || tunnel.Integrity != 0 {
		t.Errorf("typed transforms = %d/%d/%d; want unset beside TransformErr",
			tunnel.Encryption, tunnel.EncryptionKeyBits, tunnel.Integrity)
	}
}
