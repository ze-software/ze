package testqemu

import (
	"net"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/appliance"
	"github.com/ze-software/ze/internal/test/runner"
)

// VALIDATES: the SSH port probe refuses a port another process holds on the
// address a QEMU hostfwd binds, loopback or wildcard, so two concurrent runs
// never hand QEMU the same port.
// METHOD: hold 127.0.0.1:<port> (the docker-lab forward), then 0.0.0.0:<port>
// (the run and installer forward), each with SO_REUSEADDR set as QEMU's slirp
// sets it, and ask the probe for exactly that port.
// PREVENTS: the 2026-10-21 docker-lab collision, where a wildcard probe bound
// beside another VM's 127.0.0.1 forward and the second run died with an sshd
// timeout.
func TestForwardPortRefusesAHeldForward(t *testing.T) {
	for _, address := range []string{"127.0.0.1:0", "0.0.0.0:0"} {
		t.Run(address, func(t *testing.T) {
			var config net.ListenConfig
			held, err := config.Listen(t.Context(), "tcp4", address)
			if err != nil {
				t.Fatalf("hold %s: %v", address, err)
			}
			defer held.Close() //nolint:errcheck // the test's own listener
			tcp, ok := held.Addr().(*net.TCPAddr)
			if !ok {
				t.Fatalf("listener address %v is not TCP", held.Addr())
			}
			if port, err := reserveForwardPortIn(tcp.Port, tcp.Port); err == nil {
				t.Fatalf("probe answered port %d while %s holds it", port, held.Addr())
			}
		})
	}
}

// VALIDATES: the probe refuses a port another process has bound on 127.0.0.1
// and not yet listened on, the window between another QEMU's bind and its
// listen.
// METHOD: bind a raw socket to 127.0.0.1:0 with SO_REUSEADDR, as slirp does,
// without listening, then ask the probe for that port. On Linux a probe that
// keeps SO_REUSEADDR set binds beside it; this is the red Linux can show, the
// listening case above being red on macOS.
// PREVENTS: a probe whose own SO_REUSEADDR lets it share a forward's port.
func TestForwardPortRefusesABoundLoopbackForward(t *testing.T) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("open a socket: %v", err)
	}
	defer syscall.Close(fd) //nolint:errcheck // the test's own socket
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		t.Fatalf("set SO_REUSEADDR: %v", err)
	}
	if err := syscall.Bind(fd, &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}); err != nil {
		t.Fatalf("bind 127.0.0.1:0: %v", err)
	}
	bound, err := syscall.Getsockname(fd)
	if err != nil {
		t.Fatalf("read the bound port: %v", err)
	}
	inet, ok := bound.(*syscall.SockaddrInet4)
	if !ok {
		t.Fatalf("bound address %v is not IPv4", bound)
	}
	if port, err := reserveForwardPortIn(inet.Port, inet.Port); err == nil {
		t.Fatalf("probe answered port %d while 127.0.0.1:%d is bound", port, inet.Port)
	}
}

// VALIDATES: the probe still answers a port nobody holds.
// METHOD: take a free loopback port from the kernel, release it, then probe it.
// PREVENTS: a probe so strict it refuses every port and stops every run.
func TestForwardPortAnswersAFreePort(t *testing.T) {
	var config net.ListenConfig
	listener, err := config.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("take a port: %v", err)
	}
	tcp, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address %v is not TCP", listener.Addr())
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("release the port: %v", err)
	}
	port, err := reserveForwardPortIn(tcp.Port, tcp.Port)
	if err != nil {
		t.Fatalf("probe refused free port %d: %v", tcp.Port, err)
	}
	if port != tcp.Port {
		t.Fatalf("probe answered %d, want %d", port, tcp.Port)
	}
}

// VALIDATES: two forward ports asked for in one run are distinct, so the
// installer SSH and web forwards of one VM never name the same port.
// METHOD: take a free loopback port from the kernel, release it, then ask for
// two ports from a range that starts there, with nothing bound in between.
// PREVENTS: the 2026-10-22 installer collision, where bootTargetSSH asked
// the probe twice and both forwards took its first free port.
func TestForwardPortsInOneRunAreDistinct(t *testing.T) {
	first := releasedLoopbackPort(t)
	ssh, err := reserveForwardPortIn(first, first+20)
	if err != nil {
		t.Fatalf("first port: %v", err)
	}
	web, err := reserveForwardPortIn(first, first+20)
	if err != nil {
		t.Fatalf("second port: %v", err)
	}
	if ssh == web {
		t.Fatalf("both requests answered port %d", ssh)
	}
}

