// VALIDATES: the VPP backend itself puts an explicit IKE BYPASS in its SPD for every
// PROTECT entry it installs: UDP ports 500 and 4500 at the policy's local tunnel
// endpoint, outbound and inbound, sent BEFORE the PROTECT entry and ranked ahead of
// every entry a caller can ask for. Read from the binary-API messages the backend
// sends, through the capturing channel of rfc7296_vpp_message_test.go.
// PREVENTS: a Child SA whose selector covers the two IKE endpoints handing Ze's own
// IKE to ESP on VPP, where the per-socket policy that exempts IKE on Linux
// (BypassIKESocket) decides nothing, and VPP dropping the peer's IKE on an
// interface its SPD is bound to.

//go:build ze_vpp

package dataplane

import (
	"net"
	"net/netip"
	"testing"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/ipsec"
	"go.fd.io/govpp/binapi/ipsec_types"
)

// sentSPDEntries returns every SPD entry message in what the backend sent, in order.
func sentSPDEntries(ch *capturingChannel) []*ipsec.IpsecSpdEntryAddDelV2 {
	var entries []*ipsec.IpsecSpdEntryAddDelV2
	for _, msg := range ch.sent {
		if req, ok := msg.(*ipsec.IpsecSpdEntryAddDelV2); ok {
			entries = append(entries, req)
		}
	}
	return entries
}

// isIKEBypassEntry reports whether an SPD entry lets UDP at local, on one of the two
// IKE ports, through unprotected from any remote address and port. It reads the fields
// VPP matches on, never the rank, so a test can ask about the rank apart.
func isIKEBypassEntry(entry *ipsec_types.IpsecSpdEntryV2, local net.IP) bool {
	host := ip_types.NewAddress(local)
	anyStart, anyStop := ip_types.NewAddress(net.IPv4zero), ip_types.NewAddress(net.IPv4bcast)
	if local.To4() == nil {
		anyStart = ip_types.NewAddress(net.IPv6zero)
		anyStop = ip_types.NewAddress(net.IP{
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		})
	}
	if entry.Policy != ipsec_types.IPSEC_API_SPD_ACTION_BYPASS || entry.Protocol != 17 {
		return false
	}
	if entry.LocalAddressStart != host || entry.LocalAddressStop != host {
		return false
	}
	if entry.RemoteAddressStart != anyStart || entry.RemoteAddressStop != anyStop {
		return false
	}
	if entry.LocalPortStart != entry.LocalPortStop {
		return false
	}
	if entry.LocalPortStart != 500 && entry.LocalPortStart != 4500 {
		return false
	}
	return entry.RemotePortStart == 0 && entry.RemotePortStop == 65535
}

// requireIKEBypassAheadOfProtect asserts the property the SPD owes for one PROTECT
// install: among the SPD entry adds sent, the four IKE BYPASS entries of local (each
// port, each direction) are all sent before the first PROTECT entry, and every one of
// them outranks every PROTECT entry in VPP's order, where the higher priority is
// searched first.
func requireIKEBypassAheadOfProtect(t *testing.T, ch *capturingChannel, local net.IP) {
	t.Helper()
	type bypassKey struct {
		port     uint16
		outbound bool
	}
	seen := make(map[bypassKey]int32)
	protects := 0
	for _, req := range sentSPDEntries(ch) {
		if !req.IsAdd {
			continue
		}
		requireResolvable(t, req)
		if req.Entry.Policy == ipsec_types.IPSEC_API_SPD_ACTION_PROTECT {
			protects++
			for key, rank := range seen {
				if rank <= req.Entry.Priority {
					t.Errorf("the IKE bypass for port %d (outbound %v) has VPP priority %d and the protect entry %d: VPP searches the higher first, so the protect entry would capture Ze's own IKE",
						key.port, key.outbound, rank, req.Entry.Priority)
				}
			}
			if len(seen) != 4 {
				t.Errorf("a protect entry was sent after %d of the 4 IKE bypass entries of %v, want all 4 in VPP first: %v",
					len(seen), local, seen)
			}
			continue
		}
		if isIKEBypassEntry(&req.Entry, local) {
			seen[bypassKey{port: req.Entry.LocalPortStart, outbound: req.Entry.IsOutbound}] = req.Entry.Priority
		}
	}
	if protects == 0 {
		t.Fatal("no protect entry was sent, so nothing was asserted about the IKE bypass")
	}
	for _, port := range []uint16{500, 4500} {
		for _, outbound := range []bool{true, false} {
			if _, ok := seen[bypassKey{port: port, outbound: outbound}]; !ok {
				t.Errorf("no IKE bypass entry for UDP port %d at %v was sent (outbound %v)", port, local, outbound)
			}
		}
	}
}

