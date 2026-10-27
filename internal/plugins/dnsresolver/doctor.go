// Design: docs/architecture/dns/lan-resolver.md -- the readiness checks ze doctor runs
// Related: config.go -- parseConfig, the one parse the checks share with the verifier

package dnsresolver

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// doctorProbeTimeout bounds one upstream probe. DoctorProbeTimeout can only
// shorten it.
const doctorProbeTimeout = 3 * time.Second

// listenerBind is the bind probe the listener check runs. It is a variable
// so a test can stand in a refused bind; nothing else assigns it.
var listenerBind = probeBindable

// doctorChecks are the readiness checks the resolver registers: one for
// each runtime dependency, its listeners and its upstreams.
func doctorChecks() []registry.DoctorCheckDef {
	return []registry.DoctorCheckDef{
		{
			Name:         "dns-resolver-listen-capability",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        727,
			Dependencies: []string{"fib-kernel"},
			Platforms:    []string{"any"},
			Codes:        []string{diagnostic.CodeDoctorDNSResolverPortUnavailable},
			Check:        checkResolverListeners,
		},
		{
			Name:         "dns-resolver-upstream-reachable",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        728,
			Dependencies: []string{"fib-kernel"},
			Platforms:    []string{"any"},
			Codes:        []string{diagnostic.CodeDoctorDNSResolverUpstreamUnreachable},
			Check:        checkResolverUpstreams,
		},
	}
}

// doctorConfig parses the resolver config out of the doctor's tree through
// the same delivery the plugin sees (Tree.ToPluginMap, JSON, parseConfig),
// and answers ok only for an enabled resolver. A config that does not parse
// answers ok false too: config validation, which ze doctor runs first,
// reports that config, and a readiness probe of an invalid config would only
// repeat it.
func doctorConfig(ctx registry.DoctorCheckContext) (resolverConfig, bool) {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok {
		return resolverConfig{}, false
	}
	if tree == nil {
		return resolverConfig{}, false
	}
	data, err := json.Marshal(tree.ToPluginMap())
	if err != nil {
		return resolverConfig{}, false
	}
	cfg, err := parseConfig(string(data))
	if err != nil {
		return resolverConfig{}, false
	}
	return cfg, cfg.Enabled
}

// checkResolverListeners warns for each listener of an enabled resolver
// that cannot bind its address and port for both UDP and TCP.
func checkResolverListeners(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	// doctorConfig answers ok false only for a disabled resolver, or a config
	// validation already reports: neither has a listener to probe.
	if cfg, ok := doctorConfig(ctx); ok {
		return listenerDiagnostics(cfg.Listeners, listenerBind)
	}
	return nil
}

// listenerDiagnostics is the decision: one warning for each listener whose
// bind is refused for lack of permission. A busy port or an address that is
// not on this host is reported by ze doctor's listener probe, which probes
// every ze:listener endpoint (doctor-listen-unavailable,
// internal/component/doctor checkListeners), so this check names only the
// cause that probe cannot: port 53 needs CAP_NET_BIND_SERVICE. bind is a
// seam for testing.
func listenerDiagnostics(listeners []listenerEndpoint, bind func(netip.AddrPort) error) []rpc.DoctorCheckDiagnostic {
	var out []rpc.DoctorCheckDiagnostic
	for _, l := range listeners {
		endpoint := netip.AddrPortFrom(l.IP, l.Port)
		err := bind(endpoint)
		if err == nil {
			continue
		}
		if !errors.Is(err, os.ErrPermission) {
			continue
		}
		var msg textbuf.Buffer
		msg.WriteString("dns-resolver listener ")
		msg.WriteString(l.Name)
		msg.WriteString(" cannot bind ")
		msg.WriteString(endpoint.String())
		msg.WriteString(" (")
		msg.WriteString(err.Error())
		msg.WriteString("): a port below 1024 needs CAP_NET_BIND_SERVICE or root; " +
			"run ze with that capability, or set `service dns-resolver listener ")
		msg.WriteString(l.Name)
		msg.WriteString(" port` to 1024 or above")
		out = append(out, rpc.DoctorCheckDiagnostic{
			Code:     diagnostic.CodeDoctorDNSResolverPortUnavailable,
			Severity: "warning",
			Message:  msg.String(),
		})
	}
	return out
}

// probeBindable binds endpoint for UDP and TCP and releases both at once.
func probeBindable(endpoint netip.AddrPort) error {
	addr := endpoint.String()
	var lc net.ListenConfig
	ctx := context.Background()
	pc, err := lc.ListenPacket(ctx, "udp", addr)
	if err != nil {
		return err
	}
	if cerr := pc.Close(); cerr != nil {
		loggerPtr.Load().Debug("dns-resolver: close udp port probe", "error", cerr)
	}
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	if cerr := ln.Close(); cerr != nil {
		loggerPtr.Load().Debug("dns-resolver: close tcp port probe", "error", cerr)
	}
	return nil
}

// checkResolverUpstreams warns when an enabled resolver has no forward
// server that answers a probe query.
func checkResolverUpstreams(ctx registry.DoctorCheckContext) []rpc.DoctorCheckDiagnostic {
	// doctorConfig answers ok false only for a disabled resolver, or a config
	// validation already reports: neither has an upstream to probe.
	if cfg, ok := doctorConfig(ctx); ok {
		return upstreamDiagnostics(cfg.Forward, diagnostic.DoctorProbeTimeout(doctorProbeTimeout))
	}
	return nil
}

// upstreamDiagnostics probes each upstream in order and warns only when none
// answers: one answering upstream is enough to resolve, and failover reaches
// it. Any reply counts, whatever its rcode, because it proves the server is
// there and speaks DNS.
func upstreamDiagnostics(forward []upstream, timeout time.Duration) []rpc.DoctorCheckDiagnostic {
	var tried textbuf.Buffer
	for i, u := range forward {
		err := probeUpstream(u.addrPort(), timeout)
		if err == nil {
			return nil
		}
		if i > 0 {
			tried.WriteString(", ")
		}
		tried.WriteString(u.addrPort().String())
		tried.WriteString(": ")
		tried.WriteString(err.Error())
	}
	var msg textbuf.Buffer
	msg.WriteString("no dns-resolver forward server answered a probe query within ")
	msg.WriteString(timeout.String())
	msg.WriteString(" (tried ")
	msg.WriteString(tried.String())
	msg.WriteString("), so every client query would get SERVFAIL; check `service dns-resolver forward server` " +
		"and the route and firewall towards each upstream")
	return []rpc.DoctorCheckDiagnostic{{
		Code:     diagnostic.CodeDoctorDNSResolverUpstreamUnreachable,
		Severity: "warning",
		Message:  msg.String(),
	}}
}

// probeUpstream sends the query the resolver itself would send for the
// root NS RRset, over UDP through the resolver's own exchange, and answers
// the error when no matching reply came back within timeout.
func probeUpstream(up netip.AddrPort, timeout time.Duration) error {
	query := upstreamQuery(dns.Question{Name: ".", Qtype: dns.TypeNS, Qclass: dns.ClassINET}, dns.Id())
	_, err := exchange("udp", up, query, time.Now().Add(timeout))
	return err
}
