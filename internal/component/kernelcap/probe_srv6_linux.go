//go:build linux

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Overview: probe.go -- the shared /proc root and the test override
// Related: probe_other.go -- the non-Linux counterpart returning StateUnknown
// Related: policy_linux.go -- stepFailedFor classifying EACCES under confining policies
//
// Behavioral probe for Linux SRv6 seg6 LWTUNNEL encapsulation
// (CONFIG_IPV6_SEG6_LWTUNNEL), forwarded inner TTL/Hop Limit decrement
// (CONFIG_IPV6_SEG6_FORWARD_TTL, gokrazy/kernel/patches/0014-seg6-forward-ttl.patch,
// RFC 8986 Section 5.1 S05), and forwarded DF-clear IPv4 pre-encapsulation
// fragmentation (CONFIG_IPV6_SEG6_FORWARD_FRAG,
// gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch).
//
// Inside a throwaway network and mount namespace, the probe enables forwarding
// on a single L3 TUN device, installs the same seg6 encap route that
// fib/kernel's buildSEG6Encap sends, writes one marked 44-byte IPv4 UDP packet
// into the TUN fd to check encap and inner TTL decrement, and when encap is
// present writes one 1460-byte DF-clear IPv4 UDP packet followed by a 44-byte
// sentinel packet on the same fd to verify inner IPv4 fragmentation before
// encapsulation. The first definitive verdict (present or absent) is memoized
// for the process life because CONFIG_IPV6_SEG6_LWTUNNEL and patches 0014 and
// 0015 are compiled in; unknown and denied verdicts are never kept.

package kernelcap

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	srv6ProbeTUNDevice      = "/dev/net/tun"
	srv6ProbeTUNName        = "zesrv6p0"
	srv6ProbeSentTTL        = 64
	srv6ProbeSrcPort        = 40000
	srv6ProbeDstPort        = 40001
	srv6ProbeMarker         = "ze-srv6-ttl-prbe"
	srv6ProbeFragMarker     = "ze-srv6-frg-prbe"
	srv6ProbeSentinelMarker = "ze-srv6-frg-done"
	srv6ProbeFragID         = 0x5678
	srv6ProbeFragSize       = 1460
	srv6ProbeMaxOuterLen    = 1500
	srv6ProbeDeadline       = time.Second
	srv6ProbeFrameBufSize   = 2048
	srv6ProbeSeg6Encap      = 1

	ipv4MinHeaderLen = 20
	ipv4FlagDF       = 0x4000
	ipv4FlagMF       = 0x2000
	ipv4FragOffMask  = 0x1fff
	ipv6HeaderLen    = 40
	srhMinHeaderLen  = 8
	srhRoutingType   = 4
	udpHeaderLen     = 8
)