// TestVPPProtectInstallsIKEBypassFirst: method: install an OUTBOUND protect policy whose
// selector is 0.0.0.0/0 to 0.0.0.0/0 for every protocol, the site-to-site selector that
// covers the two IKE endpoints, and read the SPD entry adds the backend sent.
// It must send, before the protect entry, a BYPASS for UDP local port 500 and one for
// 4500 at the policy's tunnel source 192.0.2.1, outbound and inbound, each ranked ahead
// of the protect entry in VPP's descending order.
//
// RFC 4301 Section 5.2: "(This implies that IKE traffic MUST have an explicit BYPASS entry in
// the SPD.)"
//
// MUTATION: in installProtectEntry, drop the ensureIKEBypass call: no bypass entry is
// sent and the four "no IKE bypass entry" errors fire. Send the protect entry before
// it: "a protect entry was sent after 0 of the 4". Set vppIKEBypassPriority to
// -PriorityChildSA-1: the rank error fires.
func TestVPPProtectInstallsIKEBypassFirst(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	p := testSPParams()
	p.Src = &net.IPNet{IP: net.IPv4zero.To4(), Mask: net.CIDRMask(0, 32)}
	p.Dst = &net.IPNet{IP: net.IPv4zero.To4(), Mask: net.CIDRMask(0, 32)}
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}
	requireIKEBypassAheadOfProtect(t, ch, p.TunnelSrc)

	for _, req := range sentSPDEntries(ch) {
		if req.Entry.Policy != ipsec_types.IPSEC_API_SPD_ACTION_BYPASS {
			continue
		}
		if req.Entry.SpdID != b.spdID {
			t.Errorf("an IKE bypass entry went to SPD %d, want the SPD the protect entry is in (%d)", req.Entry.SpdID, b.spdID)
		}
		if req.Entry.Priority != vppIKEBypassPriority {
			t.Errorf("an IKE bypass entry has VPP priority %d, want %d, the rank no caller's entry reaches", req.Entry.Priority, vppIKEBypassPriority)
		}
	}
}

// TestVPPInboundProtectBypassesIKEAtTheTunnelDestination: method: install an INBOUND SA
// and its protect policy, written as the engine writes them (the tunnel source is the
// peer, the destination this node), and read the entries sent. The IKE bypass must name
// the tunnel DESTINATION, this node's own address, on the local side.
// PREVENTS: the bypass exempting the peer's address, which would leave Ze's own IKE
// captured and pass the peer's unrelated UDP in clear.
//
// MUTATION: in vppIKEEndpoint, return p.TunnelSrc for both directions: the entries name
// 198.51.100.1 and the four "no IKE bypass entry ... at 192.0.2.1" errors fire.
func TestVPPInboundProtectBypassesIKEAtTheTunnelDestination(t *testing.T) {
	b, ch := newCapturingBackend()
	sa := testSAParams()
	sa.Dir = SADirIn
	sa.Src, sa.Dst = sa.Dst, sa.Src
	if err := b.InstallSA(sa); err != nil {
		t.Fatalf("InstallSA: %v", err)
	}
	ch.sent = nil

	p := testSPParams()
	p.Dir = SADirIn
	p.Src, p.Dst = p.Dst, p.Src
	p.TunnelSrc, p.TunnelDst = p.TunnelDst, p.TunnelSrc
	if err := b.InstallPolicy(p); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}
	requireIKEBypassAheadOfProtect(t, ch, net.ParseIP("192.0.2.1"))
}

// TestVPPIKEBypassIPv6Endpoint: method: build the bypass entries of an IPv6 local
// endpoint. Each must name that one address locally and the whole IPv6 space remotely.
// PREVENTS: an IPv6 tunnel getting IPv4 ranges, which VPP stores in a chain no IPv6
// packet is matched against.
func TestVPPIKEBypassIPv6Endpoint(t *testing.T) {
	local := net.ParseIP("2001:db8::1")
	entries := vppIKEBypassEntries(7, netip.MustParseAddr("2001:db8::1"))
	if len(entries) != 4 {
		t.Fatalf("built %d entries, want 4 (two ports, two directions)", len(entries))
	}
	for i := range entries {
		if !isIKEBypassEntry(&entries[i], local) {
			t.Errorf("entry %d is not an IKE bypass at %v: %+v", i, local, entries[i])
		}
		if entries[i].SpdID != 7 {
			t.Errorf("entry %d names SPD %d, want 7", i, entries[i].SpdID)
		}
	}
}

