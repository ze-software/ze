// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- inbound Child SA states and the second ESP form
// Related: rfc4301_sad_selector_linux_test.go -- the selector probe whose Child SA, inner packets and listeners this test reuses
//
// VALIDATES: against a real Linux XFRM stack on Ze's kernel, that bare ESP received on a
// tunnel-mode Child SA whose IKE SA runs on UDP port 4500 with no NAT detected is checked
// against the SA's inbound selectors. That Child SA is installed with the ESP-in-UDP
// template (newChildSA sets UDPEncap on port 4500) and AcceptBothESPForms, the shape of the
// defect in plan/release/fixes/spec-ipsec-dual-form-receive-spd-bypass.md. Bare ESP inside
// the selectors, whole and fragmented, is delivered; bare ESP whose inner source is outside
// them is dropped by its own SA selector (XfrmInStateMismatch) and never delivered.
// A temporary explicit DISCARD inside that selector separately proves the SPD is read
// (XfrmInPolBlock), on both wire paths, before removal restores valid delivery. It holds on both
// of the receiver's paths, and the test pins which path carried each datagram: a whole
// datagram is rewritten to ESP-in-UDP by the fast path's tc ingress program on the veth
// (its IPv4 rewrite counter, read through dataplane.ESPFormFastPathReporter, advances by
// one), and a fragmented one is left to the complete path, which reads it reassembled from
// the raw ESP socket and re-presents it on ze-espform (the counter does not move). Both
// devices carry disable_policy=0, so both paths run the SA and SPD checks.
// PREVENTS: the inbound SPD bypass of the dual-form receive path. XFRM refuses the bare form
// on a templated state, espFormReceiver re-presents it as ESP-in-UDP to Ze's own address,
// and that datagram arrives on lo. Linux creates lo with disable_policy set
// (net/ipv4/devinet.c), so the input route marks the packet IPSKB_NOPOLICY and
// __xfrm_policy_check2 (include/net/xfrm.h) returns before any policy is read: the inner
// packet is delivered whatever its addresses.
//
// Why the loopback probes could not see this: every earlier dual-form probe put both ends on
// lo in one namespace, so the bare datagram and the re-presented one both arrived on lo, and
// a probe that clears lo's disable_policy (as sadSelNetns must, to measure anything) removes
// the very flag the defect rides on. Here the peer sits in a second network namespace and
// reaches Ze over a veth, which carries disable_policy=0 like any real link, and lo keeps the
// disable_policy Linux gives it. A packet that reaches XFRM on the veth is policy-checked; a
// packet re-presented over lo is not, so the drop rows separate the two paths.
//
// THE SPD IS THE PRODUCTION ONE. Before the Child SA is installed, espPolSPD installs the
// node-wide catch-all `unmatched discard` through installUnmatched and then the IKE bypass
// through installIKEBypass, in the order applyConfig (apply.go) runs them on the first
// apply, and the test reads both back through the dataplane's policy listing, together
// with the ESP form reader's per-socket policy (esp-form-socket). Two things follow from
// that shape. An inner packet outside its own state selector is dropped before the
// SPD lookup as XfrmInStateMismatch (__xfrm_policy_check, net/xfrm/xfrm_policy.c).
// A more-specific temporary DISCARD inside the state selector makes the SPD reject
// an otherwise admitted packet as XfrmInPolBlock. Both checks require XfrmInNoPols
// unchanged. The bare datagram itself, outer
// 192.0.2.2 -> 192.0.2.1 protocol 50, matches no Child SA entry, so on its way to the raw
// ESP reader (raw_rcv runs xfrm4_policy_check, net/ipv4/raw.c) it meets the catch-all too:
// only the reader's per-socket exemption lets the refused copy through.
//
// RED PROOFS the main thread records:
//   - Delete the espFormBypassInboundPolicy call in startLocked
//     (ike/dataplane/espform_linux.go). Rows 3 and 4 of
//     TestRFC4301BareESPOnTemplatedSAIsPolicyChecked go red: raw_rcv drops the refused
//     copy at the catch-all, the reader never receives it, and the fragmented datagram is
//     never re-presented, so row 3 is not delivered. Without the catch-all the copy met no
//     entry and passed, which is why the SPD has the production shape here.
//   - Make writeESPForm (ike/dataplane/espform.go) write `buf[1] = 0` instead of the
//     received TOS. The fragmented row of TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE
//     goes red: the complete path re-presents the reassembled datagram with no CE, and the
//     inner packet keeps its ECN-capable 0b10. Its whole-datagram row stays green: the fast
//     path rewrites the header in place and keeps the TOS.
//   - Force the fast path's firewall gate open: make writeGateLocked
//     (ike/dataplane/espform_fastpath_gate_linux.go) write espFormFastPathGateOpen into the
//     map whatever value it is given, or drop the gate lookup from the program
//     (espform_program_linux.go). The drop-rule row of
//     TestRFC4301BareESPFirewallDropKeepsTheFastPathOff goes red: the program rewrites the
//     whole bare datagram to ESP-in-UDP at tc ingress, it passes the `meta l4proto esp
//     drop` rule as UDP, XFRM decrypts it and the inner packet is delivered, Rewritten4
//     moves by one and the rule's counter does not move.
//
// METHOD: privileged QEMU on Ze's kernel. Moving a link into another network namespace and
// creating two namespaces from one locked thread need CAP_NET_ADMIN and CAP_SYS_ADMIN in the
// initial user namespace, and the probe reads the thread's own /proc/thread-self/net
// counters. The whole test runs on one locked thread, with no t.Run subtests, because the
// namespaces are bound to that thread. The ESP datagrams are built here with the key read back
// from the installed inbound state, so the kernel decrypts them as it would the peer's.

//go:build linux

package engine

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"net"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	// The outer addresses, one per namespace, on the two ends of the veth.
	espPolZeOuter   = "192.0.2.1"
	espPolPeerOuter = "192.0.2.2"
	espPolOuterBits = "/24"
	espPolZeLink    = "zeespz0"
	espPolPeerLink  = "zeespp0"

	// espPolLinkMTU is the veth MTU the fragment rows exceed.
	espPolLinkMTU = 1500
	// espPolLargeBodyOctets is the inner UDP body of the fragment rows: its ESP datagram
	// is over twice espPolLinkMTU, so the peer's kernel sends it in fragments.
	espPolLargeBodyOctets = 3000
)

// espPolLab is the probe's two network namespaces: Ze's, where the calling thread stays
// between steps, and the peer's, entered only to create the peer's sockets.
type espPolLab struct {
	ze   netns.NsHandle
	peer netns.NsHandle
}

