//go:build ze_vpp

// VALIDATES: the VPP backend names the vppctl command that removes a policy it
// installed, from the entry it recorded, after the policy's SA is gone (PolicyDeleteCommand).
// PREVENTS: an operator told to run `ip xfrm policy delete` on a VPP dataplane, where it
// removes nothing, and a command rebuilt without the SA id VPP matches the delete on.

package dataplane

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestVPPPolicyDeleteCommandAfterSAGone: method: install testSPParams over the SA
// testSAParams installed, forget the SA's SAD id the way RemoveSA does, and ask for the
// command. It must be the vppctl `ipsec policy del` line naming the recorded SA id and
// every field VPP matches, and must not be an `ip xfrm` command.
func TestVPPPolicyDeleteCommandAfterSAGone(t *testing.T) {
	b, _ := installedPolicyBackend(t)
	p := testSPParams()
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}
	identity, err := saIdentityOf(p.SAID, p.TunnelDst, p.Proto)
	if err != nil {
		t.Fatalf("saIdentityOf: %v", err)
	}
	sadID, ok := b.sadIDs[identity]
	if !ok {
		t.Fatal("setup: the installed SA has no SAD id")
	}
	delete(b.sadIDs, identity)

	got, err := b.PolicyDeleteCommand(p)
	if err != nil {
		t.Fatalf("PolicyDeleteCommand: %v", err)
	}
	var want textbuf.Buffer
	want.Str("vppctl ipsec policy del spd ").Uint32(b.spdID).Str(" priority -2000 outbound action protect sa ").
		Uint32(sadID).Str(" protocol 255 local-ip-range 192.0.2.0 - 192.0.2.255 ").
		Str("remote-ip-range 198.51.100.0 - 198.51.100.255 local-port-range 0 - 65535 remote-port-range 0 - 65535")
	if got != string(want.Bytes()) {
		t.Fatalf("command\n got %q\nwant %q", got, string(want.Bytes()))
	}
}

// TestVPPPolicyDeleteCommandRefusesUnknownEntry: method: ask for the command of a policy
// the backend never installed. It must refuse, naming the vppctl command that lists the
// SPD, rather than write a command for an entry that does not exist.
func TestVPPPolicyDeleteCommandRefusesUnknownEntry(t *testing.T) {
	b, _ := installedPolicyBackend(t)
	_, err := b.PolicyDeleteCommand(testSPParams())
	if err == nil {
		t.Fatal("named a command for a policy the backend never installed")
	}
	if !strings.Contains(err.Error(), "vppctl show ipsec spd") {
		t.Fatalf("refusal names no command to find the entry: %v", err)
	}
}