// TestVPPIKEBypassLivesWithItsProtectEntries: method: install two protect policies that
// share a local endpoint (a second SA to the same peer, the shape of a rekey), then
// remove them one at a time. The second install must add no bypass entry; removing the
// first policy must delete the policy alone; removing the last must delete the four
// bypass entries AFTER the protect entry, and leave the backend recording none.
// PREVENTS: a rekey stripping the bypass a live Child SA still needs, and a bypass
// outliving every policy it was installed for.
//
// MUTATION: in releaseProtectIKEBypass, drop the loop over b.protectLocal: the first
// removal sends 5 deletes. Drop the releaseIKEBypass call: the last removal sends 1.
func TestVPPIKEBypassLivesWithItsProtectEntries(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	first := testSPParams()
	if err := b.InstallPolicy(first); err != nil {
		t.Fatalf("InstallPolicy: %v", err)
	}

	rekeyed := testSAParams()
	rekeyed.SPI = 0x55667788
	if err := b.InstallSA(rekeyed); err != nil {
		t.Fatalf("InstallSA of the second SA: %v", err)
	}
	second := testSPParams()
	second.SAID = rekeyed.SPI
	ch.sent = nil
	if err := b.InstallPolicy(second); err != nil {
		t.Fatalf("InstallPolicy of the second policy: %v", err)
	}
	if got := sentSPDEntries(ch); len(got) != 1 || got[0].Entry.Policy != ipsec_types.IPSEC_API_SPD_ACTION_PROTECT {
		t.Fatalf("the second policy on one local endpoint sent %d SPD entries, want its protect entry alone", len(got))
	}

	ch.sent = nil
	if err := b.RemovePolicyParams(first); err != nil {
		t.Fatalf("RemovePolicyParams of the first policy: %v", err)
	}
	if got := sentSPDEntries(ch); len(got) != 1 || got[0].IsAdd || got[0].Entry.Policy != ipsec_types.IPSEC_API_SPD_ACTION_PROTECT {
		t.Fatalf("removing one of two policies sent %d SPD entries, want the delete of its protect entry alone: the other policy still needs the IKE bypass", len(got))
	}

	ch.sent = nil
	if err := b.RemovePolicyParams(second); err != nil {
		t.Fatalf("RemovePolicyParams of the last policy: %v", err)
	}
	got := sentSPDEntries(ch)
	if len(got) != 5 {
		t.Fatalf("removing the last policy sent %d SPD entries, want 5 (its protect entry, then the four IKE bypass entries)", len(got))
	}
	if got[0].IsAdd || got[0].Entry.Policy != ipsec_types.IPSEC_API_SPD_ACTION_PROTECT {
		t.Errorf("the first delete is %+v, want the protect entry: the bypass must outlive it", got[0].Entry)
	}
	for i, req := range got[1:] {
		if req.IsAdd || !isIKEBypassEntry(&req.Entry, first.TunnelSrc) {
			t.Errorf("delete %d is {IsAdd:%v %+v}, want the delete of an IKE bypass entry", i+1, req.IsAdd, req.Entry)
		}
	}
	if len(b.ikeBypass) != 0 || len(b.protectLocal) != 0 {
		t.Errorf("the backend still records %d bypassed endpoints and %d protect entries after the last removal", len(b.ikeBypass), len(b.protectLocal))
	}
}

