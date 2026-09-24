// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- atomic kernel migration.
// These tests require CONFIG_XFRM_MIGRATE with XFRM_MSG_MIGRATE_STATE (Linux 7.2),
// CAP_NET_ADMIN and CAP_NET_RAW. The integration package runs in the QEMU suite.
//go:build integration && linux

package dataplane

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

type migrationKernelSnapshot struct {
	state  xfrmMigrationState
	replay []byte
}

func migrationKernelFixture(t *testing.T, window uint8, esn bool) (*xfrmMobikeBackend, TunnelMigration) {
	t.Helper()
	encapNetns(t)
	encapNetnsUsable(t)
	dp, err := newXFRMBackend()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dp.Close(); err != nil {
			t.Error(err)
		}
	})
	b, ok := dp.(*xfrmMobikeBackend)
	if !ok {
		t.Skip("kernel lacks atomic XFRM_MSG_MIGRATE_STATE or CONFIG_XFRM_MIGRATE")
	}
	m := TunnelMigration{
		OldLocal: net.ParseIP("198.51.100.1"), OldRemote: net.ParseIP("198.51.100.2"),
		NewLocal: net.ParseIP("192.0.2.1"), NewRemote: net.ParseIP("192.0.2.2"),
		InboundSPI: 0x71910001, OutboundSPI: 0x71910002, IfID: 41, ReqID: 739,
		LocalPort: 4500, RemotePort: 62001, NATDetected: false,
	}
	for i, spi := range []uint32{m.InboundSPI, m.OutboundSPI} {
		src, dst := m.OldRemote, m.OldLocal
		if i == 1 {
			src, dst = dst, src
		}
		p := SAParams{
			SPI: spi, Src: src, Dst: dst, Proto: ProtoESP, Mode: ModeTunnel,
			IfID: m.IfID, ReqID: m.ReqID, ReplayWin: window,
			EncAlgo: "aes256gcm", EncKey: encapTestAEADKey, IsAEAD: true,
			UDPEncap: true, UDPEncapSPort: 4500, UDPEncapDPort: 4500,
		}
		state, err := xfrmStateFromParams(p)
		if err != nil {
			t.Fatal(err)
		}
		state.ESN = esn
		state.Limits.TimeHard = 3600
		state.Limits.ByteHard = 1 << 20
		if err := netlink.XfrmStateAdd(state); err != nil {
			skipUnprivileged(t, err)
			t.Fatal(err)
		}
		migrationSeedKernelState(t, state, window, esn)
	}
	_, local, err := net.ParseCIDR("10.71.1.0/24")
	if err != nil {
		t.Fatal(err)
	}
	_, remote, err := net.ParseCIDR("10.71.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	m.Policies = []SPParams{
		{Src: remote, Dst: local, Dir: SADirIn, Proto: ProtoESP, Mode: ModeTunnel, IfID: m.IfID, ReqID: m.ReqID,
			Owner: "migration-peer", Priority: PriorityChildSA, TunnelSrc: m.OldRemote, TunnelDst: m.OldLocal,
			UpperProto: 6, SrcPort: ExactPortMatch(179), DstPort: ExactPortMatch(50100)},
		{Src: local, Dst: remote, Dir: SADirOut, Proto: ProtoESP, Mode: ModeTunnel, IfID: m.IfID, ReqID: m.ReqID,
			Owner: "migration-peer", Priority: PriorityChildSA, TunnelSrc: m.OldLocal, TunnelDst: m.OldRemote,
			UpperProto: 6, SrcPort: ExactPortMatch(50100), DstPort: ExactPortMatch(179)},
	}
	for _, p := range m.Policies {
		if err := b.InstallPolicy(p); err != nil {
			t.Fatal(err)
		}
	}
	return b, m
}

