// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- MOBIKE kernel boundary.
//go:build linux

package dataplane

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// migrationFakeKernel replaces the SAD reads and the state moves MigrateTunnel makes
// with fakes that hold the tunnel m describes on its old addresses, and answers how many
// state moves were made. Before each move it runs during, so a test can read what the
// receiver holds while the kernel has not moved yet. fail, when not zero, is the move
// that fails, counted from one. The seams are restored at cleanup; the test MUST NOT run
// in parallel.
func migrationFakeKernel(t *testing.T, m TunnelMigration, fail int, during func()) *int {
	t.Helper()
	get, execute := xfrmMigrationStateGet, xfrmMigrationExecute
	t.Cleanup(func() { xfrmMigrationStateGet, xfrmMigrationExecute = get, execute })
	xfrmMigrationStateGet = func(id *netlink.XfrmState) (xfrmMigrationState, error) {
		var src net.IP
		switch {
		case id.Dst.Equal(m.OldLocal) && uint32(id.Spi) == m.InboundSPI:
			src = m.OldRemote
		case id.Dst.Equal(m.OldRemote) && uint32(id.Spi) == m.OutboundSPI:
			src = m.OldLocal
		default:
			return xfrmMigrationState{}, unix.ESRCH
		}
		var st xfrmMigrationState
		st.info.Family = unix.AF_INET
		st.info.Mode = kernelModeTunnel
		st.info.Reqid = m.ReqID
		st.ifID = m.IfID
		st.info.Id.Daddr.FromIP(id.Dst)
		st.info.Id.Spi = nl.Swap32(uint32(id.Spi))
		st.info.Id.Proto = ProtoESP
		st.info.Saddr.FromIP(src)
		return st, nil
	}
	moves := 0
	xfrmMigrationExecute = func(*nl.NetlinkRequest) error {
		moves++
		during()
		if moves == fail {
			return unix.EIO
		}
		return nil
	}
	return &moves
}

