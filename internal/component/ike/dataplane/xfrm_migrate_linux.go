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

	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
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

// xfrmMigrationAvailable reports whether the backend may advertise mobility. Only
// a present answer does: an undetermined kernel is not offered MOBIKE.
func xfrmMigrationAvailable() bool {
	return xfrmMigrationProbe().State == kernelcap.StatePresent
}

// xfrmMigrationProbe asks the kernel whether it handles XFRM_MSG_MIGRATE_STATE.
// It is both the backend's own question and the ipsec-mobike capability's probe
// (kernelcap_linux.go), so ze doctor and the backend cannot disagree.
func xfrmMigrationProbe() kernelcap.Result {
	// AF_UNSPEC cannot identify an installed SA: verify_newsa_info accepts only
	// AF_INET/AF_INET6. ESRCH therefore proves this handler exists without any
	// possible mutation.
	var body [xfrmMigrateStateLen]byte
	body[19] = 1 // SPI 1, network order; old family remains AF_UNSPEC.
	body[22] = ProtoESP
	nl.NativeEndian().PutUint16(body[128:130], unix.AF_INET)
	req := nl.NewNetlinkRequest(xfrmMsgMigrateState, unix.NLM_F_ACK)
	req.AddRawData(body[:])
	return classifyXFRMMigration(xfrmMigrationExecute(req))
}

// errMigrationAccepted is reported when the kernel accepted a migration naming no
// SA. That answers nothing, so it is no verdict.
var errMigrationAccepted = errors.New("the kernel accepted a state migration naming no SA")

