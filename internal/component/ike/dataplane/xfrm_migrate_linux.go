// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- live Child SA migration.
// RFC: rfc/short/rfc4555.md -- tunnel endpoint changes, Section 3.5.
// Related: espform_linux.go -- inbound ESP continues to accept both wire forms.

//go:build linux

package dataplane

import (
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// Linux 7.2 include/uapi/linux/xfrm.h, struct xfrm_user_migrate_state.
// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/include/uapi/linux/xfrm.h?h=v7.2
// Introduced by a9d155ea9b44d9b979796506bec518222f10b9e6, first present in v7.2.
// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=a9d155ea9b44d9b979796506bec518222f10b9e6
//
// Offsets in the 132-byte native-endian payload (addresses and SPI are network order):
//
//	 0..23   old xfrm_usersa_id (destination, SPI, family, protocol, padding)
//	24..39   new destination       40..55   new source
//	56..63   old mark              64..119  unchanged xfrm_selector
//
// 120..123  unchanged reqid      124..127  flags (zero)
// 128..129  new family           130..131  reserved (zero).
const (
	xfrmMsgMigrateState = 0x29
	xfrmMigrateStateLen = 132
)

// Only this wrapper advertises mobility. An older kernel still gets an ordinary
// XFRM backend, not a capability that would fail after negotiating MOBIKE.
type xfrmMobikeBackend struct{ *xfrmBackend }

var _ TunnelMigrator = (*xfrmMobikeBackend)(nil)

// These seams exercise failures between successful kernel mutations, including a
// failed rollback. Tests MUST restore them and MUST NOT run in parallel.
var (
	xfrmMigrationExecute      = executeXFRMMigration
	xfrmMigrationStateGet     = readXFRMMigrationState
	xfrmMigrationPolicyGet    = netlink.XfrmPolicyGet
	xfrmMigrationPolicyUpdate = netlink.XfrmPolicyUpdate
	xfrmMigrationStateDel     = netlink.XfrmStateDel
)

// xfrmMigrationState keeps only identity, selector and encapsulation. Keys never
// escape readXFRMMigrationState, and replay/lifetime snapshots are never written.
type xfrmMigrationState struct {
	info  nl.XfrmUsersaInfo
	encap nl.XfrmEncapTmpl
	ifID  uint32
}

type xfrmStateMove struct {
	before   xfrmMigrationState
	src, dst net.IP
	encap    nl.XfrmEncapTmpl
}

func xfrmMigrationAvailable() bool {
	// AF_UNSPEC cannot identify an installed SA: verify_newsa_info accepts only
	// AF_INET/AF_INET6. ESRCH therefore proves this handler exists without any
	// possible mutation. EINVAL is NOT evidence: old kernels return it for an
	// unknown netlink message type as well.
	var body [xfrmMigrateStateLen]byte
	body[19] = 1 // SPI 1, network order; old family remains AF_UNSPEC.
	body[22] = ProtoESP
	nl.NativeEndian().PutUint16(body[128:130], unix.AF_INET)
	req := nl.NewNetlinkRequest(xfrmMsgMigrateState, unix.NLM_F_ACK)
	req.AddRawData(body[:])
	err := xfrmMigrationExecute(req)
	return errors.Is(err, unix.ESRCH)
}

func executeXFRMMigration(req *nl.NetlinkRequest) error {
	_, err := req.Execute(unix.NETLINK_XFRM, 0)
	return err
}

// MigrateTunnel MUST be serialized with rekey and teardown by its caller; mu also
// excludes all other mutations made through this backend. External XFRM writers
// must not modify Ze-owned state during the transaction.
//
// Linux 7.2 xfrm_do_migrate_state locks the old state, synchronizes live replay and
// lifetime counters, then retires it before publishing the replacement. This is
// what prevents sequence/IV reuse. Legacy XFRM_MSG_MIGRATE copies unlocked state;
// GETSA + DELSA + NEWSA would replay a stale userspace snapshot. Neither is used.
// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/net/xfrm/xfrm_user.c?h=v7.2
// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/include/net/xfrm.h?h=v7.2
func (b *xfrmMobikeBackend) MigrateTunnel(m TunnelMigration) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := validateXFRMMigration(m); err != nil {
		return err
	}
	moves, policies, err := b.prepareMigration(m)
	if err != nil {
		return err
	}
	if b.espForms == nil {
		return errNoESPFormReceiver
	}
	oldTarget, watched := b.espForms.reg.target(m.InboundSPI)
	peer, _ := netip.AddrFromSlice(m.NewRemote.To4())
	local, _ := netip.AddrFromSlice(m.NewLocal.To4())
	// Acquire the receiver before touching the SAD; a socket failure leaves the
	// existing tunnel intact. The old target is restored on any rollback.
	if err := b.espForms.Watch(m.InboundSPI, peer, local); err != nil {
		return err
	}
	b.espForms.reg.retarget(m.InboundSPI, espFormTarget{
		peer: peer, local: local, peerPort: m.RemotePort, localPort: m.LocalPort,
	})

	moved := 0
	for i := range moves {
		err = xfrmMigrationExecute(xfrmStateMigrationRequest(&moves[i], false))
		if err != nil {
			break
		}
		moved++
	}
	if err == nil {
		for i := range policies {
			updated := *policies[i]
			updated.Tmpls = []netlink.XfrmPolicyTmpl{policies[i].Tmpls[0]}
			updated.Tmpls[0].Src, updated.Tmpls[0].Dst = m.NewLocal, m.NewRemote
			if updated.Dir == netlink.XFRM_DIR_IN {
				updated.Tmpls[0].Src, updated.Tmpls[0].Dst = m.NewRemote, m.NewLocal
			}
			if err = xfrmMigrationPolicyUpdate(&updated); err != nil {
				break
			}
		}
	}
	if err == nil {
		return nil
	}

	cause := fmt.Errorf("xfrm: migrate tunnel: %w", err)
	var rollback error
	// Reverse migration synchronizes the CURRENT kernel state again. Replaying
	// the pre-migration replay/lifetime fields would reuse nonces after traffic.
	for i := moved - 1; i >= 0; i-- {
		rollback = errors.Join(rollback, xfrmMigrationExecute(xfrmStateMigrationRequest(&moves[i], true)))
	}
	for _, p := range policies {
		rollback = errors.Join(rollback, xfrmMigrationPolicyUpdate(p))
	}
	// A kernel failure after retiring an SA, or a lost netlink acknowledgement,
	// must not be reported as an intact old tunnel.
	for i := range moves {
		state, stateErr := xfrmMigrationStateGet(xfrmMigrationIdentity(&moves[i].before))
		if stateErr == nil {
			if !sameXFRMMigrationState(&state, &moves[i].before) {
				stateErr = fmt.Errorf("xfrm: rollback did not restore the state endpoints or encapsulation")
			}
		}
		rollback = errors.Join(rollback, stateErr)
	}
	if watched {
		b.espForms.reg.retarget(m.InboundSPI, oldTarget)
	} else {
		b.espForms.Forget(m.InboundSPI)
	}
	if rollback == nil {
		return cause
	}
	b.espForms.Forget(m.InboundSPI)
	// Policies stay PROTECT and owned. Removing them here would let traffic
	// escape unencrypted. The caller MUST retire this Child SA on ErrTunnelMigrationLost.
	cleanup := b.discardMigrationStates(&moves)
	return errors.Join(ErrTunnelMigrationLost, cause, fmt.Errorf("xfrm: migration rollback: %w", rollback), cleanup)
}