// TestXFRMMigrationReceiverFollowsTheMove proves MigrateTunnel changes the receiver of
// bare ESP only after the SAD move succeeded, and never for a move it refuses.
//
// VALIDATES: while the states move, the receiver still serves the SA on its old local
// address, and once the move succeeded it serves it on the new one, with the new peer
// and ports; a move onto the local address another Child SA on the same inbound SPI
// holds is refused before any state moves, as an operator error that wraps
// errESPFormLocalTaken, names the SPI and both addresses and carries the SA listing as
// its fix, and neither SA's watch changes; a failed move that rolls back leaves the
// receiver on the old address; a rollback that is lost (ErrTunnelMigrationLost) forgets
// the SA on its old address, because the caller retires that Child SA.
// PREVENTS: bare ESP to the old address refused while the kernel still holds the SA
// there, and a collision found after the SAD already moved.
// METHOD: the fake SAD seams (migrationFakeKernel) hold the tunnel; the receiver's
// registry is read from inside each state move.
// MUTATION: in MigrateTunnel, call followMove before the state-move loop rather than
// after it: the read inside the first move finds the SA already on the new address.
// MUTATION: in MigrateTunnel, drop the admitsMove check: the refused case makes two
// state moves and answers nil.
// MUTATION: in MigrateTunnel, drop the Forget of the lost-rollback branch: the lost
// rollback leaves the SA watched on its old address.
func TestXFRMMigrationReceiverFollowsTheMove(t *testing.T) {
	const spi = 0x71910004
	oldPeer, oldLocal := netip.MustParseAddr("198.51.100.2"), netip.MustParseAddr("198.51.100.1")
	newPeer, newLocal := netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("192.0.2.1")
	otherPeer := netip.MustParseAddr("203.0.113.2")
	m := TunnelMigration{
		OldLocal: net.IP(oldLocal.AsSlice()), OldRemote: net.IP(oldPeer.AsSlice()),
		NewLocal: net.IP(newLocal.AsSlice()), NewRemote: net.IP(newPeer.AsSlice()),
		InboundSPI: spi, OutboundSPI: 0x71910005, IfID: 7, ReqID: 739,
		LocalPort: espFormUDPPort, RemotePort: 62001,
	}
	before := espFormTarget{peer: oldPeer, local: oldLocal, peerPort: espFormUDPPort, localPort: espFormUDPPort}
	after := espFormTarget{peer: newPeer, local: newLocal, peerPort: 62001, localPort: espFormUDPPort}
	other := espFormTarget{peer: otherPeer, local: newLocal, peerPort: espFormUDPPort, localPort: espFormUDPPort}
	backend := func() (*xfrmMobikeBackend, *espFormReceiver) {
		r := newESPFormReceiver(slogutil.DiscardLogger())
		r.reg.watch(spi, natTarget(oldPeer, oldLocal))
		return &xfrmMobikeBackend{xfrmBackend: &xfrmBackend{espForms: r}}, r
	}
	// onOld fails the test unless r serves the SA on its old address alone.
	onOld := func(t *testing.T, r *espFormReceiver, when string) {
		t.Helper()
		if got, ok := r.target(spi, oldLocal); !ok || got != before {
			t.Errorf("%s: the SA on the old address is %+v, %v, want %+v", when, got, ok, before)
		}
	}

	t.Run("moved", func(t *testing.T) {
		b, r := backend()
		moves := migrationFakeKernel(t, m, 0, func() {
			onOld(t, r, "during the state move")
			if got, ok := r.target(spi, newLocal); ok {
				t.Errorf("during the state move the receiver already serves %+v on the new address", got)
			}
		})
		if err := b.MigrateTunnel(m); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		if *moves != 2 {
			t.Errorf("made %d state moves, want 2", *moves)
		}
		if got, ok := r.target(spi, newLocal); !ok || got != after {
			t.Errorf("after the move the SA on the new address is %+v, %v, want %+v", got, ok, after)
		}
		if _, ok := r.target(spi, oldLocal); ok {
			t.Error("after the move the SA is still served on its old address")
		}
	})

	t.Run("collision refused", func(t *testing.T) {
		b, r := backend()
		r.reg.watch(spi, natTarget(otherPeer, newLocal))
		moves := migrationFakeKernel(t, m, 0, func() {})
		err := b.MigrateTunnel(m)
		var oe *opererr.Error
		if !errors.As(err, &oe) {
			t.Fatalf("a move onto another SA's address answered %v, want an operator error", err)
		}
		if !errors.Is(err, errESPFormLocalTaken) {
			t.Errorf("the refusal %v does not wrap %v", err, errESPFormLocalTaken)
		}
		if oe.Fix() != ESPFormFixListSAs {
			t.Errorf("the refusal's fix is %q, want %q", oe.Fix(), ESPFormFixListSAs)
		}
		for _, text := range []string{"inbound SPI 1905328132", "from local address 198.51.100.1 to 192.0.2.1", ESPFormFixClearPeer + "<name>"} {
			if !strings.Contains(err.Error(), text) {
				t.Errorf("the refusal %q does not carry %q", err.Error(), text)
			}
		}
		if *moves != 0 {
			t.Errorf("the refused move made %d state moves, want none", *moves)
		}
		onOld(t, r, "after the refusal")
		if got, ok := r.target(spi, newLocal); !ok || got != other {
			t.Errorf("after the refusal the other SA is %+v, %v, want %+v", got, ok, other)
		}
	})

	t.Run("failed move rolled back", func(t *testing.T) {
		b, r := backend()
		moves := migrationFakeKernel(t, m, 1, func() {})
		err := b.MigrateTunnel(m)
		if !errors.Is(err, unix.EIO) || errors.Is(err, ErrTunnelMigrationLost) {
			t.Fatalf("a failed move that rolled back answered %v, want the move's EIO alone", err)
		}
		if *moves != 1 {
			t.Errorf("made %d state moves, want the one that failed", *moves)
		}
		onOld(t, r, "after the rollback")
		if _, ok := r.target(spi, newLocal); ok {
			t.Error("after the rollback the receiver serves the SA on the new address")
		}
	})

	// The second move fails, the first is rolled back, and the state read back then is
	// not the one moved, so the rollback is lost and the caller retires the Child SA.
	t.Run("lost rollback forgets the SA", func(t *testing.T) {
		b, r := backend()
		moves := migrationFakeKernel(t, m, 2, func() {})
		fakeGet := xfrmMigrationStateGet
		// migrationFakeKernel's cleanup restores the real seam over this wrapper.
		xfrmMigrationStateGet = func(id *netlink.XfrmState) (xfrmMigrationState, error) {
			st, err := fakeGet(id)
			if *moves > 0 {
				st.info.Reqid++
			}
			return st, err
		}
		err := b.MigrateTunnel(m)
		if !errors.Is(err, ErrTunnelMigrationLost) {
			t.Fatalf("a rollback whose state reads back changed answered %v, want %v", err, ErrTunnelMigrationLost)
		}
		if got, ok := r.target(spi, oldLocal); ok {
			t.Errorf("after the lost rollback the SA is still watched on its old address as %+v", got)
		}
		if _, ok := r.target(spi, newLocal); ok {
			t.Error("after the lost rollback the receiver serves the SA on the new address")
		}
	})
}