var (
	srv6ProbeTUNV4Addr   = [4]byte{198, 51, 100, 1}
	srv6ProbeSrcV4Addr   = [4]byte{198, 51, 100, 2}
	srv6ProbeDstV4Net    = [4]byte{203, 0, 113, 0}
	srv6ProbeDstV4Addr   = [4]byte{203, 0, 113, 1}
	srv6ProbeTUNV6Addr   = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0xff, 0xff, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	srv6ProbeUnderlayNet = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0xff, 0xfe, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	srv6ProbeTargetSID   = [16]byte{0x20, 0x01, 0x0d, 0xb8, 0xff, 0xfe, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
)

var (
	errSRv6ForwardTTLUnchanged = errors.New("forwarded packets leave a seg6 headend with inner TTL unchanged " +
		"(no CONFIG_IPV6_SEG6_FORWARD_TTL: gokrazy/kernel/patches/0014-seg6-forward-ttl.patch is not applied)")
	errSRv6ForwardFragDropped = errors.New("forwarded oversized DF-clear IPv4 packets are dropped at a seg6 headend " +
		"instead of fragmented before encapsulation " +
		"(no CONFIG_IPV6_SEG6_FORWARD_FRAG: gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch is not applied)")
	errSRv6ProbeTimeout = errors.New("timed out waiting for the SRv6 probe packet on the throwaway TUN device")
)

type srv6IPv4Fragment struct {
	offset int
	more   bool
	data   []byte
}

var srv6ProbeMemo struct {
	mu          sync.Mutex
	cached      bool
	encap       Result
	forwardTTL  Result
	forwardFrag Result
}

// srv6ProbeRunner runs one uncached SRv6 probe. A var so unit tests drive
// memoization deterministically without namespaces or TUN devices.
var srv6ProbeRunner = runSRv6Probe

// srv6ProbeWritableSysctl makes /proc/sys writable inside the probe namespace.
// A var so unit tests drive policy denial and remount failures without
// CAP_SYS_ADMIN.
var srv6ProbeWritableSysctl = remountSysctlWritable

// srv6ProbeOpenTUN opens /dev/net/tun. A var so unit tests drive missing device
// nodes and permission refusals without host state.
var srv6ProbeOpenTUN = openSRv6ProbeTUN

// SRv6Encap reports whether the running kernel supports seg6 LWTUNNEL
// encapsulation routes (CONFIG_IPV6_SEG6_LWTUNNEL).
//
// Safe for concurrent use.
func SRv6Encap() Result {
	encap, _, _ := runSRv6ProbeMemoized()
	return encap
}

// SRv6ForwardTTL reports whether the running kernel decrements the inner IPv4
// TTL or IPv6 Hop Limit on forwarded packets entering a seg6 L3 encapsulation
// route (CONFIG_IPV6_SEG6_FORWARD_TTL, gokrazy/kernel/patches/0014-seg6-forward-ttl.patch).
//
// Safe for concurrent use.
func SRv6ForwardTTL() Result {
	_, forwardTTL, _ := runSRv6ProbeMemoized()
	return forwardTTL
}

// SRv6ForwardFrag reports whether the running kernel fragments forwarded
// DF-clear IPv4 packets exceeding the effective seg6 MTU before encapsulation
// (CONFIG_IPV6_SEG6_FORWARD_FRAG,
// gokrazy/kernel/patches/0015-seg6-forward-df-clear-frag.patch).
//
// Safe for concurrent use.
func SRv6ForwardFrag() Result {
	_, _, forwardFrag := runSRv6ProbeMemoized()
	return forwardFrag
}

// resetSRv6ProbeMemo clears the memoized SRv6 probe verdict for unit tests.
func resetSRv6ProbeMemo() {
	srv6ProbeMemo.mu.Lock()
	srv6ProbeMemo.cached = false
	srv6ProbeMemo.encap = Result{}
	srv6ProbeMemo.forwardTTL = Result{}
	srv6ProbeMemo.forwardFrag = Result{}
	srv6ProbeMemo.mu.Unlock()
}

// runSRv6ProbeMemoized returns the memoized definitive SRv6 verdict, or runs
// the probe once and stores the result when it is definitive. Unknown and
// denied verdicts are never cached.
func runSRv6ProbeMemoized() (Result, Result, Result) {
	srv6ProbeMemo.mu.Lock()
	defer srv6ProbeMemo.mu.Unlock()
	if srv6ProbeMemo.cached {
		return srv6ProbeMemo.encap, srv6ProbeMemo.forwardTTL, srv6ProbeMemo.forwardFrag
	}
	encap, forwardTTL, forwardFrag := srv6ProbeRunner()
	if isDefinitiveSRv6Verdict(encap, forwardTTL, forwardFrag) {
		srv6ProbeMemo.cached = true
		srv6ProbeMemo.encap = encap
		srv6ProbeMemo.forwardTTL = forwardTTL
		srv6ProbeMemo.forwardFrag = forwardFrag
	}
	return encap, forwardTTL, forwardFrag
}

// isDefinitiveSRv6Verdict reports whether the triple represents compiled-in
// kernel facts safe to memoize for the process life: all absent, or encap
// present with both forwardTTL and forwardFrag present or absent.
func isDefinitiveSRv6Verdict(encap, forwardTTL, forwardFrag Result) bool {
	if encap.State == StateAbsent {
		return forwardTTL.State == StateAbsent && forwardFrag.State == StateAbsent
	}
	if encap.State != StatePresent {
		return false
	}
	return isDefinitiveSingleState(forwardTTL.State) && isDefinitiveSingleState(forwardFrag.State)
}

func isDefinitiveSingleState(state State) bool {
	if state == StatePresent {
		return true
	}
	return state == StateAbsent
}

// runSRv6Probe executes askSRv6InThrowawayNamespace inside a fresh network
// namespace on a locked thread that exits when the probe finishes.
func runSRv6Probe() (Result, Result, Result) {
	var encap, forwardTTL, forwardFrag Result
	nsResult := InThrowawayNetworkNamespace(func() Result {
		encap, forwardTTL, forwardFrag = askSRv6InThrowawayNamespace()
		return encap
	})
	// When Unshare(CLONE_NEWNET) fails, the closure never runs and nsResult
	// holds the StateUnknown reason.
	if encap.State == StateUnspecified {
		return nsResult, nsResult, nsResult
	}
	return encap, forwardTTL, forwardFrag
}

func openSRv6ProbeTUN() (int, error) {
	return unix.Open(srv6ProbeTUNDevice, unix.O_RDWR|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
}

// askSRv6InThrowawayNamespace runs inside InThrowawayNetworkNamespace. It
// remounts /proc/sys writable in a private mount namespace, sets the three
// forwarding sysctls, creates a TUN device, installs a seg6 encap route,
// injects one marked 44-byte IPv4 UDP packet to classify inner IPv4 TTL, and
// when encap is present injects one 1460-byte DF-clear IPv4 UDP packet plus a
// 44-byte sentinel packet to classify pre-encapsulation IPv4 fragmentation.
func askSRv6InThrowawayNamespace() (Result, Result, Result) {
	sysctlPaths := srv6ProbeSysctlPaths()
	if err := srv6ProbeWritableSysctl(); err != nil {
		res := stepFailedFor(fmt.Errorf("make /proc/sys writable in the probe namespace: %w", err), sysctlPaths...)
		return res, res, res
	}
	sysctls := [3]struct {
		path  string
		value string
	}{
		{srv6IPv4ForwardPath(), "1"},
		{srv6IPv6ForwardPath(), "1"},
		{srv6IPv4SendRedirectsPath(), "0"},
	}
	for _, entry := range sysctls {
		if err := writeFile(entry.path, []byte(entry.value), 0o644); err != nil {
			res := stepFailedFor(fmt.Errorf("write %s in the probe namespace: %w", entry.path, err), sysctlPaths...)
			return res, res, res
		}
	}

	fd, err := srv6ProbeOpenTUN()
	if err != nil {
		res := classifySRv6OpenTUNError(err)
		return res, res, res
	}
	defer func() {
		_ = unix.Close(fd) //nolint:errcheck // closing a throwaway TUN fd during namespace teardown
	}()

	ifr, err := unix.NewIfreq(srv6ProbeTUNName)
	if err != nil {
		panic("BUG: SRv6 probe TUN name " + srv6ProbeTUNName + " does not fit an ifreq: " + err.Error())
	}
	ifr.SetUint16(unix.IFF_TUN | unix.IFF_NO_PI)
	if err := unix.IoctlIfreq(fd, unix.TUNSETIFF, ifr); err != nil {
		res := stepFailedFor(fmt.Errorf("create TUN device %s via TUNSETIFF: %w", srv6ProbeTUNName, err), sysctlPaths...)
		return res, res, res
	}

	handle, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		res := Result{State: StateUnknown, Reason: fmt.Errorf("open rtnetlink in the SRv6 probe namespace: %w", err)}
		return res, res, res
	}
	defer handle.Close()

	link, err := handle.LinkByName(srv6ProbeTUNName)
	if err != nil {
		res := Result{State: StateUnknown, Reason: fmt.Errorf("look up SRv6 probe TUN %s: %w", srv6ProbeTUNName, err)}
		return res, res, res
	}
	if err := handle.LinkSetUp(link); err != nil {
		res := Result{State: StateUnknown, Reason: fmt.Errorf("bring up SRv6 probe TUN %s: %w", srv6ProbeTUNName, err)}
		return res, res, res
	}

	tunV4 := srv6ProbeTUNV4Addr
	if err := handle.AddrAdd(link, &netlink.Addr{
		IPNet: &net.IPNet{IP: net.IP(tunV4[:]), Mask: net.CIDRMask(24, 32)},
	}); err != nil {
		res := Result{State: StateUnknown, Reason: fmt.Errorf("add IPv4 address to SRv6 probe TUN %s: %w", srv6ProbeTUNName, err)}
		return res, res, res
	}

	tunV6 := srv6ProbeTUNV6Addr
	if err := handle.AddrAdd(link, &netlink.Addr{
		IPNet: &net.IPNet{IP: net.IP(tunV6[:]), Mask: net.CIDRMask(64, 128)},
		Flags: unix.IFA_F_NODAD,
	}); err != nil {
		res := Result{State: StateUnknown, Reason: fmt.Errorf("add IPv6 address to SRv6 probe TUN %s: %w", srv6ProbeTUNName, err)}
		return res, res, res
	}

	dstV4 := srv6ProbeDstV4Net
	targetSID := srv6ProbeTargetSID
	seg6Route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       &net.IPNet{IP: net.IP(dstV4[:]), Mask: net.CIDRMask(24, 32)},
		Scope:     netlink.SCOPE_LINK,
		Encap: &netlink.SEG6Encap{
			Mode:     srv6ProbeSeg6Encap,
			Segments: []net.IP{net.IP(targetSID[:])},
		},
	}
	if err := handle.RouteAdd(seg6Route); err != nil {
		return classifySRv6RouteAddError(err)
	}

	underlayV6 := srv6ProbeUnderlayNet
	underlayRoute := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       &net.IPNet{IP: net.IP(underlayV6[:]), Mask: net.CIDRMask(64, 128)},
	}
	if err := handle.RouteAdd(underlayRoute); err != nil {
		res := Result{State: StateUnknown, Reason: fmt.Errorf("install SRv6 probe underlay route: %w", err)}
		return res, res, res
	}

	marker := []byte(srv6ProbeMarker)
	packet := buildSRv6ProbePacket(srv6ProbeSentTTL, marker)
	err = writeSRv6ProbePacket(fd, packet)
	if err != nil {
		res := Result{State: StateUnknown, Reason: fmt.Errorf("write SRv6 probe packet to %s: %w", srv6ProbeTUNName, err)}
		return res, res, res
	}

	encap, forwardTTL := pollSRv6ProbePacket(fd, srv6ProbeTargetSID, marker, srv6ProbeSentTTL, srv6ProbeDeadline)
	if encap.State != StatePresent {
		return encap, forwardTTL, encap
	}

	fragPacket := buildSRv6FragProbePacket(srv6ProbeSentTTL, srv6ProbeFragID, []byte(srv6ProbeFragMarker))
	sentinelMarker := []byte(srv6ProbeSentinelMarker)
	sentinelPacket := buildSRv6ProbePacket(srv6ProbeSentTTL, sentinelMarker)

	err = writeSRv6ProbePacket(fd, fragPacket)
	if err != nil {
		res := Result{State: StateUnknown, Reason: fmt.Errorf("write SRv6 DF-clear probe packet to %s: %w", srv6ProbeTUNName, err)}
		return encap, forwardTTL, res
	}
	err = writeSRv6ProbePacket(fd, sentinelPacket)
	if err != nil {
		res := Result{State: StateUnknown, Reason: fmt.Errorf("write SRv6 DF-clear sentinel packet to %s: %w", srv6ProbeTUNName, err)}
		return encap, forwardTTL, res
	}

	forwardFrag := pollSRv6FragProbe(
		fd,
		srv6ProbeTargetSID,
		fragPacket[ipv4MinHeaderLen:],
		sentinelMarker,
		srv6ProbeDeadline,
	)
	return encap, forwardTTL, forwardFrag
}

