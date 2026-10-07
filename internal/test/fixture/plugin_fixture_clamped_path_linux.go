//go:build linux

// Design: docs/architecture/diagnostics/active-probes.md -- the Don't Fragment mode
// Design: docs/architecture/diagnostics/path-mtu.md -- the show mtu run
// Design: docs/functional-tests.md -- a clamped path built by the test itself
// Related: register_clamped_path_linux.go -- the two fixture names
// Related: plugin_fixture_ping_df.go -- the observer the ping tests run inside the daemon
// Related: plugin_fixture_show_mtu.go -- the observers the show mtu tests run inside the daemon
// Related: internal/test/runner/netns_linux.go -- the runner's own per-test namespace (Fix B)
//
// plugin_fixture_clamped_path_linux.go builds the path a Don't Fragment probe
// is refused on, then runs one command inside it:
//
//	sender (10.99.1.1) -- 1600 -- router -- 1400 -- far (10.99.2.1, .3, .9)
//
// Three named network namespaces joined by two veth pairs. The near link
// carries every probe the daemon sends, so the router and not the sender
// refuses an oversized one, and the refusal arrives as an ICMP Fragmentation
// Needed the daemon reads off its error queue. The command, `ze start <conf>`
// in every test today, is started with the fixture's thread inside the sender
// namespace, and inherits it: a child takes the network namespace of the
// thread that forks it, which is the assumption the runner's Fix B mode
// validates (TestNetnsLaunchChildInheritsNamespace).
//
// The same file carries `plugin/isolated-netns`, one namespace with loopback
// and nothing else, for a command that must find no route at all.
//
// Two options change what the command inherits. `without-net-raw` drops
// CAP_NET_RAW from the thread's capability bounding set before the command
// starts: root re-derives its permitted set from the bounding set at exec, so
// the child holds none, which is what `capsh --drop=cap_net_raw` does. The
// fixture then reads the child's CapEff from /proc and refuses to go on when
// the bit is still set, so a green run cannot come from a daemon that kept the
// privilege. `far-daemon <conf>` starts a second `ze start` in the far
// namespace before the command, for a test whose daemon needs a peer to talk
// to. Each far daemon runs from a copy of its file in a directory of its own,
// because a daemon's live store sits beside its configuration file and admits
// one owner.
//
// `ipv6` adds an IPv6 plane to the same topology: a Global /64 on each link
// (2001:db8:99:1::/64 near, 2001:db8:99:2::/64 far), a fixed Link-Local
// address on every veth end (fe80::99:1:1 on sr0, fe80::99:1:2 on rs0,
// fe80::99:2:2 on rf0, fe80::99:2:1 on fr0), the routes both ways through the
// router, and IPv6 forwarding in it. Every address is added NODAD, so it is
// usable the moment the link is up rather than tentative for a second.
//
// `peer <namespace> <script>` and `peer-after <file> <namespace> <script>` run
// `le test peer <script>` inside `sender`, `router` or `far`: the first before
// the command, the second after it, once `<file>` exists. That is how a sender
// waits for a receiver's session: the receiver's script declares
// `option=established-file:path=<file>`, which the peer writes on the daemon's
// first UPDATE. With a peer declared, the peers decide the verdict. The fixture
// waits for every one of them, a peer that exits non-zero is the fixture's
// error, the command is stopped once they are all done, and a command that
// exits first is an error, because a receiver whose daemon died can never see
// what it asserts.
//
// Everything the shell script this replaces did with iproute2 and libcap is a
// netlink message or a syscall here, so the test needs no `ip`, no `capsh` and
// no `sh` in the guest. The namespaces are deleted when the command exits,
// whatever its exit code, and a stale namespace of the same name left by a
// killed run is removed before the new one is created.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// clampedNearMTU is the sender and router-near link MTU.
	clampedNearMTU = 1600
	// clampedFarMTU is the router-far and far link MTU, the clamp under test.
	clampedFarMTU = 1400
	// clampedSenderLink is the sender's veth end, the underlay `show mtu` names.
	clampedSenderLink = "sr0"
	clampedRouterNear = "rs0"
	clampedRouterFar  = "rf0"
	clampedFarLink    = "fr0"

	clampedSenderAddr    = "10.99.1.1/24"
	clampedRouterNearIP  = "10.99.1.2"
	clampedRouterNearCID = "10.99.1.2/24"
	clampedRouterFarIP   = "10.99.2.2"
	clampedRouterFarCID  = "10.99.2.2/24"
	clampedSenderNet     = "10.99.1.0/24"
	clampedFarNet        = "10.99.2.0/24"
	// clampedFarHost is the far address the ping and show mtu observers probe.
	clampedFarHost = "10.99.2.1/32"

	// The IPv6 plane `ipv6` adds: one Global /64 per link, and a fixed
	// Link-Local address on each veth end so a test can name it.
	clampedSenderAddr6    = "2001:db8:99:1::1/64"
	clampedSenderLL       = "fe80::99:1:1/64"
	clampedRouterNearIP6  = "2001:db8:99:1::2"
	clampedRouterNearCID6 = "2001:db8:99:1::2/64"
	clampedRouterNearLL   = "fe80::99:1:2/64"
	clampedRouterFarIP6   = "2001:db8:99:2::2"
	clampedRouterFarCID6  = "2001:db8:99:2::2/64"
	clampedRouterFarLL    = "fe80::99:2:2/64"
	clampedFarAddr6       = "2001:db8:99:2::1/64"
	clampedFarLL          = "fe80::99:2:1/64"
	clampedSenderNet6     = "2001:db8:99:1::/64"
	clampedFarNet6        = "2001:db8:99:2::/64"

	// clampedPingGroupRange admits every group to the datagram ICMP socket
	// inside the sender namespace, where the range is per namespace.
	clampedPingGroupRange = "0 65535"
	// clampedFarDaemonEnv keeps a far-side responder off the real xfrm
	// dataplane; only the IKE negotiation is wanted from it.
	clampedFarDaemonEnv = "ze_test_ike_dataplane=noop"

	// clampedPeerWait bounds how long a `peer-after` waits for its file. The
	// runner's own test timeout still ends the whole run first when it is
	// shorter.
	clampedPeerWait = 60 * time.Second
	// clampedPeerPoll is how often that wait looks for the file.
	clampedPeerPoll = 100 * time.Millisecond

	procSysIPForward      = "/proc/sys/net/ipv4/ip_forward"
	procSysIPv6Forward    = "/proc/sys/net/ipv6/conf/all/forwarding"
	procSysPingGroupRange = "/proc/sys/net/ipv4/ping_group_range"

	// netnsArgPeer is the fixture keyword that declares a peer run.
	netnsArgPeer = "peer"
	// lePeerVerb is the `le test` verb that runs a peer script.
	lePeerVerb = "peer"
)

