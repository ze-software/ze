// Design: docs/architecture/config/syntax.md -- listener conflict detection at config parse time
// Overview: environment.go -- environment configuration loading

package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
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

// listenerService defines a config tree path to a service with ze:listener entries.
// Discovered dynamically from the YANG schema by DiscoverListenerServices.
type listenerService struct {
	name           string   // Human-readable name for error messages
	protocol       string   // ProtocolTCP or ProtocolUDP
	containers     []string // Path from tree root to the service container (parent of the ze:listener list)
	listName       string   // Name of the ze:listener list itself (e.g. "server", "wireguard")
	serverList     bool     // True if the list uses zt:listener (ip+port children); false for flat listen-port
	hasEnabledLeaf bool     // True if the schema parent container has an "enabled" child
}

// DiscoverListenerServices walks the schema tree and returns all services
// marked with ze:listener. The returned slice replaces the former hardcoded
// knownListenerServices list. Each entry carries enough information for
// CollectListeners to navigate the config tree and extract endpoints.
//
// Naming: path components between the root and the ze:listener list are joined
// with "-", dropping "environment" and "interface" prefixes (common top-level
// groupings) and "server" suffixes (conventional list names). This produces
// names like "web", "ssh", "plugin-hub", "api-server-rest", "wireguard".
//
// Protocol: lists whose schema children include both "ip" and "port" (from the
// zt:listener grouping) are TCP. Lists with a "listen-port" child are UDP.
func DiscoverListenerServices(schema *Schema) []listenerService {
	var services []listenerService
	walkListenerNodes(schema.root, nil, &services)
	return services
}

// walkListenerNodes recursively walks the schema tree, collecting ze:listener lists.
// parentNode is the schema node that contains the children being iterated.
func walkListenerNodes(parentNode Node, path []string, services *[]listenerService) {
	cp, ok := parentNode.(childProvider)
	if !ok {
		return
	}
	for _, name := range cp.Children() {
		child := cp.Get(name)
		childPath := append(append([]string(nil), path...), name)

		if ln, ok := child.(*ListNode); ok && ln.Listener {
			svc := buildListenerService(ln, childPath, parentNode)
			*services = append(*services, svc)
		}

		// Recurse into containers and lists.
		walkListenerNodes(child, childPath, services)
	}
}

// buildListenerService constructs a listenerService from a ze:listener list
// node, its full schema path, and the schema parent node (used to check for
// an "enabled" leaf that gates collection).
func buildListenerService(ln *ListNode, fullPath []string, parentNode Node) listenerService {
	listName := fullPath[len(fullPath)-1]
	containers := fullPath[:len(fullPath)-1]

	// Determine protocol from list children.
	protocol := ProtocolTCP
	serverList := ln.Has("ip") && ln.Has("port")
	if !serverList && ln.Has("listen-port") {
		protocol = ProtocolUDP
	}

	// Derive human-readable name: drop well-known top-level grouping
	// containers (environment, telemetry, interface) and the list name
	// if it is the conventional "server". Other top-level containers
	// like "plugin" are kept because they carry meaning (e.g. "plugin-hub").
	var nameParts []string
	for i, p := range containers {
		if i == 0 && (p == "environment" || p == "interface" || p == "telemetry") {
			continue
		}
		nameParts = append(nameParts, p)
	}
	if listName != "server" {
		nameParts = append(nameParts, listName)
	}
	name := strings.Join(nameParts, "-")
	if name == "" {
		name = listName
	}

	// Check whether the schema parent container defines an "enabled" leaf.
	// Services with an enabled leaf require explicit enabled=true in config
	// (YANG default is false). Services without one are always collected.
	hasEnabled := false
	if pcp, ok := parentNode.(childProvider); ok {
		if child := pcp.Get("enabled"); child != nil {
			if _, isLeaf := child.(*LeafNode); isLeaf {
				hasEnabled = true
			}
		}
	}

	return listenerService{
		name:           name,
		protocol:       protocol,
		containers:     containers,
		listName:       listName,
		serverList:     serverList,
		hasEnabledLeaf: hasEnabled,
	}
}

// CollectListeners walks the config tree and collects all listener endpoints
// from services marked with ze:listener in the YANG schema. Services with
// enabled=false are skipped.
//
// Note: YANG refine defaults (ip/port) are not present in the raw Tree.
// Conflict detection only covers endpoints with explicitly configured ip+port.
// Services relying solely on YANG defaults with empty server entries are not checked.
func CollectListeners(tree *Tree, schema *Schema) []ListenerEndpoint {
	services := DiscoverListenerServices(schema)
	var endpoints []ListenerEndpoint

	for _, svc := range services {
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
		// Services without an enabled leaf in the schema (e.g. plugin-hub)
		// are always collected.
		if svc.hasEnabledLeaf {
			v, ok := container.Get("enabled")
			if !ok || v != configTrue {
				continue
			}
		}

		if svc.serverList {
			// Standard shape: list server { ip ...; port ...; }
			for _, entry := range container.GetListOrdered(svc.listName) {
				ep := parseListenerEntry(svc.name, svc.protocol, entry.Key, entry.Value)
				if ep != nil {
					endpoints = append(endpoints, *ep)
				}
			}
		} else {
			// Flat shape: list entries with a listen-port leaf (e.g. wireguard).
			// IP is 0.0.0.0 because the kernel binds on all addresses.
			for _, entry := range container.GetListOrdered(svc.listName) {
				portStr, ok := entry.Value.Get("listen-port")
				if !ok || portStr == "" {
					continue
				}
				port, err := strconv.ParseUint(portStr, 10, 16)
				if err != nil || port == 0 {
					continue
				}
				endpoints = append(endpoints, ListenerEndpoint{
					Service:  svc.name + " " + entry.Key,
					Protocol: svc.protocol,
					IP:       net.IPv4zero,
					Port:     uint16(port), //nolint:gosec // ParseUint bitSize=16 bounds value
				})
			}
		}
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
