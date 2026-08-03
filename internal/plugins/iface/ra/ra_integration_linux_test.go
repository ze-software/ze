//go:build integration && linux

// Design: docs/features/interfaces.md -- Router Advertisement sender (Linux)
// Related: sender_linux.go -- the sender these tests drive
// Related: ifacera.go -- the RFC 4861 constants the timing assertions read
//
// These tests run the real sender against the real kernel in an ephemeral
// network namespace holding one veth pair. One end is the advertising router,
// the other end is the host. Nothing is faked: the advertisement leaves a raw
// ICMPv6 socket, crosses the veth, and is read back by a capture socket or
// consumed by the kernel's own SLAAC code.
//
// They need CAP_NET_ADMIN and CAP_NET_RAW, so they skip with a reason when the
// process is not root. Run them with `make ze-qemu-integration-test`.

package ifacera

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/net/ipv6"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/ndp"
	"github.com/ze-software/ze/internal/core/slogutil"

	// The sender resolves its interface through iface.Resolve, which needs an
	// active backend. This import registers the netlink one.
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
)

// ── netns + veth helpers ────────────────────────────────────────────────────

// raNSName builds a namespace name short enough for /var/run/netns.
func raNSName(testName string) string {
	name := strings.NewReplacer("/", "_", " ", "_", "(", "", ")", "").Replace(testName)
	if len(name) > 8 {
		name = name[len(name)-8:]
	}
	return "zra_" + name
}

// withRAVeth runs fn in a fresh network namespace holding one veth pair.
// router is the advertising end, host is the receiving end. Both ends are up,
// both have a Duplicate Address Detection-free link-local address, and the
// netlink backend is loaded.
//
// Each test passes its OWN device names. The iface resolver caches a binding
// per logical name and only a monitor event drops it, so two namespaces that
// reused one name would hand the second test the first test's ifindex.
func withRAVeth(t *testing.T, router, host string, fn func()) {
	t.Helper()

	if os.Geteuid() != 0 {
		t.Skip("requires root: the test creates a network namespace, a veth pair, and raw ICMPv6 sockets")
	}

	runtime.LockOSThread()
	unlocked := false
	unlock := func() {
		if !unlocked {
			runtime.UnlockOSThread()
			unlocked = true
		}
	}

	origNS, err := netns.Get()
	if err != nil {
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}

	nsName := raNSName(t.Name())
	newNS, err := netns.NewNamed(nsName)
	if err != nil {
		origNS.Close()
		unlock()
		t.Skipf("requires CAP_NET_ADMIN: %v", err)
	}

	t.Cleanup(func() {
		if restoreErr := netns.Set(origNS); restoreErr != nil {
			t.Errorf("restore namespace: %v", restoreErr)
		}
		origNS.Close()
		newNS.Close()
		netns.DeleteNamed(nsName) //nolint:errcheck // best-effort netns cleanup
		unlock()
	})

	if err := iface.EnsureBackend(); err != nil {
		t.Fatalf("load iface backend: %v", err)
	}

	if err := netlink.LinkAdd(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: router},
		PeerName:  host,
	}); err != nil {
		t.Fatalf("add veth %s/%s: %v", router, host, err)
	}

	for _, dev := range []string{router, host} {
		// Duplicate Address Detection holds a fresh link-local address
		// tentative for about one second, and the kernel will not send from a
		// tentative source. Nothing else is on this link, so the detection has
		// nothing to find and only costs the test a second on every link
		// transition.
		writeSysctl(t, dev, "accept_dad", "0")
		setLinkUp(t, dev)
	}
	for _, dev := range []string{router, host} {
		waitLinkLocal(t, dev, 5*time.Second)
	}

	fn()
}