// classifySRv6OpenTUNError maps an error from opening /dev/net/tun. Missing
// device nodes answer StateUnknown naming /dev/net/tun; EACCES goes through
// stepFailedFor so a confining security policy answers StateDenied.
func classifySRv6OpenTUNError(err error) Result {
	wrapped := fmt.Errorf("open %s for the SRv6 probe: %w", srv6ProbeTUNDevice, err)
	if errors.Is(err, unix.ENOENT) {
		return Result{State: StateUnknown, Reason: wrapped}
	}
	if errors.Is(err, unix.ENODEV) {
		return Result{State: StateUnknown, Reason: wrapped}
	}
	if errors.Is(err, fs.ErrNotExist) {
		return Result{State: StateUnknown, Reason: wrapped}
	}
	if errors.Is(err, unix.EACCES) {
		return stepFailedFor(wrapped, srv6ProbeSysctlPaths()...)
	}
	return Result{State: StateUnknown, Reason: wrapped}
}

// classifySRv6RouteAddError maps a failed seg6 RouteAdd. EOPNOTSUPP from
// lwtunnel_valid_encap_type means the kernel lacks CONFIG_IPV6_SEG6_LWTUNNEL,
// so srv6-encap, srv6-forward-ttl, and srv6-forward-frag are all absent; any
// other error leaves all three unknown.
func classifySRv6RouteAddError(err error) (Result, Result, Result) {
	if errors.Is(err, unix.EOPNOTSUPP) {
		res := Result{
			State:  StateAbsent,
			Reason: fmt.Errorf("the kernel refused a seg6 LWTUNNEL encap route (no CONFIG_IPV6_SEG6_LWTUNNEL): %w", err),
		}
		return res, res, res
	}
	res := Result{
		State:  StateUnknown,
		Reason: fmt.Errorf("install SRv6 probe route: %w", err),
	}
	return res, res, res
}

