// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Related: dhcp_shortframe_integration_linux_test.go -- the veth lab and frame helpers
//
// VALIDATES: the DHCPv4 lease loop on a real kernel. The client binds a short
// lease over a veth pair, and the lab plays the server through RENEWING,
// REBINDING, DHCPNAK and expiry, reading every frame the client sends.
// PREVENTS: a client that drops its lease every period (renews once, then
// rebinds, expires and rediscovers), broadcasts its RENEW, misses a broadcast
// DHCPNAK while renewing, drops its client identifier on renewal, or keeps a
// lease past its expiry.

//go:build linux

package ifacedhcp

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/mdlayher/packet"
	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/userns"
)

const (
	// lmLease, lmT1 and lmT2 are the lease every lab grants: RENEWING starts
	// two seconds after the REQUEST, REBINDING four, expiry six.
	lmLease = 6 * time.Second
	lmT1    = 2 * time.Second
	lmT2    = 4 * time.Second
	// lmRouteMetric keeps the lease's default route clear of the one the
	// guest boots with.
	lmRouteMetric = 4242
	// lmRenewals is how many RENEW/ACK exchanges the hold test runs: four
	// of them span more than one lease.
	lmRenewals = 4
)

var (
	// lmServerB answers a REBIND in place of sfServerIP.
	lmServerB = net.IP{192, 0, 2, 2}
	// lmRenumbered is the address a renewal ACK moves the client to.
	lmRenumbered = net.IP{192, 0, 2, 51}
)

// lmFrame is one DHCP message the client sent, with its IP and UDP addressing.
type lmFrame struct {
	msg    *dhcpv4.DHCPv4
	source *net.UDPAddr
	target *net.UDPAddr
	read   time.Time
}

// lmLab is a veth pair with a running client on one end and a raw socket on
// the other.
type lmLab struct {
	t      *testing.T
	zeSide string
	conn   *packet.Conn
	zeMAC  net.HardwareAddr
	client *DHCPClient
}

// lmStart builds the lab and starts the client with cfg.
func lmStart(t *testing.T, cfg dHCPConfig) *lmLab {
	t.Helper()

	zeSide, conn := sfSetupLab(t)
	zeIfc, err := net.InterfaceByName(zeSide)
	if err != nil {
		t.Fatalf("InterfaceByName(%s): %v", zeSide, err)
	}
	cfg.RouteMetric = lmRouteMetric
	client, err := newDHCPClient(zeSide, "0", &recordingBus{}, true, false, cfg, clock.RealClock{})
	if err != nil {
		t.Fatalf("newDHCPClient(%s): %v", zeSide, err)
	}
	if err := client.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(client.Stop)
	return &lmLab{t: t, zeSide: zeSide, conn: conn, zeMAC: zeIfc.HardwareAddr, client: client}
}

// read returns the next DHCP message the client sends, of any type.
func (l *lmLab) read(within time.Duration) lmFrame {
	l.t.Helper()

	deadline := time.Now().Add(within)
	if err := l.conn.SetReadDeadline(deadline); err != nil {
		l.t.Fatalf("SetReadDeadline: %v", err)
	}
	buf := make([]byte, 2048)
	for {
		n, _, err := l.conn.ReadFrom(buf)
		if err != nil {
			l.t.Fatalf("no DHCP message from the client within %s: %v", within, err)
		}
		frame, ok := lmParse(buf[:n])
		if !ok {
			continue
		}
		frame.read = time.Now()
		return frame
	}
}

// readRequest returns the next DHCPREQUEST and fails on a DHCPDISCOVER: the
// client that rediscovers has dropped the lease the test holds.
func (l *lmLab) readRequest(within time.Duration) lmFrame {
	l.t.Helper()

	frame := l.read(within)
	if frame.msg.MessageType() != dhcpv4.MessageTypeRequest {
		l.t.Fatalf("client sent %s, want a DHCPREQUEST extending the lease", frame.msg.MessageType())
	}
	return frame
}