// setLinkUp brings one device admin-up.
func setLinkUp(t *testing.T, dev string) {
	t.Helper()
	link, err := netlink.LinkByName(dev)
	if err != nil {
		t.Fatalf("link %q: %v", dev, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("up %q: %v", dev, err)
	}
}

// setLinkDown takes one device admin-down. The veth peer loses carrier with it.
func setLinkDown(t *testing.T, dev string) {
	t.Helper()
	link, err := netlink.LinkByName(dev)
	if err != nil {
		t.Fatalf("link %q: %v", dev, err)
	}
	if err := netlink.LinkSetDown(link); err != nil {
		t.Fatalf("down %q: %v", dev, err)
	}
}

// linkIndex returns the kernel ifindex of one device.
func linkIndex(t *testing.T, dev string) int {
	t.Helper()
	link, err := netlink.LinkByName(dev)
	if err != nil {
		t.Fatalf("link %q: %v", dev, err)
	}
	return link.Attrs().Index
}

// linkHardwareAddress returns the MAC of one device.
func linkHardwareAddress(t *testing.T, dev string) net.HardwareAddr {
	t.Helper()
	link, err := netlink.LinkByName(dev)
	if err != nil {
		t.Fatalf("link %q: %v", dev, err)
	}
	return link.Attrs().HardwareAddr
}

// writeSysctl sets one per-interface IPv6 sysctl. /proc/sys/net is per network
// namespace and resolves against the calling thread's namespace, and the
// caller is locked to the namespace thread, so this writes the test's
// namespace and never the host's.
func writeSysctl(t *testing.T, dev, key, value string) {
	t.Helper()
	path := "/proc/sys/net/ipv6/conf/" + dev + "/" + key
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatalf("write %s = %s: %v", path, value, err)
	}
}

// waitLinkLocal waits for a usable (not tentative) link-local address on dev.
// RFC 4861 Section 4.2 makes the link-local address the only legal source of a
// Router Advertisement, so a sender started before one exists sends from the
// unspecified address and every host drops it.
func waitLinkLocal(t *testing.T, dev string, timeout time.Duration) {
	t.Helper()
	if !waitFor(timeout, func() bool {
		link, err := netlink.LinkByName(dev)
		if err != nil {
			return false
		}
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
		if err != nil {
			return false
		}
		for _, a := range addrs {
			if a.IP.IsLinkLocalUnicast() && a.Flags&ifaFlagTentative == 0 {
				return true
			}
		}
		return false
	}) {
		t.Fatalf("no usable link-local address on %s within %s", dev, timeout)
	}
}

// ifaFlagTentative is the kernel IFA_F_TENTATIVE bit. Declared here so the test
// reads the flag without depending on the netlink backend's private copy.
const ifaFlagTentative = 0x40

// waitFor polls cond until it returns true or the timeout runs out.
func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// ── sender configuration ────────────────────────────────────────────────────

// testPrefix is the advertised prefix. Each test uses its own so a stray
// address from another test cannot satisfy an assertion.
func raSpecFor(dev string, prefix netip.Prefix, minimum, maximum time.Duration) iface.RASenderSpec {
	return iface.RASenderSpec{
		Interface: dev,
		Unit:      "0",
		Advertisement: ndp.RAConfig{
			CurHopLimit:    64,
			RouterLifetime: 1800,
			Prefixes: []ndp.PrefixInfo{{
				Prefix:            prefix,
				OnLink:            true,
				Autonomous:        true,
				ValidLifetime:     7200,
				PreferredLifetime: 3600,
			}},
		},
		MinimumInterval: minimum,
		MaximumInterval: maximum,
	}
}

// startSender starts the sender under test on dev.
func startSender(t *testing.T, spec iface.RASenderSpec) *Sender {
	t.Helper()
	s, err := NewSender(spec, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("NewSender on %s: %v", spec.Interface, err)
	}
	return s
}

// ── capture socket ──────────────────────────────────────────────────────────

// raCapture reads Router Advertisements arriving on one device.
type raCapture struct {
	conn net.PacketConn
	pc   *ipv6.PacketConn
}

// openRACapture opens a raw ICMPv6 socket bound to dev that accepts Router
// Advertisements only, and asks the kernel for the received Hop Limit.
//
// The socket needs no multicast join: every IPv6 interface is a member of
// ff02::1 (all-nodes), which is where an advertisement goes.
func openRACapture(t *testing.T, dev string) *raCapture {
	t.Helper()
	conn, err := (&net.ListenConfig{}).ListenPacket(context.Background(), "ip6:ipv6-icmp", "::")
	if err != nil {
		t.Skipf("requires CAP_NET_RAW: %v", err)
	}
	if err := bindToDevice(conn, dev); err != nil {
		conn.Close()
		t.Fatalf("bind capture socket to %s: %v", dev, err)
	}
	pc := ipv6.NewPacketConn(conn)
	if err := pc.SetControlMessage(ipv6.FlagHopLimit, true); err != nil {
		conn.Close()
		t.Fatalf("request received hop limit on %s: %v", dev, err)
	}
	var filter ipv6.ICMPFilter
	filter.SetAll(true)
	filter.Accept(ipv6.ICMPTypeRouterAdvertisement)
	if err := pc.SetICMPFilter(&filter); err != nil {
		conn.Close()
		t.Fatalf("set ICMP6_FILTER on %s: %v", dev, err)
	}
	c := &raCapture{conn: conn, pc: pc}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // best-effort socket cleanup
	return c
}