// classifyXFRMMigration reads the probe's errno. ESRCH is the handler running
// its lookup, so it is presence. EINVAL is a kernel before 7.2, which refuses the
// message type above XFRM_MSG_MAX; ENOPROTOOPT is the handler built without
// CONFIG_XFRM_MIGRATE; EPROTONOSUPPORT is no XFRM netlink at all. Those three
// are absence. Every other errno, EPERM included, did not reach the question.
//
// EINVAL is absence and not an unread answer because a 7.2 handler cannot
// return it for this probe (net/xfrm/xfrm_user.c, v7.2). xfrm_user_rcv_msg
// answers `if (type > XFRM_MSG_MAX) return -EINVAL;` before its CAP_NET_ADMIN
// check, so an older kernel answers EINVAL to every caller. In
// xfrm_do_migrate_state every EINVAL before the lookup tests a field the probe
// sets valid: `if (!um->id.spi)` (SPI 1), `if (um->reserved)` (zero),
// `um->flags & ~XFRM_MIGRATE_STATE_KNOWN_FLAGS` (zero), and
// verify_selector_prefixlen, which returns 0 for `case AF_UNSPEC:` (the probe's
// selector family). The probe carries no attribute for the MIGRATE_STATE
// attribute filter to refuse, and its body is the 132 bytes of
// struct xfrm_user_migrate_state, so the size check passes too.
func classifyXFRMMigration(err error) kernelcap.Result {
	if errors.Is(err, unix.ESRCH) {
		return kernelcap.Result{State: kernelcap.StatePresent}
	}
	if errors.Is(err, unix.EINVAL) {
		return kernelcap.Result{State: kernelcap.StateAbsent, Reason: err}
	}
	if errors.Is(err, unix.ENOPROTOOPT) {
		return kernelcap.Result{State: kernelcap.StateAbsent, Reason: err}
	}
	if errors.Is(err, unix.EPROTONOSUPPORT) {
		return kernelcap.Result{State: kernelcap.StateAbsent, Reason: err}
	}
	if err == nil {
		return kernelcap.Result{State: kernelcap.StateUnknown, Reason: errMigrationAccepted}
	}
	return kernelcap.Result{State: kernelcap.StateUnknown, Reason: err}
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
	// The SA the receiver serves is the inbound SPI on the old local address, the pair
	// XFRM keys the state on. Only an IPv4 inbound state is ever watched (InstallSA
	// refuses a dual-form SA over IPv6), so an old local address with no IPv4 form names
	// no watched SA.
	var from netip.Addr
	var oldTarget espFormTarget
	watched := false
	if oldLocal, ok := netip.AddrFromSlice(m.OldLocal.To4()); ok {
		from = oldLocal
		oldTarget, watched = b.espForms.target(m.InboundSPI, oldLocal)
	}
	peer, err := xfrmMigrationIPv4("peer", m.NewRemote)
	if err != nil {
		return err
	}
	local, err := xfrmMigrationIPv4("local", m.NewLocal)
	if err != nil {
		return err
	}
	newTarget := espFormTarget{peer: peer, local: local, peerPort: m.RemotePort, localPort: m.LocalPort}
	// A move the receiver could not follow is refused while nothing has changed: the
	// receiver tells SAs on one inbound SPI apart by local address, so a move onto the
	// address another such SA holds would leave one of them unserved. The registry's
	// own check decides it (espFormRegistry.admitsMove, the check retarget makes).
	if err := b.espForms.admitsMove(m.InboundSPI, from, local); err != nil {
		return xfrmMigrationCollision(err, m, local)
	}
	// RFC 7296 Section 2.23: "all devices MUST be able to receive and process both UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time."
	// prepareMigration templates the inbound state, so after the
	// move the kernel refuses its bare ESP and only the receiver serves that form. An SA
	// the receiver already watches has the paths open. For one it does not, the paths are
	// opened, and held, before the SAD is touched: a raw socket or ze-espform device that
	// cannot open refuses the move while nothing has changed, as InstallSA refuses an SA
	// whose bare form nothing would serve. The hold ends in watchHeld once the move
	// landed, or in release when it did not: held says it has not ended yet, and the
	// deferred release ends it only then, so a panic before either cannot leave the
	// paths held with no SA, and no path ends it twice.
	held := false
	if !watched {
		if err := b.espForms.acquire(); err != nil {
			return xfrmMigrationUnserved(err, m, local)
		}
		held = true
		defer func() {
			if held {
				b.espForms.release()
			}
		}()
	}

	// The receiver follows the SAD, never leads it: until the states move, the kernel
	// still holds the SA on its old address, so the receiver keeps re-presenting bare
	// ESP there. The registry changes only after the move succeeds, so a failed move
	// that rolls back leaves it as it was.
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
			if updated.Dir != netlink.XFRM_DIR_OUT {
				updated.Tmpls[0].Src, updated.Tmpls[0].Dst = m.NewRemote, m.NewLocal
			}
			if err = xfrmMigrationPolicyUpdate(&updated); err != nil {
				break
			}
		}
	}
	if err == nil {
		// followMove ends the hold in watchHeld.
		held = false
		b.followMove(m.InboundSPI, watched, from, newTarget)
		return nil
	}

	// The SA the hold was taken for is not registered: the move did not land, and on a
	// lost rollback the caller retires the Child SA.
	if held {
		held = false
		b.espForms.release()
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
	// The receiver never moved, so a restored tunnel is served where it was.
	if rollback == nil {
		return cause
	}
	// The caller retires this Child SA on ErrTunnelMigrationLost, so its watch goes now.
	// The receiver never moved, so a watched SA is still on its old local address.
	if watched {
		b.espForms.Forget(m.InboundSPI, oldTarget.local)
	}
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

// xfrmMigrationCollision answers MigrateTunnel's refusal of a move the receiver of both
// ESP wire forms cannot follow, from the receiver's answer to admitsMove. Only
// errESPFormLocalTaken is that refusal; any other answer passes through, as a receiver
// that cannot be asked is not a collision.
//
// The fix line lists the peers with their inbound SPIs: the move names no peer, and
// clearing either peer whose Child SA carries the SPI gives that peer's new Child SA a
// new SPI, which ends the collision.
func xfrmMigrationCollision(err error, m TunnelMigration, local netip.Addr) error {
	if !errors.Is(err, errESPFormLocalTaken) {
		return err
	}
	var problem textbuf.Buffer
	problem.Str("the MOBIKE move of the Child SA with inbound SPI ").Uint32(m.InboundSPI).
		Str(" from local address ").Str(m.OldLocal.String()).Str(" to ").Str(local.String()).
		Str(" was refused, because another Child SA with the same inbound SPI is received on ").
		Str(local.String())
	return opererr.Wrap(err, problem.String(),
		"the receiver of both ESP wire forms tells the SAs on one SPI apart by local address, so Ze "+
			"keeps the tunnel on its old address and changes nothing. Clearing either peer whose Child SA "+
			"carries that SPI, with "+ESPFormFixClearPeer+"<name>, gives its new Child SA a new SPI; the "+
			"command below lists each peer with its inbound SPI",
		ESPFormFixListSAs)
}

// xfrmMigrationUnserved answers MigrateTunnel's refusal of a move whose SA the receiver
// of bare ESP could not serve, because its reader socket or its ze-espform device did not
// open (espFormReceiver.acquire). The cause is rendered after the reason, so its text is
// not repeated in the problem, and the fix is the cause's own: every open failure the
// host can cause is an operator error that names its fix (openESPFormPaths), and the
// doctor reports the same failure with the same fix. A cause that is no operator error is
// a Ze defect no operator command fixes, so it passes through with the move's context
// and no fix line.
func xfrmMigrationUnserved(err error, m TunnelMigration, local netip.Addr) error {
	var problem textbuf.Buffer
	problem.Str("the MOBIKE move of the Child SA with inbound SPI ").Uint32(m.InboundSPI).
		Str(" from local address ").Str(m.OldLocal.String()).Str(" to ").Str(local.String()).
		Str(" was refused, because Ze cannot open the receiver of bare ESP, its raw ESP socket and the ").
		Str(espFormTUNName).Str(" device")
	refusal, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		return fmt.Errorf("%s: %w", problem.String(), err)
	}
	return opererr.Wrap(espFormRefusalCause{err: err, refusal: refusal}, problem.String(),
		"after the move the SA's inbound state carries the ESP-in-UDP template, so the kernel refuses its "+
			"bare ESP, and only that receiver serves the bare form. Ze installs no SA that receives one ESP "+
			"form only, so it keeps the tunnel on its old address and changes nothing",
		refusal.Fix())
}

