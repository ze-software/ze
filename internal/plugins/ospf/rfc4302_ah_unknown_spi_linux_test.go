// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- OSPFv3 manual-key AH SAs installed into XFRM.
// Related: ipsec_install.go -- buildIPsecInterfaceSAs, the states this probe installs.
// Related: ipsec_rfc4302_test.go -- the unit tests over the SA and policy Ze builds.
//
// VALIDATES: against a real Linux XFRM stack, RFC 4302 Section 3.4.2 on the AH SAs the OSPF
// installer puts in the SAD: an AH packet addressed to the router whose SPI matches no SA
// is discarded and counted (XfrmInNoStates, the auditable event), while the same packet
// under the SPI Ze installed finds its SA and fails there on the ICV instead.
// PREVENTS: an OSPFv3 AH packet with an unknown SPI reaching OSPF, or being dropped with no
// record of the event.
//
// No privilege is needed: ahProbeOwnNamespace re-runs the unit in a user and network
// namespace of its own, where the unit is root and every XFRM change stays private.

//go:build linux

package ospf

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

const (
	ahProbeChildEnv = "ZE_OSPF_RFC4302_AH_PROBE"
	ahProbeLink     = "ospfah0"
	ahProbeSPI      = 256
	ahProbeStat     = "XfrmInNoStates"
	ahProbeICVStat  = "XfrmInStateProtoError"
	ahProbeProtoAH  = 51
)

var ahProbeLocal = netip.MustParseAddr("fe80::1")

// ahProbeOwnNamespace runs the calling test again in a new user and network namespace, and
// returns true only in that child, where the body runs.
func ahProbeOwnNamespace(t *testing.T) bool {
	t.Helper()
	if os.Getenv(ahProbeChildEnv) == "1" {
		return true
	}
	args := []string{"-test.run", "^" + t.Name() + "$", "-test.v", "-test.count=1"}
	// A coverage run hands the binary a counter directory; passing it on lets the code the
	// child runs appear in the profile `./le rfc discriminate-record` reads.
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-test.gocoverdir=") {
			args = append(args, arg)
		}
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], args...)
	cmd.Env = append(os.Environ(), ahProbeChildEnv+"=1")
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

// ahProbeSetup brings up a dummy link carrying fe80::1 and installs the OSPF AH SAs for it
// through the installer's own path over the real XFRM backend. It returns the ifindex.
func ahProbeSetup(t *testing.T) int {
	t.Helper()
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatalf("lo up: %v", err)
	}
	link := &netlink.Dummy{}
	link.Name = ahProbeLink
	if err := netlink.LinkAdd(link); err != nil {
		t.Skipf("dummy link unavailable: %v", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("%s up: %v", ahProbeLink, err)
	}
	addr, err := netlink.ParseAddr(ahProbeLocal.String() + "/64")
	if err != nil {
		t.Fatalf("parse %s: %v", ahProbeLocal, err)
	}
	addr.Flags = syscall.IFA_F_NODAD
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatalf("add %s: %v", ahProbeLocal, err)
	}
	ifindex := link.Attrs().Index

	inst := newIPsecInstaller(nil, nil)
	t.Cleanup(func() {
		if err := dataplane.CloseBackend(); err != nil {
			t.Logf("close xfrm backend: %v", err)
		}
	})
	inst.setTransportSource(func(string) (netip.Addr, int, bool) { return ahProbeLocal, ifindex, true })
	inst.setConfig([]interfaceConfig{ahIface(ahProbeSPI)})
	inst.onInterfaceUp(ifindex, "eth1")
	if _, ok := inst.status("eth1"); !ok {
		t.Fatal("the installer reports no IPsec status for the interface")
	}
	states, err := netlink.XfrmStateList(netlink.FAMILY_V6)
	if err != nil {
		t.Fatalf("list xfrm states: %v", err)
	}
	for index := range states {
		if states[index].Spi == ahProbeSPI && states[index].Proto == netlink.XFRM_PROTO_AH && states[index].Dst.Equal(ahProbeLocal.AsSlice()) {
			return ifindex
		}
	}
	t.Fatalf("no AH state for SPI %d to %s among %d installed states", ahProbeSPI, ahProbeLocal, len(states))
	return 0
}

