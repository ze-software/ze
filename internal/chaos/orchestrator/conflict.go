// Design: docs/architecture/chaos-web-dashboard.md -- listener conflict detection for ze-chaos

package orchestrator

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
)

// validateChaosListenerConflicts checks for overlapping ip:port bindings among
// ze-chaos single-port listeners. Range bases (--port, --listen-base) are excluded
// because they allocate N ports per peer count.
//
// Flags with value 0 (int ports) or "" (addr:port) are disabled and excluded.
func validateChaosListenerConflicts(sshPort, webUIPort, lgPort, zeMCPPort int, webAddr, pprofAddr, metricsAddr, zePprofAddr, mcpAddr string) error {
	var endpoints []config.ListenerEndpoint

	// Integer port flags bind on 127.0.0.1 (ze-chaos default local-addr).
	localhost := net.IPv4(127, 0, 0, 1)
	for _, ep := range []struct {
		name string
		port int
	}{
		{"ssh", sshPort},
		{"web-ui", webUIPort},
		{"looking-glass", lgPort},
		{"ze-mcp", zeMCPPort},
	} {
		if ep.port == 0 {
			continue
		}
		endpoints = append(endpoints, config.ListenerEndpoint{
			Service: ep.name,
			IP:      localhost,
			Port:    uint16(ep.port), //nolint:gosec // port validated 0-65535 by flag parsing
		})
	}

	// String addr:port flags.
	for _, ep := range []struct {
		name string
		addr string
	}{
		{"chaos-web", webAddr},
		{"chaos-pprof", pprofAddr},
		{"chaos-metrics", metricsAddr},
		{"ze-pprof", zePprofAddr},
		{"chaos-mcp", mcpAddr},
	} {
		if ep.addr == "" {
			continue
		}
		parsed := parseAddrPort(ep.addr)
		if parsed == nil {
			continue
		}
		endpoints = append(endpoints, config.ListenerEndpoint{
			Service: ep.name,
			IP:      parsed.ip,
			Port:    parsed.port,
		})
	}

	return config.ValidateListenerConflicts(endpoints)
}

// ValidateRangeConflicts checks whether any single-port listener falls inside
// the port ranges allocated by --port or --listen-base. Each range is
// [base, base + peers*2) since each peer gets 2 ports (one for ze, one for the tool).
func ValidateRangeConflicts(bgpBase, listenBase, peers, sshPort, webUIPort, lgPort, zeMCPPort int, webAddr, pprofAddr, metricsAddr, zePprofAddr, mcpAddr string) error {
	bgpEnd := bgpBase + peers*2
	listenEnd := listenBase + peers*2

	type entry struct {
		name string
		port int
	}

	var singles []entry

	for _, ep := range []struct {
		name string
		port int
	}{
		{"ssh", sshPort},
		{"web-ui", webUIPort},
		{"looking-glass", lgPort},
		{"ze-mcp", zeMCPPort},
	} {
		if ep.port != 0 {
			singles = append(singles, entry{ep.name, ep.port})
		}
	}

	for _, ep := range []struct {
		name string
		addr string
	}{
		{"chaos-web", webAddr},
		{"chaos-pprof", pprofAddr},
		{"chaos-metrics", metricsAddr},
		{"ze-pprof", zePprofAddr},
		{"chaos-mcp", mcpAddr},
	} {
		if ep.addr == "" {
			continue
		}
		parsed := parseAddrPort(ep.addr)
		if parsed == nil {
			continue
		}
		singles = append(singles, entry{ep.name, int(parsed.port)})
	}

	for _, s := range singles {
		if s.port >= bgpBase && s.port < bgpEnd {
			return fmt.Errorf("%s port %d falls inside bgp port range %d-%d (--port %d, %d peers)",
				s.name, s.port, bgpBase, bgpEnd-1, bgpBase, peers)
		}
		if s.port >= listenBase && s.port < listenEnd {
			return fmt.Errorf("%s port %d falls inside listen-base range %d-%d (--listen-base %d, %d peers)",
				s.name, s.port, listenBase, listenEnd-1, listenBase, peers)
		}
	}

	return nil
}

// ValidateConfigRangeConflicts is the struct-path guard equivalent of the
// flag-path ValidateRangeConflicts wiring in cli.go. It re-derives the BGP and
// listen port ranges from the assembled OrchestratorConfig's peer profiles and
// checks that the config's single-port service listeners (web, metrics, mcp)
// do not fall inside those ranges. RunOrchestrator calls it at entry so that
// any programmatic caller that builds an OrchestratorConfig directly -- not
// only the flag path -- is protected from the same port clash (AC-10).
func ValidateConfigRangeConflicts(cfg *orchestratorConfig) error {
	if len(cfg.Profiles) == 0 {
		return nil
	}
	// Peer ports are assigned as BasePort+index (ze) and ListenBase+index
	// (tool), so the minimum over profiles recovers the two range bases the
	// flag path passes as *port and *listen-base.
	bgpBase := cfg.Profiles[0].ZePort
	listenBase := cfg.Profiles[0].Port
	for _, p := range cfg.Profiles {
		if p.ZePort < bgpBase {
			bgpBase = p.ZePort
		}
		if p.Port < listenBase {
			listenBase = p.Port
		}
	}
	peers := len(cfg.Profiles)

	// OrchestratorConfig carries only the web/metrics/mcp single-port
	// listeners; the ssh/looking-glass/pprof flags are consumed before the
	// struct is built and are not part of it, so pass them empty here.
	return ValidateRangeConflicts(bgpBase, listenBase, peers,
		0, 0, 0, 0,
		cfg.WebAddr, "", cfg.MetricsAddr, "", cfg.McpAddr)
}

type parsedEndpoint struct {
	ip   net.IP
	port uint16
}

func parseAddrPort(s string) *parsedEndpoint {
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		// Try bare port number (e.g., "6060").
		if !strings.Contains(s, ":") {
			p, err := strconv.ParseUint(s, 10, 16)
			if err != nil || p == 0 {
				return nil
			}
			return &parsedEndpoint{ip: net.IPv4zero, port: uint16(p)} //nolint:gosec // validated by ParseUint range
		}
		return nil
	}

	var ip net.IP
	if host == "" {
		ip = net.IPv4zero
	} else {
		ip = net.ParseIP(host)
		if ip == nil {
			return nil
		}
	}

	p, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil || p == 0 {
		return nil
	}

	return &parsedEndpoint{ip: ip, port: uint16(p)} //nolint:gosec // validated by ParseUint range
}
