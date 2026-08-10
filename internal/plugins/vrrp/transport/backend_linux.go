//go:build linux

// RFC: rfc/short/rfc9568.md -- Section 5.1/5.2 (advert transport), 7.2 (source identity)
// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- Linux raw proto-112 backend (rx parent / tx macvlan)
// Related: garp_linux.go -- AF_PACKET gratuitous-ARP sender on the macvlan
// Related: na_linux.go -- raw ICMPv6 unsolicited-NA sender on the macvlan
//
// The Linux backend opens, per instance, a raw proto-112 RX socket bound to the
// PARENT interface with the VRRP multicast group joined, and a raw proto-112 TX
// socket bound to the instance's MACVLAN so adverts egress with the virtual MAC.
// IPv4 TX uses IP_HDRINCL with an explicitly built header (the addressless macvlan
// has no primary IPv4 for kernel source selection); IPv6 TX pins the source via an
// IPV6_PKTINFO cmsg and offloads the checksum to the kernel (IPV6_CHECKSUM). A
// per-instance readLoop wakes every rcvTimeout to observe the stop signal
// (goroutine-lifecycle: one readLoop + one announcer per instance).

package transport

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

const (
	// rcvTimeout bounds a blocking Recvfrom/Recvmsg so the readLoop wakes to check
	// its stop channel even when no datagram arrives.
	rcvTimeout = 500 * time.Millisecond
	// rxBufLen is the per-instance receive buffer. It holds a 60-byte max IPv4
	// header plus the largest advert payload (MaxLenV3v6 = 264); 2048 gives
	// headroom and truncates a jumbo datagram, which the codec length-check drops.
	rxBufLen = 2048
	// oobBufLen backs the v6 control-message blob (IPV6_HOPLIMIT + IPV6_PKTINFO).
	oobBufLen = 128
)

type linuxBackend struct{}

// NewBackend returns the Linux raw-socket backend.
func NewBackend() Backend { return linuxBackend{} }

// htons converts a uint16 to network byte order for the AF_PACKET protocol field.
func htons(v uint16) uint16 { return (v<<8)&0xff00 | (v>>8)&0x00ff }

