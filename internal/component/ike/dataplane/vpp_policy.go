// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- VPP dataplane backend
// Overview: vpp.go -- the SA half of the same backend

//go:build ze_vpp

package dataplane

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"slices"

	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/ipsec"
	"go.fd.io/govpp/binapi/ipsec_types"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The SECURITY POLICY half of the VPP IPsec backend: the SPD this backend creates,
// the interface it binds it to, and the entries it puts in it. vpp.go holds the
// SECURITY ASSOCIATION half, and the two meet at spdEntry, where a PROTECT policy
// resolves the SAD id InstallSA allocated for the SA it names.

// vppProtectMode refuses a PROTECT policy this backend would install WRONGLY.
//
// ipsec_spd_entry_v2 carries no mode at all. In the VPP model the mode lives on the
// SAD entry, as IPSEC_API_SAD_FLAG_IS_TUNNEL. A transport-mode policy and a
// tunnel-mode policy would therefore reach VPP as the same request. That is the
// silent-wrong-mode failure the netlink library guard was written to stop (kernelXFRMMode,
// dataplane.go). Reading p.Mode here is what keeps the two apart.
//
// It is reachable independently of IKE. internal/plugins/ospf/ipsec_install.go asks
// for ModeTransport today.
//
// A BYPASS policy carries no template, so it needs no mode at all (SPActionBypass,
// dataplane.go) and never reaches this check.
//
// Refusing is the minimum ai/rules/protocol.md allows. Implementing transport mode
// here means the GoVPP SAD tunnel flag plus the matching SPD entry, and it is the better
// answer whenever someone can test it against a real VPP.
func vppProtectMode(mode uint8) error {
	if mode != ModeTunnel {
		return fmt.Errorf(
			"%w: vpp: policy mode %d is not implemented by this backend, which builds tunnel-mode SPD entries only; installing it would program tunnel mode and report success",
			ErrNotSupported, mode)
	}
	return nil
}

// vppPortRange converts a port selector to the inclusive range an SPD entry takes.
//
// PortMatch is either every port or exactly one (AnyPortMatch and ExactPortMatch,
// dataplane.go), and both are ranges. A partial mask is neither, so it is refused
// rather than widened.
func vppPortRange(m PortMatch) (start, stop uint16, err error) {
	switch m.Mask {
	case 0:
		return 0, math.MaxUint16, nil
	case math.MaxUint16:
		return m.Port, m.Port, nil
	}
	return 0, 0, fmt.Errorf(
		"%w: vpp: port mask %#04x names no contiguous port range, and an SPD entry carries a range",
		ErrNotSupported, m.Mask)
}

// vppICMPSelectorError refuses an ICMP type/code or any other selector that
// VPP's matcher would ignore. TCP, UDP and SCTP are its port-aware protocols.
//
// RFC 4301 Section 4.4.1.1: "If the Next Layer Protocol value is ICMP, then there is a
// 16-bit selector for the ICMP message type and code."
//
// VPP's SPD has no such selector. Its policy match reads the port ranges only for TCP,
// UDP and SCTP and returns the policy before comparing them for every other protocol
// (src/vnet/ipsec/ipsec_output.h, ipsec_output_policy_match and
// ipsec6_output_policy_match: "if (PREDICT_FALSE ((pr != IP_PROTOCOL_TCP) && (pr !=
// IP_PROTOCOL_UDP) && (pr != IP_PROTOCOL_SCTP))) return p;"). An entry for one ICMP type would therefore
// match EVERY ICMP message: a discard written for redirects would drop echo too, and a
// bypass written for echo would pass every ICMP message in the clear. It is refused
// rather than installed wider than written, with ErrSAOptionRefused: a working backend
// refusing what the entry asks for, never ErrNotSupported, which the engine reads as a
// platform without IPsec and keeps the Child SA degraded (isXFRMUnsupported).
func vppICMPSelectorError(p SPParams) error {
	if p.SrcPort.IsAny() && p.DstPort.IsAny() {
		return nil
	}
	// The same matcher limitation applies to every non-TCP/UDP/SCTP selector,
	// including UDP-Lite ports and MH Type. Refuse before emitting a widened SPD.
	switch p.UpperProto {
	case 6, 17, 132:
		return nil
	}
	if !IsICMPProto(p.UpperProto) {
		// The entry carries its role and, for a Child SA, its peer, but not the policy
		// name or the traffic-selector number, so those stay placeholders. Setting a
		// side that is already any to any changes nothing, so both sides are named.
		peer := "<peer>"
		if p.Owner != "" {
			peer = p.Owner
		}
		var fix textbuf.Buffer
		// Only the two roles an operator writes ports for name their own command; any
		// other role names both, since the entry does not say which config holds it.
		//exhaustive:ignore
		switch p.Role {
		case PolicyRoleOperator:
			fix.Str("set vpn ipsec policy <name> local port any; set vpn ipsec policy <name> remote port any")
		case PolicyRoleChildSA:
			fix.Str("set vpn ipsec site-to-site peer ").Str(peer).Str(" traffic-selector <n> local port any; ").
				Str("set vpn ipsec site-to-site peer ").Str(peer).Str(" traffic-selector <n> remote port any")
		default:
			fix.Str("set vpn ipsec policy <name> local port any; set vpn ipsec policy <name> remote port any, or ").
				Str("set vpn ipsec site-to-site peer ").Str(peer).Str(" traffic-selector <n> local port any; ").
				Str("set vpn ipsec site-to-site peer ").Str(peer).Str(" traffic-selector <n> remote port any")
		}
		var problem textbuf.Buffer
		problem.Str("vpp: protocol ").Uint8(p.UpperProto).Str(" has a constrained selector that this backend cannot match")
		return opererr.Wrap(ErrSAOptionRefused, problem.String(),
			"VPP's SPD compares ports only for TCP, UDP and SCTP, so installing this selector would match every packet of that protocol; set both ports of the entry to any",
			fix.String())
	}
	return fmt.Errorf(
		"%w: vpp: a policy for protocol %d names ICMP type %d mask %#04x and code %d mask %#04x, and VPP's SPD compares ports only for TCP, UDP and SCTP, so it would match every ICMP message; "+
			"remove icmp-type and icmp-code from the entry, or program it on the Linux XFRM dataplane",
		ErrSAOptionRefused, p.UpperProto, p.SrcPort.Port, p.SrcPort.Mask, p.DstPort.Port, p.DstPort.Mask)
}

// vppICMPErrorReturnError refuses a policy that asks for the RFC 4301 Section 6.2
// return-SA mapping of an outbound ICMP error.
//
// RFC 4301 Section 6.2: "The implementation extracts the header for the packet that
// triggered the error (from the ICMP message payload), reverses the source and
// destination IP address fields, extracts the protocol field, and reverses the port
// fields (if accessible)." VPP's outbound SPD lookup classifies a packet on its own
// header alone (ipsec_output_policy_match, src/vnet/ipsec/ipsec_output.h), and no node
// under src/vnet/ipsec reads the header an ICMP error embeds, so an error that matches
// no entry is never mapped to the SA of the traffic it reports.
func vppICMPErrorReturnError(p SPParams) error {
	if err := vppICMPErrorCheckError(p.ICMPErrorReturn); err != nil {
		return fmt.Errorf("vpp: policy: %w", err)
	}
	return nil
}