func sameXFRMMigrationState(a, b *xfrmMigrationState) bool {
	return a.info.Id == b.info.Id && a.info.Saddr == b.info.Saddr &&
		a.info.Family == b.info.Family && a.info.Reqid == b.info.Reqid &&
		a.ifID == b.ifID && a.encap == b.encap
}

func validateXFRMMigration(m TunnelMigration) error {
	for _, ip := range []net.IP{m.OldLocal, m.OldRemote, m.NewLocal, m.NewRemote} {
		if !validTunnelEndpoint(ip) {
			return fmt.Errorf("xfrm: migration requires unicast tunnel endpoints")
		}
		if ip.IsMulticast() {
			return fmt.Errorf("xfrm: migration cannot use multicast endpoints")
		}
	}
	// The dual-form receiver currently serves IPv4 only, as InstallSA documents.
	if m.NewLocal.To4() == nil || m.NewRemote.To4() == nil {
		return fmt.Errorf("xfrm: migration cannot receive both ESP forms over IPv6: %w", ErrNotSupported)
	}
	if (m.OldLocal.To4() == nil) != (m.OldRemote.To4() == nil) {
		return fmt.Errorf("xfrm: migration old endpoint families differ")
	}
	if m.InboundSPI == 0 || m.OutboundSPI == 0 || m.ReqID == 0 {
		return fmt.Errorf("xfrm: migration requires both SPIs and a request ID")
	}
	if m.LocalPort != espFormUDPPort || m.RemotePort == 0 || m.RemotePort == 500 {
		return fmt.Errorf("xfrm: migration requires local UDP 4500 and a non-IKE-500 peer port")
	}
	return nil
}