// Seed nonzero legacy/bitmap/ESN replay and lifetime data through the kernel AE
// interface. A migration that reinstalls keys would lose these exact live values.
func migrationSeedKernelState(t *testing.T, state *netlink.XfrmState, window uint8, esn bool) {
	t.Helper()
	old, err := readXFRMMigrationState(state)
	if err != nil {
		t.Fatal(err)
	}
	var header [48]byte // struct xfrm_aevent_id, include/uapi/linux/xfrm.h.
	id := nl.XfrmUsersaId{Spi: nl.Swap32(uint32(state.Spi)), Family: unix.AF_INET, Proto: ProtoESP}
	id.Daddr.FromIP(state.Dst)
	copy(header[:24], id.Serialize())
	var src nl.XfrmAddress
	src.FromIP(state.Src)
	copy(header[24:40], src[:])
	nl.NativeEndian().PutUint32(header[44:48], uint32(state.Reqid))
	req := nl.NewNetlinkRequest(nl.XFRM_MSG_NEWAE, unix.NLM_F_REPLACE|unix.NLM_F_ACK)
	req.AddRawData(header[:])
	if window <= 32 {
		replay := nl.XfrmReplayState{OSeq: 100, Seq: 80, BitMap: 0x80000005}
		req.AddRawData(nl.NewRtAttr(nl.XFRMA_REPLAY_VAL, replay.Serialize()).Serialize())
	} else {
		replay := nl.XfrmReplayStateEsn{BmpLen: 2, OSeq: 100, Seq: 80, ReplayWindow: uint32(window)}
		if esn {
			replay.OSeqHi, replay.SeqHi = 3, 2
		}
		// The vendored Serialize deliberately omits Bmp for NEWSA. AE replacement
		// requires the existing bitmap in the payload as well.
		payload := make([]byte, nl.SizeofXfrmReplayStateEsn+8)
		copy(payload, replay.Serialize())
		nl.NativeEndian().PutUint32(payload[nl.SizeofXfrmReplayStateEsn:], 0x80000005)
		nl.NativeEndian().PutUint32(payload[nl.SizeofXfrmReplayStateEsn+4:], 0x100)
		req.AddRawData(nl.NewRtAttr(nl.XFRMA_REPLAY_ESN_VAL, payload).Serialize())
	}
	lifetime := old.info.Curlft
	lifetime.Bytes, lifetime.Packets = 9000, 90
	lifetime.AddTime -= 60
	lifetime.UseTime = lifetime.AddTime + 1
	req.AddRawData(nl.NewRtAttr(nl.XFRMA_LTIME_VAL, lifetime.Serialize()).Serialize())
	if _, err := req.Execute(unix.NETLINK_XFRM, 0); err != nil {
		t.Fatalf("seed live replay/lifetime: %v", err)
	}
}

func migrationSnapshot(t *testing.T, spi uint32, dst net.IP, ifID uint32) migrationKernelSnapshot {
	t.Helper()
	var result migrationKernelSnapshot
	state, err := readXFRMMigrationState(&netlink.XfrmState{Spi: int(spi), Dst: dst, Proto: netlink.XFRM_PROTO_ESP, Ifid: int(ifID)})
	if err != nil {
		t.Fatal(err)
	}
	result.state = state
	req := nl.NewNetlinkRequest(nl.XFRM_MSG_GETSA, unix.NLM_F_ACK)
	id := nl.XfrmUsersaId{Spi: nl.Swap32(spi), Family: uint16(nl.GetIPFamily(dst)), Proto: ProtoESP}
	id.Daddr.FromIP(dst)
	req.AddData(&id)
	msgs, err := req.Execute(unix.NETLINK_XFRM, nl.XFRM_MSG_NEWSA)
	defer func() {
		for _, msg := range msgs {
			clear(msg)
		}
	}()
	if err != nil || len(msgs) != 1 {
		t.Fatalf("read kernel replay: %v (%d messages)", err, len(msgs))
	}
	attrs, err := nl.ParseRouteAttr(msgs[0][nl.SizeofXfrmUsersaInfo:])
	if err != nil {
		t.Fatal(err)
	}
	for _, attr := range attrs {
		if attr.Attr.Type == nl.XFRMA_REPLAY_VAL || attr.Attr.Type == nl.XFRMA_REPLAY_ESN_VAL {
			result.replay = bytes.Clone(attr.Value)
		}
	}
	if len(result.replay) == 0 {
		t.Fatal("kernel returned no replay state")
	}
	return result
}