// linuxInstance is one open per-instance socket set.
type linuxInstance struct {
	spec        InstanceSpec
	family      uint8
	rxFD        int
	txFD        int
	annFD       int
	parentIf    int
	macvlanIf   int
	macvlanName string
	sink        rxSink

	sendMu sync.Mutex
	v6Src  netip.Addr // cached macvlan link-local (v6 source), resolved lazily

	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

func (linuxBackend) OpenInstance(spec InstanceSpec, sink rxSink) (InstanceHandle, error) {
	pb, err := iface.Resolve(spec.Parent)
	if err != nil {
		return nil, fmt.Errorf("vrrp/transport: resolve parent %s: %w", spec.Parent, err)
	}
	parentOS := pb.OsName
	if parentOS == "" {
		parentOS = spec.Parent
	}
	mvi, err := net.InterfaceByName(spec.MacvlanDevice)
	if err != nil {
		return nil, fmt.Errorf("vrrp/transport: resolve macvlan %s: %w", spec.MacvlanDevice, err)
	}

	li := &linuxInstance{
		spec:        spec,
		family:      spec.Family,
		rxFD:        -1,
		txFD:        -1,
		annFD:       -1,
		parentIf:    pb.Ifindex,
		macvlanIf:   mvi.Index,
		macvlanName: spec.MacvlanDevice,
		sink:        sink,
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
	if oerr := li.openSockets(parentOS); oerr != nil {
		_ = li.closeFDs() // cleanup on the error path; return the original openSockets error
		return nil, oerr
	}
	go li.readLoop()
	return li, nil
}

// openSockets opens the rx/tx/announce sockets for the instance's family.
func (li *linuxInstance) openSockets(parentOS string) error {
	if li.family == packet.V6 {
		return li.openV6(parentOS)
	}
	return li.openV4(parentOS)
}

func (li *linuxInstance) openV4(parentOS string) error {
	rxFD, err := openRawIP(unix.AF_INET, parentOS)
	if err != nil {
		return err
	}
	li.rxFD = rxFD
	if terr := setRcvTimeout(rxFD); terr != nil {
		return terr
	}
	// Join 224.0.0.18 by parent ifindex (RFC 9568 Constants).
	mreq := &unix.IPMreqn{Multiaddr: packet.MulticastV4.As4(), Ifindex: int32(li.parentIf)}
	if jerr := unix.SetsockoptIPMreqn(rxFD, unix.IPPROTO_IP, unix.IP_ADD_MEMBERSHIP, mreq); jerr != nil {
		return fmt.Errorf("vrrp/transport: join 224.0.0.18 on ifindex %d: %w", li.parentIf, jerr)
	}

	txFD, err := openRawIP(unix.AF_INET, li.macvlanName)
	if err != nil {
		return err
	}
	li.txFD = txFD
	// RFC 9568 Section 7.2: adverts egress the macvlan (virtual MAC); IP_HDRINCL lets
	// us set the RFC-mandated source and TTL the addressless macvlan cannot supply.
	if herr := unix.SetsockoptInt(txFD, unix.IPPROTO_IP, unix.IP_HDRINCL, 1); herr != nil {
		return fmt.Errorf("vrrp/transport: setsockopt IP_HDRINCL: %w", herr)
	}
	if lerr := unix.SetsockoptInt(txFD, unix.IPPROTO_IP, unix.IP_MULTICAST_LOOP, 0); lerr != nil {
		return fmt.Errorf("vrrp/transport: setsockopt IP_MULTICAST_LOOP: %w", lerr)
	}

	annFD, err := openGARPSocket(li.macvlanIf)
	if err != nil {
		return err
	}
	li.annFD = annFD
	return nil
}

func (li *linuxInstance) openV6(parentOS string) error {
	rxFD, err := openRawIP(unix.AF_INET6, parentOS)
	if err != nil {
		return err
	}
	li.rxFD = rxFD
	if terr := setRcvTimeout(rxFD); terr != nil {
		return terr
	}
	// Join ff02::12 by parent ifindex (RFC 9568 Constants).
	mreq := &unix.IPv6Mreq{Interface: uint32(li.parentIf)}
	mreq.Multiaddr = packet.MulticastV6.As16()
	if jerr := unix.SetsockoptIPv6Mreq(rxFD, unix.IPPROTO_IPV6, unix.IPV6_JOIN_GROUP, mreq); jerr != nil {
		return fmt.Errorf("vrrp/transport: join ff02::12 on ifindex %d: %w", li.parentIf, jerr)
	}
	// Deliver dst (IPV6_PKTINFO) and hop limit (IPV6_RECVHOPLIMIT) as ancillary data
	// so the readLoop can build RxMeta for the codec's GTSM check.
	if perr := unix.SetsockoptInt(rxFD, unix.IPPROTO_IPV6, unix.IPV6_RECVPKTINFO, 1); perr != nil {
		return fmt.Errorf("vrrp/transport: setsockopt IPV6_RECVPKTINFO: %w", perr)
	}
	if herr := unix.SetsockoptInt(rxFD, unix.IPPROTO_IPV6, unix.IPV6_RECVHOPLIMIT, 1); herr != nil {
		return fmt.Errorf("vrrp/transport: setsockopt IPV6_RECVHOPLIMIT: %w", herr)
	}

	txFD, err := openRawIP(unix.AF_INET6, li.macvlanName)
	if err != nil {
		return err
	}
	li.txFD = txFD
	// RFC 9568 Section 5.1.2.3: Hop Limit MUST be 255 (holo bug 13 negative).
	if herr := unix.SetsockoptInt(txFD, unix.IPPROTO_IPV6, unix.IPV6_MULTICAST_HOPS, 255); herr != nil {
		return fmt.Errorf("vrrp/transport: setsockopt IPV6_MULTICAST_HOPS: %w", herr)
	}
	// Traffic class 0xc0 (CS6) mirrors the v4 TOS; loopback off; kernel computes the
	// VRRP checksum at offset 6 (RFC 9568 Section 5.2.8, IPV6_CHECKSUM).
	if terr := unix.SetsockoptInt(txFD, unix.IPPROTO_IPV6, unix.IPV6_TCLASS, 0xc0); terr != nil {
		return fmt.Errorf("vrrp/transport: setsockopt IPV6_TCLASS: %w", terr)
	}
	if lerr := unix.SetsockoptInt(txFD, unix.IPPROTO_IPV6, unix.IPV6_MULTICAST_LOOP, 0); lerr != nil {
		return fmt.Errorf("vrrp/transport: setsockopt IPV6_MULTICAST_LOOP: %w", lerr)
	}
	if cerr := unix.SetsockoptInt(txFD, unix.IPPROTO_IPV6, unix.IPV6_CHECKSUM, 6); cerr != nil {
		return fmt.Errorf("vrrp/transport: setsockopt IPV6_CHECKSUM: %w", cerr)
	}

	annFD, err := openNASocket(li.macvlanName)
	if err != nil {
		return err
	}
	li.annFD = annFD
	return nil
}

// openRawIP opens an AF_INET/AF_INET6 SOCK_RAW proto-112 socket bound to bindDev.
func openRawIP(family int, bindDev string) (int, error) {
	fd, err := unix.Socket(family, unix.SOCK_RAW, int(packet.ProtoNumber))
	if err != nil {
		return -1, fmt.Errorf("vrrp/transport: socket(SOCK_RAW, proto 112) needs CAP_NET_RAW: %w", err)
	}
	if berr := unix.SetsockoptString(fd, unix.SOL_SOCKET, unix.SO_BINDTODEVICE, bindDev); berr != nil {
		closeFD(fd)
		return -1, fmt.Errorf("vrrp/transport: bind socket to %s: %w", bindDev, berr)
	}
	return fd, nil
}

func setRcvTimeout(fd int) error {
	tv := unix.Timeval{Sec: int64(rcvTimeout / time.Second), Usec: int64((rcvTimeout % time.Second) / time.Microsecond)}
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
		return fmt.Errorf("vrrp/transport: setsockopt SO_RCVTIMEO: %w", err)
	}
	return nil
}

// SendAdvert transmits a prepared advertisement. IPv4 sends the full IP_HDRINCL
// datagram to 224.0.0.18; IPv6 sends the VRRP message with an IPV6_PKTINFO cmsg
// pinning the macvlan link-local source to ff02::12. A v6 send with no link-local
// yet returns ErrNoLinkLocal (the DAD window; the FSM retries).
func (li *linuxInstance) SendAdvert(frame []byte) error {
	li.sendMu.Lock()
	defer li.sendMu.Unlock()
	if li.family == packet.V6 {
		src, ok := li.v6SourceLocked()
		if !ok {
			return ErrNoLinkLocal
		}
		oob := pktinfoOOB(src.As16(), li.macvlanIf)
		sa := &unix.SockaddrInet6{Addr: packet.MulticastV6.As16(), ZoneId: uint32(li.macvlanIf)}
		if err := unix.Sendmsg(li.txFD, frame, oob, sa, 0); err != nil {
			if errors.Is(err, unix.EINVAL) {
				// The kernel EINVALs a pktinfo source that is no longer a valid
				// non-tentative address on the egress device (removed, DAD-cycled,
				// regenerated). Drop the cache and report no-link-local so the
				// next FSM tick re-resolves and retries (R-2 / AC-10 semantics).
				li.v6Src = netip.Addr{}
				return ErrNoLinkLocal
			}
			return fmt.Errorf("vrrp/transport: v6 advert sendmsg: %w", err)
		}
		return nil
	}
	sa := &unix.SockaddrInet4{Addr: packet.MulticastV4.As4()}
	if err := unix.Sendto(li.txFD, frame, 0, sa); err != nil {
		return fmt.Errorf("vrrp/transport: v4 advert sendto: %w", err)
	}
	return nil
}

// SendAnnounce transmits a prepared announcement frame on the family-appropriate
// announce socket.
func (li *linuxInstance) SendAnnounce(frame []byte) error {
	li.sendMu.Lock()
	defer li.sendMu.Unlock()
	if li.family == packet.V6 {
		return li.sendNALocked(frame)
	}
	return li.sendGARPLocked(frame)
}

// v6SourceLocked returns the cached macvlan link-local, resolving it on first use.
// li.sendMu must be held.
func (li *linuxInstance) v6SourceLocked() (netip.Addr, bool) {
	if li.v6Src.IsValid() {
		return li.v6Src, true
	}
	src, ok := macvlanLinkLocal(li.macvlanName)
	if ok {
		li.v6Src = src
	}
	return src, ok
}

// Transport.AnnounceMaster reaches warmV6Source through a comma-ok assertion,
// which cannot report a break: when 3c9644e15 unexported the method and left
// the interface declaring the exported name, nothing satisfied v6SourceWarmer,
// the assertion stopped matching and the warm silently never ran. This check
// turns that same mistake into a build error.
var _ v6SourceWarmer = (*linuxInstance)(nil)

// warmV6Source resolves and caches the macvlan link-local on the caller's
// goroutine (v6SourceWarmer). The orchestrator calls it at AnnounceMaster so the
// announcer worker never performs the netlink resolution itself: netlink sockets
// are created in the calling thread's network namespace, and the worker thread's
// namespace is not guaranteed to be the instance's (QEMU netns tests; Mistake
// Log). A still-tentative or absent link-local stays unresolved; the send path
// then skips and counts {reason=no-link-local}.
func (li *linuxInstance) warmV6Source() {
	li.sendMu.Lock()
	defer li.sendMu.Unlock()
	li.v6SourceLocked()
}

// Close stops the readLoop and closes all sockets. Idempotent.
func (li *linuxInstance) Close() error {
	var err error
	li.closeOnce.Do(func() {
		close(li.stop)
		<-li.done // readLoop self-wakes within rcvTimeout and exits
		err = li.closeFDs()
	})
	return err
}

func (li *linuxInstance) closeFDs() error {
	var first error
	for _, fd := range []int{li.rxFD, li.txFD, li.annFD} {
		if fd < 0 {
			continue
		}
		if cerr := unix.Close(fd); cerr != nil && first == nil {
			first = cerr
		}
	}
	return first
}

func (li *linuxInstance) stopped() bool {
	select {
	case <-li.stop:
		return true
	default:
		return false
	}
}

// readLoop dispatches to the family-specific receive loop.
func (li *linuxInstance) readLoop() {
	defer close(li.done)
	if li.family == packet.V6 {
		li.readLoopV6()
		return
	}
	li.readLoopV4()
}

// readLoopV4 reads proto-112 datagrams (WITH the IPv4 header, raw AF_INET),
// strips the header IHL-aware (spec-vrrp-1), stamps the parent ifindex, copies the
// payload once, and delivers it drop-on-overflow.
func (li *linuxInstance) readLoopV4() {
	var buf [rxBufLen]byte
	for {
		if li.stopped() {
			return
		}
		n, _, err := unix.Recvfrom(li.rxFD, buf[:], 0)
		if err != nil {
			if recvRetry(err) {
				continue
			}
			return
		}
		payload, meta, serr := packet.StripIPv4Header(buf[:n])
		if serr != nil {
			li.sink.counters.packetError(reasonMalformedIPv4)
			continue
		}
		meta.IfIndex = li.parentIf
		cp := make([]byte, len(payload))
		copy(cp, payload)
		li.sink.deliver(RxItem{Meta: meta, Payload: cp})
	}
}

// readLoopV6 reads proto-112 payloads (raw AF_INET6 delivers no IPv6 header) plus
// the hop limit and dst from ancillary data, and the src from the sender address.
func (li *linuxInstance) readLoopV6() {
	var buf [rxBufLen]byte
	var oob [oobBufLen]byte
	for {
		if li.stopped() {
			return
		}
		n, oobn, _, from, err := unix.Recvmsg(li.rxFD, buf[:], oob[:], 0)
		if err != nil {
			if recvRetry(err) {
				continue
			}
			return
		}
		ttl, dst := parseV6Ancillary(oob[:oobn])
		var src netip.Addr
		if sa, ok := from.(*unix.SockaddrInet6); ok {
			src = netip.AddrFrom16(sa.Addr)
		}
		meta := packet.RxMeta{TTL: ttl, Src: src, Dst: dst, Family: packet.V6, IfIndex: li.parentIf}
		cp := make([]byte, n)
		copy(cp, buf[:n])
		li.sink.deliver(RxItem{Meta: meta, Payload: cp})
	}
}

// recvRetry reports whether a Recvfrom/Recvmsg error is a transient wakeup
// (SO_RCVTIMEO / signal) that should retry; a closed-socket error returns false
// so the loop exits.
func recvRetry(err error) bool {
	if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
		return true
	}
	if errors.Is(err, unix.EBADF) || errors.Is(err, unix.EINVAL) {
		return false
	}
	return true
}