// buildSRv6ProbePacket constructs one IPv4 UDP packet from 198.51.100.2:40000 to
// 203.0.113.1:40001 with ttl, a valid IPv4 header checksum, UDP checksum 0, and
// marker as payload.
func buildSRv6ProbePacket(ttl byte, marker []byte) []byte {
	totalLen := ipv4MinHeaderLen + udpHeaderLen + len(marker)
	pkt := make([]byte, totalLen)
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(totalLen)) //nolint:gosec // probe packet is 44 bytes, far below math.MaxUint16
	binary.BigEndian.PutUint16(pkt[4:6], 0x1234)
	pkt[8] = ttl
	pkt[9] = unix.IPPROTO_UDP
	copy(pkt[12:16], srv6ProbeSrcV4Addr[:])
	copy(pkt[16:20], srv6ProbeDstV4Addr[:])
	binary.BigEndian.PutUint16(pkt[10:12], ipv4HeaderChecksum(pkt[:ipv4MinHeaderLen]))

	udp := pkt[ipv4MinHeaderLen:]
	binary.BigEndian.PutUint16(udp[0:2], srv6ProbeSrcPort)
	binary.BigEndian.PutUint16(udp[2:4], srv6ProbeDstPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpHeaderLen+len(marker))) //nolint:gosec // probe UDP payload is 24 bytes
	binary.BigEndian.PutUint16(udp[6:8], 0)
	copy(udp[udpHeaderLen:], marker)
	return pkt
}