func assertMigrationSnapshot(t *testing.T, before, after migrationKernelSnapshot) {
	t.Helper()
	if !bytes.Equal(before.replay, after.replay) {
		t.Fatalf("live replay/sequence changed: %x -> %x", before.replay, after.replay)
	}
	if before.state.info.Curlft != after.state.info.Curlft || before.state.info.Lft != after.state.info.Lft {
		t.Fatal("migration reset traffic counters or key lifetime")
	}
	if before.state.info.Sel != after.state.info.Sel || before.state.info.Reqid != after.state.info.Reqid || before.state.ifID != after.state.ifID {
		t.Fatal("migration changed selectors, request ID or XFRM interface")
	}
}

// Exercise the live kernel mechanism with all replay layouts. The outgoing NAT
// template is removed, the incoming one keeps both ESP receive forms available.
func TestXFRMMigrationPreservesLiveKernelState(t *testing.T) {
	for _, tc := range []struct {
		name   string
		window uint8
		esn    bool
	}{{"legacy", 32, false}, {"bitmap", 64, false}, {"esn", 64, true}} {
		t.Run(tc.name, func(t *testing.T) {
			b, m := migrationKernelFixture(t, tc.window, tc.esn)
			beforeIn := migrationSnapshot(t, m.InboundSPI, m.OldLocal, m.IfID)
			beforeOut := migrationSnapshot(t, m.OutboundSPI, m.OldRemote, m.IfID)
			if err := b.MigrateTunnel(m); err != nil {
				t.Fatal(err)
			}
			afterIn := migrationSnapshot(t, m.InboundSPI, m.NewLocal, m.IfID)
			afterOut := migrationSnapshot(t, m.OutboundSPI, m.NewRemote, m.IfID)
			assertMigrationSnapshot(t, beforeIn, afterIn)
			assertMigrationSnapshot(t, beforeOut, afterOut)
			if !xfrmMigrationIP(afterIn.state.info.Saddr, afterIn.state.info.Family).Equal(m.NewRemote) || !xfrmMigrationIP(afterOut.state.info.Saddr, afterOut.state.info.Family).Equal(m.NewLocal) {
				t.Fatal("kernel retained the old source endpoints")
			}
			if afterOut.state.encap.EncapType != 0 || afterIn.state.encap.EncapType != 2 || nl.Swap16(afterIn.state.encap.EncapSport) != m.RemotePort {
				t.Fatal("kernel did not remove outgoing NAT-T or update incoming NAT-T")
			}
			for _, p := range m.Policies {
				want, err := xfrmPolicyFromParams(p)
				if err != nil {
					t.Fatal(err)
				}
				got, err := netlink.XfrmPolicyGet(want)
				if err != nil {
					t.Fatal(err)
				}
				src, dst := m.NewLocal, m.NewRemote
				if p.Dir == SADirIn {
					src, dst = dst, src
				}
				if len(got.Tmpls) != 1 || !got.Tmpls[0].Src.Equal(src) || !got.Tmpls[0].Dst.Equal(dst) || got.SrcPort != want.SrcPort || got.DstPort != want.DstPort {
					t.Fatal("policy endpoint update lost the installed selectors")
				}
				if owner, known := b.policies.ownerOf(p); !known || owner != p.Owner {
					t.Fatal("migration released policy ownership")
				}
			}
			for _, id := range []*netlink.XfrmState{
				{Spi: int(m.InboundSPI), Dst: m.OldLocal, Proto: netlink.XFRM_PROTO_ESP, Ifid: int(m.IfID)},
				{Spi: int(m.OutboundSPI), Dst: m.OldRemote, Proto: netlink.XFRM_PROTO_ESP, Ifid: int(m.IfID)},
			} {
				if _, err := readXFRMMigrationState(id); !errors.Is(err, unix.ESRCH) && !errors.Is(err, unix.ENOENT) {
					t.Fatalf("old destination still holds the SA: %v", err)
				}
			}
			// Port-only NAT reappearance uses the same atomic mechanism and retains
			// the retired pair's policies when the caller supplies nil.
			m.OldLocal, m.OldRemote = m.NewLocal, m.NewRemote
			m.Policies, m.NATDetected, m.RemotePort = nil, true, 62002
			if err := b.MigrateTunnel(m); err != nil {
				t.Fatal(err)
			}
			last := migrationSnapshot(t, m.OutboundSPI, m.NewRemote, m.IfID)
			assertMigrationSnapshot(t, afterOut, last)
			if last.state.encap.EncapType != 2 || nl.Swap16(last.state.encap.EncapDport) != 62002 {
				t.Fatal("NAT appearance did not update the outgoing encapsulation")
			}
		})
	}
}

