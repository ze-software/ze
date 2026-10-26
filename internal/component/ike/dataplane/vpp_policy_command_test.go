//go:build ze_vpp

// VALIDATES: the VPP backend names the vppctl command that removes a policy it
// installed, from the entry it recorded, after the policy's SA is gone (PolicyDeleteCommand).
// PREVENTS: an operator told to run `ip xfrm policy delete` on a VPP dataplane, where it
// removes nothing, and a command rebuilt without the SA id VPP matches the delete on.

package dataplane

import (
	"errors"
	"strings"
	"testing"

	"go.fd.io/govpp/binapi/ipsec"

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

// TestVPPStrandedPolicyRetryDeletesRecordedEntry: method: install testSPParams over
// testSAParams, make VPP refuse the SPD entry delete, remove the SA as the engine's
// teardown does next (removeChildSAOutgoing), then retry the removal with VPP accepting.
// The retry must send the delete of the entry the add sent, its SA id included, and
// only then forget it; a third removal must answer that the backend holds no such entry.
// PREVENTS: the retry rebuilding the entry, finding no SAD id for the removed SA, and
// answering ErrNotSupported, which the engine reads as the policy gone (policyGone), so
// the cross-peer record was released while VPP still held the PROTECT entry
// (RFC 3948 Section 5.1).
func TestVPPStrandedPolicyRetryDeletesRecordedEntry(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	p := testSPParams()
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}
	added := sentSPD(t, ch).Entry

	ch.refuse = map[string]int32{(&ipsec.IpsecSpdEntryAddDelV2{}).GetMessageName(): -1}
	err := b.RemovePolicyParams(p)
	if err == nil {
		t.Fatal("RemovePolicyParams succeeded with VPP refusing the delete")
	}
	if errors.Is(err, ErrNotSupported) {
		t.Fatalf("a refused delete answered ErrNotSupported, which the engine reads as gone: %v", err)
	}
	ch.refuse = nil
	sa := testSAParams()
	if err := b.RemoveSA(sa.SPI, sa.Dst, sa.Proto); err != nil {
		t.Fatalf("RemoveSA: %v", err)
	}

	ch.sent = nil
	if err := b.RemovePolicyParams(p); err != nil {
		t.Fatalf("the retry after the SA was removed failed: %v", err)
	}
	if req := sentSPD(t, ch); req.IsAdd || req.Entry != added {
		t.Fatalf("the retry sent add=%v entry %+v, want the delete of the entry the add sent %+v", req.IsAdd, req.Entry, added)
	}
	if len(b.spdEntries) != 0 {
		t.Fatalf("the backend still records %d SPD entries after the retry deleted the only one", len(b.spdEntries))
	}
	if err := b.RemovePolicyParams(p); !errors.Is(err, ErrNotSupported) {
		t.Fatalf("removing the deleted policy again answered %v, want ErrNotSupported (no such entry held)", err)
	}
}

// TestVPPRemovalSparesLiveSAEntry: method: install two PROTECT entries with one selector
// over two SAs, as a make-before-break rekey does, remove the first SA, then remove the
// first policy. The delete must name the first SA's id and the entry naming the live
// second SA must stay recorded.
// PREVENTS: a removal matched on the selector alone deleting the live Child SA's entry.
func TestVPPRemovalSparesLiveSAEntry(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	first := testSPParams()
	second := testSPParams()
	second.SAID = 0x55667788
	sa := testSAParams()
	sa.SPI = second.SAID
	if err := b.InstallSA(sa); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	for _, p := range []SPParams{first, second} {
		if err := b.InstallPolicy(p); err != nil {
			t.Fatalf("InstallPolicy: %v", err)
		}
	}
	if len(b.spdEntries) != 2 {
		t.Fatalf("setup: %d recorded entries, want 2", len(b.spdEntries))
	}
	live := b.spdEntries[1]
	firstSA := testSAParams()
	if err := b.RemoveSA(firstSA.SPI, firstSA.Dst, firstSA.Proto); err != nil {
		t.Fatalf("RemoveSA: %v", err)
	}

	ch.sent = nil
	if err := b.RemovePolicyParams(first); err != nil {
		t.Fatalf("RemovePolicyParams: %v", err)
	}
	if req := sentSPD(t, ch); req.Entry.SaID == live.SaID {
		t.Fatalf("the removal deleted the entry of the live SA %d", live.SaID)
	}
	if len(b.spdEntries) != 1 || b.spdEntries[0] != live {
		t.Fatalf("recorded entries %+v, want only the live SA's %+v", b.spdEntries, live)
	}
}

// TestVPPRemovalWhileBothSAsLive: method: install two PROTECT entries with one selector
// over two SAs, as a make-before-break rekey does, keep both SAs installed, then remove
// the first policy. The delete must name the first SA's SAD id, and the entry naming the
// second SA must stay recorded.
// PREVENTS: entryOfSA accepting every PROTECT entry while p's SA is live, which makes
// both entries candidates, so the removal of the old Child SA's policy during a rekey
// is refused as ambiguous, or deletes the new Child SA's entry.
func TestVPPRemovalWhileBothSAsLive(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	first := testSPParams()
	second := testSPParams()
	second.SAID = 0x55667788
	sa := testSAParams()
	sa.SPI = second.SAID
	if err := b.InstallSA(sa); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	for _, p := range []SPParams{first, second} {
		if err := b.InstallPolicy(p); err != nil {
			t.Fatalf("InstallPolicy: %v", err)
		}
	}
	if len(b.spdEntries) != 2 {
		t.Fatalf("setup: %d recorded entries, want 2", len(b.spdEntries))
	}
	identity, err := saIdentityOf(first.SAID, first.TunnelDst, first.Proto)
	if err != nil {
		t.Fatalf("saIdentityOf: %v", err)
	}
	firstID, ok := b.sadIDs[identity]
	if !ok {
		t.Fatal("setup: the first SA has no SAD id")
	}
	kept := b.spdEntries[1]
	if kept.SaID == firstID {
		t.Fatalf("setup: both entries name SAD id %d", firstID)
	}

	ch.sent = nil
	if err := b.RemovePolicyParams(first); err != nil {
		t.Fatalf("RemovePolicyParams with both SAs live: %v", err)
	}
	if req := sentSPD(t, ch); req.IsAdd || req.Entry.SaID != firstID {
		t.Fatalf("the removal sent add=%v SaID %d, want the delete of the first SA's entry, SaID %d", req.IsAdd, req.Entry.SaID, firstID)
	}
	if len(b.spdEntries) != 1 || b.spdEntries[0] != kept {
		t.Fatalf("recorded entries %+v, want only the second SA's %+v", b.spdEntries, kept)
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
