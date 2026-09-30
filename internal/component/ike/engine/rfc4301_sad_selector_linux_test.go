// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- Child SA install into the dataplane
// Related: rfc4301_sad_selector_test.go -- the same selectors read off the SAParams and SPParams Ze builds
//
// VALIDATES: against a real Linux XFRM stack, that a Child SA installed by createFirstChildSA
// accepts a decrypted packet inside the negotiated selectors and drops one outside them, in
// tunnel mode (the inbound require-policy drops it, XfrmInNoPols) and in transport mode (the
// inbound state's own selector drops it, XfrmInStateMismatch).
// PREVENTS: a transport-mode Child SA whose SAD entry carries no selector. A transport-only
// secpath passes the kernel's inbound policy check, so without the state selector a peer
// could send any port or protocol over the SA and have it delivered.
//
// The probe needs no privilege. It re-executes itself in a new user and network namespace,
// where it is root over a namespace of its own, and it skips only when the kernel refuses
// that namespace. The build tag is bare linux, not integration, because the units carry an
// RFC requirement tag and `./le rfc discriminate-record` runs them with no build tag
// (docs/architecture/testing/qemu-integration.md, "Build Tags").
//
// The instrument is the pair of a UDP listener and /proc/net/xfrm_stat. The ESP datagrams
// are built here with the key read back from the installed kernel state, so the kernel
// decrypts them exactly as it would decrypt the peer's.

//go:build linux

package engine

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	sadSelChildEnv = "ZE_RFC4301_SAD_SELECTOR_PROBE"

	sadSelLocalOuter  = "10.250.0.1"
	sadSelRemoteOuter = "10.250.0.2"
	sadSelInnerLocal  = "10.2.0.1"
	sadSelInnerRemote = "10.1.0.5"
	sadSelInnerStray  = "10.9.0.5"
	sadSelPort        = 5000
	sadSelStrayPort   = 6000
	sadSelSourcePort  = 40000

	sadSelStatNoPols        = "XfrmInNoPols"
	sadSelStatStateMismatch = "XfrmInStateMismatch"

	sadSelProtoUDP  = 17
	sadSelProtoICMP = 1
	sadSelProtoIPv4 = 4
)

// sadSelOwnNamespace runs the calling test again in a new user and network namespace, and
// returns true only in that child, where the body runs.
func sadSelOwnNamespace(t *testing.T) bool {
	t.Helper()
	if os.Getenv(sadSelChildEnv) == "1" {
		return true
	}
	args := []string{"-test.run", "^" + t.Name() + "$", "-test.v", "-test.count=1"}
	// A coverage run hands the binary a counter directory, and the parent's profile is
	// built from every counter file in it. Passing it on lets the code the child runs
	// appear in that profile, which is what `./le rfc discriminate-record` reads.
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-test.gocoverdir=") {
			args = append(args, arg)
		}
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], args...)
	cmd.Env = append(os.Environ(), sadSelChildEnv+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	}
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Skipf("the kernel refused a user and network namespace for the probe: %v", err)
	}
	if err != nil {
		t.Fatalf("probe in its own namespace failed: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "--- SKIP") {
		t.Skipf("probe skipped:\n%s", out)
	}
	t.Logf("probe output:\n%s", out)
	return false
}

// sadSelNetns brings loopback up and puts every address the probe uses on it.
func sadSelNetns(t *testing.T) {
	t.Helper()
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatalf("lo up: %v", err)
	}
	// Linux creates loopback with disable_policy set, so a packet received on lo skips
	// the inbound policy check entirely (DST_NOPOLICY on its input route). Every other
	// interface defaults to 0, which is the setting a peer's packets meet, so the probe
	// clears it here. Left set, every drop below would read as a delivery.
	if err := os.WriteFile("/proc/sys/net/ipv4/conf/lo/disable_policy", []byte("0"), 0o600); err != nil {
		t.Fatalf("clear lo disable_policy: %v", err)
	}
	for _, a := range []string{sadSelLocalOuter, sadSelRemoteOuter, sadSelInnerLocal} {
		addr, err := netlink.ParseAddr(a + "/32")
		if err != nil {
			t.Fatalf("parse %s: %v", a, err)
		}
		if err := netlink.AddrAdd(lo, addr); err != nil {
			t.Fatalf("add %s to lo: %v", a, err)
		}
	}
}