// newEspPolLab skips unless the test runs as root, then locks the thread and builds the
// two namespaces joined by a veth, leaving the thread in Ze's. Ze's side holds the outer
// address, the inner local address on lo and a default route through the peer; the peer
// side holds its outer address and a static neighbor entry for Ze.
func newEspPolLab(t *testing.T) *espPolLab {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("requires privileged QEMU on Ze's kernel: two network namespaces joined by a veth need CAP_NET_ADMIN and CAP_SYS_ADMIN in the initial user namespace, which userns.Enter cannot grant; run it as root in the guest (`./le test qemu all`, or `./le rfc record ... kernel <vmlinuz>`)")
	}
	runtime.LockOSThread()
	origin, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("read the thread's network namespace: %v", err)
	}
	ze, err := netns.New()
	if err != nil {
		origin.Close() //nolint:errcheck // the unshare already failed; that error is the one reported
		runtime.UnlockOSThread()
		t.Fatalf("make Ze's network namespace: %v", err)
	}
	peer, err := netns.New()
	if err != nil {
		ze.Close() //nolint:errcheck // the second unshare failed; that error is the one reported
		if setErr := netns.Set(origin); setErr != nil {
			// The thread stays locked, so the runtime retires it with the namespace.
			t.Fatalf("make the peer's network namespace: %v; and return the thread: %v", err, setErr)
		}
		origin.Close() //nolint:errcheck // restored above; the handle is spent
		runtime.UnlockOSThread()
		t.Fatalf("make the peer's network namespace: %v", err)
	}
	lab := &espPolLab{ze: ze, peer: peer}
	t.Cleanup(func() {
		peer.Close() //nolint:errcheck // the namespace goes with its last reference
		ze.Close()   //nolint:errcheck // the namespace goes with its last reference
		if err := netns.Set(origin); err != nil {
			// The thread stays locked, so the runtime retires it with the namespace.
			t.Errorf("return the thread to its network namespace: %v", err)
			return
		}
		origin.Close() //nolint:errcheck // restored above; the handle is spent
		runtime.UnlockOSThread()
	})

	lab.enter(t, ze)
	veth := &netlink.Veth{
		Name:          espPolZeLink,
		MTU:           espPolLinkMTU,
		PeerName:      espPolPeerLink,
		PeerMTU:       espPolLinkMTU,
		PeerNamespace: netlink.NsFd(int(peer)),
	}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatalf("add the veth: %v", err)
	}
	espPolUp(t, "lo")
	espPolAddr(t, "lo", sadSelInnerLocal+"/32")
	espPolAddr(t, espPolZeLink, espPolZeOuter+espPolOuterBits)
	espPolUp(t, espPolZeLink)
	// The reverse-path filter runs on the decapsulated packet before the policy check, so
	// a strict setting inherited from the initial namespace would drop the inner packet
	// first, on lo for the re-presented form and on the veth for a source with no route.
	// The probe measures the policy check, so it clears the filter on both devices. The
	// default route through the peer gives every inner source a route, as a router has,
	// for a link that later sets a loose filter of its own.
	for _, conf := range []string{"all", "lo", espPolZeLink} {
		path := "/proc/sys/net/ipv4/conf/" + conf + "/rp_filter"
		if err := os.WriteFile(path, []byte("0"), 0o600); err != nil {
			t.Fatalf("clear %s: %v", path, err)
		}
	}
	zeLink := espPolLink(t, espPolZeLink)
	if err := netlink.RouteAdd(&netlink.Route{
		LinkIndex: zeLink.Attrs().Index, Gw: net.ParseIP(espPolPeerOuter),
	}); err != nil {
		t.Fatalf("add the default route through the peer: %v", err)
	}
	zeMAC := zeLink.Attrs().HardwareAddr

	lab.enter(t, peer)
	espPolUp(t, "lo")
	espPolAddr(t, espPolPeerLink, espPolPeerOuter+espPolOuterBits)
	espPolUp(t, espPolPeerLink)
	// Ze would answer ARP; a static entry keeps the first datagram of each row off the
	// neighbor queue.
	if err := netlink.NeighAdd(&netlink.Neigh{
		LinkIndex: espPolLink(t, espPolPeerLink).Attrs().Index, Family: unix.AF_INET,
		State: netlink.NUD_PERMANENT, IP: net.ParseIP(espPolZeOuter), HardwareAddr: zeMAC,
	}); err != nil {
		t.Fatalf("add the peer's neighbor entry for Ze: %v", err)
	}
	lab.enter(t, ze)
	return lab
}

// enter moves the calling thread into ns, one of the lab's two namespaces.
func (l *espPolLab) enter(t *testing.T, ns netns.NsHandle) {
	t.Helper()
	if err := netns.Set(ns); err != nil {
		t.Fatalf("enter a probe namespace: %v", err)
	}
}

func espPolLink(t *testing.T, name string) netlink.Link {
	t.Helper()
	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("find %s: %v", name, err)
	}
	return link
}

func espPolUp(t *testing.T, name string) {
	t.Helper()
	if err := netlink.LinkSetUp(espPolLink(t, name)); err != nil {
		t.Fatalf("bring %s up: %v", name, err)
	}
}

func espPolAddr(t *testing.T, name, cidr string) {
	t.Helper()
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		t.Fatalf("parse %s: %v", cidr, err)
	}
	if err := netlink.AddrAdd(espPolLink(t, name), addr); err != nil {
		t.Fatalf("add %s to %s: %v", cidr, name, err)
	}
}

// espPolSPD loads the real XFRM backend in Ze's namespace and gives it the SPD a first
// configuration apply builds before any IKE socket opens: the node-wide catch-all
// `unmatched discard` through installUnmatched, then the IKE bypass through
// installIKEBypass, in applyConfig's order (apply.go). The application override guard,
// which applyConfig attaches between the two, is a cgroup program and changes nothing a
// packet in this probe meets, so it is not attached. Cleanup removes both sets in the
// order releaseSPD does, bypass first and catch-all last, before the backend closes.
func espPolSPD(t *testing.T) {
	t.Helper()
	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatalf("load xfrm backend: %v", err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close xfrm backend: %v", err)
		}
	})
	log := slogutil.DiscardLogger()
	if err := installUnmatched(dataplane.Get(), dataplane.SPActionDiscard, log); err != nil {
		t.Fatalf("install the SPD catch-all unmatched discard: %v", err)
	}
	installIKEBypass(dataplane.Get(), log)
	t.Cleanup(func() {
		removeIKEBypass(dataplane.Get(), log)
		removeUnmatched(dataplane.Get(), log)
	})
}

// espPolWantSPD fails the test unless the dataplane's policy listing holds what the probe
// relies on: the IPv4 catch-all as an inbound DISCARD, the IPv4 inbound IKE bypass on UDP
// port 4500, and the ESP form reader's per-socket policy. It is read back through
// ListPolicies, the kernel's SPD dump, so it judges what the kernel holds rather than what
// the installers were asked for.
func espPolWantSPD(t *testing.T) {
	t.Helper()
	policies, err := dataplane.Get().ListPolicies()
	if err != nil {
		t.Fatalf("list the SPD: %v", err)
	}
	var catchAll, bypass, reader bool
	for i := range policies {
		p := &policies[i]
		ipv4 := p.Src != nil && p.Src.IP.To4() != nil
		switch {
		case p.PerSocket && p.Role == dataplane.PolicyRoleESPFormSocket:
			reader = true
		case p.Role == dataplane.PolicyRoleCatchAll && p.Dir == dataplane.SADirIn && ipv4:
			catchAll = p.Action == dataplane.SPActionDiscard
		case p.Role == dataplane.PolicyRoleIKEBypass && p.Dir == dataplane.SADirIn && ipv4 &&
			p.DstPort == dataplane.ExactPortMatch(transport.NATTPort):
			bypass = p.Action == dataplane.SPActionBypass
		}
	}
	if !catchAll {
		t.Errorf("the SPD listing holds no IPv4 inbound catch-all DISCARD (%d entries); the drop rows would not judge the production SPD", len(policies))
	}
	if !bypass {
		t.Errorf("the SPD listing holds no IPv4 inbound IKE bypass on UDP port %d (%d entries)", transport.NATTPort, len(policies))
	}
	if !reader {
		t.Errorf("the SPD listing holds no esp-form-socket policy (%d entries); the catch-all would drop the refused copy before the reader reads it", len(policies))
	}
}

