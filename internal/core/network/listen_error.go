// Design: docs/architecture/behavior/peer-lifecycle.md — a listener the kernel refuses, in the operator's terms
// Related: network.go — ListenerFactory, whose Listen returns the refusal this file explains
// Related: ../../component/doctor/checks_listener.go — the bind probe that reports the same refusal before a start

package network

import (
	"errors"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// unprivilegedPortStartPath is the sysctl net.ipv4.ip_unprivileged_port_start:
// the lowest port a process without CAP_NET_BIND_SERVICE may bind
// (Documentation/networking/ip-sysctl.rst). The kernel keeps it per network
// namespace, and /proc/sys answers for the namespace of the reading process,
// which is the one that binds.
const unprivilegedPortStartPath = "/proc/sys/net/ipv4/ip_unprivileged_port_start"

// capNetBindService is the capability a port below the unprivileged start
// needs, spelled as capabilities(7) and privilege.CapabilityRemedy spell it.
const capNetBindService = "CAP_NET_BIND_SERVICE"

// policyDenialCommand shows the kernel's record of a security policy that
// refused a call: AppArmor logs its denials to the kernel log, and SELinux does
// too when auditd does not run.
const policyDenialCommand = "sudo journalctl -k -g denied"

// addressListCommand lists the addresses this host holds, so the operator can
// compare them with the configured one.
const addressListCommand = "ip -brief address show"

// unknownCauseCommand is the fix for a refusal this file does not explain:
// the doctor's listener probe binds every configured endpoint again and names
// what it finds.
const unknownCauseCommand = "ze doctor"

// ListenSite names one listening socket the configuration asked for. It holds
// what an operator needs to find the configuration in a refusal.
type ListenSite struct {
	// Network is "tcp" or "udp", as net.Listen and net.ListenPacket take it.
	Network string
	// Endpoint is the configured address and port.
	Endpoint netip.AddrPort
	// Owner names what configured the listener, as the operator wrote it:
	// "bgp peer upstream1", "bgp group rs-clients", "ssh". Empty when the
	// caller cannot name one.
	Owner string
}

// unprivilegedPortStart reads the sysctl. It is a variable so a test can
// answer for the kernel, including a kernel whose value cannot be read.
var unprivilegedPortStart = UnprivilegedPortStart

// UnprivilegedPortStart answers net.ipv4.ip_unprivileged_port_start for the
// network namespace of this process: a port below it needs
// CAP_NET_BIND_SERVICE. It answers an error when the value cannot be read or
// parsed (no /proc, a kernel older than 4.11, a platform other than Linux), and
// never a guessed 1024.
func UnprivilegedPortStart() (uint16, error) {
	data, err := os.ReadFile(unprivilegedPortStartPath)
	if err != nil {
		return 0, err
	}
	start, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 16)
	if err != nil {
		return 0, err
	}
	return uint16(start), nil //nolint:gosec // ParseUint with bitSize 16 refuses a value above 65535
}

// ListenError turns a listener the kernel refused to open into the operator
// error every route reports: the BGP start, a reload that adds a peer or a
// dynamic group, an address that appears, and the doctor's bind probe. cause is
// the error net.Listen or net.ListenPacket returned, kept as the evidence, so
// errors.Is still answers syscall.EACCES or syscall.EADDRNOTAVAIL through it.
//
// The why and the fix follow the kernel's answer:
//
//   - a permission refusal on a port below the kernel's unprivileged port
//     start names CAP_NET_BIND_SERVICE, and the fix is
//     privilege.CapabilityRemedy, which names the systemd repair only when Ze
//     established that it runs under the ze service
//   - a permission refusal on a port at or above that start is not about the
//     capability: a security policy refused it
//   - a permission refusal when the start cannot be read says the port may
//     need CAP_NET_BIND_SERVICE, rather than asserting it
//   - an address another socket holds, and an address no interface holds,
//     keep their own cause and fix
//
// Safe for concurrent use.
func ListenError(site ListenSite, cause error) *opererr.Error {
	problem := listenProblem(site)
	if errors.Is(cause, os.ErrPermission) {
		return permissionRefusal(site, problem, cause)
	}
	if errors.Is(cause, syscall.EADDRINUSE) {
		return opererr.Wrap(cause, problem,
			"another socket on this host already listens on this address and port: stop the program that holds it, or configure another port",
			portHolderCommand(site))
	}
	if errors.Is(cause, syscall.EADDRNOTAVAIL) {
		var why textbuf.Buffer
		why.Str("no interface on this host holds the address ").Addr(site.Endpoint.Addr()).
			Str(": add the address to an interface, or configure an address this host holds")
		return opererr.Wrap(cause, problem, why.String(), addressListCommand)
	}
	return opererr.Wrap(cause, problem, "the kernel refused to open the listening socket", unknownCauseCommand)
}

// IsPortPermissionRefusal reports whether cause is the kernel refusing a bind
// for lack of permission (EACCES or EPERM): the case whose answer depends on
// the capabilities of the process that bound, so a caller that probed from
// another process than the daemon says which process it measured.
func IsPortPermissionRefusal(cause error) bool {
	return errors.Is(cause, os.ErrPermission)
}

// listenProblem is the first line of every listen refusal: the address and
// port, and what configured them.
func listenProblem(site ListenSite) string {
	var problem textbuf.Buffer
	problem.Str("cannot listen on ").Str(site.Endpoint.String())
	if site.Owner != "" {
		problem.Str(" for ").Str(site.Owner)
	}
	return problem.String()
}

// permissionRefusal explains an EACCES or EPERM from bind. Linux refuses a
// port below net.ipv4.ip_unprivileged_port_start to a process without
// CAP_NET_BIND_SERVICE (inet_port_requires_bind_service, called from the bind
// of net/ipv4/af_inet.c); any other permission refusal of a bind comes from a
// security policy, an LSM or a cgroup BPF program.
func permissionRefusal(site ListenSite, problem string, cause error) *opererr.Error {
	port := site.Endpoint.Port()
	var why textbuf.Buffer
	start, readErr := unprivilegedPortStart()
	if readErr != nil {
		why.Str("a port below the kernel's unprivileged port start needs ").Str(capNetBindService).
			Str(", and ze cannot read that start from ").Str(unprivilegedPortStartPath).
			Str(", so port ").Uint16(port).Str(" may need it")
		return opererr.Wrap(cause, problem, why.String(), privilege.CapabilityRemedy(capNetBindService))
	}
	if port < start {
		why.Str("port ").Uint16(port).Str(" is below the kernel's unprivileged port start ").Uint16(start).
			Str(" (net.ipv4.ip_unprivileged_port_start), so only a process that holds ").
			Str(capNetBindService).Str(" can listen on it")
		return opererr.Wrap(cause, problem, why.String(), privilege.CapabilityRemedy(capNetBindService))
	}
	why.Str("port ").Uint16(port).Str(" is not below the kernel's unprivileged port start ").Uint16(start).
		Str(", so ").Str(capNetBindService).
		Str(" is not the cause: a security policy (SELinux, AppArmor or a cgroup BPF program) refused the bind")
	return opererr.Wrap(cause, problem, why.String(), policyDenialCommand)
}

// portHolderCommand lists the socket that holds the site's port, with the
// program that owns it.
func portHolderCommand(site ListenSite) string {
	var command textbuf.Buffer
	command.Str("sudo ss -")
	if site.Network == "udp" {
		command.Str("lunp")
	} else {
		command.Str("ltnp")
	}
	command.Str(" 'sport = :").Uint16(site.Endpoint.Port()).Byte('\'')
	return command.String()
}