// buildSRv6FragProbePacket constructs one 1460-byte DF-clear IPv4 UDP packet
// from 198.51.100.2:40000 to 203.0.113.1:40001 with ttl, fragID, a valid IPv4
// header checksum, UDP checksum 0, and marker followed by deterministic
// non-uniform bytes as the UDP payload. On a 1500-byte MTU TUN device with
// 64-byte single-SID SRv6 headroom (effective MTU 1436), the packet exceeds the
// effective MTU by 24 bytes and must fragment into two inner IPv4 fragments.
func buildSRv6FragProbePacket(ttl byte, fragID uint16, marker []byte) []byte {
	pkt := make([]byte, srv6ProbeFragSize)
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], srv6ProbeFragSize)
	binary.BigEndian.PutUint16(pkt[4:6], fragID)
	pkt[8] = ttl
	pkt[9] = unix.IPPROTO_UDP
	copy(pkt[12:16], srv6ProbeSrcV4Addr[:])
	copy(pkt[16:20], srv6ProbeDstV4Addr[:])
	binary.BigEndian.PutUint16(pkt[10:12], ipv4HeaderChecksum(pkt[:ipv4MinHeaderLen]))

	udp := pkt[ipv4MinHeaderLen:]
	binary.BigEndian.PutUint16(udp[0:2], srv6ProbeSrcPort)
	binary.BigEndian.PutUint16(udp[2:4], srv6ProbeDstPort)
	binary.BigEndian.PutUint16(udp[4:6], srv6ProbeFragSize-ipv4MinHeaderLen)
	binary.BigEndian.PutUint16(udp[6:8], 0)

	payload := udp[udpHeaderLen:]
	copied := copy(payload, marker)
	for i := copied; i < len(payload); i++ {
		payload[i] = byte((i*31 + 17) & 0xff)
	}
	return pkt
}