// espFormRefusalCause carries an operator error as the cause of another one that takes
// over its fix. Its text is the refusal's problem and its own cause, never its why or fix
// line: the outer error renders its own fix last, so the fix appears once, as the doctor
// renders a refusal (espFormTUNUnavailable, engine/doctor_esp_form_fast_path.go). Unwrap
// answers err unchanged, so errors.Is and errors.As still reach every error it carries.
type espFormRefusalCause struct {
	err     error
	refusal *opererr.Error
}

func (c espFormRefusalCause) Error() string {
	var text textbuf.Buffer
	text.Str(c.refusal.Problem())
	if cause := c.refusal.Unwrap(); cause != nil {
		text.Str(": ").Err(cause)
	}
	return text.String()
}

func (c espFormRefusalCause) Unwrap() error {
	return c.err
}

// followMove points the receiver at a tunnel the SAD has already moved: a watched SA
// moves within the registry, so its old address is not left behind as a second SA, and
// an SA that was not watched is registered on its new address, on the paths acquire
// opened and held before the move, which cannot fail.
//
// A retarget failure costs serving only. The kernel holds the moved states and receives
// the ESP-in-UDP form, so rolling the SAD back for it would trade a working tunnel for
// the bare form. The failure is logged at Warn with its fix, here alone (retarget logs
// nothing), and the SAD keeps the move: clearing the peer installs a new Child SA, which
// InstallSA watches where it lives.
func (b *xfrmMobikeBackend) followMove(spi uint32, watched bool, from netip.Addr, target espFormTarget) {
	if !watched {
		b.espForms.watchHeld(spi, target)
		return
	}
	err := b.espForms.retarget(spi, from, target)
	if err == nil {
		return
	}
	b.espForms.logger().Warn("xfrm: a MOBIKE move carried a Child SA to a new address, but the receiver of "+
		"bare ESP could not follow it, so the SA still receives the ESP-in-UDP form and not the bare form. "+
		"Ze keeps the move. Clearing the peer that holds the SA, with "+ESPFormFixClearPeer+"<name>, "+
		"installs a new Child SA that is received in both forms; the listing shows each peer's inbound SPI",
		"spi", spi, "local", from, "new-local", target.local, "error", err,
		"fix", ESPFormFixListSAs, "code", diagnostic.CodeDoctorIPsecESPFormFastPath)
}