// clampedFarAddrs are the far namespace's addresses: the probed host, a second
// IKE responder and the `show mtu` reference address.
var clampedFarAddrs = []string{"10.99.2.1/24", "10.99.2.3/24", "10.99.2.9/24"}

// clampedNamespaces are the role names `peer` and `peer-after` accept.
var clampedNamespaces = []string{"sender", "router", "far"}

// netnsRunPlan is what the fixture arguments ask for.
type netnsRunPlan struct {
	// prefix names the namespaces: `<prefix>-s`, `-r` and `-f` for the
	// clamped path, the prefix itself for the isolated namespace.
	prefix string
	// routeMTU pins a host route to the far host at this MTU in the sender
	// namespace, so the kernel's own estimate is wrong before the run. Zero
	// pins nothing.
	routeMTU int
	// farDaemons are configuration files each started as `ze start <conf>`
	// in the far namespace before the command.
	farDaemons []string
	// peers are the `le test peer` runs, in the order the arguments named
	// them. Any peer makes the peers, not the command, decide the verdict.
	peers []netnsPeer
	// ipv6 adds the IPv6 plane to the clamped path.
	ipv6 bool
	// withoutNetRaw drops CAP_NET_RAW from the command's bounding set.
	withoutNetRaw bool
	// command is the argv run in the sender namespace.
	command []string
}

// netnsPeer is one `le test peer <script>` the fixture runs in a namespace.
type netnsPeer struct {
	// namespace is `sender`, `router` or `far`.
	namespace string
	// script is the peer's expectation file, the .ci's own tmpfs file.
	script string
	// after names the file that MUST exist before the peer starts. Empty
	// starts the peer before the command.
	after string
}