// ahProbeSend sends one AH packet to fe80::1 under spi, carrying an OSPF (89) payload and
// an ICV of zeros sized for HMAC-SHA-256-128, padded to the 8-octet IPv6 alignment.
func ahProbeSend(t *testing.T, ifindex int, spi uint32) {
	t.Helper()
	fd, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_RAW, ahProbeProtoAH)
	if err != nil {
		t.Fatalf("raw AH socket: %v", err)
	}
	defer syscall.Close(fd) //nolint:errcheck // Test socket; nothing to recover on close.

	packet := make([]byte, 32+16)
	packet[0] = 89 // Next Header: OSPF
	packet[1] = 32/4 - 2
	binary.BigEndian.PutUint32(packet[4:8], spi)
	binary.BigEndian.PutUint32(packet[8:12], 1) // Sequence Number
	to := &syscall.SockaddrInet6{Addr: ahProbeLocal.As16(), ZoneId: uint32(ifindex)}
	if err := syscall.Sendto(fd, packet, 0, to); err != nil {
		t.Fatalf("send AH to %s: %v", ahProbeLocal, err)
	}
}

// ahProbeCounter reads one counter out of the namespace's own /proc/net/xfrm_stat.
func ahProbeCounter(t *testing.T, name string) int {
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

// ahProbeListen opens a raw OSPF (89) socket, the one an OSPFv3 router reads, so a probe
// can tell whether a packet reached OSPF.
func ahProbeListen(t *testing.T) int {
	t.Helper()
	fd, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_RAW, 89)
	if err != nil {
		t.Fatalf("raw OSPF socket: %v", err)
	}
	t.Cleanup(func() { syscall.Close(fd) }) //nolint:errcheck // Test socket; nothing to recover on close.
	tv := syscall.Timeval{Usec: 200_000}
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
		t.Fatalf("set receive timeout: %v", err)
	}
	return fd
}

// ahProbeReceived reports whether the OSPF socket received anything within its timeout.
func ahProbeReceived(fd int) bool {
	buf := make([]byte, 256)
	n, _, err := syscall.Recvfrom(fd, buf, 0)
	return err == nil && n > 0
}

func TestRFC4302UnknownSPIAHPacketIsDiscardedAndCounted(t *testing.T) {
	// Goal: RFC 4302 Section 3.4.2 on the SAD the OSPF installer fills. Method: install the
	// interface's AH SAs (SPI 256) into a private XFRM stack, send an AH packet to the
	// router under an SPI no SA holds, and read the kernel's record of it.
	// RFC requirement: RFC4302-3.4.2-1 positive -- an AH packet whose SPI matches no SA is
	// discarded (nothing reaches the OSPF socket) and the discard is recorded as an
	// auditable event (XfrmInNoStates rises by one).
	if !ahProbeOwnNamespace(t) {
		return
	}
	ifindex := ahProbeSetup(t)
	ospfSocket := ahProbeListen(t)

	before := ahProbeCounter(t, ahProbeStat)
	ahProbeSend(t, ifindex, ahProbeSPI^0x5a5a)
	if ahProbeReceived(ospfSocket) {
		t.Fatal("an AH packet whose SPI no SA holds reached the OSPF socket")
	}
	if got := ahProbeCounter(t, ahProbeStat) - before; got != 1 {
		t.Fatalf("%s rose by %d on an unknown SPI, want 1", ahProbeStat, got)
	}
}

func TestRFC4302InstalledSPIAHPacketFindsItsSA(t *testing.T) {
	// Goal: the discard above is the missing-SA rule, not a drop of every AH packet.
	// Method: the same packet under the SPI Ze installed finds its SA, so it is not counted
	// as a missing state; its zero ICV then fails verification with that SA.
	// RFC requirement: RFC4302-3.4.2-1 negative -- a packet for which a valid SA exists is
	// not discarded as SA-less: XfrmInNoStates does not move, and the SA's own ICV check
	// (XfrmInStateProtoError) is what rejects the forged packet.
	if !ahProbeOwnNamespace(t) {
		return
	}
	ifindex := ahProbeSetup(t)
	ospfSocket := ahProbeListen(t)

	noStates := ahProbeCounter(t, ahProbeStat)
	icvFailures := ahProbeCounter(t, ahProbeICVStat)
	ahProbeSend(t, ifindex, ahProbeSPI)
	if ahProbeReceived(ospfSocket) {
		t.Fatal("an AH packet with a forged ICV reached the OSPF socket")
	}
	if got := ahProbeCounter(t, ahProbeStat) - noStates; got != 0 {
		t.Fatalf("%s rose by %d under the installed SPI, want 0", ahProbeStat, got)
	}
	if got := ahProbeCounter(t, ahProbeICVStat) - icvFailures; got != 1 {
		t.Fatalf("%s rose by %d under the installed SPI, want 1", ahProbeICVStat, got)
	}
}
