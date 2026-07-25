// Design: plan/learned/994-geodns-3-observability-cli.md -- geodns listen-port bind-capability doctor check
// Related: register.go -- DoctorChecks registration (geodns-listen-capability)
//
// geodns defaults to the unprivileged port 5300, but an operator may move it to
// the standard DNS port 53 (privileged, <1024). This check warns, before the
// engine starts, when geodns is configured on a privileged port the process
// cannot bind -- typically a missing CAP_NET_BIND_SERVICE / root. It probes each
// configured listen address/port (not the wildcard) and releases immediately, so
// it matches what the server binds, and it is silent for the unprivileged default.
package geodns

import (
	"context"
	"net"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/dnsserver"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const privilegedPortCeiling = 1024

// probeTarget is one (host, port) endpoint the capability check probes.
type probeTarget struct {
	host string
	port int
}

// checkGeoDNSListenCapability reads the geodns listener list from the tree and
// delegates the decision to geodnsListenDiagnostic. (Cross-service port
// conflicts are detected separately by the ze:listener extension at parse time;
// this check covers bind capability for privileged ports.)
func checkGeoDNSListenCapability(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	svc := tree.GetContainer("service")
	if svc == nil {
		return nil
	}
	g := svc.GetContainer("geodns")
	if g == nil {
		return nil
	}
	enabled, _ := g.Get("enabled")

	var targets []probeTarget
	for _, lt := range g.GetList("listener") {
		host, _ := lt.Get("ip")
		if host == "" {
			host = "127.0.0.1"
		}
		port := defaultListenPort
		if portStr, ok := lt.Get("port"); ok {
			if p, err := strconv.Atoi(portStr); err == nil {
				port = p
			}
		}
		targets = append(targets, probeTarget{host: host, port: port})
	}
	if len(targets) == 0 {
		targets = []probeTarget{{host: "127.0.0.1", port: defaultListenPort}, {host: "::1", port: defaultListenPort}}
	}
	return geodnsListenDiagnostic(enabled == configValueTrue, targets, probeBindable)
}

// geodnsListenDiagnostic decides whether to warn: only an enabled geodns with a
// privileged-port endpoint that is not bindable produces a diagnostic. bindable
// is a seam for testing.
func geodnsListenDiagnostic(enabled bool, targets []probeTarget, bindable func(host string, port int) bool) []rpc.DoctorCheckDiagnostic {
	if !enabled {
		return nil
	}
	for _, t := range targets {
		if t.port < 1 || t.port >= privilegedPortCeiling {
			continue
		}
		if !bindable(t.host, t.port) {
			return []rpc.DoctorCheckDiagnostic{{
				Code:     "doctor-geodns-port-unavailable",
				Severity: "warning",
				Message:  "cannot bind a configured privileged geodns listener (UDP/TCP); needs CAP_NET_BIND_SERVICE or root, or the address/port is already in use",
			}}
		}
	}
	return nil
}

// checkGeoDNSTLSCert validates the DoT/DoH certificate material whenever a
// secure listener is enabled, reusing the shared dnsserver cert check.
func checkGeoDNSTLSCert(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	svc := tree.GetContainer("service")
	if svc == nil {
		return nil
	}
	g := svc.GetContainer("geodns")
	if g == nil {
		return nil
	}
	tlsC := g.GetContainer("tls")
	dohC := g.GetContainer("doh")
	var certFile, keyFile string
	if tlsC != nil {
		certFile, _ = tlsC.Get("cert-file")
		keyFile, _ = tlsC.Get("key-file")
	}
	return geodnsTLSDiagnostic(containerTrue(tlsC, "enabled"), containerTrue(dohC, "enabled"),
		resolveCertPath(certFile, ctx.ConfigDir), resolveCertPath(keyFile, ctx.ConfigDir), time.Now())
}

// geodnsTLSDiagnostic is the pure decision function: no cert validation unless a
// secure listener is enabled; otherwise delegate to the shared cert check.
func geodnsTLSDiagnostic(dotEnabled, dohEnabled bool, certFile, keyFile string, now time.Time) []rpc.DoctorCheckDiagnostic {
	if !dotEnabled && !dohEnabled {
		return nil
	}
	problems := dnsserver.CheckCertMaterial(certFile, keyFile, now)
	out := make([]rpc.DoctorCheckDiagnostic, 0, len(problems))
	for _, p := range problems {
		out = append(out, rpc.DoctorCheckDiagnostic{Code: p.Code, Severity: p.Severity, Message: p.Message})
	}
	return out
}

// containerTrue reports whether container c has leaf key set to "true".
func containerTrue(c *config.Tree, key string) bool {
	if c == nil {
		return false
	}
	v, _ := c.Get(key)
	return v == configValueTrue
}

// resolveCertPath makes a config-relative cert path absolute against the config
// directory.
func resolveCertPath(p, dir string) string {
	if p == "" || dir == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
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
		loggerPtr.Load().Debug("geodns: close udp port probe", "error", cerr)
	}
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	if cerr := ln.Close(); cerr != nil {
		loggerPtr.Load().Debug("geodns: close tcp port probe", "error", cerr)
	}
	return true
}
