// VALIDATES: RFC 4301 Section 4.1 multicast SAs on the path where Ze installs them, the
// RFC 4552 manually keyed OSPFv3 SAs for ff02::5 and ff02::6. The installer runs over the
// real XFRM backend in its own user and network namespace, and the kernel's SAD lookup is
// then asked for each group by SPI and destination.
// PREVENTS: a multicast SA the inbound lookup cannot find by its group, two group SAs that
// share their SPI (RFC 4552 Section 7) collapsing into one, and a group SA whose source
// address is pinned so that a neighbor's datagram maps to nothing.

//go:build linux

package ospf

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

const (
	// mcastSAChildEnv marks the re-executed test binary that runs inside the namespace.
	mcastSAChildEnv = "ZE_OSPF_MCAST_SA_PROBE_CHILD"
	// mcastSASPI is the manual SPI the interface is configured with. RFC 4552 Section 7
	// gives both groups and the unicast destinations this one SPI.
	mcastSASPI = 0x4301f1
	// mcastSAStraySPI is an SPI no interface is configured with.
	mcastSAStraySPI = 0x4301f2
	// mcastSAInterface is the interface the probe protects: loopback, the one link a fresh
	// namespace has.
	mcastSAInterface = "lo"
)

// mcastSAOwnNamespace runs the calling test again in a new user and network namespace, and
// returns true only in that child, where the body runs.
func mcastSAOwnNamespace(t *testing.T) bool {
	t.Helper()
	if os.Getenv(mcastSAChildEnv) == "1" {
		return true
	}
	args := []string{"-test.run", "^" + t.Name() + "$", "-test.v", "-test.count=1"}
	// A coverage run hands the binary a counter directory; passing it on lets the code the
	// child runs reach the profile `./le rfc discriminate-record` reads.
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-test.gocoverdir=") {
			args = append(args, arg)
		}
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], args...)
	cmd.Env = append(os.Environ(), mcastSAChildEnv+"=1")
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

// mcastSAInstall protects loopback with one manually keyed ESP interface through the
// installer's own path (onInterfaceUp, buildIPsecInterfaceSAs, the XFRM backend), exactly
// as an OSPFv3 interface comes up. The installer closes when the test ends.
func mcastSAInstall(t *testing.T) {
	t.Helper()
	lo, err := netlink.LinkByName(mcastSAInterface)
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatalf("lo up: %v", err)
	}
	if err := dataplane.Load("xfrm"); err != nil {
		t.Skipf("xfrm backend load: %v", err)
	}
	inst := newIPsecInstaller(nil, nil)
	t.Cleanup(inst.Close)
	inst.setTransportSource(func(string) (netip.Addr, int, bool) {
		return netip.MustParseAddr("fe80::1"), lo.Attrs().Index, true
	})
	inst.setConfig([]interfaceConfig{{
		Name:  mcastSAInterface,
		IPsec: &ipsecInterfaceConfig{SPI: mcastSASPI, Protocol: "esp", AuthAlgo: "sha256", AuthKey: hexKey(32)},
	}})
	inst.onInterfaceUp(lo.Attrs().Index, mcastSAInterface)
	if _, ok := inst.status(mcastSAInterface); !ok {
		t.Fatal("the installer recorded no installed SA for the interface")
	}
}

// mcastSALookup asks the kernel SAD for the ESP state keyed by (destination, SPI). The
// XFRM_MSG_GETSA handler resolves it with xfrm_state_lookup, the function XFRM input uses
// to map a received ESP datagram to its SA, and that lookup reads no source address.
func mcastSALookup(dst netip.Addr, spi uint32) (*netlink.XfrmState, error) {
	return netlink.XfrmStateGet(&netlink.XfrmState{
		Dst:   dst.AsSlice(),
		Spi:   int(spi),
		Proto: netlink.XFRM_PROTO_ESP,
	})
}