// Fail after both kernel SAs and one policy moved. The reverse migration must
// preserve the counters while restoring the old endpoints and policy ownership.
func TestXFRMMigrationRollsBackPolicyFailure(t *testing.T) {
	b, m := migrationKernelFixture(t, 64, false)
	beforeIn := migrationSnapshot(t, m.InboundSPI, m.OldLocal, m.IfID)
	beforeOut := migrationSnapshot(t, m.OutboundSPI, m.OldRemote, m.IfID)
	original := xfrmMigrationPolicyUpdate
	t.Cleanup(func() { xfrmMigrationPolicyUpdate = original })
	calls := 0
	xfrmMigrationPolicyUpdate = func(p *netlink.XfrmPolicy) error {
		calls++
		if calls == 2 {
			return unix.EIO
		}
		return original(p)
	}
	if err := b.MigrateTunnel(m); !errors.Is(err, unix.EIO) || errors.Is(err, ErrTunnelMigrationLost) {
		t.Fatalf("rollback result = %v", err)
	}
	assertMigrationSnapshot(t, beforeIn, migrationSnapshot(t, m.InboundSPI, m.OldLocal, m.IfID))
	assertMigrationSnapshot(t, beforeOut, migrationSnapshot(t, m.OutboundSPI, m.OldRemote, m.IfID))
	for _, p := range m.Policies {
		want, err := xfrmPolicyFromParams(p)
		if err != nil {
			t.Fatal(err)
		}
		got, err := netlink.XfrmPolicyGet(want)
		if err != nil || len(got.Tmpls) != 1 || !got.Tmpls[0].Dst.Equal(p.TunnelDst) {
			t.Fatalf("policy rollback failed: %v", err)
		}
		if owner, known := b.policies.ownerOf(p); !known || owner != p.Owner {
			t.Fatal("rollback released policy ownership")
		}
	}
}

// A reverse-migration failure must destroy both candidate SAD pairs while leaving
// PROTECT policies owned, so a caller cannot mistake the error for an intact tunnel.
func TestXFRMMigrationFailedRollbackDiscardsStates(t *testing.T) {
	b, m := migrationKernelFixture(t, 64, false)
	original := xfrmMigrationExecute
	t.Cleanup(func() { xfrmMigrationExecute = original })
	calls := 0
	xfrmMigrationExecute = func(req *nl.NetlinkRequest) error {
		calls++
		if calls == 2 || calls == 3 {
			return unix.EIO
		}
		return original(req)
	}
	if err := b.MigrateTunnel(m); !errors.Is(err, ErrTunnelMigrationLost) {
		t.Fatalf("failed rollback result = %v", err)
	}
	states, err := b.ListSAs(m.IfID)
	if err != nil || len(states) != 0 {
		t.Fatalf("failed rollback stranded %d states: %v", len(states), err)
	}
	for _, p := range m.Policies {
		want, err := xfrmPolicyFromParams(p)
		if err != nil {
			t.Fatal(err)
		}
		got, err := netlink.XfrmPolicyGet(want)
		if err != nil || got.Action != netlink.XFRM_POLICY_ALLOW || len(got.Tmpls) != 1 {
			t.Fatalf("failed rollback removed traffic protection: %v", err)
		}
		if owner, known := b.policies.ownerOf(p); !known || owner != p.Owner {
			t.Fatal("failed rollback released policy ownership")
		}
	}
	if _, watched := b.espForms.reg.target(m.InboundSPI); watched {
		t.Fatal("failed rollback left a stale ESP demultiplexer entry")
	}
}

