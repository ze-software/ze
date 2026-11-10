// VALIDATES: on Ze's kernel, the application override guard
// (override_guard_linux.go) refuses every per-socket IPsec policy option in Ze's
// network namespace to any process but the one that loaded it, leaves another
// network namespace alone, lets Ze's own ESP form reader policy through, and keeps
// doing so when the loader runs in a PID namespace of its own (a container); with no
// guard, the same call succeeds and the listing names it socket-override.
// PREVENTS: a guard that loads and attaches but refuses nothing, refuses Ze's own
// receive path, or reaches past Ze's SPD; and the exemption that compared the
// initial-namespace tgid with os.Getpid and so refused Ze in every container.
// METHOD: privileged QEMU (`./le test qemu all`, or `./le rfc record ... kernel`):
// loading a cgroup BPF program and attaching it at the cgroup2 root need CAP_BPF and
// CAP_SYS_ADMIN in the initial user namespace, which userns.Enter cannot grant. Each
// test runs on a locked thread in a network namespace of its own, so the guard
// governs no other test's sockets. The applications are this test binary re-run as
// TestOverrideGuardHelperProcess.

//go:build linux

package dataplane

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// The test's own link and range: the catch-all covers only the range, so nothing
// else the namespace sends meets it.
const (
	guardTestLink   = "zegrd0"
	guardTestCIDR4  = "198.51.100.129/26"
	guardTestRange4 = "198.51.100.128/26"
	guardTestPeer4  = "198.51.100.130"
	guardTestPort   = 9
)

// The helper's modes, read from guardHelperEnv by TestOverrideGuardHelperProcess.
const (
	guardHelperEnv = "ZE_TEST_OVERRIDE_GUARD_HELPER"
	// guardHelperApply joins the namespace at fd 3, tries the eight calls, sends one
	// probe, and holds its sockets until its stdin closes.
	guardHelperApply = "apply"
	// guardHelperOtherNetns makes a network namespace of its own and tries the
	// eight calls there.
	guardHelperOtherNetns = "other-netns"
	// guardHelperOwner joins the namespace at fd 3, loads the guard, sets the ESP
	// form reader's policy, and holds the guard until its stdin closes.
	guardHelperOwner = "owner"
)

// guardOption is one of the four per-socket policy options the guard refuses.
type guardOption struct {
	name  string
	level int
	opt   int
}

var guardOptions = []guardOption{
	{"ip-xfrm", unix.SOL_IP, unix.IP_XFRM_POLICY},
	{"ip-ipsec", unix.SOL_IP, unix.IP_IPSEC_POLICY},
	{"ipv6-xfrm", unix.SOL_IPV6, unix.IPV6_XFRM_POLICY},
	{"ipv6-ipsec", unix.SOL_IPV6, unix.IPV6_IPSEC_POLICY},
}

var guardFamilies = []struct {
	name   string
	family int
}{{"inet", unix.AF_INET}, {"inet6", unix.AF_INET6}}

// TestOverrideGuardHelperProcess is not a test: it is the application the kernel
// tests start, and it returns at once unless guardHelperEnv names a mode.
func TestOverrideGuardHelperProcess(t *testing.T) {
	mode := os.Getenv(guardHelperEnv)
	if mode == "" {
		return
	}
	runtime.LockOSThread()
	if err := runGuardHelper(mode, os.Stdout, os.Stdin); err != nil {
		fmt.Fprintf(os.Stdout, "error %v\n", err) //nolint:errcheck // exit status 1 reports the failure whether or not this line reaches the parent
		os.Exit(1)
	}
	os.Exit(0)
}