func ipv4HeaderChecksum(header []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(header); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(header[i : i+2]))
	}
	for sum > 0xffff {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

func writeSRv6ProbePacket(fd int, packet []byte) error {
	for {
		n, err := unix.Write(fd, packet)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return err
		}
		if n != len(packet) {
			return fmt.Errorf("short write to TUN: wrote %d of %d bytes", n, len(packet))
		}
		return nil
	}
}

func pollSRv6ProbePacket(fd int, targetSID [16]byte, marker []byte, sentTTL byte, timeout time.Duration) (Result, Result) {
	deadline := time.Now().Add(timeout)
	var buf [srv6ProbeFrameBufSize]byte
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			res := Result{State: StateUnknown, Reason: errSRv6ProbeTimeout}
			return res, res
		}
		timeoutMs := int(remaining.Milliseconds())
		if timeoutMs <= 0 {
			timeoutMs = 1
		}
		fds := [1]unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}} //nolint:gosec // Linux file descriptors fit in int32
		ready, err := unix.Poll(fds[:], timeoutMs)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			res := Result{State: StateUnknown, Reason: fmt.Errorf("poll SRv6 probe TUN device: %w", err)}
			return res, res
		}
		if ready == 0 {
			continue
		}
		n, err := unix.Read(fd, buf[:])
		if err != nil {
			if errors.Is(err, unix.EAGAIN) {
				continue
			}
			if errors.Is(err, unix.EINTR) {
				continue
			}
			res := Result{State: StateUnknown, Reason: fmt.Errorf("read SRv6 probe TUN device: %w", err)}
			return res, res
		}
		innerTTL, matched := extractSRv6InnerTTL(buf[:n], targetSID, marker)
		if !matched {
			continue
		}
		return classifySRv6InnerTTL(innerTTL, sentTTL)
	}
}

// extractSRv6InnerTTL checks whether frame is an IPv6 packet addressed to
// targetSID carrying a Segment Routing Header (Routing Type 4, Next Header
// IPIP) encapsulating an IPv4 UDP packet whose payload begins with marker.
// Every slice access is bounded against len(frame) before indexing so any
// unsolicited or truncated frame (such as IPv6 router solicitations or MLD)
// is safely ignored.
func extractSRv6InnerTTL(frame []byte, targetSID [16]byte, marker []byte) (byte, bool) {
	if len(frame) < ipv6HeaderLen+srhMinHeaderLen {
		return 0, false
	}
	if frame[0]>>4 != 6 {
		return 0, false
	}
	if frame[6] != unix.IPPROTO_ROUTING {
		return 0, false
	}
	if !bytes.Equal(frame[24:40], targetSID[:]) {
		return 0, false
	}

	srh := frame[ipv6HeaderLen:]
	if srh[0] != unix.IPPROTO_IPIP {
		return 0, false
	}
	if srh[2] != srhRoutingType {
		return 0, false
	}
	srhLen := (int(srh[1]) + 1) * 8
	if len(srh) < srhLen {
		return 0, false
	}

	inner := srh[srhLen:]
	if len(inner) < ipv4MinHeaderLen {
		return 0, false
	}
	if inner[0]>>4 != 4 {
		return 0, false
	}
	ihl := int(inner[0]&0x0f) * 4
	if ihl < ipv4MinHeaderLen {
		return 0, false
	}
	if inner[9] != unix.IPPROTO_UDP {
		return 0, false
	}
	if len(inner) < ihl+udpHeaderLen+len(marker) {
		return 0, false
	}
	payload := inner[ihl+udpHeaderLen:]
	if !bytes.Equal(payload[:len(marker)], marker) {
		return 0, false
	}
	return inner[8], true
}