func (b *xfrmMobikeBackend) prepareMigration(m TunnelMigration) ([2]xfrmStateMove, []*netlink.XfrmPolicy, error) {
	moves := [2]xfrmStateMove{{src: m.NewRemote, dst: m.NewLocal}, {src: m.NewLocal, dst: m.NewRemote}}
	oldSrc := [2]net.IP{m.OldRemote, m.OldLocal}
	oldDst := [2]net.IP{m.OldLocal, m.OldRemote}
	spis := [2]uint32{m.InboundSPI, m.OutboundSPI}
	for i := range moves {
		state, err := xfrmMigrationStateGet(&netlink.XfrmState{Dst: oldDst[i], Spi: int(spis[i]), Proto: netlink.XFRM_PROTO_ESP, Ifid: int(m.IfID)})
		if err != nil {
			return moves, nil, fmt.Errorf("xfrm: read migration state: %w", err)
		}
		if state.info.Mode != kernelModeTunnel || state.info.Reqid != m.ReqID || state.ifID != m.IfID || !xfrmMigrationIP(state.info.Saddr, state.info.Family).Equal(oldSrc[i]) {
			return moves, nil, fmt.Errorf("xfrm: migration state identity changed")
		}
		moves[i].before = state
		if !oldDst[i].Equal(moves[i].dst) {
			_, targetErr := xfrmMigrationStateGet(&netlink.XfrmState{
				Dst: moves[i].dst, Spi: int(spis[i]), Proto: netlink.XFRM_PROTO_ESP, Ifid: int(m.IfID),
			})
			if targetErr == nil {
				return moves, nil, fmt.Errorf("xfrm: migration destination SPI is already occupied")
			}
			if !errors.Is(targetErr, unix.ESRCH) && !errors.Is(targetErr, unix.ENOENT) {
				return moves, nil, fmt.Errorf("xfrm: inspect migration destination: %w", targetErr)
			}
		}
		// RFC 7296 Section 2.23: "all devices MUST be able to receive and
		// process both UDP-encapsulated ESP and non-UDP-encapsulated ESP
		// packets at any time." Keep inbound templated; the receiver serves bare.
		if i == 0 || m.NATDetected {
			moves[i].encap.EncapType = uint16(netlink.XFRM_ENCAP_ESPINUDP)
			moves[i].encap.EncapSport = nl.Swap16(m.LocalPort)
			moves[i].encap.EncapDport = nl.Swap16(m.RemotePort)
			if i == 0 {
				moves[i].encap.EncapSport, moves[i].encap.EncapDport = moves[i].encap.EncapDport, moves[i].encap.EncapSport
			}
		}
	}
	// A superseded pair still receives traffic until Delete. Its replacement has
	// already moved the shared policies, so only the exact SPI pair moves here.
	if m.Policies == nil {
		return moves, nil, nil
	}
	policies := make([]*netlink.XfrmPolicy, 0, len(m.Policies))
	var inbound, outbound bool
	for _, p := range m.Policies {
		if p.Action != SPActionProtect || p.Mode != ModeTunnel || p.Proto != ProtoESP || p.IfID != m.IfID || p.ReqID != m.ReqID {
			return moves, nil, fmt.Errorf("xfrm: migration policy does not belong to the tunnel")
		}
		index := 1
		switch p.Dir {
		case SADirIn:
			index, inbound = 0, true
		case SADirOut:
			outbound = true
		default:
			return moves, nil, fmt.Errorf("xfrm: migration requires inbound/outbound policies")
		}
		if !p.TunnelSrc.Equal(oldSrc[index]) || !p.TunnelDst.Equal(oldDst[index]) {
			return moves, nil, fmt.Errorf("xfrm: migration policy endpoints changed")
		}
		owner, known := b.policies.ownerOf(p)
		if !known {
			return moves, nil, fmt.Errorf("xfrm: migration policy has no owner record")
		}
		if owner != p.Owner {
			return moves, nil, &PolicyOwnedError{Selector: policyKey(p), HeldBy: owner, Wanted: p.Owner}
		}
		want, err := xfrmPolicyFromParams(p)
		if err != nil {
			return moves, nil, err
		}
		got, err := xfrmMigrationPolicyGet(want)
		if err != nil {
			return moves, nil, fmt.Errorf("xfrm: read migration policy: %w", err)
		}
		if got.Action != netlink.XFRM_POLICY_ALLOW || len(got.Tmpls) != 1 || got.Ifid != want.Ifid || got.Priority != want.Priority || got.Mark != nil {
			return moves, nil, fmt.Errorf("xfrm: installed migration policy changed")
		}
		tmpl := got.Tmpls[0]
		if tmpl.Mode != netlink.XFRM_MODE_TUNNEL || tmpl.Proto != netlink.XFRM_PROTO_ESP || tmpl.Reqid != int(m.ReqID) || !tmpl.Src.Equal(p.TunnelSrc) || !tmpl.Dst.Equal(p.TunnelDst) {
			return moves, nil, fmt.Errorf("xfrm: installed migration template changed")
		}
		policies = append(policies, got)
	}
	if !inbound || !outbound {
		return moves, nil, fmt.Errorf("xfrm: migration needs policies for both directions")
	}
	return moves, policies, nil
}