func runGuardHelper(mode string, out io.Writer, in io.Reader) error {
	switch mode {
	case guardHelperApply:
		if err := unix.Setns(3, unix.CLONE_NEWNET); err != nil {
			return fmt.Errorf("join the test's network namespace: %w", err)
		}
		fds, err := tryOverrides(out)
		if err != nil {
			return err
		}
		defer closeFDs(fds)
		if _, err := fmt.Fprintf(out, "probe %s\n", errnoName(unix.Sendto(fds[0], []byte("ze-guard-probe"), 0,
			&unix.SockaddrInet4{Port: guardTestPort, Addr: [4]byte(net.ParseIP(guardTestPeer4).To4())}))); err != nil {
			return fmt.Errorf("report the probe: %w", err)
		}
		if _, err := fmt.Fprintln(out, "ready"); err != nil {
			return fmt.Errorf("report ready: %w", err)
		}
		_, err = io.Copy(io.Discard, in)
		return err
	case guardHelperOtherNetns:
		if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
			return fmt.Errorf("make a network namespace: %w", err)
		}
		fds, err := tryOverrides(out)
		if err != nil {
			return err
		}
		closeFDs(fds)
		if _, err := fmt.Fprintln(out, "ready"); err != nil {
			return fmt.Errorf("report ready: %w", err)
		}
		return nil
	case guardHelperOwner:
		if err := unix.Setns(3, unix.CLONE_NEWNET); err != nil {
			return fmt.Errorf("join the test's network namespace: %w", err)
		}
		if _, err := fmt.Fprintf(out, "pid %d\n", os.Getpid()); err != nil {
			return fmt.Errorf("report the pid: %w", err)
		}
		b := &xfrmBackend{}
		if err := b.AttachOverrideGuard(); err != nil {
			return fmt.Errorf("attach the guard: %w", err)
		}
		defer b.DetachOverrideGuard() //nolint:errcheck // the helper exits next; the link closes with it
		var lc net.ListenConfig
		conn, err := lc.ListenPacket(context.Background(), espFormListenNetwork, "0.0.0.0")
		if err != nil {
			return fmt.Errorf("open the ESP form socket: %w", err)
		}
		defer conn.Close() //nolint:errcheck // a probe socket
		if _, err := fmt.Fprintf(out, "esp-form %s\n", errnoName(espFormBypassInboundPolicy(conn))); err != nil {
			return fmt.Errorf("report the ESP form policy: %w", err)
		}
		if _, err := fmt.Fprintln(out, "ready"); err != nil {
			return fmt.Errorf("report ready: %w", err)
		}
		_, err = io.Copy(io.Discard, in)
		return err
	}
	return fmt.Errorf("unknown helper mode %q", mode)
}

// tryOverrides sets an ALLOW out policy through each option on a socket of each
// family, reports each result as "call <family>/<option> <errno>", and answers the
// sockets, IPv4 first, still open. A report that cannot be written closes the
// sockets and answers the write error.
func tryOverrides(out io.Writer) ([]int, error) {
	var fds []int
	for _, f := range guardFamilies {
		fd, err := unix.Socket(f.family, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
		if err != nil {
			if _, werr := fmt.Fprintf(out, "socket %s %s\n", f.name, errnoName(err)); werr != nil {
				closeFDs(fds)
				return nil, fmt.Errorf("report the socket: %w", werr)
			}
			continue
		}
		fds = append(fds, fd)
		for _, o := range guardOptions {
			err := unix.SetsockoptString(fd, o.level, o.opt, string(socketOverridePolicy(f.family).Serialize()))
			if _, werr := fmt.Fprintf(out, "call %s/%s %s\n", f.name, o.name, errnoName(err)); werr != nil {
				closeFDs(fds)
				return nil, fmt.Errorf("report the call: %w", werr)
			}
		}
	}
	return fds, nil
}

func closeFDs(fds []int) {
	for _, fd := range fds {
		unix.Close(fd) //nolint:errcheck // a probe socket; nothing was written through it that matters
	}
}

// socketOverridePolicy is what an application would set: ALLOW out, empty
// selector, no lifetime, priority 0 (not Ze's mark).
func socketOverridePolicy(family int) *nl.XfrmUserpolicyInfo {
	return &nl.XfrmUserpolicyInfo{
		Sel: nl.XfrmSelector{Family: uint16(family)},
		Lft: nl.XfrmLifetimeCfg{
			SoftByteLimit:   xfrmInfinity,
			HardByteLimit:   xfrmInfinity,
			SoftPacketLimit: xfrmInfinity,
			HardPacketLimit: xfrmInfinity,
		},
		Dir:    1, // XFRM_POLICY_OUT
		Action: xfrmPolicyActionAllow,
	}
}

func errnoName(err error) string {
	if err == nil {
		return "ok"
	}
	if errno, ok := errors.AsType[syscall.Errno](err); ok {
		if name := unix.ErrnoName(errno); name != "" {
			return name
		}
	}
	return strings.ReplaceAll(err.Error(), " ", "_")
}

// guardHelper is a running helper and the lines it reported before "ready".
type guardHelper struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	reports map[string]string
}

