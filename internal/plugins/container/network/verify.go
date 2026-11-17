// Design: docs/architecture/container/network.md -- the config the package reads and the rules that refuse it
// Related: address.go -- the address arithmetic the rules use
// Related: backend.go -- the backend rule V-19
// Related: register.go -- registers parseSections as the network sibling's parse

package network

import (
	"encoding/json"
	"net"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/core/configorder"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The config words this package reads, as the container schema and the
// interface and firewall schemas spell them.
const (
	containerRoot  = "container"
	interfaceRoot  = "interface"
	firewallRoot   = "firewall"
	listNetwork    = "network"
	listInstance   = "instance"
	leafName       = "name"
	leafPrefix     = "prefix"
	leafGateway    = "gateway"
	leafMTU        = "mtu"
	leafMasquerade = "masquerade"
	leafAddress    = "address"
	leafHardware   = "mac-address"
	leafBackend    = "backend"
)

// Bounds the schema states and the interface component enforces. Each is
// checked again here, the second check of the pair.
const (
	mtuDefault        = 1500
	mtuMin            = 68
	mtuMax            = 16000
	mtuIPv6Min        = 1280
	ipv4LengthMax     = 30
	ipv6LengthMax     = 126
	masqueradeOn      = true
	networksMax       = 64   // the interface component's bridges in one owner's set
	attachmentsMax    = 1024 // the interface component's links one owner holds
	interfaceDepthMax = 16   // the depth of the interface config the address walk reads
)

// Config is the network package's part of the container config. Its zero
// value holds no network: nothing is attached and nothing is translated. It
// holds no func and no channel, so two configs compare with reflect.DeepEqual.
type Config struct {
	// Networks are sorted by name.
	Networks []Network
	// Instances are the instances with at least one attachment, sorted by
	// name.
	Instances []Instance
}

// Network is one entry of the container network list.
type Network struct {
	Name   string
	Bridge string
	// Prefixes are masked, one per family, the IPv4 one first.
	Prefixes []netip.Prefix
	// Gateways hold one address per prefix, in the same order: the set
	// gateway, or the network address plus one.
	Gateways   []netip.Addr
	MTU        uint32
	Masquerade bool
}

// Instance is a container's list of attachments, in the order the operator
// wrote them: the first is eth0 inside the container.
type Instance struct {
	Name        string
	Attachments []Attachment
}

// Attachment is one entry of an instance's network list.
type Attachment struct {
	Network string
	// Addresses are the addresses the operator set, at most one per family.
	Addresses []netip.Addr
	// HardwareAddress is the set address, else the derived one, in the
	// kernel's lower-case colon form.
	HardwareAddress string
}

// network answers the network named name.
func (c *Config) network(name string) (Network, bool) {
	for _, network := range c.Networks {
		if network.Name == name {
			return network, true
		}
	}
	return Network{}, false
}

// delivery is what one parse reads from the delivered sections.
type delivery struct {
	container     map[string]any
	containerData string
	interfaces    map[string]any
	firewall      map[string]any
	hasInterface  bool
	hasFirewall   bool
}

// parseSections reads and judges the network part of the delivered config:
// the container root, and the interface and firewall roots the package reads.
// It is the network sibling's parse, so it runs at `ze config validate`, at a
// reload's verify and at the boot configure, and changes no state. Every
// refusal is an opererr.Error.
func parseSections(sections []sdk.ConfigSection) (Config, error) {
	read, err := readDelivery(sections)
	if err != nil {
		return Config{}, err
	}
	cfg, err := parseNetworks(read.container)
	if err != nil {
		return Config{}, err
	}
	if len(cfg.Networks) == 0 {
		// No network: an attachment can only name an unknown one.
		return cfg, refuseAttachmentsWithoutNetwork(read.container)
	}
	if err := checkComponents(read); err != nil {
		return Config{}, err
	}
	if err := checkBackends(read, cfg); err != nil {
		return Config{}, err
	}
	if err := checkInterfaceConfig(cfg, read.interfaces); err != nil {
		return Config{}, err
	}
	instances, err := parseInstances(cfg, read.container)
	if err != nil {
		return Config{}, err
	}
	cfg.Instances = instances
	if err := checkAttachments(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// readDelivery unwraps the three roots. A root the delivery does not carry is
// absent; a removed root arrives as an empty object and is absent too.
func readDelivery(sections []sdk.ConfigSection) (delivery, error) {
	var read delivery
	for _, section := range sections {
		switch section.Root {
		case containerRoot:
			body, _, err := unwrapSection(section)
			if err != nil {
				return delivery{}, err
			}
			read.container = body
			read.containerData = section.Data
		case interfaceRoot:
			body, present, err := unwrapSection(section)
			if err != nil {
				return delivery{}, err
			}
			read.interfaces = body
			read.hasInterface = present
		case firewallRoot:
			body, present, err := unwrapSection(section)
			if err != nil {
				return delivery{}, err
			}
			read.firewall = body
			read.hasFirewall = present
		}
	}
	return read, nil
}

// unwrapSection answers the body of a delivered section, and whether the
// root holds one: a removed root arrives with no data, or as an object with
// no body, and holds none.
func unwrapSection(section sdk.ConfigSection) (body map[string]any, present bool, err error) {
	if section.Data == "" {
		return nil, false, nil
	}
	var delivered map[string]any
	if err := json.Unmarshal([]byte(section.Data), &delivered); err != nil {
		return nil, false, opererr.Wrap(err,
			"the "+section.Root+" config ze delivered to the container plugin is not a JSON object",
			"the container networks are judged against that config, and ze refuses what it cannot read",
			"ze show config "+section.Root,
		)
	}
	body = configvalue.Section(section.Root, delivered)
	return body, body != nil, nil
}

// parseNetworks reads the network list and judges each network alone (the
// name, V-5, V-13, V-14) and every pair of them (V-5 overlap).
func parseNetworks(container map[string]any) (Config, error) {
	entries := configvalue.ListEntries(container[listNetwork])
	if len(entries) > networksMax {
		return Config{}, opererr.New(
			"the config holds "+strconv.Itoa(len(entries))+" container networks, more than "+strconv.Itoa(networksMax),
			"each network is a bridge ze asks the interface component for, and one owner holds at most "+strconv.Itoa(networksMax),
			"delete container network NAME for the networks no container needs",
		)
	}
	var cfg Config
	for _, entry := range entries {
		network, err := parseNetwork(entry)
		if err != nil {
			return Config{}, err
		}
		cfg.Networks = append(cfg.Networks, network)
	}
	for index, network := range cfg.Networks {
		for _, other := range cfg.Networks[index+1:] {
			if err := refuseOverlap(network, other); err != nil {
				return Config{}, err
			}
		}
	}
	return cfg, nil
}

// parseNetwork reads one network entry.
func parseNetwork(entry configvalue.ListEntry) (Network, error) {
	bridge, err := bridgeName(entry.Key)
	if err != nil {
		return Network{}, err
	}
	network := Network{Name: entry.Key, Bridge: bridge, MTU: mtuDefault, Masquerade: masqueradeOn}
	prefixes, err := parsePrefixes(entry)
	if err != nil {
		return Network{}, err
	}
	network.Prefixes = prefixes
	gateways, err := parseGateways(entry, prefixes)
	if err != nil {
		return Network{}, err
	}
	network.Gateways = gateways
	mtu, err := parseMTU(entry, prefixes)
	if err != nil {
		return Network{}, err
	}
	network.MTU = mtu
	if raw, present := entry.Fields[leafMasquerade]; present {
		value, read := configvalue.Bool(raw)
		if !read {
			return Network{}, opererr.New(
				"network "+entry.Key+": masquerade is neither true nor false",
				"the leaf states whether outbound traffic is translated, so ze refuses a value it cannot read rather than pick one",
				"set container network "+entry.Key+" masquerade true",
			)
		}
		network.Masquerade = value
	}
	return network, nil
}

// parsePrefixes reads a network's prefixes: at least one, masked, one per
// family (V-5), each of a length the network can use (V-13). The IPv4 prefix
// comes first.
func parsePrefixes(entry configvalue.ListEntry) ([]netip.Prefix, error) {
	values := configvalue.LeafList(entry.Fields[leafPrefix])
	if len(values) == 0 {
		return nil, opererr.New(
			"network "+entry.Key+" has no prefix",
			"a network needs an address range for its gateway and its containers",
			"set container network "+entry.Key+" prefix 10.88.0.0/24",
		)
	}
	var ipv4, ipv6 netip.Prefix
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, refusePrefix(entry.Key, value, "it is not an address prefix")
		}
		if prefix != prefix.Masked() {
			return nil, refusePrefix(entry.Key, value, "it has host bits set; the prefix is "+prefix.Masked().String())
		}
		if err := checkPrefixLength(entry.Key, prefix); err != nil {
			return nil, err
		}
		if prefix.Addr().Is4() {
			if ipv4.IsValid() {
				return nil, refuseSecondPrefix(entry.Key, ipv4, prefix)
			}
			ipv4 = prefix
			continue
		}
		if ipv6.IsValid() {
			return nil, refuseSecondPrefix(entry.Key, ipv6, prefix)
		}
		ipv6 = prefix
	}
	var prefixes []netip.Prefix
	if ipv4.IsValid() {
		prefixes = append(prefixes, ipv4)
	}
	if ipv6.IsValid() {
		prefixes = append(prefixes, ipv6)
	}
	return prefixes, nil
}

// checkPrefixLength is rule V-13: IPv4 1 to 30, IPv6 1 to 126, so the
// network holds a gateway and at least one container.
func checkPrefixLength(name string, prefix netip.Prefix) error {
	if prefix.Bits() < 1 {
		return refuseLength(name, prefix)
	}
	if prefix.Addr().Is4() {
		if prefix.Bits() > ipv4LengthMax {
			return refuseLength(name, prefix)
		}
		return nil
	}
	if prefix.Bits() > ipv6LengthMax {
		return refuseLength(name, prefix)
	}
	return nil
}

// parseGateways reads the set gateways, at most one per prefix and each a
// host address of its prefix (V-5), and answers one gateway per prefix.
func parseGateways(entry configvalue.ListEntry, prefixes []netip.Prefix) ([]netip.Addr, error) {
	gateways := make([]netip.Addr, len(prefixes))
	for _, value := range configvalue.LeafList(entry.Fields[leafGateway]) {
		address, err := netip.ParseAddr(value)
		if err != nil {
			return nil, refuseGateway(entry.Key, value, "it is not an address")
		}
		index := prefixIndex(prefixes, address)
		if index < 0 {
			return nil, refuseGateway(entry.Key, value, "it is outside the network's prefixes")
		}
		if !isHostAddress(prefixes[index], address) {
			return nil, refuseGateway(entry.Key, value, "it is the network or broadcast address of "+prefixes[index].String())
		}
		if gateways[index].IsValid() {
			return nil, refuseGateway(entry.Key, value, "the network already has the gateway "+gateways[index].String()+" in that family")
		}
		gateways[index] = address
	}
	for index, prefix := range prefixes {
		if !gateways[index].IsValid() {
			gateways[index] = defaultGateway(prefix)
		}
	}
	return gateways, nil
}

// prefixIndex answers the index of the prefix holding address, or -1.
func prefixIndex(prefixes []netip.Prefix, address netip.Addr) int {
	for index, prefix := range prefixes {
		if prefix.Contains(address) {
			return index
		}
	}
	return -1
}

// parseMTU reads the mtu: 68 to 16000, default 1500, and at least 1280 with
// an IPv6 prefix (V-14).
func parseMTU(entry configvalue.ListEntry, prefixes []netip.Prefix) (uint32, error) {
	mtu := int64(mtuDefault)
	if raw, present := entry.Fields[leafMTU]; present {
		value, read := configvalue.Int(raw)
		if !read {
			return 0, refuseMTU(entry.Key, "it is not a whole number")
		}
		mtu = value
	}
	if mtu < mtuMin {
		return 0, refuseMTU(entry.Key, "it is below 68")
	}
	if mtu > mtuMax {
		return 0, refuseMTU(entry.Key, "it is above 16000")
	}
	for _, prefix := range prefixes {
		if prefix.Addr().Is6() {
			if mtu < mtuIPv6Min {
				return 0, opererr.New(
					"network "+entry.Key+": mtu "+strconv.Itoa(int(mtu))+" is below 1280 and the network has an IPv6 prefix",
					"IPv6 needs a link that carries 1280 bytes",
					"set container network "+entry.Key+" mtu 1280",
				)
			}
		}
	}
	return uint32(mtu), nil
}

// refuseOverlap is the overlap half of rule V-5.
func refuseOverlap(network, other Network) error {
	for _, prefix := range network.Prefixes {
		for _, held := range other.Prefixes {
			if prefix.Overlaps(held) {
				return opererr.New(
					"network "+network.Name+": prefix "+prefix.String()+" overlaps network "+other.Name+" ("+held.String()+")",
					"two networks with one range cannot both be routed",
					"set container network "+network.Name+" prefix with a prefix that overlaps no other network",
				)
			}
		}
	}
	return nil
}

// refuseAttachmentsWithoutNetwork refuses an instance attached to a network
// when the config declares none.
func refuseAttachmentsWithoutNetwork(container map[string]any) error {
	for _, entry := range configvalue.ListEntries(container[listInstance]) {
		attachments, err := configorder.Entries(entry.Fields, listNetwork, leafName)
		if err != nil {
			return refuseUnreadable(entry.Key, err)
		}
		if len(attachments) > 0 {
			return refuseUnknownNetwork(entry.Key, attachments[0].Key)
		}
	}
	return nil
}

// checkComponents refuses a network when the interface or firewall component
// is not configured: the bridge, link and table requests are refused while
// its component is not loaded, and a config section is what loads it.
func checkComponents(read delivery) error {
	if !read.hasInterface {
		return opererr.New(
			"a container network is configured and the config has no interface section",
			"ze asks the interface component for each network's bridge and each container's link, and that component runs only when the config holds an interface section",
			"set interface backend netlink",
		)
	}
	if !read.hasFirewall {
		return opererr.New(
			"a container network is configured and the config has no firewall section",
			"ze asks the firewall component for each network's translation tables, and that component runs only when the config holds a firewall section",
			"set firewall backend nft",
		)
	}
	return nil
}

// checkInterfaceConfig is rule V-16: no interface entry is named as a
// network's bridge, and no address the interface config sets lies inside a
// network prefix.
func checkInterfaceConfig(cfg Config, interfaces map[string]any) error {
	for _, value := range interfaces {
		for _, entry := range configvalue.ListEntries(value) {
			for _, network := range cfg.Networks {
				if entry.Key == network.Bridge {
					return refuseInterfaceClash(network, "its bridge device "+network.Bridge, "delete interface "+network.Bridge)
				}
			}
		}
	}
	var addresses []netip.Addr
	collectAddresses(interfaces, 0, &addresses)
	for _, address := range addresses {
		for _, network := range cfg.Networks {
			if prefixIndex(network.Prefixes, address) >= 0 {
				return refuseInterfaceClash(network, "the address "+address.String()+" of its prefix",
					"set container network "+network.Name+" prefix with a prefix that holds no interface address")
			}
		}
	}
	return nil
}

// collectAddresses appends every value of an address leaf or leaf-list under
// node that reads as an address or an address with a length. The interface
// schema nests addresses under several lists; the walk stops at
// interfaceDepthMax, deeper than that schema goes.
func collectAddresses(node map[string]any, depth int, addresses *[]netip.Addr) {
	if depth > interfaceDepthMax {
		return
	}
	for key, value := range node {
		if key == leafAddress {
			for _, word := range configvalue.LeafList(value) {
				if prefix, err := netip.ParsePrefix(word); err == nil {
					*addresses = append(*addresses, prefix.Addr())
					continue
				}
				if address, err := netip.ParseAddr(word); err == nil {
					*addresses = append(*addresses, address)
				}
			}
		}
		if child, isMap := value.(map[string]any); isMap {
			collectAddresses(child, depth+1, addresses)
		}
	}
}

// parseInstances reads every instance's attachments, in the order the
// operator wrote them.
func parseInstances(cfg Config, container map[string]any) ([]Instance, error) {
	var instances []Instance
	count := 0
	for _, entry := range configvalue.ListEntries(container[listInstance]) {
		attachments, err := configorder.Entries(entry.Fields, listNetwork, leafName)
		if err != nil {
			return nil, refuseUnreadable(entry.Key, err)
		}
		if len(attachments) == 0 {
			continue
		}
		instance := Instance{Name: entry.Key}
		for _, attachment := range attachments {
			parsed, err := parseAttachment(cfg, entry.Key, attachment)
			if err != nil {
				return nil, err
			}
			instance.Attachments = append(instance.Attachments, parsed)
		}
		count += len(instance.Attachments)
		if count > attachmentsMax {
			return nil, opererr.New(
				"the config attaches containers to networks more than "+strconv.Itoa(attachmentsMax)+" times",
				"each attachment is a link ze asks the interface component for, and one owner holds at most "+strconv.Itoa(attachmentsMax),
				"delete container instance NAME network NET for the attachments no container needs",
			)
		}
		instances = append(instances, instance)
	}
	return instances, nil
}

// parseAttachment reads one attachment and judges its addresses against its
// network (V-4). The uniqueness rules across attachments are checkAttachments.
func parseAttachment(cfg Config, instance string, entry configorder.Entry) (Attachment, error) {
	network, found := cfg.network(entry.Key)
	if !found {
		return Attachment{}, refuseUnknownNetwork(instance, entry.Key)
	}
	attachment := Attachment{Network: network.Name}
	seen := make([]bool, len(network.Prefixes))
	for _, value := range configvalue.LeafList(entry.Map[leafAddress]) {
		address, err := netip.ParseAddr(value)
		if err != nil {
			return Attachment{}, refuseAddress(instance, network, value, "it is not an address")
		}
		index := prefixIndex(network.Prefixes, address)
		if index < 0 {
			return Attachment{}, refuseAddress(instance, network, value, "it is not inside the network's prefixes")
		}
		if !isHostAddress(network.Prefixes[index], address) {
			return Attachment{}, refuseAddress(instance, network, value, "it is the network or broadcast address of "+network.Prefixes[index].String())
		}
		if address == network.Gateways[index] {
			return Attachment{}, refuseAddress(instance, network, value, "it is the network's gateway")
		}
		if seen[index] {
			return Attachment{}, refuseAddress(instance, network, value, "the attachment already sets an address of that family")
		}
		seen[index] = true
		attachment.Addresses = append(attachment.Addresses, address)
	}
	hardware, err := attachmentHardwareAddress(instance, network.Name, entry)
	if err != nil {
		return Attachment{}, err
	}
	attachment.HardwareAddress = hardware
	return attachment, nil
}

// attachmentHardwareAddress answers the set mac-address in the kernel's form,
// else the derived one.
func attachmentHardwareAddress(instance, network string, entry configorder.Entry) (string, error) {
	raw, present := entry.Map[leafHardware]
	if !present {
		return derivedHardwareAddress(network, instance).String(), nil
	}
	word, isWord := raw.(string)
	if !isWord {
		return "", refuseHardwareAddress(instance, network, "")
	}
	address, err := net.ParseMAC(word)
	if err != nil {
		return "", refuseHardwareAddress(instance, network, word)
	}
	if len(address) != 6 {
		return "", refuseHardwareAddress(instance, network, word)
	}
	return address.String(), nil
}

// checkAttachments judges the attachments of each network together: no
// address held twice (V-4), no hardware address twice (V-17), and enough free
// addresses for every attachment that sets none (V-15).
func checkAttachments(cfg Config) error {
	for _, network := range cfg.Networks {
		if err := checkNetworkAttachments(cfg, network); err != nil {
			return err
		}
	}
	return nil
}

func checkNetworkAttachments(cfg Config, network Network) error {
	addresses := make(map[netip.Addr]string)
	hardware := make(map[string]string)
	needs := make([]int, len(network.Prefixes))
	reserved := make([]int, len(network.Prefixes))
	for _, instance := range cfg.Instances {
		for _, attachment := range instance.Attachments {
			if attachment.Network != network.Name {
				continue
			}
			if holder, held := hardware[attachment.HardwareAddress]; held {
				return opererr.New(
					"containers "+holder+" and "+instance.Name+" have the same hardware address "+attachment.HardwareAddress+" on network "+network.Name,
					"two links with one hardware address both lose traffic",
					"set container instance "+instance.Name+" network "+network.Name+" mac-address with another address",
				)
			}
			hardware[attachment.HardwareAddress] = instance.Name
			for index, prefix := range network.Prefixes {
				address, set := addressOfFamily(attachment.Addresses, prefix)
				if !set {
					needs[index]++
					continue
				}
				if holder, held := addresses[address]; held {
					return refuseAddress(instance.Name, network, address.String(), "container "+holder+" holds it")
				}
				addresses[address] = instance.Name
				reserved[index]++
			}
		}
	}
	for index, prefix := range network.Prefixes {
		free := freeAddressCount(prefix, reserved[index])
		if needs[index] > free {
			return opererr.New(
				"network "+network.Name+": "+strconv.Itoa(needs[index])+" containers attach and prefix "+prefix.String()+" has "+strconv.Itoa(max(free, 0))+" free addresses",
				"each attachment needs an address",
				"set container network "+network.Name+" prefix with a larger prefix, or delete container instance NAME network "+network.Name,
			)
		}
	}
	return nil
}

func refusePrefix(name, value, defect string) error {
	return opererr.New(
		"network "+name+": prefix "+value+" is not a prefix ze accepts: "+defect,
		"the prefix is the network's address range, and ze refuses one it would have to correct",
		"set container network "+name+" prefix with a prefix such as 10.88.0.0/24",
	)
}

func refuseSecondPrefix(name string, held, second netip.Prefix) error {
	return opererr.New(
		"network "+name+": prefix "+second.String()+" is a second prefix of the family of "+held.String(),
		"a network holds one prefix per address family",
		"delete container network "+name+" prefix "+second.String(),
	)
}

func refuseLength(name string, prefix netip.Prefix) error {
	return opererr.New(
		"network "+name+": prefix "+prefix.String()+" has a length a container network cannot use",
		"the network needs room for a gateway and at least one container: IPv4 takes 1 to 30, IPv6 1 to 126",
		"set container network "+name+" prefix with a length inside the range",
	)
}

func refuseGateway(name, value, defect string) error {
	return opererr.New(
		"network "+name+": gateway "+value+" is not a gateway ze accepts: "+defect,
		"the gateway is the router's address on the network and each container's default route",
		"set container network "+name+" gateway with a host address of its prefix",
	)
}

func refuseMTU(name, defect string) error {
	return opererr.New(
		"network "+name+": mtu is not a size from 68 to 16000: "+defect,
		"the mtu is set on the bridge and on both ends of each link",
		"set container network "+name+" mtu 1500",
	)
}

func refuseInterfaceClash(network Network, what, fix string) error {
	return opererr.New(
		"network "+network.Name+": "+what+" is already declared under interface",
		"two owners cannot hold one device name or one address",
		fix,
	)
}

func refuseUnknownNetwork(instance, network string) error {
	return opererr.New(
		"container "+instance+": network "+network+" is not a configured container network",
		"an attachment joins the container to a network the config declares",
		"set container network "+network+" prefix 10.88.0.0/24, or delete container instance "+instance+" network "+network,
	)
}

func refuseUnreadable(instance string, err error) error {
	return opererr.Wrap(err,
		"container "+instance+": the network list arrived without its order",
		"the first attachment is eth0 inside the container, so ze refuses a list whose order it cannot read",
		"ze show config container instance "+instance,
	)
}

func refuseAddress(instance string, network Network, value, defect string) error {
	return opererr.New(
		"container "+instance+": address "+value+" is not an address network "+network.Name+" accepts: "+defect,
		"each container's address is inside its network's prefix, held by no one else, and not the gateway",
		"set container instance "+instance+" network "+network.Name+" address with an address of the prefix",
	)
}

func refuseHardwareAddress(instance, network, value string) error {
	return opererr.New(
		"container "+instance+": mac-address "+value+" on network "+network+" is not six octets in colon form",
		"the address is set on the container's end of the link",
		"set container instance "+instance+" network "+network+" mac-address 02:00:00:00:00:01",
	)
}