// capturedRA is one advertisement as it arrived on the wire.
type capturedRA struct {
	message  raMessage
	hopLimit int
	source   net.IP
	at       time.Time
}

// next returns the next advertisement, or an error when none arrives inside
// timeout.
func (c *raCapture) next(timeout time.Duration) (capturedRA, error) {
	buf := make([]byte, 1500)
	if err := c.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return capturedRA{}, err
	}
	n, cm, src, err := c.pc.ReadFrom(buf)
	if err != nil {
		return capturedRA{}, err
	}
	at := time.Now()
	msg, err := parseRA(buf[:n])
	if err != nil {
		return capturedRA{}, err
	}
	got := capturedRA{message: msg, at: at}
	if cm != nil {
		got.hopLimit = cm.HopLimit
	}
	if ip, ok := src.(*net.IPAddr); ok {
		got.source = ip.IP
	}
	return got, nil
}

// drain reads and discards everything already queued, so a later read reports
// only what arrives after this point.
func (c *raCapture) drain() {
	buf := make([]byte, 1500)
	for {
		if err := c.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
			return
		}
		if _, _, _, err := c.pc.ReadFrom(buf); err != nil {
			return
		}
	}
}

// isTimeout reports whether err is a read deadline expiry rather than a real
// failure.
func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// sendCraftedRA sends one bare 16-octet Router Advertisement out dev with the
// given IPv6 Hop Limit and no options. It is the control for the hop limit
// assertion: a capture that reported a constant would report 255 for this one
// too, so the value the sender is credited with is a property of the wire.
func sendCraftedRA(t *testing.T, dev string, hopLimit int) {
	t.Helper()
	conn, err := (&net.ListenConfig{}).ListenPacket(context.Background(), "ip6:ipv6-icmp", "::")
	if err != nil {
		t.Fatalf("open control socket: %v", err)
	}
	defer conn.Close() //nolint:errcheck // best-effort socket cleanup
	if err := bindToDevice(conn, dev); err != nil {
		t.Fatalf("bind control socket to %s: %v", dev, err)
	}
	pc := ipv6.NewPacketConn(conn)
	if err := pc.SetMulticastHopLimit(hopLimit); err != nil {
		t.Fatalf("set control hop limit %d: %v", hopLimit, err)
	}
	advertisement := make([]byte, 16)
	advertisement[0] = ndp.ICMPv6TypeRouterAdvertisement
	control := &ipv6.ControlMessage{IfIndex: linkIndex(t, dev), HopLimit: hopLimit}
	dst := &net.UDPAddr{IP: net.ParseIP(allNodesGroup), Zone: dev}
	if _, err := pc.WriteTo(advertisement, control, dst); err != nil {
		t.Fatalf("send control advertisement on %s: %v", dev, err)
	}
}

// sendRouterSolicitation sends one Router Solicitation out dev, as a host
// looking for a router does (RFC 4861 Section 6.3.7).
func sendRouterSolicitation(t *testing.T, dev string) {
	t.Helper()
	conn, err := (&net.ListenConfig{}).ListenPacket(context.Background(), "ip6:ipv6-icmp", "::")
	if err != nil {
		t.Fatalf("open solicitation socket: %v", err)
	}
	defer conn.Close() //nolint:errcheck // best-effort socket cleanup
	if err := bindToDevice(conn, dev); err != nil {
		t.Fatalf("bind solicitation socket to %s: %v", dev, err)
	}
	pc := ipv6.NewPacketConn(conn)
	if err := pc.SetMulticastHopLimit(advertisementHopLimit); err != nil {
		t.Fatalf("set solicitation hop limit: %v", err)
	}

	// RFC 4861 Section 4.1: type, code, checksum, then four reserved octets.
	// The kernel computes the checksum for a raw ICMPv6 socket.
	solicitation := make([]byte, 8)
	solicitation[0] = ndp.ICMPv6TypeRouterSolicitation

	control := &ipv6.ControlMessage{IfIndex: linkIndex(t, dev), HopLimit: advertisementHopLimit}
	dst := &net.UDPAddr{IP: net.ParseIP(allRoutersGroup), Zone: dev}
	if _, err := pc.WriteTo(solicitation, control, dst); err != nil {
		t.Fatalf("send Router Solicitation on %s: %v", dev, err)
	}
}