// classifySRv6InnerTTL turns the encapsulated inner IPv4 TTL into the verdicts
// for srv6-encap and srv6-forward-ttl.
//
// RFC 8986 Section 5.1: "S05.   Decrement inner IPv6 Hop Limit or IPv4 TTL"
func classifySRv6InnerTTL(innerTTL, sentTTL byte) (Result, Result) {
	if innerTTL == sentTTL-1 {
		return Result{State: StatePresent}, Result{State: StatePresent}
	}
	if innerTTL == sentTTL {
		return Result{State: StatePresent}, Result{
			State:  StateAbsent,
			Reason: errSRv6ForwardTTLUnchanged,
		}
	}
	var msg textbuf.Buffer
	msg.Str("SRv6 probe observed unexpected inner IPv4 TTL ").Uint8(innerTTL).
		Str(" for sent TTL ").Uint8(sentTTL)
	return Result{State: StatePresent}, Result{
		State:  StateUnknown,
		Reason: errors.New(msg.String()),
	}
}

// pollSRv6FragProbe reads encapsulated frames from fd until either two or more
// valid inner IPv4 fragments reassemble to expectedPayload (StatePresent), the
// trailing sentinel packet arrives before completion (StateAbsent), or timeout
// expires (StateUnknown).
func pollSRv6FragProbe(
	fd int,
	targetSID [16]byte,
	expectedPayload, sentinelMarker []byte,
	timeout time.Duration,
) Result {
	deadline := time.Now().Add(timeout)
	var buf [srv6ProbeFrameBufSize]byte
	reassembled := make([]byte, len(expectedPayload))
	seen := make([]bool, len(expectedPayload))
	fragCount := 0
	received := 0
	gotFinal := false

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return Result{State: StateUnknown, Reason: errSRv6ProbeTimeout}
		}
		timeoutMs := int(remaining.Milliseconds())
		if timeoutMs <= 0 {
			timeoutMs = 1
		}
		fds := [1]unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}} //nolint:gosec // Linux file descriptors fit in int32
		ready, err := unix.Poll(fds[:], timeoutMs)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return Result{State: StateUnknown, Reason: fmt.Errorf("poll SRv6 DF-clear probe TUN device: %w", err)}
		}
		if ready == 0 {
			continue
		}
		n, err := unix.Read(fd, buf[:])
		if err != nil {
			if errors.Is(err, unix.EAGAIN) {
				continue
			}
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return Result{State: StateUnknown, Reason: fmt.Errorf("read SRv6 DF-clear probe TUN device: %w", err)}
		}
		frame := buf[:n]

		_, isSentinel := extractSRv6InnerTTL(frame, targetSID, sentinelMarker)
		if isSentinel {
			return Result{State: StateAbsent, Reason: errSRv6ForwardFragDropped}
		}

		frag, ok := extractSRv6InnerIPv4Fragment(frame, targetSID, srv6ProbeFragID)
		if !ok {
			continue
		}
		end := frag.offset + len(frag.data)
		if end > len(expectedPayload) {
			continue
		}
		if !frag.more && end != len(expectedPayload) {
			continue
		}
		if !bytes.Equal(frag.data, expectedPayload[frag.offset:end]) {
			continue
		}
		if slices.Contains(seen[frag.offset:end], true) {
			continue
		}

		copy(reassembled[frag.offset:end], frag.data)
		for i := frag.offset; i < end; i++ {
			seen[i] = true
		}
		received += len(frag.data)
		fragCount++
		if !frag.more {
			gotFinal = true
		}
		if fragCount >= 2 && gotFinal && received == len(expectedPayload) && bytes.Equal(reassembled, expectedPayload) {
			return Result{State: StatePresent}
		}
	}
}