func xfrmStateMigrationRequest(move *xfrmStateMove, reverse bool) *nl.NetlinkRequest {
	var body [xfrmMigrateStateLen]byte
	oldDst := move.before.info.Id.Daddr
	oldFamily := move.before.info.Family
	newSrc, newDst := move.src, move.dst
	encap := move.encap
	if reverse {
		oldDst.FromIP(move.dst)
		oldFamily = uint16(nl.GetIPFamily(move.dst))
		newSrc = xfrmMigrationIP(move.before.info.Saddr, move.before.info.Family)
		newDst = xfrmMigrationIP(move.before.info.Id.Daddr, move.before.info.Family)
		encap = move.before.encap
	}
	id := nl.XfrmUsersaId{Daddr: oldDst, Spi: move.before.info.Id.Spi, Family: oldFamily, Proto: ProtoESP}
	copy(body[:24], id.Serialize())
	var addr nl.XfrmAddress
	addr.FromIP(newDst)
	copy(body[24:40], addr[:])
	addr = nl.XfrmAddress{}
	addr.FromIP(newSrc)
	copy(body[40:56], addr[:])
	copy(body[64:120], move.before.info.Sel.Serialize())
	nl.NativeEndian().PutUint32(body[120:124], move.before.info.Reqid)
	nl.NativeEndian().PutUint16(body[128:130], uint16(nl.GetIPFamily(newDst)))
	req := nl.NewNetlinkRequest(xfrmMsgMigrateState, unix.NLM_F_ACK)
	// RawData follows Data in nl.NetlinkRequest; append the attribute as raw bytes
	// too, so the fixed header remains first.
	req.AddRawData(body[:])
	// A zero encap_type explicitly removes encapsulation. Omitting XFRMA_ENCAP
	// would inherit it, so NAT-on -> NAT-off MUST still include this attribute.
	req.AddRawData(nl.NewRtAttr(nl.XFRMA_ENCAP, encap.Serialize()).Serialize())
	return req
}

func xfrmMigrationIdentity(state *xfrmMigrationState) *netlink.XfrmState {
	return &netlink.XfrmState{Dst: xfrmMigrationIP(state.info.Id.Daddr, state.info.Family), Spi: int(nl.Swap32(state.info.Id.Spi)), Proto: netlink.XFRM_PROTO_ESP, Ifid: int(state.ifID)}
}