// espPolChild installs one tunnel-mode Child SA through createFirstChildSA over the XFRM
// backend espPolSPD loaded, in Ze's namespace, for an IKE SA that floated to port 4500 with
// no NAT detected. The NAT-T socket is bound on Ze's outer address and carries
// UDP_ENCAP_ESPINUDP, as apply.go sets it in production. The negotiated pair is
// sadSelChild's tunnel pair: UDP to local port 5000 of 10.2.0.0/16 from 10.1.0.0/16.
func espPolChild(t *testing.T) *ChildSA {
	t.Helper()
	natt := mbTransport(t, net.JoinHostPort(espPolZeOuter, strconv.Itoa(transport.NATTPort)), true)
	if err := transport.EnableESPInUDP(natt.Conn()); err != nil {
		t.Fatalf("set UDP_ENCAP_ESPINUDP on the NAT-T socket: %v", err)
	}

	sa := testSA()
	sa.IsInitiator = true
	sa.bindSockets(nil, natt)
	sa.floatToNATTPort()
	sa.NATDetected = false
	sa.peerEndpoint = &net.UDPAddr{IP: net.ParseIP(espPolPeerOuter), Port: transport.NATTPort}
	tsi := mustCIDRNet(t, "10.2.0.0/16")
	tsr := mustCIDRNet(t, "10.1.0.0/16")
	sa.NegotiatedTSi, sa.NegotiatedTSr = tsi, tsr
	sa.NegotiatedPairs = []tsPair{{
		I: tsSelector{Net: tsi, Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: sadSelPort}, Proto: sadSelProtoUDP},
		R: tsSelector{Net: tsr, Proto: sadSelProtoUDP},
	}}
	group := ipsec.ESPGroup{
		Name:      "esp-gcm",
		Lifetime:  3600,
		PFS:       ipsec.PFSDisable,
		Proposals: []ipsec.ESPProposal{{Number: 1, Encryption: ipsec.EncryptionAES256GCM}},
	}
	child, err := createFirstChildSA(sa, group, espPolZeOuter, espPolPeerOuter, 0, dataplane.Get(), slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	t.Cleanup(child.Clear)
	if !child.ESPInstalled {
		t.Fatal("the Child SA was not installed into XFRM")
	}
	if !child.UDPEncap {
		t.Fatal("the Child SA of an IKE SA on port 4500 is not UDP-encapsulated; the probe does not hold the defect's shape")
	}
	if child.NATDetected {
		t.Fatal("the Child SA records a NAT; the probe needs port 4500 with no NAT")
	}
	return child
}

// espPolSender sends ESP for the inbound SA of one Child SA from the peer's namespace, in
// both wire forms, keyed with the key the kernel state holds. The two sockets share one
// sequence counter, so neither form replays the other's numbers.
type espPolSender struct {
	spi   uint32
	seq   uint32
	aead  cipher.AEAD
	salt  []byte
	bare  *net.IPConn
	encap *net.UDPConn
}

// newEspPolSender reads the inbound state's key in Ze's namespace, then opens the sender's
// sockets in the peer's: a socket keeps the namespace it was created in. The bare socket
// does not set DF, so a datagram over the link MTU leaves in fragments.
func newEspPolSender(t *testing.T, lab *espPolLab, child *ChildSA) *espPolSender {
	t.Helper()
	state, err := netlink.XfrmStateGet(&netlink.XfrmState{
		Dst: net.ParseIP(espPolZeOuter), Spi: int(child.InboundSPI), Proto: netlink.XFRM_PROTO_ESP,
	})
	if err != nil {
		t.Fatalf("read back the inbound state: %v", err)
	}
	if state.Encap == nil || state.Encap.Type != netlink.XFRM_ENCAP_ESPINUDP {
		t.Fatalf("the inbound state carries no ESP-in-UDP template (%+v); the probe does not hold the defect's shape", state.Encap)
	}
	if state.Aead == nil || len(state.Aead.Key) != 36 {
		t.Fatalf("inbound state carries no AES-256-GCM key: %+v", state.Aead)
	}
	key := state.Aead.Key
	block, err := aes.NewCipher(key[:32])
	if err != nil {
		t.Fatalf("aes: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}

	lab.enter(t, lab.peer)
	bare, bareErr := net.ListenIP("ip4:50", &net.IPAddr{IP: net.ParseIP(espPolPeerOuter)})
	encap, encapErr := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(espPolPeerOuter), Port: transport.NATTPort})
	lab.enter(t, lab.ze)
	if bareErr == nil {
		t.Cleanup(func() {
			if err := bare.Close(); err != nil {
				t.Errorf("close the peer's raw ESP socket: %v", err)
			}
		})
	}
	if encapErr == nil {
		t.Cleanup(func() {
			if err := encap.Close(); err != nil {
				t.Errorf("close the peer's ESP-in-UDP socket: %v", err)
			}
		})
	}
	if bareErr != nil {
		t.Fatalf("the peer's raw ESP socket: %v", bareErr)
	}
	if encapErr != nil {
		t.Fatalf("the peer's ESP-in-UDP socket: %v", encapErr)
	}
	raw, err := bare.SyscallConn()
	if err != nil {
		t.Fatalf("the peer's raw ESP socket: %v", err)
	}
	var setErr error
	if err := raw.Control(func(fd uintptr) {
		setErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_DONT)
	}); err != nil {
		t.Fatalf("reach the peer's raw ESP socket: %v", err)
	}
	if setErr != nil {
		t.Fatalf("clear DF on the peer's raw ESP socket (IP_PMTUDISC_DONT): %v", setErr)
	}
	return &espPolSender{spi: child.InboundSPI, aead: aead, salt: key[32:], bare: bare, encap: encap}
}

// seal wraps payload in one ESP packet (RFC 4303 with RFC 4106 AES-GCM), the framing
// sadSelSender.send builds.
func (s *espPolSender) seal(nextHeader byte, payload []byte) []byte {
	s.seq++
	header := make([]byte, 8)
	binary.BigEndian.PutUint32(header[0:4], s.spi)
	binary.BigEndian.PutUint32(header[4:8], s.seq)
	iv := make([]byte, 8)
	binary.BigEndian.PutUint32(iv[4:8], s.seq)

	padOctets := (4 - (len(payload)+2)%4) % 4
	plain := make([]byte, 0, len(payload)+padOctets+2)
	plain = append(plain, payload...)
	for i := range padOctets {
		plain = append(plain, byte(i+1))
	}
	plain = append(plain, byte(padOctets), nextHeader)

	nonce := append(append([]byte{}, s.salt...), iv...)
	return append(append(header, iv...), s.aead.Seal(nil, nonce, plain, header)...)
}

// sendBare sends inner as bare ESP (IP protocol 50) to Ze's outer address over the veth.
func (s *espPolSender) sendBare(t *testing.T, inner []byte) {
	t.Helper()
	if _, err := s.bare.WriteToIP(s.seal(sadSelProtoIPv4, inner), &net.IPAddr{IP: net.ParseIP(espPolZeOuter)}); err != nil {
		t.Fatalf("send bare ESP: %v", err)
	}
}

// sendEncap sends inner as ESP-in-UDP, port 4500 to port 4500, the form the inbound
// state's template names.
func (s *espPolSender) sendEncap(t *testing.T, inner []byte) {
	t.Helper()
	target := &net.UDPAddr{IP: net.ParseIP(espPolZeOuter), Port: transport.NATTPort}
	if _, err := s.encap.WriteToUDP(s.seal(sadSelProtoIPv4, inner), target); err != nil {
		t.Fatalf("send ESP-in-UDP: %v", err)
	}
}

// espPolLargeUDP returns a UDP segment with no checksum carrying bodyOctets of payload.
func espPolLargeUDP(targetPort uint16, bodyOctets int) []byte {
	seg := make([]byte, 8+bodyOctets)
	binary.BigEndian.PutUint16(seg[0:2], sadSelSourcePort)
	binary.BigEndian.PutUint16(seg[2:4], targetPort)
	binary.BigEndian.PutUint16(seg[4:6], uint16(len(seg)))
	for i := range bodyOctets {
		seg[8+i] = byte(i)
	}
	return seg
}