func TestXFRMMigrationRefusesAnotherPolicyOwner(t *testing.T) {
	b, m := migrationKernelFixture(t, 64, false)
	before := migrationSnapshot(t, m.InboundSPI, m.OldLocal, m.IfID)
	m.Policies[0].Owner = "another-peer"
	var owned *PolicyOwnedError
	if err := b.MigrateTunnel(m); !errors.As(err, &owned) {
		t.Fatalf("foreign policy owner was accepted: %v", err)
	}
	assertMigrationSnapshot(t, before, migrationSnapshot(t, m.InboundSPI, m.OldLocal, m.IfID))
	p, err := xfrmPolicyFromParams(m.Policies[0])
	if err != nil {
		t.Fatal(err)
	}
	got, err := netlink.XfrmPolicyGet(p)
	if err != nil || len(got.Tmpls) != 1 || !got.Tmpls[0].Dst.Equal(m.OldLocal) {
		t.Fatalf("refused migration changed another owner's policy: %v", err)
	}
}

func TestXFRMMigrationRefusesOccupiedDestinationSPI(t *testing.T) {
	b, m := migrationKernelFixture(t, 64, false)
	foreign, err := xfrmStateFromParams(SAParams{
		SPI: m.InboundSPI, Src: m.NewRemote, Dst: m.NewLocal,
		Proto: ProtoESP, Mode: ModeTunnel, IfID: m.IfID, ReqID: 999,
		EncAlgo: "aes256gcm", EncKey: encapTestAEADKey, IsAEAD: true, ReplayWin: 64,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.XfrmStateAdd(foreign); err != nil {
		t.Fatal(err)
	}
	before := migrationSnapshot(t, m.InboundSPI, m.OldLocal, m.IfID)
	if err := b.MigrateTunnel(m); err == nil {
		t.Fatal("occupied destination SPI was accepted")
	}
	assertMigrationSnapshot(t, before, migrationSnapshot(t, m.InboundSPI, m.OldLocal, m.IfID))
	got := migrationSnapshot(t, m.InboundSPI, m.NewLocal, m.IfID)
	if got.state.info.Reqid != 999 {
		t.Fatal("refused migration replaced the foreign SA")
	}
}

func TestXFRMMigrationRetainsFailedDeletionForTeardown(t *testing.T) {
	b, m := migrationKernelFixture(t, 64, false)
	execute, remove := xfrmMigrationExecute, xfrmMigrationStateDel
	t.Cleanup(func() {
		xfrmMigrationExecute, xfrmMigrationStateDel = execute, remove
	})
	calls := 0
	xfrmMigrationExecute = func(req *nl.NetlinkRequest) error {
		calls++
		if calls == 2 || calls == 3 {
			return unix.EIO
		}
		return execute(req)
	}
	xfrmMigrationStateDel = func(id *netlink.XfrmState) error {
		if id.Dst.Equal(m.NewLocal) {
			return unix.EIO
		}
		return remove(id)
	}
	if err := b.MigrateTunnel(m); !errors.Is(err, ErrTunnelMigrationLost) {
		t.Fatalf("failed cleanup result = %v", err)
	}
	// Teardown still identifies the Child by its uncommitted old destination.
	// RemoveSA must retry the alternate destination retained by migration.
	xfrmMigrationStateDel = remove
	if err := b.RemoveSA(m.InboundSPI, m.OldLocal, ProtoESP); err != nil {
		t.Fatal(err)
	}
	states, err := b.ListSAs(m.IfID)
	if err != nil || len(states) != 0 {
		t.Fatalf("teardown stranded %d states after retry: %v", len(states), err)
	}
}

// Send actual protected traffic across a veth into a second namespace before and
// after mobility. Reading the peer's packets rules out a self-addressed route that
// never used the outbound policy, and catches sequence/nonce reuse on the wire.
func TestXFRMMigrationKeepsOutboundSequenceOnPeerWire(t *testing.T) {
	b, m := migrationKernelFixture(t, 64, false)
	udp, esp := migrationPeerCapture(t, m.IfID)
	conn, err := net.DialIP("ip4:6", &net.IPAddr{IP: net.ParseIP("10.71.1.1")}, &net.IPAddr{IP: net.ParseIP("10.71.2.1")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})
	var tcp [20]byte
	binary.BigEndian.PutUint16(tcp[:2], 50100)
	binary.BigEndian.PutUint16(tcp[2:4], 179)
	tcp[12], tcp[13] = 5<<4, 2
	if err := udp.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(tcp[:]); err != nil {
		t.Fatal(err)
	}
	var packet [2048]byte
	n, from, err := udp.ReadFromUDP(packet[:])
	if err != nil {
		t.Fatalf("peer received no pre-migration UDP-ESP: %v", err)
	}
	if n < 8 || binary.BigEndian.Uint32(packet[:4]) != m.OutboundSPI || binary.BigEndian.Uint32(packet[4:8]) != 101 || !from.IP.Equal(m.OldLocal) {
		t.Fatalf("wrong pre-migration ESP identity: %x from %v", packet[:n], from)
	}
	before := migrationSnapshot(t, m.OutboundSPI, m.OldRemote, m.IfID)
	if before.state.info.Curlft.Packets != 91 || before.state.info.Curlft.Bytes <= 9000 {
		t.Fatal("positive-control packet did not traverse the outbound SA")
	}
	if err := b.MigrateTunnel(m); err != nil {
		t.Fatal(err)
	}
	assertMigrationSnapshot(t, before, migrationSnapshot(t, m.OutboundSPI, m.NewRemote, m.IfID))
	if err := esp.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(tcp[:]); err != nil {
		t.Fatal(err)
	}
	n, source, err := esp.ReadFromIP(packet[:])
	if err != nil {
		t.Fatalf("peer received no post-migration bare ESP: %v", err)
	}
	if n < 8 || binary.BigEndian.Uint32(packet[:4]) != m.OutboundSPI || binary.BigEndian.Uint32(packet[4:8]) != 102 || !source.IP.Equal(m.NewLocal) {
		t.Fatalf("post-migration ESP reused a sequence or retained the old endpoint: %x from %v", packet[:n], source)
	}
	after := migrationSnapshot(t, m.OutboundSPI, m.NewRemote, m.IfID)
	if after.state.info.Curlft.Packets != 92 || after.state.info.Curlft.Bytes <= before.state.info.Curlft.Bytes {
		t.Fatal("post-migration packet did not advance the existing SA counters")
	}
}

// The caller's encapNetns has locked this goroutine to its namespace's thread.
// Sockets opened in the peer remain attached to that namespace after Set(local).
func migrationPeerCapture(t *testing.T, ifID uint32) (*net.UDPConn, *net.IPConn) {
	t.Helper()
	local, err := netns.Get()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := netns.Set(local); err != nil {
			t.Error(err)
		}
		if err := local.Close(); err != nil {
			t.Error(err)
		}
	}()
	peer, err := netns.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := peer.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := netns.Set(local); err != nil {
		t.Fatal(err)
	}
	link := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: "mobike0"}, PeerName: "mobike1", PeerNamespace: netlink.NsFd(int(peer))}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	underlay := migrationLinkAddrs(t, "mobike0", "198.51.100.1/24", "192.0.2.1/24")
	xfrmi := &netlink.Xfrmi{LinkAttrs: netlink.LinkAttrs{Name: "mobike-xfrm", ParentIndex: underlay.Attrs().Index}, Ifid: ifID}
	if err := netlink.LinkAdd(xfrmi); err != nil {
		t.Fatal(err)
	}
	xfrmLink := migrationLinkAddrs(t, "mobike-xfrm", "10.71.1.1/24")
	_, destination, err := net.ParseCIDR("10.71.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if err := netlink.RouteAdd(&netlink.Route{LinkIndex: xfrmLink.Attrs().Index, Dst: destination, Scope: netlink.SCOPE_LINK}); err != nil {
		t.Fatal(err)
	}
	if err := netns.Set(peer); err != nil {
		t.Fatal(err)
	}
	migrationLinkAddrs(t, "mobike1", "198.51.100.2/24", "192.0.2.2/24")
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 4500})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := udp.Close(); err != nil {
			t.Error(err)
		}
	})
	esp, err := net.ListenIP("ip4:50", &net.IPAddr{IP: net.IPv4zero})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := esp.Close(); err != nil {
			t.Error(err)
		}
	})
	return udp, esp
}