// TestXFRMMigrationUnwatchedSAOpensThePathsBeforeTheMove proves an SA the receiver does
// not watch has the receiver's paths opened before the SAD moves, and is registered on
// its new address only after the move landed. RFC 7296 Section 2.23: the moved inbound
// state is templated, so the receiver is the only server of its bare ESP.
//
// VALIDATES: paths that cannot open refuse the move as an operator error that wraps the
// cause, names the SPI and both addresses, and carries the cause's fix (the TUN open's,
// and the raw socket's privilege fix), with no state moved and the registry unchanged;
// a move that lands has the paths held during the move with no SA registered, and then
// serves the SA on the new address with the new peer and ports and nothing on the old
// one; a failed move that rolls back ends the hold and registers nothing.
// PREVENTS: a templated SA left receiving the ESP-in-UDP form only, because the paths
// failed to open after the SAD had already moved.
// METHOD: the fake SAD seams (migrationFakeKernel) hold the tunnel, and the receiver's
// openPaths seam refuses or answers the open without a raw socket or a device.
// MUTATION: in MigrateTunnel, call acquire after the state-move loop rather than before
// it: the refused move makes two state moves before it answers.
// MUTATION: in followMove, return without watchHeld for an SA that was not watched: after
// the move nothing serves the SA on the new address.
// MUTATION: in MigrateTunnel, replace the release of the failure path with held = false,
// so neither it nor the deferred release ends the hold: the failed move leaves a hold,
// so the paths stay open with no SA watched.
func TestXFRMMigrationUnwatchedSAOpensThePathsBeforeTheMove(t *testing.T) {
	const spi = 0x71910006
	oldPeer, oldLocal := netip.MustParseAddr("198.51.100.2"), netip.MustParseAddr("198.51.100.1")
	newPeer, newLocal := netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("192.0.2.1")
	m := TunnelMigration{
		OldLocal: net.IP(oldLocal.AsSlice()), OldRemote: net.IP(oldPeer.AsSlice()),
		NewLocal: net.IP(newLocal.AsSlice()), NewRemote: net.IP(newPeer.AsSlice()),
		InboundSPI: spi, OutboundSPI: 0x71910007, IfID: 7, ReqID: 739,
		LocalPort: espFormUDPPort, RemotePort: 62001,
	}
	after := espFormTarget{peer: newPeer, local: newLocal, peerPort: 62001, localPort: espFormUDPPort}
	backend := func(t *testing.T, open func() (espFormPaths, error)) (*xfrmMobikeBackend, *espFormReceiver) {
		t.Helper()
		r := newESPFormReceiver(slogutil.DiscardLogger())
		r.openPaths = open
		t.Cleanup(func() {
			if err := r.Close(); err != nil {
				t.Errorf("close the receiver: %v", err)
			}
		})
		return &xfrmMobikeBackend{xfrmBackend: &xfrmBackend{espForms: r}}, r
	}
	// unchanged fails the test unless r watches nothing and holds nothing.
	unchanged := func(t *testing.T, r *espFormReceiver, when string) {
		t.Helper()
		if !r.reg.empty() {
			t.Errorf("%s: the receiver watches an SA", when)
		}
		if r.holds != 0 {
			t.Errorf("%s: the receiver holds its paths %d times, want none", when, r.holds)
		}
	}

	refusals := []struct {
		name  string
		cause error
		fix   string
	}{
		{"operator error cause", opererr.New("Ze cannot open the TUN device", "the device is a directory", "ls -l /dev/net/tun"), "ls -l /dev/net/tun"},
		{"raw socket refused", espFormSocketRefusal(unix.EPERM), privilege.DaemonCapabilitiesFix},
	}
	for _, tc := range refusals {
		t.Run("paths cannot open, "+tc.name, func(t *testing.T) {
			b, r := backend(t, func() (espFormPaths, error) { return espFormPaths{}, tc.cause })
			moves := migrationFakeKernel(t, m, 0, func() {})
			err := b.MigrateTunnel(m)
			var oe *opererr.Error
			if !errors.As(err, &oe) {
				t.Fatalf("a move whose paths cannot open answered %v, want an operator error", err)
			}
			if !errors.Is(err, tc.cause) {
				t.Errorf("the refusal %v does not wrap the cause %v", err, tc.cause)
			}
			if oe.Fix() != tc.fix {
				t.Errorf("the refusal's fix is %q, want %q", oe.Fix(), tc.fix)
			}
			for _, text := range []string{"inbound SPI 1905328134", "from local address 198.51.100.1 to 192.0.2.1", espFormTUNName} {
				if !strings.Contains(oe.Problem(), text) {
					t.Errorf("the refusal's problem %q does not carry %q", oe.Problem(), text)
				}
			}
			if *moves != 0 {
				t.Errorf("the refused move made %d state moves, want none", *moves)
			}
			unchanged(t, r, "after the refusal")
		})
	}

	t.Run("moved", func(t *testing.T) {
		opened := 0
		b, r := backend(t, func() (espFormPaths, error) { opened++; return newIdleESPFormPaths(), nil })
		moves := migrationFakeKernel(t, m, 0, func() {
			if opened != 1 {
				t.Errorf("during the state move the paths were opened %d times, want once, before the move", opened)
			}
			if r.holds != 1 {
				t.Errorf("during the state move the receiver holds its paths %d times, want once", r.holds)
			}
			if !r.reg.empty() {
				t.Error("during the state move the receiver already watches the SA")
			}
		})
		if err := b.MigrateTunnel(m); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		if *moves != 2 {
			t.Errorf("made %d state moves, want 2", *moves)
		}
		if got, ok := r.target(spi, newLocal); !ok || got != after {
			t.Errorf("after the move the SA on the new address is %+v, %v, want %+v", got, ok, after)
		}
		if got, ok := r.target(spi, oldLocal); ok {
			t.Errorf("after the move an SA is served on the old address as %+v", got)
		}
		if r.holds != 0 {
			t.Errorf("after the move the receiver holds its paths %d times, want none", r.holds)
		}
	})

	t.Run("failed move rolled back", func(t *testing.T) {
		b, r := backend(t, func() (espFormPaths, error) { return newIdleESPFormPaths(), nil })
		moves := migrationFakeKernel(t, m, 1, func() {})
		err := b.MigrateTunnel(m)
		if !errors.Is(err, unix.EIO) || errors.Is(err, ErrTunnelMigrationLost) {
			t.Fatalf("a failed move that rolled back answered %v, want the move's EIO alone", err)
		}
		if *moves != 1 {
			t.Errorf("made %d state moves, want the one that failed", *moves)
		}
		unchanged(t, r, "after the rollback")
	})
}

