//go:build integration && linux

// Design: docs/architecture/testing/interop.md -- SRv6 service-route packet proof.
// Related: vpp_srv6_integration_test.go -- native container and daemon lifecycle.
// Related: vpp_srv6_wire_integration_linux_test.go -- BGP and Ethernet boundaries.
// VPP API: https://github.com/FDio/vpp/blob/master/src/vnet/srv6/sr.api.
// VPP CLI: https://github.com/FDio/vpp/tree/master/src/vnet/srv6.
package testdeployment

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"go.fd.io/govpp/adapter/socketclient"
	"go.fd.io/govpp/binapi/sr"
	"go.fd.io/govpp/binapi/sr_types"
	"go.fd.io/govpp/core"
)

var vppSRv6Port = flag.Int("vpp-srv6-port", 0, "BGP listener port for the native VPP SRv6 probe")

const (
	vppSRv6Source  = "2001:db8:94::1"
	vppSRv6NextHop = "2001:db8:94::2"
	vppSRv6SIDOne  = "2001:db8:95::a"
	vppSRv6SIDTwo  = "2001:db8:95::b"
	vppSRv6Host    = "ze-sr-host"
	vppSRv6Link    = "ze-sr-vpp"
	vppSRv6HostMAC = "02:00:00:94:00:02"
	vppSRv6VPPMAC  = "02:00:00:94:00:01"
)

// TestVPPSRv6ServiceRouteProbe runs only inside the existing deployment lab.
// No SR policy or steering entry is installed by the probe: all such state MUST
// come from the BGP UPDATE -> RIB -> sysrib -> FIB production path.
// RFC 9252 Section 5: "When steering for SRv6 services is based on shortest path
// forwarding (e.g., best effort or IGP Flexible Algorithm [IGP-FLEX-ALGO]) to the
// egress PE, the ingress PE encapsulates the IPv4 or IPv6 customer packet in an
// outer IPv6 header (using H.Encaps or H.Encaps.Red flavors specified in
// [RFC8986]), where the destination address is the SRv6 Service SID associated
// with the related BGP route update."
func TestVPPSRv6ServiceRouteProbe(t *testing.T) {
	if *vppSRv6Port == 0 {
		t.Skip("launched by TestVPPSRv6ServiceRoute with a real VPP and daemon")
	}
	vppSRv6Underlay(t)
	conn, err := core.Connect(socketclient.NewVppClient("/run/vpp/api.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Disconnect()
	api := sr.NewServiceClient(conn)
	first := netip.MustParsePrefix("10.94.1.0/24")
	second := netip.MustParsePrefix("10.94.2.0/24")
	sidOne := netip.MustParseAddr(vppSRv6SIDOne)
	sidTwo := netip.MustParseAddr(vppSRv6SIDTwo)
	vppSRv6AwaitState(t, api, nil)
	fd, index := vppSRv6PacketSocket(t)

	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: *vppSRv6Port})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close() //nolint:errcheck // test teardown
	if err := listener.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatal(err)
	}
	t.Log("srv6 peer listening")
	peer, err := listener.AcceptTCP()
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close() //nolint:errcheck // test teardown
	// RFC 4271 Section 4.2 and RFC 8950 Section 4: negotiate before UPDATEs.
	vppSRv6Open(t, peer)
	connectedGeneration := vppSRv6AwaitConnected(t, "")

	// RFC 9252 Sections 3.1 and 5.3: the SID is carried on the service route.
	vppSRv6Update(t, peer, first, sidOne)
	vppSRv6Update(t, peer, second, sidOne)
	vppSRv6AwaitState(t, api, map[netip.Prefix]netip.Addr{first: sidOne, second: sidOne})
	// RFC 9252 Section 5: the packet, not an API acknowledgement, is the oracle.
	vppSRv6Packet(t, fd, index, first.Addr().Next(), sidOne, 1)
	vppSRv6Packet(t, fd, index, second.Addr().Next(), sidOne, 2)

	// A normal Ze restart must restore confirmed ownership of the still-live
	// external VPP. Relative identity is asserted, never a fixed BSID value.
	before := vppSRv6PolicyIDs(t, api)
	t.Log("srv6 ze restart requested")
	if err := listener.SetDeadline(time.Now().Add(40 * time.Second)); err != nil {
		t.Fatal(err)
	}
	restarted, err := listener.AcceptTCP()
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close() //nolint:errcheck // test teardown
	peer = restarted
	vppSRv6Open(t, peer)
	_ = vppSRv6AwaitConnected(t, connectedGeneration)
	vppSRv6Update(t, peer, first, sidOne)
	vppSRv6Update(t, peer, second, sidOne)
	vppSRv6AwaitState(t, api, map[netip.Prefix]netip.Addr{first: sidOne, second: sidOne})
	vppSRv6SamePolicyIDs(t, before, vppSRv6PolicyIDs(t, api))
	vppSRv6Packet(t, fd, index, first.Addr().Next(), sidOne, 11)
	vppSRv6Packet(t, fd, index, second.Addr().Next(), sidOne, 12)
	t.Log("srv6 Ze restart retained confirmed policy identity and forwarding")

	// Withdrawing one of two references MUST preserve the shared policy.
	vppSRv6Withdraw(t, peer, first)
	vppSRv6AwaitState(t, api, map[netip.Prefix]netip.Addr{second: sidOne})
	vppSRv6Packet(t, fd, index, first.Addr().Next(), netip.Addr{}, 9)
	vppSRv6Packet(t, fd, index, second.Addr().Next(), sidOne, 10)
	vppSRv6Update(t, peer, first, sidOne)
	vppSRv6AwaitState(t, api, map[netip.Prefix]netip.Addr{first: sidOne, second: sidOne})

	// Replacing one route MUST NOT release the old policy used by the other.
	vppSRv6Update(t, peer, first, sidTwo)
	vppSRv6AwaitState(t, api, map[netip.Prefix]netip.Addr{first: sidTwo, second: sidOne})
	vppSRv6Packet(t, fd, index, first.Addr().Next(), sidTwo, 3)
	vppSRv6Packet(t, fd, index, second.Addr().Next(), sidOne, 4)

	// An explicit MP_UNREACH removes only the first service. The remaining
	// route is a live positive control for the withdrawn route's packet test.
	vppSRv6Withdraw(t, peer, first)
	vppSRv6AwaitState(t, api, map[netip.Prefix]netip.Addr{second: sidOne})
	vppSRv6Packet(t, fd, index, first.Addr().Next(), netip.Addr{}, 5)
	vppSRv6Packet(t, fd, index, second.Addr().Next(), sidOne, 6)
	vppSRv6Withdraw(t, peer, second)
	vppSRv6AwaitState(t, api, nil)
	vppSRv6Packet(t, fd, index, second.Addr().Next(), netip.Addr{}, 7)

	// Reinstall after last-user deletion to prove the empty state was not a
	// dead dataplane. The next explicit withdrawal must clean up again.
	vppSRv6Update(t, peer, first, sidOne)
	vppSRv6AwaitState(t, api, map[netip.Prefix]netip.Addr{first: sidOne})
	vppSRv6Packet(t, fd, index, first.Addr().Next(), sidOne, 8)
	vppSRv6Withdraw(t, peer, first)
	vppSRv6AwaitState(t, api, nil)
	t.Log("srv6 forwarding proof passed")
}