// parseV6Ancillary extracts the hop limit (IPV6_HOPLIMIT) and destination
// (IPV6_PKTINFO) from a recvmsg control blob. Linux delivers the hop limit as a
// native-endian 32-bit int.
func parseV6Ancillary(oob []byte) (ttl uint8, dst netip.Addr) {
	if len(oob) == 0 {
		return 0, netip.Addr{}
	}
	msgs, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return 0, netip.Addr{}
	}
	for _, m := range msgs {
		if m.Header.Level != unix.IPPROTO_IPV6 {
			continue
		}
		switch m.Header.Type {
		case unix.IPV6_HOPLIMIT:
			if len(m.Data) >= 4 {
				ttl = uint8(binary.NativeEndian.Uint32(m.Data[:4]))
			}
		case unix.IPV6_PKTINFO:
			if len(m.Data) >= 16 {
				var a [16]byte
				copy(a[:], m.Data[:16])
				dst = netip.AddrFrom16(a)
			}
		}
	}
	return ttl, dst
}

// pktinfoOOB builds an IPV6_PKTINFO control message pinning the send source
// address and egress ifindex. The unsafe overlay of the cmsg header/data is the
// standard idiom (arch-correct via unix.Cmsghdr.SetLen).
func pktinfoOOB(src [16]byte, ifindex int) []byte {
	oob := make([]byte, unix.CmsgSpace(unix.SizeofInet6Pktinfo))
	h := (*unix.Cmsghdr)(unsafe.Pointer(&oob[0])) //nolint:gosec // cmsg header overlay (standard cmsg idiom)
	h.Level = unix.IPPROTO_IPV6
	h.Type = unix.IPV6_PKTINFO
	h.SetLen(unix.CmsgLen(unix.SizeofInet6Pktinfo))
	pi := (*unix.Inet6Pktinfo)(unsafe.Pointer(&oob[unix.CmsgLen(0)])) //nolint:gosec // cmsg data overlay
	pi.Addr = src
	pi.Ifindex = uint32(ifindex)
	return oob
}