// parseNetnsRunArgs reads `netns <prefix> [route-mtu <octets>]
// [far-daemon <conf>]... [ipv6] [peer <namespace> <script>]...
// [peer-after <file> <namespace> <script>]... [without-net-raw] run
// <argv...>`. Every keyword precedes its value, and `run` takes the rest of
// the line.
func parseNetnsRunArgs(args []string) (*netnsRunPlan, error) {
	plan := &netnsRunPlan{}
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "netns":
			if index+1 >= len(args) {
				return nil, errors.New("netns takes a prefix")
			}
			index++
			plan.prefix = args[index]
		case "route-mtu":
			if index+1 >= len(args) {
				return nil, errors.New("route-mtu takes an octet count")
			}
			index++
			mtu, err := strconv.Atoi(args[index])
			if err != nil {
				return nil, fmt.Errorf("route-mtu %q: %w", args[index], err)
			}
			plan.routeMTU = mtu
		case "far-daemon":
			if index+1 >= len(args) {
				return nil, errors.New("far-daemon takes a configuration file")
			}
			index++
			plan.farDaemons = append(plan.farDaemons, args[index])
		case "ipv6":
			plan.ipv6 = true
		case netnsArgPeer:
			if index+2 >= len(args) {
				return nil, errors.New("peer takes a namespace and a script")
			}
			peer, err := newNetnsPeer(args[index+1], args[index+2], "")
			if err != nil {
				return nil, err
			}
			plan.peers = append(plan.peers, peer)
			index += 2
		case "peer-after":
			if index+3 >= len(args) {
				return nil, errors.New("peer-after takes a file, a namespace and a script")
			}
			peer, err := newNetnsPeer(args[index+2], args[index+3], args[index+1])
			if err != nil {
				return nil, err
			}
			plan.peers = append(plan.peers, peer)
			index += 3
		case "without-net-raw":
			plan.withoutNetRaw = true
		case "run":
			plan.command = args[index+1:]
			index = len(args)
		default:
			return nil, fmt.Errorf("unknown argument %q; expected netns, route-mtu, far-daemon, ipv6, peer, peer-after, without-net-raw or run", args[index])
		}
	}
	if plan.prefix == "" {
		return nil, errors.New("netns <prefix> is required")
	}
	if len(plan.command) == 0 {
		return nil, errors.New("run <argv...> is required")
	}
	return plan, nil
}

// newNetnsPeer refuses a namespace the clamped path does not build, at parse
// time, so a typo fails before any namespace exists.
func newNetnsPeer(namespace, script, after string) (netnsPeer, error) {
	if slices.Contains(clampedNamespaces, namespace) {
		return netnsPeer{namespace: namespace, script: script, after: after}, nil
	}
	return netnsPeer{}, fmt.Errorf("peer namespace %q; expected sender, router or far", namespace)
}

// netnsSay writes one line to stderr, where the runner reads the fixture's
// report.
func netnsSay(parts ...string) {
	var line textbuf.Buffer
	for _, part := range parts {
		line.Str(part)
	}
	line.Byte('\n')
	line.StdErr() //nolint:errcheck // stderr is the report channel; a failed write has no other channel to report on
}

// testNetns is one named network namespace and the netlink handle that
// programs it without entering it.
type testNetns struct {
	name   string
	ns     netns.NsHandle
	handle *netlink.Handle
}

// createTestNetns creates a named namespace and returns with the calling
// thread back in `orig`. A stale namespace of the same name is removed first:
// a killed run leaves its bind mount behind, and the next run of the same
// test would otherwise fail on EEXIST.
func createTestNetns(name string, orig netns.NsHandle) (*testNetns, error) {
	netns.DeleteNamed(name) //nolint:errcheck // best-effort; an absent name is the common case
	ns, err := netns.NewNamed(name)
	if err != nil {
		return nil, fmt.Errorf("create network namespace %q (needs CAP_SYS_ADMIN): %w", name, err)
	}
	if err := netns.Set(orig); err != nil {
		ns.Close() //nolint:errcheck // best-effort close on the error path
		return nil, fmt.Errorf("return to the original network namespace: %w", err)
	}
	handle, err := netlink.NewHandleAt(ns)
	if err != nil {
		ns.Close() //nolint:errcheck // best-effort close on the error path
		return nil, fmt.Errorf("open a netlink handle in %q: %w", name, err)
	}
	lo, err := handle.LinkByName("lo")
	if err != nil {
		handle.Close()
		ns.Close() //nolint:errcheck // best-effort close on the error path
		return nil, fmt.Errorf("find lo in %q: %w", name, err)
	}
	if err := handle.LinkSetUp(lo); err != nil {
		handle.Close()
		ns.Close() //nolint:errcheck // best-effort close on the error path
		return nil, fmt.Errorf("bring lo up in %q: %w", name, err)
	}
	return &testNetns{name: name, ns: ns, handle: handle}, nil
}

// remove deletes the namespace. Every link inside it goes with it.
func (n *testNetns) remove() {
	n.handle.Close()
	n.ns.Close() //nolint:errcheck // best-effort close at teardown
	if err := netns.DeleteNamed(n.name); err != nil {
		netnsSay("delete network namespace ", n.name, ": ", err.Error())
	}
}