// vppSRv6Underlay prepares prerequisites, not service routes. The fixed neighbor
// eliminates ND timing; both SIDs are reached through the ordinary IPv6 FIB.
func vppSRv6Underlay(t *testing.T) {
	t.Helper()
	hostMAC, err := net.ParseMAC(vppSRv6HostMAC)
	if err != nil {
		t.Fatal(err)
	}
	vppMAC, err := net.ParseMAC(vppSRv6VPPMAC)
	if err != nil {
		t.Fatal(err)
	}
	link := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: vppSRv6Host, HardwareAddr: hostMAC},
		PeerName: vppSRv6Link, PeerHardwareAddr: vppMAC}
	if err := netlink.LinkAdd(link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := netlink.LinkDel(link); err != nil {
			t.Error("delete SRv6 veth: ", err)
		}
	})
	for _, name := range []string{vppSRv6Host, vppSRv6Link} {
		iface, err := netlink.LinkByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := netlink.LinkSetUp(iface); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range []string{
		"create host-interface name " + vppSRv6Link + " hw-addr " + vppSRv6VPPMAC,
		"set interface state host-" + vppSRv6Link + " up",
		"set interface ip address host-" + vppSRv6Link + " 192.0.2.1/24",
		"set interface ip address host-" + vppSRv6Link + " " + vppSRv6Source + "/64",
		"set ip neighbor host-" + vppSRv6Link + " " + vppSRv6NextHop + " " + vppSRv6HostMAC + " static",
		"ip route add 2001:db8:95::/64 via " + vppSRv6NextHop + " host-" + vppSRv6Link,
		"set sr encaps source addr " + vppSRv6Source,
	} {
		vppSRv6CLI(t, command)
	}
	vppSRv6CLI(t, "show ip6 fib 2001:db8:95::/64")
	vppSRv6CLI(t, "trace add af-packet-input 16")
}

