//go:build linux

// Design: docs/architecture/diagnostics/active-probes.md -- the Don't Fragment mode
// Design: docs/architecture/diagnostics/path-mtu.md -- the show mtu run
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
// to.
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
	"runtime"
	"strconv"
	"strings"
	"syscall"

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

	// clampedPingGroupRange admits every group to the datagram ICMP socket
	// inside the sender namespace, where the range is per namespace.
	clampedPingGroupRange = "0 65535"
	// clampedFarDaemonEnv keeps a far-side responder off the real xfrm
	// dataplane; only the IKE negotiation is wanted from it.
	clampedFarDaemonEnv = "ze_test_ike_dataplane=noop"

	procSysIPForward      = "/proc/sys/net/ipv4/ip_forward"
	procSysPingGroupRange = "/proc/sys/net/ipv4/ping_group_range"
)

// clampedFarAddrs are the far namespace's addresses: the probed host, a second
// IKE responder and the `show mtu` reference address.
var clampedFarAddrs = []string{"10.99.2.1/24", "10.99.2.3/24", "10.99.2.9/24"}

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
	// withoutNetRaw drops CAP_NET_RAW from the command's bounding set.
	withoutNetRaw bool
	// command is the argv run in the sender namespace.
	command []string
}

// parseNetnsRunArgs reads `netns <prefix> [route-mtu <octets>]
// [far-daemon <conf>]... [without-net-raw] run <argv...>`. Every keyword
// precedes its value, and `run` takes the rest of the line.
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
		case "without-net-raw":
			plan.withoutNetRaw = true
		case "run":
			plan.command = args[index+1:]
			index = len(args)
		default:
			return nil, fmt.Errorf("unknown argument %q; expected netns, route-mtu, far-daemon, without-net-raw or run", args[index])
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
	if plan.withoutNetRaw {
		if err := p.sender.writeSysctl(orig, procSysPingGroupRange, clampedPingGroupRange); err != nil {
			return err
		}
	}
	return nil
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

	return runInNetns(ctx, plan, path.sender, orig)
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
// far-daemon argument, off the real dataplane. A daemon dies with this process
// (Pdeathsig) and with the context, so a killed fixture leaves no responder
// behind.
func startFarDaemons(ctx context.Context, plan *netnsRunPlan, far *testNetns, orig netns.NsHandle) ([]*exec.Cmd, error) {
	if len(plan.farDaemons) == 0 {
		return nil, nil
	}
	if err := netns.Set(far.ns); err != nil {
		return nil, fmt.Errorf("enter %s: %w", far.name, err)
	}
	var daemons []*exec.Cmd
	var startErr error
	for _, conf := range plan.farDaemons {
		daemon := exec.CommandContext(ctx, "ze", "start", conf) //nolint:gosec // test fixture; the argument is the .ci's own file
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

// stopFarDaemons kills and reaps each far daemon, as the script's trap did.
func stopFarDaemons(daemons []*exec.Cmd) {
	for _, daemon := range daemons {
		daemon.Process.Kill() //nolint:errcheck // a daemon that already exited is fine
		daemon.Wait()         //nolint:errcheck // the exit status of a killed responder is not an assertion
	}
}

// runInNetns starts the plan's command inside `ns`, on the locked calling
// thread, and waits for it. With withoutNetRaw the bounding set loses
// CAP_NET_RAW first, and the child's CapEff is read back before the run is
// allowed to count. The thread returns to `orig` once the child has started,
// and the context kills the child when the runner tears the test down.
func runInNetns(ctx context.Context, plan *netnsRunPlan, ns *testNetns, orig netns.NsHandle) error {
	if err := netns.Set(ns.ns); err != nil {
		return fmt.Errorf("enter %s: %w", ns.name, err)
	}
	if plan.withoutNetRaw {
		// PR_CAPBSET_DROP is per thread and irreversible; the command below is
		// forked from this thread and inherits the reduced bounding set.
		if err := unix.Prctl(unix.PR_CAPBSET_DROP, unix.CAP_NET_RAW, 0, 0, 0); err != nil {
			return fmt.Errorf("drop CAP_NET_RAW from the bounding set: %w", err)
		}
	}
	command := exec.CommandContext(ctx, plan.command[0], plan.command[1:]...) //nolint:gosec // test fixture; the argv is the .ci's own line
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	startErr := command.Start()
	if err := netns.Set(orig); err != nil {
		return fmt.Errorf("return from %s: %w", ns.name, err)
	}
	if startErr != nil {
		return fmt.Errorf("start %s in %s: %w", strings.Join(plan.command, " "), ns.name, startErr)
	}
	if plan.withoutNetRaw {
		if err := confirmNetRawDropped(command.Process.Pid); err != nil {
			command.Process.Kill() //nolint:errcheck // the child is refused whatever it is doing
			command.Wait()         //nolint:errcheck // reaped only
			return err
		}
	}
	if err := command.Wait(); err != nil {
		return fmt.Errorf("%s in %s: %w", strings.Join(plan.command, " "), ns.name, err)
	}
	return nil
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
