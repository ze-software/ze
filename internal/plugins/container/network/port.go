// Design: docs/architecture/container/network.md -- published ports
// Related: verify.go -- parseInstances reads each instance's port list here
// Related: tables.go -- the destination translation terms of each port

package network

import (
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/opererr"
)

// The words of the port list, as the container schema spells them.
const (
	listPort          = "port"
	leafProtocol      = "protocol"
	leafPublicAddress = "public-address"
	leafPublicPort    = "public-port"
	leafContainerPort = "container-port"
	protocolTCP       = "tcp"
	protocolUDP       = "udp"
)

// Bounds of the published ports. Each port is one destination translation
// term per public address at two hooks, so the bounds keep the contributed
// tables inside the firewall component's request bound.
const (
	portsMax           = 256 // published ports in one config
	publicAddressesMax = 16  // public addresses of one port
)

// Port is one entry of an instance's port list.
type Port struct {
	Name     string
	Protocol string
	// PublicAddresses are the router addresses the port is published on. An empty
	// list publishes it on every address of the router, in each family the
	// instance has an address of on its first network.
	PublicAddresses []netip.Addr
	PublicPorts     portRange
	ContainerPorts  portRange
}

// portRange is a first-last range of ports; one port is a range of one.
type portRange struct {
	first, last uint16
}

func (r portRange) size() int {
	return int(r.last) - int(r.first) + 1
}

func (r portRange) overlaps(other portRange) bool {
	if r.last < other.first {
		return false
	}
	return other.last >= r.first
}

// String is the firewall config form: "8080" or "8000-8010".
func (r portRange) String() string {
	if r.first == r.last {
		return strconv.Itoa(int(r.first))
	}
	return strconv.Itoa(int(r.first)) + "-" + strconv.Itoa(int(r.last))
}

// parsePorts reads an instance's port list and judges each entry alone:
// its protocol, its public addresses, and rule V-6 on its two ranges. The
// rules across entries and against the instance's network are checkPorts.
func parsePorts(instance string, fields map[string]any) ([]Port, error) {
	entries := configvalue.ListEntries(fields[listPort])
	ports := make([]Port, 0, len(entries))
	for _, entry := range entries {
		port, err := parsePort(instance, entry)
		if err != nil {
			return nil, err
		}
		ports = append(ports, port)
	}
	return ports, nil
}

func parsePort(instance string, entry configvalue.ListEntry) (Port, error) {
	port := Port{Name: entry.Key, Protocol: protocolTCP}
	if raw, present := entry.Fields[leafProtocol]; present {
		protocol, err := portProtocol(instance, entry.Key, raw)
		if err != nil {
			return Port{}, err
		}
		port.Protocol = protocol
	}
	values := configvalue.LeafList(entry.Fields[leafPublicAddress])
	if len(values) > publicAddressesMax {
		return Port{}, refusePort(instance, entry.Key,
			"it lists "+strconv.Itoa(len(values))+" public addresses, more than "+strconv.Itoa(publicAddressesMax),
			"delete container instance "+instance+" port "+entry.Key+" public-address ADDRESS for the addresses no client uses")
	}
	for _, value := range values {
		address, err := netip.ParseAddr(value)
		if err != nil {
			return Port{}, refusePort(instance, entry.Key, "public-address "+value+" is not an address",
				"set container instance "+instance+" port "+entry.Key+" public-address with an address of the router")
		}
		port.PublicAddresses = append(port.PublicAddresses, address.Unmap())
	}
	listen, err := parsePortRange(instance, entry.Key, leafPublicPort, entry.Fields)
	if err != nil {
		return Port{}, err
	}
	target, err := parsePortRange(instance, entry.Key, leafContainerPort, entry.Fields)
	if err != nil {
		return Port{}, err
	}
	if err := checkRanges(instance, entry.Key, listen, target); err != nil {
		return Port{}, err
	}
	port.PublicPorts = listen
	port.ContainerPorts = target
	return port, nil
}