// startGuardHelper runs the helper in mode, the test's namespace at its fd 3, with
// extra clone flags, and reads its report up to "ready".
func startGuardHelper(t *testing.T, mode string, nsFile *os.File, cloneFlags uintptr) *guardHelper {
	t.Helper()
	// No deadline: stop, from the test's cleanup, ends the helper by closing its
	// stdin and judges its exit, which a context canceled at test end would preempt.
	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestOverrideGuardHelperProcess$", "-test.count=1")
	cmd.Env = append(os.Environ(), guardHelperEnv+"="+mode)
	cmd.ExtraFiles = []*os.File{nsFile}
	cmd.SysProcAttr = &syscall.SysProcAttr{Cloneflags: cloneFlags}
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("helper stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("helper stdout: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start the %s helper: %v", mode, err)
	}
	h := &guardHelper{cmd: cmd, stdin: stdin, reports: map[string]string{}}
	t.Cleanup(func() { h.stop(t) })
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "ready" {
			return h
		}
		key, value, _ := strings.Cut(line, " ")
		if key == "call" || key == "socket" {
			key, value, _ = strings.Cut(value, " ")
		}
		h.reports[key] = value
	}
	t.Fatalf("the %s helper stopped before ready: %v", mode, h.reports)
	return nil
}

// stop closes the helper's stdin, which ends its hold, and waits for it. A second
// call does nothing.
func (h *guardHelper) stop(t *testing.T) {
	t.Helper()
	if h.stdin == nil {
		return
	}
	h.stdin.Close() //nolint:errcheck // closing is the signal; the wait below reports the outcome
	h.stdin = nil
	if err := h.cmd.Wait(); err != nil {
		t.Errorf("helper exit: %v (reports %v)", err, h.reports)
	}
}

// enterGuardNetns skips unless the test runs as root, then moves the test's thread
// into a network namespace of its own for the rest of the test, carrying lo and a
// dummy link that holds the test range. It answers the namespace as a file to hand
// to a helper.
func enterGuardNetns(t *testing.T) *os.File {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("requires privileged QEMU: loading a cgroup BPF program and attaching it at the cgroup2 root need CAP_BPF and CAP_SYS_ADMIN in the initial user namespace, which userns.Enter cannot grant; run `./le test qemu all`")
	}
	runtime.LockOSThread()
	origin, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("read the thread's network namespace: %v", err)
	}
	ns, err := netns.New()
	if err != nil {
		origin.Close() //nolint:errcheck // the unshare already failed; that error is the one reported
		runtime.UnlockOSThread()
		t.Fatalf("make a network namespace: %v", err)
	}
	t.Cleanup(func() {
		ns.Close() //nolint:errcheck // the namespace goes with its last reference
		if err := netns.Set(origin); err != nil {
			// The thread stays locked, so the runtime retires it with the namespace.
			t.Errorf("return the thread to its network namespace: %v", err)
			return
		}
		origin.Close() //nolint:errcheck // restored above; the handle is spent
		runtime.UnlockOSThread()
	})
	nsFile, err := os.Open("/proc/thread-self/ns/net")
	if err != nil {
		t.Fatalf("open the test's network namespace: %v", err)
	}
	t.Cleanup(func() { nsFile.Close() }) //nolint:errcheck // read only

	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("find lo: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatalf("bring lo up: %v", err)
	}
	if err := netlink.LinkAdd(&netlink.Dummy{Name: guardTestLink}); err != nil {
		t.Fatalf("add %s: %v", guardTestLink, err)
	}
	link, err := netlink.LinkByName(guardTestLink)
	if err != nil {
		t.Fatalf("find %s: %v", guardTestLink, err)
	}
	addr, err := netlink.ParseAddr(guardTestCIDR4)
	if err != nil {
		t.Fatalf("parse %s: %v", guardTestCIDR4, err)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		t.Fatalf("address %s: %v", guardTestLink, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("bring %s up: %v", guardTestLink, err)
	}
	return nsFile
}

// guardCatchAll installs an if_id 0 DISCARD entry for the test range, the shape of
// the SPD's final entry, so a probe that reaches the SPD is blocked and counted.
func guardCatchAll(t *testing.T, b *xfrmBackend) {
	t.Helper()
	installForTest(t, b, SPParams{
		Src: mustCIDR(t, "0.0.0.0/0"), Dst: mustCIDR(t, guardTestRange4),
		Dir: SADirOut, Action: SPActionDiscard, Priority: PriorityUnmatched,
	})
}