// CatchAllSupported refuses, for the reason vppPolicyInterface gives: a VPP SPD takes
// effect only on the interfaces it is bound to, so VPP holds no entry bound to no
// interface, and the engine's catch-all is exactly that entry.
func (b *vppBackend) CatchAllSupported() error {
	return fmt.Errorf("%w: vpp: an SPD entry applies only to the interfaces its SPD is bound to", ErrNoNodeWideSPD)
}

// AttachOverrideGuard declares there is nothing to guard: VPP's SPD lives in VPP, and
// a per-socket policy reaches only the kernel's XFRM, so no application can override
// VPP's SPD by setsockopt. Writing VPP's SPD needs its binary API socket.
func (b *vppBackend) AttachOverrideGuard() error {
	return fmt.Errorf("vpp: %w", ErrNoSocketPolicySPD)
}

// DetachOverrideGuard has nothing to remove.
func (b *vppBackend) DetachOverrideGuard() error { return nil }

// OverrideGuardState answers not applicable, for the reason AttachOverrideGuard gives.
func (b *vppBackend) OverrideGuardState() (OverrideGuardState, error) {
	return OverrideGuardState{Status: OverrideGuardNotApplicable}, nil
}

func (b *vppBackend) InstallPolicy(p SPParams) error {
	swIfIndex, err := vppPolicyInterface(p)
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// The entry is built BEFORE the SPD is created, so a policy this backend refuses
	// leaves no empty SPD bound to an interface behind it. That covers every
	// PRE-SEND refusal, and only those: vppPolicyInterface runs before the lock and
	// spdEntry runs before ensureSPD, so no validation reaches VPP. The add itself
	// failing is the remaining path, and undoSPD below closes it.
	entry, err := b.spdEntry(p)
	if err != nil {
		return err
	}
	if entry.Policy != ipsec_types.IPSEC_API_SPD_ACTION_PROTECT {
		return b.installSPDEntry(swIfIndex, entry)
	}
	// RFC 4301 Section 5.2
	local, err := vppIKEEndpoint(p)
	if err != nil {
		return err
	}
	return b.installProtectEntry(swIfIndex, entry, local)
}

// installSPDEntry creates and binds the SPD when needed, sends one entry that is not a
// PROTECT entry, and records it. b.mu is held.
func (b *vppBackend) installSPDEntry(swIfIndex uint32, entry ipsec_types.IpsecSpdEntryV2) error {
	spdID, made, err := b.ensureSPD(swIfIndex)
	if err != nil {
		return err
	}
	entry.SpdID = spdID

	if err := b.sendSPDEntry(true, entry); err != nil {
		return errors.Join(err, b.undoSPD(swIfIndex, made))
	}
	// VPP matches an SPD entry by its whole content, so the entry that was sent IS
	// the handle Close sends back (removeInstalled, vpp.go).
	b.spdEntries = append(b.spdEntries, entry)
	return nil
}

// installProtectEntry is installSPDEntry for a PROTECT entry: the IKE BYPASS entries of
// its local endpoint are in VPP BEFORE the entry is sent (ensureIKEBypass), so no
// PROTECT entry is ever in force without them, and a failure takes back the ones this
// call added before it undoes the SPD. b.mu is held.
func (b *vppBackend) installProtectEntry(swIfIndex uint32, entry ipsec_types.IpsecSpdEntryV2, local netip.Addr) error {
	spdID, made, err := b.ensureSPD(swIfIndex)
	if err != nil {
		return err
	}
	entry.SpdID = spdID

	// RFC 4301 Section 5.2
	bypassed, err := b.ensureIKEBypass(local)
	if err != nil {
		return errors.Join(err, b.releaseIKEBypass(local, bypassed), b.undoSPD(swIfIndex, made))
	}
	if err := b.sendSPDEntry(true, entry); err != nil {
		return errors.Join(err, b.releaseIKEBypass(local, bypassed), b.undoSPD(swIfIndex, made))
	}
	b.spdEntries = append(b.spdEntries, entry)
	if b.protectLocal == nil {
		b.protectLocal = make(map[ipsec_types.IpsecSpdEntryV2]netip.Addr)
	}
	b.protectLocal[entry] = local
	return nil
}

// The UDP ports of the IKE exchange, the ports an IKE BYPASS entry names on its local
// side.
//
// RFC 7296 Section 2: "IKE normally listens and sends on UDP port 500, though IKE messages
// may also be received on UDP port 4500 with a slightly different format (see Section 2.23)."
//
// They are also the two ports VPP's own input node reads as IKE: ipsec4_input_node and
// ipsec6_input_node hand a UDP datagram to the SPD lookup only when its destination port
// is 500 or 4500 (src/vnet/ipsec/ipsec_input.c). transport.IKEPort and transport.NATTPort
// hold the same numbers, and that package imports this one, so they are restated here.
var vppIKEPorts = [...]uint16{500, 4500}

// vppIKEBypassPriority ranks an IKE BYPASS entry ahead of every entry a caller can ask
// for. VPP keeps a chain sorted by priority DESCENDING (vppPriority), this is the largest
// value its i32 priority holds, and vppSPDSelector refuses a policy whose rank would
// reach it, so no PROTECT, DISCARD or BYPASS entry of a caller ties with it or beats it.
const vppIKEBypassPriority int32 = math.MaxInt32

// vppIKEEndpoint returns this node's end of the tunnel a PROTECT policy protects, which
// is the address Ze's IKE exchange for that tunnel runs on: an outbound policy's tunnel
// source and an inbound policy's tunnel destination (childPolicies reverses the pair
// with the direction, ike/engine/child.go).
//
// An absent endpoint is refused before anything is sent. With no address to exempt, the
// PROTECT entry would be in force with no IKE BYPASS beside it.
func vppIKEEndpoint(p SPParams) (netip.Addr, error) {
	endpoint := p.TunnelSrc
	if p.Dir == SADirIn {
		endpoint = p.TunnelDst
	}
	if !validTunnelEndpoint(endpoint) {
		return netip.Addr{}, fmt.Errorf(
			"vpp: a protect policy needs its local tunnel endpoint, the address the IKE bypass entries exempt, and it is %v (tunnel src=%v dst=%v, direction %d)",
			endpoint, p.TunnelSrc, p.TunnelDst, p.Dir)
	}
	local, ok := netip.AddrFromSlice(endpoint)
	if !ok {
		return netip.Addr{}, fmt.Errorf("vpp: the local tunnel endpoint %v of a protect policy is not an IP address", endpoint)
	}
	return local.Unmap(), nil
}