// ── independent decoder ─────────────────────────────────────────────────────
//
// The decoder is written from RFC 4861 rather than from ndp.BuildRA. A test
// that decoded with the encoder's own tables would agree with any layout the
// encoder produced, including a wrong one.

// raPrefixOption is one decoded Prefix Information option.
type raPrefixOption struct {
	prefix            netip.Prefix
	onLink            bool
	autonomous        bool
	validLifetime     uint32
	preferredLifetime uint32
}

// raMessage is one decoded Router Advertisement.
type raMessage struct {
	messageType    uint8
	code           uint8
	curHopLimit    uint8
	managed        bool
	otherConfig    bool
	routerLifetime uint16
	reachableTime  uint32
	retransTimer   uint32
	sourceLLA      net.HardwareAddr
	prefixes       []raPrefixOption
}

// parseRA decodes one Router Advertisement (RFC 4861 Section 4.2 and 4.6).
func parseRA(b []byte) (raMessage, error) {
	const headerLen = 16
	if len(b) < headerLen {
		return raMessage{}, fmt.Errorf("advertisement is %d octets, want at least %d", len(b), headerLen)
	}
	msg := raMessage{
		messageType:    b[0],
		code:           b[1],
		curHopLimit:    b[4],
		managed:        b[5]&0x80 != 0,
		otherConfig:    b[5]&0x40 != 0,
		routerLifetime: binary.BigEndian.Uint16(b[6:8]),
		reachableTime:  binary.BigEndian.Uint32(b[8:12]),
		retransTimer:   binary.BigEndian.Uint32(b[12:16]),
	}

	for off := headerLen; off+2 <= len(b); {
		optionType := b[off]
		optionLen := int(b[off+1]) * 8
		if optionLen == 0 {
			return raMessage{}, fmt.Errorf("option type %d at offset %d has length 0", optionType, off)
		}
		if off+optionLen > len(b) {
			return raMessage{}, fmt.Errorf("option type %d at offset %d runs past the message", optionType, off)
		}
		switch optionType {
		case ndp.OptSourceLinkLayerAddress:
			msg.sourceLLA = net.HardwareAddr(append([]byte(nil), b[off+2:off+optionLen]...))
		case ndp.OptPrefixInformation:
			if optionLen != 32 {
				return raMessage{}, fmt.Errorf("prefix option length %d, want 32", optionLen)
			}
			addr, ok := netip.AddrFromSlice(b[off+16 : off+32])
			if !ok {
				return raMessage{}, errors.New("prefix option carries no address")
			}
			prefix, err := addr.Prefix(int(b[off+2]))
			if err != nil {
				return raMessage{}, fmt.Errorf("prefix option: %w", err)
			}
			msg.prefixes = append(msg.prefixes, raPrefixOption{
				prefix:            prefix,
				onLink:            b[off+3]&0x80 != 0,
				autonomous:        b[off+3]&0x40 != 0,
				validLifetime:     binary.BigEndian.Uint32(b[off+4 : off+8]),
				preferredLifetime: binary.BigEndian.Uint32(b[off+8 : off+12]),
			})
		}
		off += optionLen
	}
	return msg, nil
}

// ── tests ───────────────────────────────────────────────────────────────────

