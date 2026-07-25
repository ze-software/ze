// Design: plan/learned/920-mpls-ldp.md -- LDP port-646 readiness doctor check
// Related: register.go -- DoctorChecks registration (ldp-port)
// Related: wire.go -- ldpHelloPort / ldpSessionPort (646) this check probes
//
// LDP discovery and sessions bind UDP and TCP port 646 (RFC 5036 Section 2.5),
// a privileged port (<1024). This doctor check warns, before the engine starts,
// when ldp is configured but port 646 cannot be bound -- typically a missing
// CAP_NET_BIND_SERVICE / root, or the port already held by another process.
package ldp

import (
	"context"
	"net"
	"strconv"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ldpPortProbe is a test seam over probeLDPPort.
var ldpPortProbe = probeLDPPort

// checkLDPPort warns when ldp is configured but port 646 cannot be bound. ldp
// owns the ldp config block, so it owns this readiness check
// (ai/rules/doctor-checks.md "Where to Register Checks").
func checkLDPPort(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if tree.GetContainer("ldp") == nil {
		return nil
	}
	if ldpPortProbe() {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     "doctor-ldp-port-unavailable",
		Severity: "warning",
		Message:  "cannot bind LDP port 646 (UDP/TCP); needs CAP_NET_BIND_SERVICE or root, or the port is already in use",
	}}
}

// probeLDPPort reports whether the LDP port (646) can be bound for both UDP and
// TCP. It binds and immediately releases each; a privileged-port or in-use
// failure makes it return false.
func probeLDPPort() bool {
	return probePortBindable(net.JoinHostPort("", strconv.Itoa(ldpHelloPort)))
}

// probePortBindable reports whether addr can be bound for both UDP and TCP.
func probePortBindable(addr string) bool {
	var lc net.ListenConfig
	ctx := context.Background()
	pc, err := lc.ListenPacket(ctx, "udp", addr)
	if err != nil {
		return false
	}
	if cerr := pc.Close(); cerr != nil {
		logger().Warn("ldp: close udp port probe", "err", cerr)
	}
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	if cerr := ln.Close(); cerr != nil {
		logger().Warn("ldp: close tcp port probe", "err", cerr)
	}
	return true
}