// vppIKEBypassEntries builds the four IKE BYPASS entries of one local IKE endpoint in
// SPD spdID: outbound and inbound, for each port of vppIKEPorts.
//
// RFC 4301 Section 5.2: "(This implies that IKE traffic MUST have an explicit BYPASS entry in
// the SPD.)"
//
// THE LOCAL SIDE is one address and one port, Ze's own. THE REMOTE SIDE is every address
// and every port of the same family: a peer behind a NAT presents a rewritten source
// port, a MOBIKE move changes its address, and a peer with no Child SA yet has to reach
// the listener for its first exchange.
//
// THE LOCAL ADDRESS is what keeps this from being the node-wide entry removed in
// e700fb7392. That entry named the port alone, so it also matched a protected host's
// forwarded datagram from source port 500 or 4500 and let it out in clear. A forwarded
// datagram carries the host's address, never Ze's tunnel endpoint.
//
// WHAT VPP MATCHES (v26.06, src/vnet/ipsec). Outbound, ipsec_output_policy_match
// (ipsec_output.h) walks one chain in priority order and compares addresses, protocol
// and, for UDP, both port ranges, so the outbound entries match Ze's IKE datagrams and
// nothing else. Inbound, a datagram to UDP port 500 or 4500 goes to
// ipsec_esp_packet_process (ipsec_input.c), which tries the PROTECT entries by SPI, then
// the BYPASS entries, then the DISCARD entries, and DROPS what none matches ("Drop by
// default if no match on PROTECT, BYPASS or DISCARD"). So once an SPD is bound to an
// interface, the peer's IKE reaches Ze only through an inbound BYPASS entry.
//
// WHAT VPP IGNORES INBOUND. ipsec_input_policy_match and ipsec6_input_policy_match
// (ipsec_input.c) compare an inbound BYPASS or DISCARD entry by its address ranges
// alone: the entry's protocol and ports are never read. ESP and AH take the same path,
// PROTECT then BYPASS then DISCARD. So the two inbound entries built here are not "UDP
// 500 and 4500 only": they pass out of the SPD every ESP and AH packet addressed to the
// local endpoint that no PROTECT entry matches, an SPI no SA holds or a live SPI from a
// source its PROTECT entry does not name, and no DISCARD entry ever sees it.
//
// RFC 4301 Section 5.2: "If there is no match, discard the traffic."
//
// VPP does not discard it here. The packet goes on to ip-local, where in SPD mode
// nothing registers IP protocol 50, so it is punted, to the Linux host under linux-cp.
// Its discard therefore rests on the ip-local and punt path, and no test proves it.
// Latent today, because vppPolicyInterface refuses every policy IKE produces. The fix,
// Ze's VPP patch making inbound BYPASS and DISCARD honor protocol and ports, is owned
// by plan/post-release/features/spec-ipsec-vpp-policy-interface.md (AC-10).
func vppIKEBypassEntries(spdID uint32, local netip.Addr) []ipsec_types.IpsecSpdEntryV2 {
	host := ip_types.NewAddress(local.AsSlice())
	first := make(net.IP, local.BitLen()/8)
	last := make(net.IP, len(first))
	for i := range last {
		last[i] = math.MaxUint8
	}
	entries := make([]ipsec_types.IpsecSpdEntryV2, 0, 2*len(vppIKEPorts))
	for _, port := range vppIKEPorts {
		for _, outbound := range [...]bool{true, false} {
			entries = append(entries, ipsec_types.IpsecSpdEntryV2{
				SpdID:              spdID,
				Priority:           vppIKEBypassPriority,
				IsOutbound:         outbound,
				Policy:             ipsec_types.IPSEC_API_SPD_ACTION_BYPASS,
				Protocol:           uint8(ip_types.IP_API_PROTO_UDP),
				LocalAddressStart:  host,
				LocalAddressStop:   host,
				RemoteAddressStart: ip_types.NewAddress(first),
				RemoteAddressStop:  ip_types.NewAddress(last),
				LocalPortStart:     port,
				LocalPortStop:      port,
				RemotePortStart:    0,
				RemotePortStop:     math.MaxUint16,
			})
		}
	}
	return entries
}

// ensureIKEBypass puts the IKE BYPASS entries of one local IKE endpoint in this
// backend's SPD, and returns the ones THIS call added, so a caller whose own request
// then fails takes back exactly those (releaseIKEBypass). b.mu is held, and the SPD
// exists (ensureSPD).
//
// WHY THIS BACKEND OWNS THEM. On Linux Ze's IKE is exempt on its own sockets
// (BypassIKESocket), and a socket policy reaches the kernel's XFRM alone:
// VPP's SPD lives in VPP and never reads it (AttachOverrideGuard). On VPP the exemption
// can only be an SPD entry, and an SPD entry applies to the interfaces its SPD is bound
// to, which only this backend knows. So the engine asks for nothing and names no
// backend: every PROTECT entry this backend installs brings the entries of its own
// local endpoint with it.
//
// WHY PER PROTECT ENTRY. A PROTECT entry is what can capture the exchange: the ordinary
// site-to-site selector 0.0.0.0/0 covers the two addresses IKE itself runs between, and
// then the outbound entry hands Ze's own IKE to ESP, so the tunnel can be neither
// rekeyed nor deleted. It is also the one request that tells this backend which address
// Ze's IKE runs on (vppIKEEndpoint).
//
// NOT COVERED: an SPD bound by a BYPASS or DISCARD entry before any PROTECT entry
// exists. VPP then drops the peer's IKE on that interface (vppIKEBypassEntries), and
// this backend has been told no IKE address to exempt. Nothing reaches that state today,
// because vppPolicyInterface refuses every policy the IKE engine and the operator SPD
// produce. plan/post-release/features/spec-ipsec-vpp-policy-interface.md binds the SPD
// from the WAN interface leaf, and owes the bypass at that bind.
func (b *vppBackend) ensureIKEBypass(local netip.Addr) ([]ipsec_types.IpsecSpdEntryV2, error) {
	var added []ipsec_types.IpsecSpdEntryV2
	for _, entry := range vppIKEBypassEntries(b.spdID, local) {
		if slices.Contains(b.ikeBypass[local], entry) {
			continue
		}
		if err := b.sendSPDEntry(true, entry); err != nil {
			return added, fmt.Errorf("vpp: IKE bypass of %s, without which the protect policy is not installed: %w", local, err)
		}
		if b.ikeBypass == nil {
			b.ikeBypass = make(map[netip.Addr][]ipsec_types.IpsecSpdEntryV2)
		}
		b.ikeBypass[local] = append(b.ikeBypass[local], entry)
		added = append(added, entry)
	}
	return added, nil
}

// releaseIKEBypass sends back the given IKE BYPASS entries of one local endpoint and
// forgets each one VPP removed. An entry whose delete VPP refuses stays recorded, so
// Close sends it again (removeInstalled, vpp.go) and a later PROTECT entry for the same
// endpoint does not add a second copy. entries MUST NOT alias b.ikeBypass[local], which
// this function edits. b.mu is held.
func (b *vppBackend) releaseIKEBypass(local netip.Addr, entries []ipsec_types.IpsecSpdEntryV2) error {
	var errs []error
	for _, entry := range entries {
		if err := b.sendSPDEntry(false, entry); err != nil {
			errs = append(errs, fmt.Errorf("vpp: IKE bypass of %s: %w", local, err))
			continue
		}
		held := b.ikeBypass[local]
		if i := slices.Index(held, entry); i >= 0 {
			b.ikeBypass[local] = slices.Delete(held, i, i+1)
		}
	}
	if len(b.ikeBypass[local]) == 0 {
		delete(b.ikeBypass, local)
	}
	return errors.Join(errs...)
}