// sadSelStat reads one counter out of the namespace's own /proc/net/xfrm_stat.
func sadSelStat(t *testing.T, name string) int {
	t.Helper()
	raw, err := os.ReadFile("/proc/net/xfrm_stat")
	if err != nil {
		t.Fatalf("read xfrm_stat: %v", err)
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
	t.Fatalf("%s absent from /proc/net/xfrm_stat", name)
	return 0
}

// sadSelChild installs one Child SA through createFirstChildSA over the real XFRM backend.
// The negotiated pair is UDP, local port 5000, between tsLocal and tsRemote.
func sadSelChild(t *testing.T, transportMode bool, tsLocal, tsRemote string) *ChildSA {
	t.Helper()
	if err := dataplane.Load("xfrm"); err != nil {
		t.Fatalf("load xfrm backend: %v", err)
	}
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Errorf("close xfrm backend: %v", err)
		}
	})

	sa := testSA()
	sa.IsInitiator = true
	sa.UseTransportMode = transportMode
	tsi := mustCIDRNet(t, tsLocal)
	tsr := mustCIDRNet(t, tsRemote)
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
	child, err := createFirstChildSA(sa, group, sadSelLocalOuter, sadSelRemoteOuter, 0, dataplane.Get(), slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	if !child.ESPInstalled {
		t.Fatal("the Child SA was not installed into XFRM")
	}
	t.Cleanup(child.Clear)
	return child
}

func mustCIDRNet(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return n
}

// sadSelSender sends ESP datagrams for the inbound SA of one Child SA, keyed with the
// key the kernel state holds.
type sadSelSender struct {
	spi  uint32
	seq  uint32
	aead cipher.AEAD
	salt []byte
	conn *net.IPConn
}

func newSadSelSender(t *testing.T, child *ChildSA) *sadSelSender {
	t.Helper()
	state, err := netlink.XfrmStateGet(&netlink.XfrmState{
		Dst: net.ParseIP(sadSelLocalOuter), Spi: int(child.InboundSPI), Proto: netlink.XFRM_PROTO_ESP,
	})
	if err != nil {
		t.Fatalf("read back the inbound state: %v", err)
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
	conn, err := net.ListenIP("ip4:50", &net.IPAddr{IP: net.ParseIP(sadSelRemoteOuter)})
	if err != nil {
		t.Fatalf("raw ESP socket: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close raw ESP socket: %v", err)
		}
	})
	return &sadSelSender{spi: child.InboundSPI, aead: aead, salt: key[32:], conn: conn}
}

// send wraps payload in one ESP packet (RFC 4303 with RFC 4106 AES-GCM) and sends it.
func (s *sadSelSender) send(t *testing.T, nextHeader byte, payload []byte) {
	t.Helper()
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
	packet := append(append(header, iv...), s.aead.Seal(nil, nonce, plain, header)...)
	if _, err := s.conn.WriteToIP(packet, &net.IPAddr{IP: net.ParseIP(sadSelLocalOuter)}); err != nil {
		t.Fatalf("send ESP: %v", err)
	}
}

// sadSelUDP returns a UDP segment with no checksum, which IPv4 permits.
func sadSelUDP(targetPort uint16) []byte {
	body := []byte("ze-rfc4301-sad-selector")
	seg := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint16(seg[0:2], sadSelSourcePort)
	binary.BigEndian.PutUint16(seg[2:4], targetPort)
	binary.BigEndian.PutUint16(seg[4:6], uint16(8+len(body)))
	return append(seg, body...)
}

// sadSelICMPEcho returns an ICMP echo request.
func sadSelICMPEcho() []byte {
	msg := []byte{8, 0, 0, 0, 0, 1, 0, 1}
	binary.BigEndian.PutUint16(msg[2:4], sadSelChecksum(msg))
	return msg
}

// sadSelIPv4 returns an IPv4 packet from source to the local inner address carrying
// payload, for the inner header of tunnel mode.
func sadSelIPv4(source string, proto byte, payload []byte) []byte {
	hdr := make([]byte, 20, 20+len(payload))
	hdr[0] = 0x45
	binary.BigEndian.PutUint16(hdr[2:4], uint16(20+len(payload)))
	hdr[8] = 64
	hdr[9] = proto
	copy(hdr[12:16], net.ParseIP(source).To4())
	copy(hdr[16:20], net.ParseIP(sadSelInnerLocal).To4())
	binary.BigEndian.PutUint16(hdr[10:12], sadSelChecksum(hdr))
	return append(hdr, payload...)
}

func sadSelChecksum(b []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(b[i : i+2]))
	}
	for sum > 0xffff {
		sum = sum&0xffff + sum>>16
	}
	return ^uint16(sum)
}

// sadSelListen binds a UDP listener the probe reads delivery from.
func sadSelListen(t *testing.T, address string, port int) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(address), Port: port})
	if err != nil {
		t.Fatalf("listen %s:%d: %v", address, port, err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Errorf("close listener: %v", err)
		}
	})
	return conn
}

// sadSelDelivered reports whether one datagram reached conn within a short wait.
func sadSelDelivered(t *testing.T, conn *net.UDPConn) bool {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 256)
	_, _, err := conn.ReadFromUDP(buf)
	return err == nil
}

