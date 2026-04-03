// Design: docs/architecture/chaos-web-dashboard.md -- listener conflict detection for ze-chaos

package main

import (
	"net"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// validateChaosListenerConflicts checks for overlapping ip:port bindings among
// ze-chaos single-port listeners. Range bases (--port, --listen-base) are excluded
// because they allocate N ports per peer count.
//
// Flags with value 0 (int ports) or "" (addr:port) are disabled and excluded.
func validateChaosListenerConflicts(sshPort, webUIPort, lgPort int, webAddr, pprofAddr, metricsAddr, zePprofAddr string) error {
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

type parsedEndpoint struct {
	ip   net.IP
	port uint16
}

// parseAddrPort parses "addr:port" or ":port" into IP and port.
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