// espPolNoPols reads XfrmInNoPols from the thread's own namespace: /proc/net follows the
// thread group leader, which stayed outside. A decapsulated packet that matches no SPD
// entry advances it.
func espPolNoPols(t *testing.T) int {
	t.Helper()
	return espPolStat(t, sadSelStatNoPols)
}

// espPolStat reads one XFRM counter from the thread's own namespace.
func espPolStat(t *testing.T, name string) int {
	t.Helper()
	raw, err := os.ReadFile("/proc/thread-self/net/xfrm_stat")
	if err != nil {
		t.Fatalf("read /proc/thread-self/net/xfrm_stat: %v; it is the instrument this test measures with", err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == name {
			n, convErr := strconv.Atoi(fields[1])
			if convErr != nil {
				t.Fatalf("parse %s = %q: %v", name, fields[1], convErr)
			}
			return n
		}
	}
	t.Fatalf("%s absent from /proc/thread-self/net/xfrm_stat", name)
	return 0
}

// espPolReassembled reads the IPv4 ReasmOKs counter of the thread's own namespace: the
// number of datagrams Ze's side rebuilt from fragments.
func espPolReassembled(t *testing.T) int {
	t.Helper()
	raw, err := os.ReadFile("/proc/thread-self/net/snmp")
	if err != nil {
		t.Fatalf("read /proc/thread-self/net/snmp: %v", err)
	}
	var names []string
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "Ip:" {
			continue
		}
		if names == nil {
			names = fields
			continue
		}
		for i, name := range names {
			if name != "ReasmOKs" || i >= len(fields) {
				continue
			}
			n, convErr := strconv.Atoi(fields[i])
			if convErr != nil {
				t.Fatalf("parse ReasmOKs = %q: %v", fields[i], convErr)
			}
			return n
		}
		break
	}
	t.Fatal("ReasmOKs absent from /proc/thread-self/net/snmp")
	return 0
}

// espPolFastPathWait bounds each wait for the fast path's worker: to attach the program to
// Ze's end of the veth, and to open or shut the program's firewall gate. The attach follows
// a netlink link dump on the worker's goroutine, and each judgment of the nftables ruleset
// follows the watch's signal of a commit, so both land after the call that caused them
// returns rather than during it.
const espPolFastPathWait = 5 * time.Second

// espPolFastPath reads the fast path of the loaded backend's dual-form receiver through
// the interface the doctor check reads it by.
func espPolFastPath(t *testing.T) dataplane.ESPFormFastPathState {
	t.Helper()
	reporter, ok := dataplane.Get().(dataplane.ESPFormFastPathReporter)
	if !ok {
		t.Fatalf("the loaded backend %T reports no ESP form fast path", dataplane.Get())
	}
	st, err := reporter.ESPFormFastPathState()
	if err != nil {
		t.Fatalf("read the ESP form fast path state: %v", err)
	}
	return st
}

// espPolFastPathAttached waits until the fast path rewrites on Ze's end of the veth: it is
// Attached with that link among its links, and its firewall gate is open. The program
// leaves every packet untouched while the gate is shut, and the gate opens only after the
// worker's first judgment of the nftables ruleset found no input-path rule that can drop
// ESP, which follows the attach, so Attached alone does not say the rows' whole datagrams
// will be rewritten. Opening is expected here: newEspPolLab's namespaces carry no nftables
// ruleset at all (espPolSPD's installUnmatched and installIKEBypass install XFRM policies,
// not nftables rules), so the judgment finds nothing that can drop ESP.
//
// It fails the test when the condition does not hold within espPolFastPathWait, when the
// fast path stops being Attached, and at once when FirewallCause is set: the ruleset could
// not be read or watched, and the gate then stays shut until stop. It polls the state, the
// condition itself, on a ticker.
func espPolFastPathAttached(t *testing.T) {
	t.Helper()
	st := espPolFastPathUntil(t, "attached to "+espPolZeLink+" with its firewall gate open",
		func(st dataplane.ESPFormFastPathState) bool {
			return slices.Contains(st.Attached, espPolZeLink) && st.GateOpen
		})
	if st.FirewallBlocked {
		t.Fatalf("the fast path's gate is open while it reports an nftables rule that can drop ESP (%s)", st.FirewallSite)
	}
}

// espPolFastPathUntil polls the fast path's state on a ticker until done holds, and
// answers that state. It fails the test when the fast path is not Attached, when its
// FirewallCause is set (the gate stays shut until stop, so no wait can open it), and when
// done does not hold within espPolFastPathWait; want says what done waits for.
func espPolFastPathUntil(t *testing.T, want string, done func(dataplane.ESPFormFastPathState) bool) dataplane.ESPFormFastPathState {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(espPolFastPathWait)
	for {
		st := espPolFastPath(t)
		if st.Status != dataplane.ESPFormFastPathAttached {
			t.Fatalf("the ESP form fast path is %s (cause %q), want %s: the whole-datagram rows would go through the complete path", st.Status, st.Cause, dataplane.ESPFormFastPathAttached)
		}
		if st.FirewallCause != "" {
			t.Fatalf("the ESP form fast path cannot judge the nftables ruleset (%s), so its gate stays shut; want it %s", st.FirewallCause, want)
		}
		if done(st) {
			return st
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatalf("after %s the ESP form fast path is not %s: attached to %v, gate open %t, firewall blocked %t at %q",
				espPolFastPathWait, want, st.Attached, st.GateOpen, st.FirewallBlocked, st.FirewallSite)
		}
	}
}