// releaseProtectIKEBypass runs after a recorded entry was deleted from VPP and from
// b.spdEntries: when it was the last PROTECT entry naming its local IKE endpoint, the
// IKE BYPASS entries of that endpoint go with it. b.mu is held.
func (b *vppBackend) releaseProtectIKEBypass(removed ipsec_types.IpsecSpdEntryV2) error {
	local, protect := b.protectLocal[removed]
	// refuse-by-default: guards RemovePolicyParams -- only a PROTECT entry is recorded with a local endpoint, so removing a bypass or discard entry releases no IKE bypass
	if !protect {
		return nil
	}
	// An idempotent re-install records the same entry twice, and VPP still holds the
	// other copy.
	if slices.Contains(b.spdEntries, removed) {
		return nil
	}
	delete(b.protectLocal, removed)
	for _, other := range b.protectLocal {
		if other == local {
			return nil
		}
	}
	if err := b.releaseIKEBypass(local, slices.Clone(b.ikeBypass[local])); err != nil {
		return fmt.Errorf("vpp: the policy is removed, and an IKE bypass entry it needed is not; Close sends it again: %w", err)
	}
	return nil
}

// sendSPDEntry adds or deletes one SPD entry. The add and the delete are one function
// because VPP matches an entry by its whole content: a delete that rebuilt any field
// differently would miss, or remove a different policy. b.mu is held.
func (b *vppBackend) sendSPDEntry(add bool, entry ipsec_types.IpsecSpdEntryV2) error {
	verb := "del"
	if add {
		verb = "add"
	}
	reply := &ipsec.IpsecSpdEntryAddDelV2Reply{}
	err := b.ch.SendRequest(&ipsec.IpsecSpdEntryAddDelV2{IsAdd: add, Entry: entry}).ReceiveReply(reply)
	if err == nil && reply.Retval != 0 {
		err = fmt.Errorf("retval %d", reply.Retval)
	}
	if err != nil {
		return fmt.Errorf("vpp: ipsec spd entry %s: %w", verb, err)
	}
	return nil
}

// forgetSPDEntry drops one entry from the record of what this backend installed, so
// Close does not send back a delete VPP has already performed. IpsecSpdEntryV2 is
// comparable and VPP matches on the whole of it, so equality is the same test VPP
// applies. b.mu is held.
func (b *vppBackend) forgetSPDEntry(entry ipsec_types.IpsecSpdEntryV2) {
	for i, held := range b.spdEntries {
		if held == entry {
			b.spdEntries = append(b.spdEntries[:i], b.spdEntries[i+1:]...)
			return
		}
	}
}

// spdCreated records what one ensureSPD call CREATED in VPP.
//
// A caller whose own request then fails undoes exactly that and no more: an SPD that
// already held this backend's other policies keeps them, and an interface another
// call bound stays bound. Creating the SPD implies binding it in the same call,
// because a backend that had no SPD had bound none.
type spdCreated struct {
	spd  bool
	bind bool
}