func xfrmMigrationIP(addr nl.XfrmAddress, family uint16) net.IP {
	if family == unix.AF_INET {
		return net.IP(addr[:net.IPv4len])
	}
	return net.IP(addr[:])
}

func readXFRMMigrationState(id *netlink.XfrmState) (xfrmMigrationState, error) {
	var state xfrmMigrationState
	req := nl.NewNetlinkRequest(nl.XFRM_MSG_GETSA, unix.NLM_F_ACK)
	key := &nl.XfrmUsersaId{Spi: nl.Swap32(uint32(id.Spi)), Family: uint16(nl.GetIPFamily(id.Dst)), Proto: uint8(id.Proto)}
	key.Daddr.FromIP(id.Dst)
	req.AddData(key)
	msgs, err := req.Execute(unix.NETLINK_XFRM, nl.XFRM_MSG_NEWSA)
	// GETSA includes keys. Erase every received message on all return paths.
	defer func() {
		for _, msg := range msgs {
			clear(msg)
		}
	}()
	if err != nil {
		return state, err
	}
	if len(msgs) != 1 || len(msgs[0]) < nl.SizeofXfrmUsersaInfo {
		return state, fmt.Errorf("xfrm: incomplete migration state read")
	}
	state.info = *nl.DeserializeXfrmUsersaInfo(msgs[0])
	attrs, err := nl.ParseRouteAttr(msgs[0][nl.SizeofXfrmUsersaInfo:])
	if err != nil {
		return state, err
	}
	for _, attr := range attrs {
		switch attr.Attr.Type {
		case nl.XFRMA_ENCAP:
			if len(attr.Value) < nl.SizeofXfrmEncapTmpl {
				return state, fmt.Errorf("xfrm: truncated migration encapsulation")
			}
			state.encap = *nl.DeserializeXfrmEncapTmpl(attr.Value)
		case nl.XFRMA_IF_ID:
			if len(attr.Value) != 4 {
				return state, fmt.Errorf("xfrm: truncated migration interface ID")
			}
			state.ifID = nl.NativeEndian().Uint32(attr.Value)
		case nl.XFRMA_MARK:
			return state, fmt.Errorf("xfrm: marked state is not an IKE-owned migration target")
		}
	}
	if state.ifID != uint32(id.Ifid) {
		return state, fmt.Errorf("xfrm: migration state interface ID changed")
	}
	return state, nil
}

func (b *xfrmMobikeBackend) discardMigrationStates(moves *[2]xfrmStateMove) error {
	var result error
	for i := range moves {
		id := xfrmMigrationIdentity(&moves[i].before)
		for _, dst := range []net.IP{id.Dst, moves[i].dst} {
			id.Dst = dst
			got, err := xfrmMigrationStateGet(id)
			if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.ENOENT) {
				continue
			}
			if err != nil {
				b.retainMigrationCleanup(&moves[i], dst)
				result = errors.Join(result, err)
				continue
			}
			if got.info.Reqid != moves[i].before.info.Reqid {
				result = errors.Join(result, fmt.Errorf("xfrm: refused cleanup of a foreign migration state"))
				continue
			}
			if err := xfrmMigrationStateDel(id); err != nil {
				b.retainMigrationCleanup(&moves[i], dst)
				result = errors.Join(result, err)
			}
		}
	}
	return result
}

// retainMigrationCleanup lets the caller's ordinary RemoveSA retry an alternate
// destination after fail-closed cleanup met a netlink error.
func (b *xfrmMobikeBackend) retainMigrationCleanup(move *xfrmStateMove, dst net.IP) {
	old := xfrmMigrationIdentity(&move.before)
	if dst.Equal(old.Dst) {
		return
	}
	if b.migrationCleanup == nil {
		b.migrationCleanup = make(map[SAIdentity]SAIdentity)
	}
	b.migrationCleanup[IdentityOf(uint32(old.Spi), old.Dst, ProtoESP, 0)] =
		IdentityOf(uint32(old.Spi), dst, ProtoESP, move.before.ifID)
}