// TestVPPProtectRefusedTakesItsIKEBypassBack: method: VPP accepts the four bypass
// entries and refuses the protect entry alone. The install must fail, delete the four
// bypass entries it added, then unbind and delete the SPD it created, and record
// nothing.
// PREVENTS: a refused Child SA policy leaving an SPD bound to the interface with only
// bypass entries in it.
//
// MUTATION: in installProtectEntry, drop releaseIKEBypass from the entry-add failure
// path: no bypass delete is sent and b.ikeBypass keeps one endpoint.
func TestVPPProtectRefusedTakesItsIKEBypassBack(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	ch.refuseIf = func(msg api.Message) bool {
		req, ok := msg.(*ipsec.IpsecSpdEntryAddDelV2)
		return ok && req.Entry.Policy == ipsec_types.IPSEC_API_SPD_ACTION_PROTECT
	}
	p := testSPParams()
	if err := b.InstallPolicy(p); err == nil {
		t.Fatal("InstallPolicy reported success for a protect entry VPP refused")
	}

	deleted := 0
	for _, req := range sentSPDEntries(ch) {
		if !req.IsAdd && isIKEBypassEntry(&req.Entry, p.TunnelSrc) {
			deleted++
		}
	}
	if deleted != 4 {
		t.Errorf("the refused install deleted %d IKE bypass entries, want the 4 it added", deleted)
	}
	if _, ok := ch.sent[len(ch.sent)-1].(*ipsec.IpsecSpdAddDel); !ok {
		t.Errorf("the last message is %T, want the delete of the SPD the call created, after its entries", ch.sent[len(ch.sent)-1])
	}
	if b.spdID != 0 || len(b.spdBound) != 0 || len(b.ikeBypass) != 0 || len(b.spdEntries) != 0 || len(b.protectLocal) != 0 {
		t.Errorf("after the refused install the backend records spd %d, %d bindings, %d bypassed endpoints, %d entries and %d protect entries, want none",
			b.spdID, len(b.spdBound), len(b.ikeBypass), len(b.spdEntries), len(b.protectLocal))
	}
}

// TestVPPIKEBypassRefusedInstallsNoProtect: method: VPP refuses every bypass entry. The
// install must fail without sending the protect entry.
// PREVENTS: a protect entry in force with no IKE bypass beside it, the state in which
// the tunnel can be neither rekeyed nor deleted.
//
// MUTATION: in installProtectEntry, ignore the ensureIKEBypass error: the protect entry
// is sent and InstallPolicy reports success.
func TestVPPIKEBypassRefusedInstallsNoProtect(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	ch.refuseIf = func(msg api.Message) bool {
		req, ok := msg.(*ipsec.IpsecSpdEntryAddDelV2)
		return ok && req.Entry.Policy == ipsec_types.IPSEC_API_SPD_ACTION_BYPASS
	}
	if err := b.InstallPolicy(testSPParams()); err == nil {
		t.Fatal("InstallPolicy reported success while VPP refused the IKE bypass")
	}
	for _, req := range sentSPDEntries(ch) {
		if req.Entry.Policy == ipsec_types.IPSEC_API_SPD_ACTION_PROTECT {
			t.Error("a protect entry was sent after VPP refused the IKE bypass it needs")
		}
	}
	if b.spdID != 0 || len(b.spdEntries) != 0 {
		t.Errorf("the backend records spd %d and %d entries after the refusal, want none", b.spdID, len(b.spdEntries))
	}
}

// TestVPPProtectWithoutLocalEndpointRefused: method: an outbound protect policy with no
// tunnel source. It names no address to exempt, so it is refused before anything is
// sent.
func TestVPPProtectWithoutLocalEndpointRefused(t *testing.T) {
	b, ch := installedPolicyBackend(t)
	p := testSPParams()
	p.TunnelSrc = nil
	if err := b.InstallPolicy(p); err == nil {
		t.Fatal("InstallPolicy accepted a protect policy with no local tunnel endpoint")
	}
	if len(ch.sent) != 0 {
		t.Errorf("sent %d messages, want none", len(ch.sent))
	}
}

// TestVPPNoPolicyRanksWithTheIKEBypass: method: ask for a bypass at the one Ze priority
// vppPriority turns into the IKE bypass rank. It is refused before anything is sent,
// and the lowest priority a config can state stays below that rank.
// PREVENTS: a caller's DISCARD or PROTECT entry tying with the IKE bypass, where VPP's
// order between the two is not Ze's to choose.
//
// MUTATION: in vppSPDSelector, drop the rank guard: the install succeeds.
func TestVPPNoPolicyRanksWithTheIKEBypass(t *testing.T) {
	b, ch := newCapturingBackend()
	p := testSPParams()
	p.Action = SPActionDiscard
	p.SAID = 0
	p.Mode = 0
	p.Priority = -int(vppIKEBypassPriority)
	if err := b.InstallPolicy(p); err == nil {
		t.Fatal("InstallPolicy accepted a policy at the rank of the IKE bypass entries")
	}
	if len(ch.sent) != 0 {
		t.Errorf("sent %d messages, want none", len(ch.sent))
	}
	if got := vppPriority(1); got >= vppIKEBypassPriority {
		t.Errorf("vppPriority(1) = %d, which reaches the IKE bypass rank %d", got, vppIKEBypassPriority)
	}
}
