// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- VPP dataplane backend
// Overview: vpp.go -- the SA half of the same backend

//go:build ze_vpp

package dataplane

import (
	"errors"
	"fmt"
	"math"
	"net"

	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/ipsec"
	"go.fd.io/govpp/binapi/ipsec_types"
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
// silent-wrong-mode failure the netlink guard was written to stop (kernelXFRMMode,
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
// selector (engine/child.go, engine/bypass.go, plugins/ospf/ipsec_install.go).
func (b *vppBackend) RemovePolicy(_, _ *net.IPNet, _ SADir) error {
	return fmt.Errorf(
		"%w: vpp: a policy cannot be removed by address and direction alone, because VPP matches the whole SPD entry; use RemovePolicyParams",
		ErrNotSupported)
}

// RemovePolicyParams removes a policy by its full selector, sending back the entry
// InstallPolicy sent.
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
	entry, err := b.spdEntry(p)
	if err != nil {
		return err
	}
	entry.SpdID = b.spdID

	if err := b.sendSPDEntry(false, entry); err != nil {
		return err
	}
	b.forgetSPDEntry(entry)
	return nil
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
// (internal/le/deployment/vppevidence.go, run_ipsec_evidence) has one API client only, so
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

// spdEntry builds the SPD entry both InstallPolicy and RemovePolicyParams send, less
// its SpdID: the caller sets that, so the entry can be validated before an SPD is
// created for it. One builder keeps the delete byte-identical to the add, which is
// what VPP needs to match it. b.mu is held.
func (b *vppBackend) spdEntry(p SPParams) (ipsec_types.IpsecSpdEntryV2, error) {
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
	action, err := vppSPDAction(p.Action)
	if err != nil {
		return entry, err
	}
	var sadID uint32
	if action == ipsec_types.IPSEC_API_SPD_ACTION_PROTECT {
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
		// the SA's own destination for both directions (childPolicyParams reverses the
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
		sadID = got
	}
	upperProto, err := vppUpperProto(p.UpperProto)
	if err != nil {
		return entry, err
	}
	localPortStart, localPortStop, err := vppPortRange(p.SrcPort)
	if err != nil {
		return entry, fmt.Errorf("vpp: policy local port: %w", err)
	}
	remotePortStart, remotePortStop, err := vppPortRange(p.DstPort)
	if err != nil {
		return entry, fmt.Errorf("vpp: policy remote port: %w", err)
	}
	localStart, localStop, err := vppRange(p.Src)
	if err != nil {
		return entry, fmt.Errorf("vpp: policy local selector: %w", err)
	}
	remoteStart, remoteStop, err := vppRange(p.Dst)
	if err != nil {
		return entry, fmt.Errorf("vpp: policy remote selector: %w", err)
	}

	return ipsec_types.IpsecSpdEntryV2{
		// SpdID is set by the caller, from ensureSPD.
		Priority:   vppPriority(p.Priority),
		IsOutbound: p.Dir == SADirOut,
		SaID:       sadID,
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
func vppSPDAction(a SPAction) (ipsec_types.IpsecSpdAction, error) {
	switch a {
	case SPActionProtect:
		return ipsec_types.IPSEC_API_SPD_ACTION_PROTECT, nil
	case SPActionBypass:
		return ipsec_types.IPSEC_API_SPD_ACTION_BYPASS, nil
	}
	return 0, fmt.Errorf("%w: vpp: policy action %d is not an SPD disposition this backend can express", ErrNotSupported, a)
}

// vppPriority converts a Ze policy priority to the VPP one, and the SIGN FLIPS.
//
// Ze ranks LOWER first (SPParams.Priority, dataplane.go: PriorityIKEBypass 100 beats
// PriorityChildSA 2000). VPP keeps each SPD chain sorted by priority DESCENDING and
// takes the first match. A pass-through would therefore rank every Child SA policy
// ahead of the IKE bypass. A Child SA selector that captured the IKE endpoints would
// then prevent its own rekey. Negating preserves the ranking the constants express.
//
// The hardcoded 100 that stood here gave every policy the IKE bypass rank.
//
// MEASURED on VPP v26.06 in the AC-7 deployment run
// (internal/le/deployment/vppevidence.go, run_ipsec_evidence). Two outbound policies go
// into one chain and `show ipsec spd` prints it in stored order. The IKE bypass
// (Ze 100, VPP -100) is listed AHEAD of the Child SA policy (Ze 2000, VPP -2000), and
// two policies added afterwards at VPP priority 7 and 8 sort ahead of both, highest
// first. The chain is sorted DESCENDING, so the negation preserves the ranking.
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