// RFC requirement: RFC4301-5.2-2 positive -- on a tunnel-mode Child SA installed by createFirstChildSA for an IKE SA on UDP port 4500 with no NAT detected, whose inbound state carries the ESP-in-UDP template, bare ESP received over a veth from a second network namespace, as one whole datagram and as a fragmented one, with an inner packet inside the SA's selectors, is decrypted and delivered.
// RFC requirement: RFC4301-5.2-2 negative -- on the same Child SA under the first-apply SPD, bare ESP, whole and fragmented, with an inner source outside the SA's selectors is refused by its own state selector (XfrmInStateMismatch +1 for the whole fast-path datagram and +2 for the fragmented complete-path datagram, including its original bare-form rejection; XfrmInPolBlock and XfrmInNoPols unchanged) and never delivered. An additional explicit DISCARD inside the state scope separately drops both bare forms through the SPD (XfrmInPolBlock +1, XfrmInNoPols unchanged, XfrmInStateMismatch +0 whole or +1 fragmented for the original bare-form rejection), and its removal restores both valid deliveries (only the fragmented form adds its one original bare-form mismatch).
//
// RFC 4301 Section 5.2: "Then match the packet against the inbound selectors identified
// by the SAD entry to verify that the received packet is appropriate for the SA via which
// it was received."
//
// TestRFC4301BareESPOnTemplatedSAIsPolicyChecked measures AC-3 of
// spec-ipsec-dual-form-receive-spd-bypass for IPv4, under the production SPD (espPolSPD),
// which the policy listing must show before the rows (espPolWantSPD). Method: a control row
// first sends an inner packet whose source is outside the selectors in the state's own
// form, ESP-in-UDP, which XFRM receives on the veth: its own SA selector must drop it
// before SPD lookup (XfrmInStateMismatch +1, XfrmInPolBlock and XfrmInNoPols +0),
// or the counters cannot judge the rows after it.
// Then four bare ESP rows: whole and inside the selectors (delivered), whole with the
// source outside (dropped by its SA selector), fragmented and inside (delivered, and
// Ze's side reassembled it), fragmented with the source outside (dropped by its SA selector).
// While bare ESP is re-presented over lo, the two drop rows are delivered instead.
//
// The rows also pin which of the receiver's two paths carried each datagram. Before the
// rows, the fast path must be Attached with Ze's end of the veth among its links, and its
// firewall gate open: the lab's namespaces carry no nftables ruleset, so the worker's
// first judgment opens it (espPolFastPathAttached). The
// whole-datagram rows (1 and 2) must each advance the fast path's IPv4 rewrite counter by
// exactly one: the tc ingress program on the veth turned them into ESP-in-UDP, so XFRM
// received them on the veth. The control row, already ESP-in-UDP, and the fragment rows
// (3 and 4) must leave the counter where it was: the program never rewrites UDP or a
// fragment, so the fragments reach XFRM through the complete path, reassembled and
// re-presented on ze-espform. A final explicit DISCARD on the admitted host pair
// separately proves the SPD check on both paths, then its removal restores both
// deliveries. The outside-selector rows retain the independent own-SA check.
// A complete-path row also counts the original bare ESP form rejection: +1
// XfrmInStateMismatch even for an admitted packet, or +2 when the re-presented
// inner packet also fails its own SA selector. The raw clone can finish first,
// so the counter observer waits for exact terminal values, with a fixed bound.
//
//ze:guest-root
func TestRFC4301BareESPOnTemplatedSAIsPolicyChecked(t *testing.T) {
	lab := newEspPolLab(t)
	espPolSPD(t)
	child := espPolChild(t)
	sender := newEspPolSender(t, lab, child)
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)
	espPolFastPathAttached(t)
	espPolWantSPD(t)
	if t.Failed() {
		t.FailNow()
	}

	whole := func(source string) []byte {
		return sadSelIPv4(source, sadSelProtoUDP, sadSelUDP(sadSelPort))
	}
	large := func(source string) []byte {
		return sadSelIPv4(source, sadSelProtoUDP, espPolLargeUDP(sadSelPort, espPolLargeBodyOctets))
	}
	if outer := 20 + 16 + len(large(sadSelInnerRemote)) + 2 + 16; outer <= espPolLinkMTU {
		t.Fatalf("the fragment rows' ESP datagram is %d octets, within the %d-octet link MTU; it would not be fragmented", outer, espPolLinkMTU)
	}

	// Control: the state's own form, received on the veth, is held to its own SA selector.
	observedBefore := sadSelReadDropCounters(t, espPolStat)
	rewritten := espPolFastPath(t).Rewritten4
	sender.sendEncap(t, whole(sadSelInnerStray))
	if sadSelDelivered(t, inside) {
		t.Fatal("control (ESP-in-UDP, inner source outside the selectors): delivered; want it dropped by its own SA selector, or the counters cannot judge the bare rows")
	}
	sadSelWantDropCounters(t, "control (ESP-in-UDP, inner source outside the selectors)", observedBefore, sadSelReadDropCounters(t, espPolStat), sadSelDropCounters{stateMismatch: 1})
	if t.Failed() {
		t.FailNow()
	}
	if moved := espPolFastPath(t).Rewritten4 - rewritten; moved != 0 {
		t.Errorf("control (ESP-in-UDP): the fast path rewrote %d datagrams, want 0; it must leave the state's own form alone", moved)
	}

	// Row 1: whole bare ESP, inside the selectors.
	rewritten = espPolFastPath(t).Rewritten4
	sender.sendBare(t, whole(sadSelInnerRemote))
	if !sadSelDelivered(t, inside) {
		t.Fatal("row 1 (whole bare ESP, inner packet inside the selectors): not delivered; the probe cannot judge the drop in row 2")
	}
	if moved := espPolFastPath(t).Rewritten4 - rewritten; moved != 1 {
		t.Errorf("row 1 (whole bare ESP, inside the selectors): the fast path rewrote %d datagrams, want 1; the datagram did not reach XFRM through the fast path on the veth", moved)
	}

	// Row 2: whole bare ESP, inner source outside the selectors.
	observedBefore = sadSelReadDropCounters(t, espPolStat)
	rewritten = espPolFastPath(t).Rewritten4
	sender.sendBare(t, whole(sadSelInnerStray))
	if sadSelDelivered(t, inside) {
		t.Error("row 2 (whole bare ESP, inner source outside the selectors): the inner packet was delivered, want it dropped by its own SA selector")
	}
	sadSelWantDropCounters(t, "row 2 (whole bare ESP, inner source outside the selectors)", observedBefore, sadSelReadDropCounters(t, espPolStat), sadSelDropCounters{stateMismatch: 1})
	if moved := espPolFastPath(t).Rewritten4 - rewritten; moved != 1 {
		t.Errorf("row 2 (whole bare ESP, outside the selectors): the fast path rewrote %d datagrams, want 1; the drop above did not judge the fast path", moved)
	}

	// Row 3: fragmented bare ESP, inside the selectors.
	observedBefore = sadSelReadDropCounters(t, espPolStat)
	reassembled := espPolReassembled(t)
	rewritten = espPolFastPath(t).Rewritten4
	sender.sendBare(t, large(sadSelInnerRemote))
	deliveredLarge := sadSelDelivered(t, inside)
	if moved := espPolReassembled(t) - reassembled; moved < 1 {
		t.Fatalf("row 3 (fragmented bare ESP, inside the selectors): ReasmOKs moved by %d; the datagram did not reach Ze in fragments, so the row tests nothing", moved)
	}
	if !deliveredLarge {
		t.Fatal("row 3 (fragmented bare ESP, inner packet inside the selectors): not delivered; the probe cannot judge the drop in row 4")
	}
	espPolWaitDropCounters(t, "row 3 (fragmented bare ESP, inside the selectors)", observedBefore, sadSelDropCounters{stateMismatch: 1})
	if moved := espPolFastPath(t).Rewritten4 - rewritten; moved != 0 {
		t.Errorf("row 3 (fragmented bare ESP, inside the selectors): the fast path rewrote %d datagrams, want 0; a fragment belongs to the complete path", moved)
	}

	// Row 4: fragmented bare ESP, inner source outside the selectors.
	observedBefore = sadSelReadDropCounters(t, espPolStat)
	reassembled = espPolReassembled(t)
	rewritten = espPolFastPath(t).Rewritten4
	sender.sendBare(t, large(sadSelInnerStray))
	if sadSelDelivered(t, inside) {
		t.Error("row 4 (fragmented bare ESP, inner source outside the selectors): the inner packet was delivered, want it dropped by its own SA selector")
	}
	if moved := espPolReassembled(t) - reassembled; moved < 1 {
		t.Errorf("row 4 (fragmented bare ESP, inner source outside the selectors): ReasmOKs moved by %d; the datagram did not reach Ze in fragments", moved)
	}
	espPolWaitDropCounters(t, "row 4 (fragmented bare ESP, inner source outside the selectors)", observedBefore, sadSelDropCounters{stateMismatch: 2})
	if moved := espPolFastPath(t).Rewritten4 - rewritten; moved != 0 {
		t.Errorf("row 4 (fragmented bare ESP, outside the selectors): the fast path rewrote %d datagrams, want 0; the drop above did not judge the complete path", moved)
	}

	espPolInsideDiscard(t, sender, inside, whole(sadSelInnerRemote), large(sadSelInnerRemote))
}