// configureLink sets the MTU, adds each address and brings the link up.
func (n *testNetns) configureLink(name string, mtu int, addrs ...string) error {
	link, err := n.handle.LinkByName(name)
	if err != nil {
		return fmt.Errorf("find %s in %s: %w", name, n.name, err)
	}
	if err := n.handle.LinkSetMTU(link, mtu); err != nil {
		return fmt.Errorf("set %s mtu %d in %s: %w", name, mtu, n.name, err)
	}
	for _, addr := range addrs {
		parsed, err := netlink.ParseAddr(addr)
		if err != nil {
			return fmt.Errorf("parse %s: %w", addr, err)
		}
		if err := n.handle.AddrAdd(link, parsed); err != nil {
			return fmt.Errorf("add %s to %s in %s: %w", addr, name, n.name, err)
		}
	}
	if err := n.handle.LinkSetUp(link); err != nil {
		return fmt.Errorf("bring %s up in %s: %w", name, n.name, err)
	}
	return nil
}

// addAddrsNoDAD adds IPv6 addresses to a link with IFA_F_NODAD. Duplicate
// Address Detection would hold each one tentative, unusable as a source or a
// bind address, for about a second after it is added, and the veth has no
// other host on it to collide with.
func (n *testNetns) addAddrsNoDAD(name string, addrs ...string) error {
	link, err := n.handle.LinkByName(name)
	if err != nil {
		return fmt.Errorf("find %s in %s: %w", name, n.name, err)
	}
	for _, addr := range addrs {
		parsed, err := netlink.ParseAddr(addr)
		if err != nil {
			return fmt.Errorf("parse %s: %w", addr, err)
		}
		parsed.Flags = unix.IFA_F_NODAD
		if err := n.handle.AddrAdd(link, parsed); err != nil {
			return fmt.Errorf("add %s to %s in %s: %w", addr, name, n.name, err)
		}
	}
	return nil
}

// addRoute installs `dst via gw dev link`, at `mtu` when it is not zero.
func (n *testNetns) addRoute(dst, gw, link string, mtu int) error {
	dev, err := n.handle.LinkByName(link)
	if err != nil {
		return fmt.Errorf("find %s in %s: %w", link, n.name, err)
	}
	dstNet, err := netlink.ParseIPNet(dst)
	if err != nil {
		return fmt.Errorf("parse %s: %w", dst, err)
	}
	gwIP := net.ParseIP(gw)
	if gwIP == nil {
		return fmt.Errorf("parse gateway %q", gw)
	}
	route := &netlink.Route{LinkIndex: dev.Attrs().Index, Dst: dstNet, Gw: gwIP, MTU: mtu}
	if err := n.handle.RouteAdd(route); err != nil {
		return fmt.Errorf("add route %s via %s in %s: %w", dst, gw, n.name, err)
	}
	return nil
}

// writeSysctl writes a per-namespace /proc/sys key from inside the
// namespace. The calling thread MUST be locked; it returns in `orig`.
func (n *testNetns) writeSysctl(orig netns.NsHandle, path, value string) error {
	if err := netns.Set(n.ns); err != nil {
		return fmt.Errorf("enter %s: %w", n.name, err)
	}
	writeErr := os.WriteFile(path, []byte(value), 0o600)
	if err := netns.Set(orig); err != nil {
		return fmt.Errorf("return from %s: %w", n.name, err)
	}
	if writeErr != nil {
		return fmt.Errorf("write %s in %s: %w", path, n.name, writeErr)
	}
	return nil
}

// clampedPath is the three-namespace topology.
type clampedPath struct {
	sender, router, far *testNetns
}

// buildClampedPath creates the three namespaces, the two veth pairs, the
// addresses, the routes and the router's forwarding. The calling thread MUST
// be locked, and it returns in `orig`.
func buildClampedPath(plan *netnsRunPlan, orig netns.NsHandle) (*clampedPath, error) {
	path := &clampedPath{}
	var err error
	for _, ns := range []struct {
		target **testNetns
		suffix string
	}{{&path.sender, "-s"}, {&path.router, "-r"}, {&path.far, "-f"}} {
		*ns.target, err = createTestNetns(plan.prefix+ns.suffix, orig)
		if err != nil {
			path.remove()
			return nil, err
		}
	}
	if err := path.wire(plan, orig); err != nil {
		path.remove()
		return nil, err
	}
	return path, nil
}