// bind runs DORA with the lab as sfServerIP and returns the REQUEST the ACK
// answers. extra is added to the OFFER and the ACK.
func (l *lmLab) bind(extra ...dhcpv4.Modifier) lmFrame {
	l.t.Helper()

	discover := l.read(10 * time.Second)
	if discover.msg.MessageType() != dhcpv4.MessageTypeDiscover {
		l.t.Fatalf("first message %s, want DHCPDISCOVER", discover.msg.MessageType())
	}
	l.broadcast(sfServerIP, l.reply(discover.msg, dhcpv4.MessageTypeOffer, sfServerIP, sfClientIP, extra...))
	request := l.readRequest(10 * time.Second)
	l.broadcast(sfServerIP, l.reply(request.msg, dhcpv4.MessageTypeAck, sfServerIP, sfClientIP, extra...))
	l.awaitAddress(sfClientIP, true, 5*time.Second)
	return request
}

// reply builds a server answer to request granting the lab's lease.
func (l *lmLab) reply(request *dhcpv4.DHCPv4, kind dhcpv4.MessageType, server, yiaddr net.IP, extra ...dhcpv4.Modifier) *dhcpv4.DHCPv4 {
	l.t.Helper()

	mods := []dhcpv4.Modifier{
		dhcpv4.WithMessageType(kind),
		dhcpv4.WithServerIP(server),
		dhcpv4.WithYourIP(yiaddr),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(server)),
		dhcpv4.WithOption(dhcpv4.OptSubnetMask(net.CIDRMask(24, 32))),
		dhcpv4.WithOption(dhcpv4.OptIPAddressLeaseTime(lmLease)),
		dhcpv4.WithOption(dhcpv4.OptRenewTimeValue(lmT1)),
		dhcpv4.WithOption(dhcpv4.OptRebindingTimeValue(lmT2)),
	}
	msg, err := dhcpv4.NewReplyFromRequest(request, append(mods, extra...)...)
	if err != nil {
		l.t.Fatalf("build %s: %v", kind, err)
	}
	return msg
}

// nak builds the DHCPNAK a server sends: message type and server identifier.
func (l *lmLab) nak(request *dhcpv4.DHCPv4) *dhcpv4.DHCPv4 {
	l.t.Helper()

	msg, err := dhcpv4.NewReplyFromRequest(request,
		dhcpv4.WithMessageType(dhcpv4.MessageTypeNak),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(sfServerIP)),
		dhcpv4.WithOption(dhcpv4.OptMessage("address reserved elsewhere")),
	)
	if err != nil {
		l.t.Fatalf("build NAK: %v", err)
	}
	return msg
}

// broadcast sends msg from server to 255.255.255.255:68.
func (l *lmLab) broadcast(server net.IP, msg *dhcpv4.DHCPv4) {
	l.t.Helper()

	l.write(net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, lmIPv4Frame(server, net.IPv4bcast, msg.ToBytes()))
}

// unicast sends msg from server to the client's leased address, port 68, as a
// server answers a DHCPREQUEST whose ciaddr is set (RFC 2131 Section 4.1).
func (l *lmLab) unicast(server, target net.IP, msg *dhcpv4.DHCPv4) {
	l.t.Helper()

	l.write(l.zeMAC, lmIPv4Frame(server, target, msg.ToBytes()))
}

func (l *lmLab) write(mac net.HardwareAddr, frame []byte) {
	l.t.Helper()

	if _, err := l.conn.WriteTo(frame, &packet.Addr{HardwareAddr: mac}); err != nil {
		l.t.Fatalf("write frame: %v", err)
	}
}