// TestValidateXFRMMigrationRefusesIPv6AsOperatorError proves the refusal of a MOBIKE move
// to an IPv6 address is an operator error that names both new addresses, says why, ends
// with its fix, and still wraps ErrNotSupported for the engine.
//
// MUTATION: in validateXFRMMigration, return the bare ErrNotSupported for the IPv6 move:
// errors.As finds no operator error.
func TestValidateXFRMMigrationRefusesIPv6AsOperatorError(t *testing.T) {
	m := TunnelMigration{
		OldLocal: net.ParseIP("198.51.100.1"), OldRemote: net.ParseIP("198.51.100.2"),
		NewLocal: net.ParseIP("2001:db8::1"), NewRemote: net.ParseIP("2001:db8::2"),
		InboundSPI: 1, OutboundSPI: 2, ReqID: 3, LocalPort: espFormUDPPort, RemotePort: 62001,
	}
	err := validateXFRMMigration(m)
	var oe *opererr.Error
	if !errors.As(err, &oe) {
		t.Fatalf("the IPv6 move answered %v, want an operator error", err)
	}
	if !errors.Is(err, ErrNotSupported) {
		t.Errorf("the refusal %v does not wrap ErrNotSupported", err)
	}
	if oe.Fix() != "ze clear vpn ipsec sa all" {
		t.Errorf("the refusal's fix is %q, want the one xfrmMigrationIPv4 gives", oe.Fix())
	}
	for _, text := range []string{"2001:db8::1", "2001:db8::2", "IPv6 IKE transport"} {
		if !strings.Contains(err.Error(), text) {
			t.Errorf("the refusal %q does not carry %q", err.Error(), text)
		}
	}
}