// wire creates the links and programs them. Each veth is created with its far
// end already in the next namespace, so no link is ever visible in the host.
func (p *clampedPath) wire(plan *netnsRunPlan, orig netns.NsHandle) error {
	nearPair := &netlink.Veth{
		Name:          clampedSenderLink,
		PeerName:      clampedRouterNear,
		PeerNamespace: netlink.NsFd(int(p.router.ns)),
	}
	if err := p.sender.handle.LinkAdd(nearPair); err != nil {
		return fmt.Errorf("create %s/%s: %w", clampedSenderLink, clampedRouterNear, err)
	}
	farPair := &netlink.Veth{
		Name:          clampedRouterFar,
		PeerName:      clampedFarLink,
		PeerNamespace: netlink.NsFd(int(p.far.ns)),
	}
	if err := p.router.handle.LinkAdd(farPair); err != nil {
		return fmt.Errorf("create %s/%s: %w", clampedRouterFar, clampedFarLink, err)
	}
	if err := p.sender.configureLink(clampedSenderLink, clampedNearMTU, clampedSenderAddr); err != nil {
		return err
	}
	if err := p.router.configureLink(clampedRouterNear, clampedNearMTU, clampedRouterNearCID); err != nil {
		return err
	}
	if err := p.router.configureLink(clampedRouterFar, clampedFarMTU, clampedRouterFarCID); err != nil {
		return err
	}
	if err := p.far.configureLink(clampedFarLink, clampedFarMTU, clampedFarAddrs...); err != nil {
		return err
	}
	if err := p.sender.addRoute(clampedFarNet, clampedRouterNearIP, clampedSenderLink, 0); err != nil {
		return err
	}
	if plan.routeMTU != 0 {
		if err := p.sender.addRoute(clampedFarHost, clampedRouterNearIP, clampedSenderLink, plan.routeMTU); err != nil {
			return err
		}
	}
	if err := p.far.addRoute(clampedSenderNet, clampedRouterFarIP, clampedFarLink, 0); err != nil {
		return err
	}
	if err := p.router.writeSysctl(orig, procSysIPForward, "1"); err != nil {
		return err
	}
	if plan.ipv6 {
		if err := p.wireIPv6(orig); err != nil {
			return err
		}
	}
	if plan.withoutNetRaw {
		if err := p.sender.writeSysctl(orig, procSysPingGroupRange, clampedPingGroupRange); err != nil {
			return err
		}
	}
	return nil
}

// wireIPv6 adds the IPv6 plane once the links are up: the Global and the
// Link-Local address of each veth end, the route each edge needs to the other
// edge's /64, and forwarding in the router. The calling thread MUST be locked,
// and it returns in `orig`.
func (p *clampedPath) wireIPv6(orig netns.NsHandle) error {
	if err := p.sender.addAddrsNoDAD(clampedSenderLink, clampedSenderAddr6, clampedSenderLL); err != nil {
		return err
	}
	if err := p.router.addAddrsNoDAD(clampedRouterNear, clampedRouterNearCID6, clampedRouterNearLL); err != nil {
		return err
	}
	if err := p.router.addAddrsNoDAD(clampedRouterFar, clampedRouterFarCID6, clampedRouterFarLL); err != nil {
		return err
	}
	if err := p.far.addAddrsNoDAD(clampedFarLink, clampedFarAddr6, clampedFarLL); err != nil {
		return err
	}
	if err := p.sender.addRoute(clampedFarNet6, clampedRouterNearIP6, clampedSenderLink, 0); err != nil {
		return err
	}
	if err := p.far.addRoute(clampedSenderNet6, clampedRouterFarIP6, clampedFarLink, 0); err != nil {
		return err
	}
	return p.router.writeSysctl(orig, procSysIPv6Forward, "1")
}

// namespace answers the namespace a `peer` role names. The parser accepted
// only the three roles, so another name is a fixture defect.
func (p *clampedPath) namespace(role string) (*testNetns, error) {
	switch role {
	case "sender":
		return p.sender, nil
	case "router":
		return p.router, nil
	case "far":
		return p.far, nil
	}
	return nil, fmt.Errorf("BUG: peer namespace %q passed the parser", role)
}

// remove deletes whichever namespaces were created.
func (p *clampedPath) remove() {
	for _, ns := range []*testNetns{p.sender, p.router, p.far} {
		if ns != nil {
			ns.remove()
		}
	}
}