// neighbor lets the kernel send to ip across the veth, as sfSetupLab does for
// sfServerIP.
func (l *lmLab) neighbor(ip net.IP) {
	l.t.Helper()

	zeLink, err := netlink.LinkByName(l.zeSide)
	if err != nil {
		l.t.Fatalf("LinkByName(%s): %v", l.zeSide, err)
	}
	peerIfc, err := net.InterfaceByName(strings.Replace(l.zeSide, "zedhcpz", "zedhcpp", 1))
	if err != nil {
		l.t.Fatalf("peer of %s: %v", l.zeSide, err)
	}
	neigh := &netlink.Neigh{
		LinkIndex:    zeLink.Attrs().Index,
		Family:       netlink.FAMILY_V4,
		State:        netlink.NUD_PERMANENT,
		IP:           ip,
		HardwareAddr: peerIfc.HardwareAddr,
	}
	if err := netlink.NeighSet(neigh); err != nil {
		l.t.Fatalf("neighbor %v on %s: %v", ip, l.zeSide, err)
	}
}

// hasAddress reports whether ip is configured on the client's link.
func (l *lmLab) hasAddress(ip net.IP) bool {
	l.t.Helper()

	link, err := netlink.LinkByName(l.zeSide)
	if err != nil {
		l.t.Fatalf("LinkByName(%s): %v", l.zeSide, err)
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		l.t.Fatalf("AddrList(%s): %v", l.zeSide, err)
	}
	for i := range addrs {
		if addrs[i].IP.Equal(ip) {
			return true
		}
	}
	return false
}

// hasDefaultRoute reports whether the lease's default route via sfServerIP
// is installed on the client's link.
func (l *lmLab) hasDefaultRoute() bool {
	l.t.Helper()

	link, err := netlink.LinkByName(l.zeSide)
	if err != nil {
		l.t.Fatalf("LinkByName(%s): %v", l.zeSide, err)
	}
	routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
	if err != nil {
		l.t.Fatalf("RouteList(%s): %v", l.zeSide, err)
	}
	for i := range routes {
		if routes[i].Gw.Equal(sfServerIP) && routes[i].Priority == lmRouteMetric {
			return true
		}
	}
	return false
}

