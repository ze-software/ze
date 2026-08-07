// Design: docs/architecture/config/syntax.md -- listener conflict detection at config parse time
// Overview: environment.go -- environment configuration loading

package config

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
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

// ListenerService describes one ze:listener service the YANG schema declares: its
// name, the config-tree path that configures it, and the shape of its list. It is
// the canonical description, so a caller that needs the set of listeners a config
// CAN carry reads it from DiscoverListenerServices rather than keeping a second
// copy (ai/rules/evidence.md).
//
// The fields are exported because callers outside this package need them. The
// doctor dependency inventory (internal/component/doctor/doctor_test.go) walks
// every service, and builds the tree that configures each one from Containers and
// ListName rather than from a hand-written path table.
type ListenerService struct {
	Name string // Human-readable name, also the ListenerEndpoint.Service prefix
	// Protocols are the transports the service binds on each endpoint, and there
	// can be more than one: a DNS service binds UDP and TCP on the same port
	// (dnsserver.Manager.bind, internal/core/dnsserver/manager.go). The SCHEMA
	// cannot say which -- the zt:listener grouping carries ip and port and no
	// transport -- so the service registers them with RegisterListenerProtocols
	// and the shape of the list is only the fallback.
	Protocols []string
	// Containers is the path from the tree root to the service container, which
	// is the parent of the ze:listener list.
	Containers []string
	ListName   string // Name of the ze:listener list itself (e.g. "server", "wireguard")
	// ServerList is true when the list uses the zt:listener grouping (ip and port
	// children) and false for a flat list carrying listen-port.
	ServerList bool
	// HasEnabledLeaf is true when the schema parent container declares an
	// "enabled" child, which gates collection.
	HasEnabledLeaf bool
}

// DiscoverListenerServices walks the schema tree and returns all services
// marked with ze:listener, sorted by name. The returned slice replaces the former
// hardcoded knownListenerServices list. Each entry carries enough information for
// CollectListeners to navigate the config tree and extract endpoints, and enough
// for a caller outside this package to build that config itself.
//
// Naming: path components between the root and the ze:listener list are joined
// with "-", dropping "environment" and "interface" prefixes (common top-level
// groupings) and "server" suffixes (conventional list names). This produces
// names like "web", "ssh", "plugin-hub", "api-server-rest", "wireguard".
//
// Protocol: lists whose schema children include both "ip" and "port" (from the
// zt:listener grouping) are TCP. Lists with a "listen-port" child are UDP.
func DiscoverListenerServices(schema *Schema) []ListenerService {
	var services []ListenerService
	walkListenerNodes(schema.root, nil, &services)
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	return services
}