// The probe must distinguish an absent handler from a supported handler's missing
// SA. Both an unknown message and a bad argument can return EINVAL.
func TestXFRMMigrationCapabilityProbe(t *testing.T) {
	old := xfrmMigrationExecute
	t.Cleanup(func() { xfrmMigrationExecute = old })
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"supported", unix.ESRCH, true},
		{"old-kernel", unix.EINVAL, false},
		{"disabled-kconfig", unix.ENOPROTOOPT, false},
		{"unprivileged", unix.EPERM, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			xfrmMigrationExecute = func(req *nl.NetlinkRequest) error {
				body := req.Serialize()[unix.NLMSG_HDRLEN:]
				if req.Type != xfrmMsgMigrateState || len(body) != xfrmMigrateStateLen {
					t.Fatal("probe did not use the state-migration ABI")
				}
				id := nl.DeserializeXfrmUsersaId(body)
				if id.Family != unix.AF_UNSPEC || id.Spi == 0 {
					t.Fatal("probe can target a live SA, or fails before state lookup")
				}
				return tc.err
			}
			if got := xfrmMigrationAvailable(); got != tc.want {
				t.Fatalf("capability = %v, want %v for %v", got, tc.want, tc.err)
			}
		})
	}
}

// Decode the message Linux consumes. In particular, omitting the encapsulation
// attribute inherits NAT-T rather than disabling it, and replay/key attributes
// would reintroduce stale userspace state into a live migration.
func TestXFRMMigrationRequestPreservesSelectorAndRemovesEncapsulation(t *testing.T) {
	move := xfrmStateMove{src: net.ParseIP("192.0.2.10"), dst: net.ParseIP("192.0.2.20")}
	move.before.info.Family = unix.AF_INET
	move.before.info.Id.Daddr.FromIP(net.ParseIP("198.51.100.20"))
	move.before.info.Saddr.FromIP(net.ParseIP("198.51.100.10"))
	move.before.info.Id.Spi = nl.Swap32(0x12345678)
	move.before.info.Id.Proto = ProtoESP
	move.before.info.Reqid = 739
	move.before.info.Sel.Family = unix.AF_INET6
	move.before.info.Sel.PrefixlenS = 64
	move.before.info.Sel.PrefixlenD = 96
	move.before.info.Sel.Saddr.FromIP(net.ParseIP("2001:db8:1::"))
	move.before.info.Sel.Daddr.FromIP(net.ParseIP("2001:db8:2::"))
	move.before.info.Sel.Sport = nl.Swap16(179)
	move.before.info.Sel.SportMask = 0xffff
	move.before.info.Sel.Proto = 6
	move.before.encap = nl.XfrmEncapTmpl{
		EncapType: 2, EncapSport: nl.Swap16(4500), EncapDport: nl.Swap16(62001),
	}
	for _, reverse := range []bool{false, true} {
		req := xfrmStateMigrationRequest(&move, reverse)
		body := req.Serialize()[unix.NLMSG_HDRLEN:]
		id := nl.DeserializeXfrmUsersaId(body)
		if id.Spi != move.before.info.Id.Spi || id.Proto != ProtoESP {
			t.Fatal("migration changed the ESP SPI or protocol")
		}
		if !bytes.Equal(body[64:120], move.before.info.Sel.Serialize()) {
			t.Fatal("migration changed the inner selector")
		}
		if nl.NativeEndian().Uint32(body[120:124]) != 739 {
			t.Fatal("migration changed the request ID")
		}
		attrs, err := nl.ParseRouteAttr(body[xfrmMigrateStateLen:])
		if err != nil {
			t.Fatal(err)
		}
		if len(attrs) != 1 || attrs[0].Attr.Type != nl.XFRMA_ENCAP {
			t.Fatalf("migration must carry only the explicit encapsulation attribute: %v", attrs)
		}
		encap := nl.DeserializeXfrmEncapTmpl(attrs[0].Value)
		wantSrc, wantDst := move.src, move.dst
		wantOld := net.ParseIP("198.51.100.20")
		if reverse {
			wantSrc, wantDst = net.ParseIP("198.51.100.10"), wantOld
			wantOld = move.dst
			if *encap != move.before.encap {
				t.Fatal("rollback did not restore the previous NAT ports")
			}
		} else if encap.EncapType != 0 {
			t.Fatal("NAT-off failed to send the remove-encapsulation sentinel")
		}
		if !net.IP(body[24:28]).Equal(wantDst) || !net.IP(body[40:44]).Equal(wantSrc) || !net.IP(body[:4]).Equal(wantOld) {
			t.Fatalf("wrong forward/reverse tunnel endpoints (reverse=%v)", reverse)
		}
	}
}