// VALIDATES: a port another le run holds a reservation on is skipped, though
// nothing is bound to it yet.
// METHOD: lock a free port through the runner's cross-process lock, as a
// concurrent le run holds the port it chose until its QEMU binds it, then ask
// for exactly that port.
// PREVENTS: two le runs probing in the same instant and handing two QEMUs one
// port (the 2026-10-10 row of parallel-copies-collide-on-a-deterministic-port).
func TestForwardPortSkipsAReservedPort(t *testing.T) {
	port, held := reservedReleasedLoopbackPort(t)
	defer held.Release()
	if got, err := reserveForwardPortIn(port, port); err == nil {
		t.Fatalf("probe answered port %d while another run holds its reservation", got)
	}
}

// VALIDATES: the error for an exhausted band names every fixed-port setting
// that reaches it, says a live le keeps its reservations, and names the
// command that shows the holders.
// METHOD: reserve a free port through the runner's lock, ask for exactly that
// port, and read the error.
// PREVENTS: an installer or hugepages operator told to set the run's key,
// which their action never reads.
func TestForwardPortExhaustionNamesEveryFix(t *testing.T) {
	port, held := reservedReleasedLoopbackPort(t)
	defer held.Release()
	_, err := reserveForwardPortIn(port, port)
	if err == nil {
		t.Fatalf("probe answered port %d while another run holds its reservation", port)
	}
	for _, want := range []string{
		"live le process",
		"lsof +D ",
		"ze.qemu.ssh.port (ZE_QEMU_SSH_PORT)",
		"ze.install.ssh.port (ZE_INSTALL_SSH_PORT)",
		"ze.vpp.hp.ssh.port (ZE_VPP_HP_SSH_PORT)",
		"Crash capture",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// releasedLoopbackPort answers a port the kernel just handed out and released.
// reservedReleasedLoopbackPort answers a port nothing is bound to, with the
// runner's cross-process lock on it held for the caller, who MUST Release it.
//
// The kernel knows nothing of those locks: a port it hands out as free can
// still be reserved, by another le run or by this test process itself, whose
// earlier reserveForwardPortIn calls keep their reservations until it exits.
// Such a port is not the one the test asked for, so the helper takes another,
// up to reservedPortAttempts times, rather than failing the test on it.
func reservedReleasedLoopbackPort(t *testing.T) (int, *runner.PortReservation) {
	t.Helper()
	for range reservedPortAttempts {
		port := releasedLoopbackPort(t)
		held, ok, err := runner.LockPort(port)
		if err != nil {
			t.Fatalf("reserve port %d: %v", port, err)
		}
		if ok {
			return port, held
		}
	}
	t.Fatalf("every one of %d free loopback ports was already reserved", reservedPortAttempts)
	return 0, nil
}

// reservedPortAttempts bounds reservedReleasedLoopbackPort's search.
const reservedPortAttempts = 32

func releasedLoopbackPort(t *testing.T) int {
	t.Helper()
	var config net.ListenConfig
	listener, err := config.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("take a port: %v", err)
	}
	tcp, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address %v is not TCP", listener.Addr())
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("release the port: %v", err)
	}
	return tcp.Port
}

// VALIDATES: the band le takes forward ports from excludes the SSH port
// `ze appliance run` forwards by default, and Ze's own CLI SSH default.
// METHOD: read the appliance default from appliance.DefaultConfig, and the
// CLI default as a literal (defaultListen in internal/component/ssh/ssh.go is
// unexported), and require each to fall outside forwardPortFirst..forwardPortLast.
// PREVENTS: finding F-2 on bf8fff3d54, where on macOS a le VM started first and
// an appliance started second both bound 2222, and `ssh -p 2222 localhost`
// reached le's VM rather than the appliance.
func TestForwardPortBandExcludesTheApplianceSSHPort(t *testing.T) {
	const cliSSHPort = 2222 // internal/component/ssh/ssh.go, defaultListen 127.0.0.1:2222
	ports := map[string]int{
		"ze appliance run SSH forward": appliance.DefaultConfig("band").QEMU.SSHPort,
		"ze CLI SSH listener":          cliSSHPort,
	}
	for name, port := range ports {
		if port >= forwardPortFirst && port <= forwardPortLast {
			t.Errorf("%s default %d lies in the le forward band %d..%d", name, port, forwardPortFirst, forwardPortLast)
		}
	}
}