// xfrmMigrationIPv4 answers a MOBIKE move's new address as the IPv4 address the
// dual-form receiver watches. validateXFRMMigration refuses an address with no IPv4
// form first; this pairs that check where the address is converted, so a caller that
// skipped it is refused rather than handed the zero address.
func xfrmMigrationIPv4(role string, ip net.IP) (netip.Addr, error) {
	addr, ok := netip.AddrFromSlice(ip.To4())
	if ok {
		return addr, nil
	}
	return netip.Addr{}, opererr.New(
		"the MOBIKE move of an IPsec tunnel names the "+role+" address "+ip.String()+", which is not an IPv4 address",
		"the receiver of both ESP wire forms serves IPv4 only, so Ze keeps the tunnel on its old addresses",
		"ze clear vpn ipsec sa all")
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
	// A MOBIKE move to IPv6 endpoints would need the dual-form receiver to serve IPv6,
	// and no IPv6 dual-form SA can exist: Ze has no IPv6 IKE transport yet (the IKE
	// listener opens udp4 only, transport/udp.go), so InstallSA refuses one. The gap is
	// owed after the first release (plan/post-release/features/spec-ike-ipv6-transport.md).
	// The refusal wraps ErrNotSupported, which the engine reads, and carries the same fix
	// as xfrmMigrationIPv4, the check that pairs it where the address is converted.
	ipv4 := m.NewLocal.To4() != nil && m.NewRemote.To4() != nil
	if !ipv4 {
		return opererr.Wrap(ErrNotSupported,
			"the MOBIKE move of an IPsec tunnel to local address "+m.NewLocal.String()+" and peer address "+
				m.NewRemote.String()+" was refused, because both ESP wire forms would have to be received over IPv6",
			"Ze receives both ESP wire forms over IPv4 only until it has an IPv6 IKE transport, a gap owed "+
				"after the first release, so Ze keeps the tunnel on its old addresses",
			"ze clear vpn ipsec sa all")
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
	for i := range m.Policies {
		p := &m.Policies[i]
		if p.Action != SPActionProtect || p.Mode != ModeTunnel || p.Proto != ProtoESP || p.IfID != m.IfID || p.ReqID != m.ReqID {
			return moves, nil, fmt.Errorf("xfrm: migration policy does not belong to the tunnel")
		}
		index := 1
		switch p.Dir {
		case SADirIn:
			index, inbound = 0, true
		case SADirOut:
			outbound = true
		case SADirFwd:
			index = 0
		default:
			panic("BUG: migration policy has an unknown direction")
		}
		if !p.TunnelSrc.Equal(oldSrc[index]) || !p.TunnelDst.Equal(oldDst[index]) {
			return moves, nil, fmt.Errorf("xfrm: migration policy endpoints changed")
		}
		owner, known := b.policies.ownerOf(*p)
		if !known {
			return moves, nil, fmt.Errorf("xfrm: migration policy has no owner record")
		}
		if owner != p.Owner {
			return moves, nil, &PolicyOwnedError{Selector: policyKey(*p), HeldBy: owner, Wanted: p.Owner}
		}
		want, err := xfrmPolicyFromParams(*p)
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
