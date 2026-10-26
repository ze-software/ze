// Design: docs/architecture/testing/qemu-integration.md -- the QEMU forward's host port
// Related: run.go -- runOps.Port, which answers the run, docker-lab and installer forwards

package testqemu

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"syscall"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/runner"
)

// forwardHost is the one address every QEMU forward in this package binds
// (hostfwd and the crash-capture monitor), and the address every client dials.
// One address for all of them is what keeps two VMs from sharing a port: a
// second QEMU binding the same address refuses at start, where a wildcard
// forward beside a loopback one binds on macOS (slirp sets SO_REUSEADDR) and
// BSD's most-specific-bind lookup then sends every loopback ssh to the
// loopback VM. Nothing reaches these forwards from off the host.
const forwardHost = "127.0.0.1"

// The band the forward ports come from. A port in it reads as an SSH forward
// in `ss` or `lsof`, and the band is the one the operator overrides with
// ze.qemu.ssh.port or ze.install.ssh.port. It starts at 2223, not 2222:
// 2222 is the SSH port `ze appliance run` forwards by default (on the
// wildcard) and Ze's own CLI SSH default (127.0.0.1:2222). On macOS a le VM
// holding 127.0.0.1:2222 binds beside the appliance's wildcard forward, and
// `ssh -p 2222 localhost` then reaches le's VM rather than the appliance.
const (
	forwardPortFirst = 2223
	forwardPortLast  = 2322
)

// forwardReservations holds the lock on every port this process has handed a
// QEMU, for the rest of the process, and the kernel drops the locks when le
// exits however it exits. The lock covers the VM only while its le lives. A
// run or docker-lab VM dies with its le on Linux (childProcAttr), and the next
// run's reap stops it on macOS (vmrecord.go). The installer, hugepages,
// crash-capture and MPLS boot VMs carry neither, so a SIGKILLed le leaves them
// running with their port and no lock: forwardPortFree finds that live
// listener, which is why the probe runs after the lock is taken. Holding the
// locks to the end, rather than to the VM's stop, costs one port for each
// forward an action boots, a few per action, and needs no release on any of
// the stop paths. Safe for concurrent use.
var forwardReservations struct {
	sync.Mutex
	held []*runner.PortReservation
}

// reserveForwardPort answers a host port for one QEMU forward and keeps it
// reserved for the life of this process.
func reserveForwardPort() (int, error) {
	return reserveForwardPortIn(forwardPortFirst, forwardPortLast)
}

// reserveForwardPortIn answers the first port in first..last that no other
// reservation holds and that forwardPortFree finds unbound, and keeps its
// reservation. The reservation is the runner's per-port lock
// (/tmp/le-port-locks-<uid>), which every le process shares: a concurrent le
// run, or a second request in this one, skips the port even before QEMU binds
// it, which the bind probe alone cannot see.
func reserveForwardPortIn(first, last int) (int, error) {
	for port := first; port <= last; port++ {
		reservation, ok, err := runner.LockPort(port)
		if err != nil {
			return 0, fmt.Errorf("reserve a host port for the QEMU forward: %w", err)
		}
		if !ok {
			continue
		}
		free, err := forwardPortFree(port)
		if err != nil {
			reservation.Release()
			return 0, err
		}
		if !free {
			reservation.Release()
			continue
		}
		forwardReservations.Lock()
		forwardReservations.held = append(forwardReservations.held, reservation)
		forwardReservations.Unlock()
		return port, nil
	}
	var b textbuf.Buffer
	return 0, errors.New(b.Str("no free host port in ").Int(int64(first)).Str("..").Int(int64(last)).
		Str(" for a QEMU forward: every one is bound by another program, or reserved by a live le process, ").
		Str("each of which keeps the ports it reserved until it exits. ").
		Str("`lsof +D /tmp/le-port-locks-").Int(int64(os.Geteuid())).
		Str("` names the processes holding a reservation, and `lsof -nP -iTCP:").Int(int64(first)).
		Str("-").Int(int64(last)).Str("` the programs bound in the band. ").
		Str("Fix: stop a VM or le run you no longer need, or set a free port outside the band: ").
		Str("ze.qemu.ssh.port (ZE_QEMU_SSH_PORT) for run, docker-lab and the MPLS boot, ").
		Str("ze.install.ssh.port (ZE_INSTALL_SSH_PORT) for installer, ").
		Str("ze.vpp.hp.ssh.port (ZE_VPP_HP_SSH_PORT) for the hugepages proof. ").
		Str("Crash capture has no fixed-port setting and needs free ports in the band.").String())
}

// forwardProbeHosts are the addresses the probe binds: the forward's own, and
// the wildcard, because another program's wildcard listener (the kernel build
// VM, `ze appliance run`) refuses a loopback forward on Linux and splits the
// port with it on macOS.
var forwardProbeHosts = [...]string{forwardHost, "0.0.0.0"}

// forwardPortFree binds port on every probe address, IPv4 as slirp binds, with
// SO_REUSEADDR cleared. With it set, as Go sets it on every listener, macOS
// lets the probe bind beside a listening holder on another address, and Linux
// lets it bind beside a forward bound and not yet listening; without it, any
// overlapping holder refuses the bind on both. An error answers a probe that
// could not be released, never a held port.
func forwardPortFree(port int) (bool, error) {
	config := net.ListenConfig{Control: clearReuseAddress}
	for _, host := range forwardProbeHosts {
		listener, err := config.Listen(context.Background(), "tcp4", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			return false, nil //nolint:nilerr // a refused bind is the answer "held", not a failure
		}
		if err := listener.Close(); err != nil {
			return false, fmt.Errorf("release the QEMU forward port probe %s:%d: %w. A fixed SSH port skips this probe. Fix: set ze.qemu.ssh.port (ZE_QEMU_SSH_PORT) for run and docker-lab, or ze.install.ssh.port (ZE_INSTALL_SSH_PORT) for installer, to a free port", host, port, err)
		}
	}
	return true, nil
}

// clearReuseAddress runs after Go has set SO_REUSEADDR and before the bind.
func clearReuseAddress(_, _ string, raw syscall.RawConn) error {
	var optionErr error
	err := raw.Control(func(fd uintptr) {
		optionErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 0)
	})
	if err != nil {
		return err
	}
	return optionErr
}