// TestRASenderPeerAutoconfigures runs the sender on one end of a veth pair and
// lets the kernel on the other end do stateless address autoconfiguration.
//
// VALIDATES: the sender's advertisement is good enough for a real IPv6 host to
// act on. The peer forms a global address inside the advertised prefix and the
// kernel marks it autoconfigured, which RFC 4862 Section 5.5.3 requires of a
// Prefix Information option carrying the A flag.
// PREVENTS: an advertisement that decodes correctly in a unit test but that no
// host acts on, because its hop limit, its source address, or its prefix flags
// make the kernel discard it.
func TestRASenderPeerAutoconfigures(t *testing.T) {
	const router, host = "zra0", "zra1"
	withRAVeth(t, router, host, func() {
		// accept_ra 2 accepts advertisements even when the namespace forwards,
		// so the test does not depend on the forwarding default.
		writeSysctl(t, host, "accept_ra", "2")
		writeSysctl(t, host, "autoconf", "1")
		writeSysctl(t, host, "accept_ra_pinfo", "1")
		// Privacy addresses would add a second, temporary address whose origin
		// is "temporary". The test asserts on the autoconfigured one.
		writeSysctl(t, host, "use_tempaddr", "0")

		prefix := netip.MustParsePrefix("2001:db8:a01::/64")
		sender := startSender(t, raSpecFor(router, prefix, 200*time.Millisecond, 400*time.Millisecond))
		defer sender.Stop()

		var formed iface.AddrInfo
		found := waitFor(15*time.Second, func() bool {
			info, err := iface.GetInterface(host)
			if err != nil {
				return false
			}
			for _, a := range info.Addresses {
				addr, parseErr := netip.ParseAddr(a.Address)
				if parseErr != nil || !prefix.Contains(addr) {
					continue
				}
				formed = a
				return true
			}
			return false
		})
		if !found {
			t.Fatalf("%s formed no address inside %s within 15s: the peer never acted on the advertisement", host, prefix)
		}
		if formed.Origin != "slaac" {
			t.Errorf("address %s origin = %q, want %q: the kernel did not record it as autoconfigured",
				formed.Address, formed.Origin, "slaac")
		}
		if formed.PrefixLength != prefix.Bits() {
			t.Errorf("address %s prefix length = %d, want %d", formed.Address, formed.PrefixLength, prefix.Bits())
		}
		if formed.ValidLifetime == 0 {
			t.Errorf("address %s valid lifetime = 0, want the advertised lifetime", formed.Address)
		}
	})
}

// TestRASenderWireFormat reads one advertisement off the wire and checks every
// field a receiver depends on.
//
// VALIDATES: RFC 4861 Section 4.2 (Hop Limit 255, type 134, code 0, a
// link-local source), Section 4.6.1 (the Source Link-layer Address option
// carries the sending interface's MAC), and Section 4.6.2 (the Prefix
// Information option carries the advertised prefix with the L and A flags).
// PREVENTS: the sender falling back to the kernel default multicast hop limit
// of 1, which Section 6.1.2 makes every conforming host discard, and prevents a
// silently dropped or malformed option.
func TestRASenderWireFormat(t *testing.T) {
	const router, host = "zrb0", "zrb1"
	withRAVeth(t, router, host, func() {
		capture := openRACapture(t, host)
		routerMAC := linkHardwareAddress(t, router)

		prefix := netip.MustParsePrefix("2001:db8:a02::/64")
		sender := startSender(t, raSpecFor(router, prefix, 200*time.Millisecond, 400*time.Millisecond))
		defer sender.Stop()

		got, err := capture.next(10 * time.Second)
		if err != nil {
			t.Fatalf("no advertisement on %s within 10s: %v", host, err)
		}

		// RFC 4861 Section 4.2. The literal is deliberate: reading the
		// sender's own constant would agree with any value it chose.
		if got.hopLimit != 255 {
			t.Errorf("received Hop Limit = %d, want 255 (RFC 4861 Section 4.2)", got.hopLimit)
		}
		if got.source == nil || !got.source.IsLinkLocalUnicast() {
			t.Errorf("source address = %v, want a link-local address (RFC 4861 Section 4.2)", got.source)
		}
		if got.message.messageType != ndp.ICMPv6TypeRouterAdvertisement {
			t.Errorf("ICMPv6 type = %d, want %d", got.message.messageType, ndp.ICMPv6TypeRouterAdvertisement)
		}
		if got.message.code != 0 {
			t.Errorf("ICMPv6 code = %d, want 0", got.message.code)
		}
		if got.message.curHopLimit != 64 {
			t.Errorf("Cur Hop Limit = %d, want 64", got.message.curHopLimit)
		}
		if got.message.routerLifetime != 1800 {
			t.Errorf("Router Lifetime = %d, want 1800", got.message.routerLifetime)
		}

		if got.message.sourceLLA.String() != routerMAC.String() {
			t.Errorf("Source Link-layer Address option = %v, want the %s MAC %v (RFC 4861 Section 4.6.1)",
				got.message.sourceLLA, router, routerMAC)
		}

		if len(got.message.prefixes) != 1 {
			t.Fatalf("Prefix Information options = %d, want 1", len(got.message.prefixes))
		}
		pio := got.message.prefixes[0]
		if pio.prefix != prefix {
			t.Errorf("advertised prefix = %s, want %s", pio.prefix, prefix)
		}
		if !pio.onLink {
			t.Error("Prefix Information L flag is clear, want set (RFC 4861 Section 4.6.2)")
		}
		if !pio.autonomous {
			t.Error("Prefix Information A flag is clear, want set (RFC 4861 Section 4.6.2)")
		}
		if pio.validLifetime != 7200 {
			t.Errorf("Valid Lifetime = %d, want 7200", pio.validLifetime)
		}
		if pio.preferredLifetime != 3600 {
			t.Errorf("Preferred Lifetime = %d, want 3600", pio.preferredLifetime)
		}

		// The control. Stop the sender, clear its final advertisements, then
		// put one hand-built advertisement with Hop Limit 1 on the same wire.
		// The capture must report 1. Without this, a capture that always
		// reported 255 would credit the sender with a value it never set.
		sender.Stop()
		capture.drain()
		sendCraftedRA(t, router, 1)
		control, err := capture.next(5 * time.Second)
		if err != nil {
			t.Fatalf("the control advertisement did not arrive: %v", err)
		}
		if len(control.message.prefixes) != 0 {
			t.Fatalf("the control read a sender advertisement, not the crafted one")
		}
		if control.hopLimit != 1 {
			t.Errorf("control Hop Limit = %d, want 1: the capture does not read the wire, so the 255 above proves nothing",
				control.hopLimit)
		}
	})
}