// espPolWaitDropCounters waits for the exact terminal counts of both copies.
// raw_v4_input queues the raw clone before ip_protocol_deliver_rcu invokes the
// original ESP handler. Inner delivery can therefore precede the original
// bare-form mismatch. An excess counter is a failure, never an accepted floor.
func espPolWaitDropCounters(t *testing.T, row string, before, want sadSelDropCounters) {
	t.Helper()
	target := sadSelDropCounters{
		stateMismatch: before.stateMismatch + want.stateMismatch,
		noPols:        before.noPols + want.noPols,
		polBlock:      before.polBlock + want.polBlock,
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(espPolFastPathWait)
	defer deadline.Stop()
	for {
		after := sadSelReadDropCounters(t, espPolStat)
		if after == target {
			sadSelWantDropCounters(t, row, before, after, want)
			return
		}
		if after.stateMismatch > target.stateMismatch {
			sadSelWantDropCounters(t, row, before, after, want)
			return
		}
		if after.noPols > target.noPols {
			sadSelWantDropCounters(t, row, before, after, want)
			return
		}
		if after.polBlock > target.polBlock {
			sadSelWantDropCounters(t, row, before, after, want)
			return
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			sadSelWantDropCounters(t, row, before, sadSelReadDropCounters(t, espPolStat), want)
			return
		}
	}
}

// espPolWantInsideDiscard reads back the temporary host policy without matching a
// Child SA's broader /16 selector or an ESP reader's per-socket exemption.
func espPolWantInsideDiscard(t *testing.T, discard dataplane.SPParams, want bool) {
	t.Helper()
	policies, err := dataplane.Get().ListPolicies()
	if err != nil {
		t.Fatalf("list the SPD for the temporary DISCARD: %v", err)
	}
	found := false
	for i := range policies {
		p := &policies[i]
		if p.PerSocket {
			continue
		}
		if !sameIPNet(p.Src, discard.Src) {
			continue
		}
		if !sameIPNet(p.Dst, discard.Dst) {
			continue
		}
		if p.Dir != discard.Dir {
			continue
		}
		if p.IfID != discard.IfID {
			continue
		}
		if p.UpperProto != discard.UpperProto {
			continue
		}
		if p.SrcPort != discard.SrcPort {
			continue
		}
		if p.DstPort != discard.DstPort {
			continue
		}
		found = true
		if want {
			if p.Action != discard.Action {
				t.Fatalf("temporary host policy action is %d, want DISCARD", p.Action)
			}
			if p.Priority != discard.Priority {
				t.Fatalf("temporary DISCARD priority is %d, want %d", p.Priority, discard.Priority)
			}
		}
	}
	if found != want {
		t.Fatalf("temporary host DISCARD present=%t, want %t", found, want)
	}
}

// espPolInsideDiscard proves that the SPD still decides after the actual SA's
// selector accepts a packet. The host-specific entry cannot replace the Child's
// /16 require-policy: InstallPolicy upserts by selector, independent of priority.
// Its lower numerical priority wins before PriorityChildSA. Removing this exact
// entry must restore valid delivery through both receiver paths.
func espPolInsideDiscard(t *testing.T, sender *espPolSender, inside *net.UDPConn, whole, large []byte) {
	t.Helper()
	discard := dataplane.SPParams{
		Src:        mustCIDRNet(t, sadSelInnerRemote+"/32"),
		Dst:        mustCIDRNet(t, sadSelInnerLocal+"/32"),
		Dir:        dataplane.SADirIn,
		UpperProto: sadSelProtoUDP,
		DstPort:    dataplane.ExactPortMatch(sadSelPort),
		Action:     dataplane.SPActionDiscard,
		Role:       dataplane.PolicyRoleOperator,
		Priority:   dataplane.PriorityChildSA - 1,
	}
	espPolWantInsideDiscard(t, discard, false)
	if err := dataplane.Get().InstallPolicy(discard); err != nil {
		t.Fatalf("install the temporary DISCARD inside the SA's scope: %v", err)
	}
	installed := true
	remove := func() {
		if !installed {
			return
		}
		if err := dataplane.Get().RemovePolicyParams(discard); err != nil {
			t.Fatalf("remove the exact temporary DISCARD: %v", err)
		}
		installed = false
	}
	t.Cleanup(remove)
	espPolWantInsideDiscard(t, discard, true)

	exercise := func(row string, packet []byte, fragmented, wantDelivered bool) {
		before := sadSelReadDropCounters(t, espPolStat)
		rewritten := espPolFastPath(t).Rewritten4
		reassembled := espPolReassembled(t)
		sender.sendBare(t, packet)
		if delivered := sadSelDelivered(t, inside); delivered != wantDelivered {
			t.Errorf("%s: delivered=%t, want %t", row, delivered, wantDelivered)
		}
		wantDrops := sadSelDropCounters{polBlock: 1}
		if wantDelivered {
			wantDrops.polBlock = 0
		}
		if fragmented {
			wantDrops.stateMismatch = 1
		}
		espPolWaitDropCounters(t, row, before, wantDrops)
		wantRewritten := uint64(1)
		if fragmented {
			wantRewritten = 0
			if moved := espPolReassembled(t) - reassembled; moved < 1 {
				t.Errorf("%s: ReasmOKs moved by %d; the datagram did not reach Ze in fragments", row, moved)
			}
		}
		if moved := espPolFastPath(t).Rewritten4 - rewritten; moved != wantRewritten {
			t.Errorf("%s: fast-path rewrites moved by %d, want %d", row, moved, wantRewritten)
		}
	}
	exercise("explicit DISCARD, whole bare ESP inside the SA scope", whole, false, false)
	exercise("explicit DISCARD, fragmented bare ESP inside the SA scope", large, true, false)
	remove()
	espPolWantInsideDiscard(t, discard, false)
	exercise("DISCARD removed, whole bare ESP inside the SA scope", whole, false, true)
	exercise("DISCARD removed, fragmented bare ESP inside the SA scope", large, true, true)
	espPolWantSPD(t)
}

// The ECN codepoints the ECN rows use, in the low two bits of the IPv4 TOS octet (RFC 3168
// Section 5): 0b11 is CE, 0b10 is one of the two ECN-capable codepoints. DSCP stays 0, so
// each constant is the whole octet.
const (
	espPolTOSCE         = 0x03
	espPolTOSECNCapable = 0x02
)

// espPolMarkECNCapable marks an inner IPv4 packet ECN-capable (0b10) and recomputes its
// header checksum: ip_rcv_core checks the inner header when the decapsulated packet is
// received again.
func espPolMarkECNCapable(inner []byte) []byte {
	inner[1] = espPolTOSECNCapable
	binary.BigEndian.PutUint16(inner[10:12], 0)
	binary.BigEndian.PutUint16(inner[10:12], sadSelChecksum(inner[:20]))
	return inner
}

// setOuterTOS sets the TOS octet of the outer header the peer's bare ESP socket sends
// with. A raw socket keeps the ECN bits it is given: __ip_sock_set_tos
// (net/ipv4/ip_sockglue.c) clears them only for SOCK_STREAM. Every fragment of a datagram
// sent over the link MTU carries the same octet.
func (s *espPolSender) setOuterTOS(t *testing.T, tos int) {
	t.Helper()
	raw, err := s.bare.SyscallConn()
	if err != nil {
		t.Fatalf("the peer's raw ESP socket: %v", err)
	}
	var setErr error
	if err := raw.Control(func(fd uintptr) {
		setErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_TOS, tos)
	}); err != nil {
		t.Fatalf("reach the peer's raw ESP socket: %v", err)
	}
	if setErr != nil {
		t.Fatalf("set IP_TOS %#02x on the peer's raw ESP socket: %v", tos, setErr)
	}
}

