// Design: docs/architecture/diagnostics/active-probes.md -- the ICMP probe dependency check
// Related: socket.go -- OpenICMP, whose two socket constructions this check tries
// Related: register.go -- registers this check via diagnostic.RegisterDoctorCheck
//
// Every active probe needs an ICMP socket, and a host can offer one of two:
// the raw socket under CAP_NET_RAW, or Linux's unprivileged datagram socket
// when the process group is inside net.ipv4.ping_group_range. This check
// tries the two in the order OpenICMP does and reports the outcome before
// an operator's first ping, so a missing dependency is a diagnostic with a
// code rather than a socket error at the moment of use. The probe layer
// owns the socket, so it owns the check, its codes and their tests.

package probe

import (
	"context"
	"errors"
	"net/netip"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorCheckName is the registry name of the check.
const doctorCheckName = "icmp-probe-socket"

// The two codes the check emits. Both resolve through diagnostic.Lookup,
// which internal/core/diagnostic/codes.go guarantees.
const (
	// codeICMPProbeUnavailable: neither socket kind opens, so no probe can run.
	codeICMPProbeUnavailable = "doctor-icmp-probe"
	// codeICMPProbeUnprivileged: the raw socket is refused for privilege and
	// the datagram socket stands in, which ping can use and traceroute cannot.
	codeICMPProbeUnprivileged = "doctor-icmp-probe-unprivileged"
)

// checkICMPProbeSocket is the check function. It tries the IPv4 sockets
// only: CAP_NET_RAW governs the raw socket of both families, and
// ping_group_range governs the datagram socket of both (net/ipv4/ping.c
// ping_init_sock serves AF_INET6 too), so one family answers for both.
func checkICMPProbeSocket(_ diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	sock, err := OpenICMP(context.Background(), FamilyIPv4, netip.Addr{}, DFOff)
	if err != nil {
		return []diagnostic.Diagnostic{{
			Code:     codeICMPProbeUnavailable,
			Severity: diagnostic.SeverityWarning,
			Message:  unavailableMessage(err),
		}}
	}
	kind := sock.Kind()
	if closeErr := sock.Close(); closeErr != nil {
		return []diagnostic.Diagnostic{{
			Code:     codeICMPProbeUnavailable,
			Severity: diagnostic.SeverityWarning,
			Message:  "the ICMP probe socket opened but did not close: " + closeErr.Error(),
		}}
	}
	if kind == SocketRaw {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     codeICMPProbeUnprivileged,
		Severity: diagnostic.SeverityWarning,
		Message:  unprivilegedMessage(),
	}}
}

// unavailableMessage names the dependency and both ways to satisfy it.
// err already carries the refusal of each socket kind that was tried, so
// the operator reads which of the two fixes applies to this host.
func unavailableMessage(err error) string {
	var tb textbuf.Buffer
	tb.Str("no ICMP probe socket can be opened, so ping and traceroute cannot run: ")
	tb.Str(err.Error())
	if errors.Is(err, ErrDatagramICMPUnsupported) {
		return tb.Str(". Grant CAP_NET_RAW to the daemon").String()
	}
	tb.Str(". Grant CAP_NET_RAW to the daemon, or include its group ")
	tb.Int(int64(processGroupID()))
	tb.Str(" in net.ipv4.ping_group_range")
	return tb.String()
}

// unprivilegedMessage says the fallback is in use and what it cannot do.
func unprivilegedMessage() string {
	var tb textbuf.Buffer
	tb.Str("the raw ICMP socket is refused (no CAP_NET_RAW), so probes use the unprivileged datagram socket permitted by net.ipv4.ping_group_range for group ")
	tb.Int(int64(processGroupID()))
	tb.Str(": ping works, traceroute needs the raw socket and refuses to run. Grant CAP_NET_RAW to the daemon")
	return tb.String()
}

// processGroupID is the effective group the daemon runs as, which is the
// group ping_group_range is checked against.
func processGroupID() int { return os.Getegid() }