// threadXfrmStat reads one counter of the test thread's own network namespace:
// /proc/net follows the thread group leader, which stayed outside.
func threadXfrmStat(t *testing.T, name string) int { //nolint:unparam // a reader of any xfrm_stat counter; renaming it to the one counter read today would change the bodies of two RFC-recorded units
	t.Helper()
	raw, err := os.ReadFile("/proc/thread-self/net/xfrm_stat")
	if err != nil {
		t.Fatalf("reading /proc/thread-self/net/xfrm_stat: %v; it is the instrument this test measures with", err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == name {
			n, convErr := strconv.Atoi(fields[1])
			if convErr != nil {
				t.Fatalf("parsing %s %q: %v", name, fields[1], convErr)
			}
			return n
		}
	}
	t.Fatalf("%s is absent from /proc/thread-self/net/xfrm_stat", name)
	return 0
}

// socketRows answers the per-socket rows of the namespace's policy listing.
func socketRows(t *testing.T, b *xfrmBackend) []PolicyInfo {
	t.Helper()
	all, err := b.ListPolicies()
	if err != nil {
		t.Fatalf("list policies: %v", err)
	}
	var rows []PolicyInfo
	for i := range all {
		if all[i].PerSocket {
			rows = append(rows, all[i])
		}
	}
	return rows
}

// openESPFormSocket opens the ESP form reader's socket and sets its policy, as
// startLocked does.
func openESPFormSocket(t *testing.T) error {
	t.Helper()
	var lc net.ListenConfig
	conn, err := lc.ListenPacket(context.Background(), espFormListenNetwork, "0.0.0.0")
	if err != nil {
		t.Fatalf("open the ESP form socket: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // a probe socket
	return espFormBypassInboundPolicy(conn)
}

// RFC requirement: RFC4301-4.4.1-8 positive -- with the guard attached, every per-socket IPsec policy option, in both socket families, set by a process other than the one that loaded the guard in Ze's network namespace fails with EPERM and is counted, and that process's traffic still meets the SPD's DISCARD entry; the same calls in another network namespace are not refused; Ze's own ESP form socket policy is set and listed as esp-form-socket; and with the loader in a PID namespace of its own, its ESP form policy is still set while a process outside that namespace is refused.
//
// RFC 4301 Section 4.4.1: "However, the system administrator MUST be able to specify
// whether or not a user or application can override (default) system policies."
//
// TestRFC4301ApplicationOverrideDeniedInKernel measures AC-14, AC-15, AC-16 and A8-1.
//
//ze:guest-root
func TestRFC4301ApplicationOverrideDeniedInKernel(t *testing.T) {
	nsFile := enterGuardNetns(t)
	b := &xfrmBackend{}
	guardCatchAll(t, b)
	if err := b.AttachOverrideGuard(); err != nil {
		t.Fatalf("attach the guard: %v", err)
	}
	t.Cleanup(func() {
		if err := b.DetachOverrideGuard(); err != nil {
			t.Errorf("detach the guard: %v", err)
		}
	})

	// AC-14: an application in Ze's network namespace.
	blocked := threadXfrmStat(t, "XfrmOutPolBlock")
	app := startGuardHelper(t, guardHelperApply, nsFile, 0)
	for _, f := range guardFamilies {
		for _, o := range guardOptions {
			key := f.name + "/" + o.name
			if got := app.reports[key]; got != "EPERM" {
				t.Errorf("AC-14: %s answered %s, want EPERM", key, got)
			}
		}
	}
	if got := app.reports["probe"]; got != "EPERM" {
		t.Errorf("AC-14: the application's probe answered %s, want EPERM from the DISCARD entry", got)
	}
	if moved := threadXfrmStat(t, "XfrmOutPolBlock") - blocked; moved < 1 {
		t.Errorf("AC-14: XfrmOutPolBlock moved %d, want the probe blocked by the SPD", moved)
	}
	app.stop(t)
	state, err := b.OverrideGuardState()
	if err != nil {
		t.Fatalf("read the guard: %v", err)
	}
	refusals := uint64(len(guardFamilies) * len(guardOptions))
	if state.Status != OverrideGuardAttached || state.Refusals != refusals {
		t.Errorf("AC-14: guard %v with %d refusals, want attached with %d", state.Status, state.Refusals, refusals)
	}

	// AC-15: the same application in another network namespace.
	other := startGuardHelper(t, guardHelperOtherNetns, nsFile, 0)
	for key, got := range other.reports {
		if got == "EPERM" {
			t.Errorf("AC-15: %s answered EPERM in another network namespace", key)
		}
	}
	for _, key := range []string{"inet/ip-xfrm", "inet6/ipv6-xfrm"} {
		if got := other.reports[key]; got != "ok" {
			t.Errorf("AC-15: %s answered %s in another network namespace, want ok", key, got)
		}
	}
	other.stop(t)
	if state, err = b.OverrideGuardState(); err != nil || state.Refusals != refusals {
		t.Errorf("AC-15: refusals %d (err %v), want still %d", state.Refusals, err, refusals)
	}

	// AC-16: Ze's own ESP form reader.
	if err := openESPFormSocket(t); err != nil {
		t.Fatalf("AC-16: the ESP form policy was refused under the guard: %v", err)
	}
	var marked bool
	for _, row := range socketRows(t, b) {
		marked = marked || row.Role == PolicyRoleESPFormSocket
	}
	if !marked {
		t.Errorf("AC-16: no socket row reads esp-form-socket: %+v", socketRows(t, b))
	}

	// The loader in a PID namespace of its own: its tgid there is 1, never the
	// number the initial namespace knows it by.
	if err := b.DetachOverrideGuard(); err != nil {
		t.Fatalf("detach this process's guard before the owner leg: %v", err)
	}
	owner := startGuardHelper(t, guardHelperOwner, nsFile, unix.CLONE_NEWPID)
	if got := owner.reports["pid"]; got != "1" {
		t.Fatalf("the owner runs as pid %s, want 1 in its own PID namespace", got)
	}
	if got := owner.reports["esp-form"]; got != "ok" {
		t.Errorf("PID namespace: the loader's own ESP form policy answered %s, want ok", got)
	}
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open a socket: %v", err)
	}
	defer unix.Close(fd) //nolint:errcheck // a probe socket
	err = unix.SetsockoptString(fd, unix.SOL_IP, unix.IP_XFRM_POLICY, string(socketOverridePolicy(unix.AF_INET).Serialize()))
	if got := errnoName(err); got != "EPERM" {
		t.Errorf("PID namespace: a process outside the loader's namespace answered %s, want EPERM", got)
	}
	owner.stop(t)
}

// RFC requirement: RFC4301-4.4.1-8 negative -- with no guard attached (application-override allow), a process in Ze's network namespace sets an ALLOW socket policy through IP_XFRM_POLICY and IPV6_XFRM_POLICY, its traffic then passes the SPD's DISCARD entry, and the listing reads its policy as socket-override, direction out, SPD socket.
//
// TestRFC4301ApplicationOverrideAllowedInKernel measures AC-17 and AC-20.
//
//ze:guest-root
func TestRFC4301ApplicationOverrideAllowedInKernel(t *testing.T) {
	nsFile := enterGuardNetns(t)
	b := &xfrmBackend{}
	guardCatchAll(t, b)

	blocked := threadXfrmStat(t, "XfrmOutPolBlock")
	app := startGuardHelper(t, guardHelperApply, nsFile, 0)
	for _, key := range []string{"inet/ip-xfrm", "inet6/ipv6-xfrm"} {
		if got := app.reports[key]; got != "ok" {
			t.Errorf("AC-17: %s answered %s with no guard, want ok", key, got)
		}
	}
	if got := app.reports["probe"]; got == "EPERM" {
		t.Errorf("AC-17: the overriding socket's probe was blocked; its policy did not override the SPD")
	}
	if moved := threadXfrmStat(t, "XfrmOutPolBlock") - blocked; moved != 0 {
		t.Errorf("AC-17: XfrmOutPolBlock moved %d, want the probe past the SPD", moved)
	}

	rows := socketRows(t, b)
	if len(rows) < 2 {
		t.Fatalf("AC-20: %d socket rows while the application holds two policies: %+v", len(rows), rows)
	}
	for _, row := range rows {
		if row.Role != PolicyRoleSocketOverride || row.Dir != SADirOut || row.SPDName() != spdNameSocket {
			t.Errorf("AC-20: row %+v, want socket-override, out, SPD socket", row)
		}
	}
	app.stop(t)
}

// TestESPFormSocketPolicyCarriesZeMarkInKernel proves A8-4 on the running kernel: the
// ESP form reader's socket policy keeps the priority Ze wrote, so the listing reads it
// as esp-form-socket, direction in, SPD socket.
//
// VALIDATES: espFormBypassInboundPolicy's Priority, and socketPolicyRole over the dump.
// PREVENTS: a mark the kernel drops, which would list Ze's own receive path as an
// application's override.
//
//ze:guest-root
func TestESPFormSocketPolicyCarriesZeMarkInKernel(t *testing.T) {
	enterGuardNetns(t)
	b := &xfrmBackend{}
	if err := openESPFormSocket(t); err != nil {
		t.Fatalf("set the ESP form policy: %v", err)
	}
	rows := socketRows(t, b)
	if len(rows) != 1 {
		t.Fatalf("%d socket rows, want the ESP form reader's one: %+v", len(rows), rows)
	}
	row := rows[0]
	if row.Priority != PriorityESPFormSocket {
		t.Errorf("priority %d, want Ze's mark %d", row.Priority, PriorityESPFormSocket)
	}
	if row.Role != PolicyRoleESPFormSocket || row.Dir != SADirIn || row.SPDName() != spdNameSocket {
		t.Errorf("row %+v, want esp-form-socket, in, SPD socket", row)
	}
}