func portProtocol(instance, name string, raw any) (string, error) {
	word, isWord := raw.(string)
	if !isWord {
		return "", refusePort(instance, name, "its protocol is not a word",
			"set container instance "+instance+" port "+name+" protocol tcp")
	}
	switch word {
	case protocolTCP, protocolUDP:
		return word, nil
	}
	return "", refusePort(instance, name, "protocol "+word+" is neither tcp nor udp",
		"set container instance "+instance+" port "+name+" protocol tcp")
}

// parsePortRange reads one of the two port leaves: one port, or first-last.
// A range that runs backward is the first half of rule V-6.
func parsePortRange(instance, name, leaf string, fields map[string]any) (portRange, error) {
	fix := "set container instance " + instance + " port " + name + " " + leaf + " 8080"
	raw, present := fields[leaf]
	if !present {
		return portRange{}, refusePort(instance, name, "it has no "+leaf, fix)
	}
	word, isWord := raw.(string)
	if !isWord {
		return portRange{}, refusePort(instance, name, leaf+" is not a port or a range", fix)
	}
	firstWord, lastWord, isRange := strings.Cut(word, "-")
	first, valid := portNumber(firstWord)
	if !valid {
		return portRange{}, refusePort(instance, name, leaf+" "+word+" is not a port from 1 to 65535 or a range of them", fix)
	}
	if !isRange {
		return portRange{first: first, last: first}, nil
	}
	last, valid := portNumber(lastWord)
	if !valid {
		return portRange{}, refusePort(instance, name, leaf+" "+word+" is not a port from 1 to 65535 or a range of them", fix)
	}
	if last < first {
		return portRange{}, refusePort(instance, name, leaf+" "+word+" runs backward", fix+"-8090")
	}
	return portRange{first: first, last: last}, nil
}

// portNumber reads a port from 1 to 65535.
func portNumber(word string) (uint16, bool) {
	number, err := strconv.ParseUint(word, 10, 16)
	if err != nil {
		return 0, false
	}
	if number == 0 {
		return 0, false
	}
	return uint16(number), true
}

// checkRanges is the length half of rule V-6, and the exactness of a range:
// a destination translation to a port range keeps each connection's port
// when it lies inside the range, and has no form that shifts it by an
// offset. A range is therefore published only onto the same range, and ze
// refuses one it would publish onto other ports rather than approximate it.
func checkRanges(instance, name string, listen, target portRange) error {
	if listen.size() != target.size() {
		return opererr.New(
			"container "+instance+" port "+name+": "+strconv.Itoa(listen.size())+" ports are published to "+strconv.Itoa(target.size())+" ports",
			"each published port needs one port of the container",
			"set container instance "+instance+" port "+name+" container-port "+listen.String(),
		)
	}
	if listen.size() == 1 {
		return nil
	}
	if listen == target {
		return nil
	}
	return opererr.New(
		"container "+instance+" port "+name+": the range "+listen.String()+" is published to the range "+target.String()+", which starts at another port",
		"the translation keeps the port of each connection inside a range and cannot shift it, so a range is published onto the same range",
		"set container instance "+instance+" port "+name+" container-port "+listen.String(),
	)
}

// endpoint is one place a port is published: a protocol and a port range on
// one address of the router, or on every address of one family.
type endpoint struct {
	instance string
	port     string
	protocol string
	ipv4     bool
	// everyAddress publishes on every address of the family; address is
	// then not read.
	everyAddress bool
	address      netip.Addr
	ports        portRange
}

// overlaps answers whether one connection could match both endpoints.
func (e endpoint) overlaps(other endpoint) bool {
	if e.protocol != other.protocol {
		return false
	}
	if e.ipv4 != other.ipv4 {
		return false
	}
	if !e.ports.overlaps(other.ports) {
		return false
	}
	if e.everyAddress {
		return true
	}
	if other.everyAddress {
		return true
	}
	return e.address == other.address
}

