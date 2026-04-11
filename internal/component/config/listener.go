// Design: docs/architecture/config/syntax.md -- listener conflict detection at config parse time
// Overview: environment.go -- environment configuration loading

package config

import (
	"fmt"
	"net"
	"strconv"
)

// Protocol names for listener endpoints. Endpoints on different protocols
// (TCP vs UDP) never clash at the kernel level, so conflict detection
// compares Protocol first and only reports a clash when both endpoints
// share the same protocol.
const (
	ProtocolTCP = "tcp"
	ProtocolUDP = "udp"
)

// ListenerEndpoint represents a network listener for port conflict detection.
type ListenerEndpoint struct {
	Service  string // Human-readable service name (e.g., "web", "looking-glass", "bgp peer 10.0.0.1")
	Protocol string // ProtocolTCP or ProtocolUDP
	IP       net.IP // Parsed IP address; nil or unspecified means wildcard
	Port     uint16 // Listening port number
}

// listenerService defines a config tree path to a service with ze:listener server list entries.
type listenerService struct {
	name          string   // Human-readable name for error messages
	protocol      string   // ProtocolTCP or ProtocolUDP
	containers    []string // Path from tree root to the service container
	alwaysEnabled bool     // True if service has no enabled leaf (always collect)
}

// knownListenerServices enumerates all services that use ze:listener server lists.
// Derived from YANG modules: ze-web-conf, ze-ssh-conf, ze-mcp-conf, ze-lg-conf,
// ze-telemetry-conf, ze-plugin-conf, ze-api-conf. Every service here is TCP;
// UDP services (wireguard) are collected by dedicated helpers because their
// config shape does not fit the `server` sub-list pattern.
var knownListenerServices = []listenerService{
	{name: "web", protocol: ProtocolTCP, containers: []string{"environment", "web"}},
	{name: "ssh", protocol: ProtocolTCP, containers: []string{"environment", "ssh"}},
	{name: "mcp", protocol: ProtocolTCP, containers: []string{"environment", "mcp"}},
	{name: "looking-glass", protocol: ProtocolTCP, containers: []string{"environment", "looking-glass"}},
	{name: "prometheus", protocol: ProtocolTCP, containers: []string{"telemetry", "prometheus"}},
	{name: "plugin-hub", protocol: ProtocolTCP, containers: []string{"plugin", "hub"}, alwaysEnabled: true},
	{name: "api-server-rest", protocol: ProtocolTCP, containers: []string{"environment", "api-server", "rest"}},
	{name: "api-server-grpc", protocol: ProtocolTCP, containers: []string{"environment", "api-server", "grpc"}},
}

// CollectListeners walks the config tree and collects all listener endpoints
// from services with ze:listener server lists. Services with enabled=false are skipped.
//
// Note: YANG refine defaults (ip/port) are not present in the raw Tree.
// Conflict detection only covers endpoints with explicitly configured ip+port.
// Services relying solely on YANG defaults with empty server entries are not checked.
func CollectListeners(tree *Tree) []ListenerEndpoint {
	var endpoints []ListenerEndpoint

	for _, svc := range knownListenerServices {
		container := tree
		for _, name := range svc.containers {
			container = container.GetContainer(name)
			if container == nil {
				break
			}
		}
		if container == nil {
			continue
		}

		// Check enabled leaf -- YANG default is false, so absent = disabled.
		// plugin-hub has no enabled leaf (alwaysEnabled).
		if !svc.alwaysEnabled {
			v, ok := container.Get("enabled")
			if !ok || v != configTrue {
				continue
			}
		}

		// Walk server list entries.
		for _, entry := range container.GetListOrdered("server") {
			ep := parseListenerEntry(svc.name, svc.protocol, entry.Key, entry.Value)
			if ep != nil {
				endpoints = append(endpoints, *ep)
			}
		}
	}

	endpoints = append(endpoints, collectWireguardListeners(tree)...)

	return endpoints
}

