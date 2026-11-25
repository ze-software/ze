// Design: docs/features/interfaces.md -- plugin-owned devices (bridges and links)
// Related: ifacevpp.go -- CreateOwnedBridge and CreateOwnedLink, the refusals under test

package ifacevpp

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

// TestVPPBackendRefusesOwnedLink proves the VPP backend refuses an owned link
// and an owned bridge outright, before it reaches VPP, and that each refusal
// names the device and the backend leaf that makes it possible: exact or
// refuse, never a bridge domain or a tap standing in. Method: call both on a
// backend with no VPP channel, which a refusal that reached VPP would need.
func TestVPPBackendRefusesOwnedLink(t *testing.T) {
	b := &vppBackendImpl{names: newNameMap()}

	err := b.CreateOwnedLink(iface.OwnedLinkSpec{HostEnd: "zfo-7-1", Bridge: "zfo-wire", MTU: 1500, PeerName: "eth0"})
	if err == nil {
		t.Fatalf("the VPP backend accepted an owned link")
	}
	for _, want := range []string{"zfo-7-1", "network namespace", "interface backend kernel"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the owned link refusal does not say %q: %v", want, err)
		}
	}

	err = b.CreateOwnedBridge(iface.OwnedBridgeSpec{Name: "zfo-wire", MTU: 1500})
	if err == nil {
		t.Fatalf("the VPP backend accepted an owned bridge")
	}
	for _, want := range []string{"zfo-wire", "interface backend kernel"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the owned bridge refusal does not say %q: %v", want, err)
		}
	}
}