// espPolTOSListen binds the inner listener inside the selectors and asks the kernel for the
// TOS octet of every datagram it receives (IP_RECVTOS), which ip_cmsg_recv_tos
// (net/ipv4/ip_sockglue.c) reads from the delivered packet's own header:
// `put_cmsg(msg, SOL_IP, IP_TOS, 1, &ip_hdr(skb)->tos);`.
func espPolTOSListen(t *testing.T) *net.UDPConn {
	t.Helper()
	conn := sadSelListen(t, sadSelInnerLocal, sadSelPort)
	raw, err := conn.SyscallConn()
	if err != nil {
		t.Fatalf("the inner listener: %v", err)
	}
	var setErr error
	if err := raw.Control(func(fd uintptr) {
		setErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_RECVTOS, 1)
	}); err != nil {
		t.Fatalf("reach the inner listener: %v", err)
	}
	if setErr != nil {
		t.Fatalf("set IP_RECVTOS on the inner listener: %v", setErr)
	}
	return conn
}

// espPolReceivedTOS waits as sadSelDelivered does for one datagram on conn, and answers the
// TOS octet of the inner packet that carried it, from the IP_TOS control message, and
// whether one arrived at all. A datagram with no IP_TOS message fails the test: with
// IP_RECVTOS set the kernel attaches one to every datagram, so its absence is no answer.
func espPolReceivedTOS(t *testing.T, conn *net.UDPConn) (uint8, bool) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 8192)
	oob := make([]byte, 64)
	_, oobn, _, _, err := conn.ReadMsgUDP(buf, oob)
	if err != nil {
		return 0, false
	}
	msgs, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		t.Fatalf("parse the control messages of the received datagram: %v", err)
	}
	for _, m := range msgs {
		if m.Header.Level == unix.IPPROTO_IP && m.Header.Type == unix.IP_TOS && len(m.Data) >= 1 {
			return m.Data[0], true
		}
	}
	t.Fatalf("the datagram arrived with no IP_TOS control message although IP_RECVTOS is set (%d control octets)", oobn)
	return 0, false
}

// RFC requirement: RFC7296-2.24-2 positive -- on a tunnel-mode Child SA installed by createFirstChildSA for an IKE SA on UDP port 4500 with no NAT detected, whose inbound state carries the ESP-in-UDP template, under the SPD a first configuration apply builds, bare ESP received over a veth whose outer IPv4 header carries ECN CE, around an inner packet inside the SA's selectors marked ECN-capable (0b10), is delivered with the inner TOS octet CE (0x03), read from the IP_TOS control message, both as one whole datagram the fast path rewrites and as a fragmented one the complete path re-presents.
//
// RFC 7296 Section 2.24: decapsulators "MUST implement the tunnel encapsulation and
// decapsulation processing specified in [IPSECARCH] to prevent discarding of ECN
// congestion indications."
//
// TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE proves that neither of the dual-form
// receiver's paths discards the congestion indication on the bare form. The expected
// value is what XFRM computes: ipip_ecn_decapsulate (net/xfrm/xfrm_input.c, v7.0 lines
// 229 to 235) runs `if (INET_ECN_is_ce(XFRM_MODE_SKB_CB(skb)->tos))
// IP_ECN_set_ce(inner_iph);` on the outer TOS xfrm4_extract_header recorded, and
// IP_ECN_set_ce (include/net/inet_ecn.h) turns an ECN-capable inner field into CE
// (`iph->tos |= INET_ECN_CE;`), so 0x02 inside 0x03 arrives as 0x03. The state carries no
// XFRM_STATE_NOECN (xfrmStateFromParams), which would skip that step.
//
// Method: the same lab and SPD as TestRFC4301BareESPOnTemplatedSAIsPolicyChecked, with the
// peer's raw ESP socket sending at TOS CE. Row 1 is a whole datagram: the fast path's
// rewrite counter must advance by one, so it reached XFRM through the tc program, which
// keeps the TOS octet in place. Row 2 is a datagram over the link MTU: Ze's side must
// reassemble it and the counter must not move, so it reached XFRM through the complete
// path, whose reader reads the reassembled header's TOS (ip_frag_reasm keeps the head's
// CE when every fragment carries CE) and writes it into the re-presented header.
// MUTATION: writeESPForm writing buf[1] = 0 turns row 2 red (inner TOS 0x02); row 1 stays
// green, because the fast path never rebuilds the header.
//
//ze:guest-root
func TestRFC7296BareESPOnTemplatedSAKeepsTheOuterCE(t *testing.T) {
	lab := newEspPolLab(t)
	espPolSPD(t)
	child := espPolChild(t)
	sender := newEspPolSender(t, lab, child)
	listener := espPolTOSListen(t)
	espPolFastPathAttached(t)
	sender.setOuterTOS(t, espPolTOSCE)

	marked := func(payload []byte) []byte {
		return espPolMarkECNCapable(sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, payload))
	}
	large := marked(espPolLargeUDP(sadSelPort, espPolLargeBodyOctets))
	if outer := 20 + 16 + len(large) + 2 + 16; outer <= espPolLinkMTU {
		t.Fatalf("row 2's ESP datagram is %d octets, within the %d-octet link MTU; it would not be fragmented", outer, espPolLinkMTU)
	}

	// Row 1: whole bare ESP at outer TOS CE, through the fast path.
	rewritten := espPolFastPath(t).Rewritten4
	sender.sendBare(t, marked(sadSelUDP(sadSelPort)))
	tos, delivered := espPolReceivedTOS(t, listener)
	if !delivered {
		t.Fatal("row 1 (whole bare ESP, outer CE, inside the selectors): not delivered, so its TOS cannot be judged")
	}
	if moved := espPolFastPath(t).Rewritten4 - rewritten; moved != 1 {
		t.Errorf("row 1 (whole bare ESP, outer CE): the fast path rewrote %d datagrams, want 1; the row did not judge the fast path", moved)
	}
	if tos != espPolTOSCE {
		t.Errorf("row 1 (whole bare ESP, outer CE, inner 0b10): the inner packet arrived with TOS %#02x, want %#02x (CE); the fast path discarded the congestion indication", tos, espPolTOSCE)
	}

	// Row 2: fragmented bare ESP at outer TOS CE, through the complete path.
	reassembled := espPolReassembled(t)
	rewritten = espPolFastPath(t).Rewritten4
	sender.sendBare(t, large)
	tos, delivered = espPolReceivedTOS(t, listener)
	if moved := espPolReassembled(t) - reassembled; moved < 1 {
		t.Fatalf("row 2 (fragmented bare ESP, outer CE): ReasmOKs moved by %d; the datagram did not reach Ze in fragments, so the row tests nothing", moved)
	}
	if !delivered {
		t.Fatal("row 2 (fragmented bare ESP, outer CE, inside the selectors): not delivered, so its TOS cannot be judged")
	}
	if moved := espPolFastPath(t).Rewritten4 - rewritten; moved != 0 {
		t.Errorf("row 2 (fragmented bare ESP, outer CE): the fast path rewrote %d datagrams, want 0; a fragment belongs to the complete path", moved)
	}
	if tos != espPolTOSCE {
		t.Errorf("row 2 (fragmented bare ESP, outer CE, inner 0b10): the inner packet arrived with TOS %#02x, want %#02x (CE); the complete path discarded the congestion indication", tos, espPolTOSCE)
	}
}

// The operator firewall the firewall test installs in Ze's namespace: one inet table with
// one input base chain, whose policy accepts and whose one rule drops IPv4 and IPv6 ESP.
const (
	espPolFirewallTable = "zeespfw"
	espPolFirewallChain = "input"
)

