// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- MOBIKE kernel boundary.
//go:build linux

package dataplane

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/slogutil"
)

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
	r.reg.watch(spi, netip.MustParseAddr("198.51.100.2"), netip.MustParseAddr("198.51.100.1"))
	target := espFormTarget{
		peer: netip.MustParseAddr("192.0.2.2"), local: netip.MustParseAddr("192.0.2.1"),
		peerPort: 62001, localPort: 4500,
	}
	r.reg.retarget(spi, target)
	esp := bareESP(spi, 77, 0xa5)
	got := driveESPFormRun(t, r, [][]byte{esp}).records()
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