// clampedPathDriver is `plugin/clamped-path`.
func clampedPathDriver(ctx context.Context, args []string) error {
	plan, err := parseNetnsRunArgs(args)
	if err != nil {
		return err
	}
	// The thread stays locked for the life of the process: the namespace
	// switches and the bounding-set drop below are per thread, and the
	// command MUST be forked from this thread to inherit them.
	runtime.LockOSThread()
	orig, err := netns.Get()
	if err != nil {
		return fmt.Errorf("get the current network namespace: %w", err)
	}
	defer orig.Close() //nolint:errcheck // best-effort close at exit

	path, err := buildClampedPath(plan, orig)
	if err != nil {
		return err
	}
	defer path.remove()

	farDaemons, err := startFarDaemons(ctx, plan, path.far, orig)
	if err != nil {
		return err
	}
	defer stopFarDaemons(farDaemons)

	if len(plan.peers) == 0 {
		return runInNetns(ctx, plan, path.sender, orig)
	}
	return runWithPeers(ctx, plan, path, orig)
}

// isolatedNetnsDriver is `plugin/isolated-netns`: one namespace with loopback
// up and no other link, so the command finds no route to anything.
func isolatedNetnsDriver(ctx context.Context, args []string) error {
	plan, err := parseNetnsRunArgs(args)
	if err != nil {
		return err
	}
	if plan.routeMTU != 0 || len(plan.farDaemons) != 0 {
		return errors.New("isolated-netns takes neither route-mtu nor far-daemon")
	}
	if plan.ipv6 {
		return errors.New("isolated-netns takes neither ipv6 nor peer: it has no link to carry either")
	}
	if len(plan.peers) != 0 {
		return errors.New("isolated-netns takes neither ipv6 nor peer: it has no link to carry either")
	}
	runtime.LockOSThread()
	orig, err := netns.Get()
	if err != nil {
		return fmt.Errorf("get the current network namespace: %w", err)
	}
	defer orig.Close() //nolint:errcheck // best-effort close at exit

	ns, err := createTestNetns(plan.prefix, orig)
	if err != nil {
		return err
	}
	defer ns.remove()

	return runInNetns(ctx, plan, ns, orig)
}

