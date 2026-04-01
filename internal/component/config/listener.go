// Design: docs/architecture/config/syntax.md -- listener conflict detection at config parse time
// Overview: environment.go -- environment configuration loading

package config

import (
	"fmt"
	"net"
	"strconv"
)

// ListenerEndpoint represents a network listener for port conflict detection.
type ListenerEndpoint struct {
	Service string // Human-readable service name (e.g., "web", "looking-glass", "bgp peer 10.0.0.1")
	IP      net.IP // Parsed IP address; nil or unspecified means wildcard
	Port    uint16 // Listening port number
}

// listenerService defines a config tree path to a service with ze:listener server list entries.
type listenerService struct {
	name          string   // Human-readable name for error messages
	containers    []string // Path from tree root to the service container
	alwaysEnabled bool     // True if service has no enabled leaf (always collect)
}

// knownListenerServices enumerates all services that use ze:listener server lists.
// Derived from YANG modules: ze-web-conf, ze-ssh-conf, ze-mcp-conf, ze-lg-conf,
// ze-telemetry-conf, ze-plugin-conf.
var knownListenerServices = []listenerService{
	{name: "web", containers: []string{"environment", "web"}},
	{name: "ssh", containers: []string{"environment", "ssh"}},
	{name: "mcp", containers: []string{"environment", "mcp"}},
	{name: "looking-glass", containers: []string{"environment", "looking-glass"}},
	{name: "prometheus", containers: []string{"telemetry", "prometheus"}},
	{name: "plugin-hub", containers: []string{"plugin", "hub"}, alwaysEnabled: true},
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
			ep := parseListenerEntry(svc.name, entry.Key, entry.Value)
			if ep != nil {
				endpoints = append(endpoints, *ep)
			}
		}
	}

	return endpoints
}

// parseListenerEntry extracts a ListenerEndpoint from a server list entry tree.
func parseListenerEntry(service, key string, entry *Tree) *ListenerEndpoint {
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

	return &ListenerEndpoint{Service: name, IP: ip, Port: port}
}

// ValidateListenerConflicts checks a slice of endpoints for overlapping ip:port bindings.
// Wildcard addresses (0.0.0.0 for IPv4, :: for IPv6) conflict with any address in the same family.
// Cross-family (0.0.0.0 vs ::1) does NOT conflict.
// Returns an error naming both conflicting services if a conflict is found.
func ValidateListenerConflicts(endpoints []ListenerEndpoint) error {
	for i := range endpoints {
		for j := i + 1; j < len(endpoints); j++ {
			if conflicts(endpoints[i], endpoints[j]) {
				return fmt.Errorf("listener conflict: %s (%s:%d) and %s (%s:%d) bind to the same endpoint",
					endpoints[i].Service, endpoints[i].IP, endpoints[i].Port,
					endpoints[j].Service, endpoints[j].IP, endpoints[j].Port)
			}
		}
	}
	return nil
}

// conflicts returns true if two endpoints bind to overlapping ip:port pairs.
func conflicts(a, b ListenerEndpoint) bool {
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