// where names the endpoint's addresses in a refusal.
func (e endpoint) where() string {
	if !e.everyAddress {
		return e.address.String()
	}
	if e.ipv4 {
		return "every IPv4 address of the router"
	}
	return "every IPv6 address of the router"
}

// checkPorts judges the published ports against the instances' networks and
// against each other: a public address needs an address of its family on the
// instance's first network, and no two entries publish one protocol and port
// on addresses that overlap (V-6). An instance with ports and no network was
// refused by parseInstances (V-18).
func checkPorts(cfg Config) error {
	count := 0
	var published []endpoint
	for _, instance := range cfg.Instances {
		if len(instance.Ports) == 0 {
			continue
		}
		network, _ := cfg.network(instance.Attachments[0].Network) // parseInstances refused an attachment to an unknown network
		for _, port := range instance.Ports {
			count++
			if count > portsMax {
				return opererr.New(
					"the config publishes more than "+strconv.Itoa(portsMax)+" container ports",
					"each published port is a translation term ze asks the firewall component for",
					"delete container instance NAME port PORT for the ports no client uses",
				)
			}
			endpoints, err := portEndpoints(network, instance.Name, port)
			if err != nil {
				return err
			}
			for _, candidate := range endpoints {
				for _, held := range published {
					if candidate.overlaps(held) {
						return refuseSharedEndpoint(held, candidate)
					}
				}
			}
			published = append(published, endpoints...)
		}
	}
	return nil
}

// portEndpoints answers where one port is published. A port with no public
// address is published on every address of each family the instance's first
// network has a prefix of.
func portEndpoints(network Network, instance string, port Port) ([]endpoint, error) {
	base := endpoint{instance: instance, port: port.Name, protocol: port.Protocol, ports: port.PublicPorts}
	var endpoints []endpoint
	if len(port.PublicAddresses) == 0 {
		for _, prefix := range network.Prefixes {
			every := base
			every.ipv4 = prefix.Addr().Is4()
			every.everyAddress = true
			endpoints = append(endpoints, every)
		}
		return endpoints, nil
	}
	for _, address := range port.PublicAddresses {
		if _, found := prefixOfFamily(network.Prefixes, address.Is4()); !found {
			return nil, opererr.New(
				"container "+instance+" port "+port.Name+": public-address "+address.String()+" has no address of its family to reach on network "+network.Name,
				"a published port is translated to the container's address on its first network, in the family of the public address",
				"set container network "+network.Name+" prefix with a prefix of that family, or delete container instance "+instance+" port "+port.Name+" public-address "+address.String(),
			)
		}
		listed := base
		listed.ipv4 = address.Is4()
		listed.address = address
		endpoints = append(endpoints, listed)
	}
	return endpoints, nil
}

func refusePort(instance, name, defect, fix string) error {
	return opererr.New(
		"container "+instance+" port "+name+": "+defect,
		"a published port names a protocol, the router's port and the container's port",
		fix,
	)
}

// refusePortWithoutNetwork is rule V-18.
func refusePortWithoutNetwork(instance, port string) error {
	return opererr.New(
		"container "+instance+" publishes a port and has no network",
		"a published port needs an address to reach",
		"set container instance "+instance+" network NET, or delete container instance "+instance+" port "+port,
	)
}

// refuseSharedEndpoint is the overlap half of rule V-6.
func refuseSharedEndpoint(held, candidate endpoint) error {
	return opererr.New(
		"container "+candidate.instance+" port "+candidate.port+" and container "+held.instance+" port "+held.port+
			" both publish "+candidate.protocol+" port "+candidate.ports.String()+" on "+candidate.where(),
		"one connection reaches one container",
		"set container instance "+candidate.instance+" port "+candidate.port+" public-port with a port no other container publishes",
	)
}