// awaitAddress waits until ip's presence on the link is want.
func (l *lmLab) awaitAddress(ip net.IP, want bool, within time.Duration) {
	l.t.Helper()

	deadline := time.Now().Add(within)
	for l.hasAddress(ip) != want {
		if time.Now().After(deadline) {
			l.t.Fatalf("address %v present = %v after %s, want %v", ip, !want, within, want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// lmParse reads an IPv4 frame as a DHCP message sent to the server port.
func lmParse(frame []byte) (lmFrame, bool) {
	msg, ok := sfParseDHCP(frame)
	if !ok {
		return lmFrame{}, false
	}
	hlen := int(frame[0]&0x0f) * 4
	udp := frame[hlen:]
	return lmFrame{
		msg:    msg,
		source: &net.UDPAddr{IP: net.IP(bytes.Clone(frame[12:16])), Port: int(binary.BigEndian.Uint16(udp[0:2]))},
		target: &net.UDPAddr{IP: net.IP(bytes.Clone(frame[16:20])), Port: int(binary.BigEndian.Uint16(udp[2:4]))},
	}, true
}

// lmIPv4Frame wraps a DHCP message from server port 67 to client port 68.
func lmIPv4Frame(source, target net.IP, payload []byte) []byte {
	frame := sfIPv4Header(sfUDPHeaderLen + len(payload))
	copy(frame[12:16], source.To4())
	copy(frame[16:20], target.To4())
	binary.BigEndian.PutUint16(frame[10:12], 0)
	binary.BigEndian.PutUint16(frame[10:12], sfChecksum(frame))
	frame = append(frame, sfUDPHeader(len(payload))...)
	return append(frame, payload...)
}

// TestDHCPv4RenewsRepeatedlyOnVeth holds one lease across four RENEW/ACK
// exchanges, more than one lease period. Method: every ACK grants six
// seconds, so the kernel removes the address unless each renewal refreshes
// it; a DHCPDISCOVER at any point fails the test.
func TestDHCPv4RenewsRepeatedlyOnVeth(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	lab := lmStart(t, dHCPConfig{})
	lab.bind()

	for i := range lmRenewals {
		renew := lab.readRequest(lmT1 + 2*time.Second)
		if !renew.msg.ClientIPAddr.Equal(sfClientIP) {
			t.Fatalf("renewal %d: ciaddr = %v, want the leased %v", i, renew.msg.ClientIPAddr, sfClientIP)
		}
		lab.unicast(sfServerIP, sfClientIP, lab.reply(renew.msg, dhcpv4.MessageTypeAck, sfServerIP, sfClientIP))
		// RFC requirement: RFC2131-4.4.5-5 negative -- a renewal DHCPACK for the address already held is not a new address, and the client keeps it configured.
		// RFC requirement: RFC2131-4.4.5-1 positive -- while every RENEW draws a DHCPACK the lease never expires: across four renewals spanning more than one lease the client keeps its address and sends no DHCPDISCOVER.
		lab.awaitAddress(sfClientIP, true, time.Second)
	}
}

// TestDHCPv4NakOnRenewRestartsOnVeth answers the RENEW with a DHCPNAK that
// the lab broadcasts, as a server does when giaddr is zero (RFC 2131
// Section 4.1). Method: the address must go and a DHCPDISCOVER follow well
// inside the 10 s DORA back-off. userns.Enter re-runs it as root over a user
// and network namespace of its own, so it needs no host privilege.
func TestDHCPv4NakOnRenewRestartsOnVeth(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	lab := lmStart(t, dHCPConfig{})
	lab.bind()

	renew := lab.readRequest(lmT1 + 2*time.Second)
	lab.broadcast(sfServerIP, lab.nak(renew.msg))

	lab.awaitAddress(sfClientIP, false, time.Second)
	next := lab.read(3 * time.Second)
	if next.msg.MessageType() != dhcpv4.MessageTypeDiscover {
		t.Fatalf("after the DHCPNAK the client sent %s, want DHCPDISCOVER", next.msg.MessageType())
	}
	if gap := next.read.Sub(renew.read); gap >= lmT2-lmT1+time.Second {
		t.Fatalf("DHCPDISCOVER %s after the NAKed RENEW: the NAK was missed and only the REBIND saw one", gap)
	}
}

// TestDHCPv4RebindAcceptsAnyServerOnVeth leaves the RENEW unanswered and
// answers the REBIND as a second server. Method: the REBIND's frame is
// checked, server B's ACK must hold the address past the first lease's
// expiry, and the next RENEW must go to B.
func TestDHCPv4RebindAcceptsAnyServerOnVeth(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	lab := lmStart(t, dHCPConfig{})
	lab.neighbor(lmServerB)
	bound := lab.bind()

	renew := lab.readRequest(lmT1 + 2*time.Second)
	if !renew.target.IP.Equal(sfServerIP) {
		t.Fatalf("RENEW sent to %v, want the leasing server %v", renew.target.IP, sfServerIP)
	}
	rebind := lab.readRequest(lmT2 - lmT1 + 2*time.Second)
	// RFC requirement: RFC2131-4.3.2-5 positive -- the DHCPREQUEST the client sends in REBINDING goes to 255.255.255.255.
	if !rebind.target.IP.Equal(net.IPv4bcast) {
		t.Fatalf("REBIND sent to %v, want 255.255.255.255", rebind.target.IP)
	}
	if rebind.target.Port != dhcpv4.ServerPort {
		t.Fatalf("REBIND sent to port %d, want %d", rebind.target.Port, dhcpv4.ServerPort)
	}
	if !rebind.msg.ClientIPAddr.Equal(sfClientIP) {
		t.Fatalf("REBIND ciaddr = %v, want the leased %v", rebind.msg.ClientIPAddr, sfClientIP)
	}
	// RFC requirement: RFC2131-4.4.5-7 positive -- the DHCPREQUEST the client sends in REBINDING carries no server identifier option.
	if rebind.msg.Options.Has(dhcpv4.OptionServerIdentifier) {
		t.Fatalf("REBIND carries a server identifier %v", rebind.msg.ServerIdentifier())
	}
	// RFC requirement: RFC2131-3.1-5 positive -- the DHCPREQUEST the client sends in REBINDING carries neither a server identifier nor a requested IP address option.
	if rebind.msg.Options.Has(dhcpv4.OptionRequestedIPAddress) {
		t.Fatalf("REBIND carries a requested IP address %v", rebind.msg.RequestedIPAddress())
	}
	if got := rebind.msg.Flags & flagsReserved; got != 0 {
		t.Fatalf("REBIND flags = %#04x, reserved bits %#04x set", rebind.msg.Flags, got)
	}

	lab.broadcast(lmServerB, lab.reply(rebind.msg, dhcpv4.MessageTypeAck, lmServerB, sfClientIP))

	// The first lease expires lmLease after the bound REQUEST: the address
	// must outlive it on server B's lease.
	time.Sleep(time.Until(bound.read.Add(lmLease + 500*time.Millisecond)))
	if !lab.hasAddress(sfClientIP) {
		t.Fatalf("address %v dropped at the first lease's expiry: server B's ACK was not taken", sfClientIP)
	}
	next := lab.readRequest(lmT1 + 2*time.Second)
	if !next.target.IP.Equal(lmServerB) {
		t.Fatalf("RENEW after the rebind sent to %v, want server B %v", next.target.IP, lmServerB)
	}
}

// TestDHCPv4RenewIsUnicastToServer reads the RENEW's IP and UDP addressing.
// Method: the lab answers nothing, so the first REQUEST after the bound one
// is the RENEWING transmission at T1.
func TestDHCPv4RenewIsUnicastToServer(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	lab := lmStart(t, dHCPConfig{})
	lab.bind()

	renew := lab.readRequest(lmT1 + 2*time.Second)
	// RFC requirement: RFC2131-4.1-10 positive -- the RENEWING DHCPREQUEST is unicast to the address the server identifier option of the ACK named, on port 67.
	if !renew.target.IP.Equal(sfServerIP) {
		t.Fatalf("RENEW sent to %v, want the server identifier %v", renew.target.IP, sfServerIP)
	}
	if renew.target.Port != dhcpv4.ServerPort {
		t.Fatalf("RENEW sent to port %d, want %d", renew.target.Port, dhcpv4.ServerPort)
	}
	if !renew.source.IP.Equal(sfClientIP) || renew.source.Port != dhcpv4.ClientPort {
		t.Fatalf("RENEW sent from %v, want %v:%d", renew.source, sfClientIP, dhcpv4.ClientPort)
	}
	if !renew.msg.ClientIPAddr.Equal(sfClientIP) {
		t.Fatalf("RENEW ciaddr = %v, want the leased %v", renew.msg.ClientIPAddr, sfClientIP)
	}
	// RFC requirement: RFC2131-4.4.5-6 positive -- the DHCPREQUEST the client sends in RENEWING carries no server identifier option.
	if renew.msg.Options.Has(dhcpv4.OptionServerIdentifier) {
		t.Fatalf("RENEW carries a server identifier %v", renew.msg.ServerIdentifier())
	}
	if renew.msg.Options.Has(dhcpv4.OptionRequestedIPAddress) {
		t.Fatalf("RENEW carries a requested IP address %v", renew.msg.RequestedIPAddress())
	}
}

// TestDHCPv4RenewCarriesClientID configures a client identifier and a host
// name. Method: the bound REQUEST's options 61 and 12 are compared byte for
// byte with the RENEW's and, the RENEW left unanswered, the REBIND's.
func TestDHCPv4RenewCarriesClientID(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	lab := lmStart(t, dHCPConfig{ClientID: "ze-lab-client", Hostname: "ze-lab"})
	bound := lab.bind()

	wantID := bound.msg.Options.Get(dhcpv4.OptionClientIdentifier)
	wantHost := bound.msg.Options.Get(dhcpv4.OptionHostName)
	if len(wantID) == 0 || len(wantHost) == 0 {
		t.Fatalf("bound REQUEST carries client-id %q and hostname %q, want both", wantID, wantHost)
	}
	renew := lab.readRequest(lmT1 + 2*time.Second)
	rebind := lab.readRequest(lmT2 - lmT1 + 2*time.Second)
	for _, extend := range []lmFrame{renew, rebind} {
		// RFC requirement: RFC2131-2-2 positive -- the RENEWING and REBINDING DHCPREQUESTs carry the client identifier option byte for byte as the DHCPREQUEST that bound the lease.
		if got := extend.msg.Options.Get(dhcpv4.OptionClientIdentifier); !bytes.Equal(got, wantID) {
			t.Fatalf("REQUEST to %v carries client-id %q, want %q", extend.target.IP, got, wantID)
		}
		if got := extend.msg.Options.Get(dhcpv4.OptionHostName); !bytes.Equal(got, wantHost) {
			t.Fatalf("REQUEST to %v carries hostname %q, want %q", extend.target.IP, got, wantHost)
		}
	}
}

// TestDHCPv4ExtendIgnoresServerEchoesOnVeth binds a lease from a server
// whose ACK carries what a careless client would copy into its extending
// DHCPREQUESTs: a client identifier other than the configured one, a
// requested IP address, and an siaddr that is not its server identifier.
// Method: the RENEW and the REBIND, both left unanswered, are read and
// checked against each echo.
func TestDHCPv4ExtendIgnoresServerEchoesOnVeth(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	lab := lmStart(t, dHCPConfig{ClientID: "ze-lab-client"})
	echoServer := net.IP{192, 0, 2, 9}
	bound := lab.bind(
		dhcpv4.WithServerIP(echoServer),
		dhcpv4.WithOption(dhcpv4.OptClientIdentifier([]byte("server-echo"))),
		dhcpv4.WithOption(dhcpv4.OptRequestedIPAddress(sfClientIP)),
	)
	wantID := bound.msg.Options.Get(dhcpv4.OptionClientIdentifier)

	renew := lab.readRequest(lmT1 + 2*time.Second)
	// RFC requirement: RFC2131-4.1-10 negative -- an ACK whose siaddr differs from its server identifier does not move the RENEW: it is unicast to the server identifier, not to siaddr.
	if !renew.target.IP.Equal(sfServerIP) {
		t.Fatalf("RENEW sent to %v, want the server identifier %v, not siaddr %v", renew.target.IP, sfServerIP, echoServer)
	}
	// RFC requirement: RFC2131-4.4.5-6 negative -- the ACK the RENEW extends carries a server identifier, and the RENEWING DHCPREQUEST still carries none.
	if renew.msg.Options.Has(dhcpv4.OptionServerIdentifier) {
		t.Fatalf("RENEW copies the ACK's server identifier %v", renew.msg.ServerIdentifier())
	}

	rebind := lab.readRequest(lmT2 - lmT1 + 2*time.Second)
	// RFC requirement: RFC2131-4.3.2-5 negative -- with the leasing server's identifier known and reachable by unicast, the REBINDING DHCPREQUEST is still broadcast to 255.255.255.255.
	if !rebind.target.IP.Equal(net.IPv4bcast) {
		t.Fatalf("REBIND sent to %v, want 255.255.255.255", rebind.target.IP)
	}
	// RFC requirement: RFC2131-4.4.5-7 negative -- the ACK the REBIND extends carries a server identifier, and the REBINDING DHCPREQUEST still carries none.
	if rebind.msg.Options.Has(dhcpv4.OptionServerIdentifier) {
		t.Fatalf("REBIND copies the ACK's server identifier %v", rebind.msg.ServerIdentifier())
	}
	// RFC requirement: RFC2131-3.1-5 negative -- the ACK the REBIND extends carries a server identifier and a requested IP address option, and the REBINDING DHCPREQUEST carries neither.
	if rebind.msg.Options.Has(dhcpv4.OptionRequestedIPAddress) {
		t.Fatalf("REBIND copies the ACK's requested IP address %v", rebind.msg.RequestedIPAddress())
	}
	for _, extend := range []lmFrame{renew, rebind} {
		// RFC requirement: RFC2131-2-2 negative -- the ACK echoes a different client identifier, and the RENEWING and REBINDING DHCPREQUESTs still carry the one that bound the lease.
		if got := extend.msg.Options.Get(dhcpv4.OptionClientIdentifier); !bytes.Equal(got, wantID) {
			t.Fatalf("REQUEST to %v carries client-id %q, want the bound %q, not the echo", extend.target.IP, got, wantID)
		}
	}
}

// TestDHCPv4ExpiresOnTimeOnVeth binds a lease with a router and answers
// nothing after. Method: the address and the default route must be there at
// T2 and gone within a second of expiry, with a DHCPDISCOVER following.
func TestDHCPv4ExpiresOnTimeOnVeth(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	lab := lmStart(t, dHCPConfig{})
	bound := lab.bind(dhcpv4.WithOption(dhcpv4.OptRouter(sfServerIP)))
	if !lab.hasDefaultRoute() {
		t.Fatalf("no default route via %v metric %d after bind", sfServerIP, lmRouteMetric)
	}

	lab.readRequest(lmT1 + 2*time.Second)        // RENEW, unanswered
	lab.readRequest(lmT2 - lmT1 + 2*time.Second) // REBIND, unanswered
	if !lab.hasAddress(sfClientIP) || !lab.hasDefaultRoute() {
		t.Fatalf("lease torn down in REBINDING, before its expiry")
	}

	expiry := bound.read.Add(lmLease)
	next := lab.read(time.Until(expiry) + 2*time.Second)
	if next.msg.MessageType() != dhcpv4.MessageTypeDiscover {
		t.Fatalf("after expiry the client sent %s, want DHCPDISCOVER", next.msg.MessageType())
	}
	// RFC requirement: RFC2131-4.4.5-1 negative -- with no DHCPACK before the lease expires, the client removes the address and its default route within a second of expiry and sends a DHCPDISCOVER.
	if late := next.read.Sub(expiry); late > time.Second {
		t.Fatalf("DHCPDISCOVER %s after expiry, want within 1s", late)
	}
	if lab.hasAddress(sfClientIP) {
		t.Fatalf("address %v still configured after expiry", sfClientIP)
	}
	if lab.hasDefaultRoute() {
		t.Fatalf("default route via %v still installed after expiry", sfServerIP)
	}
}

// TestDHCPv4RenewalMovesToNewAddressOnVeth answers the RENEW with an ACK for
// another address. Method: the new address must be configured and the old
// one removed.
func TestDHCPv4RenewalMovesToNewAddressOnVeth(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	lab := lmStart(t, dHCPConfig{})
	lab.bind()

	renew := lab.readRequest(lmT1 + 2*time.Second)
	lab.unicast(sfServerIP, sfClientIP, lab.reply(renew.msg, dhcpv4.MessageTypeAck, sfServerIP, lmRenumbered))

	lab.awaitAddress(lmRenumbered, true, time.Second)
	// RFC requirement: RFC2131-4.4.5-5 positive -- given a new address by a renewal DHCPACK, the client removes the previous address from the interface.
	lab.awaitAddress(sfClientIP, false, time.Second)
}