// TestRASolicitedResponse sends a Router Solicitation from the host end and
// waits for the advertisement that answers it.
//
// VALIDATES: RFC 4861 Section 6.2.6. A router answers a solicitation with an
// advertisement delayed by 0 to MAX_RA_DELAY_TIME, and consecutive multicast
// advertisements stay at least MIN_DELAY_BETWEEN_RAS apart.
// PREVENTS: a router that ignores solicitations, leaving a joining host to wait
// for the next unsolicited advertisement, and prevents a router that answers
// at once and lets a solicitation flood become an advertisement flood.
func TestRASolicitedResponse(t *testing.T) {
	const router, host = "zrc0", "zrc1"
	withRAVeth(t, router, host, func() {
		capture := openRACapture(t, host)

		// The unsolicited interval is far longer than the test, so any second
		// advertisement is an answer to the solicitation and nothing else.
		const unsolicited = 10 * time.Minute
		prefix := netip.MustParsePrefix("2001:db8:a03::/64")
		sender := startSender(t, raSpecFor(router, prefix, unsolicited, unsolicited))
		defer sender.Stop()

		first, err := capture.next(10 * time.Second)
		if err != nil {
			t.Fatalf("no initial advertisement on %s within 10s: %v", host, err)
		}

		sendRouterSolicitation(t, host)

		answer, err := capture.next(minDelayBetweenRAs + maxRADelayTime + 5*time.Second)
		if err != nil {
			t.Fatalf("no advertisement answered the Router Solicitation: %v", err)
		}
		if answer.message.messageType != ndp.ICMPv6TypeRouterAdvertisement {
			t.Fatalf("answer ICMPv6 type = %d, want %d", answer.message.messageType, ndp.ICMPv6TypeRouterAdvertisement)
		}
		if len(answer.message.prefixes) != 1 || answer.message.prefixes[0].prefix != prefix {
			t.Errorf("answer carries prefixes %v, want %s", answer.message.prefixes, prefix)
		}

		// The rate limit is a floor, so assert the gap is not below it. The
		// timestamps are the arrival of each message, which is never earlier
		// than its send, so a measured gap above the floor proves a real gap
		// above the floor. A tolerance covers the scheduling cost of the two
		// reads, and no upper instant is asserted.
		const readTolerance = 200 * time.Millisecond
		if gap := answer.at.Sub(first.at); gap+readTolerance < minDelayBetweenRAs {
			t.Errorf("answer arrived %s after the previous advertisement, want at least %s (RFC 4861 Section 6.2.6)",
				gap, minDelayBetweenRAs)
		}
	})
}

