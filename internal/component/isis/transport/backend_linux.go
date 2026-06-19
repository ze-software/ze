//go:build linux

// Design: plan/spec-isis-3-l2-transport.md -- Linux AF_PACKET/SOCK_RAW backend
// Related: frame.go -- 802.3 + LLC frame build/parse
// Related: multicast.go -- ISO multicast groups joined on each circuit
//
// The Linux backend opens one AF_PACKET/SOCK_RAW socket per circuit, bound to
// the circuit's interface index. IS-IS frames are IEEE 802.3 (a length field,
// not an ethertype), so the socket binds with ETH_P_ALL rather than a registered
// ethertype: there is no ethertype to filter on. To receive the ISO multicast
// groups the circuit joins AllL1ISs / AllL2ISs / AllISs via PACKET_ADD_MEMBERSHIP
// (resolves spec assumption A-2 / risk R-2: raw multicast receive without
// promiscuous mode). SO_RCVTIMEO lets the RX goroutine wake periodically to
// observe the stop signal on link-down (risk R-3: no goroutine leak on flap).
//
// This mirrors the proven PPPoE AF_PACKET pattern
// (internal/component/pppoe/kernel_linux.go) but generalises the framing from a
// single ethertype to 802.3 + LLC and uses one socket per circuit (bound to the
// ifindex) instead of one shared discovery socket.

package transport

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// rcvTimeout bounds a blocking Recvfrom so the RX goroutine wakes to check its
// stop channel even when no frame arrives (R-3).
const rcvTimeout = 500 * time.Millisecond

// rxBufLen is the per-circuit receive buffer. Sized for a jumbo frame plus the
// 802.3 + LLC header so a maximally padded Hello fits without truncation.
const rxBufLen = 9000 + FrameHeaderLen

// htons converts a uint16 to network byte order (big-endian) for the AF_PACKET
// protocol field.
func htons(v uint16) uint16 { return (v<<8)&0xff00 | (v>>8)&0x00ff }

// linuxBackend opens AF_PACKET circuits.
type linuxBackend struct{}

// NewBackend returns the Linux AF_PACKET/SOCK_RAW backend.
func NewBackend() Backend { return linuxBackend{} }

func (linuxBackend) OpenCircuit(name string) (CircuitHandle, error) {
	ifindex, hwaddr, mtu, err := resolveInterface(name)
	if err != nil {
		return nil, err
	}

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		return nil, fmt.Errorf("isis/transport: socket(AF_PACKET) needs CAP_NET_RAW: %w", err)
	}

	// Bind to the specific interface so this socket only sees that circuit's
	// frames; dispatch by ifindex is still validated on receive.
	sa := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  ifindex,
	}
	if berr := unix.Bind(fd, sa); berr != nil {
		closeFD(fd)
		return nil, fmt.Errorf("isis/transport: bind(AF_PACKET): %w", berr)
	}

	// Join the ISO multicast groups so the kernel delivers them to this socket
	// without enabling promiscuous mode (A-2).
	for _, mac := range [][MACLen]byte{AllL1ISs, AllL2ISs, AllISs} {
		if jerr := joinMulticast(fd, ifindex, mac); jerr != nil {
			closeFD(fd)
			return nil, fmt.Errorf("isis/transport: join ISO multicast group: %w", jerr)
		}
	}

	// SO_RCVTIMEO so the RX goroutine wakes to observe the stop signal.
	tv := unix.Timeval{Sec: int64(rcvTimeout / time.Second), Usec: int64((rcvTimeout % time.Second) / time.Microsecond)}
	if serr := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); serr != nil {
		closeFD(fd)
		return nil, fmt.Errorf("isis/transport: setsockopt SO_RCVTIMEO: %w", serr)
	}

	c := &linuxCircuit{
		fd:      fd,
		ifindex: ifindex,
		hwaddr:  hwaddr,
		mtu:     mtu,
		recvCh:  make(chan RawFrame, 64),
		stop:    make(chan struct{}),
	}
	go c.readLoop()
	return c, nil
}

// linuxCircuit is one open AF_PACKET circuit.
type linuxCircuit struct {
	fd      int
	ifindex int
	hwaddr  [MACLen]byte
	mtu     int

	recvCh chan RawFrame
	stop   chan struct{}

	// sendMu serializes Send: the engine fans Hello, flood, and DIS/SNP sends at
	// each call SendPDU -> CircuitHandle.Send concurrently on the SAME circuit
	// (the transport orchestrator releases its own lock before calling
	// CircuitHandle.Send, see transport.go SendPDU). BuildFrame writes into the
	// shared sendBuf and Sendto reads it, so the lock MUST span both: it is held
	// across BuildFrame + Sendto so concurrent sends cannot interleave a torn
	// frame on the wire. Sendto copies the buffer before returning, so the lock
	// may be released once Sendto returns.
	sendMu sync.Mutex
	// sendBuf is reused across sends to avoid a per-frame allocation on the hot
	// path (buffer-first). Guarded by sendMu.
	sendBuf [rxBufLen]byte
}

func (c *linuxCircuit) IfIndex() int          { return c.ifindex }
func (c *linuxCircuit) HWAddr() [MACLen]byte  { return c.hwaddr }
func (c *linuxCircuit) MTU() int              { return c.mtu }
func (c *linuxCircuit) Recv() <-chan RawFrame { return c.recvCh }

