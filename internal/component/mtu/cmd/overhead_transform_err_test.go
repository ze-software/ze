// VALIDATES: a tunnel whose transforms the IKE engine could not resolve is
// refused by the ESP overhead arithmetic and shows no transform
// PREVENTS: show mtu sizing a tunnel from transform fields the engine marked unknown
package cmd

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
)

// TestESPOverheadUnresolvedTransformRefuses proves deriveESPOverhead checks
// TransformErr before the transform fields: even with fields that would size a
// packet, a tunnel carrying the engine's resolve error is refused, the refusal
// names that error, and the row carries no transform text.
//
// Method: a valid up tunnel from the per-transform table, with TransformErr set.
func TestESPOverheadUnresolvedTransformRefuses(t *testing.T) {
	tunnel := upTunnel(t, &espTransformCases[0], netip.MustParseAddr("192.0.2.1"), false)
	tunnel.TransformErr = crypto.ErrUnsupportedAlgorithm

	got, err := deriveESPOverhead(&tunnel)
	if !errors.Is(err, errOverheadRefused) {
		t.Fatalf("err = %v, overhead = %+v; want errOverheadRefused", err, got)
	}
	if !errors.Is(err, crypto.ErrUnsupportedAlgorithm) {
		t.Errorf("err = %v; want it to carry the engine's resolve error", err)
	}

	row := tunnelRow(&tunnel)
	if transform, ok := row[fieldTransform]; ok {
		t.Errorf("row transform = %v; an unresolved transform shows none", transform)
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("err = %q; want it to say the transforms are unknown", err)
	}
}