func vppSRv6CLI(t *testing.T, command string) {
	t.Helper()
	_ = vppSRv6CLIOutput(t, command)
}

func vppSRv6CLIOutput(t *testing.T, command string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "vppctl", vppctlArgs(command)[1:]...).CombinedOutput()
	if err != nil {
		vppSRv6Diagnostics(t)
		t.Fatalf("vppctl %s: %v: %s", command, err, output)
	}
	// vppctl returns exit status zero for CLI handler errors. Every mutation
	// this fixture issues succeeds silently except host-interface creation,
	// which MUST return the created interface name. Do not infer success from
	// a partial vocabulary of error strings.
	if !strings.HasPrefix(command, "show ") {
		expected := ""
		if strings.HasPrefix(command, "create host-interface ") {
			expected = "host-" + vppSRv6Link
		}
		if strings.TrimSpace(string(output)) != expected {
			vppSRv6Diagnostics(t)
			t.Fatalf("VPP prerequisite command rejected: %s: %s", command, output)
		}
	}
	for _, failure := range []string{"unknown input", "failed", "error:"} {
		if strings.Contains(strings.ToLower(string(output)), failure) {
			vppSRv6Diagnostics(t)
			t.Fatalf("vppctl %s: %s", command, output)
		}
	}
	t.Logf("vppctl %s:\n%s", command, output)
	return string(output)
}

// Diagnostics do not change the oracle or retry forwarding. The raw CLI path
// deliberately avoids vppSRv6CLIOutput so a failed diagnostic cannot recurse or
// replace the original failure. Limits bound both collection and retained text.
func vppSRv6Diagnostics(t *testing.T) {
	t.Helper()
	for _, command := range []string{
		"show logging", "show interface", "show hardware-interfaces",
		"show errors", "show ip6 fib", "show trace",
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		output, err := exec.CommandContext(ctx, "vppctl", vppctlArgs(command)[1:]...).CombinedOutput()
		cancel()
		if len(output) > 32768 {
			output = output[:32768]
		}
		t.Logf("VPP failure diagnostic %s (error=%v; at most 32768 bytes):\n%s", command, err, output)
	}
	for _, name := range []string{vppSRv6Host, vppSRv6Link} {
		link, err := netlink.LinkByName(name)
		if err != nil {
			t.Logf("Linux failure diagnostic interface %s: %v", name, err)
			continue
		}
		t.Logf("Linux failure diagnostic interface %s: %+v", name, link.Attrs())
	}
}

func vppSRv6AwaitState(t *testing.T, api sr.RPCService, want map[netip.Prefix]netip.Addr) {
	t.Helper()
	vppSRv6AwaitTables(t, api, vppSRv6MainTable(want))
}

type vppSRv6RouteKey struct {
	prefix netip.Prefix
	table  uint32
}

func vppSRv6MainTable(want map[netip.Prefix]netip.Addr) map[vppSRv6RouteKey]netip.Addr {
	tables := make(map[vppSRv6RouteKey]netip.Addr, len(want))
	for prefix, sid := range want {
		tables[vppSRv6RouteKey{prefix: prefix}] = sid
	}
	return tables
}

func vppSRv6AwaitTables(t *testing.T, api sr.RPCService, want map[vppSRv6RouteKey]netip.Addr) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var last error
	for {
		policies, steering, err := vppSRv6State(ctx, api)
		if err != nil {
			t.Fatal(err)
		}
		last = vppSRv6TablesMatch(policies, steering, want)
		if last == nil {
			for _, policy := range policies {
				t.Logf("VPP API policy: %+v", policy)
			}
			for _, entry := range steering {
				t.Logf("VPP API steering: %+v", entry)
			}
			vppSRv6CLI(t, "show sr policies")
			vppSRv6CLI(t, "show sr steering-policies")
			vppSRv6CLI(t, "show ip fib")
			return
		}
		select {
		case <-ctx.Done():
			vppSRv6CLI(t, "show sr policies")
			vppSRv6CLI(t, "show sr steering-policies")
			t.Fatalf("SRv6 installed-state deadline: %v; policies=%+v steering=%+v", last, policies, steering)
		case <-ticker.C:
		}
	}
}