// TestRFC4301TunnelModeInboundSelectorsAreEnforcedByTheRequirePolicy proves the tunnel-mode
// half of RFC 4301 Section 4.4.2 at the stack: the inbound SAD entry's selectors are the
// negotiated ones, as enforced for every decapsulated packet. Method: a tunnel-mode Child SA
// negotiated for UDP from 10.1.0.0/16 to local port 5000 of 10.2.0.0/16; then one inner
// packet inside the selectors (delivered), and one each outside them by source address, by
// port and by protocol (each dropped as XfrmInNoPols, none delivered).
func TestRFC4301TunnelModeInboundSelectorsAreEnforcedByTheRequirePolicy(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2-1 positive -- a tunnel-mode inbound packet whose inner
	// source, protocol and destination port are the negotiated ones is decrypted and
	// delivered, through the inbound policy createFirstChildSA installs.
	// RFC requirement: RFC4301-4.4.2-1 negative -- a tunnel-mode inbound packet outside the
	// negotiated selectors by inner source address, destination port or protocol is dropped
	// by the kernel (XfrmInNoPols) and never delivered.
	if !sadSelOwnNamespace(t) {
		return
	}
	sadSelNetns(t)
	child := sadSelChild(t, false, "10.2.0.0/16", "10.1.0.0/16")
	sender := newSadSelSender(t, child)
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)
	stray := sadSelListen(t, sadSelInnerLocal, sadSelStrayPort)

	sender.send(t, sadSelProtoIPv4, sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelPort)))
	if !sadSelDelivered(t, inside) {
		t.Fatal("an inner packet inside the negotiated selectors was not delivered; the probe cannot judge the drops below")
	}

	cases := []struct {
		name     string
		packet   []byte
		listener *net.UDPConn
	}{
		{"source outside TSr", sadSelIPv4(sadSelInnerStray, sadSelProtoUDP, sadSelUDP(sadSelPort)), inside},
		{"port outside TSi", sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelStrayPort)), stray},
		{"protocol outside the pair", sadSelIPv4(sadSelInnerRemote, sadSelProtoICMP, sadSelICMPEcho()), nil},
	}
	for _, tc := range cases {
		before := sadSelStat(t, sadSelStatNoPols)
		sender.send(t, sadSelProtoIPv4, tc.packet)
		if tc.listener != nil && sadSelDelivered(t, tc.listener) {
			t.Errorf("%s: the packet was delivered", tc.name)
		}
		if tc.listener == nil {
			time.Sleep(200 * time.Millisecond)
		}
		if got := sadSelStat(t, sadSelStatNoPols) - before; got != 1 {
			t.Errorf("%s: %s moved by %d, want 1 (dropped by the inbound require-policy)", tc.name, sadSelStatNoPols, got)
		}
	}
}

// TestRFC4301TransportModeInboundSADEntryDropsPacketsOutsideItsSelectors proves the
// transport-mode half: the inbound SAD entry itself carries the negotiated selectors.
// Method: a transport-mode Child SA negotiated for UDP from the peer to local port 5000; one
// packet to port 5000 (delivered), then one to port 6000 and one ICMP echo (each dropped as
// XfrmInStateMismatch). Without the state selector both would pass the policy check, because
// a transport-only secpath matching no inbound policy is accepted.
func TestRFC4301TransportModeInboundSADEntryDropsPacketsOutsideItsSelectors(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2-1 positive -- a transport-mode inbound packet on the
	// negotiated protocol and destination port is decrypted and delivered.
	// RFC requirement: RFC4301-4.4.2-1 negative -- a transport-mode inbound packet on another
	// destination port or another protocol is dropped by the inbound SAD entry's selector
	// (XfrmInStateMismatch) and never delivered.
	if !sadSelOwnNamespace(t) {
		return
	}
	sadSelNetns(t)
	child := sadSelChild(t, true, sadSelLocalOuter+"/32", sadSelRemoteOuter+"/32")
	sender := newSadSelSender(t, child)
	inside := sadSelListen(t, sadSelLocalOuter, sadSelPort)
	stray := sadSelListen(t, sadSelLocalOuter, sadSelStrayPort)

	sender.send(t, sadSelProtoUDP, sadSelUDP(sadSelPort))
	if !sadSelDelivered(t, inside) {
		t.Fatal("a packet inside the negotiated selectors was not delivered; the probe cannot judge the drops below")
	}

	before := sadSelStat(t, sadSelStatStateMismatch)
	sender.send(t, sadSelProtoUDP, sadSelUDP(sadSelStrayPort))
	if sadSelDelivered(t, stray) {
		t.Error("a packet to a port outside the negotiated selector was delivered")
	}
	if got := sadSelStat(t, sadSelStatStateMismatch) - before; got != 1 {
		t.Errorf("port outside: %s moved by %d, want 1", sadSelStatStateMismatch, got)
	}

	before = sadSelStat(t, sadSelStatStateMismatch)
	sender.send(t, sadSelProtoICMP, sadSelICMPEcho())
	time.Sleep(200 * time.Millisecond)
	if got := sadSelStat(t, sadSelStatStateMismatch) - before; got != 1 {
		t.Errorf("protocol outside: %s moved by %d, want 1", sadSelStatStateMismatch, got)
	}
}
