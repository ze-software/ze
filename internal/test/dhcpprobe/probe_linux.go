// Design: docs/architecture/provisioning/dhcp-server.md -- the DHCP server the probe exchanges with
// Related: frame.go -- the frames written and read here
// Related: step.go -- the steps Run performs
// Related: probe_other.go -- the refusal on a host with no veth or AF_PACKET

//go:build linux

package dhcpprobe

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// replyWait is how long one transmission waits for its reply before the
// request is sent again, the retransmission a client performs.
const replyWait = time.Second

// silentWindow bounds the wait for a reply that must not come when no
// sentinel is named: the request is sent once every replyWait across it, as a
// client retransmits, so a listener that binds within the window still
// receives one. With a sentinel the barrier is the sentinel's reply.
const silentWindow = 3 * time.Second

// peerName is the probe's end of the veth pair whose server end is name.
func peerName(name string) string {
	return name + "p"
}

// CreateLink creates the veth pair name (the daemon's listen interface) and
// name+"p" (the probe's end), brings both up and gives the server end addrs
// in order, so the first is its primary address. A link left by an earlier
// run that failed before DeleteLink is removed first: the name is the test's
// own. name MUST be at most 14 bytes.
func CreateLink(name string, addrs []netip.Prefix) error {
	if old, err := netlink.LinkByName(name); err == nil {
		if err := netlink.LinkDel(old); err != nil {
			return fmt.Errorf("dhcpprobe: remove the stale %s: %w", name, err)
		}
	}
	attrs := netlink.NewLinkAttrs()
	attrs.Name = name
	veth := &netlink.Veth{LinkAttrs: attrs, PeerName: peerName(name)}
	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("dhcpprobe: create veth %s/%s: %w", name, peerName(name), err)
	}
	for _, end := range []string{name, peerName(name)} {
		link, err := netlink.LinkByName(end)
		if err != nil {
			return fmt.Errorf("dhcpprobe: read %s back: %w", end, err)
		}
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("dhcpprobe: bring %s up: %w", end, err)
		}
	}
	for _, addr := range addrs {
		if err := changeAddr(name, addr, true); err != nil {
			return err
		}
	}
	return nil
}

// DeleteLink removes the veth pair. Deleting one end deletes its peer.
func DeleteLink(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("dhcpprobe: read %s for removal: %w", name, err)
	}
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("dhcpprobe: remove %s: %w", name, err)
	}
	return nil
}

func changeAddr(name string, prefix netip.Prefix, add bool) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("dhcpprobe: read %s: %w", name, err)
	}
	ip := prefix.Addr().As4()
	addr := &netlink.Addr{IPNet: &net.IPNet{IP: net.IP(ip[:]), Mask: net.CIDRMask(prefix.Bits(), 32)}}
	if add {
		if err := netlink.AddrAdd(link, addr); err != nil {
			return fmt.Errorf("dhcpprobe: add %s to %s: %w", prefix, name, err)
		}
		return nil
	}
	if err := netlink.AddrDel(link, addr); err != nil {
		return fmt.Errorf("dhcpprobe: remove %s from %s: %w", prefix, name, err)
	}
	return nil
}

// Probe sends requests on the probe's end of the pair and reads the replies
// the server writes on its end. Not safe for concurrent use.
type Probe struct {
	link      string
	peerIndex int
	peerMAC   net.HardwareAddr
	send      int // AF_PACKET socket that writes requests
	capture   int // AF_PACKET socket that reads every frame arriving on the probe's end
	sentinel  net.HardwareAddr
}

// Open opens the sockets on the pair CreateLink made. The caller MUST call
// Close.
func Open(name string) (*Probe, error) {
	peer, err := netlink.LinkByName(peerName(name))
	if err != nil {
		return nil, fmt.Errorf("dhcpprobe: read %s: %w", peerName(name), err)
	}
	p := &Probe{link: name, peerIndex: peer.Attrs().Index, peerMAC: peer.Attrs().HardwareAddr, send: -1, capture: -1}
	// Protocol 0: the send socket writes, and receives nothing.
	p.send, err = unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		p.send = -1
		return nil, fmt.Errorf("dhcpprobe: open the send socket: %w", err)
	}
	all := htons(unix.ETH_P_ALL)
	p.capture, err = unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, int(all))
	if err != nil {
		p.capture = -1
		p.Close() //nolint:errcheck // the open error is the one reported
		return nil, fmt.Errorf("dhcpprobe: open the capture socket: %w", err)
	}
	if err := unix.Bind(p.capture, &unix.SockaddrLinklayer{Ifindex: p.peerIndex, Protocol: all}); err != nil {
		p.Close() //nolint:errcheck // the bind error is the one reported
		return nil, fmt.Errorf("dhcpprobe: bind the capture socket to %s: %w", peerName(name), err)
	}
	return p, nil
}