// The production raw receiver must use the migrated local destination and the
// observed peer port while leaving the ESP sequence and encrypted payload intact.
func TestXFRMMigrationRetargetsBothFormReceiver(t *testing.T) {
	r := newESPFormReceiver(slogutil.DiscardLogger())
	const spi = 0x71910001
	oldLocal := netip.MustParseAddr("198.51.100.1")
	r.reg.watch(spi, natTarget(netip.MustParseAddr("198.51.100.2"), oldLocal))
	target := espFormTarget{
		peer: netip.MustParseAddr("192.0.2.2"), local: netip.MustParseAddr("192.0.2.1"),
		peerPort: 62001, localPort: 4500,
	}
	if err := r.retarget(spi, oldLocal, target); err != nil {
		t.Fatalf("the receiver refused to move its own watched SA: %v", err)
	}
	esp := bareESP(spi, 77, 0xa5)
	// Both datagrams come from the retargeted peer: the receiver serves only the SA's
	// current peer, and after the move that is target.peer. The first is addressed to
	// the SA's new local address, which XFRM now keys the state on, and is served; the
	// second to the pre-move one, which no SA holds after the move.
	conn := &fakeESPFormConn{
		from:  &net.IPAddr{IP: net.IP(target.peer.AsSlice())},
		reads: [][]byte{esp, esp},
		dsts:  []netip.Addr{target.local, oldLocal},
	}
	got := driveESPFormRunConn(t, r, conn).records()
	if len(got) != 1 {
		t.Fatalf("received %d re-presented datagrams, want one", len(got))
	}
	packet := got[0].packet
	if got[0].dst != target.local || netip.AddrFrom4([4]byte(packet[12:16])) != target.peer {
		t.Fatal("receiver retained the pre-migration endpoints")
	}
	if binary.BigEndian.Uint16(packet[20:22]) != 62001 || binary.BigEndian.Uint16(packet[22:24]) != 4500 {
		t.Fatal("receiver retained the pre-migration ports")
	}
	if !bytes.Equal(packet[espFormHeaderLen:], esp) {
		t.Fatal("receiver changed the ESP payload or sequence")
	}
}

// TestXFRMMigrationRetargetServedWithNoFastPathProgram proves the complete path follows
// a MOBIKE move while the fast path carries no program, because it detached after a
// refused gate shut or never loaded: the bare ESP of the moved SA is re-presented to its
// new endpoints.
//
// METHOD: the receiver's fast path is set to each status that holds no attached
// program, the SA is moved with retarget, and the reader runs over one datagram from
// the new peer to the new local address.
// MUTATION: in espFormReceiver.retarget, return before the registry update when the
// fast path is not Attached: the moved SA's datagram finds no SA and nothing is
// re-presented.
func TestXFRMMigrationRetargetServedWithNoFastPathProgram(t *testing.T) {
	for _, status := range []ESPFormFastPathStatus{ESPFormFastPathDetached, ESPFormFastPathUnavailable} {
		t.Run(status.String(), func(t *testing.T) {
			r := newESPFormReceiver(slogutil.DiscardLogger())
			r.fast.mu.Lock()
			r.fast.status = status
			r.fast.mu.Unlock()
			const spi = 0x71910002
			oldLocal := netip.MustParseAddr("198.51.100.1")
			r.reg.watch(spi, natTarget(netip.MustParseAddr("198.51.100.2"), oldLocal))
			target := espFormTarget{
				peer: netip.MustParseAddr("192.0.2.2"), local: netip.MustParseAddr("192.0.2.1"),
				peerPort: 62001, localPort: 4500,
			}
			if err := r.retarget(spi, oldLocal, target); err != nil {
				t.Fatalf("the receiver refused to move its watched SA while the fast path is %s: %v", status, err)
			}
			conn := &fakeESPFormConn{
				from:  &net.IPAddr{IP: net.IP(target.peer.AsSlice())},
				reads: [][]byte{bareESP(spi, 9, 0x5a)},
				dsts:  []netip.Addr{target.local},
			}
			got := driveESPFormRunConn(t, r, conn).records()
			if len(got) != 1 {
				t.Fatalf("with the fast path %s, re-presented %d datagrams after the move, want one", status, len(got))
			}
			if got[0].dst != target.local {
				t.Errorf("re-presented to %v, want the moved SA's local address %v", got[0].dst, target.local)
			}
		})
	}
}

