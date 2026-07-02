// Design: plan/learned/1033-as112-2-dns-server.md -- as112 listen-port bind-capability doctor check (finding L1)
//
// Unlike geodns (default unprivileged port 5300, operator-configurable),
// as112's port is always the fixed, privileged DNS port 53 (RFC 7534
// Section 3.5) -- so this check probes unconditionally whenever as112 is
// enabled, with no port-value branch.

package as112

import (
	"context"
	"net"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// checkAS112ListenCapability reads the as112 enabled and address-family
// leaves from the tree and delegates the decision to as112ListenDiagnostic.
// (Cross-service port conflicts are detected separately by the ze:listener
// extension at parse time; this check covers bind capability for port 53.)
func checkAS112ListenCapability(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	svc := tree.GetContainer(configRootService)
	if svc == nil {
		return nil
	}
	a := svc.GetContainer("as112")
	if a == nil {
		return nil
	}
	enabled, _ := a.Get("enabled")
	family, ok := a.Get("address-family")
	if !ok || family == "" {
		family = addressFamilyBoth
	}
	return as112ListenDiagnostic(enabled == "true", family, probeBindable)
}

// as112ListenDiagnostic decides whether to warn: only an enabled as112 with
// an unbindable port 53 on a family it actually serves produces a
// diagnostic. Probing only the IPv4 wildcard would give false confidence to
// an ipv6-only node (its real bind never touches IPv4) and would never warn
// an ipv6-only node about a real IPv6 CAP_NET_BIND_SERVICE gap. bindable is
// a seam for testing.
func as112ListenDiagnostic(enabled bool, family string, bindable func(host string, port int) bool) []rpc.DoctorCheckDiagnostic {
	if !enabled {
		return nil
	}
	for _, host := range wildcardHostsForFamily(family) {
		if !bindable(host, as112Port) {
			return []rpc.DoctorCheckDiagnostic{{
				Code:     "doctor-as112-port-unavailable",
				Severity: "warning",
				Message:  "cannot bind AS112's fixed UDP/TCP port 53; needs CAP_NET_BIND_SERVICE or root, or the port is already in use",
			}}
		}
	}
	return nil
}

// wildcardHostsForFamily returns the wildcard probe address(es) for family,
// matching the families serverEndpoints actually binds (register.go).
func wildcardHostsForFamily(family string) []string {
	switch family {
	case addressFamilyIPv4Only:
		return []string{"0.0.0.0"}
	case addressFamilyIPv6Only:
		return []string{"::"}
	default:
		return []string{"0.0.0.0", "::"}
	}
}

// probeBindable reports whether host:port can be bound for both UDP and TCP,
// binding and releasing immediately.
func probeBindable(host string, port int) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	var lc net.ListenConfig
	ctx := context.Background()
	pc, err := lc.ListenPacket(ctx, "udp", addr)
	if err != nil {
		return false
	}
	if cerr := pc.Close(); cerr != nil {
		loggerPtr.Load().Debug("as112: close udp port probe", "error", cerr)
	}
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	if cerr := ln.Close(); cerr != nil {
		loggerPtr.Load().Debug("as112: close tcp port probe", "error", cerr)
	}
	return true
}