// TestRFC4301MulticastSAsAreMappedBySPIAndGroup proves each OSPFv3 multicast group Ze
// protects has its own SA in the kernel SAD, found by the SPI and the group address the
// datagram carries, although both groups share the one manual SPI. Method: install the
// interface in a namespace, then look each group up by (group, SPI) and compare the state
// returned with the group asked for.
func TestRFC4301MulticastSAsAreMappedBySPIAndGroup(t *testing.T) {
	// RFC requirement: RFC4301-4.1-6 positive -- the manually keyed SAs for ff02::5 and ff02::6 share one SPI, and the kernel SAD lookup by SPI and destination returns, for each group, the SA whose destination is that group.
	if !mcastSAOwnNamespace(t) {
		return
	}
	mcastSAInstall(t)
	for _, group := range []netip.Addr{ospfv3transport.AllSPFRouters, ospfv3transport.AllDRouters} {
		state, err := mcastSALookup(group, mcastSASPI)
		if err != nil {
			t.Fatalf("SAD lookup (%s, spi %#x): %v", group, mcastSASPI, err)
		}
		got, ok := netip.AddrFromSlice(state.Dst)
		if !ok {
			t.Fatalf("SAD lookup (%s): state destination %v is not an address", group, state.Dst)
		}
		if got.Unmap() != group {
			t.Errorf("SAD lookup (%s, spi %#x) returned the SA for %s", group, mcastSASPI, got)
		}
	}
}

// TestRFC4301MulticastSALookupRefusesAnUnconfiguredSPIOrGroup proves a datagram the manual
// configuration did not key maps to no SA: one addressed to a protected group under an SPI
// no interface uses, and one carrying the configured SPI to a group Ze did not protect.
// Method: install the interface in a namespace, then look both up in the kernel SAD, after
// a control lookup proves the installed SA itself is found.
func TestRFC4301MulticastSALookupRefusesAnUnconfiguredSPIOrGroup(t *testing.T) {
	// RFC requirement: RFC4301-4.1-6 negative -- with the group SAs installed, the kernel SAD lookup finds no SA for ff02::5 under an SPI that was not configured, and none for the configured SPI addressed to ff02::1, a group no SA was keyed to.
	if !mcastSAOwnNamespace(t) {
		return
	}
	mcastSAInstall(t)
	if _, err := mcastSALookup(ospfv3transport.AllSPFRouters, mcastSASPI); err != nil {
		t.Fatalf("control: the configured SA for ff02::5 is not found: %v", err)
	}
	if state, err := mcastSALookup(ospfv3transport.AllSPFRouters, mcastSAStraySPI); err == nil {
		t.Errorf("ff02::5 under unconfigured spi %#x mapped to the SA for %v", mcastSAStraySPI, state.Dst)
	}
	allNodes := netip.MustParseAddr("ff02::1")
	if state, err := mcastSALookup(allNodes, mcastSASPI); err == nil {
		t.Errorf("ff02::1 under spi %#x mapped to the SA for %v", mcastSASPI, state.Dst)
	}
}

// TestRFC4301MulticastSASourceMatchingIsSetByManualConfiguration proves the manual SA
// configuration is what decides that a multicast SA is found without matching the source:
// every group SA the installer puts in the kernel carries the unspecified source, so the
// SPI and the group alone map a neighbor's datagram to it. Method: install the interface in
// a namespace and read each group's state back from the kernel.
func TestRFC4301MulticastSASourceMatchingIsSetByManualConfiguration(t *testing.T) {
	// RFC requirement: RFC4301-4.1-12 positive -- the SA the manual configuration installs for ff02::5 and for ff02::6 is found in the kernel by SPI and group and carries the unspecified source address, so no source address matching is required to map inbound traffic to it.
	if !mcastSAOwnNamespace(t) {
		return
	}
	mcastSAInstall(t)
	for _, group := range []netip.Addr{ospfv3transport.AllSPFRouters, ospfv3transport.AllDRouters} {
		state, err := mcastSALookup(group, mcastSASPI)
		if err != nil {
			t.Fatalf("SAD lookup (%s, spi %#x): %v", group, mcastSASPI, err)
		}
		if !state.Src.Equal(net.IPv6zero) {
			t.Errorf("SA for %s carries source %v, want the unspecified address: a pinned source would demand source matching", group, state.Src)
		}
	}
}