// TestXFRMMigrationRetargetRefusesAnotherSAsAddress proves a move onto the local
// address another SA with the same inbound SPI holds is refused and changes neither SA,
// while a move onto a free address carries the SA there and leaves its old address.
//
// The refusal answers errESPFormLocalTaken, so a caller can tell it from an SA that is
// not watched.
//
// MUTATION: in espFormRegistry.moveSlotLocked, the one check of this collision, drop the
// test for another SA on the new local address: the receiver's retarget then answers nil
// for the move onto localB, the moved SA replaces the other SA's target, and the other
// SA is no longer served.
func TestXFRMMigrationRetargetRefusesAnotherSAsAddress(t *testing.T) {
	r := newESPFormReceiver(slogutil.DiscardLogger())
	const spi = 0x71910003
	peerA, localA := netip.MustParseAddr("198.51.100.2"), netip.MustParseAddr("198.51.100.1")
	peerB, localB := netip.MustParseAddr("192.0.2.2"), netip.MustParseAddr("192.0.2.1")
	r.reg.watch(spi, natTarget(peerA, localA))
	r.reg.watch(spi, natTarget(peerB, localB))

	onto := espFormTarget{peer: netip.MustParseAddr("203.0.113.2"), local: localB, peerPort: 62001, localPort: 4500}
	if err := r.retarget(spi, localA, onto); !errors.Is(err, errESPFormLocalTaken) {
		t.Fatalf("a move onto the other SA's local address answered %v, want %v", err, errESPFormLocalTaken)
	}
	if got, ok := r.target(spi, localB); !ok || got.peer != peerB {
		t.Errorf("the other SA is %+v, %v after the refused move, want its own peer %v", got, ok, peerB)
	}
	if got, ok := r.target(spi, localA); !ok || got.peer != peerA {
		t.Errorf("the SA is %+v, %v after the refused move, want it left on %v", got, ok, localA)
	}

	free := netip.MustParseAddr("203.0.113.1")
	moved := espFormTarget{peer: netip.MustParseAddr("203.0.113.2"), local: free, peerPort: 62001, localPort: 4500}
	if err := r.retarget(spi, localA, moved); err != nil {
		t.Fatalf("a move onto a free local address was refused: %v", err)
	}
	if got, ok := r.target(spi, free); !ok || got != moved {
		t.Errorf("the moved SA is %+v, %v, want %+v", got, ok, moved)
	}
	if _, ok := r.target(spi, localA); ok {
		t.Error("the moved SA is still watched on its old local address")
	}
	if _, ok := r.target(spi, localB); !ok {
		t.Error("the move took the other SA's watch with it")
	}
}

// TestXFRMMigrationIPv4RefusesAnAddressWithNoIPv4Form proves the conversion of a MOBIKE
// move's new address refuses one with no IPv4 form, naming it, rather than handing
// the dual-form receiver the zero address, and converts an IPv4 one unchanged.
// MUTATION: discard the ok of netip.AddrFromSlice in xfrmMigrationIPv4 and answer the
// address with no error.
func TestXFRMMigrationIPv4RefusesAnAddressWithNoIPv4Form(t *testing.T) {
	got, err := xfrmMigrationIPv4("peer", net.ParseIP("192.0.2.7"))
	if err != nil || got != netip.MustParseAddr("192.0.2.7") {
		t.Fatalf("an IPv4 address: %v, %v, want 192.0.2.7 and no error", got, err)
	}
	got, err = xfrmMigrationIPv4("local", net.ParseIP("2001:db8::1"))
	if err == nil {
		t.Fatalf("an IPv6 address: %v and no error, want a refusal", got)
	}
	if got.IsValid() {
		t.Errorf("an IPv6 address: answered %v beside the refusal, want no address", got)
	}
	if !bytes.Contains([]byte(err.Error()), []byte("2001:db8::1")) {
		t.Errorf("refusal %q does not name the address", err)
	}
}