// undoSPD removes what one ensureSPD call created, after the request it was created
// for failed.
//
// Without it the "no empty SPD" invariant held for validation refusals alone. The SPD
// was already created and already BOUND when the entry add was sent. A refused add
// therefore left it bound with none of this backend's policies in it. What VPP does
// with a packet matching no entry in a bound SPD is VPP's own default disposition,
// which this tree cannot read. Either way the interface carries a database ze asked
// for and did not fill.
func (b *vppBackend) undoSPD(swIfIndex uint32, made spdCreated) error {
	var errs []error
	if made.bind {
		if err := b.unbindSPD(swIfIndex); err != nil {
			errs = append(errs, err)
		}
	}
	if made.spd {
		if err := b.deleteSPD(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// unbindSPD detaches this backend's SPD from one interface and forgets the binding.
// The interface stops enforcing the SPD. The SPD itself survives. b.mu is held.
func (b *vppBackend) unbindSPD(swIfIndex uint32) error {
	req := &ipsec.IpsecInterfaceAddDelSpd{
		IsAdd:     false,
		SwIfIndex: interface_types.InterfaceIndex(swIfIndex),
		SpdID:     b.spdID,
	}
	reply := &ipsec.IpsecInterfaceAddDelSpdReply{}
	err := b.ch.SendRequest(req).ReceiveReply(reply)
	if err == nil && reply.Retval != 0 {
		err = fmt.Errorf("retval %d", reply.Retval)
	}
	if err != nil {
		return fmt.Errorf("vpp: ipsec spd %d unbind from interface %d: %w", b.spdID, swIfIndex, err)
	}
	delete(b.spdBound, swIfIndex)
	return nil
}

// deleteSPD removes the SPD this backend created and forgets its id, so the next
// ensureSPD makes a new one. It is sent AFTER every interface is unbound: an SPD a
// packet can still reach is one VPP is still consulting. b.mu is held.
func (b *vppBackend) deleteSPD() error {
	reply := &ipsec.IpsecSpdAddDelReply{}
	err := b.ch.SendRequest(&ipsec.IpsecSpdAddDel{IsAdd: false, SpdID: b.spdID}).ReceiveReply(reply)
	if err == nil && reply.Retval != 0 {
		err = fmt.Errorf("retval %d", reply.Retval)
	}
	if err != nil {
		return fmt.Errorf("vpp: ipsec spd del id=%d: %w", b.spdID, err)
	}
	b.spdID = 0
	return nil
}

// RemovePolicy is REFUSED, because VPP deletes an SPD entry by matching the whole
// entry rather than a key. ipsec_spd_entry_add_del_v2 with is_add 0 carries the same
// ipsec_spd_entry_v2 the add carried, and priority, action, SA id, protocol and both
// port ranges are part of it. Three arguments cannot name that entry, so a delete
// built from them would either miss or remove a different policy.
//
// Every caller in the tree already uses RemovePolicyParams, which carries the whole
// selector (engine/child.go, engine/spd_policy.go, plugins/ospf/ipsec_install.go).
func (b *vppBackend) RemovePolicy(_, _ *net.IPNet, _ SADir) error {
	return fmt.Errorf(
		"%w: vpp: a policy cannot be removed by address and direction alone, because VPP matches the whole SPD entry; use RemovePolicyParams",
		ErrNotSupported)
}

// RemovePolicyParams removes a policy by its full selector, sending back the entry
// InstallPolicy sent and recorded (recordedEntry).
func (b *vppBackend) RemovePolicyParams(p SPParams) error {
	if _, err := vppPolicyInterface(p); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// The delete never CREATES an SPD. A policy this backend never installed lives in
	// no SPD of its, and creating one to delete from would report success over an
	// empty database.
	if b.spdID == 0 {
		return fmt.Errorf(
			"%w: vpp: this backend has created no SPD, so it installed no policy to remove",
			ErrNotSupported)
	}
	// The entry is the one this backend recorded, never one rebuilt from p: a removal
	// that failed is retried after the Child SA's SA is removed (removeChildSAOutgoing,
	// ike/engine/child.go), and rebuilding then has no SAD id to name, so the retry
	// answered "installed no such SA" and the engine read the policy as gone while VPP
	// still held it.
	entry, err := b.recordedEntry(p)
	if err != nil {
		return err
	}

	if err := b.sendSPDEntry(false, entry); err != nil {
		return err
	}
	b.forgetSPDEntry(entry)
	return b.releaseProtectIKEBypass(entry)
}

// recordedEntry returns the SPD entry this backend recorded (spdEntries) for p, the
// entry VPP matches a delete on. b.mu is held.
//
// A PROTECT entry carries its SA's SAD id, which p does not: p carries the SPI. While
// this backend still holds p's SA, the entry is the one naming that SA's id, so two
// Child SAs that share a selector during a make-before-break rekey are told apart. Once
// the SA is removed (RemoveSA forgets its id), the entry is the one whose SA this
// backend holds no longer. allocSadID never reuses an id, so such an entry belongs to
// no live SA, and an entry naming a live SA is another Child SA's policy, never this
// one. VPP keeps the removed SA while the entry references it, and frees it once the
// entry goes (spdEntries, vpp.go).
//
// None recorded means this backend holds no such policy: it never installed it, or a
// removal already took it out. That is answered with ErrNotSupported, as a removal from
// no SPD is. Two or more candidates would make the delete a guess, so they are refused.
func (b *vppBackend) recordedEntry(p SPParams) (ipsec_types.IpsecSpdEntryV2, error) {
	probe, err := vppSPDSelector(p)
	if err != nil {
		// An entry the selector check refuses (an ICMP type selector, the ICMP error
		// return mapping) had its install refused by the same check before any write,
		// so this backend never held it: answered as a removal from no SPD is, so the
		// engine's ledger (policyGone) does not record it as left behind. The refusal is
		// kept as text only, because a chain that also carried ErrSAOptionRefused would
		// read as not removed (isXFRMUnsupported checks that sentinel first).
		if errors.Is(err, ErrSAOptionRefused) {
			return probe, fmt.Errorf("%w: vpp: this backend never installed the entry, its install was refused: %v", ErrNotSupported, err) //nolint:errorlint // the refusal must not join the chain, see above
		}
		return probe, err
	}
	probe.SpdID = b.spdID
	liveID, live, err := b.liveSADID(&p, probe.Policy)
	if err != nil {
		return probe, err
	}
	var found []ipsec_types.IpsecSpdEntryV2
	for _, held := range b.spdEntries {
		match := held
		match.SaID = 0
		if match != probe {
			continue
		}
		if !b.entryOfSA(&held, liveID, live) {
			continue
		}
		found = append(found, held)
	}
	if len(found) == 0 {
		return probe, fmt.Errorf(
			"%w: vpp: this backend holds no SPD entry for this policy in SPD %d, so it never installed it or has removed it; list the SPD with 'vppctl show ipsec spd %d'",
			ErrNotSupported, b.spdID, b.spdID)
	}
	if len(found) > 1 {
		return probe, fmt.Errorf(
			"vpp: %d SPD entries in SPD %d carry this selector under SAs this backend has removed, and VPP deletes one by its whole entry; find the one to remove with 'vppctl show ipsec spd %d'",
			len(found), b.spdID, b.spdID)
	}
	return found[0], nil
}

// liveSADID returns the SAD id of the SA a PROTECT policy p names, and whether this
// backend still holds that SA. An action other than PROTECT names no SA, so it answers
// not held, and entryOfSA does not read the id then. b.mu is held.
func (b *vppBackend) liveSADID(p *SPParams, action ipsec_types.IpsecSpdAction) (uint32, bool, error) {
	// refuse-by-default: guards entryOfSA -- a bypass or discard entry carries no SA, so no SAD id is asked for
	if action != ipsec_types.IPSEC_API_SPD_ACTION_PROTECT {
		return 0, false, nil
	}
	identity, err := saIdentityOf(p.SAID, p.TunnelDst, p.Proto)
	if err != nil {
		return 0, false, err
	}
	id, ok := b.sadIDs[identity]
	return id, ok, nil
}

// entryOfSA reports whether a recorded entry with p's selector is p's own: any bypass
// or discard entry, a PROTECT entry naming p's live SA, or, once p's SA is removed, a
// PROTECT entry naming an SA this backend no longer holds. b.mu is held.
func (b *vppBackend) entryOfSA(held *ipsec_types.IpsecSpdEntryV2, liveID uint32, live bool) bool {
	if held.Policy != ipsec_types.IPSEC_API_SPD_ACTION_PROTECT {
		return true
	}
	if live {
		return held.SaID == liveID
	}
	for _, id := range b.sadIDs {
		if id == held.SaID {
			return false
		}
	}
	return true
}

// PolicyDeleteCommand implements PolicyDeleteCommander with the VPP CLI command that
// removes the SPD entry this backend recorded for p, run through vppctl.
//
// VPP matches a delete on the whole entry, its SA id included: the `ipsec policy`
// command (ipsec_policy_add_del_command_fn, src/vnet/ipsec/ipsec_cli.c) parses `del`
// with the same spd, priority, direction, action, sa, protocol, address-range and
// port-range fields the add carried. The command is therefore written from the entry
// this backend recorded (spdEntries), never rebuilt from p: when a removal failure is
// reported the Child SA's states are already removed and their SAD ids forgotten
// (RemoveSA), so only the record still names the SA id. The entry is the one
// RemovePolicyParams would delete (recordedEntry), so the command and Ze's own retry
// name the same entry, and a selector no entry or several entries answer is refused.
func (b *vppBackend) PolicyDeleteCommand(p SPParams) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	entry, err := b.recordedEntry(p)
	if err != nil {
		return "", err
	}
	return vppPolicyDelCommand(&entry), nil
}

// vppPolicyDelCommand writes the `vppctl ipsec policy del` command for entry, every
// field VPP matches a delete on. An SA is named only for a PROTECT entry, the one action
// that carries one, and the action words are the ones unformat_ipsec_policy_action
// reads (src/vnet/ipsec/ipsec_format.c).
func vppPolicyDelCommand(entry *ipsec_types.IpsecSpdEntryV2) string {
	var b textbuf.Buffer
	b.Str("vppctl ipsec policy del spd ").Uint32(entry.SpdID)
	b.Str(" priority ").Int(int64(entry.Priority))
	if entry.IsOutbound {
		b.Str(" outbound")
	} else {
		b.Str(" inbound")
	}
	b.Str(" action ").Str(vppActionWord(entry.Policy))
	if entry.Policy == ipsec_types.IPSEC_API_SPD_ACTION_PROTECT {
		b.Str(" sa ").Uint32(entry.SaID)
	}
	b.Str(" protocol ").Uint8(entry.Protocol)
	b.Str(" local-ip-range ").Str(entry.LocalAddressStart.String()).Str(" - ").Str(entry.LocalAddressStop.String())
	b.Str(" remote-ip-range ").Str(entry.RemoteAddressStart.String()).Str(" - ").Str(entry.RemoteAddressStop.String())
	b.Str(" local-port-range ").Uint16(entry.LocalPortStart).Str(" - ").Uint16(entry.LocalPortStop)
	b.Str(" remote-port-range ").Uint16(entry.RemotePortStart).Str(" - ").Uint16(entry.RemotePortStop)
	return string(b.Bytes())
}

// vppActionWord is the VPP CLI word for an SPD action. The set is closed: vppSPDAction
// produces only bypass, discard and protect, so any other value is a Ze defect.
func vppActionWord(a ipsec_types.IpsecSpdAction) string {
	switch a {
	case ipsec_types.IPSEC_API_SPD_ACTION_BYPASS:
		return "bypass"
	case ipsec_types.IPSEC_API_SPD_ACTION_DISCARD:
		return "discard"
	case ipsec_types.IPSEC_API_SPD_ACTION_PROTECT:
		return "protect"
	case ipsec_types.IPSEC_API_SPD_ACTION_RESOLVE:
		panic("BUG: vppSPDAction never produces the resolve action")
	default:
		panic("BUG: SPD entry carries an action vppSPDAction never produces")
	}
}
func (b *vppBackend) ListPolicies() ([]PolicyInfo, error) {
	return nil, fmt.Errorf("%w: the vpp backend cannot enumerate the SPD; use the VPP CLI to read it back", ErrNotSupported)
}

// vppPolicyInterface returns the VPP interface index this policy applies to.
//
// VPP has no node-wide security policy database. A policy lives in an SPD, and an SPD
// takes effect on the interfaces it is bound to (ipsec_interface_add_del_spd), so a
// policy that names no interface cannot be expressed at all.
//
// SPParams.IfIndex is that interface. Its declaration describes the Linux form, an
// XFRM sel.ifindex, because Linux is where the field started. What it names is the
// interface index OF THE FORWARDING PLANE THE BACKEND PROGRAMS, so this backend reads
// it as a VPP sw_if_index. The one producer of a non-zero value passes a KERNEL index
// (buildIPsecPolicies, plugins/ospf/ipsec_install.go), and it can reach this backend:
// defaultDataplaneSource in the same file returns whatever backend is loaded. What
// stops it is vppProtectMode below and vppUnsupportedSA (vpp.go), neither of which
// reads the index. SPParams.IfIndex (dataplane.go) states that chain in full.
//
// Zero is REFUSED. It is the "node-wide" value IKE leaves (SPParams.IfIndex,
// dataplane.go), and VPP has no node-wide SPD to put such a policy in. It is also
// sw_if_index 0, a real VPP interface, so treating it as unset would silently program
// the wrong one.
func vppPolicyInterface(p SPParams) (uint32, error) {
	if p.IfIndex <= 0 || p.IfIndex > math.MaxUint32 {
		return 0, fmt.Errorf(
			"%w: vpp: a policy applies to the interfaces its SPD is bound to, and SPParams.IfIndex is %d; VPP has no node-wide SPD",
			ErrNotSupported, p.IfIndex)
	}
	return uint32(p.IfIndex), nil //nolint:gosec // bounded on the line above
}

// ensureSPD creates this backend's SPD once and binds it to swIfIndex once, then
// returns the id.
//
// VPP CREATES NO SPD OF ITS OWN. Every SPD is created by an API client
// (ipsec_spd_add_del) and takes effect only on the interfaces it is bound to
// (ipsec_interface_add_del_spd). An SPD entry sent with spd_id 0 therefore lands in a
// database nobody created, which is what this backend used to do: it hardcoded 0, and
// nothing in the tree ever sent either of the two messages above.
//
// The id is not hardcoded either. It is the lowest id VPP does not already hold, read
// from VPP, and the creation is CONFIRMED by reading the SPD list back.
//
// A CREATED SPD whose interface bind then fails is kept, not undone. It is bound to
// nothing, so it acts on no traffic, b.spdID records it so Close deletes it, and the
// next call reuses it rather than making a second. b.mu is held.
func (b *vppBackend) ensureSPD(swIfIndex uint32) (uint32, spdCreated, error) {
	var made spdCreated
	if b.spdID == 0 {
		id, err := b.freeSPDID()
		if err != nil {
			return 0, made, err
		}
		reply := &ipsec.IpsecSpdAddDelReply{}
		if err := b.ch.SendRequest(&ipsec.IpsecSpdAddDel{IsAdd: true, SpdID: id}).ReceiveReply(reply); err != nil {
			return 0, made, fmt.Errorf("vpp: ipsec spd add id=%d: %w", id, err)
		}
		if reply.Retval != 0 {
			return 0, made, fmt.Errorf("vpp: ipsec spd add id=%d: retval %d", id, reply.Retval)
		}
		// VPP is asked whether the SPD exists rather than assumed to have made it.
		// A retval of 0 says the request was accepted; the dump says the database is
		// there to put a policy in.
		held, err := b.spdIDs()
		if err != nil {
			return 0, made, err
		}
		if !held[id] {
			return 0, made, fmt.Errorf("vpp: ipsec spd add id=%d reported success and VPP does not hold that SPD", id)
		}
		b.spdID = id
		made.spd = true
	}
	if !b.spdBound[swIfIndex] {
		reply := &ipsec.IpsecInterfaceAddDelSpdReply{}
		req := &ipsec.IpsecInterfaceAddDelSpd{IsAdd: true, SwIfIndex: interface_types.InterfaceIndex(swIfIndex), SpdID: b.spdID}
		if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
			return 0, made, fmt.Errorf("vpp: ipsec spd %d bind to interface %d: %w", b.spdID, swIfIndex, err)
		}
		if reply.Retval != 0 {
			return 0, made, fmt.Errorf("vpp: ipsec spd %d bind to interface %d: retval %d", b.spdID, swIfIndex, reply.Retval)
		}
		if b.spdBound == nil {
			b.spdBound = make(map[uint32]bool)
		}
		b.spdBound[swIfIndex] = true
		made.bind = true
	}
	return b.spdID, made, nil
}

// spdIDs reads the SPD ids VPP holds.
func (b *vppBackend) spdIDs() (map[uint32]bool, error) {
	held := make(map[uint32]bool)
	req := b.ch.SendMultiRequest(&ipsec.IpsecSpdsDump{})
	for {
		details := &ipsec.IpsecSpdsDetails{}
		stop, err := req.ReceiveReply(details)
		if err != nil {
			return nil, fmt.Errorf("vpp: ipsec spds dump: %w", err)
		}
		if stop {
			break
		}
		held[details.SpdID] = true
	}
	return held, nil
}

// freeSPDID returns the lowest SPD id VPP does not hold, starting at 1.
//
// An id VPP already holds belongs to whoever created it, and adding entries to it
// would put this backend's policies in another client's database.
//
// UNVERIFIED, and the check is a TIME-OF-CHECK: the dump proves the id was free when
// VPP answered, never that this backend then OWNS it. Another API client creating the
// same id between the dump and the ipsec_spd_add_del below would leave both writing
// one database. Whether VPP refuses the duplicate add, or accepts it and shares the
// SPD, is decided in VPP's own source, which is not in this tree. That producer is a
// foreign system, so running it is the only evidence (ai/rules/evidence.md, "Claims
// About the State of the Project"), and the AC-7 deployment run
// (internal/le/test/deployment/vppevidence.go, run_ipsec_evidence) has one API client only, so
// it does not settle it. What IS checked is that the SPD exists after the add
// (ensureSPD reads the list back), which catches an add VPP dropped and not an add
// that landed on somebody else's database.
func (b *vppBackend) freeSPDID() (uint32, error) {
	held, err := b.spdIDs()
	if err != nil {
		return 0, err
	}
	for id := uint32(1); id < math.MaxUint32; id++ {
		if !held[id] {
			return id, nil
		}
	}
	return 0, fmt.Errorf("vpp: the SPD id space is exhausted")
}

// spdEntry builds the SPD entry InstallPolicy sends, less its SpdID: the caller sets
// that, so the entry can be validated before an SPD is created for it. A delete does
// not rebuild it: RemovePolicyParams sends back the entry InstallPolicy recorded
// (recordedEntry), byte-identical to the add, which is what VPP needs to match it.
// b.mu is held.
func (b *vppBackend) spdEntry(p SPParams) (ipsec_types.IpsecSpdEntryV2, error) {
	entry, err := vppSPDSelector(p)
	if err != nil {
		return entry, err
	}
	if entry.Policy == ipsec_types.IPSEC_API_SPD_ACTION_PROTECT {
		if err := vppProtectMode(p.Mode); err != nil {
			return entry, err
		}
		// A PROTECT policy hands traffic to ONE SA, named by its SAD id. Zero is a
		// valid SAD id in VPP. A policy sent with SaID 0 therefore protects with
		// whatever SA holds that id, or with none. That is the
		// zero-looks-like-an-answer failure ai/rules/evidence.md names, and it is the
		// VPP form of the empty XFRM template that left the Linux backend forwarding
		// nothing.
		if p.SAID == 0 {
			return entry, fmt.Errorf(
				"%w: vpp: a protect policy must name the SA that protects it, and SPParams.SAID is 0; the policy would resolve to no SA",
				ErrNotSupported)
		}
		// SPParams.SAID carries the SPI, and the SPI is not the VPP SAD id (saIdentity).
		// The SA is looked up by the identity this policy's direction implies: the
		// tunnel destination is the address the protected packets are sent to, which is
		// the SA's own destination for both directions (childPolicies reverses the
		// pair with the direction, ike/engine/child.go).
		identity, err := saIdentityOf(p.SAID, p.TunnelDst, p.Proto)
		if err != nil {
			return entry, err
		}
		got, ok := b.sadIDs[identity]
		if !ok {
			return entry, fmt.Errorf(
				"%w: vpp: a protect policy names SA spi=%d dst=%v proto=%d, and this backend has installed no such SA; the policy would resolve to an id it did not allocate",
				ErrNotSupported, p.SAID, p.TunnelDst, p.Proto)
		}
		entry.SaID = got
	}
	return entry, nil
}

// vppSPDSelector builds every field of the SPD entry for p except its SpdID and its SA
// id (spdEntry), which are this backend's state. It is also what recordedEntry
// matches a recorded entry on, the SA id compared apart.
func vppSPDSelector(p SPParams) (ipsec_types.IpsecSpdEntryV2, error) {
	var entry ipsec_types.IpsecSpdEntryV2
	// DIRECTION FIRST, and refused explicitly, as vppUnsupportedSA refuses it for an
	// SA. ipsec_spd_entry_v2 carries one boolean, is_outbound, so SADirFwd and an unset
	// Dir both reached VPP as INBOUND. plugins/ospf/ipsec_install.go produces SADirFwd
	// (buildIPsecPolicies), and it was refused only because its policy is also
	// transport mode and the mode check happened to run first.
	if p.Dir != SADirIn && p.Dir != SADirOut {
		return entry, fmt.Errorf(
			"%w: vpp: policy direction %d names neither inbound (%d) nor outbound (%d), and an ipsec_spd_entry_v2 carries one direction; installing it would program an inbound policy",
			ErrNotSupported, p.Dir, SADirIn, SADirOut)
	}
	action := vppSPDAction(p.Action)
	priority := vppPriority(p.Priority)
	// No entry a caller asks for ranks with or ahead of this backend's IKE BYPASS
	// entries (vppIKEBypassPriority): VPP's order between two entries of one rank is not
	// Ze's to choose.
	if priority >= vppIKEBypassPriority {
		return entry, fmt.Errorf(
			"vpp: policy priority %d becomes VPP rank %d, which this backend keeps for its IKE bypass entries; a policy priority is 0 or above",
			p.Priority, priority)
	}
	upperProto, err := vppUpperProto(p.UpperProto)
	if err != nil {
		return entry, err
	}
	// RFC 4301 Section 4.4.1.1
	if err := vppICMPSelectorError(p); err != nil {
		return entry, err
	}
	// RFC 4301 Section 6.2
	if err := vppICMPErrorReturnError(p); err != nil {
		return entry, err
	}
	// SPParams names the selector by the PACKET'S fields, source and destination, and
	// the engine writes an inbound policy with Src on the remote side (childPairPolicy,
	// ike/engine/child.go). An ipsec_spd_entry_v2 names it by SIDE, local and remote:
	// VPP's outbound lookup compares a packet's source with the local range, and its
	// inbound lookup compares a packet's DESTINATION with the local range. So an
	// outbound policy's Src is the local side, and an inbound policy's Dst is.
	//
	// MEASURED on VPP v26.06 on 2026-11-20 by the cleartext probe
	// (internal/le/test/deployment/vppevidencecleartext.go). With Src written to the
	// local range for both directions, an inbound DISCARD for remote 100.64.47.0/24
	// left ESP from 100.64.47.10 to 100.64.45.1 unmatched ('discard' 0, 'not matched'
	// 1) and discarded ESP from 100.64.45.10 to 100.64.47.1 ('discard' 1): every
	// inbound entry was installed address-reversed.
	local, remote := p.Src, p.Dst
	localPort, remotePort := p.SrcPort, p.DstPort
	if p.Dir == SADirIn {
		local, remote = p.Dst, p.Src
		localPort, remotePort = p.DstPort, p.SrcPort
	}
	localPortStart, localPortStop, err := vppPortRange(localPort)
	if err != nil {
		return entry, fmt.Errorf("vpp: policy local port: %w", err)
	}
	remotePortStart, remotePortStop, err := vppPortRange(remotePort)
	if err != nil {
		return entry, fmt.Errorf("vpp: policy remote port: %w", err)
	}
	localStart, localStop, err := vppRange(local)
	if err != nil {
		return entry, fmt.Errorf("vpp: policy local selector: %w", err)
	}
	remoteStart, remoteStop, err := vppRange(remote)
	if err != nil {
		return entry, fmt.Errorf("vpp: policy remote selector: %w", err)
	}

	return ipsec_types.IpsecSpdEntryV2{
		// SpdID is set by the caller, from ensureSPD.
		Priority:   priority,
		IsOutbound: p.Dir == SADirOut,
		Policy:     action,
		// The SPD protocol is the UPPER-LAYER protocol of the matched traffic, never
		// the IPsec transform. Sending p.Proto here put ESP (50) in a field that names
		// TCP, UDP or OSPF, so the policy matched ESP-in-ESP and nothing else.
		Protocol:           upperProto,
		LocalAddressStart:  localStart,
		LocalAddressStop:   localStop,
		RemoteAddressStart: remoteStart,
		RemoteAddressStop:  remoteStop,
		// Any port is the WHOLE range. A zero pair stood here, and a zero pair is the
		// range that matches port 0 alone.
		LocalPortStart:  localPortStart,
		LocalPortStop:   localPortStop,
		RemotePortStart: remotePortStart,
		RemotePortStop:  remotePortStop,
	}, nil
}

// vppAnyProtocol is the value an ipsec_spd_entry_v2 takes to match EVERY upper-layer
// protocol. It is not zero.
//
// MEASURED on VPP v26.06 in the AC-7 deployment run. VPP's own CLI, given no protocol,
// creates a policy `show ipsec spd` prints as "protocol any", and asked for protocol
// 255 it prints the same. Asked for protocol 0 it prints
// "protocol IP6_HOP_BY_HOP_OPTIONS", which is IANA protocol number 0.
const vppAnyProtocol = 255

// vppUpperProto maps the Ze upper-layer protocol selector to the VPP one.
//
// ZERO IS NOT "ANY" IN VPP. SPParams.UpperProto is 0 for "match every protocol"
// (dataplane.go), which is what the IKE engine leaves. The zero used to be passed
// through, and the measurement above is what it produced: a policy matching IPv6
// hop-by-hop options and nothing else. Every Child SA policy would have protected no
// traffic while VPP reported the policy installed.
//
// 255 as an INPUT is refused rather than passed through. IANA reserves protocol 255,
// and sending it would be indistinguishable from the "any" above, so a caller asking
// for it would get a policy matching everything.
func vppUpperProto(proto uint8) (uint8, error) {
	switch proto {
	case 0:
		return vppAnyProtocol, nil
	case vppAnyProtocol:
		return 0, fmt.Errorf(
			"%w: vpp: upper-layer protocol %d is IANA reserved and is VPP's every-protocol selector, so a policy naming it would match every protocol",
			ErrNotSupported, proto)
	}
	return proto, nil
}

// vppSPDAction maps the Ze policy action to the VPP SPD action. It never defaults.
// PROTECT was hardcoded here, so a bypass policy reached VPP as a protect policy. It
// then black-holed the traffic it was meant to let through (SPParams.Action,
// dataplane.go).
func vppSPDAction(a SPAction) ipsec_types.IpsecSpdAction {
	switch a {
	case SPActionProtect:
		return ipsec_types.IPSEC_API_SPD_ACTION_PROTECT
	case SPActionBypass:
		return ipsec_types.IPSEC_API_SPD_ACTION_BYPASS
	case SPActionDiscard:
		// RFC 4301 Section 7.4: "All implementations MUST support DISCARDing of
		// fragments using the normal SPD packet classification mechanisms." VPP
		// carries the disposition itself, so the entry drops matching traffic in
		// the forwarding plane rather than being refused here.
		return ipsec_types.IPSEC_API_SPD_ACTION_DISCARD
	default:
		panic("BUG: policy has an unknown VPP action")
	}
}

// vppPriority converts a Ze policy priority to the VPP one, and the SIGN FLIPS.
//
// Ze ranks LOWER first (SPParams.Priority: an operator entry at its default order 1000
// beats a peer at PriorityChildSA 2000). VPP keeps each SPD chain sorted by priority
// DESCENDING and takes the first match. A pass-through would therefore reverse the
// operator's order: the entry ranked last would decide every packet two entries both
// match. Negating preserves the ranking the operator wrote.
//
// The hardcoded 100 that stood here gave every policy one rank.
//
// A priority of 0 or above becomes a rank of 0 or below, and this backend's own IKE
// BYPASS entries sit above every one of them, at vppIKEBypassPriority (ensureIKEBypass).
// The per-socket policy that exempts Ze's IKE on Linux (BypassIKESocket)
// reaches the kernel's XFRM alone and does NOT exempt it from VPP's SPD, so on VPP that
// rank is what keeps a Child SA policy from capturing the exchange that rekeys it.
//
// MEASURED on VPP v26.06 in the AC-7 deployment run
// (internal/le/test/deployment/vppevidencerun.go, runIPsec). Two outbound policies go
// into one chain and `show ipsec spd` prints it in stored order. The bypass at Ze 100
// (VPP -100) is listed AHEAD of the Child SA policy (Ze 2000, VPP -2000), and two
// policies added afterwards at VPP priority 7 and 8 sort ahead of both, highest first.
// The chain is sorted DESCENDING, so the negation preserves the ranking.
func vppPriority(p int) int32 {
	switch {
	case p > math.MaxInt32:
		p = math.MaxInt32
	case p < -math.MaxInt32:
		p = -math.MaxInt32
	}
	return int32(-p) //nolint:gosec // clamped to the int32 range on the two lines above
}

// vppRange converts a CIDR selector to the address RANGE a VPP SPD entry takes.
//
// ipsec_spd_entry_v2 has no prefix field. It carries four addresses, a start and a
// stop for each side. A prefix is therefore the range from its network address to its
// broadcast address. The struct this replaced modeled two prefixes, which is a
// different concept rather than a different field order.
func vppRange(n *net.IPNet) (start, stop ip_types.Address, err error) {
	if n == nil || n.IP == nil {
		return start, stop, fmt.Errorf("%w: vpp: the policy selector carries no prefix", ErrNotSupported)
	}
	ip, mask := n.IP, n.Mask
	if v4 := ip.To4(); v4 != nil {
		ip = v4
		if len(mask) == net.IPv6len {
			mask = mask[net.IPv6len-net.IPv4len:]
		}
	} else {
		ip = ip.To16()
	}
	if ip == nil || len(mask) != len(ip) {
		return start, stop, fmt.Errorf("%w: vpp: prefix %s has a mask of %d octets for an address of %d", ErrNotSupported, n, len(n.Mask), len(ip))
	}
	first := make(net.IP, len(ip))
	last := make(net.IP, len(ip))
	for i := range ip {
		first[i] = ip[i] & mask[i]
		last[i] = first[i] | ^mask[i]
	}
	return ip_types.NewAddress(first), ip_types.NewAddress(last), nil
}