func migrationLinkAddrs(t *testing.T, name string, addresses ...string) netlink.Link {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatal(err)
	}
	for _, cidr := range addresses {
		addr, err := netlink.ParseAddr(cidr)
		if err != nil {
			t.Fatal(err)
		}
		if err := netlink.AddrAdd(link, addr); err != nil {
			t.Fatal(err)
		}
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	return link
}

// Installing a replacement Child after a mapped-port move must configure the
// receiver as well as the kernel state. Observe the production receiver's UDP
// datagram rather than its registry: the old implementation injected port 4500.
func TestXFRMMigrationRekeyReceiverUsesMappedPorts(t *testing.T) {
	if !encapOwnProcess(t) {
		return
	}
	encapNetns(t)
	encapNetnsUsable(t)
	dp, err := newXFRMBackend()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := dp.Close(); err != nil {
			t.Error(err)
		}
	})
	const spi = 0x71910003
	if err := dp.InstallSA(SAParams{
		SPI: spi, Src: net.ParseIP(encapLoopbackPeerAddr), Dst: net.ParseIP(encapLoopbackAddr),
		Proto: ProtoESP, Mode: ModeTunnel, ReqID: 740, ReplayWin: 32,
		EncAlgo: "aes256gcm", EncKey: encapTestAEADKey, IsAEAD: true,
		UDPEncap: true, UDPEncapSPort: 62001, UDPEncapDPort: 48001, AcceptBothESPForms: true,
	}); err != nil {
		t.Fatal(err)
	}
	reader, err := (&net.ListenConfig{}).ListenPacket(t.Context(), "ip4:udp", encapLoopbackAddr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := reader.SetReadDeadline(time.Now().Add(encapHybridReadDeadline)); err != nil {
		t.Fatal(err)
	}
	esp := encapESPBytes(spi)
	encapInjectBare(t, encapLoopbackPeerAddr, esp)
	var packet [2048]byte
	n, _, err := reader.ReadFrom(packet[:])
	if err != nil {
		t.Fatalf("production receiver emitted no UDP datagram: %v", err)
	}
	if n != 8+len(esp) || !bytes.Equal(packet[8:n], esp) {
		t.Fatalf("receiver changed ESP data: %x", packet[:n])
	}
	if binary.BigEndian.Uint16(packet[:2]) != 62001 || binary.BigEndian.Uint16(packet[2:4]) != 48001 {
		t.Fatalf("replacement receiver used stale UDP ports: %x", packet[:8])
	}
}