func vppSRv6State(ctx context.Context, api sr.RPCService) ([]*sr.SrPoliciesDetails, []*sr.SrSteeringPolDetails, error) {
	policies, err := api.SrPoliciesDump(ctx, &sr.SrPoliciesDump{})
	if err != nil {
		return nil, nil, err
	}
	var gotPolicies []*sr.SrPoliciesDetails
	for {
		entry, err := policies.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		gotPolicies = append(gotPolicies, entry)
	}
	steering, err := api.SrSteeringPolDump(ctx, &sr.SrSteeringPolDump{})
	if err != nil {
		return nil, nil, err
	}
	var gotSteering []*sr.SrSteeringPolDetails
	for {
		entry, err := steering.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		gotSteering = append(gotSteering, entry)
	}
	return gotPolicies, gotSteering, nil
}

// vppSRv6StateMatches treats dump payloads as untrusted data. A BSID's value is
// deliberately unconstrained except that it must differ from the remote SID and
// every steering entry must reference the policy with the matching SID segment.
func vppSRv6StateMatches(policies []*sr.SrPoliciesDetails, steering []*sr.SrSteeringPolDetails, want map[netip.Prefix]netip.Addr) error {
	return vppSRv6TablesMatch(policies, steering, vppSRv6MainTable(want))
}

func vppSRv6TablesMatch(policies []*sr.SrPoliciesDetails, steering []*sr.SrSteeringPolDetails, want map[vppSRv6RouteKey]netip.Addr) error {
	segments := make(map[netip.Addr]netip.Addr)
	for _, policy := range policies {
		if !policy.IsEncap {
			return errors.New("service policy is not encapsulation mode")
		}
		if policy.FibTable != 0 {
			return fmt.Errorf("policy underlay table = %d, want 0", policy.FibTable)
		}
		if len(policy.SidLists) != 1 {
			return fmt.Errorf("policy has %d segment lists, want 1", len(policy.SidLists))
		}
		if policy.SidLists[0].NumSids != 1 {
			return errors.New("policy is not a single received SID")
		}
		bsid := netip.AddrFrom16(policy.Bsid)
		sid := netip.AddrFrom16(policy.SidLists[0].Sids[0])
		if bsid == sid {
			return errors.New("received SID was reused as the locally owned BSID")
		}
		segments[bsid] = sid
	}
	wantedSIDs := make(map[netip.Addr]bool)
	for _, sid := range want {
		wantedSIDs[sid] = true
	}
	if len(policies) != len(wantedSIDs) {
		return fmt.Errorf("policies = %d, want %d shared-SID policies", len(policies), len(wantedSIDs))
	}
	if len(steering) != len(want) {
		return fmt.Errorf("steering entries = %d, want %d", len(steering), len(want))
	}
	seen := make(map[vppSRv6RouteKey]bool)
	for _, entry := range steering {
		if entry.TrafficType != sr_types.SR_STEER_API_IPV4 {
			return fmt.Errorf("unexpected steering traffic type %d", entry.TrafficType)
		}
		prefix, err := netip.ParsePrefix(entry.Prefix.ToIPNet().String())
		if err != nil {
			return err
		}
		key := vppSRv6RouteKey{prefix: prefix, table: entry.FibTable}
		if seen[key] {
			return fmt.Errorf("duplicate steering prefix %s table %d", prefix, key.table)
		}
		seen[key] = true
		sid, exists := want[key]
		if !exists {
			return fmt.Errorf("unexpected steering prefix %s table %d", prefix, key.table)
		}
		if segments[netip.AddrFrom16(entry.Bsid)] != sid {
			return fmt.Errorf("prefix %s does not bind to an owned policy with segment %s", prefix, sid)
		}
	}
	return nil
}

func vppSRv6PolicyIDs(t *testing.T, api sr.RPCService) map[netip.Addr]netip.Addr {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	policies, _, err := vppSRv6State(ctx, api)
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[netip.Addr]netip.Addr, len(policies))
	for _, policy := range policies {
		if len(policy.SidLists) != 1 {
			t.Fatal("policy identity snapshot requires one segment list")
		}
		ids[netip.AddrFrom16(policy.Bsid)] = netip.AddrFrom16(policy.SidLists[0].Sids[0])
	}
	return ids
}

func vppSRv6SamePolicyIDs(t *testing.T, before, after map[netip.Addr]netip.Addr) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("policy identity count changed: before=%v after=%v", before, after)
	}
	for bsid, sid := range before {
		if after[bsid] != sid {
			t.Fatalf("confirmed policy identity changed: before=%v after=%v", before, after)
		}
	}
}