// walkListenerNodes recursively walks the schema tree, collecting ze:listener lists.
// parentNode is the schema node that contains the children being iterated.
func walkListenerNodes(parentNode Node, path []string, services *[]ListenerService) {
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

// buildListenerService constructs a ListenerService from a ze:listener list
// node, its full schema path, and the schema parent node (used to check for
// an "enabled" leaf that gates collection).
func buildListenerService(ln *ListNode, fullPath []string, parentNode Node) ListenerService {
	listName := fullPath[len(fullPath)-1]
	containers := fullPath[:len(fullPath)-1]

	// Fallback transport, from the shape of the list alone. A zt:listener list
	// (ip + port) says NOTHING about the transport, so TCP here is a guess and
	// only a registered protocol makes it a fact. A flat listen-port list is
	// wireguard, which is UDP by construction.
	fallback := ProtocolTCP
	serverList := ln.Has("ip") && ln.Has("port")
	if !serverList && ln.Has("listen-port") {
		fallback = ProtocolUDP
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
	name := textbuf.Join(nameParts, "-")
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

	return ListenerService{
		Name:           name,
		Protocols:      listenerProtocolsFor(name, fallback),
		Containers:     containers,
		ListName:       listName,
		ServerList:     serverList,
		HasEnabledLeaf: hasEnabled,
	}
}

var listenerProtocolsMu sync.RWMutex

// listenerProtocols maps a ze:listener service to the transports it binds.
//
// Seeded with the BUILTIN services whose list shape gives the wrong answer, and
// seeded here rather than from RegisterBuiltinListenerDefaults because that runs
// lazily for the doctor while CollectListeners also runs from config validation
// (cmd_validate.go, graph.go, bgp/config/loader_create.go). A transport that is
// wrong there misses a real port conflict. Plugins add their own from their
// register.go.
var listenerProtocols = map[string][]string{
	// The L2TP control plane is UDP: the endpoints come from
	// internal/component/l2tp/config.go and (*UDPListener).Start
	// (internal/component/l2tp/listener.go) binds them with ListenPacket and
	// asserts *net.UDPConn. Probed as TCP, the check succeeded whatever held
	// UDP/1701, so it could not detect the conflict it exists to detect.
	"l2tp": {ProtocolUDP},
}

// RegisterListenerProtocols records the transports a named ze:listener service
// binds on each of its endpoints. Register every one: a DNS service binds UDP and
// TCP, and probing only one of them passes a config whose other half cannot bind.
//
// The schema cannot answer this. zt:listener carries ip and port and no
// transport, so without a registration buildListenerService assumes TCP, and
// `ze doctor` probed TCP/1701 for the L2TP control plane, which binds UDP
// ((*UDPListener).Start, internal/component/l2tp/listener.go, calls ListenPacket
// and asserts *net.UDPConn). That probe succeeds whatever holds UDP/1701, so it
// could not fail and the coverage it claimed did not exist.
//
// Registration must happen before the first DiscoverListenerServices call. The
// builtins meet that by being the initialiser of listenerProtocols above, which
// runs at package initialisation; a plugin meets it from its own register.go,
// which runs before main. Neither can use RegisterBuiltinListenerDefaults: its
// only caller is the doctor's registerListenerDefaultsOnce, so it never runs for
// `ze config validate`, and CollectListeners does.
//
// A registration naming a transport ze cannot probe is refused WHOLE, and the
// service falls back to the list shape. That is the same class of bad
// registration as an unusable ListenerDefault, and it fails in the opposite and
// worse direction: an unknown network string reaches net.Listen through
// probeListener (internal/component/doctor/checks_listener.go), which errors, and
// `ze doctor` then reports doctor-listen-unavailable against a listener that is
// perfectly healthy. Silence hides a problem; this invents one.
//
// Whole, and not the valid subset, for two reasons. The call is ONE claim -- "this
// service binds exactly these" -- so keeping part of it asserts a narrower claim
// than anyone wrote, and for a dual-stack service that is the half-probe this
// registry exists to remove. And a subset can be worse than no registration at
// all: on a flat listen-port list the shape fallback is UDP, so keeping TCP out
// of ("junk", tcp) would probe the wrong transport where refusing the lot
// probes the right one.
func RegisterListenerProtocols(serviceName string, protocols ...string) {
	if bad, found := unprobeableProtocol(protocols); found {
		listenerDefaultLogger().Error("listener protocol is not one ze can probe; whole registration refused, service falls back to its list shape",
			"service", serviceName, "protocol", bad, "known", []string{ProtocolTCP, ProtocolUDP})
		// Drop any previous registration too: the caller has just tried to
		// restate this service's transports, and leaving the old set would keep
		// probing a claim they were replacing.
		forgetListenerProtocols(serviceName)
		return
	}
	if len(protocols) == 0 {
		listenerDefaultLogger().Error("listener protocols named none; service falls back to its list shape", "service", serviceName)
		forgetListenerProtocols(serviceName)
		return
	}
	listenerProtocolsMu.Lock()
	listenerProtocols[serviceName] = append([]string(nil), protocols...)
	listenerProtocolsMu.Unlock()
}

// unprobeableProtocol returns the first transport ze has no probe for, if any.
// tcp and udp are the whole set: probeListener picks ListenPacket for udp and
// net.Listen for everything else, so a third name is a runtime error dressed as
// a diagnostic.
func unprobeableProtocol(protocols []string) (string, bool) {
	for _, p := range protocols {
		if p != ProtocolTCP && p != ProtocolUDP {
			return p, true
		}
	}
	return "", false
}

func forgetListenerProtocols(serviceName string) {
	listenerProtocolsMu.Lock()
	delete(listenerProtocols, serviceName)
	listenerProtocolsMu.Unlock()
}

// listenerProtocolsFor returns the registered transports for a service, or the
// shape-derived fallback when it registered none.
func listenerProtocolsFor(serviceName, fallback string) []string {
	listenerProtocolsMu.RLock()
	defer listenerProtocolsMu.RUnlock()
	if p, ok := listenerProtocols[serviceName]; ok && len(p) > 0 {
		return append([]string(nil), p...)
	}
	return []string{fallback}
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
		for _, name := range svc.Containers {
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
		if svc.HasEnabledLeaf {
			v, ok := container.Get("enabled")
			if !ok || v != configTrue {
				continue
			}
		}

		// One endpoint per transport: a service that binds UDP and TCP on the
		// same port has two, and a probe of one says nothing about the other.
		if svc.ServerList {
			// Standard shape: list server { ip ...; port ...; }
			for _, entry := range container.GetListOrdered(svc.ListName) {
				for _, protocol := range svc.Protocols {
					ep := parseListenerEntry(svc.Name, protocol, entry.Key, entry.Value)
					if ep != nil {
						endpoints = append(endpoints, *ep)
					}
				}
			}
		} else {
			// Flat shape: list entries with a listen-port leaf (e.g. wireguard).
			// IP is 0.0.0.0 because the kernel binds on all addresses.
			for _, entry := range container.GetListOrdered(svc.ListName) {
				portStr, ok := entry.Value.Get("listen-port")
				if !ok || portStr == "" {
					continue
				}
				port, err := strconv.ParseUint(portStr, 10, 16)
				if err != nil || port == 0 {
					continue
				}
				for _, protocol := range svc.Protocols {
					endpoints = append(endpoints, ListenerEndpoint{
						Service:  listenerEndpointName(svc.Name, entry.Key),
						Protocol: protocol,
						IP:       net.IPv4zero,
						Port:     uint16(port), //nolint:gosec // ParseUint bitSize=16 bounds value
					})
				}
			}
		}
	}

	return endpoints
}

// listenerEndpointName is what every endpoint of a keyed list is called: the
// service name, then the list key. One helper so the three producers cannot
// drift apart, which would make the same endpoint look like two to the callers
// that key on Service.
func listenerEndpointName(service, key string) string {
	if key == "" {
		return service
	}
	var tb textbuf.Buffer
	return tb.Str(service).Byte(' ').Str(key).String()
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

	return &ListenerEndpoint{Service: listenerEndpointName(service, key), Protocol: protocol, IP: ip, Port: port}
}

// ListenerConflict describes a pair of conflicting listener endpoints.
type ListenerConflict struct {
	A   ListenerEndpoint
	B   ListenerEndpoint
	Err error
}

// ValidateListenerConflicts checks a slice of endpoints for overlapping
// protocol:ip:port bindings. Wildcard addresses (0.0.0.0 for IPv4, :: for IPv6)
// conflict with any address in the same family. Cross-family (0.0.0.0 vs ::1)
// does NOT conflict. Cross-protocol (TCP:N vs UDP:N) never conflicts either.
// Returns an error naming both conflicting services if a conflict is found.
func ValidateListenerConflicts(endpoints []ListenerEndpoint) error {
	if c := FindListenerConflict(endpoints); c != nil {
		return c.Err
	}
	return nil
}

// FindListenerConflict returns the first conflicting endpoint pair, or nil.
func FindListenerConflict(endpoints []ListenerEndpoint) *ListenerConflict {
	for i := range endpoints {
		for j := i + 1; j < len(endpoints); j++ {
			if conflicts(endpoints[i], endpoints[j]) {
				return &ListenerConflict{
					A: endpoints[i],
					B: endpoints[j],
					Err: fmt.Errorf("listener conflict: %s (%s %s:%d) and %s (%s %s:%d) bind to the same endpoint",
						endpoints[i].Service, ProtocolLabel(endpoints[i].Protocol), endpoints[i].IP, endpoints[i].Port,
						endpoints[j].Service, ProtocolLabel(endpoints[j].Protocol), endpoints[j].IP, endpoints[j].Port),
				}
			}
		}
	}
	return nil
}

// ProtocolLabel returns the protocol for display. Endpoints built by tests
// without an explicit Protocol field are shown as "tcp" since every
// pre-Phase-5 service in ze was TCP.
func ProtocolLabel(p string) string {
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
	if ProtocolLabel(a.Protocol) != ProtocolLabel(b.Protocol) {
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

// ListenerDefault holds the endpoint a ze:listener service falls back to when
// the config does not spell it out. The YANG refine that declares the same
// values never reaches the schema (the Ze YANG compiler drops refine
// statements), so the values are registered in Go instead.
//
// The registration must mirror the SERVICE's own config extraction, never the
// YANG refine. mcp refines port 8080 and its extraction passes an empty default
// port (extractMCPBlock, loader_extract.go), so ExtractMCPConfig starts no
// listener at all unless the operator names a port: registering 8080 for it made
// doctor warn about a listener that does not exist.
type ListenerDefault struct {
	// IPs are the addresses the service binds by default. Most services bind
	// one; geodns binds a dual-stack pair (127.0.0.1 and ::1) on the same port.
	IPs  []string
	Port string
	// WhenListEmpty reports whether the service listens on IPs:Port when its
	// ze:listener list is EMPTY. It is false for a service that starts NO
	// listener then, where the values describe only the per-ENTRY fallback:
	// registering such a service as WhenListEmpty would make doctor probe a
	// port the daemon never binds.
	WhenListEmpty bool
}

var listenerDefaultsMu sync.RWMutex
var listenerDefaults = map[string]ListenerDefault{}

// RegisterListenerDefault registers the endpoint a named ze:listener service
// listens on when its server list is EMPTY. The same port is the per-entry
// fallback: an entry that names an ip and omits the port binds it.
//
// Register a service here only when its own config extraction synthesizes an
// endpoint from an empty list (ExtractAPIConfig and its siblings in
// loader_extract.go do). A service that starts nothing on an empty list belongs
// in RegisterListenerEntryDefault, and one that starts nothing either way belongs
// in neither.
func RegisterListenerDefault(serviceName, ip, port string) {
	RegisterListenerDefaultIPs(serviceName, []string{ip}, port)
}

// RegisterListenerDefaultIPs is RegisterListenerDefault for a service whose empty
// list yields SEVERAL endpoints on one port. parseListeners
// (internal/plugins/geodns/config.go) returns 127.0.0.1 and ::1 together, and
// probing only the first would pass a config whose other half cannot bind.
func RegisterListenerDefaultIPs(serviceName string, ips []string, port string) {
	storeListenerDefault(serviceName, ListenerDefault{IPs: ips, Port: port, WhenListEmpty: true})
}

// RegisterListenerEntryDefault registers the ip and port an ENTRY of a named
// ze:listener list falls back to when it omits them, for a service that starts
// NO listener when the list is empty.
//
// l2tp is the shape: ParseParameters (internal/component/l2tp/config.go) appends
// one listen address per server entry and applies DefaultListenPort to an entry
// that omits the port, and appends nothing at all for an empty list.
func RegisterListenerEntryDefault(serviceName, ip, port string) {
	storeListenerDefault(serviceName, ListenerDefault{IPs: []string{ip}, Port: port})
}

// listenerDefaultLogger reports a registration that cannot produce an endpoint.
var listenerDefaultLogger = slogutil.LazyLogger("config.listener")

// storeListenerDefault records a default only when it can produce an endpoint,
// and says so when it cannot.
//
// A registration that yields nothing is the failure this whole inventory exists
// to remove: doctor probes nothing for the service, the covered row goes on
// claiming it does, and nothing says so. Dropping it here rather than storing it
// makes that consequence VISIBLE instead of partial -- a stored default with no
// usable IP still filled the per-entry path, so the service looked probed from
// one direction and was not from the other, and TestDoctorProbesEveryCoveredListener
// now fails for it (ai/rules/evidence.md: fail closed or say something).
//
// Every caller is an init() or a registration function with literal arguments, so
// this is a compile-desk error, never something an operator can trigger.
func storeListenerDefault(serviceName string, def ListenerDefault) {
	if reason := unusableListenerDefault(def); reason != "" {
		listenerDefaultLogger().Error("listener default is unusable; not registered",
			"service", serviceName, "reason", reason, "port", def.Port, "addresses", def.IPs)
		// CLEAR rather than leave. A refused RE-registration that returned early
		// would leave the previous default in place, so the service would keep
		// being probed at an endpoint its owner has just tried to change, and the
		// log line would describe a state the map does not hold.
		listenerDefaultsMu.Lock()
		delete(listenerDefaults, serviceName)
		listenerDefaultsMu.Unlock()
		return
	}
	listenerDefaultsMu.Lock()
	listenerDefaults[serviceName] = def
	listenerDefaultsMu.Unlock()
}

// unusableListenerDefault returns why a default cannot produce an endpoint, or
// "" when it can.
func unusableListenerDefault(def ListenerDefault) string {
	if len(def.IPs) == 0 {
		return "no IP address"
	}
	for _, ip := range def.IPs {
		if net.ParseIP(ip) == nil {
			return "unparseable IP address"
		}
	}
	if port, err := strconv.ParseUint(def.Port, 10, 16); err != nil || port == 0 {
		return "unusable port"
	}
	return ""
}

// CollectListenersWithDefaults extends CollectListeners with the registered
// defaults, which the raw Tree does not carry. It fills two shapes
// CollectListeners drops:
//
//   - an EMPTY server list, for a service that listens on its default endpoint
//     then (ListenerDefault.WhenListEmpty).
//   - an ENTRY that omits the port, for any service with a registered default.
//     parseListenerEntry returns nil there because the port is 0, while the
//     daemon binds the default port, so this is the endpoint doctor must probe.
func CollectListenersWithDefaults(tree *Tree, schema *Schema) []ListenerEndpoint {
	endpoints := CollectListeners(tree, schema)

	services := DiscoverListenerServices(schema)
	listenerDefaultsMu.RLock()
	defer listenerDefaultsMu.RUnlock()

	for _, svc := range services {
		container := tree
		for _, name := range svc.Containers {
			container = container.GetContainer(name)
			if container == nil {
				break
			}
		}
		if container == nil {
			continue
		}

		if svc.HasEnabledLeaf {
			v, ok := container.Get("enabled")
			if !ok || v != configTrue {
				continue
			}
		}

		def, ok := listenerDefaults[svc.Name]
		if !ok {
			continue
		}

		entries := container.GetListOrdered(svc.ListName)
		if len(entries) == 0 {
			if !def.WhenListEmpty {
				continue
			}
			for _, ip := range def.IPs {
				for _, protocol := range svc.Protocols {
					if ep := defaultListenerEndpoint(svc.Name, protocol, "", ip, def.Port); ep != nil {
						endpoints = append(endpoints, *ep)
					}
				}
			}
			continue
		}

		// A flat list (wireguard) has no ip/port pair to complete: an entry
		// without listen-port gets an ephemeral port from the kernel, which is
		// not an endpoint anything can probe.
		if !svc.ServerList {
			continue
		}
		for _, entry := range entries {
			// CollectListeners already produced the endpoint for an entry that
			// names its port, and deliberately drops one whose ip is malformed.
			if portStr, _ := entry.Value.Get("port"); portStr != "" {
				continue
			}
			ipStr, _ := entry.Value.Get("ip")
			if ipStr != "" && net.ParseIP(ipStr) == nil {
				continue
			}
			// The entry names one address, so its own ip wins over the whole
			// default set; a dual-stack default only applies to the empty list.
			entryIP := ipStr
			if entryIP == "" && len(def.IPs) > 0 {
				entryIP = def.IPs[0]
			}
			for _, protocol := range svc.Protocols {
				if ep := defaultListenerEndpoint(svc.Name, protocol, entry.Key, entryIP, def.Port); ep != nil {
					endpoints = append(endpoints, *ep)
				}
			}
		}
	}

	return endpoints
}

// defaultListenerEndpoint builds one endpoint from a registered default.
//
// Both refusals below are unreachable through the registered defaults, because
// storeListenerDefault already refuses an unusable address or port, and the
// per-entry path parses the entry's own ip before calling. They stay as the
// fail-closed shape: a probe of 0.0.0.0 or of port 0 is a wrong answer, and no
// answer is the safer one (ai/rules/evidence.md).
func defaultListenerEndpoint(service, protocol, key, defIP, defPort string) *ListenerEndpoint {
	ip := net.ParseIP(defIP)
	if ip == nil {
		return nil
	}
	port, err := strconv.ParseUint(defPort, 10, 16)
	if err != nil || port == 0 {
		return nil
	}
	return &ListenerEndpoint{
		Service:  listenerEndpointName(service, key),
		Protocol: protocol,
		IP:       ip,
		Port:     uint16(port), //nolint:gosec // ParseUint bitSize=16 bounds value
	}
}