// TestRAFinalZeroLifetimeOnWire stops the sender and reads the advertisement
// that retires the router.
//
// The unit test of the same behavior is TestRAFinalZeroLifetime
// (sender_linux_test.go), which drives sendFinal directly. This one proves the
// message reaches the wire, so the two names differ and both build under
// `-tags integration`.
//
// VALIDATES: RFC 4861 Section 6.2.5. A router that stops advertising sends a
// final advertisement with Router Lifetime 0, so hosts drop it from their
// default router list at once.
// PREVENTS: a stopped Ze staying a default router on every host of the link for
// the whole advertised router lifetime, which black-holes their traffic.
func TestRAFinalZeroLifetimeOnWire(t *testing.T) {
	const router, host = "zrd0", "zrd1"
	withRAVeth(t, router, host, func() {
		capture := openRACapture(t, host)

		// Long enough that no unsolicited advertisement can be confused with
		// the final one.
		const unsolicited = 10 * time.Minute
		prefix := netip.MustParsePrefix("2001:db8:a04::/64")
		sender := startSender(t, raSpecFor(router, prefix, unsolicited, unsolicited))

		first, err := capture.next(10 * time.Second)
		if err != nil {
			t.Fatalf("no initial advertisement on %s within 10s: %v", host, err)
		}
		if first.message.routerLifetime == 0 {
			t.Fatalf("the initial advertisement already carries Router Lifetime 0, so the final one proves nothing")
		}

		sender.Stop()

		final, err := capture.next(5 * time.Second)
		if err != nil {
			t.Fatalf("no final advertisement after Stop within 5s: %v", err)
		}
		if final.message.routerLifetime != 0 {
			t.Errorf("final Router Lifetime = %d, want 0 (RFC 4861 Section 6.2.5)", final.message.routerLifetime)
		}
		// Section 4.2 scopes the zero to the router's use as a default router,
		// so the rest of the message must stay as configured.
		if len(final.message.prefixes) != 1 || final.message.prefixes[0].prefix != prefix {
			t.Errorf("final advertisement carries prefixes %v, want %s", final.message.prefixes, prefix)
		}
		if final.hopLimit != 255 {
			t.Errorf("final advertisement Hop Limit = %d, want 255", final.hopLimit)
		}
	})
}

// TestRALinkDownUp takes the advertising link down and brings it back.
//
// VALIDATES: no advertisement reaches the wire while the link is down, the
// sender survives the flap and advertises again on its own when the link
// returns (RFC 4861 Section 6.2.4), and Stop leaves no goroutine behind.
// PREVENTS: a sender that dies on the send errors a down link returns, that
// loses its multicast group join or its device binding across a flap, and a
// sender whose solicitation reader outlives its socket, which leaks one
// goroutine per interface on every config apply.
//
// The quiet window is enforced by the kernel as well as by the sender: a down
// link carries nothing whatever the sender does. The sender's own pause
// (onLinkEvent) reads iface link events, which only the interface component's
// event bus delivers, so this test does not reach that path.
func TestRALinkDownUp(t *testing.T) {
	const router, host = "zre0", "zre1"
	withRAVeth(t, router, host, func() {
		capture := openRACapture(t, host)

		before := runtime.NumGoroutine()

		prefix := netip.MustParsePrefix("2001:db8:a05::/64")
		sender := startSender(t, raSpecFor(router, prefix, 300*time.Millisecond, 600*time.Millisecond))

		for i := range 2 {
			if _, err := capture.next(10 * time.Second); err != nil {
				t.Fatalf("advertisement %d did not arrive on the up link: %v", i+1, err)
			}
		}

		setLinkDown(t, router)
		// One maximum interval covers the advertisement that was already in
		// flight when the link went down.
		time.Sleep(700 * time.Millisecond)
		capture.drain()

		// Four maximum intervals with nothing on the wire. A sender that kept
		// advertising would deliver six in this window.
		if got, err := capture.next(2500 * time.Millisecond); err == nil {
			t.Errorf("an advertisement arrived while %s was down: Router Lifetime %d",
				router, got.message.routerLifetime)
		} else if !isTimeout(err) {
			t.Fatalf("read while the link was down: %v", err)
		}

		setLinkUp(t, router)
		waitLinkLocal(t, router, 10*time.Second)
		if _, err := capture.next(15 * time.Second); err != nil {
			t.Errorf("no advertisement resumed within 15s of %s coming up: %v", router, err)
		}

		sender.Stop()

		// Stop waits for the send loop, and closing the socket ends the
		// solicitation reader a moment later, so the count is polled rather
		// than read once.
		if !waitFor(5*time.Second, func() bool { return runtime.NumGoroutine() <= before }) {
			t.Errorf("goroutines after Stop = %d, want no more than %d before the sender started",
				runtime.NumGoroutine(), before)
		}
	})
}
