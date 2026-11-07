// Design: docs/guide/firewall.md -- tcp-mss-set, the clamp a forwarded SYN meets
// Related: frame.go -- the SYN this lab sends and the parse of the one it reads
// Related: msslab_other.go -- the refusal on a host with no veth or AF_PACKET

//go:build linux

package msslab

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// The router owns .1 (::1) on each side. The SYN comes from .10 on the
// ingress side and goes to .20 on the egress side, which no interface holds,
// so the router forwards it rather than delivering it.
var (
	ingressRouterV4 = net.IPv4(192, 0, 2, 1).To4()
	egressRouterV4  = net.IPv4(198, 51, 100, 1).To4()
	ingressRouterV6 = net.ParseIP("2001:db8:a::1")
	egressRouterV6  = net.ParseIP("2001:db8:b::1")

	egressRouterMAC = net.HardwareAddr{0x02, 0, 0, 0, 0x02, 0x01}
	egressPeerMAC   = net.HardwareAddr{0x02, 0, 0, 0, 0x02, 0x02}
	// targetMAC is NOT the egress peer's address. The forwarded frame then
	// reaches the peer as PACKET_OTHERHOST: the capture socket still reads it,
	// and the peer's IP layer drops it, so the router never forwards its own
	// output a second time.
	targetMAC = net.HardwareAddr{0x02, 0, 0, 0, 0x02, 0x20}
)

// Lab is one router: the calling thread's network namespace with two veth
// pairs. The SYN is injected on the ingress peer, forwarded by the kernel
// through every netfilter hook a forwarded packet meets (prerouting, forward,
// postrouting), and read on the egress peer, so what it reports is what the
// next hop would receive.
//
// Not safe for concurrent use. Open, every Forward and Close MUST run on a
// thread that is in the router namespace: a test locks its OS thread first,
// and a fixture process runs wholly inside the test's namespace.
type Lab struct {
	ingress     string
	egress      string
	ingressPeer int // ifindex the SYN is injected on
	inject      int // AF_PACKET socket that sends
	capture     int // AF_PACKET socket that reads the forwarded frame
	sourcePort  uint16
}

// Open builds the lab in the calling thread's namespace. ingress and egress
// name the router-side veths; the peer of each is the same name with a "p"
// suffix, so each name MUST be at most 14 bytes. The caller MUST call Close.
func Open(ingress, egress string) (*Lab, error) {
	if err := addVethPair(ingress, ingressRouterMAC[:], ingressPeerMAC[:]); err != nil {
		return nil, err
	}
	lab := &Lab{ingress: ingress, inject: -1, capture: -1, sourcePort: 40000}
	if err := addVethPair(egress, egressRouterMAC, egressPeerMAC); err != nil {
		lab.Close() //nolint:errcheck // the setup error is the one reported
		return nil, err
	}
	lab.egress = egress
	if err := lab.setup(); err != nil {
		lab.Close() //nolint:errcheck // the setup error is the one reported
		return nil, err
	}
	return lab, nil
}

func (l *Lab) setup() error {
	if err := l.address(); err != nil {
		return err
	}
	if err := l.forwarding(); err != nil {
		return err
	}
	return l.sockets()
}