// macvlanLinkLocal returns the macvlan device's usable IPv6 link-local address,
// if present. RFC 9568 Section 7.2: the v6 advert / NA source is the sending
// interface's link-local. It goes through the iface resolver seam because
// AddrInfo carries the DAD state: a TENTATIVE link-local (IFA_F_TENTATIVE, DAD
// still running right after device creation) is NOT usable -- the kernel rejects
// a tentative IPV6_PKTINFO source with EINVAL (ip6_datagram_send_ctl requires a
// non-tentative assigned address) -- so it is skipped until DAD completes (R-2;
// QEMU-observed, Mistake Log).
func macvlanLinkLocal(name string) (netip.Addr, bool) {
	addrs, err := resolveIfaceAddresses(name)
	if err != nil {
		return netip.Addr{}, false
	}
	for _, a := range addrs {
		if a.Family != "" && a.Family != "ipv6" {
			continue
		}
		if !a.LinkLocal || a.Tentative {
			continue
		}
		ip, perr := netip.ParseAddr(a.Address)
		if perr == nil && ip.Is6() {
			return ip, true
		}
	}
	return netip.Addr{}, false
}

func closeFD(fd int) {
	if err := unix.Close(fd); err != nil {
		logger().Warn("vrrp/transport: close fd", "err", err)
	}
}