// espPolNFT opens an nftables connection to Ze's namespace. The handle is passed
// explicitly rather than read from the calling thread, so every message reaches the
// namespace the program and the fast path's watch run in, whichever thread dials.
func espPolNFT(t *testing.T, lab *espPolLab) *nftables.Conn {
	t.Helper()
	conn, err := nftables.New(nftables.WithNetNSFd(int(lab.ze)))
	if err != nil {
		t.Fatalf("open an nftables connection to Ze's namespace: %v", err)
	}
	return conn
}

// espPolDropESP commits, in one transaction from outside Ze, the operator rule
// `table inet zeespfw { chain input { type filter hook input priority filter; policy
// accept; meta l4proto esp counter drop } }`, and answers the table and chain the rule
// lives in. The table goes with Ze's namespace at cleanup, so nothing removes it then.
func espPolDropESP(t *testing.T, lab *espPolLab) (*nftables.Table, *nftables.Chain) {
	t.Helper()
	conn := espPolNFT(t, lab)
	table := conn.AddTable(&nftables.Table{Family: nftables.TableFamilyINet, Name: espPolFirewallTable})
	policy := nftables.ChainPolicyAccept
	chain := conn.AddChain(&nftables.Chain{
		Name:     espPolFirewallChain,
		Table:    table,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookInput,
		Priority: nftables.ChainPriorityFilter,
		Policy:   &policy,
	})
	conn.AddRule(&nftables.Rule{
		Table: table,
		Chain: chain,
		Exprs: []expr.Any{
			&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{unix.IPPROTO_ESP}},
			&expr.Counter{},
			&expr.Verdict{Kind: expr.VerdictDrop},
		},
	})
	if err := conn.Flush(); err != nil {
		t.Fatalf("commit the drop-ESP table %s in Ze's namespace: %v", espPolFirewallTable, err)
	}
	return table, chain
}

// espPolDropped reads back, from the kernel, the packet count of the drop rule's counter:
// the number of datagrams the rule dropped. The chain must hold exactly that one rule with
// one counter, or the count would judge another rule.
func espPolDropped(t *testing.T, lab *espPolLab, table *nftables.Table, chain *nftables.Chain) uint64 {
	t.Helper()
	rules, err := espPolNFT(t, lab).GetRules(table, chain)
	if err != nil {
		t.Fatalf("read the rules of %s %s: %v", espPolFirewallTable, espPolFirewallChain, err)
	}
	if len(rules) != 1 {
		t.Fatalf("chain %s of table %s holds %d rules, want the one drop-ESP rule", espPolFirewallChain, espPolFirewallTable, len(rules))
	}
	for _, e := range rules[0].Exprs {
		if counter, ok := e.(*expr.Counter); ok {
			return counter.Packets
		}
	}
	t.Fatalf("the drop-ESP rule read back carries no counter (%d expressions)", len(rules[0].Exprs))
	return 0
}

// espPolRemoveFirewall deletes the drop-ESP table, in one transaction from outside Ze.
func espPolRemoveFirewall(t *testing.T, lab *espPolLab, table *nftables.Table) {
	t.Helper()
	conn := espPolNFT(t, lab)
	conn.DelTable(table)
	if err := conn.Flush(); err != nil {
		t.Fatalf("delete the drop-ESP table %s in Ze's namespace: %v", espPolFirewallTable, err)
	}
}

// TestRFC4301BareESPFirewallDropKeepsTheFastPathOff proves that the fast path never carries
// a peer's bare ESP past an nftables rule that drops ESP on the input path. A datagram the
// tc ingress program rewrote reaches every netfilter hook as UDP, so a `meta l4proto esp
// drop` rule would never see it; the fast path therefore shuts its gate while the kernel's
// ruleset holds such a rule, and the bare datagram stays ESP until the rule has judged it.
// It proves a firewall interaction, not an RFC requirement, so it carries no RFC tag; its
// file name follows the selector probe whose lab it shares.
//
// Method: the same lab, SPD and Child SA as TestRFC4301BareESPOnTemplatedSAIsPolicyChecked.
// The fast path first rewrites, its gate open (espPolFastPathAttached). Then, from outside
// Ze, as an operator's nft command would, the test commits an inet table whose input chain
// holds `meta l4proto esp counter drop`, and waits, bounded, for the fast path's watch to
// see the commit: FirewallBlocked with the site naming the table and the chain, and the gate
// shut. Row 1 sends one whole bare ESP datagram inside the selectors, which the fast path
// would rewrite while its gate is open: Rewritten4 must not move, the drop rule's counter,
// read back from the kernel, must move by exactly one, and nothing is delivered. The test
// then deletes the table and waits, bounded, for the gate to open again. Row 2 sends the
// same datagram: Rewritten4 moves by exactly one and the inner packet is delivered, so row
// 1's drop was the rule's and not a fault of the lab.
//
// MUTATION: force the gate open (writeGateLocked writing espFormFastPathGateOpen whatever
// it is given, or the program skipping its gate lookup) and row 1 goes red: the datagram is
// rewritten, passes the rule as UDP and is delivered, and the counter stays still.
//
//ze:guest-root
func TestRFC4301BareESPFirewallDropKeepsTheFastPathOff(t *testing.T) {
	lab := newEspPolLab(t)
	espPolSPD(t)
	child := espPolChild(t)
	sender := newEspPolSender(t, lab, child)
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)
	espPolFastPathAttached(t)
	whole := sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelPort))

	table, chain := espPolDropESP(t, lab)
	st := espPolFastPathUntil(t, "shut by the drop-ESP rule", func(st dataplane.ESPFormFastPathState) bool {
		return st.FirewallBlocked && !st.GateOpen
	})
	if !strings.Contains(st.FirewallSite, "table "+espPolFirewallTable) {
		t.Errorf("the fast path's firewall site %q does not name table %s", st.FirewallSite, espPolFirewallTable)
	}
	if !strings.Contains(st.FirewallSite, "chain "+espPolFirewallChain) {
		t.Errorf("the fast path's firewall site %q does not name chain %s", st.FirewallSite, espPolFirewallChain)
	}

	// Row 1: whole bare ESP, inside the selectors, under the drop rule.
	dropped := espPolDropped(t, lab, table, chain)
	rewritten := espPolFastPath(t).Rewritten4
	sender.sendBare(t, whole)
	if sadSelDelivered(t, inside) {
		t.Error("row 1 (whole bare ESP under `meta l4proto esp drop`): the inner packet was delivered, want it dropped by the nftables rule")
	}
	if moved := espPolFastPath(t).Rewritten4 - rewritten; moved != 0 {
		t.Errorf("row 1 (whole bare ESP under the drop rule): the fast path rewrote %d datagrams, want 0; a rewritten datagram passes the rule as UDP", moved)
	}
	if moved := espPolDropped(t, lab, table, chain) - dropped; moved != 1 {
		t.Errorf("row 1 (whole bare ESP under the drop rule): the rule's counter moved by %d, want 1; the rule did not judge the datagram", moved)
	}
	if t.Failed() {
		t.FailNow()
	}

	// Row 2: the table is gone, so the gate opens and the same datagram is rewritten.
	espPolRemoveFirewall(t, lab, table)
	espPolFastPathUntil(t, "open again after the drop-ESP table was deleted", func(st dataplane.ESPFormFastPathState) bool {
		return st.GateOpen && !st.FirewallBlocked
	})
	rewritten = espPolFastPath(t).Rewritten4
	sender.sendBare(t, whole)
	if !sadSelDelivered(t, inside) {
		t.Error("row 2 (whole bare ESP, no firewall): not delivered; row 1's drop cannot be told from a fault of the lab")
	}
	if moved := espPolFastPath(t).Rewritten4 - rewritten; moved != 1 {
		t.Errorf("row 2 (whole bare ESP, no firewall): the fast path rewrote %d datagrams, want 1; the gate did not open again", moved)
	}
}