// Close closes the sockets and removes both veth pairs. Deleting one end of a
// veth deletes its peer.
func (l *Lab) Close() error {
	var errs []error
	for _, fd := range []int{l.inject, l.capture} {
		if fd < 0 {
			continue
		}
		if err := unix.Close(fd); err != nil {
			errs = append(errs, fmt.Errorf("msslab: close a socket: %w", err))
		}
	}
	for _, name := range []string{l.ingress, l.egress} {
		if name == "" {
			continue
		}
		link, err := netlink.LinkByName(name)
		if err != nil {
			errs = append(errs, fmt.Errorf("msslab: read %s for removal: %w", name, err))
			continue
		}
		if err := netlink.LinkDel(link); err != nil {
			errs = append(errs, fmt.Errorf("msslab: remove %s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}

// Forward sends one SYN of the family, carrying sent, and answers the MSS
// option of that same SYN as the egress peer received it. The probe is told
// apart from every other frame by its TCP source port, which each call
// changes. No frame within wait is an error, never an absent option.
func (l *Lab) Forward(family Family, sent Option, wait time.Duration) (Seen, error) {
	l.sourcePort++
	frame, err := synFrame(family, sent, l.sourcePort)
	if err != nil {
		return Seen{}, err
	}
	if err := l.drain(); err != nil {
		return Seen{}, err
	}
	kind, err := etherType(family)
	if err != nil {
		return Seen{}, err
	}
	target := &unix.SockaddrLinklayer{Ifindex: l.ingressPeer, Halen: 6, Protocol: htons(kind)}
	copy(target.Addr[:], ingressRouterMAC[:])
	if err := unix.Sendto(l.inject, frame, 0, target); err != nil {
		return Seen{}, fmt.Errorf("msslab: send the %s SYN on %sp: %w", family, l.ingress, err)
	}
	return l.read(family, wait)
}

func (l *Lab) read(family Family, wait time.Duration) (Seen, error) {
	deadline := time.Now().Add(wait)
	var buf [2048]byte
	for time.Now().Before(deadline) {
		poll := []unix.PollFd{{Fd: int32(l.capture), Events: unix.POLLIN}}
		remaining := time.Until(deadline)
		ready, err := unix.Poll(poll, int((remaining+time.Millisecond-1)/time.Millisecond))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return Seen{}, fmt.Errorf("msslab: poll the capture socket: %w", err)
		}
		if ready == 0 {
			break
		}
		n, from, err := unix.Recvfrom(l.capture, buf[:], unix.MSG_DONTWAIT)
		if errors.Is(err, unix.EAGAIN) {
			continue
		}
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return Seen{}, fmt.Errorf("msslab: read the capture socket: %w", err)
		}
		link, ok := from.(*unix.SockaddrLinklayer)
		if !ok {
			continue
		}
		if link.Pkttype == unix.PACKET_OUTGOING {
			continue
		}
		seen, match := parseSYN(buf[:n], family, l.sourcePort)
		if match {
			return seen, nil
		}
	}
	return Seen{}, fmt.Errorf("msslab: no forwarded %s SYN from port %d reached %sp within %s, "+
		"so the router or a rule dropped it", family, l.sourcePort, l.egress, wait)
}

// drain discards frames queued before this probe, so a late copy of an
// earlier probe cannot answer for this one.
func (l *Lab) drain() error {
	var buf [2048]byte
	for {
		_, _, err := unix.Recvfrom(l.capture, buf[:], unix.MSG_DONTWAIT)
		if errors.Is(err, unix.EAGAIN) {
			return nil
		}
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("msslab: drain the capture socket: %w", err)
		}
	}
}

func addVethPair(name string, mac, peerMAC net.HardwareAddr) error {
	attrs := netlink.NewLinkAttrs()
	attrs.Name = name
	attrs.HardwareAddr = mac
	veth := &netlink.Veth{LinkAttrs: attrs, PeerName: name + "p", PeerHardwareAddr: peerMAC}
	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("msslab: create veth %s/%sp: %w", name, name, err)
	}
	for _, end := range []string{name, name + "p"} {
		link, err := netlink.LinkByName(end)
		if err != nil {
			return fmt.Errorf("msslab: read %s back: %w", end, err)
		}
		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("msslab: bring %s up: %w", end, err)
		}
	}
	return nil
}

