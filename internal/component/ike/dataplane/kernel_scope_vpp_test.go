// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact scope refusal.
// Related: vpp.go -- vppUnsupportedSA refuses options the SAD cannot enforce.
// Upstream: https://github.com/FDio/vpp, ipsec_sad_entry_v3 has no selector set.

//go:build ze_vpp

package dataplane

import (
	"errors"
	"net"
	"strings"
	"testing"
)

// TestVPPStateScopeRefusedBeforeAPI drives InstallSA with an explicit scope and
// proves that an unscoped SAD entry is never installed in its place.
//
// The scope is a valid one, built by NewSAStateScope as the engine builds it, so the
// refusal is the backend's answer to a scope it cannot enforce and not to an
// uninitialized one.
func TestVPPStateScopeRefusedBeforeAPI(t *testing.T) {
	b, ch := newCapturingBackend()
	p := testSAParams()
	scope, err := NewSAStateScope([]SASelector{{
		Src: &net.IPNet{IP: net.IP{10, 1, 0, 0}, Mask: net.CIDRMask(16, 32)},
		Dst: &net.IPNet{IP: net.IP{10, 2, 0, 0}, Mask: net.CIDRMask(16, 32)},
	}}, false)
	if err != nil {
		t.Fatalf("build the scope the backend is handed: %v", err)
	}
	p.Scope = scope
	err = b.InstallSA(p)
	if !errors.Is(err, ErrSAOptionRefused) || errors.Is(err, ErrNotSupported) {
		t.Fatalf("explicit scope silently dropped or degraded: %v", err)
	}
	if !strings.Contains(err.Error(), "vpp") || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("scope refusal does not name the unsupported backend option: %v", err)
	}
	if len(ch.sent) != 0 || len(ch.dumps) != 0 {
		t.Fatalf("refused scope sent %d requests and %d dumps", len(ch.sent), len(ch.dumps))
	}
	if len(b.sadIDs) != 0 {
		t.Fatalf("refused scope allocated SAD identities: %+v", b.sadIDs)
	}
}