// Close closes both sockets.
func (p *Probe) Close() error {
	var errs []error
	for _, fd := range []int{p.send, p.capture} {
		if fd < 0 {
			continue
		}
		if err := unix.Close(fd); err != nil {
			errs = append(errs, fmt.Errorf("dhcpprobe: close a socket: %w", err))
		}
	}
	p.send, p.capture = -1, -1
	return errors.Join(errs...)
}

// Run performs steps in order and prints one line per exchange. It stops at
// the first step that fails. reload performs StepReload, which needs the
// daemon the caller started.
func (p *Probe) Run(steps []Step, reload func() error, emit func(string)) error {
	for i := range steps {
		if err := p.step(&steps[i], reload, emit); err != nil {
			return err
		}
	}
	return nil
}

func (p *Probe) step(s *Step, reload func() error, emit func(string)) error {
	switch s.Kind {
	case StepAddrAdd:
		return changeAddr(p.link, s.Prefix, true)
	case StepAddrDel:
		return changeAddr(p.link, s.Prefix, false)
	case StepRoute:
		return p.route(s.Prefix)
	case StepNeigh:
		return p.neigh(s.Addr)
	case StepReload:
		return reload()
	case StepSentinel:
		p.sentinel = s.MAC
		return nil
	case StepExchange:
		if s.Exchange.Silent {
			return p.silent(&s.Exchange, emit)
		}
		return p.exchange(&s.Exchange, emit)
	case StepUnspecified:
	}
	return errors.New("dhcpprobe: a step with no kind reached Run")
}

func (p *Probe) route(prefix netip.Prefix) error {
	link, err := netlink.LinkByName(p.link)
	if err != nil {
		return fmt.Errorf("dhcpprobe: read %s: %w", p.link, err)
	}
	ip := prefix.Addr().As4()
	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       &net.IPNet{IP: net.IP(ip[:]), Mask: net.CIDRMask(prefix.Bits(), 32)},
		Scope:     netlink.SCOPE_LINK,
	}
	if err := netlink.RouteAdd(route); err != nil {
		return fmt.Errorf("dhcpprobe: route %s through %s: %w", prefix, p.link, err)
	}
	return nil
}

func (p *Probe) neigh(addr netip.Addr) error {
	link, err := netlink.LinkByName(p.link)
	if err != nil {
		return fmt.Errorf("dhcpprobe: read %s: %w", p.link, err)
	}
	ip := addr.As4()
	entry := &netlink.Neigh{
		LinkIndex:    link.Attrs().Index,
		Family:       netlink.FAMILY_V4,
		State:        netlink.NUD_PERMANENT,
		IP:           net.IP(ip[:]),
		HardwareAddr: p.peerMAC,
	}
	if err := netlink.NeighSet(entry); err != nil {
		return fmt.Errorf("dhcpprobe: neighbour %s on %s: %w", addr, p.link, err)
	}
	return nil
}

// exchange sends ex until a reply of the expected type arrives, then checks
// every field the exchange names. A reply of another type on the last
// attempt, or none at all, is an error.
func (p *Probe) exchange(ex *Exchange, emit func(string)) error {
	var last Reply
	got := false
	for range ex.Attempts {
		reply, ok, err := p.transact(&ex.Request)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		last, got = reply, true
		if reply.Type == ex.Want.Type {
			break
		}
	}
	if !got {
		return fmt.Errorf("dhcpprobe: exchange %s: no reply to %d %s requests, %s each",
			ex.Label, ex.Attempts, ex.Request.Type, replyWait)
	}
	emit(ex.Line(last))
	return ex.Check(last)
}