// address gives the router its two sides and the egress side its permanent
// neighbour. IPv6 addresses skip DAD, so the router forwards from the first
// probe rather than after a tentative period.
func (l *Lab) address() error {
	sides := []struct {
		name   string
		v4, v6 net.IP
	}{{l.ingress, ingressRouterV4, ingressRouterV6}, {l.egress, egressRouterV4, egressRouterV6}}
	for _, side := range sides {
		link, err := netlink.LinkByName(side.name)
		if err != nil {
			return fmt.Errorf("msslab: read %s: %w", side.name, err)
		}
		v4 := &netlink.Addr{IPNet: &net.IPNet{IP: side.v4, Mask: net.CIDRMask(24, 32)}}
		if err := netlink.AddrAdd(link, v4); err != nil {
			return fmt.Errorf("msslab: address %s on %s: %w", side.v4, side.name, err)
		}
		v6 := &netlink.Addr{IPNet: &net.IPNet{IP: side.v6, Mask: net.CIDRMask(64, 128)}, Flags: unix.IFA_F_NODAD}
		if err := netlink.AddrAdd(link, v6); err != nil {
			return fmt.Errorf("msslab: address %s on %s: %w", side.v6, side.name, err)
		}
	}
	egress, err := netlink.LinkByName(l.egress)
	if err != nil {
		return fmt.Errorf("msslab: read %s: %w", l.egress, err)
	}
	targets := []struct {
		ip     net.IP
		family int
	}{{net.IP(targetV4[:]), netlink.FAMILY_V4}, {net.IP(targetV6[:]), netlink.FAMILY_V6}}
	for _, target := range targets {
		neigh := &netlink.Neigh{
			LinkIndex:    egress.Attrs().Index,
			Family:       target.family,
			State:        netlink.NUD_PERMANENT,
			IP:           target.ip,
			HardwareAddr: targetMAC,
		}
		if err := netlink.NeighAdd(neigh); err != nil {
			return fmt.Errorf("msslab: neighbour %s on %s: %w", target.ip, l.egress, err)
		}
	}
	return nil
}

// forwarding makes the namespace a router. /proc/sys/net answers for the
// calling thread's namespace. Source validation is off because the SYN comes
// from a host the router has never heard from.
func (l *Lab) forwarding() error {
	settings := []struct{ path, value string }{
		{"/proc/sys/net/ipv4/ip_forward", "1"},
		{"/proc/sys/net/ipv6/conf/all/forwarding", "1"},
		{"/proc/sys/net/ipv4/conf/all/rp_filter", "0"},
		{"/proc/sys/net/ipv4/conf/" + l.ingress + "/rp_filter", "0"},
	}
	for _, setting := range settings {
		if err := os.WriteFile(setting.path, []byte(setting.value), 0o600); err != nil {
			return fmt.Errorf("msslab: set %s to %s: %w", setting.path, setting.value, err)
		}
	}
	return nil
}

func (l *Lab) sockets() error {
	peer, err := netlink.LinkByName(l.ingress + "p")
	if err != nil {
		return fmt.Errorf("msslab: read %sp: %w", l.ingress, err)
	}
	l.ingressPeer = peer.Attrs().Index
	// Protocol 0: the inject socket sends, and receives nothing.
	l.inject, err = unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		l.inject = -1
		return fmt.Errorf("msslab: open the inject socket: %w", err)
	}
	capturePeer, err := netlink.LinkByName(l.egress + "p")
	if err != nil {
		return fmt.Errorf("msslab: read %sp: %w", l.egress, err)
	}
	all := htons(unix.ETH_P_ALL)
	l.capture, err = unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, int(all))
	if err != nil {
		l.capture = -1
		return fmt.Errorf("msslab: open the capture socket: %w", err)
	}
	if err := unix.Bind(l.capture, &unix.SockaddrLinklayer{Ifindex: capturePeer.Attrs().Index, Protocol: all}); err != nil {
		return fmt.Errorf("msslab: bind the capture socket to %sp: %w", l.egress, err)
	}
	return nil
}

// htons answers v in network byte order, which AF_PACKET takes for its
// protocol field on every architecture Ze builds (all little-endian).
func htons(v uint16) uint16 {
	return v<<8 | v>>8
}