// Send frames pdu with 802.3 + LLC into the reusable send buffer and transmits
// it to dst on this circuit's interface. The PDU is sent verbatim -- no padding,
// no alteration (umbrella "Final PDU bytes" contract).
func (c *linuxCircuit) Send(dst, src [MACLen]byte, pdu []byte) error {
	// Hold sendMu across BuildFrame + Sendto: the shared sendBuf is written by
	// BuildFrame and read by Sendto, and concurrent senders (Hello/flood/DIS)
	// would otherwise interleave and transmit a torn frame. Sendto copies before
	// returning, so the buffer is free for the next sender once Sendto returns.
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	n, err := BuildFrame(c.sendBuf[:], dst, src, pdu)
	if err != nil {
		return err
	}
	var addr [8]byte
	copy(addr[:], dst[:])
	sa := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  c.ifindex,
		Halen:    MACLen,
		Addr:     addr,
	}
	if serr := unix.Sendto(c.fd, c.sendBuf[:n], 0, sa); serr != nil {
		return fmt.Errorf("isis/transport: sendto: %w", serr)
	}
	return nil
}

// Close stops the RX goroutine and closes the socket. Closing the fd unblocks a
// pending Recvfrom.
func (c *linuxCircuit) Close() error {
	if !c.stopOnce() {
		return nil // already closed
	}
	return unix.Close(c.fd)
}

// stopOnce closes the stop channel exactly once, reporting whether this call did
// the closing (so Close only closes the fd on the first call).
func (c *linuxCircuit) stopOnce() bool {
	select {
	case <-c.stop:
		return false
	default:
		close(c.stop)
		return true
	}
}

// stopped reports whether the circuit has been asked to stop.
func (c *linuxCircuit) stopped() bool {
	select {
	case <-c.stop:
		return true
	default:
		return false
	}
}

// readLoop reads frames, validates the 802.3 + LLC header, and delivers the
// stripped PDU as a RawFrame. The PDU is COPIED out of the shared receive buffer
// before queueing so the engine may retain it. Frames that fail validation are
// dropped; only an accepted ISO multicast destination is delivered (level/area
// enforcement is isis-5).
func (c *linuxCircuit) readLoop() {
	defer close(c.recvCh)
	var buf [rxBufLen]byte
	for {
		if c.stopped() {
			return
		}
		n, from, err := unix.Recvfrom(c.fd, buf[:], 0)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
				continue // SO_RCVTIMEO wakeup or signal; re-check stop
			}
			if errors.Is(err, unix.EBADF) || errors.Is(err, unix.EINVAL) {
				return // socket closed
			}
			continue
		}
		f, perr := ParseFrame(buf[:n])
		if perr != nil {
			continue // crafted/short/ethertype frame: reject, do not deliver
		}
		// Only accept the ISO multicast destinations; ignore anything else the
		// bound socket happened to see.
		if !IsISMulticastMAC(f.DstMAC) {
			continue
		}
		ifindex := c.ifindex
		if sll, ok := from.(*unix.SockaddrLinklayer); ok && sll.Ifindex != 0 {
			ifindex = sll.Ifindex
		}
		pdu := make([]byte, len(f.PDU))
		copy(pdu, f.PDU)
		select {
		case c.recvCh <- RawFrame{IfIndex: ifindex, DstMAC: f.DstMAC, SrcMAC: f.SrcMAC, PDU: pdu}:
		case <-c.stop:
			return
		}
	}
}

// joinMulticast joins an ISO multicast group on the circuit's interface so the
// kernel delivers those frames without promiscuous mode.
func joinMulticast(fd, ifindex int, mac [MACLen]byte) error {
	mreq := unix.PacketMreq{
		Ifindex: int32(ifindex),
		Type:    unix.PACKET_MR_MULTICAST,
		Alen:    MACLen,
	}
	copy(mreq.Address[:], mac[:])
	return unix.SetsockoptPacketMreq(fd, unix.SOL_PACKET, unix.PACKET_ADD_MEMBERSHIP, &mreq)
}

// resolveInterface looks up an interface by name and returns its index, MAC, and
// MTU via ioctl, mirroring internal/component/pppoe/kernel_linux.go.
func resolveInterface(name string) (ifindex int, hwaddr [MACLen]byte, mtu int, err error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return 0, hwaddr, 0, fmt.Errorf("isis/transport: socket for ioctl(%s): %w", name, err)
	}
	defer closeFD(fd)

	ifr, err := unix.NewIfreq(name)
	if err != nil {
		return 0, hwaddr, 0, fmt.Errorf("isis/transport: ifreq(%s): %w", name, err)
	}
	if ierr := unix.IoctlIfreq(fd, unix.SIOCGIFINDEX, ifr); ierr != nil {
		return 0, hwaddr, 0, fmt.Errorf("isis/transport: SIOCGIFINDEX(%s): %w", name, ierr)
	}
	ifindex = int(ifr.Uint32())

	if ierr := unix.IoctlIfreq(fd, unix.SIOCGIFHWADDR, ifr); ierr != nil {
		return 0, hwaddr, 0, fmt.Errorf("isis/transport: SIOCGIFHWADDR(%s): %w", name, ierr)
	}
	// SIOCGIFHWADDR returns sockaddr{sa_family(2), sa_data(14)}; Ifreq has no
	// hwaddr accessor, so read the raw union past IFNAMSIZ and the 2-byte family.
	ifrUnion := (*[24]byte)(unsafe.Add(unsafe.Pointer(ifr), unix.IFNAMSIZ)) //nolint:gosec // ioctl data layout
	copy(hwaddr[:], ifrUnion[2:2+MACLen])

	if ierr := unix.IoctlIfreq(fd, unix.SIOCGIFMTU, ifr); ierr != nil {
		return 0, hwaddr, 0, fmt.Errorf("isis/transport: SIOCGIFMTU(%s): %w", name, ierr)
	}
	mtu = int(ifr.Uint32())

	return ifindex, hwaddr, mtu, nil
}

func closeFD(fd int) {
	if cerr := unix.Close(fd); cerr != nil {
		logger().Warn("isis/transport: close fd", "err", cerr)
	}
}