// silent proves that the server does not answer ex. With a sentinel named, ex
// is sent once and a DHCPDISCOVER from the sentinel follows it: the server
// answers one listener's requests in arrival order, so once the sentinel's
// reply is read, a reply to ex would already have been. Without one, ex is
// sent once every replyWait across silentWindow and no reply may arrive.
func (p *Probe) silent(ex *Exchange, emit func(string)) error {
	if p.sentinel == nil {
		for range int(silentWindow / replyWait) {
			reply, ok, err := p.transact(&ex.Request)
			if err != nil {
				return err
			}
			if ok {
				emit(ex.Line(reply))
				return fmt.Errorf("dhcpprobe: exchange %s: the server answered %s, want no reply", ex.Label, reply.Type)
			}
		}
		emit(ex.SilentLine())
		return nil
	}
	if err := p.drain(); err != nil {
		return err
	}
	if err := p.write(&ex.Request); err != nil {
		return err
	}
	sentinel := Request{Type: MsgDiscover, XID: xidFor("sentinel " + ex.Label), MAC: p.sentinel, Broadcast: true}
	for range exchangeAttempts {
		if err := p.write(&sentinel); err != nil {
			return err
		}
		reply, ok, err := p.readEither(ex.Request.XID, sentinel.XID, replyWait)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if reply.XID == ex.Request.XID {
			emit(ex.Line(reply))
			return fmt.Errorf("dhcpprobe: exchange %s: the server answered %s, want no reply", ex.Label, reply.Type)
		}
		emit(ex.SilentLine())
		return nil
	}
	return fmt.Errorf("dhcpprobe: exchange %s: the sentinel %s was never answered, so the silence proves nothing",
		ex.Label, p.sentinel)
}

// transact sends req once and waits replyWait for its reply.
func (p *Probe) transact(req *Request) (Reply, bool, error) {
	if err := p.drain(); err != nil {
		return Reply{}, false, err
	}
	if err := p.write(req); err != nil {
		return Reply{}, false, err
	}
	return p.read(req.XID, replyWait)
}

func (p *Probe) write(req *Request) error {
	frame, err := buildFrame(*req)
	if err != nil {
		return err
	}
	to := &unix.SockaddrLinklayer{Ifindex: p.peerIndex, Protocol: htons(unix.ETH_P_IP), Halen: 6}
	copy(to.Addr[:], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	if err := unix.Sendto(p.send, frame, 0, to); err != nil {
		return fmt.Errorf("dhcpprobe: send %s on %s: %w", req.Type, peerName(p.link), err)
	}
	return nil
}

func (p *Probe) read(xid uint32, wait time.Duration) (Reply, bool, error) {
	return p.readEither(xid, xid, wait)
}

// readEither answers the first reply carrying xid or other that arrives
// within wait. ok is false when none did.
func (p *Probe) readEither(xid, other uint32, wait time.Duration) (Reply, bool, error) {
	deadline := time.Now().Add(wait)
	var buf [2048]byte
	for time.Now().Before(deadline) {
		poll := []unix.PollFd{{Fd: int32(p.capture), Events: unix.POLLIN}} //nolint:gosec // a file descriptor fits in int32
		remaining := time.Until(deadline)
		ready, err := unix.Poll(poll, int((remaining+time.Millisecond-1)/time.Millisecond))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return Reply{}, false, fmt.Errorf("dhcpprobe: poll the capture socket: %w", err)
		}
		if ready == 0 {
			break
		}
		n, from, err := unix.Recvfrom(p.capture, buf[:], unix.MSG_DONTWAIT)
		if errors.Is(err, unix.EAGAIN) {
			continue
		}
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return Reply{}, false, fmt.Errorf("dhcpprobe: read the capture socket: %w", err)
		}
		link, ok := from.(*unix.SockaddrLinklayer)
		if !ok {
			continue
		}
		// The probe's own requests leave through this end too.
		if link.Pkttype == unix.PACKET_OUTGOING {
			continue
		}
		reply, ok := parseReply(buf[:n])
		if !ok {
			continue
		}
		if reply.XID == xid {
			return reply, true, nil
		}
		if reply.XID == other {
			return reply, true, nil
		}
	}
	return Reply{}, false, nil
}

// drain discards frames queued before this request, so a late reply to an
// earlier transmission cannot answer for this one.
func (p *Probe) drain() error {
	var buf [2048]byte
	for {
		_, _, err := unix.Recvfrom(p.capture, buf[:], unix.MSG_DONTWAIT)
		if errors.Is(err, unix.EAGAIN) {
			return nil
		}
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("dhcpprobe: drain the capture socket: %w", err)
		}
	}
}

// htons answers v in network byte order, which AF_PACKET takes for its
// protocol field on every architecture Ze builds (all little-endian).
func htons(v uint16) uint16 {
	return v<<8 | v>>8
}