// extractSRv6InnerIPv4Fragment checks whether frame is an unfragmented outer
// IPv6+SRH packet (at most srv6ProbeMaxOuterLen bytes, Routing Type 4, Next
// Header IPIP) addressed to targetSID carrying a valid DF-clear inner IPv4 UDP
// fragment matching fragID and the probe endpoints. Every slice access is
// bounds-checked against len(frame) before indexing so unsolicited, truncated,
// or outer-fragmented frames are safely ignored.
func extractSRv6InnerIPv4Fragment(frame []byte, targetSID [16]byte, fragID uint16) (srv6IPv4Fragment, bool) {
	if len(frame) > srv6ProbeMaxOuterLen {
		return srv6IPv4Fragment{}, false
	}
	if len(frame) < ipv6HeaderLen+srhMinHeaderLen {
		return srv6IPv4Fragment{}, false
	}
	if frame[0]>>4 != 6 {
		return srv6IPv4Fragment{}, false
	}
	if frame[6] != unix.IPPROTO_ROUTING {
		return srv6IPv4Fragment{}, false
	}
	if !bytes.Equal(frame[24:40], targetSID[:]) {
		return srv6IPv4Fragment{}, false
	}

	ipv6PayloadLen := int(binary.BigEndian.Uint16(frame[4:6]))
	if ipv6PayloadLen < srhMinHeaderLen {
		return srv6IPv4Fragment{}, false
	}
	if ipv6HeaderLen+ipv6PayloadLen > len(frame) {
		return srv6IPv4Fragment{}, false
	}

	srh := frame[ipv6HeaderLen : ipv6HeaderLen+ipv6PayloadLen]
	if srh[0] != unix.IPPROTO_IPIP {
		return srv6IPv4Fragment{}, false
	}
	if srh[2] != srhRoutingType {
		return srv6IPv4Fragment{}, false
	}
	srhLen := (int(srh[1]) + 1) * 8
	if len(srh) < srhLen {
		return srv6IPv4Fragment{}, false
	}

	inner := srh[srhLen:]
	if len(inner) < ipv4MinHeaderLen {
		return srv6IPv4Fragment{}, false
	}
	if inner[0]>>4 != 4 {
		return srv6IPv4Fragment{}, false
	}
	ihl := int(inner[0]&0x0f) * 4
	if ihl < ipv4MinHeaderLen {
		return srv6IPv4Fragment{}, false
	}
	if len(inner) < ihl {
		return srv6IPv4Fragment{}, false
	}
	totalLen := int(binary.BigEndian.Uint16(inner[2:4]))
	if totalLen < ihl {
		return srv6IPv4Fragment{}, false
	}
	if totalLen > len(inner) {
		return srv6IPv4Fragment{}, false
	}
	if ipv4HeaderChecksum(inner[:ihl]) != 0 {
		return srv6IPv4Fragment{}, false
	}
	if binary.BigEndian.Uint16(inner[4:6]) != fragID {
		return srv6IPv4Fragment{}, false
	}
	if inner[9] != unix.IPPROTO_UDP {
		return srv6IPv4Fragment{}, false
	}
	if !bytes.Equal(inner[12:16], srv6ProbeSrcV4Addr[:]) {
		return srv6IPv4Fragment{}, false
	}
	if !bytes.Equal(inner[16:20], srv6ProbeDstV4Addr[:]) {
		return srv6IPv4Fragment{}, false
	}

	flagsFrag := binary.BigEndian.Uint16(inner[6:8])
	if flagsFrag&ipv4FlagDF != 0 {
		return srv6IPv4Fragment{}, false
	}
	more := flagsFrag&ipv4FlagMF != 0
	offset := int(flagsFrag&ipv4FragOffMask) * 8
	if offset == 0 && !more {
		return srv6IPv4Fragment{}, false
	}

	fragData := inner[ihl:totalLen]
	if len(fragData) == 0 {
		return srv6IPv4Fragment{}, false
	}
	if more && len(fragData)%8 != 0 {
		return srv6IPv4Fragment{}, false
	}

	return srv6IPv4Fragment{
		offset: offset,
		more:   more,
		data:   fragData,
	}, true
}