// startFarDaemons starts `ze start <conf>` in the far namespace for each
// far-daemon argument, off the real dataplane, each from a directory of its
// own (farDaemonConfig). A daemon dies with this process (Pdeathsig) and with
// the context, so a killed fixture leaves no responder behind.
func startFarDaemons(ctx context.Context, plan *netnsRunPlan, far *testNetns, orig netns.NsHandle) ([]*exec.Cmd, error) {
	if len(plan.farDaemons) == 0 {
		return nil, nil
	}
	configs := make([]string, 0, len(plan.farDaemons))
	for _, conf := range plan.farDaemons {
		config, err := farDaemonConfig(conf)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	if err := netns.Set(far.ns); err != nil {
		return nil, fmt.Errorf("enter %s: %w", far.name, err)
	}
	var daemons []*exec.Cmd
	var startErr error
	for _, conf := range configs {
		daemon := exec.CommandContext(ctx, "ze", "start", conf) //nolint:gosec // test fixture; the argument is a copy of the .ci's own file
		daemon.Dir = filepath.Dir(conf)
		daemon.Env = append(os.Environ(), clampedFarDaemonEnv)
		daemon.Stdout = os.Stdout
		daemon.Stderr = os.Stderr
		daemon.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
		if err := daemon.Start(); err != nil {
			startErr = fmt.Errorf("start far daemon %s: %w", conf, err)
			break
		}
		daemons = append(daemons, daemon)
	}
	if err := netns.Set(orig); err != nil {
		stopFarDaemons(daemons)
		return nil, fmt.Errorf("return from %s: %w", far.name, err)
	}
	if startErr != nil {
		stopFarDaemons(daemons)
		return nil, startErr
	}
	return daemons, nil
}

// farDaemonConfig copies one far-daemon configuration into a new directory
// beside it and returns the copy's absolute path. `ze start <file>` keeps its
// live store in the directory that holds the file, and the store is owned by
// one process: the .ci's tmpfs files all land in the test's work directory,
// beside the near daemon's own, so started where they are, only one of the
// three daemons would open its store. The work directory, which the runner
// removes, holds the copies.
func farDaemonConfig(conf string) (string, error) {
	body, err := os.ReadFile(conf) //nolint:gosec // test fixture; the path is the .ci's own far-daemon argument
	if err != nil {
		return "", fmt.Errorf("read far daemon %s: %w", conf, err)
	}
	dir, err := os.MkdirTemp(filepath.Dir(conf), "far-daemon-")
	if err != nil {
		return "", fmt.Errorf("make a directory for far daemon %s: %w", conf, err)
	}
	config, err := filepath.Abs(filepath.Join(dir, filepath.Base(conf)))
	if err != nil {
		return "", fmt.Errorf("resolve far daemon %s: %w", conf, err)
	}
	if err := os.WriteFile(config, body, 0o600); err != nil {
		return "", fmt.Errorf("copy far daemon %s: %w", conf, err)
	}
	return config, nil
}

// stopFarDaemons kills and reaps each far daemon, as the script's trap did.
func stopFarDaemons(daemons []*exec.Cmd) {
	for _, daemon := range daemons {
		daemon.Process.Kill() //nolint:errcheck // a daemon that already exited is fine
		daemon.Wait()         //nolint:errcheck // the exit status of a killed responder is not an assertion
	}
}

// startInNetns starts argv inside `ns`, forked from the locked calling thread
// so the child inherits the namespace, and returns the thread to `orig`. The
// child dies with this process (Pdeathsig) and with the context.
func startInNetns(ctx context.Context, argv []string, ns *testNetns, orig netns.NsHandle) (*exec.Cmd, error) {
	if err := netns.Set(ns.ns); err != nil {
		return nil, fmt.Errorf("enter %s: %w", ns.name, err)
	}
	command := exec.CommandContext(ctx, argv[0], argv[1:]...) //nolint:gosec // test fixture; the argv is the .ci's own line
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	startErr := command.Start()
	if err := netns.Set(orig); err != nil {
		if startErr == nil {
			command.Process.Kill() //nolint:errcheck // the thread is lost; the child goes with it
			command.Wait()         //nolint:errcheck // reaped only
		}
		return nil, fmt.Errorf("return from %s: %w", ns.name, err)
	}
	if startErr != nil {
		return nil, fmt.Errorf("start %s in %s: %w", strings.Join(argv, " "), ns.name, startErr)
	}
	return command, nil
}

// startCommand starts the plan's command in `ns`. With withoutNetRaw the
// bounding set loses CAP_NET_RAW first, and the child's CapEff is read back
// before the run is allowed to count.
func startCommand(ctx context.Context, plan *netnsRunPlan, ns *testNetns, orig netns.NsHandle) (*exec.Cmd, error) {
	if plan.withoutNetRaw {
		// PR_CAPBSET_DROP is per thread and irreversible; the command below is
		// forked from this thread and inherits the reduced bounding set, and so
		// does every `peer-after` started after it.
		if err := unix.Prctl(unix.PR_CAPBSET_DROP, unix.CAP_NET_RAW, 0, 0, 0); err != nil {
			return nil, fmt.Errorf("drop CAP_NET_RAW from the bounding set: %w", err)
		}
	}
	command, err := startInNetns(ctx, plan.command, ns, orig)
	if err != nil {
		return nil, err
	}
	if plan.withoutNetRaw {
		if err := confirmNetRawDropped(command.Process.Pid); err != nil {
			command.Process.Kill() //nolint:errcheck // the child is refused whatever it is doing
			command.Wait()         //nolint:errcheck // reaped only
			return nil, err
		}
	}
	return command, nil
}

// runInNetns starts the plan's command inside `ns`, on the locked calling
// thread, and waits for it. The context kills the child when the runner tears
// the test down.
func runInNetns(ctx context.Context, plan *netnsRunPlan, ns *testNetns, orig netns.NsHandle) error {
	command, err := startCommand(ctx, plan, ns, orig)
	if err != nil {
		return err
	}
	if err := command.Wait(); err != nil {
		return fmt.Errorf("%s in %s: %w", strings.Join(plan.command, " "), ns.name, err)
	}
	return nil
}

// processExit is one watched child's end.
type processExit struct {
	label   string
	err     error
	command bool
}

// watchedProcesses reaps the children of a peer run. Each child gets one
// goroutine for its lifetime, which waits on it and sends one processExit to
// a channel sized for every child, so no sender ever blocks. The owner MUST
// call stop before it returns, which kills every child and drains every
// outstanding exit, so no goroutine outlives the run. Not safe for concurrent
// use: only the fixture's locked thread calls it.
type watchedProcesses struct {
	exits   chan processExit
	started []*exec.Cmd
	pending int
}

// watch hands a started child to its waiter goroutine.
func (w *watchedProcesses) watch(child *exec.Cmd, label string, command bool) {
	w.started = append(w.started, child)
	w.pending++
	go reapWatchedProcess(child, label, command, w.exits)
}

// reapWatchedProcess is the waiter goroutine: it ends when the child does.
func reapWatchedProcess(child *exec.Cmd, label string, command bool, exits chan<- processExit) {
	exits <- processExit{label: label, err: child.Wait(), command: command}
}

// stop kills every child and drains every exit still owed. It MUST be called
// once the run is decided.
func (w *watchedProcesses) stop() {
	for _, child := range w.started {
		child.Process.Kill() //nolint:errcheck // a child that already exited is fine
	}
	for w.pending > 0 {
		<-w.exits
		w.pending--
	}
}

// runWithPeers is the run when the plan names a peer: the peers that start
// before the command, the command, then each `peer-after` once its file
// exists, in the order the arguments named them. It returns when every peer
// has exited zero, or at the first failure: a peer that exits non-zero, a
// command that exits while a peer is still owed, a file that never appears.
func runWithPeers(ctx context.Context, plan *netnsRunPlan, path *clampedPath, orig netns.NsHandle) error {
	watched := &watchedProcesses{exits: make(chan processExit, len(plan.peers)+1)}
	defer watched.stop()

	var deferred []netnsPeer
	for _, peer := range plan.peers {
		if peer.after != "" {
			deferred = append(deferred, peer)
			continue
		}
		if err := startPeer(ctx, peer, path, orig, watched); err != nil {
			return err
		}
	}
	command, err := startCommand(ctx, plan, path.sender, orig)
	if err != nil {
		return err
	}
	watched.watch(command, strings.Join(plan.command, " "), true)

	peersOwed := len(plan.peers)
	poll := time.NewTicker(clampedPeerPoll)
	defer poll.Stop()
	waitEnd := time.Now().Add(clampedPeerWait)
	for peersOwed > 0 {
		if len(deferred) > 0 {
			ready, err := peerFileExists(deferred[0].after)
			if err != nil {
				return err
			}
			if ready {
				if err := startPeer(ctx, deferred[0], path, orig, watched); err != nil {
					return err
				}
				deferred = deferred[1:]
				waitEnd = time.Now().Add(clampedPeerWait)
				continue
			}
			if time.Now().After(waitEnd) {
				return fmt.Errorf("peer-after %s: the file did not appear within %s", deferred[0].after, clampedPeerWait)
			}
		}
		select {
		case exit := <-watched.exits:
			watched.pending--
			if exit.command && exit.err != nil {
				return fmt.Errorf("%s exited while %d peer(s) were still owed: %w", exit.label, peersOwed, exit.err)
			}
			if exit.command {
				return fmt.Errorf("%s exited zero while %d peer(s) were still owed", exit.label, peersOwed)
			}
			if exit.err != nil {
				return fmt.Errorf("%s: %w", exit.label, exit.err)
			}
			netnsSay("PEER-PASSED: ", exit.label)
			peersOwed--
		case <-poll.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// startPeer starts `le test peer <script>` in the peer's namespace and hands
// it to the watcher.
func startPeer(ctx context.Context, peer netnsPeer, path *clampedPath, orig netns.NsHandle, watched *watchedProcesses) error {
	ns, err := path.namespace(peer.namespace)
	if err != nil {
		return err
	}
	child, err := startInNetns(ctx, []string{"le", "test", lePeerVerb, peer.script}, ns, orig)
	if err != nil {
		return err
	}
	watched.watch(child, "peer "+peer.namespace+" "+peer.script, false)
	return nil
}

// peerFileExists answers whether a `peer-after` file has been written. Only a
// missing file is "not yet"; any other stat error is reported.
func peerFileExists(name string) (bool, error) {
	_, err := os.Stat(name)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("peer-after %s: %w", name, err)
}

// confirmNetRawDropped reads the child's CapEff after its exec and reports the
// drop on stderr in the words the .ci asserts on. The bit still set is the
// GUARD line and an error: the daemon would open a raw socket and the test
// would pass for the wrong reason.
func confirmNetRawDropped(pid int) error {
	var pathBuf textbuf.Buffer
	statusPath := pathBuf.Str("/proc/").Int(int64(pid)).Str("/status").String()
	status, err := os.ReadFile(statusPath) //nolint:gosec // /proc/<pid>/status of the child this fixture just started
	if err != nil {
		return fmt.Errorf("read %s: %w", statusPath, err)
	}
	for line := range strings.Lines(string(status)) {
		rest, ok := strings.CutPrefix(line, "CapEff:")
		if !ok {
			continue
		}
		held := strings.TrimSpace(rest)
		mask, err := strconv.ParseUint(held, 16, 64)
		if err != nil {
			return fmt.Errorf("parse CapEff %q: %w", held, err)
		}
		if mask&(1<<unix.CAP_NET_RAW) != 0 {
			netnsSay("GUARD: cap_net_raw is still in CapEff 0x", held, ", the daemon would open a raw socket")
			return fmt.Errorf("cap_net_raw still held by pid %d: CapEff 0x%s", pid, held)
		}
		netnsSay("DROPPED: cap_net_raw absent from CapEff 0x", held)
		return nil
	}
	return fmt.Errorf("no CapEff line in %s", statusPath)
}