// collectWireguardListeners walks interface.wireguard list entries and
// emits a UDP listener endpoint for each entry that has a listen-port set.
// Wireguard uses a flat `leaf listen-port` directly on the list entry
// rather than a nested `server` sub-list, so it does not fit the
// knownListenerServices walker and needs its own collector. The IP is
// always 0.0.0.0 because the kernel binds on both families unconditionally;
// a cross-family clash detector would need a separate "binds all families"
// signal which is out of scope here.
func collectWireguardListeners(tree *Tree) []ListenerEndpoint {
	ifaceC := tree.GetContainer("interface")
	if ifaceC == nil {
		return nil
	}
	wgList := ifaceC.GetList("wireguard")
	if len(wgList) == 0 {
		return nil
	}
	var endpoints []ListenerEndpoint
	for name, entry := range wgList {
		portStr, ok := entry.Get("listen-port")
		if !ok || portStr == "" {
			continue
		}
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil || port == 0 {
			continue
		}
		endpoints = append(endpoints, ListenerEndpoint{
			Service:  "wireguard " + name,
			Protocol: ProtocolUDP,
			IP:       net.IPv4zero,
			Port:     uint16(port), //nolint:gosec // ParseUint bitSize=16 bounds value
		})
	}
	return endpoints
}

// parseListenerEntry extracts a ListenerEndpoint from a server list entry tree.
func parseListenerEntry(service, protocol, key string, entry *Tree) *ListenerEndpoint {
	ipStr, _ := entry.Get("ip")
	portStr, _ := entry.Get("port")

	if ipStr == "" && portStr == "" {
		return nil
	}

	ip := net.ParseIP(ipStr)
	if ip == nil && ipStr != "" {
		return nil // Malformed IP, skip entry.
	}
	if ip == nil {
		ip = net.IPv4zero // Missing IP = wildcard (binds all interfaces).
	}
	var port uint16
	if portStr != "" {
		if v, err := strconv.ParseUint(portStr, 10, 16); err == nil {
			port = uint16(v) //nolint:gosec // Validated by ParseUint range
		}
	}

	if port == 0 {
		return nil
	}

	name := service
	if key != "" {
		name = service + " " + key
	}

	return &ListenerEndpoint{Service: name, Protocol: protocol, IP: ip, Port: port}
}

// ValidateListenerConflicts checks a slice of endpoints for overlapping
// protocol:ip:port bindings. Wildcard addresses (0.0.0.0 for IPv4, :: for IPv6)
// conflict with any address in the same family. Cross-family (0.0.0.0 vs ::1)
// does NOT conflict. Cross-protocol (TCP:N vs UDP:N) never conflicts either.
// Returns an error naming both conflicting services if a conflict is found.
func ValidateListenerConflicts(endpoints []ListenerEndpoint) error {
	for i := range endpoints {
		for j := i + 1; j < len(endpoints); j++ {
			if conflicts(endpoints[i], endpoints[j]) {
				return fmt.Errorf("listener conflict: %s (%s %s:%d) and %s (%s %s:%d) bind to the same endpoint",
					endpoints[i].Service, protocolLabel(endpoints[i].Protocol), endpoints[i].IP, endpoints[i].Port,
					endpoints[j].Service, protocolLabel(endpoints[j].Protocol), endpoints[j].IP, endpoints[j].Port)
			}
		}
	}
	return nil
}

// protocolLabel returns the protocol for display. Endpoints built by tests
// without an explicit Protocol field are shown as "tcp" since every
// pre-Phase-5 service in ze was TCP.
func protocolLabel(p string) string {
	if p == "" {
		return ProtocolTCP
	}
	return p
}

// conflicts returns true if two endpoints bind to overlapping
// protocol:ip:port bindings. Endpoints on different protocols never clash
// at the kernel level even if they share ip:port. An empty Protocol field
// is treated as TCP.
func conflicts(a, b ListenerEndpoint) bool {
	if protocolLabel(a.Protocol) != protocolLabel(b.Protocol) {
		return false
	}
	if a.Port != b.Port {
		return false
	}
	return ipsConflict(a.IP, b.IP)
}

// ipsConflict returns true if two IPs would conflict when binding on the same port.
// Wildcard (0.0.0.0 or ::) conflicts with any address in the same family.
// Cross-family never conflicts.
func ipsConflict(a, b net.IP) bool {
	// Normalize to 16-byte form for consistent comparison.
	a = a.To16()
	b = b.To16()
	if a == nil || b == nil {
		return false
	}

	aV4 := a.To4() != nil
	bV4 := b.To4() != nil

	// Cross-family: IPv4 and IPv6 never conflict.
	if aV4 != bV4 {
		return false
	}

	// Same address: always conflicts.
	if a.Equal(b) {
		return true
	}

	// Wildcard check within the same family.
	if aV4 {
		return a.Equal(net.IPv4zero) || b.Equal(net.IPv4zero)
	}
	return a.Equal(net.IPv6zero) || b.Equal(net.IPv6zero)
}
